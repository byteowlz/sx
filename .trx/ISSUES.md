# Issues

## Open

### [trx-gdr2] Add pluggable search backend architecture (P1, epic)
Refactor sx to support pluggable search backends via a common interface. Create SearchBackend interface with Search() method. Implement SearxngBackend (existing), BraveBackend, TavilyBackend. Add config: engine=searxng, fallback_engines=[brave,tavily]. Auto-retry with fallback on connection errors. Each backend maps its response format to sx's SearchResult struct.

### [trx-73nn] Add Tavily Search API backend (P2, epic)
Add Tavily Search API as a fallback or explicit engine option. Tavily is optimized for LLMs and returns URLs plus optional extracted content inline. 1000 free credits per month. Endpoint: POST https://api.tavily.com/search with Bearer token. Unique feature: include_raw_content=true returns page content directly, reducing need for separate scrpr call.

### [trx-60bd] Add Brave Search API backend (P2, epic)
Add Brave Search API as a fallback search engine. Brave provides a clean REST API with 2000 free requests per month at 1 req/sec. Endpoint: GET https://api.search.brave.com/res/v1/web/search with X-Subscription-Token header. Returns JSON with title, URL, description. Good fallback when SearXNG is unreachable.

### [trx-zq4d] Document sx+scrpr integration patterns (P3, epic)
Document recommended usage patterns and pipelines combining sx and scrpr. Include: 1) Basic search-to-content pipeline with examples, 2) When to use sx --text vs sx | scrpr, 3) Rate limiting best practices, 4) Choosing extraction backends (local vs Tavily vs Jina), 5) Search backend selection (SearXNG vs Brave vs Tavily). Add to README or create PIPELINES.md.

## Closed

- [trx-pmq3] Add default output format to config (closed 2026-02-12)
- [trx-7r7h] Add query history feature (closed 2026-02-12)
- [trx-gqz0] Add shell completion support (closed 2026-02-12)
- [trx-8csx] Standardize category naming convention (closed 2026-02-12)
- [trx-evw1] Remove redundant --link flag alias (closed 2026-02-12)
- [trx-jjx4] Rename -t flag from time-range to -r (closed 2026-02-12)
- [trx-ngye] Change default behavior to non-interactive mode (closed 2026-02-12)
- [trx-fkdr] Fix version flag to display version instead of searching (closed 2026-02-12)
