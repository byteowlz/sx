package main

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

type SearchResult struct {
	Title         string                 `json:"title"`
	URL           string                 `json:"url"`
	Content       string                 `json:"content"`
	Engine        string                 `json:"engine"`
	Engines       []string               `json:"engines"`
	Category      string                 `json:"category"`
	Template      string                 `json:"template"`
	PublishedDate string                 `json:"publishedDate"`
	Author        string                 `json:"author"`
	Length        interface{}            `json:"length"`
	Source        string                 `json:"source"`
	Resolution    string                 `json:"resolution"`
	ImgSrc        string                 `json:"img_src"`
	Address       map[string]interface{} `json:"address"`
	Longitude     float64                `json:"longitude"`
	Latitude      float64                `json:"latitude"`
	Journal       string                 `json:"journal"`
	Publisher     string                 `json:"publisher"`
	MagnetLink    string                 `json:"magnetlink"`
	Seed          int                    `json:"seed"`
	Leech         int                    `json:"leech"`
	FileSize      string                 `json:"filesize"`
	Size          string                 `json:"size"`
	Metadata      string                 `json:"metadata"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

var safeSearchOptions = map[string]int{
	"none":     0,
	"moderate": 1,
	"strict":   2,
}

var timeRangeOptions = []string{"day", "week", "month", "year"}
var timeRangeShortOptions = []string{"d", "w", "m", "y"}

var searxngCategories = []string{
	"general", "news", "videos", "images", "music",
	"map", "science", "it", "files", "social+media",
}

func performSearch(query string, config *Config, searchOpts *SearchOptions) ([]SearchResult, error) {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	if config.NoVerifySSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	var searchURL string
	var requestBody io.Reader

	if searchOpts.Site != "" {
		query = fmt.Sprintf("site:%s %s", searchOpts.Site, query)
	}

	if strings.ToUpper(config.HTTPMethod) == "POST" {
		searchURL = fmt.Sprintf("%s/search", config.SearxngURL)

		data := url.Values{}
		data.Set("q", query)
		data.Set("format", "json")

		if searchOpts.Categories != nil && len(searchOpts.Categories) > 0 {
			categories := make([]string, len(searchOpts.Categories))
			copy(categories, searchOpts.Categories)
			// Replace social+media with social media for POST requests
			for i, cat := range categories {
				if cat == "social+media" {
					categories[i] = "social media"
				}
			}
			data.Set("categories", strings.Join(categories, ","))
		}

		if searchOpts.Engines != nil && len(searchOpts.Engines) > 0 {
			data.Set("engines", strings.Join(searchOpts.Engines, ","))
		}

		if searchOpts.Language != "" {
			data.Set("language", searchOpts.Language)
		}

		if searchOpts.PageNo > 1 {
			data.Set("pageno", strconv.Itoa(searchOpts.PageNo))
		}

		if searchOpts.SafeSearch != "" {
			if val, ok := safeSearchOptions[searchOpts.SafeSearch]; ok {
				data.Set("safesearch", strconv.Itoa(val))
			}
		}

		if searchOpts.TimeRange != "" {
			data.Set("time_range", searchOpts.TimeRange)
		}

		requestBody = strings.NewReader(data.Encode())
	} else {
		// GET request
		u, err := url.Parse(config.SearxngURL + "/search")
		if err != nil {
			return nil, fmt.Errorf("invalid SearXNG URL: %v", err)
		}

		params := url.Values{}
		params.Set("q", query)
		params.Set("format", "json")

		if searchOpts.Categories != nil && len(searchOpts.Categories) > 0 {
			params.Set("categories", strings.Join(searchOpts.Categories, ","))
		}

		if searchOpts.Engines != nil && len(searchOpts.Engines) > 0 {
			params.Set("engines", strings.Join(searchOpts.Engines, ","))
		}

		if searchOpts.Language != "" {
			params.Set("language", searchOpts.Language)
		}

		if searchOpts.SafeSearch != "" {
			if val, ok := safeSearchOptions[searchOpts.SafeSearch]; ok {
				params.Set("safesearch", strconv.Itoa(val))
			}
		}

		if searchOpts.TimeRange != "" {
			params.Set("time_range", searchOpts.TimeRange)
		}

		if searchOpts.PageNo > 1 {
			params.Set("pageno", strconv.Itoa(searchOpts.PageNo))
		}

		u.RawQuery = params.Encode()
		searchURL = u.String()
	}

	var req *http.Request
	var err error

	if strings.ToUpper(config.HTTPMethod) == "POST" {
		req, err = http.NewRequest("POST", searchURL, requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest("GET", searchURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	if !config.NoUserAgent {
		req.Header.Set("User-Agent", "sx/1.0")
	}

	if config.SearxngUsername != "" && config.SearxngPassword != "" {
		req.SetBasicAuth(config.SearxngUsername, config.SearxngPassword)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return searchResp.Results, nil
}

func validateCategory(category string) bool {
	for _, cat := range searxngCategories {
		if cat == category {
			return true
		}
	}
	return false
}

func validateTimeRange(timeRange string) bool {
	for _, tr := range timeRangeOptions {
		if tr == timeRange {
			return true
		}
	}
	for _, tr := range timeRangeShortOptions {
		if tr == timeRange {
			return true
		}
	}
	return false
}

func expandTimeRange(timeRange string) string {
	switch timeRange {
	case "d":
		return "day"
	case "w":
		return "week"
	case "m":
		return "month"
	case "y":
		return "year"
	default:
		return timeRange
	}
}
