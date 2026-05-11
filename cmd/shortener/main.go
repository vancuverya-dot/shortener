package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vancuverya-dot/shortener/internal/config"
	"github.com/vancuverya-dot/shortener/internal/config/db"
	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

func main() {

	serverConfig := config.LoadServerConfig()

	var writeToDb bool = len(serverConfig.DatabaseDSN) > 0

	handler.Init(writeToDb, "http://"+serverConfig.ServerAddress+"/")
	service.InitConsoleLogger()
	defer service.SyncConsoleLogger()

	var err error

	if writeToDb {
		err = migration(serverConfig.DatabaseDSN)
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "migrate")
		}

		var dbConn *pgxpool.Pool
		dbConn, err = db.InitDB(serverConfig.DatabaseDSN)
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "init db")
		}
		storage.Init(dbConn)
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
	r.Get("/api/user/urls", handler.GetURLsByUser)
	r.Delete("/api/user/urls", handler.UrlDelete)

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
