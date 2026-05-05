package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerHTTPStartsAndResponds(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	if err := s.RegisterTool("task.ping", "Ping the task system", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	if url == "" {
		t.Fatal("expected non-empty http url")
	}
	if !s.HTTPRunning() {
		t.Fatal("expected http running")
	}

	// Health check.
	resp, err := http.Get(strings.Replace(url, "/mcp", "/health", 1))
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestServerHTTPInitialize(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": ProtocolVersion,
			"clientInfo":      map[string]any{"name": "test-client", "version": "1.0"},
		},
	}
	result := postJSONRPC(t, url, reqBody)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("expected protocol version %q, got %v", ProtocolVersion, result["protocolVersion"])
	}
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "focus-tui" {
		t.Fatalf("unexpected server info: %v", serverInfo)
	}
}

func TestServerHTTPToolsList(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	_ = s.RegisterTool("task.create", "Create a task", map[string]any{
		"type":       "object",
		"properties": map[string]any{"title": map[string]any{"type": "string"}},
	}, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"id": "task-1"}, nil
	})
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	result := postJSONRPC(t, url, reqBody)
	list, _ := result["tools"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(list))
	}
	tool, _ := list[0].(map[string]any)
	if tool["name"] != "task_create" {
		t.Fatalf("unexpected tool: %v", tool)
	}
}

func TestServerHTTPToolsCall(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	_ = s.RegisterTool("task.echo", "Echo a task", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{
			"task_id": params["task_id"],
			"ok":      true,
		}, nil
	})
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "task.echo",
			"arguments": map[string]any{
				"task_id": "task-123",
			},
		},
	}
	result := postJSONRPC(t, url, reqBody)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("expected content, got none")
	}
	textItem, _ := content[0].(map[string]any)
	if textItem["type"] != "text" {
		t.Fatalf("expected text content, got %v", textItem)
	}
	if !strings.Contains(textItem["text"].(string), "task-123") {
		t.Fatalf("expected result to contain task-123, got %v", textItem["text"])
	}
}

func TestServerHTTPUnknownMethod(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "foo.bar",
	}
	resp := postRaw(t, url, reqBody)
	if resp.Error == nil {
		t.Fatalf("expected error for unknown method")
	}
	if resp.Error.Code != ErrMethodNotFound {
		t.Fatalf("expected method not found, got %d", resp.Error.Code)
	}
}

func TestServerHTTPMethodNotAllowed(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServerUnixSocketLifecycle(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "focus-mcp.sock")
	s := NewServer(socket, "")
	if s.Running() {
		t.Fatal("expected server stopped by default")
	}
	if err := s.RegisterTool("task.ping", "Ping", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if err := s.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	if !s.Running() {
		t.Fatal("expected running")
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if s.Running() {
		t.Fatal("expected stopped")
	}
}

func TestServerStopIsIdempotent(t *testing.T) {
	s := NewServer("", "")
	if err := s.Stop(); err != nil {
		t.Fatalf("stop on fresh server: %v", err)
	}
}

// postJSONRPC sends a JSON-RPC request and returns the result payload.
func postJSONRPC(t *testing.T, url string, reqBody map[string]any) map[string]any {
	t.Helper()
	resp := postRaw(t, url, reqBody)
	if resp.Error != nil {
		t.Fatalf("jsonrpc error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", resp.Result)
	}
	return result
}

func postRaw(t *testing.T, url string, reqBody map[string]any) JSONRPCResponse {
	t.Helper()
	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(b))
	}
	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rpcResp
}

func unixSocketNotPermitted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "operation not permitted")
}

// TestServerHTTPConcurrentCalls verifies the HTTP server handles concurrent requests safely.
func TestServerHTTPConcurrentCalls(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	_ = s.RegisterTool("task.counter", "Counter", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"count": 1}, nil
	})
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			reqBody := map[string]any{
				"jsonrpc": "2.0",
				"id":      i,
				"method":  "tools/call",
				"params": map[string]any{
					"name":      "task.counter",
					"arguments": map[string]any{},
				},
			}
			result := postJSONRPC(t, url, reqBody)
			content, _ := result["content"].([]any)
			if len(content) == 0 {
				t.Error("expected content")
			}
		}()
	}
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}
}

// TestServerHTTPPing verifies the ping method.
func TestServerHTTPPing(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "ping",
	}
	result := postJSONRPC(t, url, reqBody)
	if result == nil {
		t.Fatal("expected empty result object")
	}
}

// TestServerRegisterToolDuplicateOverwrites verifies duplicate registration updates the tool.
func TestServerRegisterToolDuplicateOverwrites(t *testing.T) {
	s := NewServer("", "127.0.0.1:0")
	_ = s.RegisterTool("task.x", "First", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"v": 1}, nil
	})
	_ = s.RegisterTool("task.x", "Second", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"v": 2}, nil
	})

	if s.ToolCount() != 1 {
		t.Fatalf("expected 1 tool, got %d", s.ToolCount())
	}

	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "task.x",
			"arguments": map[string]any{},
		},
	}
	result := postJSONRPC(t, url, reqBody)
	content, _ := result["content"].([]any)
	textItem, _ := content[0].(map[string]any)
	if !strings.Contains(textItem["text"].(string), `"v": 2`) {
		t.Fatalf("expected overwritten handler result, got %s", textItem["text"])
	}
}
