package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"

	"github.com/vancuverya-dot/shortener/internal/service"
)

// gzipWriterPool переиспользует *gzip.Writer между запросами, избегая
// повторной аллокации внутренних буферов сжатия
// на каждый ответ. gzip.NewWriter — одна из самых дорогих по памяти операций
// в стандартной библиотеке, поэтому пул даёт заметный эффект при высокой
// частоте запросов.
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipReaderPool переиспользует *gzip.Reader между запросами с входящим
// сжатым телом, аналогично gzipWriterPool.
var gzipReaderPool = sync.Pool{
	New: func() interface{} {
		return new(gzip.Reader)
	},
}

// GzipMiddleware — HTTP-мидлвара, обеспечивающая прозрачное сжатие трафика.
//
// При получении запроса с заголовком Content-Encoding: gzip разжимает тело
// запроса перед передачей дальше по цепочке хэндлеров. При формировании
// ответа сжимает тело методом gzip, если Content-Type ответа содержит
// "application/json" или "text/html", и устанавливает заголовок
// Content-Encoding: gzip. Объекты gzip.Writer/gzip.Reader переиспользуются
// через sync.Pool, чтобы не аллоцировать их заново на каждый запрос.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Header.Get("Content-Encoding") == "gzip" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			gzReader := gzipReaderPool.Get().(*gzip.Reader)
			if err := gzReader.Reset(bytes.NewReader(bodyBytes)); err != nil {
				gzipReaderPool.Put(gzReader)
				service.Log.Debugw(err.Error(), "event", "Invalid gzip data")
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}

			decodedBody, err := io.ReadAll(gzReader)
			gzReader.Close()
			gzipReaderPool.Put(gzReader)
			if err != nil {
				service.Log.Debugw(err.Error(), "event", "Failed to decode gzip body")
				http.Error(w, "Failed to decode gzip body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(decodedBody))
			r.Header.Del("Content-Encoding")
		}

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
			header:         make(http.Header),
		}

		next.ServeHTTP(rw, r)

		maps.Copy(w.Header(), rw.Header())
		contentType := rw.Header().Get("Content-Type")
		shouldCompress := strings.Contains(contentType, "application/json") ||
			strings.Contains(contentType, "text/html")

		if shouldCompress {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")

			w.WriteHeader(rw.statusCode)

			gzWriter := gzipWriterPool.Get().(*gzip.Writer)
			gzWriter.Reset(w)

			gzWriter.Write(rw.body.Bytes())
			gzWriter.Close()

			gzipWriterPool.Put(gzWriter)
			return
		}

		w.WriteHeader(rw.statusCode)
		w.Write(rw.body.Bytes())
	})
}

// responseWriter буферизует тело и код статуса ответа во время обработки
// запроса нижестоящим хэндлером, чтобы GzipMiddleware могла принять решение
// о сжатии после того, как стал известен Content-Type ответа, и только
// затем записать результат в исходный http.ResponseWriter.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	header     http.Header
}

// Header возвращает заголовки буферизованного ответа, лениво инициализируя
// карту заголовков при первом вызове.
func (rec *responseWriter) Header() http.Header {
	if rec.header == nil {
		rec.header = make(http.Header)
	}
	return rec.header
}

// WriteHeader запоминает код статуса ответа без немедленной записи
// в исходный http.ResponseWriter — фактическая запись происходит
// в GzipMiddleware после принятия решения о сжатии.
func (rec *responseWriter) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
}

// Write дописывает байты тела ответа во внутренний буфер вместо немедленной
// отправки клиенту.
func (rec *responseWriter) Write(b []byte) (int, error) {
	return rec.body.Write(b)
}

// gzipReader оборачивает gzip.Reader вместе с исходным io.ReadCloser тела
// запроса, чтобы при закрытии корректно закрывались оба ресурса.
type gzipReader struct {
	*gzip.Reader
	io.ReadCloser
}

// Read читает декомпрессированные данные из вложенного gzip.Reader.
func (gz *gzipReader) Read(p []byte) (n int, err error) {
	return gz.Reader.Read(p)
}

// Close закрывает gzip.Reader, а затем исходный io.ReadCloser тела запроса.
// Возвращает первую встреченную ошибку.
func (gz *gzipReader) Close() error {
	if err := gz.Reader.Close(); err != nil {
		return err
	}
	return gz.ReadCloser.Close()
}
