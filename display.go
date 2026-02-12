package main

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/fatih/color"
	"github.com/go-shiori/go-readability"

	"sx/backends"
)

const maxContentWords = 128

// Common realistic user agents to rotate through
var userAgents = []string{
	// Chrome on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	// Chrome on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	// Firefox on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	// Firefox on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	// Safari on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	// Edge on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

// SearchResult is an alias for backends.SearchResult
type SearchResult = backends.SearchResult

type SearchOptions struct {
	Categories     []string
	SearxngEngines []string // SearXNG-specific engines (not to confuse with search backends)
	SafeSearch     string
	Language       string
	TimeRange      string
	Site           string
	PageNo         int
	Expand         bool
	JSON           bool
	First          bool
	Lucky          bool
	NoPrompt       bool
	Interactive    bool
	Unsafe         bool
	LinksOnly      bool
	OutputFile     string
	Top            bool
	Clean          bool
	TextOnly       bool
	HTMLOnly       bool
	ExplicitEngine string // --engine flag: force a specific search backend
}

func printResults(results []SearchResult, count int, startAt int, expand bool, noColor bool, query string) {
	if noColor {
		color.NoColor = true
	}

	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.FgHiBlack)

	fmt.Println()

	// Display the query at the top
	bold := color.New(color.FgWhite, color.Bold)
	fmt.Printf("Query: %s\n\n", bold.Sprint(query))
	fmt.Println()

	end := startAt + count
	if end > len(results) {
		end = len(results)
	}

	for i, result := range results[startAt:end] {
		index := startAt + i + 1

		// Format title (truncate if too long)
		title := result.Title
		if title == "" {
			title = "No title"
		}
		if len(title) > 70 {
			title = title[:67] + "..."
		}

		// Extract domain from URL
		domain := extractDomain(result.URL)

		// Format and print result header
		fmt.Printf(" %s %s %s\n",
			cyan.Sprintf("%2d.", index),
			green.Sprint(title),
			yellow.Sprintf("[%s]", domain),
		)

		// Show full URL if expand is enabled
		if expand && result.URL != "" {
			fmt.Printf("     %s\n", result.URL)
		}

		// Format and print content
		if result.Content != "" {
			content := formatContent(result.Content)
			lines := wrapText(content, getTerminalWidth()-5)
			for _, line := range lines {
				fmt.Printf("     %s\n", line)
			}
		}

		// Category-specific formatting
		printCategorySpecific(result, dim)

		// Print engines
		printEngines(result, dim)

		fmt.Println()
	}
}

func extractDomain(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	parts := strings.Split(urlStr, "//")
	if len(parts) > 1 {
		return strings.Split(parts[1], "/")[0]
	}
	return strings.Split(parts[0], "/")[0]
}

