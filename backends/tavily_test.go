package backends

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTavilyBackend_Name(t *testing.T) {
	b := NewTavilyBackend("key", 10*time.Second, "basic", false, false)
	if b.Name() != "tavily" {
		t.Errorf("expected 'tavily', got %q", b.Name())
	}
}

func TestTavilyBackend_IsAvailable(t *testing.T) {
	tests := []struct {
		apiKey string
		want   bool
	}{
		{"", false},
		{"tvly-xxx", true},
	}
	for _, tt := range tests {
		b := NewTavilyBackend(tt.apiKey, 10*time.Second, "basic", false, false)
		if got := b.IsAvailable(); got != tt.want {
			t.Errorf("IsAvailable(%q) = %v, want %v", tt.apiKey, got, tt.want)
		}
	}
}

func TestTavilyBackend_Defaults(t *testing.T) {
	b := NewTavilyBackend("key", 0, "", false, false)
	if b.Timeout != 15*time.Second {
		t.Errorf("expected default timeout 15s, got %v", b.Timeout)
	}
	if b.SearchDepth != "basic" {
		t.Errorf("expected default search_depth 'basic', got %q", b.SearchDepth)
	}
}

func TestTavilyBackend_Search_Unavailable(t *testing.T) {
	b := NewTavilyBackend("", 10*time.Second, "basic", false, false)
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

func newTestTavilyBackend(serverURL, apiKey, depth string, rawContent, answer bool) *TavilyBackend {
	return &TavilyBackend{
		APIKey:            apiKey,
		Timeout:           10 * time.Second,
		SearchDepth:       depth,
		IncludeRawContent: rawContent,
		IncludeAnswer:     answer,
		BaseURL:           serverURL,
		client:            &http.Client{Timeout: 10 * time.Second},
	}
}

func TestTavilyBackend_Search_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify auth
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Parse request body
		body, _ := io.ReadAll(r.Body)
		var req tavilyRequest
		json.Unmarshal(body, &req)

		if req.Query != "golang" {
			t.Errorf("expected query 'golang', got %q", req.Query)
		}
		if req.SearchDepth != "basic" {
			t.Errorf("expected search_depth 'basic', got %q", req.SearchDepth)
		}

		resp := tavilyResponse{
			Query:        "golang",
			Answer:       "Go is a programming language",
			ResponseTime: "0.5",
			Results: []tavilyResult{
				{Title: "Go Dev", URL: "https://go.dev", Content: "Official Go site", Score: 0.95},
				{Title: "Go Wiki", URL: "https://wiki.com/go", Content: "Go language wiki", Score: 0.85},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "test-key", "basic", false, false)
	results, err := b.Search(SearchOptions{Query: "golang", NumResults: 5})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
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
	if results[0].Engine != "tavily" {
		t.Errorf("expected engine 'tavily', got %q", results[0].Engine)
	}
}

func TestTavilyBackend_Search_WithRawContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req tavilyRequest
		json.Unmarshal(body, &req)

		if !req.IncludeRawContent {
			t.Error("expected include_raw_content to be true")
		}

		resp := tavilyResponse{
			Results: []tavilyResult{
				{
					Title:      "Test",
					URL:        "https://test.com",
					Content:    "Short snippet",
					RawContent: "Full page content with lots of text here",
					Score:      0.9,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "key", "basic", true, false)
	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// When IncludeRawContent is true and RawContent is available, it should be used
	if results[0].Content != "Full page content with lots of text here" {
		t.Errorf("expected raw content, got %q", results[0].Content)
	}
}

func TestTavilyBackend_Search_SiteFilter(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req tavilyRequest
		json.Unmarshal(body, &req)
		capturedQuery = req.Query

		resp := tavilyResponse{Results: []tavilyResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "key", "basic", false, false)
	b.Search(SearchOptions{Query: "test", Site: "example.com"})

	if capturedQuery != "site:example.com test" {
		t.Errorf("expected 'site:example.com test', got %q", capturedQuery)
	}
}

func TestTavilyBackend_Search_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail": {"error": "invalid key"}}`))
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "bad-key", "basic", false, false)
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

func TestTavilyBackend_Search_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`rate limited`))
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "key", "basic", false, false)
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

func TestTavilyBackend_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json at all`))
	}))
	defer server.Close()

	b := newTestTavilyBackend(server.URL, "key", "basic", false, false)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTavilyBackend_Search_NumResults(t *testing.T) {
	var capturedMaxResults int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req tavilyRequest
		json.Unmarshal(body, &req)
		capturedMaxResults = req.MaxResults

		resp := tavilyResponse{Results: []tavilyResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test with valid num
	b := newTestTavilyBackend(server.URL, "key", "basic", false, false)
	b.Search(SearchOptions{Query: "test", NumResults: 7})
	if capturedMaxResults != 7 {
		t.Errorf("expected max_results=7, got %d", capturedMaxResults)
	}

	// Test with 0 (should default to 10)
	b.Search(SearchOptions{Query: "test", NumResults: 0})
	if capturedMaxResults != 10 {
		t.Errorf("expected default max_results=10, got %d", capturedMaxResults)
	}

	// Test with >20 (should cap at 10)
	b.Search(SearchOptions{Query: "test", NumResults: 50})
	if capturedMaxResults != 10 {
		t.Errorf("expected capped max_results=10, got %d", capturedMaxResults)
	}
}
