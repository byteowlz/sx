package backends

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MCPHTTPClient is a minimal JSON-RPC over HTTP MCP client.
type MCPHTTPClient struct {
	BaseURL string
	client  *http.Client
}

func NewMCPHTTPClient(baseURL string, timeout time.Duration) *MCPHTTPClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MCPHTTPClient{
		BaseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

func (c *MCPHTTPClient) call(method string, id int, params interface{}) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp mcpRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid MCP JSON-RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (c *MCPHTTPClient) Initialize() error {
	_, err := c.call("initialize", 1, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "sx",
			"version": "2.2.0",
		},
	})
	return err
}

func (c *MCPHTTPClient) CallTool(toolName string, args map[string]interface{}) (json.RawMessage, error) {
	result, err := c.call("tools/call", 2, map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
