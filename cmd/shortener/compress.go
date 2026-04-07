package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/vancuverya-dot/shortener/internal/service"
)

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Header.Get("Content-Encoding") == "gzip" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}
			r.Body.Close()
			gzReader, err := gzip.NewReader(bytes.NewReader(bodyBytes))

			if err != nil {
				service.Log.Debugw(err.Error(), "event", "Invalid gzip data")
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}

			defer gzReader.Close()
			decodedBody, err := io.ReadAll(gzReader)
			if err != nil {
				service.Log.Debugw(err.Error(), "event", "Failed to decode gzip body")
				http.Error(w, "Failed to decode gzip body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(decodedBody))
			r.Header.Del("Content-Encoding")
			// }
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

			gzWriter := gzip.NewWriter(w)
			defer gzWriter.Close()

			gzWriter.Write(rw.body.Bytes())
			return
		}

		w.WriteHeader(rw.statusCode)
		w.Write(rw.body.Bytes())
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	header     http.Header
}

func (rec *responseWriter) Header() http.Header {
	if rec.header == nil {
		rec.header = make(http.Header)
	}
	return rec.header
}

func (rec *responseWriter) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
}

func (rec *responseWriter) Write(b []byte) (int, error) {
	return rec.body.Write(b)
}

type gzipReader struct {
	*gzip.Reader
	io.ReadCloser
}

func (gz *gzipReader) Read(p []byte) (n int, err error) {
	return gz.Reader.Read(p)
}

func (gz *gzipReader) Close() error {
	if err := gz.Reader.Close(); err != nil {
		return err
	}
	return gz.ReadCloser.Close()
}
