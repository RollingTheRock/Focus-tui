package agents

import "context"

// DriverEventType categorises events emitted by a headless agent driver.
type DriverEventType string

const (
	DriverEventTurnStarted      DriverEventType = "turn_started"
	DriverEventTurnComplete     DriverEventType = "turn_complete"
	DriverEventMessageDelta     DriverEventType = "message_delta"
	DriverEventToolCall         DriverEventType = "tool_call"
	DriverEventToolResult       DriverEventType = "tool_result"
	DriverEventApprovalRequest  DriverEventType = "approval_request"
	DriverEventApprovalResolved DriverEventType = "approval_resolved"
	DriverEventStatusUpdate     DriverEventType = "status_update"
	DriverEventStepBegin        DriverEventType = "step_begin"
	DriverEventStepInterrupted  DriverEventType = "step_interrupted"
	DriverEventCompactionBegin  DriverEventType = "compaction_begin"
	DriverEventCompactionEnd    DriverEventType = "compaction_end"
	DriverEventError            DriverEventType = "error"
)

// DriverEvent is a normalised event from a headless agent driver.
type DriverEvent struct {
	Type      DriverEventType
	Turn      int
	Timestamp int64 // UnixNano
	Payload   any
}

// MessageDeltaPayload carries a chunk of assistant text.
type MessageDeltaPayload struct {
	Text string
}

// ToolCallPayload carries a tool invocation.
type ToolCallPayload struct {
	ID        string
	Name      string
	Arguments string // JSON
}

// ToolResultPayload carries the result of a tool call.
type ToolResultPayload struct {
	ID      string
	Content string
	Error   string
}

// ApprovalRequestPayload carries a request for human approval.
type ApprovalRequestPayload struct {
	RequestID   string
	ToolCallID  string
	Action      string
	Description string
}

// StatusUpdatePayload carries a progress update from the agent.
type StatusUpdatePayload struct {
	Message string
}

// DriverStartRequest configures a headless agent session.
type DriverStartRequest struct {
	SessionID    string
	TaskID       string
	PlanID       string
	StepID       string
	WorktreeID   string
	Objective    string // high-level goal
	Acceptance   string // acceptance criteria
	EnvVars      []string
	MaxTurns     int // 0 = unlimited
}

// AgentDriver is the abstraction for programmatically driving an AI agent
// in headless mode.
type AgentDriver interface {
	// Provider returns the agent provider this driver supports.
	Provider() Provider

	// Start launches a headless agent session. The returned DriverSession
	// provides a stream of events and methods to interact with the agent.
	Start(ctx context.Context, req DriverStartRequest) (DriverSession, error)
}

// DriverSession represents an active headless agent session.
type DriverSession interface {
	// Events returns a read-only channel of real-time events from the agent.
	// The channel is closed when the session ends (done, error, or cancelled).
	Events() <-chan DriverEvent

	// Send sends a user message to the agent (intervention).
	Send(ctx context.Context, message string) error

	// Approve responds to an approval request.
	// decision should be "approve", "approve_for_session", or "reject".
	Approve(ctx context.Context, requestID string, decision string) error

	// Cancel cancels the current turn (if any).
	Cancel(ctx context.Context) error

	// Stop terminates the entire session.
	Stop(ctx context.Context) error
}
