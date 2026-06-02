package diffview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDiffViewUnifiedBasic(t *testing.T) {
	dv := New().
		Before("main.go", "line 1\nline 2\n").
		After("main.go", "line 1\nmodified line\nline 3\n").
		Width(80)

	output := dv.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	plain := ansi.Strip(output)

	// Should contain filename header
	if !strings.Contains(plain, "main.go") {
		t.Errorf("expected output to contain filename")
	}

	// Should contain hunk header
	if !strings.Contains(plain, "@@") {
		t.Errorf("expected output to contain hunk header")
	}

	// Should contain modified line (equal context)
	if !strings.Contains(plain, "line 1") {
		t.Errorf("expected output to contain context line")
	}

	// Should contain the modified line
	if !strings.Contains(plain, "modified line") {
		t.Errorf("expected output to contain modified line")
	}
}

func TestDiffViewUnifiedInsertion(t *testing.T) {
	dv := New().
		Before("test.go", "a\nb\n").
		After("test.go", "a\nb\nc\n").
		Width(80)

	output := dv.String()
	if !strings.Contains(output, "c") {
		t.Errorf("expected output to contain inserted line 'c'")
	}
}

func TestDiffViewUnifiedDeletion(t *testing.T) {
	dv := New().
		Before("test.go", "a\nb\nc\n").
		After("test.go", "a\nb\n").
		Width(80)

	output := dv.String()
	if !strings.Contains(output, "c") {
		t.Errorf("expected output to contain deleted line 'c'")
	}
}

func TestDiffViewNewFile(t *testing.T) {
	dv := New().
		Before("new.go", "").
		After("new.go", "package main\n").
		Width(80)

	output := dv.String()
	if !strings.Contains(output, "package main") {
		t.Errorf("expected output to contain new file content")
	}
}

func TestDiffViewDeletedFile(t *testing.T) {
	dv := New().
		Before("old.go", "package main\n").
		After("old.go", "").
		Width(80)

	output := dv.String()
	if !strings.Contains(output, "package main") {
		t.Errorf("expected output to contain deleted file content")
	}
}

func TestDiffViewSplitLayout(t *testing.T) {
	dv := New().
		Before("main.go", "line 1\nold line\n").
		After("main.go", "line 1\nnew line\n").
		Split().
		Width(120)

	output := dv.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	if !strings.Contains(output, "line 1") {
		t.Errorf("expected output to contain context")
	}
}

func TestDiffViewLineNumbersDisabled(t *testing.T) {
	dv := New().
		Before("main.go", "a\n").
		After("main.go", "a\nb\n").
		LineNumbers(false).
		Width(80)

	output := dv.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestDiffViewHeightClipping(t *testing.T) {
	dv := New().
		Before("main.go", "line 1\nline 2\nline 3\nline 4\nline 5\n").
		After("main.go", "line 1\nline 2\nline 3\nline 4\nline 5\n").
		Width(80).
		Height(3)

	output := dv.String()
	lines := strings.Count(output, "\n") + 1
	if lines > 4 {
		t.Errorf("expected at most 4 lines with height=3, got %d", lines)
	}
}

func TestDiffViewYOffset(t *testing.T) {
	dv := New().
		Before("main.go", "a\nb\nc\nd\ne\nf\n").
		After("main.go", "a\nb\nCHANGED\nd\ne\nf\n").
		Width(80).
		Height(3).
		YOffset(2)

	output := dv.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestDiffViewTabReplacement(t *testing.T) {
	dv := New().
		Before("main.go", "\ta\n").
		After("main.go", "\tb\n").
		Width(80).
		TabWidth(4)

	output := dv.String()
	if strings.Contains(output, "\t") {
		t.Errorf("expected tabs to be replaced with spaces")
	}
}

func TestDiffViewMultipleHunks(t *testing.T) {
	before := "func a() {}\n\nfunc b() {}\n\nfunc c() {}\n"
	after := "func a() { return }\n\nfunc b() {}\n\nfunc c() { return }\n"

	dv := New().
		Before("main.go", before).
		After("main.go", after).
		Width(80)

	output := dv.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Should have two hunk headers (changes at func a and func c)
	count := strings.Count(output, "@@")
	if count < 2 {
		t.Errorf("expected at least 2 hunk headers, got %d", count)
	}
}

func TestDiffViewContextLines(t *testing.T) {
	before := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"
	after := "line 1\nline 2\nline 3\nCHANGED\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"

	dv := New().
		Before("main.go", before).
		After("main.go", after).
		Width(80).
		ContextLines(2)

	output := dv.String()
	if !strings.Contains(output, "CHANGED") {
		t.Errorf("expected output to contain changed line")
	}
}

func TestPad(t *testing.T) {
	if pad(1, 3) != "  1" {
		t.Errorf("expected '  1', got %q", pad(1, 3))
	}
	if pad(123, 3) != "123" {
		t.Errorf("expected '123', got %q", pad(123, 3))
	}
}

func TestTernary(t *testing.T) {
	if ternary(true, "a", "b") != "a" {
		t.Error("expected 'a' for true condition")
	}
	if ternary(false, "a", "b") != "b" {
		t.Error("expected 'b' for false condition")
	}
}

func TestBtoi(t *testing.T) {
	if btoi(true) != 1 {
		t.Error("expected 1 for true")
	}
	if btoi(false) != 0 {
		t.Error("expected 0 for false")
	}
}
