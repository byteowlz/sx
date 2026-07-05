package backends

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// scrapeUserAgent is a browser-like User-Agent used by the keyless scraper
// backends (bing, brave-web). Search engines serve challenge pages or 403s
// to obviously non-browser clients.
const scrapeUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"

// BingBackend implements keyless search by parsing Bing's HTML results.
// No API key required.
type BingBackend struct {
	BaseURL string
	Timeout time.Duration
	client  *http.Client
}

func NewBingBackend(timeout time.Duration) *BingBackend {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &BingBackend{
		BaseURL: "https://www.bing.com",
		Timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (b *BingBackend) Name() string {
	return "bing"
}

// IsAvailable always returns true: the backend is keyless.
func (b *BingBackend) IsAvailable() bool {
	return true
}

func (b *BingBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	query := opts.Query
	if opts.Site != "" {
		query = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	count := opts.NumResults
	if count <= 0 {
		count = 10
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", strconv.Itoa(count))
	if opts.PageNo > 1 {
		params.Set("first", strconv.Itoa((opts.PageNo-1)*count+1))
	}
	switch opts.SafeSearch {
	case "none":
		params.Set("adlt", "off")
	case "moderate":
		params.Set("adlt", "demote")
	case "strict":
		params.Set("adlt", "strict")
	}
	if opts.Language != "" {
		params.Set("setlang", opts.Language)
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
	doc.Find("li.b_algo").Each(func(_ int, sel *goquery.Selection) {
		link := sel.Find("h2 a").First()
		href, ok := link.Attr("href")
		if !ok || href == "" {
			return
		}
		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}

		content := strings.TrimSpace(sel.Find(".b_caption p").First().Text())
		if content == "" {
			content = strings.TrimSpace(sel.Find("p").First().Text())
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     decodeBingURL(href),
			Content: content,
			Engine:  b.Name(),
			Engines: []string{b.Name()},
		})
	})

	// No parsed results on the first page: distinguish "genuinely nothing
	// found" from a challenge/captcha page that has no results container.
	if len(results) == 0 && opts.PageNo <= 1 && doc.Find("#b_results").Length() == 0 {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("no results container in response, likely a bot challenge page"),
			Code:    ErrCodeDegraded,
		}
	}

	// Bing serves plausible-looking but unrelated decoy results to clients
	// it classifies as bots. Silently wrong results are worse than an error,
	// so reject result sets that don't match the query and let fallbacks run.
	if len(results) > 0 && !resultsMatchQuery(opts.Query, results) {
		return nil, &BackendError{
			Backend: b.Name(),
			Err:     fmt.Errorf("results do not match the query, likely bot-decoy results"),
			Code:    ErrCodeDegraded,
		}
	}

	if opts.NumResults > 0 && len(results) > opts.NumResults {
		results = results[:opts.NumResults]
	}
	return results, nil
}

// resultsMatchQuery reports whether the result set mentions the query's
// meaningful terms (4+ characters, to skip stopwords): at least two distinct
// terms across all results, or one for single-term queries. Bing degrades
// bot-classified clients by answering only a head term ("make" instead of
// "how to make sourdough bread") or with entirely unrelated decoys; both
// leave at most one term matched. Matching is prefix-based in both
// directions so that e.g. a "golang" query matches results that only say
// "Go". This is a tripwire for wholesale decoy result sets, not a relevance
// ranker: queries with no meaningful terms pass.
func resultsMatchQuery(query string, results []SearchResult) bool {
	var terms []string
	for _, t := range strings.Fields(strings.ToLower(query)) {
		t = strings.Trim(t, `"'()`)
		if len(t) >= 4 {
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 {
		return true
	}
	need := 2
	if len(terms) == 1 {
		need = 1
	}

	isSep := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}
	matched := make(map[string]struct{})
	for _, r := range results {
		hay := strings.ToLower(r.Title + " " + r.Content + " " + r.URL)
		for _, w := range strings.FieldsFunc(hay, isSep) {
			if len(w) < 2 {
				continue
			}
			for _, t := range terms {
				if strings.HasPrefix(w, t) || strings.HasPrefix(t, w) {
					matched[t] = struct{}{}
					if len(matched) >= need {
						return true
					}
				}
			}
		}
	}
	return false
}

// decodeBingURL resolves Bing's bing.com/ck/a redirect links, which carry the
// target URL base64-encoded in the "u" query parameter (prefixed with "a1").
// Unrecognized links are returned unchanged.
func decodeBingURL(href string) string {
	if !strings.Contains(href, "bing.com/ck/") {
		return href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	enc := u.Query().Get("u")
	if !strings.HasPrefix(enc, "a1") {
		return href
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(enc[2:], "="))
	if err != nil {
		return href
	}
	target := string(decoded)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return href
	}
	return target
}
