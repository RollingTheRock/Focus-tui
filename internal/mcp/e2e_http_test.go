package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMCPHTTPFullFlow(t *testing.T) {
	srv := NewServer("", "127.0.0.1:0")
	_ = srv.RegisterTool("task.ping", "Ping the system", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true, "ts": time.Now().Unix()}, nil
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("unix start: %v", err)
	}
	url, err := srv.StartHTTP()
	if err != nil {
		t.Fatalf("http start: %v", err)
	}
	defer srv.Stop()

	if url == "" {
		t.Fatal("expected non-empty url")
	}
	t.Logf("MCP HTTP URL: %s", url)

	// initialize
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]any{"name": "test", "version": "1.0"},
		},
	})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initRes JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&initRes)
	resp.Body.Close()
	if initRes.Error != nil {
		t.Fatalf("initialize error: %v", initRes.Error)
	}
	resMap, _ := initRes.Result.(map[string]any)
	if resMap["protocolVersion"] != ProtocolVersion {
		t.Fatalf("unexpected protocol version: %v", resMap["protocolVersion"])
	}

	// tools/list
	body, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2,
		"method": "tools/list",
	})
	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var listRes JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&listRes)
	resp.Body.Close()
	if listRes.Error != nil {
		t.Fatalf("tools/list error: %v", listRes.Error)
	}
	listMap, _ := listRes.Result.(map[string]any)
	tools, _ := listMap["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	// tools/call
	body, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 3,
		"method": "tools/call",
		"params": map[string]any{
			"name": "task.ping",
			"arguments": map[string]any{},
		},
	})
	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	var callRes JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&callRes)
	resp.Body.Close()
	if callRes.Error != nil {
		t.Fatalf("tools/call error: %v", callRes.Error)
	}
	callMap, _ := callRes.Result.(map[string]any)
	content, _ := callMap["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in tool result")
	}
	textItem, _ := content[0].(map[string]any)
	if textItem["type"] != "text" {
		t.Fatalf("expected text content, got %v", textItem["type"])
	}
	t.Logf("Tool result: %s", textItem["text"])

	fmt.Println("\n✓ MCP HTTP server full flow verified:")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  initialize: ok\n")
	fmt.Printf("  tools/list: %d tools\n", len(tools))
	fmt.Printf("  tools/call: ok\n")
}
