package adapters

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func repoRoot() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "..", "..")
}

func BenchmarkGitLocalGetStatus(b *testing.B) {
	adapter := NewGitLocalAdapter()
	repo := repoRoot()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.GetStatus(repo)
		if err != nil {
			b.Fatalf("get status: %v", err)
		}
	}
}

func BenchmarkGitLocalListWorktrees(b *testing.B) {
	adapter := NewGitLocalAdapter()
	repo := repoRoot()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.ListWorktrees(repo)
		if err != nil {
			b.Fatalf("list worktrees: %v", err)
		}
	}
}

func BenchmarkGitLocalGetBranches(b *testing.B) {
	adapter := NewGitLocalAdapter()
	repo := repoRoot()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.GetBranches(repo)
		if err != nil {
			b.Fatalf("get branches: %v", err)
		}
	}
}

func BenchmarkGitLocalStatusConcurrent(b *testing.B) {
	adapter := NewGitLocalAdapter()
	repo := repoRoot()
	const concurrency = 20

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for c := 0; c < concurrency; c++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := adapter.GetStatus(repo)
				if err != nil {
					b.Logf("get status: %v", err)
				}
			}()
		}
		wg.Wait()
	}
	total := concurrency * b.N
	b.ReportMetric(float64(total)/b.Elapsed().Seconds(), "reqs/sec")
}
