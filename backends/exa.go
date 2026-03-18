package backends

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	ExaModeAuto = "auto"
	ExaModeAPI  = "api"
	ExaModeMCP  = "mcp"
)

var markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)

// ExaBackend supports Exa via direct API or MCP.
type ExaBackend struct {
	Mode       string
	APIKey     string
	Timeout    time.Duration
	BaseURL    string
	MCPURL     string
	MCPTool    string
	NumResults int
	client     *http.Client
}

func NewExaBackend(mode, apiKey string, timeout time.Duration, mcpURL, mcpTool string, numResults int) *ExaBackend {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if mode == "" {
		mode = ExaModeAuto
	}
	if mcpTool == "" {
		mcpTool = "exa-web-search"
	}
	if numResults <= 0 {
		numResults = 10
	}
	return &ExaBackend{
		Mode:       mode,
		APIKey:     apiKey,
		Timeout:    timeout,
		BaseURL:    "https://api.exa.ai/search",
		MCPURL:     mcpURL,
		MCPTool:    mcpTool,
		NumResults: numResults,
		client:     &http.Client{Timeout: timeout},
	}
}

func (e *ExaBackend) Name() string {
	return "exa"
}

func (e *ExaBackend) IsAvailable() bool {
	switch e.Mode {
	case ExaModeAPI:
		return strings.TrimSpace(e.APIKey) != ""
	case ExaModeMCP:
		return strings.TrimSpace(e.MCPURL) != ""
	case ExaModeAuto:
		fallthrough
	default:
		return strings.TrimSpace(e.APIKey) != "" || strings.TrimSpace(e.MCPURL) != ""
	}
}

func (e *ExaBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	count := opts.NumResults
	if count <= 0 {
		count = e.NumResults
	}
	if count <= 0 {
		count = 10
	}

	switch e.Mode {
	case ExaModeAPI:
		return e.searchAPI(query, count)
	case ExaModeMCP:
		return e.searchMCP(query, count)
	case ExaModeAuto:
		fallthrough
	default:
		if strings.TrimSpace(e.APIKey) != "" {
			results, err := e.searchAPI(query, count)
			if err == nil {
				return results, nil
			}
		}
		if strings.TrimSpace(e.MCPURL) != "" {
			return e.searchMCP(query, count)
		}
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("Exa not configured (need API key or MCP URL)"), Code: ErrCodeUnavailable}
	}
}

type exaAPIRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults,omitempty"`
}

type exaAPIResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Text    string `json:"text"`
		Summary string `json:"summary"`
	} `json:"results"`
}

func (e *ExaBackend) searchAPI(query string, count int) ([]SearchResult, error) {
	if strings.TrimSpace(e.APIKey) == "" {
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("Exa API key not configured"), Code: ErrCodeUnavailable}
	}

	payload, err := json.Marshal(exaAPIRequest{Query: query, NumResults: count})
	if err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: err, Code: ErrCodeInvalidResponse}
	}

	req, err := http.NewRequest("POST", e.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: err, Code: ErrCodeNetwork}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", e.APIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: err, Code: ErrCodeNetwork}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: err, Code: ErrCodeInvalidResponse}
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("authentication failed: %s", string(body)), Code: ErrCodeAuth}
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("rate limited: %s", string(body)), Code: ErrCodeRateLimit}
		}
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)), Code: resp.StatusCode}
	}

	var parsed exaAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("failed to parse JSON: %w", err), Code: ErrCodeInvalidResponse}
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		content := r.Text
		if strings.TrimSpace(content) == "" {
			content = r.Summary
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
			Engine:  e.Name(),
			Engines: []string{e.Name()},
		})
	}

	return results, nil
}

type mcpToolCallResult struct {
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	Content           []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content,omitempty"`
}

func (e *ExaBackend) searchMCP(query string, count int) ([]SearchResult, error) {
	if strings.TrimSpace(e.MCPURL) == "" {
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("Exa MCP URL not configured"), Code: ErrCodeUnavailable}
	}
	client := NewMCPHTTPClient(e.MCPURL, e.Timeout)
	_ = client.Initialize() // best effort for servers that require initialize first

	resultRaw, err := client.CallTool(e.MCPTool, map[string]interface{}{
		"query":       query,
		"num_results": count,
		"numResults":  count,
	})
	if err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: err, Code: ErrCodeNetwork}
	}

	var toolResult mcpToolCallResult
	if err := json.Unmarshal(resultRaw, &toolResult); err != nil {
		return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("failed to parse MCP tool response: %w", err), Code: ErrCodeInvalidResponse}
	}

	results := extractResultsFromStructured(toolResult.StructuredContent, e.Name())
	if len(results) > 0 {
		return results, nil
	}

	for _, c := range toolResult.Content {
		if c.Type != "text" || strings.TrimSpace(c.Text) == "" {
			continue
		}
		parsed := parseMarkdownLinks(c.Text, e.Name())
		if len(parsed) > 0 {
			return parsed, nil
		}
	}

	return nil, &BackendError{Backend: e.Name(), Err: fmt.Errorf("MCP tool returned no parsable search results"), Code: ErrCodeInvalidResponse}
}

func extractResultsFromStructured(raw json.RawMessage, engine string) []SearchResult {
	if len(raw) == 0 {
		return nil
	}

	var withResults struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Text    string `json:"text"`
			Summary string `json:"summary"`
			Snippet string `json:"snippet"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &withResults); err == nil && len(withResults.Results) > 0 {
		results := make([]SearchResult, 0, len(withResults.Results))
		for _, r := range withResults.Results {
			content := firstNonEmpty(r.Text, r.Content, r.Summary, r.Snippet)
			results = append(results, SearchResult{Title: r.Title, URL: r.URL, Content: content, Engine: engine, Engines: []string{engine}})
		}
		return results
	}

	return nil
}

func parseMarkdownLinks(text, engine string) []SearchResult {
	matches := markdownLinkRe.FindAllStringSubmatch(text, -1)
	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		results = append(results, SearchResult{Title: strings.TrimSpace(m[1]), URL: strings.TrimSpace(m[2]), Content: "", Engine: engine, Engines: []string{engine}})
	}
	return results
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
