package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"focus/internal/models"
)

func (s *Store) ListAgentDefinitions() ([]models.AgentDefinition, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, binary, args, env_vars, provider_type,
		       tags, category, install_hint, capabilities, is_installed, is_enabled,
		       last_used_at, created_at, updated_at
		FROM agent_definitions
		ORDER BY category, name
	`)
	if err != nil {
		return nil, fmt.Errorf("list agent definitions: %w", err)
	}
	defer rows.Close()

	return scanAgentDefinitions(rows)
}

func (s *Store) GetAgentDefinition(id string) (*models.AgentDefinition, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, binary, args, env_vars, provider_type,
		       tags, category, install_hint, capabilities, is_installed, is_enabled,
		       last_used_at, created_at, updated_at
		FROM agent_definitions
		WHERE id = ?
	`, id)
	def, err := scanAgentDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent definition: %w", err)
	}
	return def, nil
}

func (s *Store) SaveAgentDefinition(def models.AgentDefinition) error {
	args, err := json.Marshal(def.Args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}
	envVars, err := json.Marshal(def.EnvVars)
	if err != nil {
		return fmt.Errorf("marshal env_vars: %w", err)
	}
	tags, err := json.Marshal(def.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	caps, err := json.Marshal(def.Capabilities)
	if err != nil {
		return fmt.Errorf("marshal capabilities: %w", err)
	}

	var lastUsed sql.NullTime
	if def.LastUsedAt != nil {
		lastUsed = sql.NullTime{Time: *def.LastUsedAt, Valid: true}
	}

	_, err = s.db.Exec(`
		INSERT INTO agent_definitions (
			id, name, description, binary, args, env_vars, provider_type,
			tags, category, install_hint, capabilities, is_installed, is_enabled,
			last_used_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			binary = excluded.binary,
			args = excluded.args,
			env_vars = excluded.env_vars,
			provider_type = excluded.provider_type,
			tags = excluded.tags,
			category = excluded.category,
			install_hint = excluded.install_hint,
			capabilities = excluded.capabilities,
			is_installed = excluded.is_installed,
			is_enabled = excluded.is_enabled,
			last_used_at = excluded.last_used_at,
			updated_at = excluded.updated_at
	`,
		def.ID, def.Name, def.Description, def.Binary,
		string(args), string(envVars), def.ProviderType,
		string(tags), def.Category, def.InstallHint,
		string(caps), def.IsInstalled, def.IsEnabled, lastUsed,
		def.CreatedAt, def.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save agent definition: %w", err)
	}
	return nil
}

func (s *Store) DeleteAgentDefinition(id string) error {
	_, err := s.db.Exec(`DELETE FROM agent_definitions WHERE id = ? AND category = 'registered'`, id)
	if err != nil {
		return fmt.Errorf("delete agent definition: %w", err)
	}
	return nil
}

func scanAgentDefinitions(rows *sql.Rows) ([]models.AgentDefinition, error) {
	var out []models.AgentDefinition
	for rows.Next() {
		def, err := scanAgentDefinitionFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *def)
	}
	return out, rows.Err()
}

func scanAgentDefinition(row *sql.Row) (*models.AgentDefinition, error) {
	// sql.Row does not implement the same interface as sql.Rows for Scan,
	// so we duplicate the logic slightly.
	var def models.AgentDefinition
	var argsStr, envVarsStr, tagsStr, capsStr string
	var lastUsed sql.NullTime

	err := row.Scan(
		&def.ID, &def.Name, &def.Description, &def.Binary,
		&argsStr, &envVarsStr, &def.ProviderType,
		&tagsStr, &def.Category, &def.InstallHint,
		&capsStr, &def.IsInstalled, &def.IsEnabled,
		&lastUsed, &def.CreatedAt, &def.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if lastUsed.Valid {
		def.LastUsedAt = &lastUsed.Time
	}
	_ = json.Unmarshal([]byte(argsStr), &def.Args)
	_ = json.Unmarshal([]byte(envVarsStr), &def.EnvVars)
	_ = json.Unmarshal([]byte(tagsStr), &def.Tags)
	_ = json.Unmarshal([]byte(capsStr), &def.Capabilities)

	return &def, nil
}

func scanAgentDefinitionFromRows(rows *sql.Rows) (*models.AgentDefinition, error) {
	var def models.AgentDefinition
	var argsStr, envVarsStr, tagsStr, capsStr string
	var lastUsed sql.NullTime

	err := rows.Scan(
		&def.ID, &def.Name, &def.Description, &def.Binary,
		&argsStr, &envVarsStr, &def.ProviderType,
		&tagsStr, &def.Category, &def.InstallHint,
		&capsStr, &def.IsInstalled, &def.IsEnabled,
		&lastUsed, &def.CreatedAt, &def.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if lastUsed.Valid {
		def.LastUsedAt = &lastUsed.Time
	}
	_ = json.Unmarshal([]byte(argsStr), &def.Args)
	_ = json.Unmarshal([]byte(envVarsStr), &def.EnvVars)
	_ = json.Unmarshal([]byte(tagsStr), &def.Tags)
	_ = json.Unmarshal([]byte(capsStr), &def.Capabilities)

	return &def, nil
}

// Helper to update just the installed flag after a PATH scan.
func (s *Store) UpdateAgentDefinitionInstalled(id string, installed bool) error {
	_, err := s.db.Exec(`
		UPDATE agent_definitions
		SET is_enabled = CASE WHEN ? = 0 AND is_enabled = 1 THEN 0 ELSE is_enabled END,
		    updated_at = ?
		WHERE id = ?
	`, installed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update agent installed: %w", err)
	}
	return nil
}

// Helper to toggle enabled state.
func (s *Store) ToggleAgentDefinitionEnabled(id string) error {
	_, err := s.db.Exec(`
		UPDATE agent_definitions
		SET is_enabled = NOT is_enabled,
		    updated_at = ?
		WHERE id = ?
	`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("toggle agent enabled: %w", err)
	}
	return nil
}
