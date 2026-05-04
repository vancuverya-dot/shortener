package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vancuverya-dot/shortener/internal/handler"
)

func TestRoutes(t *testing.T) {
	handler.Init(false, "http://localhost:8080/")

	mux := http.NewServeMux()
	mux.HandleFunc(`POST /`, handler.UrlPost)
	mux.HandleFunc("GET /{id}", handler.UrlGet)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"POST root", "POST", "http://localhost:8082/", http.StatusCreated},
		{"GET with ID", "GET", "http://localhost:8082/", http.StatusTemporaryRedirect},
		{"DELETE root", "DELETE", "http://localhost:8082/", http.StatusBadRequest},
		{"PUT root", "PUT", "http://localhost:8082/", http.StatusBadRequest},
	}

	getId := ``

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var req *http.Request

			if tt.name == "POST root" {
				req = httptest.NewRequest(tt.method, tt.path,
					strings.NewReader(`https://practicum.yandex.ru/profile/go-advanced/?from=learn_subscriptions-with-prof-recommendations`))
				req.Header.Set("Content-Type", "text/plain")
			} else if tt.name == "GET with ID" {
				req = httptest.NewRequest(tt.method, getId, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if tt.name == "POST root" {
				getId = w.Body.String()
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}
