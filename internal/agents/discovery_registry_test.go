package agents

import (
	"testing"

	"focus/internal/store"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryRegistryBootstrap(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	dr := NewDiscoveryRegistry(s)
	err = dr.BootstrapIfEmpty()
	require.NoError(t, err)

	defs, err := dr.ListAll()
	require.NoError(t, err)
	require.Len(t, defs, 11) // 4 built-in + 7 recommended

	enabled, err := dr.ListEnabled()
	require.NoError(t, err)
	// All returned agents must be both installed and enabled.
	// In a clean environment this is empty; if the host has agents
	// installed the scan will discover them.
	for _, e := range enabled {
		require.True(t, e.IsInstalled, "enabled agent %s should be installed", e.ID)
		require.True(t, e.IsEnabled, "enabled agent %s should be enabled", e.ID)
	}
}
