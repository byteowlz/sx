package backends

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TavilyBackend implements SearchBackend for Tavily Search API
type TavilyBackend struct {
	APIKey            string
	Timeout           time.Duration
	SearchDepth       string // "basic" (1 credit) or "advanced" (2 credits)
	IncludeRawContent bool   // Return full page content inline
	IncludeAnswer     bool   // Return a direct answer
	client            *http.Client
}

// NewTavilyBackend creates a new Tavily Search backend
func NewTavilyBackend(apiKey string, timeout time.Duration, searchDepth string, includeRawContent, includeAnswer bool) *TavilyBackend {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	if searchDepth == "" {
		searchDepth = "basic"
	}
	return &TavilyBackend{
		APIKey:            apiKey,
		Timeout:           timeout,
		SearchDepth:       searchDepth,
		IncludeRawContent: includeRawContent,
		IncludeAnswer:     includeAnswer,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the backend identifier
func (t *TavilyBackend) Name() string {
	return "tavily"
}

// IsAvailable checks if Tavily API key is configured
func (t *TavilyBackend) IsAvailable() bool {
	return t.APIKey != ""
}

// tavilyRequest is the POST body for Tavily search
type tavilyRequest struct {
	Query             string `json:"query"`
	SearchDepth       string `json:"search_depth,omitempty"`
	MaxResults        int    `json:"max_results,omitempty"`
	IncludeRawContent bool   `json:"include_raw_content,omitempty"`
	IncludeAnswer     bool   `json:"include_answer,omitempty"`
}

// tavilyResponse is the Tavily search API response
type tavilyResponse struct {
	Query        string          `json:"query"`
	Answer       string          `json:"answer"`
	Results      []tavilyResult  `json:"results"`
	ResponseTime string          `json:"response_time"`
}

type tavilyResult struct {
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Content    string  `json:"content"`
	RawContent string  `json:"raw_content"`
	Score      float64 `json:"score"`
}

// Search performs a search against Tavily Search API
func (t *TavilyBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if !t.IsAvailable() {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("Tavily API key not configured"),
			Code:    ErrCodeUnavailable,
		}
	}

	// Build request body
	numResults := opts.NumResults
	if numResults <= 0 || numResults > 20 {
		numResults = 10
	}

	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	reqBody := tavilyRequest{
		Query:             query,
		SearchDepth:       t.SearchDepth,
		MaxResults:        numResults,
		IncludeRawContent: t.IncludeRawContent,
		IncludeAnswer:     t.IncludeAnswer,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("failed to marshal request: %v", err),
			Code:    ErrCodeInvalidResponse,
		}
	}

	req, err := http.NewRequest("POST", "https://api.tavily.com/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("failed to create request: %v", err),
			Code:    ErrCodeNetwork,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("request failed: %v", err),
			Code:    ErrCodeNetwork,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("failed to read response: %v", err),
			Code:    ErrCodeInvalidResponse,
		}
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 401, 403:
			return nil, &BackendError{
				Backend: t.Name(),
				Err:     fmt.Errorf("authentication failed: %s", string(respBody)),
				Code:    ErrCodeAuth,
			}
		case 429:
			return nil, &BackendError{
				Backend: t.Name(),
				Err:     fmt.Errorf("rate limited: %s", string(respBody)),
				Code:    ErrCodeRateLimit,
			}
		default:
			return nil, &BackendError{
				Backend: t.Name(),
				Err:     fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody)),
				Code:    resp.StatusCode,
			}
		}
	}

	var tavilyResp tavilyResponse
	if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
		return nil, &BackendError{
			Backend: t.Name(),
			Err:     fmt.Errorf("failed to parse JSON: %v", err),
			Code:    ErrCodeInvalidResponse,
		}
	}

	// Convert Tavily results to SearchResult
	results := make([]SearchResult, len(tavilyResp.Results))
	for i, r := range tavilyResp.Results {
		content := r.Content
		if t.IncludeRawContent && r.RawContent != "" {
			content = r.RawContent
		}

		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
			Engine:  t.Name(),
			Engines: []string{t.Name()},
		}
	}

	return results, nil
}
