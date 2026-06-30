package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/sixafter/nanoid"
	"github.com/vancuverya-dot/shortener/internal/auth"
	"github.com/vancuverya-dot/shortener/internal/observer"
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

type UrlsService struct {
	urls         map[string]string
	urlsMu       sync.RWMutex
	gen          nanoid.Interface
	writeToDB    bool
	servPath     string
	worker       *service.Worker
	audit        *observer.Subject
	fileObserver *observer.FileObserver
}

func New(writeToDB bool, servPath string, auditFile string, auditURL string) *UrlsService {
	s := &UrlsService{
		urls:      make(map[string]string),
		writeToDB: writeToDB,
		servPath:  servPath,
		worker:    service.NewWorker(storage.DeleteURLBatch),
		audit:     observer.NewSubject(),
	}

	if auditFile != "" {
		if fo, err := observer.NewFileObserver(auditFile); err != nil {
			service.Log.Warnw(err.Error(), "event", "audit file observer init failed")
		} else {
			s.fileObserver = fo
			s.audit.Subscribe(fo, 128, func(err error) {
				service.Log.Warnw(err.Error(), "event", "audit file observer failed")
			})
		}

	}
	if auditURL != "" {
		s.audit.Subscribe(observer.NewHTTPObserver(auditURL), 128, func(err error) {
			service.Log.Warnw(err.Error(), "event", "audit http observer failed")
		})
	}

	alphabet := "AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz0123456789"

	gen, err := nanoid.NewGenerator(
		nanoid.WithAlphabet(alphabet),
		nanoid.WithLengthHint(10),
	)
	if err != nil {
		panic(err)
	}
	s.gen = gen

	return s
}

func (s *UrlsService) LoadURLs(urls map[string]string) {
	s.urlsMu.Lock()
	defer s.urlsMu.Unlock()
	s.urls = urls
}

func (s *UrlsService) DumpURLs() map[string]string {
	s.urlsMu.RLock()
	defer s.urlsMu.RUnlock()

	dump := make(map[string]string, len(s.urls))
	for k, v := range s.urls {
		dump[k] = v
	}
	return dump
}

type UrlRequest struct {
	Url string `json:"url"`
}

type UrlResponse struct {
	Result string `json:"result"`
}

type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

type UserURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func (s *UrlsService) UrlPostBatch(w http.ResponseWriter, r *http.Request) {
	var requests []BatchRequest

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	responses := make([]BatchResponse, 0, len(requests))

	if s.writeToDB {
		shortURLs := make([]string, len(requests))
		originalURLs := make([]string, len(requests))

		for i, req := range requests {
			id, err := s.gen.New()
			if err != nil {
				service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			shortURLs[i] = id.String()
			originalURLs[i] = req.OriginalURL
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resultIDs, err := storage.InsertURLBatch(ctx, shortURLs, originalURLs)
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "shortener - Error inserting URL batch into database")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		for i, req := range requests {
			responses = append(responses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      s.servPath + resultIDs[i],
			})
		}
	} else {
		for _, req := range requests {
			id, err := s.gen.New()
			if err != nil {
				service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			s.setURL(id.String(), req.OriginalURL)
			responses = append(responses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      s.servPath + id.String(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	body, _ := json.Marshal(responses)
	w.Write(body)
}

func (s *UrlsService) PingDB(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := storage.PingDB(ctx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		service.Log.Errorw("Database ping failed", "error", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *UrlsService) UrlPost(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	bodyString := string(bodyBytes)
	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error getting or creating user ID")
	}

	id, err := s.gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if s.writeToDB {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), bodyString, userID)
		if err != nil {
			if errors.Is(err, storage.ErrConflict) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(s.servPath + shortURL))
				return
			}
			service.Log.Errorw(err.Error(), "event", "shortener - Error inserting url")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		s.setURL(id.String(), bodyString)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(s.servPath + id.String()))

	s.notifyAudit(observer.NewEvent(observer.ActionShorten, userID, bodyString))
}

func (s *UrlsService) UrlPostJson(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") &&
		!strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var urlRequest UrlRequest

	err := json.NewDecoder(r.Body).Decode(&urlRequest)

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Invalid JSON")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error getting or creating user ID")
	}

	service.Log.Infof("Received URL: %s", urlRequest.Url)

	id, err := s.gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if s.writeToDB {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), urlRequest.Url, userID)
		if err != nil {
			if errors.Is(err, storage.ErrConflict) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)

				body, _ := json.Marshal(UrlResponse{
					Result: s.servPath + shortURL,
				})
				w.Write(body)
				return
			}
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		s.setURL(id.String(), urlRequest.Url)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	urlResonse := UrlResponse{
		Result: s.servPath + id.String(),
	}

	err = json.NewEncoder(w).Encode(urlResonse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	s.notifyAudit(observer.NewEvent(observer.ActionShorten, userID, urlRequest.Url))
}

func (s *UrlsService) UrlGet(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)

	if s.writeToDB {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		target, isDeleted, err := storage.GetOriginalURL(ctx, r.PathValue("id"))
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "shortener - Error retrieving URL from database")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if isDeleted {
			w.WriteHeader(http.StatusGone)
			return
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		s.notifyAudit(observer.NewEvent(observer.ActionFollow, userID, target))
		return
	}

	target := s.getURL(r.PathValue("id"))
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	s.notifyAudit(observer.NewEvent(observer.ActionFollow, userID, target))
}

func (s *UrlsService) setURL(short, original string) {
	s.urlsMu.Lock()
	defer s.urlsMu.Unlock()
	s.urls[short] = original
}

func (s *UrlsService) getURL(short string) string {
	s.urlsMu.RLock()
	defer s.urlsMu.RUnlock()
	v := s.urls[short]
	return v
}

func (s *UrlsService) GetURLsByUser(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error getting or creating user ID")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	shortURLs, originalURLs, err := storage.GetURLsByUser(ctx, userID)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error retrieving URLs by user")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(shortURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responses := make([]UserURLResponse, len(shortURLs))
	for i := range shortURLs {
		responses[i] = UserURLResponse{
			ShortURL:    s.servPath + shortURLs[i],
			OriginalURL: originalURLs[i],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(responses)
	w.Write(body)
}

func (s *UrlsService) UrlDelete(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var shortURLs []string
	if err := json.NewDecoder(r.Body).Decode(&shortURLs); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	go func() {
		s.worker.Add(service.DeleteTask{
			ShortURLs: shortURLs,
			UserID:    userID,
		})
	}()

	w.WriteHeader(http.StatusAccepted)
}

func (s *UrlsService) notifyAudit(event observer.Event) {
	s.audit.Notify(event)
}

func (s *UrlsService) Stop() {
	s.audit.Stop()
	s.worker.Stop()
	if s.fileObserver != nil {
		if err := s.fileObserver.Close(); err != nil {
			service.Log.Warnw(err.Error(), "event", "audit file observer close failed")
		}
	}
}
