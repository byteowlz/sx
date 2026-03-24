package backends

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJinaBackend_Name(t *testing.T) {
	b := NewJinaBackend("key", 2*time.Second, false, "")
	if b.Name() != "jina" {
		t.Errorf("expected 'jina', got %q", b.Name())
	}
}

func TestJinaBackend_IsAvailable(t *testing.T) {
	tests := []struct {
		apiKey       string
		allowKeyless bool
		want         bool
	}{
		{"", false, false},
		{"key", false, true},
		{"", true, true},
	}
	for _, tt := range tests {
		b := NewJinaBackend(tt.apiKey, 2*time.Second, tt.allowKeyless, "")
		if got := b.IsAvailable(); got != tt.want {
			t.Errorf("IsAvailable(apiKey=%q, keyless=%v) = %v, want %v", tt.apiKey, tt.allowKeyless, got, tt.want)
		}
	}
}

func TestJinaBackend_NotConfigured(t *testing.T) {
	b := NewJinaBackend("", 2*time.Second, false, "https://s.jina.ai")
	if b.IsAvailable() {
		t.Fatal("expected unavailable backend")
	}
	if _, err := b.Search(SearchOptions{Query: "test"}); err == nil {
		t.Fatal("expected error when backend is not configured")
	}
}

func TestJinaBackend_Search_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify POST method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Verify headers
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization: Bearer test-key")
		}

		// Parse request body
		body, _ := io.ReadAll(r.Body)
		var req jinaRequest
		json.Unmarshal(body, &req)

		if req.Query != "golang" {
			t.Errorf("expected query 'golang', got %q", req.Query)
		}

		resp := jinaResponse{
			Code:   200,
			Status: 20000,
			Data: []jinaResult{
				{Title: "Go Dev", URL: "https://go.dev", Description: "Official Go site"},
				{Title: "Go Wiki", URL: "https://wiki.example.com/go", Description: "Go wiki"},
				{Title: "Go Blog", URL: "https://blog.example.com/go", Description: "Go blog"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewJinaBackend("test-key", 2*time.Second, false, server.URL)
	results, err := b.Search(SearchOptions{Query: "golang", NumResults: 2})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (limited by NumResults), got %d", len(results))
	}
	if results[0].Title != "Go Dev" {
		t.Errorf("expected 'Go Dev', got %q", results[0].Title)
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("expected 'https://go.dev', got %q", results[0].URL)
	}
	if results[0].Content != "Official Go site" {
		t.Errorf("expected 'Official Go site', got %q", results[0].Content)
	}
	if results[0].Engine != "jina" {
		t.Errorf("expected engine 'jina', got %q", results[0].Engine)
	}
}

func TestJinaBackend_Search_SiteFilter(t *testing.T) {
	var capturedSiteHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSiteHeader = r.Header.Get("X-Site")

		// Verify query doesn't contain site: prefix
		body, _ := io.ReadAll(r.Body)
		var req jinaRequest
		json.Unmarshal(body, &req)
		if req.Query != "test" {
			t.Errorf("expected clean query 'test', got %q", req.Query)
		}

		resp := jinaResponse{Data: []jinaResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewJinaBackend("key", 2*time.Second, false, server.URL)
	b.Search(SearchOptions{Query: "test", Site: "example.com"})

	if capturedSiteHeader != "https://example.com" {
		t.Errorf("expected X-Site header 'https://example.com', got %q", capturedSiteHeader)
	}
}

func TestJinaBackend_Search_Language(t *testing.T) {
	var capturedLang string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jinaRequest
		json.Unmarshal(body, &req)
		capturedLang = req.Language

		resp := jinaResponse{Data: []jinaResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewJinaBackend("key", 2*time.Second, false, server.URL)
	b.Search(SearchOptions{Query: "test", Language: "de"})

	if capturedLang != "de" {
		t.Errorf("expected language 'de', got %q", capturedLang)
	}
}

func TestJinaBackend_Search_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail": "invalid key"}`))
	}))
	defer server.Close()

	b := NewJinaBackend("bad-key", 2*time.Second, false, server.URL)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeAuth {
		t.Errorf("expected ErrCodeAuth, got %d", backendErr.Code)
	}
}

func TestJinaBackend_Search_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	b := NewJinaBackend("key", 2*time.Second, false, server.URL)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	backendErr, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Code != ErrCodeRateLimit {
		t.Errorf("expected ErrCodeRateLimit, got %d", backendErr.Code)
	}
}

func TestJinaBackend_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	b := NewJinaBackend("key", 2*time.Second, false, server.URL)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJinaBackend_Search_ContentFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jinaResponse{
			Data: []jinaResult{
				{Title: "No Desc", URL: "https://example.com", Description: "", Content: "Full page content here"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewJinaBackend("key", 2*time.Second, false, server.URL)
	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "Full page content here" {
		t.Errorf("expected content fallback, got %q", results[0].Content)
	}
}


