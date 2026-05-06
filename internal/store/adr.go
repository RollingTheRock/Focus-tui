package store

import (
	"database/sql"

	"focus/internal/models"
)

type ADRRecord = models.ADRRecord
type ADRConstraintRecord = models.ADRConstraintRecord

func (s *Store) ListADRs() ([]ADRRecord, error) {
	const q = `
		SELECT id, title, status, version, context, decision, consequences,
		       superseded_by, created_by, created_at, accepted_at, accepted_by, updated_at
		FROM adrs
		ORDER BY id ASC
	`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ADRRecord
	for rows.Next() {
		var r ADRRecord
		var consequences, supersededBy, acceptedBy sql.NullString
		var acceptedAt sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Status, &r.Version,
			&r.Context, &r.Decision, &consequences,
			&supersededBy, &r.CreatedBy, &r.CreatedAt,
			&acceptedAt, &acceptedBy, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if consequences.Valid {
			r.Consequences = consequences.String
		}
		if supersededBy.Valid {
			s := supersededBy.String
			r.SupersededBy = &s
		}
		if acceptedAt.Valid {
			t := acceptedAt.Time
			r.AcceptedAt = &t
		}
		if acceptedBy.Valid {
			s := acceptedBy.String
			r.AcceptedBy = &s
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *Store) GetADR(id string) (*ADRRecord, error) {
	const q = `
		SELECT id, title, status, version, context, decision, consequences,
		       superseded_by, created_by, created_at, accepted_at, accepted_by, updated_at
		FROM adrs WHERE id = ?
	`
	row := s.db.QueryRow(q, id)
	var r ADRRecord
	var consequences, supersededBy, acceptedBy sql.NullString
	var acceptedAt sql.NullTime
	if err := row.Scan(
		&r.ID, &r.Title, &r.Status, &r.Version,
		&r.Context, &r.Decision, &consequences,
		&supersededBy, &r.CreatedBy, &r.CreatedAt,
		&acceptedAt, &acceptedBy, &r.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if consequences.Valid {
		r.Consequences = consequences.String
	}
	if supersededBy.Valid {
		s := supersededBy.String
		r.SupersededBy = &s
	}
	if acceptedAt.Valid {
		t := acceptedAt.Time
		r.AcceptedAt = &t
	}
	if acceptedBy.Valid {
		s := acceptedBy.String
		r.AcceptedBy = &s
	}
	return &r, nil
}

func (s *Store) ListADRConstraints(adrID string) ([]ADRConstraintRecord, error) {
	const q = `
		SELECT id, adr_id, category, rule, rationale, created_at
		FROM adr_constraints
		WHERE adr_id = ?
		ORDER BY category, rule ASC
	`
	rows, err := s.db.Query(q, adrID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ADRConstraintRecord
	for rows.Next() {
		var r ADRConstraintRecord
		var rationale sql.NullString
		if err := rows.Scan(&r.ID, &r.ADRID, &r.Category, &r.Rule, &rationale, &r.CreatedAt); err != nil {
			return nil, err
		}
		if rationale.Valid {
			r.Rationale = rationale.String
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
