package graph

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Commit represents a commit node in the graph.
type Commit struct {
	Hash    string
	Parents []string
	Subject string
	Author  string
	Date    string
	Style   *lipgloss.Style
}

// Renderer renders a commit graph using box-drawing characters.
type Renderer struct {
	commits []Commit
	width   int
}

// NewRenderer creates a new graph renderer.
func NewRenderer(commits []Commit) *Renderer {
	return &Renderer{commits: commits}
}

// Render returns the rendered graph lines.
func (r *Renderer) Render(width int) []string {
	r.width = width
	if len(r.commits) == 0 {
		return nil
	}

	var lines []string
	for i, c := range r.commits {
		line := r.renderCommitLine(i, c)
		lines = append(lines, line)
	}
	return lines
}

func (r *Renderer) renderCommitLine(index int, c Commit) string {
	graphChar := "◯ "
	if index == len(r.commits)-1 {
		graphChar = "● "
	} else if len(c.Parents) > 1 {
		graphChar = "⏣ "
	}

	shortHash := c.Hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	graphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	if c.Style != nil {
		graphStyle = *c.Style
	}

	parts := []string{
		graphStyle.Render(graphChar),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Render(shortHash),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(truncate(c.Author, 12)),
		" ",
		truncate(c.Subject, r.width-35),
	}

	return strings.Join(parts, "")
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
