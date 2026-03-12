package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMultiSearxngBackend_IsAvailable(t *testing.T) {
	b := NewMultiSearxngBackend(
		[]string{"", "not-a-url"},
		"", "", "GET", 2*time.Second, false, false,
		SearxngStrategyOrdered,
	)

	if b.IsAvailable() {
		t.Fatal("expected backend to be unavailable")
	}
}

func TestMultiSearxngBackend_SearchOrderedFallsBack(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream error"))
	}))
	defer failing.Close()

	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearxngResponse{
			Results: []searxngResult{{
				Title: "fallback result",
				URL:   "https://example.com/fallback",
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer working.Close()

	b := NewMultiSearxngBackend(
		[]string{failing.URL, working.URL},
		"", "", "GET", 2*time.Second, false, false,
		SearxngStrategyOrdered,
	)

	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("expected successful fallback, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "fallback result" {
		t.Fatalf("unexpected result title: %q", results[0].Title)
	}
}

func TestMultiSearxngBackend_SearchParallelFastest(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		resp := SearxngResponse{Results: []searxngResult{{Title: "slow", URL: "https://example.com/slow"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer slow.Close()

	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearxngResponse{Results: []searxngResult{{Title: "fast", URL: "https://example.com/fast"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer fast.Close()

	b := NewMultiSearxngBackend(
		[]string{slow.URL, fast.URL},
		"", "", "GET", 2*time.Second, false, false,
		SearxngStrategyParallelFastest,
	)

	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "fast" {
		t.Fatalf("expected fastest result, got %q", results[0].Title)
	}
}

func TestDeduplicateSearxngURLs(t *testing.T) {
	urls := []string{"", "https://a.example", "https://a.example", "https://b.example"}
	got := DeduplicateSearxngURLs(urls)
	if len(got) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(got))
	}
	if got[0] != "https://a.example" || got[1] != "https://b.example" {
		t.Fatalf("unexpected deduped urls: %#v", got)
	}
}
