package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestDeleteWorktree_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     DeleteWorktree
		wantErr string
	}{
		{"missing path", DeleteWorktree{}, "worktree path required"},
		{"valid", DeleteWorktree{WorktreePath: "/repo/wt1"}, ""},
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

func TestDeleteWorktree_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed worktree context
	if err := s.SaveWorktreeContext(models.WorktreeContextRecord{WorktreeID: "/repo/wt1", RepoID: "/repo"}); err != nil {
		t.Fatalf("seed worktree: %v", err)
	}

	cmd := &DeleteWorktree{WorktreePath: "/repo/wt1"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	wc, err := s.GetWorktreeContext("/repo/wt1")
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}
	if wc != nil {
		t.Fatal("expected worktree to be deleted")
	}
}
