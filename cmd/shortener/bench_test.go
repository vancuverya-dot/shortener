package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/sixafter/nanoid"
	"github.com/vancuverya-dot/shortener/internal/config/db"
	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

// benchDSN читает DSN тестовой БД из переменной окружения DATABASE_DSN —
// той же, что использует сам сервис (см. internal/config) и что передают
// автотесты практикума через флаг -database-dsn. Если переменная не задана,
// бенчмарк, требующий БД, пропускается (b.Skip) — это позволяет гонять
// `go test -bench` локально без БД без падения сборки.
func benchDSN(b *testing.B) string {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		b.Skip("DATABASE_DSN is not set, skipping DB-backed benchmark")
	}
	return dsn
}

// BenchmarkNanoIDGenerate измеряет скорость и аллокации генерации
// короткого идентификатора — это вызывается на каждый POST-запрос.
// Не требует БД и не пропускается.
func BenchmarkNanoIDGenerate(b *testing.B) {
	alphabet := "AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz0123456789"
	gen, err := nanoid.NewGenerator(
		nanoid.WithAlphabet(alphabet),
		nanoid.WithLengthHint(10),
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := gen.New(); err != nil {
			b.Fatal(err)
		}
	}
}

// setupHandlerWithDB поднимает реальное подключение к PostgreSQL через
// db.InitDB, инициализирует storage и handler в режиме записи в БД,
// и возвращает мультиплексор с маршрутами POST / и GET /{id}.
func setupHandlerWithDB(b *testing.B, dsn string) http.Handler {
	b.Helper()

	pool, err := db.InitDB(dsn)
	if err != nil {
		b.Fatalf("connect to db: %v", err)
	}
	b.Cleanup(func() {
		db.CloseDB()
	})

	storage.Init(pool)
	handler.Init(true, "http://localhost:8080/", "", "")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /", handler.UrlPost)
	mux.HandleFunc("GET /{id}", handler.UrlGet)
	return mux
}

// BenchmarkUrlPost измеряет полный цикл обработки POST / с реальной записью
// в PostgreSQL: генерация ID + INSERT (через handler.UrlPost -> storage.InsertURL).
func BenchmarkUrlPost(b *testing.B) {
	dsn := benchDSN(b)
	mux := setupHandlerWithDB(b, dsn)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf("https://example.com/bench/%d", i)
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			b.Fatalf("unexpected status: %d, body: %s", w.Code, w.Body.String())
		}
	}
}

// BenchmarkUrlGet измеряет полный цикл обработки GET /{id} с чтением
// из PostgreSQL (handler.UrlGet -> storage.GetOriginalURL).
//
// Перед измерением заранее создаёт N коротких ссылок через сам сервис,
// чтобы профиль снимался на реалистичном наборе уже существующих записей.
func BenchmarkUrlGet(b *testing.B) {
	dsn := benchDSN(b)
	mux := setupHandlerWithDB(b, dsn)

	const seedSize = 1000
	ids := make([]string, 0, seedSize)
	for i := 0; i < seedSize; i++ {
		body := fmt.Sprintf("https://example.com/bench-get/%d", i)
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			b.Fatalf("seed insert failed: %d, body: %s", w.Code, w.Body.String())
		}

		short := w.Body.String()
		short = short[strings.LastIndex(short, "/")+1:]
		ids = append(ids, short)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/"+id, nil)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			b.Fatalf("unexpected status: %d for id %s (iter %s)", w.Code, id, strconv.Itoa(i))
		}
	}
}
