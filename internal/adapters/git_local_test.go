package adapters

import (
	"testing"

	gitmodel "focus/internal/git"
)

func TestParsePorcelainV2StagedOnlyUsesDotAsUnmodified(t *testing.T) {
	adapter := NewGitLocalAdapter()
	status := &gitmodel.Status{}
	output := []byte("1 M. N... 100644 100644 100644 df967b96a579e45a18b8251732d16804b2e56a55 5ea2ed416fbd4a4cbe227b75fe255dd7fa6bd4d6 a.txt\x00")

	if err := adapter.parsePorcelainV2(output, status); err != nil {
		t.Fatalf("parse porcelain v2: %v", err)
	}

	if got := len(status.StagedFiles); got != 1 {
		t.Fatalf("expected 1 staged file, got %d", got)
	}
	if got := len(status.UnstagedFiles); got != 0 {
		t.Fatalf("expected 0 unstaged files for M., got %d", got)
	}
	if status.StagedFiles[0].Path != "a.txt" {
		t.Fatalf("expected staged file a.txt, got %q", status.StagedFiles[0].Path)
	}
}

func TestParsePorcelainV2UnstagedOnlyUsesDotAsUnmodified(t *testing.T) {
	adapter := NewGitLocalAdapter()
	status := &gitmodel.Status{}
	output := []byte("1 .M N... 100644 100644 100644 df967b96a579e45a18b8251732d16804b2e56a55 df967b96a579e45a18b8251732d16804b2e56a55 a.txt\x00")

	if err := adapter.parsePorcelainV2(output, status); err != nil {
		t.Fatalf("parse porcelain v2: %v", err)
	}

	if got := len(status.StagedFiles); got != 0 {
		t.Fatalf("expected 0 staged files for .M, got %d", got)
	}
	if got := len(status.UnstagedFiles); got != 1 {
		t.Fatalf("expected 1 unstaged file, got %d", got)
	}
	if status.UnstagedFiles[0].Path != "a.txt" {
		t.Fatalf("expected unstaged file a.txt, got %q", status.UnstagedFiles[0].Path)
	}
}

func TestParsePorcelainV2MixedStateKeepsBothSides(t *testing.T) {
	adapter := NewGitLocalAdapter()
	status := &gitmodel.Status{}
	output := []byte("1 MM N... 100644 100644 100644 5626abf0f72e58d7a153368ba57db4c673c0e171 814f4a422927b82f5f8a43f8fab6d3839e3983f2 a.txt\x00")

	if err := adapter.parsePorcelainV2(output, status); err != nil {
		t.Fatalf("parse porcelain v2: %v", err)
	}

	if got := len(status.StagedFiles); got != 1 {
		t.Fatalf("expected 1 staged file for MM, got %d", got)
	}
	if got := len(status.UnstagedFiles); got != 1 {
		t.Fatalf("expected 1 unstaged file for MM, got %d", got)
	}
	if status.StagedFiles[0].Path != "a.txt" || status.UnstagedFiles[0].Path != "a.txt" {
		t.Fatalf("expected mixed-state file to stay aligned, got staged=%q unstaged=%q", status.StagedFiles[0].Path, status.UnstagedFiles[0].Path)
	}
}
