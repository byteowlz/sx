# sx

Multi-engine web search from the command line

`sx` is a CLI tool for searching the web from your terminal. It supports
multiple search backends -- [SearXNG](https://github.com/searxng/searxng)
(self-hosted), [Brave Search](https://api.search.brave.com/) and
[Tavily](https://tavily.com/) -- with automatic fallback when the primary engine
is unreachable.

This is a Go port of the original Python [searxngr](https://github.com/scross01/searxngr) project, extended with multi-engine support.

## Key Features

- **Multiple search backends** - SearXNG, Brave Search, Tavily with automatic fallback
- **Terminal-based interface** with colorized output
- **Non-interactive by default** for scripting; `-i` for interactive mode
- **Search engine selection** (bing, duckduckgo, google, etc. via SearXNG)
- Support for **search categories** (general, news, images, videos, science, etc.)
- **Safe search filtering** (none, moderate, strict)
- **Time-range filtering** (day, week, month, year)
- **JSON output** for scripting
- **Built-in content extraction** - fetch and convert results to clean markdown
- **Anti-bot detection** - rotating user agents, realistic headers, random delays
- **Query history** - searchable history with `sx history`
- **Shell completions** - bash, zsh, fish, powershell
- **Cross-platform** (macOS, Linux, Windows)

## Installation

```shell
go install github.com/your-repo/sx@latest
```

Or build from source:

```shell
git clone https://github.com/your-repo/sx.git
cd sx
go build -o sx .
```

## Configuration

Config is stored at `$XDG_CONFIG_HOME/sx/config.toml` (typically `~/.config/sx/config.toml`).
Created automatically on first run.

### Example config.toml

```toml
# sx configuration file

# Primary search engine (searxng, brave, tavily)
engine = "searxng"

# Fallback engines tried in order if primary fails
fallback_engines = ["brave", "tavily"]

# SearXNG instance settings
searxng_url = "https://searxng.example.com"
# searxng_username = ""
# searxng_password = ""

# General settings
result_count = 10
safe_search = "strict"
http_method = "GET"
timeout = 30.0
expand = false
no_verify_ssl = false
no_user_agent = false
no_color = false
debug = false

# Output defaults
# default_output = ""       # "interactive" to default to interactive mode
history_enabled = true
max_history = 100

# Brave Search API (https://api.search.brave.com/)
# Free tier: 2,000 requests/month
[engines_brave]
api_key = ""  # or set BRAVE_API_KEY env var

# Tavily Search API (https://tavily.com/)
# Free tier: 1,000 credits/month
[engines_tavily]
api_key = ""                  # or set TAVILY_API_KEY env var
search_depth = "basic"        # basic (1 credit) or advanced (2 credits)
include_raw_content = false   # return full page content with results
include_answer = false        # return a direct answer
```

### API Keys via Environment Variables

```shell
export BRAVE_API_KEY="your-brave-key"
export TAVILY_API_KEY="tvly-your-tavily-key"
```

## Usage

### Basic Search

```shell
sx "why is the sky blue"
sx "golang tutorials" -n 5
```

### Select Search Engine

```shell
# Use a specific backend
sx "query" --engine brave
sx "query" --engine tavily

# Default: uses primary engine with automatic fallback
sx "query"
```

### Output Links for Piping

```shell
# Get URLs only (one per line)
sx "golang testing" -L -n 5

# Pipe to other tools
sx "rust tutorials" -L -n 3 | xargs open
```

### Fetch and Convert Pages to Markdown

```shell
# Top result as markdown
sx "golang channels tutorial" --text --top

# Multiple results saved to file
sx "rust ownership" --text -n 3 -o results.md
```

### Pipelines with scrpr

`sx` pairs with [scrpr](https://github.com/byteowlz/scrpr) for content extraction:

```shell
# Search + extract content
sx "query" -L -n 5 | scrpr --format markdown

# Save to directory
sx "query" -L -n 5 | scrpr --format markdown -o articles/

# Use Jina Reader for JS-heavy sites
sx "query" -L -n 5 | scrpr -B jina --format markdown

# With rate limiting
sx "query" -L -n 10 | scrpr --delay 0.5 --continue-on-error
```

### Other Options

```shell
# Categories
sx "query" -N              # news
sx "query" -V              # videos
sx "query" -S              # social media
sx "query" -F              # files

# Filtering
sx "query" -r week         # time range: day, week, month, year
sx "query" -w example.com  # site-specific search
sx "query" --safe-search none

# Output formats
sx "query" --json          # JSON output
sx "query" --json -c       # Clean JSON (no null fields)
sx "query" -H              # Raw HTML with anti-bot headers

# Interactive mode
sx "query" -i

# History
sx history
sx history clear
sx history -n 50

# Shell completions
sx completion bash
sx completion zsh
```

### All Flags

```
Flags:
      --categories strings   search categories (general, news, videos, images, music, etc.)
      --clean                omit empty/null values in JSON output
      --debug                show debug output
  -e, --engines strings      SearXNG engines to use
      --engine string        search backend (searxng, brave, tavily)
  -x, --expand               show full URLs in results
  -F, --files                files category shortcut
  -j, --first                open first result in browser
  -h, --help                 help for sx
  -H, --html                 fetch raw HTML with anti-bot headers
      --http-method string   GET or POST for SearXNG (default "GET")
  -i, --interactive          enter interactive mode after results
      --json                 JSON output
  -l, --language string      search language
  -L, --links-only           output URLs only, one per line
      --lucky                open random result in browser
  -M, --music                music category shortcut
  -N, --news                 news category shortcut
      --no-verify-ssl        skip SSL verification
      --nocolor              disable colors
      --noua                 disable user agent
  -n, --num int              results per page (default 10)
  -o, --output string        save output to file
      --safe-search string   none, moderate, strict (default "strict")
      --searxng-url string   SearXNG instance URL
  -w, --site string          search within a specific site
  -S, --social               social media category shortcut
  -T, --text                 fetch pages and convert to markdown
  -r, --time-range string    day, week, month, year
      --timeout float        request timeout in seconds (default 30)
      --top                  show only top result
      --unsafe               disable safe search
  -v, --version              version
  -V, --videos               videos category shortcut
```

## Search Backend Comparison

| Backend | Auth | Free Tier | Best For |
|---------|------|-----------|----------|
| **SearXNG** | None (self-hosted) | Unlimited | Privacy, full control |
| **Brave** | API key | 2,000 req/month | Fallback, quick setup |
| **Tavily** | API key | 1,000 credits/month | LLM workflows, rich content |

## Troubleshooting

**Error: all backends failed**
Check your primary engine URL and API keys. Use `--debug` for details.

**Error: HTTP 429 Too Many Requests**
SearXNG rate limiting. Update server limiter settings or use a fallback engine.

**Error: failed to parse JSON response**
Enable JSON format in SearXNG's `settings.yml`:
```yaml
search:
  formats:
    - html
    - json
```

## Dependencies

- [cobra](https://github.com/spf13/cobra) - CLI framework
- [toml](https://github.com/BurntSushi/toml) - Configuration
- [color](https://github.com/fatih/color) - Terminal colors
- [go-readability](https://github.com/go-shiori/go-readability) - Content extraction
- [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) - HTML to Markdown
