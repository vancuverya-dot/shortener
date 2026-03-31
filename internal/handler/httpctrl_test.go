package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestALL(t *testing.T) {
	t.Run("UrlPost_ValidRequest", TestUrlPost_ValidRequest)
	t.Run("UrlPost_InvalidContentType", TestUrlPost_InvalidContentType)
	t.Run("UrlPost_NoContentType", TestUrlPost_NoContentType)
	t.Run("UrlPost_CaseInsensitiveContentType", TestUrlPost_CaseInsensitiveContentType)
	t.Run("UrlPost_EmptyBody", TestUrlPost_EmptyBody)
	t.Run("UrlPost_LongURL", TestUrlPost_LongURL)
	t.Run("UrlPost_MultipleRequests", TestUrlPost_MultipleRequests)
}

func TestUrlPost_ValidRequest(t *testing.T) {
	// Reset the Urls map for clean test
	Urls = make(map[string]string)

	testURL := "https://example.com/long/url/path"
	body := bytes.NewBufferString(testURL)

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	response := w.Body.String()
	if !strings.HasPrefix(response, "http://localhost:8080/") {
		t.Errorf("expected response to start with 'http://localhost:8080/', got %s", response)
	}

	// Extract the ID from response
	id := strings.TrimPrefix(response, "http://localhost:8080/")
	if id == "" {
		t.Error("expected non-empty ID in response")
	}

	// Verify URL was stored
	if Urls[id] != testURL {
		t.Errorf("expected stored URL to be %s, got %s", testURL, Urls[id])
	}
}

func TestUrlPost_InvalidContentType(t *testing.T) {
	testURL := "https://example.com/test"
	body := bytes.NewBufferString(testURL)

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "bad_mime_type") {
		t.Errorf("expected error message to contain 'bad_mime_type', got %s", w.Body.String())
	}
}

func TestUrlPost_NoContentType(t *testing.T) {
	testURL := "https://example.com/test"
	body := bytes.NewBufferString(testURL)

	req := httptest.NewRequest("POST", "/", body)
	// No Content-Type header

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when Content-Type is missing, got %d", w.Code)
	}
}

func TestUrlPost_CaseInsensitiveContentType(t *testing.T) {
	Urls = make(map[string]string)

	testURL := "https://example.com/test"
	body := bytes.NewBufferString(testURL)

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "TEXT/PLAIN")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for uppercase Content-Type, got %d", w.Code)
	}
}

func TestUrlPost_EmptyBody(t *testing.T) {
	Urls = make(map[string]string)

	body := bytes.NewBufferString("")

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify empty URL was stored
	response := w.Body.String()
	id := strings.TrimPrefix(response, "http://localhost:8080/")
	if Urls[id] != "" {
		t.Errorf("expected stored URL to be empty, got %s", Urls[id])
	}
}

func TestUrlPost_LongURL(t *testing.T) {
	Urls = make(map[string]string)

	longURL := "https://example.com/" + strings.Repeat("a", 1000)
	body := bytes.NewBufferString(longURL)

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	response := w.Body.String()
	id := strings.TrimPrefix(response, "http://localhost:8080/")
	if Urls[id] != longURL {
		t.Errorf("expected stored URL to match, got %s", Urls[id])
	}
}

func TestUrlPost_MultipleRequests(t *testing.T) {
	Urls = make(map[string]string)

	Urls1 := []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	}

	var generatedIDs []string

	for _, testURL := range Urls1 {
		body := bytes.NewBufferString(testURL)
		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		UrlPost(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		response := w.Body.String()
		id := strings.TrimPrefix(response, "http://localhost:8080/")
		generatedIDs = append(generatedIDs, id)
	}

	// Verify all Urls are stored correctly
	if len(Urls) != 3 {
		t.Errorf("expected 3 Urls stored, got %d", len(Urls))
	}

	for i, id := range generatedIDs {
		if Urls[id] != Urls1[i] {
			t.Errorf("expected stored URL to be %s, got %s", Urls1[i], Urls[id])
		}
	}
}

func TestUrlGet_ExistingID(t *testing.T) {
	Urls = make(map[string]string)

	testID := "abc123"
	testURL := "https://example.com/test"
	Urls[testID] = testURL

	req := httptest.NewRequest("GET", "/"+testID, nil)
	req.SetPathValue("id", testID)

	w := httptest.NewRecorder()
	UrlGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != testURL {
		t.Errorf("expected response to be %s, got %s", testURL, w.Body.String())
	}
}

func TestUrlGet_NonExistingID(t *testing.T) {
	Urls = make(map[string]string)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")

	w := httptest.NewRecorder()
	UrlGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "" {
		t.Errorf("expected empty response for non-existent ID, got %s", w.Body.String())
	}
}

func TestUrlGet_MultipleStoredUrls(t *testing.T) {
	Urls = make(map[string]string)

	testData := map[string]string{
		"id1": "https://example.com/1",
		"id2": "https://example.com/2",
		"id3": "https://example.com/3",
	}

	for id, url := range testData {
		Urls[id] = url
	}

	for id, expectedURL := range testData {
		req := httptest.NewRequest("GET", "/"+id, nil)
		req.SetPathValue("id", id)

		w := httptest.NewRecorder()
		UrlGet(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for ID %s, got %d", id, w.Code)
		}

		if w.Body.String() != expectedURL {
			t.Errorf("for ID %s: expected %s, got %s", id, expectedURL, w.Body.String())
		}
	}
}

func TestUrlGet_EmptyID(t *testing.T) {
	Urls = make(map[string]string)

	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("id", "")

	w := httptest.NewRecorder()
	UrlGet(w, req)

	if w.Body.String() != "" {
		t.Errorf("expected empty response for empty ID, got %s", w.Body.String())
	}
}

func TestUrlPost_BodyClosing(t *testing.T) {
	Urls = make(map[string]string)

	testURL := "https://example.com/test"
	body := io.NopCloser(bytes.NewBufferString(testURL))

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	UrlPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Try to read from body - should fail if properly closed
	_, err := req.Body.Read(make([]byte, 1))
	if err == nil {
		t.Error("expected error reading closed body")
	}
}
