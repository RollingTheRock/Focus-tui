package mcp

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerLifecycleAndToolCall(t *testing.T) {
	s := NewServer(filepath.Join(t.TempDir(), "focus-mcp.sock"))
	if s.Running() {
		t.Fatal("expected server stopped by default")
	}
	if err := s.RegisterTool("task.ping", func(params map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if err := s.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	if !s.Running() {
		t.Fatal("expected server running")
	}
	out, err := s.CallTool("task.ping", nil)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected tool output: %+v", out)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestServerHandlesJSONRPCOverUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "focus-mcp.sock")
	s := NewServer(socket)
	if err := s.RegisterTool("task.echo", func(params map[string]any) (map[string]any, error) {
		return map[string]any{
			"task_id": params["task_id"],
			"ok":      true,
		}, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if err := s.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	defer s.Stop()

	conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("dial socket: %v", err)
	}
	defer conn.Close()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  "task.echo",
		"params": map[string]any{
			"task_id": "task-123",
		},
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      any            `json:"id"`
		Result  map[string]any `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %#v", resp)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}
	if resp.Result["task_id"] != "task-123" || resp.Result["ok"] != true {
		t.Fatalf("unexpected result payload: %+v", resp.Result)
	}
}

func TestServerReturnsRPCErrorForUnknownTool(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "focus-mcp.sock")
	s := NewServer(socket)
	if err := s.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	defer s.Stop()

	conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("dial socket: %v", err)
	}
	defer conn.Close()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-2",
		"method":  "task.not_found",
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp map[string]any
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] == nil {
		t.Fatalf("expected rpc error for unknown tool, got %+v", resp)
	}
}

func unixSocketNotPermitted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "operation not permitted")
}
