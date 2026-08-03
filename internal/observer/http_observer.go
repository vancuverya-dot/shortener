package observer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// HTTPObserver — наблюдатель, отправляющий события аудита методом POST
// на удалённый сервер-приёмник. Сетевые сбои и временные ошибки сервера
// (5xx, 429) обрабатываются автоматическими повторами через
// github.com/hashicorp/go-retryablehttp с экспоненциальной задержкой
// между попытками.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт наблюдателя, отправляющего события на указанный URL.
// Используется retryablehttp.Client, обёрнутый в стандартный *http.Client
// через StandardClient() — это позволяет коду Handle оставаться таким же,
// как при использовании обычного http.Client, при этом каждый запрос
// автоматически повторяется при сбое (по умолчанию до 4 попыток
// с экспоненциальной задержкой). Логи самой библиотеки о повторах отключены,
// чтобы не шуметь в стандартный вывод сервиса.
func NewHTTPObserver(url string) *HTTPObserver {
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient.Timeout = 5 * time.Second
	retryClient.RetryWaitMin = 100 * time.Millisecond
	retryClient.RetryWaitMax = 2 * time.Second
	retryClient.RetryMax = 3
	retryClient.Logger = nil

	return &HTTPObserver{
		url:    url,
		client: retryClient.StandardClient(),
	}
}

// Handle сериализует событие в JSON и отправляет его POST-запросом,
// автоматически повторяя попытку при сетевых ошибках или временной
// недоступности сервера-приёмника.
func (h *HTTPObserver) Handle(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}

	resp, err := h.client.Post(h.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("audit: post to %q: %w", h.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("audit: post to %q: unexpected status %d", h.url, resp.StatusCode)
	}

	return nil
}
