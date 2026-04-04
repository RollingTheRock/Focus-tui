package models

import (
	"focus/internal/config"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
)

// CommonModel holds shared state across all panels.
type CommonModel struct {
	Width  int
	Height int
	Theme  styles.Theme
	Cfg    config.Config
}

// Panel is the interface implemented by every UI sub-model.
type Panel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Panel, tea.Cmd)
	View() string
	SetSize(width, height int)
}
