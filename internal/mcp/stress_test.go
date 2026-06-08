package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestServerHTTPStressConcurrentResources verifies concurrent resource reads.
func TestServerHTTPStressConcurrentResources(t *testing.T) {
	skipMCPHTTPInShortMode(t)
	s := NewServer("", "127.0.0.1:0")

	var counter atomic.Int64
	_ = s.RegisterResource("context://counter", "Counter", "Counting resource", "application/json", func(uri string) (ResourceContent, error) {
		v := counter.Add(1)
		return ResourceContent{URI: uri, MimeType: "application/json", Text: fmt.Sprintf(`{"count":%d}`, v)}, nil
	})

	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	const numReqs = 200
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numReqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqBody := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "resources/read",
				"params": map[string]any{
					"uri": "context://counter",
				},
			}
			body, _ := json.Marshal(reqBody)
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Errorf("request failed: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	took := time.Since(start)

	if got := int(counter.Load()); got != numReqs {
		t.Fatalf("expected %d resource reads, got %d", numReqs, got)
	}

	t.Logf("MCP resource stress: %d concurrent reads in %v (%.0f reqs/sec)",
		numReqs, took, float64(numReqs)/took.Seconds())
}

// TestServerHTTPStressToolsAndResourcesMixed interleaves tool calls and resource reads.
func TestServerHTTPStressToolsAndResourcesMixed(t *testing.T) {
	skipMCPHTTPInShortMode(t)
	s := NewServer("", "127.0.0.1:0")

	var toolCalls atomic.Int64
	_ = s.RegisterTool("stress.echo", "Echo for stress", nil, func(params map[string]any) (map[string]any, error) {
		toolCalls.Add(1)
		return map[string]any{"input": params["input"]}, nil
	})

	_ = s.RegisterResource("context://static", "Static", "Static resource", "text/plain", func(uri string) (ResourceContent, error) {
		return ResourceContent{URI: uri, MimeType: "text/plain", Text: "hello"}, nil
	})

	url, err := s.StartHTTP()
	if err != nil {
		t.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	const numReqs = 100
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numReqs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var reqBody map[string]any
			if id%2 == 0 {
				reqBody = map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"method":  "tools/call",
					"params": map[string]any{
						"name":      "stress_echo",
						"arguments": map[string]any{"input": id},
					},
				}
			} else {
				reqBody = map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"method":  "resources/read",
					"params": map[string]any{
						"uri": "context://static",
					},
				}
			}
			body, _ := json.Marshal(reqBody)
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Errorf("request %d failed: %v", id, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("request %d expected 200, got %d", id, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	if got := toolCalls.Load(); got != int64(numReqs/2) {
		t.Fatalf("expected %d tool calls, got %d", numReqs/2, got)
	}

	t.Logf("MCP mixed stress: %d concurrent requests in %v (%.0f reqs/sec)",
		numReqs, took, float64(numReqs)/took.Seconds())
}
