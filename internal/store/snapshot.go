package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"focus/internal/models"
)

type PageSnapshotRecord = models.PageSnapshotRecord

func (s *Store) SavePageSnapshot(worktreeID string, snapshotJSON []byte) error {
	if worktreeID == "" {
		return fmt.Errorf("worktreeID required")
	}
	const q = `
		INSERT INTO page_snapshots (worktree_id, snapshot_json)
		VALUES (?, ?)
		ON CONFLICT(worktree_id) DO UPDATE SET
			snapshot_json = excluded.snapshot_json,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q, worktreeID, string(snapshotJSON))
	return err
}

func (s *Store) LoadPageSnapshot(worktreeID string) ([]byte, error) {
	if worktreeID == "" {
		return nil, nil
	}
	const q = `SELECT snapshot_json FROM page_snapshots WHERE worktree_id = ?`
	var raw string
	err := s.db.QueryRow(q, worktreeID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

func (s *Store) ListPageSnapshots() ([]PageSnapshotRecord, error) {
	const q = `SELECT worktree_id, snapshot_json FROM page_snapshots`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []PageSnapshotRecord
	for rows.Next() {
		var r PageSnapshotRecord
		if err := rows.Scan(&r.WorktreeID, &r.SnapshotJSON); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *Store) DeletePageSnapshot(worktreeID string) error {
	_, err := s.db.Exec(`DELETE FROM page_snapshots WHERE worktree_id = ?`, worktreeID)
	return err
}

type PageSnapshot struct {
	BodyTreeJSON []byte   `json:"bodyTree"`
	Focused      string   `json:"focused"`
	OpenEditors  []string `json:"openEditors"`
	ZoomedPane   string   `json:"zoomedPane,omitempty"`
	PreZoomTree  []byte   `json:"preZoomTree,omitempty"`
}

func MarshalPageSnapshot(s PageSnapshot) ([]byte, error) {
	return json.Marshal(s)
}

func UnmarshalPageSnapshot(data []byte) (PageSnapshot, error) {
	var s PageSnapshot
	err := json.Unmarshal(data, &s)
	return s, err
}
