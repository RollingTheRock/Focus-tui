package store

import (
	"encoding/json"
	"fmt"

	"focus/internal/models"
)

type PageSnapshotRecord = models.PageSnapshotRecord

func (s *Store) SavePageSnapshot(worktreeID string, snapshotJSON []byte) error {
	if worktreeID == "" {
		return fmt.Errorf("worktreeID required")
	}
	q := fmt.Sprintf(`
		INSERT INTO %s (worktree_id, snapshot_json)
		VALUES (?, ?)
		ON CONFLICT(worktree_id) DO UPDATE SET
			snapshot_json = excluded.snapshot_json,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("page_snapshots", "proj_page_snapshots"))
	_, err := s.exec(q, worktreeID, string(snapshotJSON))
	return err
}

func (s *Store) LoadPageSnapshot(worktreeID string) ([]byte, error) {
	if worktreeID == "" {
		return nil, nil
	}
	q := fmt.Sprintf(`SELECT snapshot_json FROM %s WHERE worktree_id = ?`, s.tbl("page_snapshots", "proj_page_snapshots"))
	var raw string
	err := s.qRow(q, worktreeID).Scan(&raw)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

func (s *Store) ListPageSnapshots() ([]PageSnapshotRecord, error) {
	q := fmt.Sprintf(`SELECT worktree_id, snapshot_json FROM %s`, s.tbl("page_snapshots", "proj_page_snapshots"))
	rows, err := s.qRows(q)
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
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE worktree_id = ?`, s.tbl("page_snapshots", "proj_page_snapshots")), worktreeID)
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
