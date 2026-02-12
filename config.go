package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	SearxngURL      string   `toml:"searxng_url"`
	SearxngUsername string   `toml:"searxng_username,omitempty"`
	SearxngPassword string   `toml:"searxng_password,omitempty"`
	ResultCount     int      `toml:"result_count"`
	Categories      []string `toml:"categories,omitempty"`
	SafeSearch      string   `toml:"safe_search"`
	Engines         []string `toml:"engines,omitempty"`
	Expand          bool     `toml:"expand"`
	Language        string   `toml:"language,omitempty"`
	HTTPMethod      string   `toml:"http_method"`
	Timeout         float64  `toml:"timeout"`
	NoVerifySSL     bool     `toml:"no_verify_ssl"`
	NoUserAgent     bool     `toml:"no_user_agent"`
	NoColor         bool     `toml:"no_color"`
	URLHandler      string   `toml:"url_handler,omitempty"`
	Debug           bool     `toml:"debug"`
	DefaultOutput   string   `toml:"default_output,omitempty"`
	HistoryEnabled  bool     `toml:"history_enabled"`
	MaxHistory      int      `toml:"max_history"`
}

const (
	defaultSearxngURL     = "https://searxng.example.com"
	defaultResultCount    = 10
	defaultSafeSearch     = "strict"
	defaultHTTPMethod     = "GET"
	defaultTimeout        = 30.0
	defaultExpand         = false
	defaultNoVerifySSL    = false
	defaultNoUserAgent    = false
	defaultNoColor        = false
	defaultDebug          = false
	defaultDefaultOutput  = ""
	defaultHistoryEnabled = true
	defaultMaxHistory     = 100
)

var defaultURLHandlers = map[string]string{
	"darwin":  "open",
	"linux":   "xdg-open",
	"windows": "explorer",
}

func getConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "sx")
}

func getDefaultConfig() *Config {
	return &Config{
		SearxngURL:     "",
		ResultCount:    defaultResultCount,
		SafeSearch:     defaultSafeSearch,
		Expand:         defaultExpand,
		HTTPMethod:     defaultHTTPMethod,
		Timeout:        defaultTimeout,
		NoVerifySSL:    defaultNoVerifySSL,
		NoUserAgent:    defaultNoUserAgent,
		NoColor:        defaultNoColor,
		Debug:          defaultDebug,
		DefaultOutput:  defaultDefaultOutput,
		HistoryEnabled: defaultHistoryEnabled,
		MaxHistory:     defaultMaxHistory,
	}
}

func loadConfig() (*Config, error) {
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "config.toml")

	config := getDefaultConfig()

	// If config file exists, load it
	if _, err := os.Stat(configFile); err == nil {
		if _, err := toml.DecodeFile(configFile, config); err != nil {
			return nil, fmt.Errorf("failed to load config: %v", err)
		}
	}

	return config, nil
}

func ensureConfig() error {
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "config.toml")

	// If config file doesn't exist, create it
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return createConfigFile(configDir, configFile)
	}

	return nil
}

func createConfigFile(configDir, configFile string) error {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Prompt for SearXNG URL
	fmt.Printf("Enter your SearXNG instance URL [%s]: ", defaultSearxngURL)
	var searxngURL string
	fmt.Scanln(&searxngURL)
	if strings.TrimSpace(searxngURL) == "" {
		searxngURL = defaultSearxngURL
	}

	// Create default config
	config := &Config{
		SearxngURL:  searxngURL,
		ResultCount: defaultResultCount,
		SafeSearch:  defaultSafeSearch,
		Expand:      defaultExpand,
		HTTPMethod:  defaultHTTPMethod,
		Timeout:     defaultTimeout,
		NoVerifySSL: defaultNoVerifySSL,
		NoUserAgent: defaultNoUserAgent,
		NoColor:     defaultNoColor,
		Debug:       defaultDebug,
	}

	// Write config to file
	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header comment
	_, err = file.WriteString("# sx configuration file\n")
	if err != nil {
		return err
	}

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return err
	}

	fmt.Printf("Created config file: %s\n", configFile)
	return nil
}
