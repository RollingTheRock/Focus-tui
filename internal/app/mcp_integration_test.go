package app

import (
	"net/http"
	"testing"

	"focus/internal/config"
	"focus/internal/store"
)

func TestMCPHTTPServerStartsWithTUI(t *testing.T) {
	if testing.Short() {
		t.Skip("MCP HTTP integration test skipped in short mode")
	}
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = "127.0.0.1:18767" // dedicated test port to avoid conflicts with other tests
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	if m.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}
	if !m.mcpServer.HTTPRunning() {
		t.Fatal("expected MCP HTTP server to be running")
	}

	url := m.mcpServer.HTTPURL()
	if url == "" {
		t.Fatal("expected non-empty MCP HTTP URL")
	}
	t.Logf("MCP HTTP URL: %s", url)

	// Verify the endpoint is actually reachable.
	resp, err := http.Get(url + "/../health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
