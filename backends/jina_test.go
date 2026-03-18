package backends

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJinaBackend_Keyless(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("- [Result A](https://example.com/a)\n- [Result B](https://example.com/b)"))
	}))
	defer server.Close()

	b := NewJinaBackend("", 2*time.Second, true, server.URL)
	results, err := b.Search(SearchOptions{Query: "test", NumResults: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result due to NumResults limit, got %d", len(results))
	}
	if results[0].URL != "https://example.com/a" {
		t.Fatalf("unexpected first result URL: %s", results[0].URL)
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
