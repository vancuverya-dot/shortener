package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
)

// ExampleUrlPost демонстрирует создание короткой ссылки через POST / с телом
// в формате text/plain. В случае успеха сервис возвращает код 201 Created
// и тело ответа в виде полного короткого URL (base_url + сгенерированный id).
//
// Сам идентификатор генерируется случайно при каждом вызове, поэтому
// в примере проверяется только код статуса и неизменная часть ответа —
// префикс базового адреса.
func ExampleUrlPost() {
	service.InitConsoleLogger()
	svc := handler.New(false, "http://localhost:8080/", "", "")

	body := strings.NewReader("https://practicum.yandex.ru/")
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	svc.UrlPost(w, req)

	fmt.Println(w.Code)
	fmt.Println(strings.HasPrefix(w.Body.String(), "http://localhost:8080/"))
}

// ExampleUrlPostJson демонстрирует создание короткой ссылки через
// POST /api/shorten с телом в формате JSON {"url": "..."}. В случае успеха
// сервис возвращает код 201 Created и тело ответа в формате
// {"result": "<короткий URL>"}.
//
// Как и в ExampleUrlPost, сам идентификатор случаен, поэтому пример
// проверяет только код статуса, Content-Type ответа и структуру JSON.
func ExampleUrlPostJson() {
	service.InitConsoleLogger()
	svc := handler.New(false, "http://localhost:8080/", "", "")

	body := strings.NewReader(`{"url":"https://practicum.yandex.ru/"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	svc.UrlPostJson(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))
	fmt.Println(strings.Contains(w.Body.String(), `"result":"http://localhost:8080/`))
}

// ExampleUrlGet демонстрирует переход по короткой ссылке через GET /{id}.
// Сервис отвечает кодом 307 Temporary Redirect с заголовком Location,
// указывающим на оригинальный URL, под который была создана короткая ссылка.
//
// Пример сначала создаёт короткую ссылку через UrlPost, чтобы получить
// валидный id, а затем переходит по нему через UrlGet.
func ExampleUrlGet() {
	service.InitConsoleLogger()
	svc := handler.New(false, "http://localhost:8080/", "", "")

	postReq := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", strings.NewReader("https://practicum.yandex.ru/"))
	postReq.Header.Set("Content-Type", "text/plain")
	postW := httptest.NewRecorder()
	svc.UrlPost(postW, postReq)

	shortURL := postW.Body.String()
	id := shortURL[strings.LastIndex(shortURL, "/")+1:]

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{id}", svc.UrlGet)

	getReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/"+id, nil)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)

	fmt.Println(getW.Code)
	fmt.Println(getW.Header().Get("Location"))
}
