package backends

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const braveResultsPage = `<html><body><div id="results">
<div class="snippet svelte-x" data-pos="1" data-type="web">
  <a href="https://go.dev/" class="l1">
    <div class="title search-snippet-title">The Go Programming Language</div>
  </a>
  <div class="generic-snippet"><div class="content">Go is an open source programming language.</div></div>
</div>
<div class="snippet svelte-x" data-pos="2" data-type="videos">
  <a href="https://youtube.com/watch?v=x"><div class="title">A video, not a web result</div></a>
</div>
<div class="snippet svelte-x" data-pos="3" data-type="web">
  <a href="https://en.wikipedia.org/wiki/Go_(programming_language)" class="l1">
    <div class="title">Go (programming language) - Wikipedia</div>
  </a>
  <div class="generic-snippet"><div class="content">Go is a general-purpose programming language.</div></div>
</div>
</div></body></html>`

func TestBraveWebBackend_Name_And_Available(t *testing.T) {
	b := NewBraveWebBackend(10 * time.Second)
	if b.Name() != "brave-web" {
		t.Errorf("expected 'brave-web', got %q", b.Name())
	}
	if !b.IsAvailable() {
		t.Error("brave-web backend should always be available (keyless)")
	}
}

func TestBraveWebBackend_Search_ParsesWebResultsOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "golang" {
			t.Errorf("expected query 'golang', got %q", r.URL.Query().Get("q"))
		}
		w.Write([]byte(braveResultsPage))
	}))
	defer server.Close()

	b := NewBraveWebBackend(10 * time.Second)
	b.BaseURL = server.URL
	results, err := b.Search(SearchOptions{Query: "golang"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 web results (video skipped), got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" || results[0].URL != "https://go.dev/" {
		t.Errorf("unexpected first result: %+v", results[0])
	}
	if results[0].Content != "Go is an open source programming language." {
		t.Errorf("unexpected snippet: %q", results[0].Content)
	}
}

func TestBraveWebBackend_Search_ChallengePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><h1>Checking your browser...</h1></body></html>`))
	}))
	defer server.Close()

	b := NewBraveWebBackend(10 * time.Second)
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

func TestBraveWebBackend_Search_GenuinelyEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div id="results"></div></body></html>`))
	}))
	defer server.Close()

	b := NewBraveWebBackend(10 * time.Second)
	b.BaseURL = server.URL
	results, err := b.Search(SearchOptions{Query: "zqxzqxzqx"})
	if err != nil {
		t.Fatalf("genuinely empty page should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %v", results)
	}
}
