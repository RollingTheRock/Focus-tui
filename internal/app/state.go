package app

// AppState represents the top-level application state.
type AppState int

const (
	StateDashboard AppState = iota
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
