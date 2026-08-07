package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/vancuverya-dot/shortener/internal/handler"
	"github.com/vancuverya-dot/shortener/internal/service"
)

// Example_urlPost демонстрирует создание короткой ссылки через POST / с телом
// в формате text/plain. В случае успеха сервис возвращает код 201 Created
// и тело ответа в виде полного короткого URL (base_url + сгенерированный id).
//
// Сам идентификатор генерируется случайно при каждом вызове, поэтому
// в примере проверяется только код статуса и неизменная часть ответа —
// префикс базового адреса.
func Example_urlPost() {
	service.InitConsoleLogger()
	svc, err := handler.New(false, "http://localhost:8080/", "", "", "")
	if err != nil {
		fmt.Println("init failed")
		return
	}

	body := strings.NewReader("https://practicum.yandex.ru/")
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	svc.URLPost(w, req)

	fmt.Println(w.Code)
	fmt.Println(strings.HasPrefix(w.Body.String(), "http://localhost:8080/"))
}

// Example_urlPostJSON демонстрирует создание короткой ссылки через
// POST /api/shorten с телом в формате JSON {"url": "..."}. В случае успеха
// сервис возвращает код 201 Created и тело ответа в формате
// {"result": "<короткий URL>"}.
//
// Как и в ExampleUrlPost, сам идентификатор случаен, поэтому пример
// проверяет только код статуса, Content-Type ответа и структуру JSON.
func Example_urlPostJSON() {
	service.InitConsoleLogger()
	svc, err := handler.New(false, "http://localhost:8080/", "", "", "")
	if err != nil {
		fmt.Println("init failed")
		return
	}

	body := strings.NewReader(`{"url":"https://practicum.yandex.ru/"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	svc.URLPostJSON(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))
	fmt.Println(strings.Contains(w.Body.String(), `"result":"http://localhost:8080/`))
}

// Example_urlGet демонстрирует переход по короткой ссылке через GET /{id}.
// Сервис отвечает кодом 307 Temporary Redirect с заголовком Location,
// указывающим на оригинальный URL, под который была создана короткая ссылка.
//
// Пример сначала создаёт короткую ссылку через UrlPost, чтобы получить
// валидный id, а затем переходит по нему через UrlGet.
func Example_urlGet() {
	service.InitConsoleLogger()
	svc, err := handler.New(false, "http://localhost:8080/", "", "", "")
	if err != nil {
		fmt.Println("init failed")
		return
	}

	postReq := httptest.NewRequest(http.MethodPost, "http://localhost:8080/", strings.NewReader("https://practicum.yandex.ru/"))
	postReq.Header.Set("Content-Type", "text/plain")
	postW := httptest.NewRecorder()
	svc.URLPost(postW, postReq)

	shortURL := postW.Body.String()
	id := shortURL[strings.LastIndex(shortURL, "/")+1:]

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{id}", svc.URLGet)

	getReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/"+id, nil)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)

	fmt.Println(getW.Code)
	fmt.Println(getW.Header().Get("Location"))
}
