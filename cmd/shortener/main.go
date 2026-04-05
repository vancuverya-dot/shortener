package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vancuverya-dot/shortener/internal/config"
	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
)

func main() {

	handler.InitNanoId()
	serverConfig := config.LoadServerConfig()
	service.InitConsoleLogger()
	defer service.SyncConsoleLogger()

	var err error
	handler.Urls, err = DeserializeFromFile(serverConfig.FileStorage)
	if err != nil {
		service.Log.Errorf(err.Error(), "event", "deserialize file")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r := chi.NewRouter()

	r.Use(GzipMiddleware)
	r.Use(Logging)
	r.Post("/", handler.UrlPost)
	r.Get("/{id}", handler.UrlGet)
	r.Post("/api/shorten", handler.UrlPostJson)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	server := &http.Server{
		Addr:    serverConfig.ServerAddress,
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			service.Log.Fatalw(err.Error(), "event", "start server")
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		service.Log.Errorw(err.Error(), "event", "server shutdown")
	}

	SerializeToFile(serverConfig.FileStorage, handler.Urls)

}
