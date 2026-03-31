package main

import (
	"net/http"

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

	if config.EnvCfg.ServerAddress != "" {
		config.FlagRunAddr = config.EnvCfg.ServerAddress
	}

	if config.EnvCfg.BaseUrl != "" {
		config.BaseUrlAddr = config.EnvCfg.BaseUrl
	}

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

	if err := http.ListenAndServe(config.FlagRunAddr, r); err != nil {
		sugar.Fatalw(err.Error(), "event", "start server")
	}

}
