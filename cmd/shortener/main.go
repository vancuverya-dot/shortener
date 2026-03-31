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
	"go.uber.org/zap"
)

var sugar zap.SugaredLogger

func main() {

	config.ParseFlags()
	config.GetEnvs()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	defer logger.Sync()

	sugar = *logger.Sugar()

	// конфигурация из флагов и переменных окружения
	if config.EnvCfg.ServerAddress != "" {
		config.FlagRunAddr = config.EnvCfg.ServerAddress
	}

	if config.EnvCfg.BaseUrl != "" {
		config.BaseUrlAddr = config.EnvCfg.BaseUrl
	}

	if config.EnvCfg.FileStorage != "" {
		config.FileStorage = config.EnvCfg.FileStorage
	}

	//загрузка данных из файла
	handler.Urls, err = DeserializeFromFile(config.FileStorage)
	if err != nil {
		sugar.Errorf(err.Error(), "event", "deserialize file")
	}

	// Создаем контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	//web сервер
	r := chi.NewRouter()
	//r.Use(middleware.Compress(5, "application/json", "text/html"))
	//r.Use(middleware.Logger)

	r.Use(GzipMiddleware)
	r.Use(Logging)
	r.Post("/", handler.UrlPost)
	r.Get("/{id}", handler.UrlGet)
	r.Post("/api/shorten", handler.UrlPostJson)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not_allowed", http.StatusBadRequest)
	})

	server := &http.Server{
		Addr:    config.FlagRunAddr,
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalw(err.Error(), "event", "start server")
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		sugar.Errorw(err.Error(), "event", "server shutdown")
	}

	SerializeToFile(config.FileStorage, handler.Urls)

}
