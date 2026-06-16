package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/third_party/kimi-agent-sdk"
	"github.com/RollingTheRock/Focus-tui/internal/third_party/kimi-agent-sdk/wire"
)

// KimiDriver implements AgentDriver for the Kimi CLI using the official
// Go SDK (subprocess + JSON-RPC over stdio).
type KimiDriver struct{}

// NewKimiDriver creates a new Kimi driver.
func NewKimiDriver() *KimiDriver {
	return &KimiDriver{}
}

// Provider returns ProviderKimi.
func (d *KimiDriver) Provider() Provider {
	return ProviderKimi
}

// Start launches a Kimi headless session.
func (d *KimiDriver) Start(ctx context.Context, req DriverStartRequest) (DriverSession, error) {
	// Build the initial prompt from objective + acceptance.
	promptText := req.Objective
	if req.Acceptance != "" {
		promptText += "\n\nAcceptance criteria:\n" + req.Acceptance
	}

	opts := []kimi.Option{
		kimi.WithExecutable("kimi"),
	}
	if cwd := req.WorktreeID; cwd != "" {
		opts = append(opts, kimi.WithWorkDir(cwd))
	}
	if len(req.EnvVars) > 0 {
		var args []string
		for _, ev := range req.EnvVars {
			args = append(args, "--env", ev)
		}
		opts = append(opts, kimi.WithArgs(args...))
	}

	session, err := kimi.NewSession(opts...)
	if err != nil {
		return nil, fmt.Errorf("kimi NewSession: %w", err)
	}

	content := wire.NewStringContent(promptText)
	turn, err := session.Prompt(ctx, content)
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("kimi Prompt: %w", err)
	}

	s := &kimiSession{
		session:         session,
		turn:            turn,
		events:          make(chan DriverEvent, 128),
		pendingApproval: make(map[string]*wire.ApprovalRequest),
	}

	go s.consumeEvents()

	return s, nil
}

type kimiSession struct {
	session *kimi.Session
	turn    *kimi.Turn
	events  chan DriverEvent

	mu              sync.Mutex
	pendingApproval map[string]*wire.ApprovalRequest
	turnNum         int
}

// Events returns the event channel.
func (s *kimiSession) Events() <-chan DriverEvent {
	return s.events
}

// Send sends a user message (starts a new turn).
func (s *kimiSession) Send(ctx context.Context, message string) error {
	content := wire.NewStringContent(message)
	turn, err := s.session.Prompt(ctx, content)
	if err != nil {
		return err
	}
	s.turn = turn
	go s.consumeEvents()
	return nil
}

// Approve responds to an approval request.
// decision should be "approve", "approve_for_session", or "reject".
func (s *kimiSession) Approve(ctx context.Context, requestID string, decision string) error {
	s.mu.Lock()
	req, ok := s.pendingApproval[requestID]
	delete(s.pendingApproval, requestID)
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("approval request %q not found", requestID)
	}

	resp := wire.ApprovalRequestResponse(decision)
	return req.Respond(wire.ApprovalResponse{
		RequestID: requestID,
		Response:  resp,
	})
}

// Cancel cancels the current turn.
func (s *kimiSession) Cancel(ctx context.Context) error {
	return s.turn.Cancel()
}

// Stop terminates the session.
func (s *kimiSession) Stop(ctx context.Context) error {
	return s.session.Close()
}

// consumeEvents reads from the Kimi SDK turn stream and converts messages
// into normalised DriverEvents.
func (s *kimiSession) consumeEvents() {
	defer close(s.events)

	turnNum := s.turnNum
	s.turnNum++

	for step := range s.turn.Steps {
		// Emit step_begin.
		s.emit(DriverEvent{
			Type:      DriverEventStepBegin,
			Turn:      turnNum,
			Timestamp: time.Now().UnixNano(),
		})

		for msg := range step.Messages {
			s.convertMessage(msg, turnNum)
		}

		// Step ended naturally (or was interrupted — interrupted is emitted
		// as an error event by convertMessage).
	}

	// Emit turn_complete.
	s.emit(DriverEvent{
		Type:      DriverEventTurnComplete,
		Turn:      turnNum,
		Timestamp: time.Now().UnixNano(),
	})
}

func (s *kimiSession) convertMessage(msg wire.Message, turnNum int) {
	ts := time.Now().UnixNano()

	switch m := msg.(type) {
	case wire.TurnBegin:
		s.emit(DriverEvent{Type: DriverEventTurnStarted, Turn: turnNum, Timestamp: ts})

	case wire.TurnEnd:
		// handled in consumeEvents after step loop

	case wire.ContentPart:
		if m.Type == wire.ContentPartTypeText {
			s.emit(DriverEvent{
				Type:      DriverEventMessageDelta,
				Turn:      turnNum,
				Timestamp: ts,
				Payload:   MessageDeltaPayload{Text: m.Text.Value},
			})
		}

	case wire.ToolCall:
		s.emit(DriverEvent{
			Type:      DriverEventToolCall,
			Turn:      turnNum,
			Timestamp: ts,
			Payload: ToolCallPayload{
				ID:        m.ID,
				Name:      m.Function.Name,
				Arguments: m.Function.Arguments.Value,
			},
		})

	case wire.ToolResult:
		s.emit(DriverEvent{
			Type:      DriverEventToolResult,
			Turn:      turnNum,
			Timestamp: ts,
			Payload: ToolResultPayload{
				ID:      "", // ToolResult does not expose ID directly in the SDK
				Content: fmt.Sprintf("%v", m),
			},
		})

	case wire.ApprovalRequest:
		s.mu.Lock()
		s.pendingApproval[m.ID] = &m
		s.mu.Unlock()
		s.emit(DriverEvent{
			Type:      DriverEventApprovalRequest,
			Turn:      turnNum,
			Timestamp: ts,
			Payload: ApprovalRequestPayload{
				RequestID:   m.ID,
				ToolCallID:  m.ToolCallID,
				Action:      m.Action,
				Description: m.Description,
			},
		})

	case wire.StatusUpdate:
		s.emit(DriverEvent{
			Type:      DriverEventStatusUpdate,
			Turn:      turnNum,
			Timestamp: ts,
			Payload:   StatusUpdatePayload{Message: fmt.Sprintf("%v", m)},
		})

	case wire.StepInterrupted:
		s.emit(DriverEvent{
			Type:      DriverEventStepInterrupted,
			Turn:      turnNum,
			Timestamp: ts,
		})

	case wire.CompactionBegin:
		s.emit(DriverEvent{
			Type:      DriverEventCompactionBegin,
			Turn:      turnNum,
			Timestamp: ts,
		})

	case wire.CompactionEnd:
		s.emit(DriverEvent{
			Type:      DriverEventCompactionEnd,
			Turn:      turnNum,
			Timestamp: ts,
		})

	default:
		// Ignore unhandled message types.
	}
}

func (s *kimiSession) emit(ev DriverEvent) {
	select {
	case s.events <- ev:
	default:
		// Channel full; drop event. In production we may want to log this.
	}
}

// _compile-time interface checks.
var _ AgentDriver = (*KimiDriver)(nil)
var _ DriverSession = (*kimiSession)(nil)
