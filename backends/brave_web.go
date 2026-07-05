package backends

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// BraveWebBackend implements keyless search by parsing Brave Search's HTML
// results (search.brave.com). No API key required, unlike the "brave"
// backend which uses the official API.
type BraveWebBackend struct {
	BaseURL string
	Timeout time.Duration
	client  *http.Client
}

func NewBraveWebBackend(timeout time.Duration) *BraveWebBackend {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &BraveWebBackend{
		BaseURL: "https://search.brave.com",
		Timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (b *BraveWebBackend) Name() string {
	return "brave-web"
}

// IsAvailable always returns true: the backend is keyless.
func (b *BraveWebBackend) IsAvailable() bool {
	return true
}

func (b *BraveWebBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("source", "web")
	if opts.PageNo > 1 {
		params.Set("offset", strconv.Itoa(opts.PageNo-1))
	}
	switch opts.SafeSearch {
	case "none":
		params.Set("safesearch", "off")
	case "moderate", "strict":
		params.Set("safesearch", opts.SafeSearch)
	}

	req, err := http.NewRequest("GET", b.BaseURL+"/search?"+params.Encode(), nil)
	if err != nil {
		return nil, &BackendError{Backend: b.Name(), Err: err, Code: ErrCodeNetwork}
	}
	req.Header.Set("User-Agent", scrapeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, &BackendError{Backend: b.Name(), Err: err, Code: ErrCodeNetwork}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &BackendError{Backend: b.Name(), Err: fmt.Errorf("rate limited"), Code: ErrCodeRateLimit}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &BackendError{Backend: b.Name(), Err: fmt.Errorf("HTTP %d", resp.StatusCode), Code: resp.StatusCode}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, &BackendError{Backend: b.Name(), Err: fmt.Errorf("failed to parse HTML: %v", err), Code: ErrCodeInvalidResponse}
	}

	var results []SearchResult
	doc.Find(`div.snippet[data-type="web"]`).Each(func(_ int, sel *goquery.Selection) {
		link := sel.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok || !strings.HasPrefix(href, "http") {
			return
		}
		title := strings.TrimSpace(link.Find(".title").First().Text())
		if title == "" {
			title = strings.TrimSpace(link.AttrOr("title", ""))
		}
		if title == "" {
			return
		}

		content := strings.TrimSpace(sel.Find(".generic-snippet .content").First().Text())
		if content == "" {
			content = strings.TrimSpace(sel.Find(".snippet-description").First().Text())
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     href,
			Content: content,
			Engine:  b.Name(),
			Engines: []string{b.Name()},
		})
	})

	// No parsed results on the first page: a real "nothing found" page still
	// renders the results container; its absence means a challenge page.
	if len(results) == 0 && opts.PageNo <= 1 && doc.Find("#results").Length() == 0 {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("no results container in response, likely a bot challenge page"),
			Code:    ErrCodeDegraded,
		}
	}

	if opts.NumResults > 0 && len(results) > opts.NumResults {
		results = results[:opts.NumResults]
	}
	return results, nil
}
