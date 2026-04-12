package editor

import "focus/internal/models"

type OpenBehavior string

const (
	OpenBehaviorDefault OpenBehavior = "default"
	OpenBehaviorVSplit  OpenBehavior = "vsplit"
)

type OpenEditorMsg struct {
	FilePath string
	Behavior OpenBehavior
}

type CloseEditorMsg struct {
	ID models.PaneID
}

type SaveCompletedMsg struct {
	ID       models.PaneID
	FilePath string
	Dirty    bool
}
