package sessiondigest

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GitDiffCollector gathers git diff information from a worktree.
type GitDiffCollector struct{}

// NewGitDiffCollector creates a new collector.
func NewGitDiffCollector() *GitDiffCollector {
	return &GitDiffCollector{}
}

// Collect runs git diff --stat and git diff in the given worktree and returns
// parsed results. If sinceRef is non-empty, diff is computed against that ref
// (e.g. the branch snapshot stored when the session started); otherwise the
// diff is against HEAD (showing working-tree changes).
func (c *GitDiffCollector) Collect(worktreePath, sinceRef string) ([]FileChange, string, string, error) {
	stat, err := c.diffStat(worktreePath, sinceRef)
	if err != nil {
		return nil, "", "", err
	}
	changes := parseDiffStat(stat)

	full, err := c.diffFull(worktreePath, sinceRef)
	if err != nil {
		return changes, stat, "", err
	}

	return changes, stat, full, nil
}

func (c *GitDiffCollector) diffStat(worktreePath, sinceRef string) (string, error) {
	args := []string{"-C", worktreePath, "diff", "--stat"}
	if sinceRef != "" {
		args = append(args, sinceRef)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *GitDiffCollector) diffFull(worktreePath, sinceRef string) (string, error) {
	args := []string{"-C", worktreePath, "diff"}
	if sinceRef != "" {
		args = append(args, sinceRef)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseDiffStat parses the output of `git diff --stat`.
// Example lines:
//
//	internal/app/app.go | 15 +++++++++++---
//	 go.mod              |  2 +-
//	 2 files changed, 12 insertions(+), 5 deletions(-)
func parseDiffStat(output string) []FileChange {
	lines := strings.Split(output, "\n")
	var changes []FileChange

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip the summary line like "2 files changed, 12 insertions(+), 5 deletions(-)"
		if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
			continue
		}

		// Split on " | " to separate path from stats
		parts := strings.Split(line, " | ")
		if len(parts) != 2 {
			continue
		}
		path := strings.TrimSpace(parts[0])
		statPart := strings.TrimSpace(parts[1])

		fc := FileChange{Path: path}

		// statPart can be "15 +++++++++++---" or "Bin 12345 -> 67890" or "2 +-"
		if strings.HasPrefix(statPart, "Bin") {
			fc.Status = "binary"
			changes = append(changes, fc)
			continue
		}

		// Count + and - in the visual bar
		spaceIdx := strings.LastIndexFunc(statPart, func(r rune) bool {
			return r == ' '
		})
		if spaceIdx >= 0 {
			bar := statPart[spaceIdx+1:]
			for _, r := range bar {
				switch r {
				case '+':
					fc.Additions++
				case '-':
					fc.Deletions++
				}
			}
		}

		// Determine status from additions/deletions or git status if needed.
		// For a simple heuristic: if both add and del > 0 -> modified;
		// if only add > 0 -> added; if only del > 0 -> deleted.
		if fc.Additions > 0 && fc.Deletions > 0 {
			fc.Status = "modified"
		} else if fc.Additions > 0 {
			fc.Status = "added"
		} else if fc.Deletions > 0 {
			fc.Status = "deleted"
		} else {
			fc.Status = "modified"
		}

		changes = append(changes, fc)
	}

	return changes
}

// CountDiffLines counts insertions and deletions from a raw diff text.
func CountDiffLines(diff string) (insertions, deletions int) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			insertions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}
	return
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