func formatContent(content string) string {
	// Simple HTML to text conversion
	content = html.UnescapeString(content)

	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	content = re.ReplaceAllString(content, "")

	// Limit word count
	words := strings.Fields(content)
	if len(words) > maxContentWords {
		words = words[:maxContentWords]
		content = strings.Join(words, " ") + " ..."
	} else {
		content = strings.Join(words, " ")
	}

	return strings.TrimSpace(content)
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= width {
			currentLine.WriteString(" " + word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

func getTerminalWidth() int {
	// Simple fallback - in a real implementation you'd use syscalls
	return 80
}

func printCategorySpecific(result SearchResult, dim *color.Color) {
	switch result.Category {
	case "news":
		if result.PublishedDate != "" {
			if date := parseDate(result.PublishedDate); date != nil {
				fmt.Printf("     %s\n", dim.Sprint(date.Format("January 2, 2006")))
			}
		}

	case "images":
		if result.Source != "" || result.Resolution != "" {
			fmt.Printf("     %s %s\n",
				dim.Sprint(result.Resolution),
				dim.Sprint(result.Source))
		}
		if result.ImgSrc != "" {
			fmt.Printf("     %s\n", result.ImgSrc)
		}

	case "videos", "music":
		var parts []string
		if result.Length != nil {
			if lengthStr := formatLength(result.Length); lengthStr != "" {
				parts = append(parts, lengthStr)
			}
		}
		if result.Author != "" {
			parts = append(parts, result.Author)
		}
		if len(parts) > 0 {
			fmt.Printf("     %s\n", dim.Sprint(strings.Join(parts, " ")))
		}

	case "map":
		if result.Address != nil {
			printAddress(result.Address, dim)
		}
		if result.Longitude != 0 || result.Latitude != 0 {
			fmt.Printf("     %s\n", dim.Sprintf("%.6f, %.6f", result.Latitude, result.Longitude))
		}

	case "science":
		var parts []string
		if result.PublishedDate != "" {
			if date := parseDate(result.PublishedDate); date != nil {
				parts = append(parts, date.Format("January 2, 2006"))
			}
		}
		if result.Journal != "" {
			parts = append(parts, result.Journal)
		}
		if result.Publisher != "" {
			parts = append(parts, result.Publisher)
		}
		if len(parts) > 0 {
			fmt.Printf("     %s\n", dim.Sprint(strings.Join(parts, " ")))
		}

	case "files":
		if result.Template == "torrent.html" {
			if result.MagnetLink != "" {
				fmt.Printf("     %s\n", dim.Sprint(result.MagnetLink))
			}
			fmt.Printf("     %s ↑%d seeders, ↓%d leechers\n",
				dim.Sprint(result.FileSize), result.Seed, result.Leech)
		} else if result.Template == "files.html" {
			fmt.Printf("     %s %s\n", dim.Sprint(result.Size), dim.Sprint(result.Metadata))
		}

	case "social media":
		if result.PublishedDate != "" {
			if date := parseDate(result.PublishedDate); date != nil {
				fmt.Printf("     %s\n", dim.Sprint(date.Format("January 2, 2006")))
			}
		}
	}
}

func printAddress(address map[string]interface{}, dim *color.Color) {
	var parts []string

	if houseNumber, ok := address["house_number"].(string); ok && houseNumber != "" {
		parts = append(parts, houseNumber)
	}
	if road, ok := address["road"].(string); ok && road != "" {
		parts = append(parts, road)
	}

	if len(parts) > 0 {
		fmt.Printf("     %s\n", strings.Join(parts, " "))
	}

	var cityParts []string
	if locality, ok := address["locality"].(string); ok && locality != "" {
		cityParts = append(cityParts, locality)
	}
	if postcode, ok := address["postcode"].(string); ok && postcode != "" {
		cityParts = append(cityParts, postcode)
	}

	if len(cityParts) > 0 {
		fmt.Printf("     %s\n", strings.Join(cityParts, ", "))
	}

	if country, ok := address["country"].(string); ok && country != "" {
		fmt.Printf("     %s\n", country)
	}
}

func formatLength(length interface{}) string {
	switch v := length.(type) {
	case float64:
		minutes := int(v / 60)
		seconds := int(v) % 60
		return fmt.Sprintf("%02d:%02d", minutes, seconds)
	case string:
		return v
	default:
		return ""
	}
}

func parseDate(dateStr string) *time.Time {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return nil
	}

	for _, layout := range layouts {
		if date, err := time.Parse(layout, dateStr); err == nil {
			return &date
		}
	}

	return nil
}

// getRandomUserAgent returns a random user agent from the pool
func getRandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// setupHTTPClient creates an HTTP client with anti-bot detection features
func setupHTTPClient(config *Config) *http.Client {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	if config.NoVerifySSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	return client
}

// setupHTTPRequest creates an HTTP request with realistic browser headers
func setupHTTPRequest(method, url string, config *Config) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// Use random user agent unless disabled
	if !config.NoUserAgent {
		req.Header.Set("User-Agent", getRandomUserAgent())
	}

	// Add common browser headers to appear more legitimate
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	return req, nil
}

func printHTMLOnly(results []SearchResult, outputFile string, config *Config) error {
	var output io.Writer = os.Stdout

	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		output = file
	}

	client := setupHTTPClient(config)

	for i, result := range results {
		if result.URL == "" {
			continue
		}

		// Add random delay between requests (100-500ms) to appear more human
		if i > 0 {
			delay := time.Duration(100+rand.Intn(400)) * time.Millisecond
			time.Sleep(delay)
		}

		// Print separator and metadata
		if i > 0 {
			fmt.Fprintln(output, "\n"+strings.Repeat("=", 80))
		}
		fmt.Fprintf(output, "<!-- URL: %s -->\n", result.URL)
		fmt.Fprintf(output, "<!-- Title: %s -->\n", result.Title)
		fmt.Fprintln(output)

		// Fetch the page
		req, err := setupHTTPRequest("GET", result.URL, config)
		if err != nil {
			fmt.Fprintf(output, "<!-- Error creating request: %v -->\n", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(output, "<!-- Error fetching page: %v -->\n", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(output, "<!-- HTTP %d error -->\n", resp.StatusCode)
			continue
		}

		// Handle gzip compression
		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				fmt.Fprintf(output, "<!-- Error creating gzip reader: %v -->\n", err)
				continue
			}
			defer reader.Close()
		default:
			reader = resp.Body
		}

		// Read the body
		bodyBytes, err := io.ReadAll(reader)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(output, "<!-- Error reading page: %v -->\n", err)
			continue
		}

		// Output raw HTML
		fmt.Fprintln(output, string(bodyBytes))
	}

	return nil
}

