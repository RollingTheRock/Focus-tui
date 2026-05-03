package sessiondigest

import (
	"testing"
)

func TestParseDiffStat(t *testing.T) {
	input := ` internal/app/app.go | 15 +++++++++++---
 go.mod              |  2 +-
 README.md           | 10 ++++++++++
 binary.bin          | Bin 12345 -> 67890 bytes
 3 files changed, 24 insertions(+), 3 deletions(-)`

	changes := parseDiffStat(input)
	if len(changes) != 4 {
		t.Fatalf("expected 4 file changes, got %d", len(changes))
	}

	// app.go: modified, +11 / -3 (11 plus, 3 minus in the visual bar)
	fc := changes[0]
	if fc.Path != "internal/app/app.go" {
		t.Errorf("expected path 'internal/app/app.go', got %q", fc.Path)
	}
	if fc.Status != "modified" {
		t.Errorf("expected status 'modified', got %q", fc.Status)
	}
	if fc.Additions != 11 {
		t.Errorf("expected 11 additions, got %d", fc.Additions)
	}
	if fc.Deletions != 3 {
		t.Errorf("expected 3 deletions, got %d", fc.Deletions)
	}

	// go.mod: modified, +1 / -1
	fc = changes[1]
	if fc.Path != "go.mod" {
		t.Errorf("expected path 'go.mod', got %q", fc.Path)
	}
	if fc.Additions != 1 || fc.Deletions != 1 {
		t.Errorf("expected +1/-1 for go.mod, got +%d/-%d", fc.Additions, fc.Deletions)
	}

	// README.md: added, +10 / -0
	fc = changes[2]
	if fc.Path != "README.md" {
		t.Errorf("expected path 'README.md', got %q", fc.Path)
	}
	if fc.Status != "added" {
		t.Errorf("expected status 'added', got %q", fc.Status)
	}
	if fc.Additions != 10 || fc.Deletions != 0 {
		t.Errorf("expected +10/-0 for README.md, got +%d/-%d", fc.Additions, fc.Deletions)
	}

	// binary.bin: binary
	fc = changes[3]
	if fc.Path != "binary.bin" {
		t.Errorf("expected path 'binary.bin', got %q", fc.Path)
	}
	if fc.Status != "binary" {
		t.Errorf("expected status 'binary', got %q", fc.Status)
	}
}

func TestParseDiffStatEmpty(t *testing.T) {
	changes := parseDiffStat("")
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes for empty input, got %d", len(changes))
	}
}

func TestCountDiffLines(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
 
 func main() {
-	fmt.Println("old")
+	fmt.Println("new")
 }
`
	ins, del := CountDiffLines(diff)
	if ins != 1 {
		t.Errorf("expected 1 insertion, got %d", ins)
	}
	if del != 1 {
		t.Errorf("expected 1 deletion, got %d", del)
	}
}

func TestDigestToMarkdown(t *testing.T) {
	d := &Digest{
		SessionProvider: "claude",
		WorktreePath:    "/tmp/test",
		Summary:         "Refactored auth middleware.",
		FilesChanged: []FileChange{
			{Path: "auth.go", Status: "modified", Additions: 10, Deletions: 5},
		},
		CommandsRun: []CommandRun{
			{Command: "go test ./...", Output: "ok"},
		},
		KeyDecisions: []string{"Use JWT instead of sessions"},
		GitDiffStat:  "1 file changed, 10 insertions(+), 5 deletions(-)",
	}

	md := d.ToMarkdown()
	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if !contains(md, "Refactored auth middleware.") {
		t.Error("markdown missing summary")
	}
	if !contains(md, "auth.go") {
		t.Error("markdown missing file change")
	}
	if !contains(md, "go test") {
		t.Error("markdown missing command")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
