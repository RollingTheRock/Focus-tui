package editor

import "github.com/RollingTheRock/Focus-tui/internal/models"

type OpenBehavior string

const (
	OpenBehaviorDefault OpenBehavior = "default"
	OpenBehaviorVSplit  OpenBehavior = "vsplit"
)

type OpenEditorMsg struct {
	FilePath   string
	Behavior   OpenBehavior
	LineNumber int
}

type CloseEditorMsg struct {
	ID models.PaneID
}

type SaveCompletedMsg struct {
	ID       models.PaneID
	FilePath string
	Dirty    bool
}
