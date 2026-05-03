//go:build e2e

package agents

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestKimiDriverE2E(t *testing.T) {
	t.Log("Creating Kimi driver...")
	driver := NewKimiDriver()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Starting headless session...")
	session, err := driver.Start(ctx, DriverStartRequest{
		WorktreeID: "./",
		Objective:  "Say hello",
	})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	t.Log("Session started successfully")

	// Consume events with a separate timeout.
	eventCtx, eventCancel := context.WithTimeout(ctx, 45*time.Second)
	defer eventCancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case ev, ok := <-session.Events():
				if !ok {
					t.Log("Event channel closed")
					return
				}
				switch ev.Type {
				case DriverEventMessageDelta:
					p := ev.Payload.(MessageDeltaPayload)
					t.Logf("[TEXT] %s", p.Text)
				case DriverEventToolCall:
					p := ev.Payload.(ToolCallPayload)
					t.Logf("[TOOL] %s(%s)", p.Name, p.Arguments)
				case DriverEventToolResult:
					p := ev.Payload.(ToolResultPayload)
					t.Logf("[RESULT] %s", p.Content)
				case DriverEventApprovalRequest:
					p := ev.Payload.(ApprovalRequestPayload)
					t.Logf("[APPROVAL] %s: %s", p.Action, p.Description)
					t.Log("Auto-approving...")
					if err := session.Approve(ctx, p.RequestID, "approve"); err != nil {
						t.Logf("Approve error: %v", err)
					}
				case DriverEventStatusUpdate:
					p := ev.Payload.(StatusUpdatePayload)
					t.Logf("[STATUS] %s", p.Message)
				case DriverEventTurnStarted:
					t.Log("[TURN] Started")
				case DriverEventTurnComplete:
					t.Log("[TURN] Complete")
					return
				case DriverEventStepBegin:
					t.Log("[STEP] Begin")
				case DriverEventStepInterrupted:
					t.Log("[STEP] Interrupted")
				case DriverEventCompactionBegin:
					t.Log("[COMPACT] Begin")
				case DriverEventCompactionEnd:
					t.Log("[COMPACT] End")
				case DriverEventError:
					t.Logf("[ERROR] %v", ev.Payload)
				default:
					t.Logf("[OTHER] %s", ev.Type)
				}
			case <-eventCtx.Done():
				t.Log("Event context cancelled")
				return
			}
		}
	}()

	select {
	case <-done:
		t.Log("Event consumer finished")
	case <-eventCtx.Done():
		t.Log("Timeout waiting for events, stopping session...")
	}

	t.Log("Stopping session...")
	if err := session.Stop(ctx); err != nil {
		t.Logf("Stop error: %v", err)
	}

	fmt.Println("=== E2E Test Complete ===")
}
