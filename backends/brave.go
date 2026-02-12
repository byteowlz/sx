package backends

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// BraveBackend implements SearchBackend for Brave Search API
type BraveBackend struct {
	APIKey  string
	Timeout time.Duration
	client  *http.Client
}

// NewBraveBackend creates a new Brave Search backend
func NewBraveBackend(apiKey string, timeout time.Duration) *BraveBackend {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &BraveBackend{
		APIKey:  apiKey,
		Timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the backend identifier
func (b *BraveBackend) Name() string {
	return "brave"
}

// IsAvailable checks if Brave API key is configured
func (b *BraveBackend) IsAvailable() bool {
	return b.APIKey != ""
}

// braveSearchResponse matches Brave Search API response structure
type braveSearchResponse struct {
	Query     braveQuery      `json:"query"`
	Web       braveWebResults `json:"web"`
}

type braveQuery struct {
	Original string `json:"original"`
}

type braveWebResults struct {
	Results []braveResult `json:"results"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age,omitempty"`
}

// Search performs a search against Brave Search API
func (b *BraveBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if !b.IsAvailable() {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("Brave API key not configured"),
			Code:    ErrCodeUnavailable,
		}
	}

	// Build URL
	baseURL := "https://api.search.brave.com/res/v1/web/search"
	params := url.Values{}
	params.Set("q", opts.Query)
	
	// Set result count (max 20)
	count := opts.NumResults
	if count <= 0 || count > 20 {
		count = 10
	}
	params.Set("count", fmt.Sprintf("%d", count))
	
	// Offset for pagination
	if opts.PageNo > 1 {
		offset := (opts.PageNo - 1) * count
		params.Set("offset", fmt.Sprintf("%d", offset))
	}
	
	// Safe search
	safeSearch := "moderate"
	if opts.SafeSearch == "none" {
		safeSearch = "off"
	} else if opts.SafeSearch == "strict" {
		safeSearch = "strict"
	}
	params.Set("safesearch", safeSearch)
	
	// Filter by site
	if opts.Site != "" {
		params.Set("site", opts.Site)
	}

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("failed to create request: %v", err),
			Code:    ErrCodeNetwork,
		}
	}

	// Add headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.APIKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("request failed: %v", err),
			Code:    ErrCodeNetwork,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("failed to read response: %v", err),
			Code:    ErrCodeInvalidResponse,
		}
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 401, 403:
			return nil, &BackendError{
				Backend: b.Name(),
				Err:     fmt.Errorf("authentication failed: %s", string(body)),
				Code:    ErrCodeAuth,
			}
		case 429:
			return nil, &BackendError{
				Backend: b.Name(),
				Err:     fmt.Errorf("rate limited: %s", string(body)),
				Code:    ErrCodeRateLimit,
			}
		default:
			return nil, &BackendError{
				Backend: b.Name(),
				Err:     fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)),
				Code:    resp.StatusCode,
			}
		}
	}

	var braveResp braveSearchResponse
	if err := json.Unmarshal(body, &braveResp); err != nil {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("failed to parse JSON: %v", err),
			Code:    ErrCodeInvalidResponse,
		}
	}

	// Convert Brave results to SearchResult
	results := make([]SearchResult, len(braveResp.Web.Results))
	for i, r := range braveResp.Web.Results {
		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Description,
			Engine:  b.Name(),
			Engines: []string{b.Name()},
		}
	}

	return results, nil
}
