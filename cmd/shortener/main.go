package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vancuverya-dot/shortener/internal/handler"
)

func main() {

	r := chi.NewRouter()
	r.Post("/", handler.UrlPost)
	r.Get("/{id}", handler.UrlGet)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	err := http.ListenAndServe(`:8080`, r)

	if err != nil {
		panic(err)
	}
}
