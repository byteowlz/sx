package backends

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const bingResultsPage = `<html><body><ol id="b_results">
<li class="b_algo"><h2><a href="https://go.dev/">The Go Programming Language</a></h2>
<div class="b_caption"><p>Build simple, secure, scalable systems with Go.</p></div></li>
<li class="b_algo"><h2><a href="https://www.bing.com/ck/a?!&amp;&amp;p=xyz&amp;u=a1aHR0cHM6Ly9lbi53aWtpcGVkaWEub3JnL3dpa2kvR28&amp;ntb=1">Go - Wikipedia</a></h2>
<div class="b_caption"><p>Go is a statically typed language.</p></div></li>
</ol></body></html>`

func TestBingBackend_Name_And_Available(t *testing.T) {
	b := NewBingBackend(10 * time.Second)
	if b.Name() != "bing" {
		t.Errorf("expected 'bing', got %q", b.Name())
	}
	if !b.IsAvailable() {
		t.Error("bing backend should always be available (keyless)")
	}
}

func TestBingBackend_Search_ParsesResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "golang" {
			t.Errorf("expected query 'golang', got %q", r.URL.Query().Get("q"))
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "Mozilla") {
			t.Errorf("expected browser-like User-Agent, got %q", ua)
		}
		w.Write([]byte(bingResultsPage))
	}))
	defer server.Close()

	b := NewBingBackend(10 * time.Second)
	b.BaseURL = server.URL
	results, err := b.Search(SearchOptions{Query: "golang"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" || results[0].URL != "https://go.dev/" {
		t.Errorf("unexpected first result: %+v", results[0])
	}
	if results[0].Content != "Build simple, secure, scalable systems with Go." {
		t.Errorf("unexpected snippet: %q", results[0].Content)
	}
	if results[1].URL != "https://en.wikipedia.org/wiki/Go" {
		t.Errorf("redirect link not decoded, got %q", results[1].URL)
	}
}

func TestBingBackend_Search_ChallengePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><h1>Verify you are human</h1></body></html>`))
	}))
	defer server.Close()

	b := NewBingBackend(10 * time.Second)
	b.BaseURL = server.URL
	_, err := b.Search(SearchOptions{Query: "golang"})
	if err == nil {
		t.Fatal("expected degraded error for challenge page")
	}
	be, ok := err.(*BackendError)
	if !ok || be.Code != ErrCodeDegraded {
		t.Errorf("expected ErrCodeDegraded BackendError, got %v", err)
	}
}

func TestBingBackend_Search_GenuinelyEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><ol id="b_results"><li class="b_no">No results</li></ol></body></html>`))
	}))
	defer server.Close()

	b := NewBingBackend(10 * time.Second)
	b.BaseURL = server.URL
	results, err := b.Search(SearchOptions{Query: "zqxzqxzqx"})
	if err != nil {
		t.Fatalf("genuinely empty page should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %v", results)
	}
}

func TestBingBackend_Search_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	b := NewBingBackend(10 * time.Second)
	b.BaseURL = server.URL
	_, err := b.Search(SearchOptions{Query: "golang"})
	be, ok := err.(*BackendError)
	if !ok || be.Code != ErrCodeRateLimit {
		t.Errorf("expected ErrCodeRateLimit, got %v", err)
	}
}

func TestBingBackend_Search_DecoyResults(t *testing.T) {
	// Bing serves unrelated decoy results to bot-classified clients; these
	// must be rejected so fallbacks run instead of showing wrong results.
	decoyPage := `<html><body><ol id="b_results">
	<li class="b_algo"><h2><a href="https://en.wikipedia.org/wiki/Tom_Cruise">Tom Cruise - Wikipedia</a></h2>
	<div class="b_caption"><p>Filmography of an American actor.</p></div></li>
	</ol></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(decoyPage))
	}))
	defer server.Close()

	b := NewBingBackend(10 * time.Second)
	b.BaseURL = server.URL
	_, err := b.Search(SearchOptions{Query: "how to make sourdough bread"})
	if err == nil {
		t.Fatal("expected degraded error for decoy results")
	}
	be, ok := err.(*BackendError)
	if !ok || be.Code != ErrCodeDegraded {
		t.Errorf("expected ErrCodeDegraded, got %v", err)
	}
}

func TestResultsMatchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		results []SearchResult
		want    bool
	}{
		{
			"honest results match",
			"how to make sourdough bread",
			[]SearchResult{{Title: "Sourdough Bread: A Beginner's Guide", Content: "step by step"}},
			true,
		},
		{
			"decoy results rejected",
			"how to make sourdough bread",
			[]SearchResult{{Title: "Tom Cruise - Wikipedia", Content: "American actor", URL: "https://en.wikipedia.org/wiki/Tom_Cruise"}},
			false,
		},
		{
			"single term match",
			"golang",
			[]SearchResult{{Title: "The Go Programming Language", URL: "https://golang.org"}},
			true,
		},
		{
			"short-word-only query not checked",
			"go db api",
			[]SearchResult{{Title: "Anything at all"}},
			true,
		},
		{
			"head-term-only junk rejected",
			"how to make sourdough bread",
			[]SearchResult{
				{Title: "Make | AI Workflow Automation", URL: "https://www.make.com/en"},
				{Title: "GNU Make", URL: "https://www.gnu.org/software/make/"},
			},
			false,
		},
		{
			"prefix variation matches",
			"golang concurrency patterns",
			[]SearchResult{{Title: "Concurrency in Go", Content: "Patterns for goroutines and channels"}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultsMatchQuery(tt.query, tt.results); got != tt.want {
				t.Errorf("resultsMatchQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestDecodeBingURL(t *testing.T) {
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			"direct link untouched",
			"https://go.dev/",
			"https://go.dev/",
		},
		{
			"ck redirect decoded",
			"https://www.bing.com/ck/a?!&&p=xyz&u=a1aHR0cHM6Ly9nby5kZXYv&ntb=1",
			"https://go.dev/",
		},
		{
			"ck redirect with padding decoded",
			"https://www.bing.com/ck/a?u=a1aHR0cHM6Ly9lbi53aWtpcGVkaWEub3JnL3dpa2kvR28=",
			"https://en.wikipedia.org/wiki/Go",
		},
		{
			"missing a1 prefix untouched",
			"https://www.bing.com/ck/a?u=bogus",
			"https://www.bing.com/ck/a?u=bogus",
		},
		{
			"non-url payload untouched",
			"https://www.bing.com/ck/a?u=a1bm90YXVybA",
			"https://www.bing.com/ck/a?u=a1bm90YXVybA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeBingURL(tt.href); got != tt.want {
				t.Errorf("decodeBingURL(%q) = %q, want %q", tt.href, got, tt.want)
			}
		})
	}
}