func printEngines(result SearchResult, dim *color.Color) {
	engines := make([]string, len(result.Engines))
	copy(engines, result.Engines)

	// Remove the main engine from the list
	if result.Engine != "" {
		for i, engine := range engines {
			if engine == result.Engine {
				engines = append(engines[:i], engines[i+1:]...)
				break
			}
		}
	}

	engineText := ""
	if result.Engine != "" {
		engineText = result.Engine
		if len(engines) > 0 {
			engineText += ", " + strings.Join(engines, ", ")
		}
	} else if len(engines) > 0 {
		engineText = strings.Join(engines, ", ")
	}

	if engineText != "" {
		fmt.Printf("     %s\n", dim.Sprintf("[%s]", engineText))
	}
}

func cleanSearchResult(result SearchResult) map[string]interface{} {
	cleaned := make(map[string]interface{})

	if result.Title != "" {
		cleaned["title"] = result.Title
	}
	if result.URL != "" {
		cleaned["url"] = result.URL
	}
	if result.Content != "" {
		cleaned["content"] = result.Content
	}
	if result.Engine != "" {
		cleaned["engine"] = result.Engine
	}
	if len(result.Engines) > 0 {
		cleaned["engines"] = result.Engines
	}
	if result.Category != "" {
		cleaned["category"] = result.Category
	}
	if result.Template != "" {
		cleaned["template"] = result.Template
	}
	if result.PublishedDate != "" {
		cleaned["publishedDate"] = result.PublishedDate
	}
	if result.Author != "" {
		cleaned["author"] = result.Author
	}
	if result.Length != nil {
		cleaned["length"] = result.Length
	}
	if result.Source != "" {
		cleaned["source"] = result.Source
	}
	if result.Resolution != "" {
		cleaned["resolution"] = result.Resolution
	}
	if result.ImgSrc != "" {
		cleaned["img_src"] = result.ImgSrc
	}
	if len(result.Address) > 0 {
		cleaned["address"] = result.Address
	}
	if result.Longitude != 0 {
		cleaned["longitude"] = result.Longitude
	}
	if result.Latitude != 0 {
		cleaned["latitude"] = result.Latitude
	}
	if result.Journal != "" {
		cleaned["journal"] = result.Journal
	}
	if result.Publisher != "" {
		cleaned["publisher"] = result.Publisher
	}
	if result.MagnetLink != "" {
		cleaned["magnetlink"] = result.MagnetLink
	}
	if result.Seed != 0 {
		cleaned["seed"] = result.Seed
	}
	if result.Leech != 0 {
		cleaned["leech"] = result.Leech
	}
	if result.FileSize != "" {
		cleaned["filesize"] = result.FileSize
	}
	if result.Size != "" {
		cleaned["size"] = result.Size
	}
	if result.Metadata != "" {
		cleaned["metadata"] = result.Metadata
	}

	return cleaned
}

