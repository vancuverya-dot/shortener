package main

import (
	"net/http"

	"github.com/vancuverya-dot/shortener/internal/handler"
)

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc(`POST /`, handler.UrlPost)

	mux.HandleFunc("GET /{id}", handler.UrlGet)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	err := http.ListenAndServe(`:8082`, mux)

	if err != nil {
		panic(err)
	}
}
