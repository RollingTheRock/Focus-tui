package store

import (
	"testing"

	"focus/internal/models"

	"github.com/stretchr/testify/require"
)

func TestAgentDefinitionCRUD(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	// Save a built-in agent.
	def := models.AgentDefinition{
		ID:           "claude",
		Name:         "Claude Code",
		Description:  "Anthropic",
		Binary:       "claude",
		ProviderType: "claude",
		Tags:         []string{"coding", "architecture"},
		Category:     "built-in",
		IsEnabled:    true,
	}
	require.NoError(t, s.SaveAgentDefinition(def))

	// List should return it.
	defs, err := s.ListAgentDefinitions()
	require.NoError(t, err)
	require.Len(t, defs, 1)
	require.Equal(t, "Claude Code", defs[0].Name)
	require.Equal(t, []string{"coding", "architecture"}, defs[0].Tags)

	// Get by ID.
	got, err := s.GetAgentDefinition("claude")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "Anthropic", got.Description)

	// Toggle enabled.
	require.NoError(t, s.ToggleAgentDefinitionEnabled("claude"))
	got, err = s.GetAgentDefinition("claude")
	require.NoError(t, err)
	require.False(t, got.IsEnabled)

	// Delete built-in should be blocked (our SQL uses category='registered').
	_ = s.DeleteAgentDefinition("claude")
	defs, _ = s.ListAgentDefinitions()
	require.Len(t, defs, 1) // still there

	// Register custom agent and delete it.
	custom := models.AgentDefinition{
		ID:       "my-agent",
		Name:     "My Agent",
		Binary:   "my-agent",
		Category: "registered",
	}
	require.NoError(t, s.SaveAgentDefinition(custom))
	require.NoError(t, s.DeleteAgentDefinition("my-agent"))
	defs, _ = s.ListAgentDefinitions()
	require.Len(t, defs, 1) // only claude remains
}
