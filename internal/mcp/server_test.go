package mcp

import "testing"

func TestServerLifecycleAndToolCall(t *testing.T) {
	s := NewServer("/tmp/focus-mcp.sock")
	if s.Running() {
		t.Fatal("expected server stopped by default")
	}
	if err := s.RegisterTool("task.ping", func(params map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if err := s.Start(); err != nil {
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
