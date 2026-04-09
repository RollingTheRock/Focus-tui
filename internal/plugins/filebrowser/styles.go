package filebrowser

import (
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

var (
	treeHeaderStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	emptyStyle      = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	errorStyle      = lipgloss.NewStyle().Foreground(appstyles.Overdue)
	selectedStyle   = lipgloss.NewStyle().Background(appstyles.Highlight)

	folderIconStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)
	fileIconStyle   = lipgloss.NewStyle().Foreground(appstyles.Text)
	nameStyle       = lipgloss.NewStyle().Foreground(appstyles.Text)
	metaStyle       = lipgloss.NewStyle().Foreground(appstyles.Subtle)
)

func renderNodeIcon(node *FileNode) string {
	if node == nil {
		return ""
	}
	if node.IsDir {
		return folderIconStyle.Render("📁")
	}
	return fileIconStyle.Render("📄")
}
