package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vancuverya-dot/shortener/internal/config"
	"github.com/vancuverya-dot/shortener/internal/config/db"
	"github.com/vancuverya-dot/shortener/internal/grpcapi"
	"github.com/vancuverya-dot/shortener/internal/grpcapi/proto"
	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
	"google.golang.org/grpc"

	"net/http/pprof"
)

// Значения устанавливаются при сборке через -ldflags -X.
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

// printBuildInfo выводит информацию о сборке в stdout.
// Незаполненные значения заменяются на "N/A".
func buildInfo() {
	fmt.Printf("Build version: %s\n", orNA(buildVersion))
	fmt.Printf("Build date: %s\n", orNA(buildDate))
	fmt.Printf("Build commit: %s\n", orNA(buildCommit))
}

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	service.InitConsoleLogger()
	defer service.SyncConsoleLogger()

	buildInfo()

	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		service.Log.Fatalf("загрузка конфигурации: %v", err)
	}

	var writeToDb bool = len(serverConfig.DatabaseDSN) > 0

	svc, err := handler.New(writeToDb, "http://"+serverConfig.ServerAddress+"/",
		serverConfig.AuditFile, serverConfig.AuditUrl, serverConfig.TrustedSubnet)
	if err != nil {
		service.Log.Fatalf("инициализация хэндлеров: %v", err)
	}

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	r := chi.NewRouter()

	r.Use(GzipMiddleware)
	r.Use(Logging)
	r.Get("/ping", svc.PingDB)
	r.Post("/", svc.URLPost)
	r.Get("/{id}", svc.URLGet)
	r.Post("/api/shorten/batch", svc.URLPostBatch)
	r.Post("/api/shorten", svc.URLPostJSON)
	r.Get("/api/user/urls", svc.GetURLsByUser)
	r.Delete("/api/user/urls", svc.URLDelete)
	r.Get("/api/internal/stats", svc.Stats)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	server := &http.Server{
		Addr:    serverConfig.ServerAddress,
		Handler: r,
	}

	if serverConfig.EnableHTTPS {
		cert, err := generateSelfSignedCert()
		if err != nil {
			service.Log.Fatalf("генерация TLS-сертификата: %v", err)
		}
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if serverConfig.EnableHTTPS {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	grpcServer := grpc.NewServer()
	proto.RegisterShortenerServiceServer(grpcServer, grpcapi.New(svc.Gen(), svc.ServPath(), svc.Audit()))
	go func() {
		listener, err := net.Listen("tcp", serverConfig.GRPCAddress)
		if err != nil {
			errCh <- err
			return
		}
		if err := grpcServer.Serve(listener); err != nil {
			errCh <- err
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

	grpcServer.GracefulStop()
	svc.Stop()

	if !writeToDb {
		if err := SerializeToFile(serverConfig.FileStorage, svc.DumpURLs()); err != nil {
			service.Log.Errorw(err.Error(), "event", "serialize file")
		}
	}

}
