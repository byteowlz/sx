package backends

import (
	"os"
	"testing"
	"time"
)

// Manual live test for Exa MCP server.
// Run with:
//
//	SX_E2E_EXA_MCP=1 SX_EXA_MCP_URL=<mcp-http-url> go test ./backends -run TestExaMCP_Live
func TestExaMCP_Live(t *testing.T) {
	if os.Getenv("SX_E2E_EXA_MCP") != "1" {
		t.Skip("set SX_E2E_EXA_MCP=1 to enable live Exa MCP test")
	}
	mcpURL := os.Getenv("SX_EXA_MCP_URL")
	if mcpURL == "" {
		t.Skip("set SX_EXA_MCP_URL to the Exa MCP HTTP endpoint")
	}

	backend := NewExaBackend(ExaModeMCP, "", 20*time.Second, mcpURL, "exa-web-search", 5)
	results, err := backend.Search(SearchOptions{Query: "golang http client", NumResults: 5})
	if err != nil {
		t.Fatalf("live Exa MCP search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("live Exa MCP search returned no results")
	}
}
