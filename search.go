package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"sx/backends"
)

var timeRangeOptions = []string{"day", "week", "month", "year"}
var timeRangeShortOptions = []string{"d", "w", "m", "y"}

var searxngCategories = []string{
	"general", "news", "videos", "images", "music",
	"map", "science", "it", "files", "social media",
}

// categoryAliases maps alternative names to canonical category names
var categoryAliases = map[string]string{
	"social+media": "social media",
	"social-media": "social media",
	"social_media": "social media",
	"socialmedia":  "social media",
}

// initBackendManager creates and configures the backend manager from config
func initBackendManager(config *Config) *backends.Manager {
	mgr := backends.NewManager()

	// Register SearXNG backend
	searxng := backends.NewSearxngBackend(
		config.SearxngURL,
		config.SearxngUsername,
		config.SearxngPassword,
		config.HTTPMethod,
		time.Duration(config.Timeout)*time.Second,
		config.NoVerifySSL,
		config.NoUserAgent,
	)
	mgr.Register(searxng)

	// Register Brave backend
	braveAPIKey := config.EnginesBrave.APIKey
	if envKey := os.Getenv("BRAVE_API_KEY"); envKey != "" {
		braveAPIKey = envKey
	}
	brave := backends.NewBraveBackend(
		braveAPIKey,
		time.Duration(config.Timeout)*time.Second,
	)
	mgr.Register(brave)

	// Register Tavily backend
	tavilyAPIKey := config.EnginesTavily.APIKey
	if envKey := os.Getenv("TAVILY_API_KEY"); envKey != "" {
		tavilyAPIKey = envKey
	}
	searchDepth := config.EnginesTavily.SearchDepth
	if searchDepth == "" {
		searchDepth = "basic"
	}
	tavily := backends.NewTavilyBackend(
		tavilyAPIKey,
		time.Duration(config.Timeout)*time.Second,
		searchDepth,
		config.EnginesTavily.IncludeRawContent,
		config.EnginesTavily.IncludeAnswer,
	)
	mgr.Register(tavily)

	// Set primary engine
	engine := config.Engine
	if engine == "" {
		engine = "searxng"
	}
	if err := mgr.SetPrimary(engine); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v, falling back to searxng\n", err)
		mgr.SetPrimary("searxng")
	}

	// Set fallback engines
	if len(config.FallbackEngines) > 0 {
		if err := mgr.SetFallbacks(config.FallbackEngines); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	return mgr
}

// performSearch executes a search using the backend manager
func performSearch(query string, config *Config, searchOpts *SearchOptions, mgr *backends.Manager, explicitEngine string) ([]backends.SearchResult, string, error) {
	opts := backends.SearchOptions{
		Query:      query,
		Categories: searchOpts.Categories,
		Engines:    searchOpts.SearxngEngines,
		Language:   searchOpts.Language,
		TimeRange:  searchOpts.TimeRange,
		Site:       searchOpts.Site,
		SafeSearch: searchOpts.SafeSearch,
		PageNo:     searchOpts.PageNo,
		NumResults: config.ResultCount,
	}

	// If an explicit engine was requested via --engine flag, use only that
	if explicitEngine != "" {
		results, err := mgr.SearchExplicit(explicitEngine, opts)
		return results, explicitEngine, err
	}

	// Otherwise use primary + fallback chain
	return mgr.Search(opts)
}

func validateCategory(category string) bool {
	for _, cat := range searxngCategories {
		if cat == category {
			return true
		}
	}
	if _, ok := categoryAliases[category]; ok {
		return true
	}
	return false
}

func normalizeCategory(category string) string {
	if canonical, ok := categoryAliases[category]; ok {
		return canonical
	}
	return category
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

// validEngineNames returns all valid engine names for help text
func validEngineNames() string {
	return strings.Join([]string{"searxng", "brave", "tavily"}, ", ")
}
