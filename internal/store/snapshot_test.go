package store

import (
	"testing"
)

func TestSaveAndLoadPageSnapshot(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	worktreeID := "/repo/feature-a"
	snapshot := PageSnapshot{
		BodyTreeJSON: []byte(`{"paneID":"shell-main"}`),
		Focused:      "shell-main",
		OpenEditors:  []string{"/repo/feature-a/main.go"},
	}
	data, err := MarshalPageSnapshot(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := s.SavePageSnapshot(worktreeID, data); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadPageSnapshot(worktreeID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	restored, err := UnmarshalPageSnapshot(loaded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(restored.BodyTreeJSON) != string(snapshot.BodyTreeJSON) {
		t.Fatalf("body tree mismatch")
	}
	if restored.Focused != snapshot.Focused {
		t.Fatalf("focused mismatch")
	}
	if len(restored.OpenEditors) != 1 || restored.OpenEditors[0] != "/repo/feature-a/main.go" {
		t.Fatalf("open editors mismatch")
	}
}

func TestListPageSnapshots(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	_ = s.SavePageSnapshot("/repo/a", []byte(`{}`))
	_ = s.SavePageSnapshot("/repo/b", []byte(`{}`))

	records, err := s.ListPageSnapshots()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestDeletePageSnapshot(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	worktreeID := "/repo/feature-a"
	_ = s.SavePageSnapshot(worktreeID, []byte(`{}`))
	_ = s.DeletePageSnapshot(worktreeID)

	loaded, err := s.LoadPageSnapshot(worktreeID)
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil after delete")
	}
}
