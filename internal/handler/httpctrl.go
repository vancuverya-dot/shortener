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
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

var (
	Urls       map[string]string
	urlsMu     sync.RWMutex
	gen        nanoid.Interface
	_writeToDb bool   = false
	_servPath  string = ""
	_worker    *service.Worker
)

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

func Init(writeToDb bool, servPath string) {
	_writeToDb = writeToDb
	_servPath = servPath
	_worker = service.NewWorker(storage.DeleteURLBatch)
	Urls = make(map[string]string)

	alphabet := "AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz0123456789"

	var err error
	gen, err = nanoid.NewGenerator(
		nanoid.WithAlphabet(alphabet),
		nanoid.WithLengthHint(10),
	)

	if err != nil {
		panic(err)
	}
}

func UrlPostBatch(w http.ResponseWriter, r *http.Request) {
	var requests []BatchRequest

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		http.Error(w, "bad_mime_type", http.StatusBadRequest)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	responses := make([]BatchResponse, 0, len(requests))

	if _writeToDb {
		shortURLs := make([]string, len(requests))
		originalURLs := make([]string, len(requests))

		for i, req := range requests {
			id, err := gen.New()
			if err != nil {
				service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
				http.Error(w, "internal error", http.StatusInternalServerError)
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
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		for i, req := range requests {
			responses = append(responses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      _servPath + resultIDs[i],
			})
		}
	} else {
		for _, req := range requests {
			id, err := gen.New()
			if err != nil {
				service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			setURL(id.String(), req.OriginalURL)
			responses = append(responses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      _servPath + id.String(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	body, _ := json.Marshal(responses)
	w.Write(body)
}

func PingDB(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := storage.PingDB(ctx)
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		service.Log.Errorw("Database ping failed", "error", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UrlPost(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, "bad_mime_type", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "500_error", http.StatusBadRequest)
		return
	}

	bodyString := string(bodyBytes)
	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error getting or creating user ID")
	}

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if _writeToDb {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), bodyString, userID)
		if err != nil {
			if errors.Is(err, storage.ErrConflict) {
				w.Header().Set("Content-Type", "plain/text")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(_servPath + shortURL))
				return
			}
			service.Log.Errorw(err.Error(), "event", "shortener - Error inserting url")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else {
		setURL(id.String(), bodyString)
	}

	w.Header().Set("Content-Type", "plain/text")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(_servPath + id.String()))
}

func UrlPostJson(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") &&
		!strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, "bad_mime_type", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var urlRequest UrlRequest

	err := json.NewDecoder(r.Body).Decode(&urlRequest)

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Invalid JSON")
		http.Error(w, "shortener - Invalid JSON", http.StatusBadRequest)
		return
	}

	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error getting or creating user ID")
	}

	service.Log.Infof("Received URL: %s", urlRequest.Url)

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if _writeToDb {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), urlRequest.Url, userID)
		if err != nil {
			if errors.Is(err, storage.ErrConflict) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)

				body, _ := json.Marshal(UrlResponse{
					Result: _servPath + shortURL,
				})
				w.Write(body)
				return
			}
			http.Error(w, "internal error", http.StatusConflict)
			return
		}
	} else {
		setURL(id.String(), urlRequest.Url)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	urlResonse := UrlResponse{
		Result: _servPath + id.String(),
	}

	err = json.NewEncoder(w).Encode(urlResonse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func UrlGet(w http.ResponseWriter, r *http.Request) {

	if _writeToDb {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		target, isDeleted, err := storage.GetOriginalURL(ctx, r.PathValue("id"))
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "shortener - Error retrieving URL from database")
			http.Error(w, "500_error", http.StatusInternalServerError)
			return
		}
		if isDeleted {
			w.WriteHeader(http.StatusGone)
			return
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, getURL(r.PathValue("id")), http.StatusTemporaryRedirect)
}

func setURL(short, original string) {
	urlsMu.Lock()
	defer urlsMu.Unlock()
	Urls[short] = original
}

func getURL(short string) string {
	urlsMu.RLock()
	defer urlsMu.RUnlock()
	v := Urls[short]
	return v
}

func GetURLsByUser(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(shortURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responses := make([]UserURLResponse, len(shortURLs))
	for i := range shortURLs {
		responses[i] = UserURLResponse{
			ShortURL:    _servPath + shortURLs[i],
			OriginalURL: originalURLs[i],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(responses)
	w.Write(body)
}

func UrlDelete(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetOrCreateUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var shortURLs []string
	if err := json.NewDecoder(r.Body).Decode(&shortURLs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	go func() {
		_worker.Add(service.DeleteTask{
			ShortURLs: shortURLs,
			UserID:    userID,
		})
	}()

	w.WriteHeader(http.StatusAccepted)
}
