package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

var (
	config     *Config
	searchOpts SearchOptions
)

func main() {
	var err error
	config, err = loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var rootCmd = &cobra.Command{
		Use:     "sx [query...]",
		Short:   "SearXNG from the command line",
		Long:    "sx is a command-line interface for SearXNG search instances, inspired by ddgr and googler.",
		Version: version,
		Run:     runSearch,
	}

	// Add flags
	rootCmd.Flags().StringVar(&config.SearxngURL, "searxng-url", config.SearxngURL, "SearXNG instance URL")
	rootCmd.Flags().StringSliceVarP(&searchOpts.Categories, "categories", "c", nil, fmt.Sprintf("list of categories to search in: %s", strings.Join(searxngCategories, ", ")))
	rootCmd.Flags().BoolVar(&searchOpts.JSON, "json", false, "output search results in JSON format")
	rootCmd.Flags().StringSliceVarP(&searchOpts.Engines, "engines", "e", nil, "list of engines to use for search")
	rootCmd.Flags().BoolVarP(&searchOpts.Expand, "expand", "x", config.Expand, "show complete URL in search results")
	rootCmd.Flags().BoolVarP(&searchOpts.First, "first", "j", false, "open the first result in web browser and exit")
	rootCmd.Flags().StringVar(&config.HTTPMethod, "http-method", config.HTTPMethod, "HTTP method to use for search requests (GET or POST)")
	rootCmd.Flags().Float64Var(&config.Timeout, "timeout", config.Timeout, "HTTP request timeout in seconds")
	rootCmd.Flags().StringVarP(&searchOpts.Language, "language", "l", "", "search results in a specific language")
	rootCmd.Flags().BoolVar(&searchOpts.Lucky, "lucky", false, "opens a random result in web browser and exit")
	rootCmd.Flags().BoolVar(&config.NoVerifySSL, "no-verify-ssl", config.NoVerifySSL, "do not verify SSL certificates")
	rootCmd.Flags().BoolVar(&config.NoColor, "nocolor", config.NoColor, "disable colored output")
	rootCmd.Flags().BoolVar(&searchOpts.NoPrompt, "np", false, "just search and exit, do not prompt")
	rootCmd.Flags().BoolVar(&config.NoUserAgent, "noua", config.NoUserAgent, "disable user agent")
	rootCmd.Flags().IntVarP(&config.ResultCount, "num", "n", config.ResultCount, "show N results per page")
	rootCmd.Flags().StringVar(&searchOpts.SafeSearch, "safe-search", config.SafeSearch, "filter results for safe search (none, moderate, strict)")
	rootCmd.Flags().StringVarP(&searchOpts.Site, "site", "w", "", "search sites using site: operator")
	rootCmd.Flags().StringVarP(&searchOpts.TimeRange, "time-range", "t", "", "search results within a specific time range (day, week, month, year)")
	rootCmd.Flags().BoolVar(&searchOpts.Unsafe, "unsafe", false, "allow unsafe search results")
	rootCmd.Flags().BoolVar(&config.Debug, "debug", config.Debug, "show debug output")

	// Category shortcuts
	var files, music, news, social, videos bool
	rootCmd.Flags().BoolVarP(&files, "files", "F", false, "show results from files section")
	rootCmd.Flags().BoolVarP(&music, "music", "M", false, "show results from music section")
	rootCmd.Flags().BoolVarP(&news, "news", "N", false, "show results from news section")
	rootCmd.Flags().BoolVarP(&social, "social", "S", false, "show results from social media section")
	rootCmd.Flags().BoolVarP(&videos, "videos", "V", false, "show results from videos section")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSearch(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		cmd.Help()
		return
	}

	// Ensure config file exists for actual searches
	if err := ensureConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
		return
	}

	query := strings.Join(args, " ")

	// Handle category shortcuts
	if files, _ := cmd.Flags().GetBool("files"); files {
		searchOpts.Categories = []string{"files"}
	}
	if music, _ := cmd.Flags().GetBool("music"); music {
		searchOpts.Categories = []string{"music"}
	}
	if news, _ := cmd.Flags().GetBool("news"); news {
		searchOpts.Categories = []string{"news"}
	}
	if social, _ := cmd.Flags().GetBool("social"); social {
		searchOpts.Categories = []string{"social+media"}
	}
	if videos, _ := cmd.Flags().GetBool("videos"); videos {
		searchOpts.Categories = []string{"videos"}
	}

	// Handle unsafe flag
	if searchOpts.Unsafe {
		searchOpts.SafeSearch = "none"
	}

	// Validate config
	if config.SearxngURL == "" {
		fmt.Fprintf(os.Stderr, "Error: searxng_url is not set in config\n")
		return
	}

	// Validate categories
	for _, category := range searchOpts.Categories {
		if !validateCategory(category) {
			fmt.Fprintf(os.Stderr, "Error: Invalid category '%s'. Supported categories are: %s\n",
				category, strings.Join(searxngCategories, ", "))
			return
		}
	}

	// Validate time range
	if searchOpts.TimeRange != "" {
		if !validateTimeRange(searchOpts.TimeRange) {
			fmt.Fprintf(os.Stderr, "Error: Invalid time range '%s'. Use: %s\n",
				searchOpts.TimeRange, strings.Join(timeRangeOptions, ", "))
			return
		}
		searchOpts.TimeRange = expandTimeRange(searchOpts.TimeRange)
	}

	// Set defaults from config
	if searchOpts.SafeSearch == "" {
		searchOpts.SafeSearch = config.SafeSearch
	}

	searchOpts.PageNo = 1
	startAt := 0
	var allResults []SearchResult

	for {
		// Fetch results until we have enough
		for len(allResults) < startAt+config.ResultCount {
			results, err := performSearch(query, config, &searchOpts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
				return
			}

			if len(results) == 0 {
				break
			}

			allResults = append(allResults, results...)
			if config.ResultCount == 0 {
				break
			}
			searchOpts.PageNo++
		}

		if len(allResults) == 0 {
			fmt.Println("No results found or an error occurred during the search.")
			return
		}

		// Handle JSON output
		if searchOpts.JSON {
			if err := printJSONResults(allResults); err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			}
			return
		}

		// Handle first/lucky options
		if searchOpts.First && len(allResults) > 0 {
			if err := openURL(allResults[0].URL); err != nil {
				fmt.Fprintf(os.Stderr, "Error opening URL: %v\n", err)
			}
			return
		}

		if searchOpts.Lucky && len(allResults) > 0 {
			randomResult := allResults[rand.Intn(len(allResults))]
			if err := openURL(randomResult.URL); err != nil {
				fmt.Fprintf(os.Stderr, "Error opening URL: %v\n", err)
			}
			return
		}

		// Display results
		count := config.ResultCount
		if count == 0 {
			count = len(allResults)
		}
		printResults(allResults, count, startAt, searchOpts.Expand, config.NoColor)

		// Exit if no prompt requested
		if searchOpts.NoPrompt {
			return
		}

		// Interactive prompt
		if !handleInteractiveSession(&query, &allResults, &startAt, &searchOpts) {
			return
		}
	}
}

