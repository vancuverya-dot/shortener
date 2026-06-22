package observer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPObserver struct {
	url    string
	client *http.Client
}

func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

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
