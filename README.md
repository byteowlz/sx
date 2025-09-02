# sx

SearXNG from the command line

`sx` is a command-line interface (CLI) tool that allows you to perform web
searches using [SearXNG](https://github.com/searxng/searxng) instances directly
from your terminal. It provides rich-formatted search results with support for
various search categories and advanced filtering options.

This is a Go port of the original Python [searxngr](https://github.com/scross01/searxngr) project.

## Key Features

- **Terminal-based interface** with colorized output
- **Search engines selection** (bing, duckduckgo, google, etc)
- Support for **search categories** (general, news, images, videos, science, etc.)
- **Safe search filtering** (none, moderate, strict)
- **Time-range filtering** (day, week, month, year)
- **JSON output** option for scripting
- **Automatic configuration** with first-time setup using TOML
- **Cross-platform** support (macOS, Linux, Windows)

## Installation

### From Source

```shell
git clone https://github.com/your-repo/sx.git
cd sx
go build -o sx .
```

### Using Go Install

```shell
go install github.com/your-repo/sx@latest
```

## Configuration

The `sx` configuration is stored in `$XDG_CONFIG_HOME/sx/config.toml`,
on Mac and Linux this is typically under `$HOME/.config/sx/config.toml` and on Windows under
`%APPDATA%/sx/config.toml`.

If the config file is not found, it will be created and populated with a
configuration template on first search. `sx` will prompt for your SearXNG
instance URL to populate the configuration file.

### Example config.toml

```toml
# sx configuration file
searxng_url = "https://searxng.example.com"
result_count = 10
safe_search = "strict"
expand = false
http_method = "GET"
timeout = 30.0
no_verify_ssl = false
no_user_agent = false
no_color = false
debug = false

# Optional configuration
# searxng_username = "username"
# searxng_password = "password"
# categories = ["general", "news"]
# engines = ["duckduckgo", "google", "brave"]
# language = "en"
# url_handler = "open"  # macOS default, "xdg-open" for Linux, "explorer" for Windows
```

### Configuration options

- `searxng_url` - set the URL of your SearXNG instance.
- `searxng_username` - username for basic auth. Optional
- `searxng_password` - password for basic auth. Optional
- `result_count` - the number results to output per page on the terminal. Default is `10`.
- `categories` - the categories to use for the search. Options include `news`, `videos`, `images`, `music`, `map`, `science`, `it`, `files`, `social+media`. Uses `general` search if not set.
- `safe_search` - set the safe search level to `none`, `moderate`, or `strict`. Uses `strict` if not set.
- `engines` - use the specified engines for the search. Uses server default if not set.
- `expand` - show the result URL in the results list. Default is `false`.
- `language` - set the search language, e.g. `en`, `en-CA`, `fr`, `es`, `de`, etc.
- `http_method` - use either `GET` or `POST` requests to the SearXNG API. Default is `GET`
- `timeout` - Timeout in seconds. Default is `30.0`.
- `no_verify_ssl` - disable SSL verification if you are hosting SearXNG with self-signed certificates. Default is `false`.
- `no_user_agent` - Clear the user agent. Default is `false`.
- `no_color` - disable color terminal output. Default is `false`.
- `debug` - show debug output. Default is `false`.

## Usage

```shell
sx why is the sky blue
```

### Options

Command line options can be used to modify the output and override the
configuration defaults.

```txt
sx is a command-line interface for SearXNG search instances, inspired by ddgr and googler.

Usage:
  sx [query...] [flags]

Flags:
  -c, --categories strings   list of categories to search in: general, news, videos, images, music, map, science, it, files, social+media
      --debug                show debug output
  -e, --engines strings      list of engines to use for search
  -x, --expand               show complete URL in search results
  -F, --files                show results from files section
  -j, --first                open the first result in web browser and exit
  -h, --help                 help for sx
      --http-method string   HTTP method to use for search requests (GET or POST) (default "GET")
      --json                 output search results in JSON format
  -l, --language string      search results in a specific language
      --lucky                opens a random result in web browser and exit
  -M, --music                show results from music section
  -N, --news                 show results from news section
      --no-verify-ssl        do not verify SSL certificates
      --nocolor              disable colored output
      --noua                 disable user agent
      --np                   just search and exit, do not prompt
  -n, --num int              show N results per page (default 10)
      --safe-search string   filter results for safe search (none, moderate, strict) (default "strict")
      --searxng-url string   SearXNG instance URL
  -w, --site string          search sites using site: operator
  -S, --social               show results from social media section
  -t, --time-range string    search results within a specific time range (day, week, month, year)
      --timeout float        HTTP request timeout in seconds (default 30)
      --unsafe               allow unsafe search results
  -v, --version              version for sx
  -V, --videos               show results from videos section
```

### Interactive Commands

After displaying search results, `sx` enters an interactive mode where you can:

- Enter a search query to perform a new search
- Type `n`, `p`, and `f` to navigate to the next, previous and first page of results
- Type the index (1, 2, 3, etc) to open the search result in a browser
- Type `c` plus the index (`c 1`, `c 2`) to show the result URL
- Type `t timerange` to change the search time range (e.g. `t week`)
- Type `site:example.com` to filter results by a specific site
- Type `x` to toggle showing result URLs
- Type `d` to toggle debug output
- Type `j` plus the index (`j 1`, `j 2`) to show the JSON result for the specified index
- Type `q`, `quit`, or `exit` to exit the program
- Type `?` for help

## Troubleshooting

**Error: HTTP 429 Too Many Requests**

The SearXNG server is limiting access to the search API. Update server limiter
setting or disable limiter for private instances in the service
`searxng/settings.toml`

**Error: failed to parse JSON response**

The SearXNG instance may be returning the results in html format. On the SearXNG
servers you need to modify the supported search formats to include json in
`searxng/settings.yml`.

```yaml
search:
  formats:
    - html
    - json
```

## Building from Source

Requirements:

- Go 1.19 or later

```shell
git clone https://github.com/your-repo/sx.git
cd sx
go mod tidy
go build -o sx .
```

## Dependencies

- [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parser
- [github.com/spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [github.com/fatih/color](https://github.com/fatih/color) - Terminal colors

