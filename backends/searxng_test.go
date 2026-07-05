package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearxngBackend_Name(t *testing.T) {
	b := NewSearxngBackend("http://localhost", "", "", "GET", 10*time.Second, false, false)
	if b.Name() != "searxng" {
		t.Errorf("expected 'searxng', got %q", b.Name())
	}
}

func TestSearxngBackend_IsAvailable(t *testing.T) {
	tests := []struct {
		baseURL string
		want    bool
	}{
		{"", false},
		{"not-a-url", false},
		{"http://localhost:8888", true},
		{"https://searx.example.com", true},
	}
	for _, tt := range tests {
		b := NewSearxngBackend(tt.baseURL, "", "", "GET", 10*time.Second, false, false)
		if got := b.IsAvailable(); got != tt.want {
			t.Errorf("IsAvailable(%q) = %v, want %v", tt.baseURL, got, tt.want)
		}
	}
}

func TestSearxngBackend_Search_Unavailable(t *testing.T) {
	b := NewSearxngBackend("", "", "", "GET", 10*time.Second, false, false)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for unavailable backend")
	}
}

func TestSearxngBackend_Search_GET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("q") != "golang" {
			t.Errorf("expected query 'golang', got %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("expected format 'json', got %q", r.URL.Query().Get("format"))
		}

		resp := SearxngResponse{
			Results: []searxngResult{
				{
					Title:   "Go Dev",
					URL:     "https://go.dev",
					Content: "Official Go site",
					Engines: []string{"google", "duckduckgo"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// The server URL includes no /search path, so we remove the trailing slash
	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	results, err := b.Search(SearchOptions{Query: "golang"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Go Dev" {
		t.Errorf("expected 'Go Dev', got %q", results[0].Title)
	}
}

func TestSearxngBackend_Search_EmptyWithUnresponsiveEngines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results": [], "unresponsive_engines": [["brave", "Suspended: too many requests"], ["startpage", "Suspended: CAPTCHA"]]}`))
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	_, err := b.Search(SearchOptions{Query: "golang"})
	if err == nil {
		t.Fatal("expected degraded-instance error for empty results with unresponsive engines")
	}
	be, ok := err.(*BackendError)
	if !ok {
		t.Fatalf("expected *BackendError, got %T", err)
	}
	if be.Code != ErrCodeDegraded {
		t.Errorf("expected ErrCodeDegraded, got %d", be.Code)
	}
	for _, want := range []string{"brave (Suspended: too many requests)", "startpage (Suspended: CAPTCHA)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestSearxngBackend_Search_EmptyWithoutUnresponsiveEngines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results": [], "unresponsive_engines": []}`))
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	results, err := b.Search(SearchOptions{Query: "golang"})
	if err != nil {
		t.Fatalf("genuinely empty result set should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %v", results)
	}
}

func TestSearxngBackend_Search_EmptyLaterPageWithUnresponsiveEngines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results": [], "unresponsive_engines": [["brave", "Suspended: too many requests"]]}`))
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	results, err := b.Search(SearchOptions{Query: "golang", PageNo: 3})
	if err != nil {
		t.Fatalf("empty later page should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %v", results)
	}
}

func TestFormatUnresponsiveEngines(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty field", ``, ""},
		{"empty list", `[]`, ""},
		{"name and reason", `[["brave", "Suspended: too many requests"]]`, "brave (Suspended: too many requests)"},
		{"name only", `[["brave"]]`, "brave"},
		{"extra fields", `[["brave", "rate limited", true]]`, "brave (rate limited, true)"},
		{"unexpected shape", `{"brave": "down"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUnresponsiveEngines(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("formatUnresponsiveEngines(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSearxngBackend_Search_POST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content-type, got %q", r.Header.Get("Content-Type"))
		}

		r.ParseForm()
		if r.FormValue("q") != "test" {
			t.Errorf("expected query 'test', got %q", r.FormValue("q"))
		}

		resp := SearxngResponse{
			Results: []searxngResult{
				{Title: "POST Result", URL: "https://post.com"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "POST", 10*time.Second, false, false)
	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 || results[0].Title != "POST Result" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestSearxngBackend_Search_WithBasicAuth(t *testing.T) {
	var capturedUser, capturedPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser, capturedPass, _ = r.BasicAuth()

		resp := SearxngResponse{Results: []searxngResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "user", "pass", "GET", 10*time.Second, false, false)
	b.Search(SearchOptions{Query: "test"})

	if capturedUser != "user" || capturedPass != "pass" {
		t.Errorf("expected user/pass, got %q/%q", capturedUser, capturedPass)
	}
}

func TestSearxngBackend_Search_WithSiteFilter(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		resp := SearxngResponse{Results: []searxngResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	b.Search(SearchOptions{Query: "test", Site: "example.com"})

	if capturedQuery != "site:example.com test" {
		t.Errorf("expected 'site:example.com test', got %q", capturedQuery)
	}
}

func TestSearxngBackend_Search_WithCategories(t *testing.T) {
	var capturedCategories string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCategories = r.URL.Query().Get("categories")
		resp := SearxngResponse{Results: []searxngResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	b.Search(SearchOptions{Query: "test", Categories: []string{"news", "social-media"}})

	if capturedCategories != "news,social media" {
		t.Errorf("expected 'news,social media', got %q", capturedCategories)
	}
}

func TestSearxngBackend_Search_WithTimeRange(t *testing.T) {
	var capturedTimeRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTimeRange = r.URL.Query().Get("time_range")
		resp := SearxngResponse{Results: []searxngResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	b.Search(SearchOptions{Query: "test", TimeRange: "week"})

	if capturedTimeRange != "week" {
		t.Errorf("expected 'week', got %q", capturedTimeRange)
	}
}

func TestSearxngBackend_Search_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestSearxngBackend_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	_, err := b.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSearxngBackend_Search_UserAgent(t *testing.T) {
	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		resp := SearxngResponse{Results: []searxngResult{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// With user agent
	b := NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, false)
	b.Search(SearchOptions{Query: "test"})
	if capturedUA != "sx/2.0" {
		t.Errorf("expected 'sx/2.0', got %q", capturedUA)
	}

	// Without user agent
	b = NewSearxngBackend(server.URL, "", "", "GET", 10*time.Second, false, true)
	b.Search(SearchOptions{Query: "test"})
	if capturedUA == "sx/2.0" {
		t.Error("expected no user agent when NoUserAgent=true")
	}
}

func TestNormalizeCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"social-media", "social media"},
		{"social+media", "social media"},
		{"social_media", "social media"},
		{"socialmedia", "social media"},
		{"news", "news"},
		{"general", "general"},
	}
	for _, tt := range tests {
		if got := normalizeCategory(tt.input); got != tt.want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
