package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func benchmarkBatchInsert(b *testing.B, count int) {
	s, err := New(":memory:")
	if err != nil {
		b.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < count; j++ {
			record := models.TaskContextRecord{
				ID:     fmt.Sprintf("task-%d-%d", i, j),
				RepoID: "repo-1",
				Title:  fmt.Sprintf("Task %d", j),
				State:  "active",
			}
			if err := s.SaveTaskContext(record); err != nil {
				b.Fatalf("save task: %v", err)
			}
		}
	}
	b.ReportMetric(float64(count*b.N)/b.Elapsed().Seconds(), "tasks/sec")
}

func BenchmarkBatchInsert1K(b *testing.B)   { benchmarkBatchInsert(b, 1000) }
func BenchmarkBatchInsert10K(b *testing.B)  { benchmarkBatchInsert(b, 10000) }
func BenchmarkBatchInsert100K(b *testing.B) { benchmarkBatchInsert(b, 100000) }

func benchmarkConcurrentInsert(b *testing.B, numGoroutines, tasksPerGoroutine int) {
	s, err := New(":memory:")
	if err != nil {
		b.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		start := time.Now()
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(gid int) {
				defer wg.Done()
				for t := 0; t < tasksPerGoroutine; t++ {
					record := models.TaskContextRecord{
						ID:     fmt.Sprintf("g%d-%d-%d", gid, i, t),
						RepoID: "repo-1",
						Title:  fmt.Sprintf("Task %d", t),
						State:  "active",
					}
					if err := s.SaveTaskContext(record); err != nil {
						b.Logf("save task: %v", err)
					}
				}
			}(g)
		}
		wg.Wait()
		_ = time.Since(start)
	}
	total := numGoroutines * tasksPerGoroutine * b.N
	b.ReportMetric(float64(total)/b.Elapsed().Seconds(), "tasks/sec")
}

func BenchmarkConcurrentInsert10x100(b *testing.B)    { benchmarkConcurrentInsert(b, 10, 100) }
func BenchmarkConcurrentInsert50x100(b *testing.B)    { benchmarkConcurrentInsert(b, 50, 100) }
func BenchmarkConcurrentInsert100x1000(b *testing.B)  { benchmarkConcurrentInsert(b, 100, 1000) }
