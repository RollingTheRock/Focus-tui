package commands

import (
	"context"
	"testing"
)

func TestRecordWorktreeHistory_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     RecordWorktreeHistory
		wantErr string
	}{
		{"missing id", RecordWorktreeHistory{RepoID: "r"}, "history id required"},
		{"missing repo", RecordWorktreeHistory{ID: "1"}, "repo id required"},
		{"valid", RecordWorktreeHistory{ID: "1", RepoID: "r"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRecordWorktreeHistory_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &RecordWorktreeHistory{
		ID:      "hst-1",
		RepoID:  "/repo",
		Branch:  "main",
		Path:    "/repo/wt1",
		Summary: "done",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	history, err := s.ListWorktreeHistory("/repo")
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	if history[0].Summary != "done" {
		t.Fatalf("expected summary %q, got %q", "done", history[0].Summary)
	}
}
