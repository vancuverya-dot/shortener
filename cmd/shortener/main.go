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
	"github.com/vancuverya-dot/shortener/internal/storage"

	"net/http/pprof"
)

func main() {

	serverConfig := config.LoadServerConfig()

	var writeToDb bool = len(serverConfig.DatabaseDSN) > 0

	svc := handler.New(writeToDb, "http://"+serverConfig.ServerAddress+"/", serverConfig.AuditFile, serverConfig.AuditUrl)
	service.InitConsoleLogger()
	defer service.SyncConsoleLogger()

	var err error

	if writeToDb {
		err = migration(serverConfig.DatabaseDSN)
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "migrate")
		}

		database, err := db.New(context.Background(), serverConfig.DatabaseDSN)
		if err != nil {
			service.Log.Errorw(err.Error(), "event", "init db")
		}
		storage.Init(database.Pool())
		defer database.Close()
	}

	urls, err := DeserializeFromFile(serverConfig.FileStorage)
	if err != nil {
		service.Log.Warnf(err.Error(), "event", "deserialize file")
		urls = make(map[string]string)
	}
	svc.LoadURLs(urls)

	debugMux := http.NewServeMux()
	debugMux.HandleFunc("/debug/pprof/", pprof.Index)
	debugMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	debugMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	debugMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	debugMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go func() {
		if err := http.ListenAndServe("localhost:8081", debugMux); err != nil && err != http.ErrServerClosed {
			service.Log.Errorw(err.Error(), "event", "pprof server failed")
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r := chi.NewRouter()

	r.Use(GzipMiddleware)
	r.Use(Logging)
	r.Get("/ping", svc.PingDB)
	r.Post("/", svc.UrlPost)
	r.Get("/{id}", svc.UrlGet)
	r.Post("/api/shorten/batch", svc.UrlPostBatch)
	r.Post("/api/shorten", svc.UrlPostJson)
	r.Get("/api/user/urls", svc.GetURLsByUser)
	r.Delete("/api/user/urls", svc.UrlDelete)

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

	svc.Stop()

	if !writeToDb {
		SerializeToFile(serverConfig.FileStorage, svc.DumpURLs())
	}

}
