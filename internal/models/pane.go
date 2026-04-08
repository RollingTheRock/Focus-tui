package models

// PaneID uniquely identifies a pane instance.
type PaneID string

// PaneType describes the function of a pane.
type PaneType string

const (
	PaneTypeHeader   PaneType = "header"
	PaneTypeShell    PaneType = "shell"
	PaneTypeTodo     PaneType = "todo"
	PaneTypePomodoro PaneType = "pomodoro"
	PaneTypeFooter   PaneType = "footer"
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
	ID       PaneID
	Name     string
	Type     PaneType
	CWD      string
	Status   PaneStatus
	Closable bool
}

// PaneFrame is the absolute screen rectangle allocated to a pane.
// Width and height include the bordered frame rendered by the app.
type PaneFrame struct {
	X int
	Y int
	W int
	H int
}
