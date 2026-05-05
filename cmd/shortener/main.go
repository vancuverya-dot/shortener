package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vancuverya-dot/shortener/internal/config"
	"github.com/vancuverya-dot/shortener/internal/config/db"
	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
)

func main() {

	serverConfig := config.LoadServerConfig()

	var writeToDb bool = len(serverConfig.Database_dsn) > 0

	handler.Init(writeToDb, serverConfig.BaseUrl)
	service.InitConsoleLogger()
	defer service.SyncConsoleLogger()

	var err error

	if writeToDb {
		err = migration(serverConfig.Database_dsn)
		if err != nil {
			service.Log.Fatalf(err.Error(), "event", "migrate")
		}

		err = db.InitDB(serverConfig.Database_dsn)
		if err != nil {
			service.Log.Fatalf(err.Error(), "event", "init db")
		}
		defer db.CloseDB()
	}

	handler.Urls, err = DeserializeFromFile(serverConfig.FileStorage)
	if err != nil {
		service.Log.Warnf(err.Error(), "event", "deserialize file")
		handler.Urls = make(map[string]string)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r := chi.NewRouter()

	r.Use(GzipMiddleware)
	r.Use(Logging)
	r.Get("/ping", handler.PingDB)
	r.Post("/", handler.UrlPost)
	r.Get("/{id}", handler.UrlGet)
	r.Post("/api/shorten/batch", handler.UrlPostBatch)
	r.Post("/api/shorten", handler.UrlPostJson)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	server := &http.Server{
		Addr:    serverConfig.ServerAddress,
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
	}()

	select {
	case err := <-errCh:
		service.Log.Errorw(err.Error(), "event", "start server failed")
		return
	case <-ctx.Done():
		service.Log.Infow("server is shutting down", "event", "signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		service.Log.Errorw(err.Error(), "event", "server shutdown")
	}

	if !writeToDb {
		SerializeToFile(serverConfig.FileStorage, handler.Urls)
	}

}
