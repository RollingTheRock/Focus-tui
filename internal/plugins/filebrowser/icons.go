package filebrowser

import (
	"path/filepath"
	"strings"

	"github.com/epilande/go-devicons"
	"charm.land/lipgloss/v2"
)

// IconMode controls how file icons are rendered.
type IconMode string

const (
	IconModeNerd    IconMode = "nerd"
	IconModeUnicode IconMode = "unicode"
	IconModeASCII   IconMode = "ascii"
	IconModeNone    IconMode = "none"
)

// FileIcon returns the icon and style for a given filename.
func FileIcon(name string, isDir bool, mode IconMode) (string, lipgloss.Style) {
	if isDir {
		return dirIcon(mode)
	}
	switch mode {
	case IconModeNerd:
		return nerdIcon(name)
	case IconModeUnicode:
		return unicodeIcon(name)
	case IconModeASCII:
		return asciiIcon(name)
	default:
		return "", lipgloss.NewStyle()
	}
}

func dirIcon(mode IconMode) (string, lipgloss.Style) {
	switch mode {
	case IconModeNerd:
		return "\uf07b", lipgloss.NewStyle().Foreground(lipgloss.Color("#878787"))
	case IconModeUnicode:
		return "▸", lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	case IconModeASCII:
		return "[-]", lipgloss.NewStyle()
	default:
		return "", lipgloss.NewStyle()
	}
}

func nerdIcon(name string) (string, lipgloss.Style) {
	st := devicons.IconForPath(name)
	if st.Icon == "" {
		return "\uf15b", lipgloss.NewStyle().Foreground(lipgloss.Color("#878787"))
	}
	return st.Icon, lipgloss.NewStyle().Foreground(lipgloss.Color(st.Color))
}

func unicodeIcon(name string) (string, lipgloss.Style) {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#00ADD8"))
	case ".md":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	case ".json", ".yaml", ".yml":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	case ".js", ".ts", ".tsx":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DF1E"))
	case ".py":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#3776AB"))
	case ".rs":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#DEA584"))
	case ".sh", ".bash":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#89E051"))
	case ".dockerfile":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#2496ED"))
	case ".mod", ".sum":
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C"))
	default:
		return "▪", lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	}
}

func asciiIcon(name string) (string, lipgloss.Style) {
	return "[]", lipgloss.NewStyle()
}
