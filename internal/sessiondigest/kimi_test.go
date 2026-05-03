package sessiondigest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEncodeKimiWorktreePath(t *testing.T) {
	path := "/home/rollingtherock/dev/Focus-tui"
	got := encodeKimiWorktreePath(path)
	want := "98475a1639e1a1456e2a301b23c9905c"
	if got != want {
		t.Errorf("encodeKimiWorktreePath(%q) = %q, want %q", path, got, want)
	}
}

func TestExtractKimiAssistantText(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{
			name: "plain string content",
			raw:  map[string]any{"content": "hello world"},
			want: "hello world",
		},
		{
			name: "array with text part",
			raw: map[string]any{
				"content": []any{
					map[string]any{"type": "think", "think": "reasoning..."},
					map[string]any{"type": "text", "text": "visible text"},
				},
			},
			want: "visible text",
		},
		{
			name: "array with multiple text parts",
			raw: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "part one "},
					map[string]any{"type": "think", "think": "..."},
					map[string]any{"type": "text", "text": "part two"},
				},
			},
			want: "part one part two",
		},
		{
			name: "empty",
			raw:  map[string]any{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKimiAssistantText(tt.raw)
			if got != tt.want {
				t.Errorf("extractKimiAssistantText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKimiReaderParseJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "context.jsonl")

	content := `{"role":"_system_prompt","content":"system"}
{"role":"_checkpoint","id":0}
{"role":"user","content":"hello"}
{"role":"_usage","token_count":100}
{"role":"assistant","content":[{"type":"think","think":"..."},{"type":"text","text":"hi there"}]}
{"role":"user","content":"bye"}
{"role":"assistant","content":"goodbye"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	reader := NewKimiReader()
	events, err := reader.parseJSONL(jsonlPath)
	if err != nil {
		t.Fatalf("parseJSONL error: %v", err)
	}

	// Expect 4 events: 2 user + 2 assistant (skip system, checkpoint, usage).
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Verify event types and content.
	if events[0].Type != EventUser || events[0].Content != "hello" {
		t.Errorf("event[0] = %v, want user/hello", events[0])
	}
	if events[1].Type != EventAssistant || events[1].Content != "hi there" {
		t.Errorf("event[1] = %v, want assistant/hi there", events[1])
	}
	if events[2].Type != EventUser || events[2].Content != "bye" {
		t.Errorf("event[2] = %v, want user/bye", events[2])
	}
	if events[3].Type != EventAssistant || events[3].Content != "goodbye" {
		t.Errorf("event[3] = %v, want assistant/goodbye", events[3])
	}
}

func TestKimiReaderFindSessionFile(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()

	// Create two session directories.
	oldSession := filepath.Join(tmpDir, "session-old")
	newSession := filepath.Join(tmpDir, "session-new")
	os.MkdirAll(oldSession, 0755)
	os.MkdirAll(newSession, 0755)

	// Create context.jsonl files.
	os.WriteFile(filepath.Join(oldSession, "context.jsonl"), []byte(`{"role":"user","content":"old"}`), 0644)
	os.WriteFile(filepath.Join(newSession, "context.jsonl"), []byte(`{"role":"user","content":"new"}`), 0644)

	// Adjust mtimes so newSession is more recent.
	os.Chtimes(oldSession, now.Add(-time.Hour), now.Add(-time.Hour))
	os.Chtimes(newSession, now.Add(-time.Minute), now.Add(-time.Minute))

	reader := NewKimiReader()
	got, err := reader.findSessionFile(tmpDir, now.Add(-2*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("findSessionFile error: %v", err)
	}

	want := filepath.Join(newSession, "context.jsonl")
	if got != want {
		t.Errorf("findSessionFile() = %q, want %q", got, want)
	}
}
