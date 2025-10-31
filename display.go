package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
)

const maxContentWords = 128

type SearchOptions struct {
	Categories []string
	Engines    []string
	SafeSearch string
	Language   string
	TimeRange  string
	Site       string
	PageNo     int
	Expand     bool
	JSON       bool
	First      bool
	Lucky      bool
	NoPrompt   bool
	Unsafe     bool
	LinksOnly  bool
	OutputFile string
	Top        bool
	Clean      bool
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
