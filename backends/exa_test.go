package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExaBackend_API(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("missing key"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]string{
				{"title": "Exa A", "url": "https://exa.example/a", "text": "A content"},
			},
		})
	}))
	defer server.Close()

	b := NewExaBackend(ExaModeAPI, "test-key", 2*time.Second, "", "", 10)
	b.BaseURL = server.URL

	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Exa A" {
		t.Fatalf("unexpected title: %s", results[0].Title)
	}
}

func TestExaBackend_MCP(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)

		if method == "initialize" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
				},
			})
			return
		}
		if method == "tools/call" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"structuredContent": map[string]interface{}{
						"results": []map[string]string{{
							"title": "Exa MCP",
							"url":   "https://exa.example/mcp",
							"text":  "from mcp",
						}},
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	b := NewExaBackend(ExaModeMCP, "", 2*time.Second, server.URL, "exa-web-search", 10)
	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Exa MCP" {
		t.Fatalf("unexpected result title: %s", results[0].Title)
	}
	if callCount < 1 {
		t.Fatal("expected at least one MCP call")
	}
}

func TestExaBackend_AutoFallsBackToMCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		if method == "initialize" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req["id"], "result": map[string]interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]string{{
					"type": "text",
					"text": "- [Fallback](https://exa.example/fallback)",
				}},
			},
		})
	}))
	defer server.Close()

	b := NewExaBackend(ExaModeAuto, "", 2*time.Second, server.URL, "exa-web-search", 10)
	results, err := b.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].URL != "https://exa.example/fallback" {
		t.Fatalf("unexpected fallback results: %#v", results)
	}
}
