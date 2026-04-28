package a2a

import "testing"

func TestRouterPublishSubscribe(t *testing.T) {
	r := NewRouter("/tmp/focus-a2a.sock")
	if err := r.Start(); err != nil {
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
