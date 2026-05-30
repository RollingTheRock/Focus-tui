package models

// PaneID uniquely identifies a pane instance.
type PaneID string

// PaneType describes the function of a pane.
type PaneType string

const (
	// Core panes (保留现有)
	PaneTypeHeader   PaneType = "header"
	PaneTypeShell    PaneType = "shell"
	PaneTypeTodo     PaneType = "todo"
	PaneTypePomodoro PaneType = "pomodoro"
	PaneTypeFooter   PaneType = "footer"

	// Plugin panes (新增 - Phase 2.4)
	PaneTypeWorktree     PaneType = "worktree"
	PaneTypeGitStatus    PaneType = "git-status"
	PaneTypeFileTree     PaneType = "file-tree"
	PaneTypeDiffView     PaneType = "diff-view"
	PaneTypeLogView      PaneType = "log-view"
	PaneTypeEditor       PaneType = "editor"
	PaneTypeAgentSession PaneType = "agent-session"
	PaneTypeAgentStore   PaneType = "agent-store"
)

// PaneStatus is lightweight display metadata for a pane.
type PaneStatus string

const (
	PaneStatusStarting PaneStatus = "starting"
	PaneStatusRunning  PaneStatus = "running"
	PaneStatusExited   PaneStatus = "exited"
	PaneStatusReady    PaneStatus = "ready"
	PaneStatusActive   PaneStatus = "active"
	PaneStatusIdle     PaneStatus = "idle"
	PaneStatusPassive  PaneStatus = "passive"
)

// PaneMeta stores app-level metadata for routing and titles.
type PaneMeta struct {
	ID             PaneID
	Name           string
	Type           PaneType
	CWD            string
	RepoID         string
	WorktreeID     string
	BranchSnapshot string
	Status         PaneStatus
	Closable       bool
}

// PaneFrame is the absolute screen rectangle allocated to a pane.
// Width and height include the bordered frame rendered by the app.
type PaneFrame struct {
	X int
	Y int
	W int
	H int
}
