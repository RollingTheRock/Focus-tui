package a2a

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRouterPublishSubscribe(t *testing.T) {
	r := NewRouter(filepath.Join(t.TempDir(), "focus-a2a.sock"))
	if err := r.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	ch, unsubscribe, err := r.Subscribe("orchestrator", 1)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	if err := r.Publish(Message{
		ID:   "msg-1",
		From: "agent-a",
		To:   "orchestrator",
		Type: "status.update",
		Payload: map[string]any{
			"task_id": "task-1",
		},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := <-ch
	if msg.ID != "msg-1" || msg.Type != "status.update" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestRouterAcceptsSocketMessageAndPublishes(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "focus-a2a.sock")
	r := NewRouter(socket)
	if err := r.Start(); err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	ch, unsubscribe, err := r.Subscribe("orchestrator", 1)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		if unixSocketNotPermitted(err) {
			t.Skipf("unix socket not permitted in this environment: %v", err)
		}
		t.Fatalf("dial socket: %v", err)
	}
	defer conn.Close()

	wireMsg := Message{
		ID:   "msg-socket-1",
		From: "agent-b",
		To:   "orchestrator",
		Type: "task.delegation",
		Payload: map[string]any{
			"task_id": "task-2",
		},
	}
	if err := json.NewEncoder(conn).Encode(wireMsg); err != nil {
		t.Fatalf("encode message: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.ID != "msg-socket-1" || msg.Type != "task.delegation" {
			t.Fatalf("unexpected routed message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for routed socket message")
	}
}

func unixSocketNotPermitted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "operation not permitted")
}
