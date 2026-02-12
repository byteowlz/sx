package backends

import (
	"fmt"
	"time"
)

// SearchResult represents a single search result
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

// SearchOptions contains parameters for a search query
type SearchOptions struct {
	Query      string
	Categories []string
	Engines    []string
	Language   string
	TimeRange  string
	Site       string
	SafeSearch string
	PageNo     int
	NumResults int
}

// BackendConfig contains engine-specific configuration
type BackendConfig struct {
	APIKey       string
	Timeout      time.Duration
	ExtraHeaders map[string]string
	// Engine-specific options
	SearchDepth       string // for Tavily: basic/advanced
	IncludeRawContent bool   // for Tavily
}

// SearchBackend is the interface that all search backends must implement
type SearchBackend interface {
	// Name returns the unique identifier for this backend
	Name() string

	// Search performs a search query and returns results
	Search(opts SearchOptions) ([]SearchResult, error)

	// IsAvailable checks if the backend is properly configured and reachable
	IsAvailable() bool
}

// BackendError represents an error from a specific backend
type BackendError struct {
	Backend string
	Err     error
	Code    int // HTTP status code or custom error code
}

func (e *BackendError) Error() string {
	return fmt.Sprintf("%s backend: %v", e.Backend, e.Err)
}

// Unwrap returns the underlying error
func (e *BackendError) Unwrap() error {
	return e.Err
}

// Error codes for backend failures
const (
	ErrCodeUnavailable = iota // Backend not configured
	ErrCodeNetwork            // Network/connectivity issue
	ErrCodeAuth               // Authentication failure
	ErrCodeRateLimit          // Rate limited
	ErrCodeInvalidResponse    // Invalid/malformed response
)
