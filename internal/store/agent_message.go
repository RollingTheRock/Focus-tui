package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

type AgentMessageRecord = models.AgentMessageRecord

func (s *Store) SaveAgentMessage(record AgentMessageRecord) error {
	if record.ID == "" {
		return fmt.Errorf("agent message id required")
	}
	if record.FromAgent == "" {
		return fmt.Errorf("agent message from required")
	}
	if record.ToAgent == "" {
		return fmt.Errorf("agent message to required")
	}
	if record.MsgType == "" {
		return fmt.Errorf("agent message type required")
	}
	if record.Payload == "" {
		record.Payload = "{}"
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	s.tryAppendEvent(events.AggregateAgentSession, record.ID, events.AgentMessageSent,
		events.AgentMessageSentPayload{
			ID:        record.ID,
			FromAgent: record.FromAgent,
			ToAgent:   record.ToAgent,
			MsgType:   record.MsgType,
			Payload:   record.Payload,
		}, events.AggregateAgentSession, record.FromAgent)
	q := fmt.Sprintf(`
		INSERT INTO %s (id, from_agent, to_agent, msg_type, payload, read_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			from_agent = excluded.from_agent,
			to_agent = excluded.to_agent,
			msg_type = excluded.msg_type,
			payload = excluded.payload,
			read_at = excluded.read_at
	`, s.tbl("agent_messages", "proj_agent_messages"))
	_, err := s.exec(
		q,
		record.ID,
		record.FromAgent,
		record.ToAgent,
		record.MsgType,
		record.Payload,
		nullableTimePtr(record.ReadAt),
		record.CreatedAt,
	)
	return err
}

func (s *Store) ListAgentMessages(target string, messageType string, limit int) ([]AgentMessageRecord, error) {
	base := fmt.Sprintf(`
		SELECT id, from_agent, to_agent, msg_type, payload, read_at, created_at
		FROM %s
	`, s.tbl("agent_messages", "proj_agent_messages"))
	filters := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if strings.TrimSpace(target) != "" {
		filters = append(filters, "to_agent = ?")
		args = append(args, target)
	}
	if strings.TrimSpace(messageType) != "" {
		filters = append(filters, "msg_type = ?")
		args = append(args, messageType)
	}

	q := base
	if len(filters) > 0 {
		q += " WHERE " + strings.Join(filters, " AND ")
	}
	q += " ORDER BY created_at DESC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AgentMessageRecord, 0)
	for rows.Next() {
		var record AgentMessageRecord
		var readAt sql.NullTime
		if err := rows.Scan(
			&record.ID,
			&record.FromAgent,
			&record.ToAgent,
			&record.MsgType,
			&record.Payload,
			&readAt,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		if readAt.Valid {
			t := readAt.Time
			record.ReadAt = &t
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
