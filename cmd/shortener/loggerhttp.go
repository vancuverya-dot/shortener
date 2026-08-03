package main

import (
	"net/http"
	"time"

	"github.com/vancuverya-dot/shortener/internal/service"
)

// Logging — HTTP-мидлвара, логирующая каждый обработанный запрос:
// URI, метод, длительность обработки, итоговый код статуса и размер тела
// ответа в байтах. Запись производится через service.Log после завершения
// обработки запроса нижестоящим хэндлером.
func Logging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: http.StatusOK,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}

		h.ServeHTTP(&lw, r)
		duration := time.Since(start)
		service.Log.Infoln(
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
	// responseData накапливает итоговый код статуса и суммарный размер
	// тела ответа, передаваемого через loggingResponseWriter, для
	// последующего логирования в Logging.
	responseData struct {
		status int
		size   int
	}

	// loggingResponseWriter оборачивает http.ResponseWriter, перехватывая
	// вызовы Write и WriteHeader, чтобы накопить статистику ответа
	// (статус-код и размер) в responseData без изменения поведения
	// самой записи в исходный http.ResponseWriter.
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

// Write записывает b в исходный http.ResponseWriter и накапливает
// суммарный размер записанных байт в responseData.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

// WriteHeader устанавливает код статуса в исходном http.ResponseWriter
// и запоминает его в responseData для последующего логирования.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}
