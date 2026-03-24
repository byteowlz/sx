package backends

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// JinaBackend implements search via Jina Search API (s.jina.ai).
type JinaBackend struct {
	APIKey       string
	AllowKeyless bool
	BaseURL      string
	Timeout      time.Duration
	client       *http.Client
}

func NewJinaBackend(apiKey string, timeout time.Duration, allowKeyless bool, baseURL string) *JinaBackend {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://s.jina.ai/"
	}
	return &JinaBackend{
		APIKey:       apiKey,
		AllowKeyless: allowKeyless,
		BaseURL:      strings.TrimRight(baseURL, "/") + "/",
		Timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
	}
}

func (j *JinaBackend) Name() string {
	return "jina"
}

func (j *JinaBackend) IsAvailable() bool {
	return strings.TrimSpace(j.APIKey) != "" || j.AllowKeyless
}

// jinaRequest is the POST body for Jina search API
type jinaRequest struct {
	Query    string `json:"q"`
	Country  string `json:"gl,omitempty"`
	Language string `json:"hl,omitempty"`
	Location string `json:"location,omitempty"`
}

// jinaResponse is the JSON response from Jina's search API
type jinaResponse struct {
	Code   int          `json:"code"`
	Status int          `json:"status"`
	Data   []jinaResult `json:"data"`
}

type jinaResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

func (j *JinaBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if !j.IsAvailable() {
		return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("Jina backend not configured"), Code: ErrCodeUnavailable}
	}

	reqBody := jinaRequest{
		Query:    opts.Query,
		Language: opts.Language,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("failed to marshal request: %v", err), Code: ErrCodeInvalidResponse}
	}

	req, err := http.NewRequest("POST", j.BaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: err, Code: ErrCodeNetwork}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(j.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+j.APIKey)
	}

	// Use X-Site header for site-scoped searches
	if opts.Site != "" {
		site := opts.Site
		if !strings.HasPrefix(site, "http://") && !strings.HasPrefix(site, "https://") {
			site = "https://" + site
		}
		req.Header.Set("X-Site", site)
	}

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: err, Code: ErrCodeNetwork}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: err, Code: ErrCodeInvalidResponse}
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("authentication failed: %s", string(body)), Code: ErrCodeAuth}
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("rate limited: %s", string(body)), Code: ErrCodeRateLimit}
		}
		return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)), Code: resp.StatusCode}
	}

	var jinaResp jinaResponse
	if err := json.Unmarshal(body, &jinaResp); err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("failed to parse JSON: %v", err), Code: ErrCodeInvalidResponse}
	}

	// Convert Jina results to SearchResult
	var results []SearchResult
	for _, r := range jinaResp.Data {
		content := r.Description
		if content == "" {
			content = r.Content
			// Truncate long content to a reasonable snippet length
			if len(content) > 500 {
				content = content[:500]
			}
		}

		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
			Engine:  j.Name(),
			Engines: []string{j.Name()},
		})
	}

	count := opts.NumResults
	if count > 0 && len(results) > count {
		results = results[:count]
	}
	return results, nil
}