func printJSONResults(results []SearchResult, query string) error {
	output := map[string]interface{}{
		"query":   query,
		"results": results,
	}
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func printJSONResultsClean(results []SearchResult, query string) error {
	cleanedResults := make([]map[string]interface{}, len(results))
	for i, result := range results {
		cleanedResults[i] = cleanSearchResult(result)
	}

	output := map[string]interface{}{
		"query":   query,
		"results": cleanedResults,
	}
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func printLinksOnly(results []SearchResult, outputFile string) error {
	var output io.Writer = os.Stdout

	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		output = file
	}

	for _, result := range results {
		if result.URL != "" {
			fmt.Fprintln(output, result.URL)
		}
	}

	return nil
}

func printJSONToFile(results []SearchResult, outputFile string, query string, clean bool) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	var output map[string]interface{}

	if clean {
		cleanedResults := make([]map[string]interface{}, len(results))
		for i, result := range results {
			cleanedResults[i] = cleanSearchResult(result)
		}
		output = map[string]interface{}{
			"query":   query,
			"results": cleanedResults,
		}
	} else {
		output = map[string]interface{}{
			"query":   query,
			"results": results,
		}
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonData)
	return err
}

func printResultsToFile(results []SearchResult, count int, startAt int, expand bool, noColor bool, query string, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Redirect stdout temporarily to file
	oldStdout := os.Stdout
	os.Stdout = file

	// Always disable color for file output
	printResults(results, count, startAt, expand, true, query)

	// Restore stdout
	os.Stdout = oldStdout

	return nil
}

func printTextOnly(results []SearchResult, outputFile string, config *Config) error {
	var output io.Writer = os.Stdout

	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		output = file
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	if config.NoVerifySSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	for i, result := range results {
		if i > 0 {
			fmt.Fprintln(output, "\n"+strings.Repeat("=", 80))
		}

		fmt.Fprintf(output, "URL: %s\n", result.URL)
		fmt.Fprintf(output, "Title: %s\n\n", result.Title)

		if result.URL == "" {
			continue
		}

		// Fetch the page
		req, err := http.NewRequest("GET", result.URL, nil)
		if err != nil {
			fmt.Fprintf(output, "Error creating request: %v\n", err)
			continue
		}

		if !config.NoUserAgent {
			req.Header.Set("User-Agent", "sx/1.0")
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(output, "Error fetching page: %v\n", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(output, "HTTP %d error\n", resp.StatusCode)
			continue
		}

		// Parse URL for readability
		parsedURL, err := url.Parse(result.URL)
		if err != nil {
			resp.Body.Close()
			fmt.Fprintf(output, "Error parsing URL: %v\n", err)
			continue
		}

		// Use readability to extract main content
		article, err := readability.FromReader(resp.Body, parsedURL)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(output, "Error extracting content: %v\n", err)
			continue
		}

		// Convert HTML to Markdown
		converter := md.NewConverter("", true, nil)
		markdown, err := converter.ConvertString(article.Content)
		if err != nil {
			fmt.Fprintf(output, "Error converting to markdown: %v\n", err)
			continue
		}

		// Print the article metadata
		if article.Byline != "" {
			fmt.Fprintf(output, "Author: %s\n", article.Byline)
		}
		if article.PublishedTime != nil && !article.PublishedTime.IsZero() {
			fmt.Fprintf(output, "Published: %s\n", article.PublishedTime.Format("2006-01-02"))
		}
		if article.Excerpt != "" {
			fmt.Fprintf(output, "Excerpt: %s\n", article.Excerpt)
		}
		fmt.Fprintln(output)

		fmt.Fprintln(output, markdown)
	}

	return nil
}
