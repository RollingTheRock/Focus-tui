package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestUpdateWorktreeContext_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     UpdateWorktreeContext
		wantErr string
	}{
		{"missing worktree id", UpdateWorktreeContext{Record: models.WorktreeContextRecord{RepoID: "r"}}, "worktree id required"},
		{"missing repo id", UpdateWorktreeContext{Record: models.WorktreeContextRecord{WorktreeID: "w"}}, "repo id required"},
		{"valid", UpdateWorktreeContext{Record: models.WorktreeContextRecord{WorktreeID: "w", RepoID: "r"}}, ""},
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

func TestUpdateWorktreeContext_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &UpdateWorktreeContext{
		Record: models.WorktreeContextRecord{
			WorktreeID: "/repo/wt1",
			RepoID:     "/repo",
			TaskMode:   "single",
			TaskName:   "Feature A",
		},
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	wc, err := s.GetWorktreeContext("/repo/wt1")
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}
	if wc == nil {
		t.Fatal("worktree not found")
	}
	if wc.TaskName != "Feature A" {
		t.Fatalf("expected task_name %q, got %q", "Feature A", wc.TaskName)
	}
}
