package main

import (
	"net/http"
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

	if config.EnvCfg.ServerAddress != "" {
		config.FlagRunAddr = config.EnvCfg.ServerAddress
	}

	if config.EnvCfg.BaseUrl != "" {
		config.BaseUrlAddr = config.EnvCfg.BaseUrl
	}

	r := chi.NewRouter()

	//r.Use(middleware.Logger)

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

func Logging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}

		h.ServeHTTP(&lw, r)
		duration := time.Since(start)
		sugar.Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"duration", duration,
			"status", responseData.status,
			"size", responseData.size,
		)
	}
	return http.HandlerFunc(logFn)
}

type (
	responseData struct {
		status int
		size   int
	}
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}
