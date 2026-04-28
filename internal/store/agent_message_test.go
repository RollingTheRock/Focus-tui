package store

import "testing"

func TestSaveAndListAgentMessages(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveAgentMessage(AgentMessageRecord{
		ID:        "msg-1",
		FromAgent: "session-a",
		ToAgent:   "orchestrator",
		MsgType:   "status_update",
		Payload:   `{"task_id":"t-1","task_state":"done"}`,
	}); err != nil {
		t.Fatalf("save msg1: %v", err)
	}
	if err := s.SaveAgentMessage(AgentMessageRecord{
		ID:        "msg-2",
		FromAgent: "orchestrator",
		ToAgent:   "session-b",
		MsgType:   "task_delegation",
		Payload:   `{"task_id":"t-2"}`,
	}); err != nil {
		t.Fatalf("save msg2: %v", err)
	}

	msgs, err := s.ListAgentMessages("orchestrator", "status_update", 10)
	if err != nil {
		t.Fatalf("list msgs: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "msg-1" {
		t.Fatalf("expected only msg-1, got %+v", msgs)
	}
}
