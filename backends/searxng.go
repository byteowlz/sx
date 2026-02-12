package backends

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SearxngBackend implements SearchBackend for SearXNG instances
type SearxngBackend struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPMethod string
	Timeout    time.Duration
	NoVerifySSL bool
	NoUserAgent bool
	client     *http.Client
}

// NewSearxngBackend creates a new SearXNG backend
func NewSearxngBackend(baseURL, username, password, httpMethod string, timeout time.Duration, noVerifySSL, noUserAgent bool) *SearxngBackend {
	client := &http.Client{
		Timeout: timeout,
	}

	if noVerifySSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	return &SearxngBackend{
		BaseURL:     baseURL,
		Username:    username,
		Password:    password,
		HTTPMethod:  strings.ToUpper(httpMethod),
		Timeout:     timeout,
		NoVerifySSL: noVerifySSL,
		NoUserAgent: noUserAgent,
		client:      client,
	}
}

// Name returns the backend identifier
func (s *SearxngBackend) Name() string {
	return "searxng"
}

// IsAvailable checks if SearXNG is configured and reachable
func (s *SearxngBackend) IsAvailable() bool {
	if s.BaseURL == "" {
		return false
	}
	
	// Try a simple health check or just validate URL is parseable
	u, err := url.Parse(s.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	
	return true
}

// Search performs a search against SearXNG
func (s *SearxngBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if !s.IsAvailable() {
		return nil, &BackendError{
			Backend: s.Name(),
			Err:     fmt.Errorf("SearXNG URL not configured"),
			Code:    ErrCodeUnavailable,
		}
	}

	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	var searchURL string
	var reqBody io.Reader

	if s.HTTPMethod == "POST" {
		searchURL = fmt.Sprintf("%s/search", s.BaseURL)
		data := s.buildParams(query, opts)
		reqBody = strings.NewReader(data.Encode())
	} else {
		u, err := url.Parse(s.BaseURL + "/search")
		if err != nil {
			return nil, &BackendError{
				Backend: s.Name(),
				Err:     fmt.Errorf("invalid SearXNG URL: %v", err),
				Code:    ErrCodeInvalidResponse,
			}
		}
		u.RawQuery = s.buildParams(query, opts).Encode()
		searchURL = u.String()
	}

	var req *http.Request
	var err error

	if s.HTTPMethod == "POST" {
		req, err = http.NewRequest("POST", searchURL, reqBody)
		if err != nil {
			return nil, s.wrapError(err, ErrCodeNetwork)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest("GET", searchURL, nil)
		if err != nil {
			return nil, s.wrapError(err, ErrCodeNetwork)
		}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	if !s.NoUserAgent {
		req.Header.Set("User-Agent", "sx/2.0")
	}

	if s.Username != "" && s.Password != "" {
		req.SetBasicAuth(s.Username, s.Password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, s.wrapError(err, ErrCodeNetwork)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &BackendError{
			Backend: s.Name(),
			Err:     fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)),
			Code:    resp.StatusCode,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, s.wrapError(err, ErrCodeInvalidResponse)
	}

	var searchResp SearxngResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, s.wrapError(fmt.Errorf("failed to parse JSON: %v", err), ErrCodeInvalidResponse)
	}

	// Transform SearxngResponse to []SearchResult
	results := make([]SearchResult, len(searchResp.Results))
	for i, r := range searchResp.Results {
		results[i] = SearchResult(r)
	}

	return results, nil
}

// buildParams constructs URL parameters for SearXNG
func (s *SearxngBackend) buildParams(query string, opts SearchOptions) url.Values {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")

	if len(opts.Categories) > 0 {
		normalized := make([]string, len(opts.Categories))
		for i, cat := range opts.Categories {
			normalized[i] = normalizeCategory(cat)
		}
		params.Set("categories", strings.Join(normalized, ","))
	}

	if len(opts.Engines) > 0 {
		params.Set("engines", strings.Join(opts.Engines, ","))
	}

	if opts.Language != "" {
		params.Set("language", opts.Language)
	}

	if opts.SafeSearch != "" {
		if val, ok := safeSearchOptions[opts.SafeSearch]; ok {
			params.Set("safesearch", strconv.Itoa(val))
		}
	}

	if opts.TimeRange != "" {
		params.Set("time_range", opts.TimeRange)
	}

	if opts.PageNo > 1 {
		params.Set("pageno", strconv.Itoa(opts.PageNo))
	}

	return params
}

func (s *SearxngBackend) wrapError(err error, code int) *BackendError {
	return &BackendError{
		Backend: s.Name(),
		Err:     err,
		Code:    code,
	}
}

// Internal response type for parsing SearXNG JSON
type SearxngResponse struct {
	Results []searxngResult `json:"results"`
}

type searxngResult SearchResult

var safeSearchOptions = map[string]int{
	"none":     0,
	"moderate": 1,
	"strict":   2,
}

// normalizeCategory converts category aliases to canonical form
func normalizeCategory(category string) string {
	aliases := map[string]string{
		"social+media": "social media",
		"social-media": "social media",
		"social_media": "social media",
		"socialmedia":  "social media",
	}
	if canonical, ok := aliases[category]; ok {
		return canonical
	}
	return category
}
