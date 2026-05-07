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
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

var Urls map[string]string
var urlsMu sync.RWMutex
var gen nanoid.Interface

type UrlRequest struct {
	Url string `json:"url"`
}

type UrlResponse struct {
	Result string `json:"result"`
}

var _writeToDb bool = false
var _servPath string = ""

type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func Init(writeToDb bool, servPath string) {
	_writeToDb = writeToDb
	_servPath = servPath

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

	for _, req := range requests {
		id, err := gen.New()
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
			return
		}

		if _writeToDb {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			_, err := storage.InsertURL(ctx, id.String(), req.OriginalURL)
			cancel()

			if err != nil && !errors.Is(err, storage.ErrConflict) {
				service.Log.Errorw(err.Error(), "event", "shortener - Error inserting URL into database")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

		} else {
			setURL(id.String(), req.OriginalURL)
		}

		responses = append(responses, BatchResponse{
			CorrelationID: req.CorrelationID,
			ShortURL:      _servPath + id.String(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(responses)
}

func PingDB(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := storage.PingDB(ctx)
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		service.Log.Fatalf("Database ping failed", "error", err.Error())
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

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if _writeToDb {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), bodyString)
		if err != nil {
			if errors.Is(err, storage.ErrConflict) {
				w.Header().Set("Content-Type", "plain/text")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(_servPath + shortURL))
				return
			}
			http.Error(w, "internal error", http.StatusConflict)
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

	service.Log.Infof("Received URL: %s", urlRequest.Url)

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	if _writeToDb {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		shortURL, err := storage.InsertURL(ctx, id.String(), urlRequest.Url)
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

		target, err := storage.GetOriginalURL(ctx, getURL(r.PathValue("id")))
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "shortener - Error retrieving URL from database")
			http.Error(w, "500_error", http.StatusConflict)
			return
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, Urls[r.PathValue("id")], http.StatusTemporaryRedirect)
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
