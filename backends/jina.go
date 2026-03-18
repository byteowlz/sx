package backends

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// JinaBackend implements keyless/keyed search via Jina search endpoint.
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
		baseURL = "https://s.jina.ai"
	}
	return &JinaBackend{
		APIKey:       apiKey,
		AllowKeyless: allowKeyless,
		BaseURL:      strings.TrimRight(baseURL, "/"),
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

func (j *JinaBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if !j.IsAvailable() {
		return nil, &BackendError{Backend: j.Name(), Err: fmt.Errorf("Jina backend not configured"), Code: ErrCodeUnavailable}
	}

	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	endpoint := fmt.Sprintf("%s/%s", j.BaseURL, url.QueryEscape(query))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, &BackendError{Backend: j.Name(), Err: err, Code: ErrCodeNetwork}
	}
	req.Header.Set("Accept", "text/plain, text/markdown, application/json")
	if strings.TrimSpace(j.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+j.APIKey)
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

	results := parseMarkdownLinks(string(body), j.Name())
	if len(results) == 0 {
		content := string(body)
		if len(content) > 500 {
			content = content[:500]
		}
		return []SearchResult{{
			Title:   "Jina result",
			URL:     endpoint,
			Content: content,
			Engine:  j.Name(),
			Engines: []string{j.Name()},
		}}, nil
	}

	count := opts.NumResults
	if count > 0 && len(results) > count {
		results = results[:count]
	}
	return results, nil
}
