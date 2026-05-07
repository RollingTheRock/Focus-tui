package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAllStandardADR(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0000-test.md", `# ADR-0000: Test Architecture Decision

- **Date:** 2026-05-08
- **Status:** Accepted

## Context

This is the context text.
It spans multiple lines.

## Decision

We decided to do the thing.

## Consequences

Things will be better.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.ID != "ADR-0000" {
		t.Errorf("ID = %q, want ADR-0000", r.ID)
	}
	if r.Title != "Test Architecture Decision" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.Status != "accepted" {
		t.Errorf("Status = %q, want accepted", r.Status)
	}
	if r.Date != "2026-05-08" {
		t.Errorf("Date = %q, want 2026-05-08", r.Date)
	}
	if r.Version != 1 {
		t.Errorf("Version = %d, want 1", r.Version)
	}
	if !strings.Contains(r.Context, "This is the context text.") {
		t.Errorf("Context = %q", r.Context)
	}
	if r.Decision != "We decided to do the thing." {
		t.Errorf("Decision = %q", r.Decision)
	}
	if !strings.Contains(r.Consequences, "Things will be better.") {
		t.Errorf("Consequences = %q", r.Consequences)
	}
}

func TestLoadAllSkipsNonADR(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# ADR Index\n\nList of ADRs.")
	writeFile(t, dir, "notes.md", "# Some Notes\n\nContent.")
	writeFile(t, dir, "0005-implementation-plan.md", `# ADR-0005 Implementation Plan

This is not a real ADR.
`)
	// File without ADR title pattern in H1
	writeFile(t, dir, "0099-weird.md", `# Just Some Document

- **Date:** 2026-01-01
- **Status:** Proposed

## Context
Not an ADR.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d: %+v", len(records), records)
	}
}

func TestLoadAllMultipleADRs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0002-beta.md", `# ADR-0002: Beta Decision

- **Date:** 2026-05-01
- **Status:** Proposed

## Context
Beta context.

## Decision
Beta decision.

## Consequences
Beta consequences.
`)
	writeFile(t, dir, "0001-alpha.md", `# ADR-0001: Alpha Decision

- **Date:** 2026-04-01
- **Status:** Accepted

## Context
Alpha context.

## Decision
Alpha decision.

## Consequences
Alpha consequences.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// Must be sorted by ID
	if records[0].ID != "ADR-0001" {
		t.Errorf("first = %q, want ADR-0001", records[0].ID)
	}
	if records[1].ID != "ADR-0002" {
		t.Errorf("second = %q, want ADR-0002", records[1].ID)
	}
}

func TestLoadAllSupersedes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0004-new.md", `# ADR-0004: New Decision

- **Date:** 2026-05-08
- **Status:** Accepted
- **Supersedes:** ADR-0002, ADR-0003

## Context
Replaces old decisions.

## Decision
New approach.

## Consequences
Cleaner.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	r := records[0]
	if r.SupersededBy == nil {
		t.Fatal("SupersededBy is nil")
	}
	if *r.SupersededBy != "ADR-0002" {
		t.Errorf("SupersededBy = %q, want ADR-0002", *r.SupersededBy)
	}
}

func TestLoadAllADR0003StyleStatus(t *testing.T) {
	dir := t.TempDir()
	// ADR-0003 uses ## Status h2 instead of metadata list
	writeFile(t, dir, "0003-style.md", `# ADR-0003: Alt Format Decision

## Status
Proposed — 2026-04-20 (v2)

## Context
This ADR uses a different metadata format.

## Decision
We handle both.

## Consequences
Flexible.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	r := records[0]
	if r.Status != "proposed" {
		t.Errorf("Status = %q, want proposed", r.Status)
	}
	if r.Date != "2026-04-20" {
		t.Errorf("Date = %q, want 2026-04-20", r.Date)
	}
}

func TestLoadAllWithThematicBreak(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0005-break.md", `# ADR-0005: With Separator

- **Date:** 2026-05-06
- **Status:** Proposed

---

## Context

Context after a thematic break.

## Decision

Decision text.

## Consequences

Consequence text.
`)

	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	r := records[0]
	if r.Context != "Context after a thematic break." {
		t.Errorf("Context = %q", r.Context)
	}
}

func TestLoadAllEmptyDir(t *testing.T) {
	dir := t.TempDir()
	records, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestLoadAllMissingDir(t *testing.T) {
	_, err := LoadAll("/nonexistent/path/adr")
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestLoadConstraints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0000-rules.md", `# ADR-0000: Rules

- **Date:** 2026-05-08
- **Status:** Accepted

## Context
Some context.

## Decision
Some decision.

## Constraints

- **must**: Always validate input -- Security requirement
- **should**: Use async processing -- Performance guideline
- **must_not**: Never hardcode credentials -- Security policy

## Consequences
Some consequences.
`)

	records, err := LoadConstraints(dir, "ADR-0000")
	if err != nil {
		t.Fatalf("LoadConstraints: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 constraints, got %d", len(records))
	}

	if records[0].Category != "must" {
		t.Errorf("c[0].Category = %q", records[0].Category)
	}
	if records[0].Rule != "Always validate input" {
		t.Errorf("c[0].Rule = %q", records[0].Rule)
	}
	if records[0].Rationale != "Security requirement" {
		t.Errorf("c[0].Rationale = %q", records[0].Rationale)
	}

	if records[1].Category != "should" {
		t.Errorf("c[1].Category = %q", records[1].Category)
	}

	if records[2].Category != "must_not" {
		t.Errorf("c[2].Category = %q", records[2].Category)
	}
}

func TestLoadConstraintsNonexistent(t *testing.T) {
	dir := t.TempDir()
	records, err := LoadConstraints(dir, "ADR-9999")
	if err != nil {
		t.Fatalf("LoadConstraints: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil for nonexistent ADR, got %v", records)
	}
}

func TestLoadConstraintsNoSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0001-no-constraints.md", `# ADR-0001: No Constraints

- **Date:** 2026-05-08
- **Status:** Accepted

## Context
Context text.

## Decision
Decision text.

## Consequences
Consequences text.
`)

	records, err := LoadConstraints(dir, "ADR-0001")
	if err != nil {
		t.Fatalf("LoadConstraints: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 constraints, got %d", len(records))
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}
