package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBraveBackend_Name(t *testing.T) {
	b := NewBraveBackend("key", 10*time.Second)
	if b.Name() != "brave" {
		t.Errorf("expected 'brave', got %q", b.Name())
	}
}

func TestBraveBackend_IsAvailable(t *testing.T) {
	tests := []struct {
		apiKey string
		want   bool
	}{
		{"", false},
		{"some-key", true},
	}
	for _, tt := range tests {
		b := NewBraveBackend(tt.apiKey, 10*time.Second)
		if got := b.IsAvailable(); got != tt.want {
			t.Errorf("IsAvailable(%q) = %v, want %v", tt.apiKey, got, tt.want)
		}
	}
}

func TestBraveBackend_Search_Unavailable(t *testing.T) {
	b := NewBraveBackend("", 10*time.Second)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for unavailable backend")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeUnavailable {
		t.Errorf("expected ErrCodeUnavailable, got %d", backendErr.Code)
	}
}

func newTestBraveBackend(serverURL, apiKey string) *BraveBackend {
	return &BraveBackend{
		APIKey:  apiKey,
		Timeout: 10 * time.Second,
		BaseURL: serverURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func TestBraveBackend_Search_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and auth header
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("X-Subscription-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("q") != "golang" {
			t.Errorf("expected query 'golang', got %q", r.URL.Query().Get("q"))
		}

		resp := braveSearchResponse{
			Query: braveQuery{Original: "golang"},
			Web: braveWebResults{
				Results: []braveResult{
					{Title: "Go Lang", URL: "https://go.dev", Description: "Official Go site"},
					{Title: "Go Wikipedia", URL: "https://en.wikipedia.org/wiki/Go", Description: "Wiki article"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "test-key")
	results, err := b.Search(SearchOptions{Query: "golang", NumResults: 5})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Go Lang" {
		t.Errorf("expected 'Go Lang', got %q", results[0].Title)
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("expected 'https://go.dev', got %q", results[0].URL)
	}
	if results[0].Content != "Official Go site" {
		t.Errorf("expected 'Official Go site', got %q", results[0].Content)
	}
	if results[0].Engine != "brave" {
		t.Errorf("expected engine 'brave', got %q", results[0].Engine)
	}
}

func TestBraveBackend_Search_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid key"}`))
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "bad-key")
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeAuth {
		t.Errorf("expected ErrCodeAuth, got %d", backendErr.Code)
	}
}

func TestBraveBackend_Search_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "key")
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeRateLimit {
		t.Errorf("expected ErrCodeRateLimit, got %d", backendErr.Code)
	}
}

func TestBraveBackend_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "key")
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeInvalidResponse {
		t.Errorf("expected ErrCodeInvalidResponse, got %d", backendErr.Code)
	}
}

func TestBraveBackend_Search_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "key")
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestBraveBackend_Search_SafeSearch(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("safesearch")
		resp := braveSearchResponse{Web: braveWebResults{Results: []braveResult{}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tests := []struct {
		safeSearch string
		want       string
	}{
		{"none", "off"},
		{"strict", "strict"},
		{"moderate", "moderate"},
		{"", "moderate"}, // default
	}

	for _, tt := range tests {
		b := newTestBraveBackend(server.URL, "key")
		b.Search(SearchOptions{Query: "test", SafeSearch: tt.safeSearch})
		if capturedQuery != tt.want {
			t.Errorf("SafeSearch(%q): expected safesearch=%q, got %q", tt.safeSearch, tt.want, capturedQuery)
		}
	}
}

func TestBraveBackend_Search_Pagination(t *testing.T) {
	var capturedOffset string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedOffset = r.URL.Query().Get("offset")
		resp := braveSearchResponse{Web: braveWebResults{Results: []braveResult{}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBraveBackend(server.URL, "key")
	b.Search(SearchOptions{Query: "test", PageNo: 3, NumResults: 10})
	if capturedOffset != "20" {
		t.Errorf("expected offset=20 for page 3, got %q", capturedOffset)
	}
}
