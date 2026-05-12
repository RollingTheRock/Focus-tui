package main

import (
	"path/filepath"
	"testing"
)

func TestResolveDBPathUsesProjectLocalByDefault(t *testing.T) {
	projectRoot := "/tmp/project"
	got, err := resolveDBPath(projectRoot)
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}
	want := filepath.Join(projectRoot, ".focus", "focus.db")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveDBPathUsesEnvOverride(t *testing.T) {
	t.Setenv("FOCUS_DB_PATH", "/tmp/custom.db")
	got, err := resolveDBPath("/tmp/project")
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}
	if got != "/tmp/custom.db" {
		t.Fatalf("expected env override, got %q", got)
	}
}