func handleInteractiveSession(query *string, allResults *[]SearchResult, startAt *int, opts *SearchOptions) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("sx (? for help): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return false
		}

		input = strings.TrimSpace(input)

		switch {
		case input == "q" || input == "quit" || input == "exit":
			return false

		case input == "?":
			printHelp()
			continue

		case input == "n": // Next page
			*startAt += config.ResultCount
			if *startAt >= len(*allResults) {
				opts.PageNo++
				return true // Need to fetch more results
			}
			printResults(*allResults, config.ResultCount, *startAt, opts.Expand, config.NoColor)
			continue

		case input == "p": // Previous page
			*startAt -= config.ResultCount
			if *startAt < 0 {
				*startAt = 0
			}
			printResults(*allResults, config.ResultCount, *startAt, opts.Expand, config.NoColor)
			continue

		case input == "f": // First page
			*startAt = 0
			printResults(*allResults, config.ResultCount, *startAt, opts.Expand, config.NoColor)
			continue

		case input == "x": // Toggle expand URLs
			opts.Expand = !opts.Expand
			printResults(*allResults, config.ResultCount, *startAt, opts.Expand, config.NoColor)
			continue

		case input == "d": // Toggle debug
			config.Debug = !config.Debug
			fmt.Printf("Debug mode %s\n", map[bool]string{true: "enabled", false: "disabled"}[config.Debug])
			continue

		case strings.HasPrefix(input, "t "): // Change time range
			timeRange := strings.TrimSpace(input[2:])
			if validateTimeRange(timeRange) {
				opts.TimeRange = expandTimeRange(timeRange)
				*startAt = 0
				opts.PageNo = 1
				*allResults = []SearchResult{}
				return true
			} else {
				fmt.Printf("Invalid time range '%s'. Use: %s\n", timeRange, strings.Join(timeRangeOptions, ", "))
			}
			continue

		case strings.HasPrefix(input, "site:"): // Change site filter
			site := strings.TrimSpace(input[5:])
			opts.Site = site
			*startAt = 0
			opts.PageNo = 1
			*allResults = []SearchResult{}
			return true

		case strings.HasPrefix(input, "c "): // Copy URL
			indexStr := strings.TrimSpace(input[2:])
			if index, err := strconv.Atoi(indexStr); err == nil && index > 0 && index <= len(*allResults) {
				url := (*allResults)[index-1].URL
				fmt.Printf("URL: %s\n", url)
			} else {
				fmt.Println("Invalid index specified.")
			}
			continue

		case strings.HasPrefix(input, "j "): // Show JSON for result
			indexStr := strings.TrimSpace(input[2:])
			if index, err := strconv.Atoi(indexStr); err == nil && index > 0 && index <= len(*allResults) {
				result := (*allResults)[index-1]
				if err := printJSONResults([]SearchResult{result}); err != nil {
					fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				}
			}
			continue

		default:
			// Check if it's a number (open result)
			if index, err := strconv.Atoi(input); err == nil && index > 0 && index <= len(*allResults) {
				url := (*allResults)[index-1].URL
				if err := openURL(url); err != nil {
					fmt.Fprintf(os.Stderr, "Error opening URL: %v\n", err)
				}
				continue
			}

			// Treat as new query
			if input != "" {
				*query = input
				*startAt = 0
				opts.PageNo = 1
				*allResults = []SearchResult{}
				return true
			}
		}
	}
}

func printHelp() {
	help := `
- Enter a search query to perform a new search.
- Type 'n', 'p', and 'f' to navigate to the next, previous and first page of results.
- Type the index (1, 2, 3, etc) to open the search result in a browser.
- Type 'c' plus the index ('c 1', 'c 2') to show the result URL.
- Type 't timerange' to change the search time range (e.g. 't week').
- Type 'site:example.com' to filter results by a specific site.
- Type 'x' to toggle showing result URLs.
- Type 'd' to toggle debug output.
- Type 'j' plus the index ('j 1', 'j 2') to show the JSON result for the specified index.
- Type 'q', 'quit', or 'exit' to exit the program.
- Type '?' for this help message.
`
	fmt.Print(help)
}

func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("explorer", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
