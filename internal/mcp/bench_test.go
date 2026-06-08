package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

func BenchmarkHTTPServerConcurrent(b *testing.B) {
	s := NewServer("", "127.0.0.1:0")

	_ = s.RegisterTool("bench.echo", "Echo benchmark", nil, func(params map[string]any) (map[string]any, error) {
		return map[string]any{"input": params["input"]}, nil
	})
	_ = s.RegisterResource("context://bench", "Bench", "Benchmark resource", "text/plain", func(uri string) (ResourceContent, error) {
		return ResourceContent{URI: uri, MimeType: "text/plain", Text: "hello"}, nil
	})

	url, err := s.StartHTTP()
	if err != nil {
		b.Fatalf("start http: %v", err)
	}
	defer s.Stop()

	const concurrency = 50

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		start := time.Now()
		for c := 0; c < concurrency; c++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				reqBody := map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"method":  "tools/call",
					"params": map[string]any{
						"name":      "bench.echo",
						"arguments": map[string]any{"input": id},
					},
				}
				body, _ := json.Marshal(reqBody)
				resp, err := http.Post(url, "application/json", bytes.NewReader(body))
				if err != nil {
					b.Logf("request failed: %v", err)
					return
				}
				resp.Body.Close()
				if resp.StatusCode != 200 {
					b.Logf("unexpected status: %d", resp.StatusCode)
				}
			}(c)
		}
		wg.Wait()
		_ = time.Since(start)
	}
	total := concurrency * b.N
	b.ReportMetric(float64(total)/b.Elapsed().Seconds(), "reqs/sec")
}
