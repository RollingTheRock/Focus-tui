package app

// AppState represents the top-level application state.
type AppState int

const (
	// StateOverviewPage is the global orchestration page that lists and manages worktree containers.
	StateOverviewPage AppState = iota
	// StateWorktreePage is a full-screen worktree workspace page.
	StateWorktreePage

	// StateDashboard is kept as a compatibility alias for the old single-page model.
	StateDashboard = StateOverviewPage
)

// AppMode represents the current interaction mode.
type AppMode int

const (
	ModeNormal AppMode = iota
	ModeInput
	ModeShell // all keyboard input forwarded to PTY
)

// OverlayKind indicates which overlay is currently displayed.
type OverlayKind int

const (
	OverlayNone   OverlayKind = iota
	OverlayPicker             // pomodoro todo-picker
)
