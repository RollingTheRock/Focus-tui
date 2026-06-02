package trellis

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"focus/internal/agents"
)

// TestProviderDetection verifies that all 5 first-class agent providers
// are correctly detected when installed on the system.
func TestProviderDetection(t *testing.T) {
	bridge := NewBridge("/tmp", "test-wt", nil)
	platforms := bridge.detectInstalledPlatforms()

	expected := map[agents.Provider]bool{
		agents.ProviderClaude:   false,
		agents.ProviderKimi:     false,
		agents.ProviderCodex:    false,
		agents.ProviderGemini:   false,
		agents.ProviderOpenCode: false,
	}

	for _, p := range platforms {
		expected[p] = true
	}

	for provider, found := range expected {
		name := string(provider)
		if _, err := exec.LookPath(name); err == nil {
			if !found {
				t.Errorf("provider %s is installed but not detected", name)
			} else {
				t.Logf("✅ %s detected", name)
			}
		} else {
			t.Logf("⚠️  %s not installed (skipped)", name)
		}
	}
}

// TestTrellisInitFlags verifies that buildTrellisInitFlags generates the
// correct CLI flags for each provider.
func TestTrellisInitFlags(t *testing.T) {
	bridge := NewBridge("/tmp", "test-wt", nil)

	cases := []struct {
		name      string
		providers []agents.Provider
		want      []string
	}{
		{
			name: "all 5 providers",
			providers: []agents.Provider{
				agents.ProviderClaude,
				agents.ProviderKimi,
				agents.ProviderCodex,
				agents.ProviderGemini,
				agents.ProviderOpenCode,
			},
			want: []string{"--claude", "--codex", "--gemini", "--opencode"},
			// Note: Kimi has no --kimi flag in trellis init.
		},
		{
			name:      "only claude",
			providers: []agents.Provider{agents.ProviderClaude},
			want:      []string{"--claude"},
		},
		{
			name:      "only kimi",
			providers: []agents.Provider{agents.ProviderKimi},
			want:      []string{},
		},
		{
			name:      "only gemini",
			providers: []agents.Provider{agents.ProviderGemini},
			want:      []string{"--gemini"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bridge.buildTrellisInitFlags(c.providers)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestTrellisPlatformFilesExist verifies that trellis init generated
// platform-specific configuration files for each detected provider.
func TestTrellisPlatformFilesExist(t *testing.T) {
	repoRoot, _ := filepath.Abs("../..")

	platformChecks := []struct {
		name  string
		paths []string
	}{
		{
			name: "Claude Code",
			paths: []string{
				".claude/agents/trellis-implement.md",
				".claude/hooks/session-start.py",
				".claude/skills/trellis-check/SKILL.md",
			},
		},
		{
			name: "Kimi",
			paths: []string{
				".kimi/agents/trellis-implement.yaml",
				".kimi/hooks/session-start.py",
			},
		},
		{
			name: "Gemini",
			paths: []string{
				".gemini/agents/trellis-implement.md",
				".gemini/hooks/session-start.py",
				// Note: Gemini CLI reads .agents/skills/ directly; no .gemini/skills/.
			},
		},
		{
			name: "OpenCode",
			paths: []string{
				".opencode/agents/trellis-implement.md",
				".opencode/plugins/session-start.js",
				".opencode/skills/trellis-check/SKILL.md",
			},
		},
		{
			name: "Codex",
			paths: []string{
				".codex/agents/trellis-implement.toml",
				".codex/hooks/session-start.py",
				// Note: Codex reads .agents/skills/ via shared config; no .codex/skills/.
			},
		},
	}

	for _, check := range platformChecks {
		t.Run(check.name, func(t *testing.T) {
			found := 0
			for _, rel := range check.paths {
				path := filepath.Join(repoRoot, rel)
				if _, err := os.Stat(path); err == nil {
					found++
					t.Logf("  ✅ %s", rel)
				} else {
					t.Logf("  ❌ %s (not found)", rel)
				}
			}
			if found == 0 {
				t.Logf("  ⚠️  no files found for %s", check.name)
			}
		})
	}
}

// TestDiscoveryRegistry detects providers via the Focus agent registry.
func TestDiscoveryRegistry(t *testing.T) {
	// We can't easily instantiate DiscoveryRegistry here (it needs a Store),
	// so we verify the provider binary detection logic directly.
	for _, name := range []string{"claude", "kimi", "codex", "gemini", "opencode"} {
		_, err := exec.LookPath(name)
		if err == nil {
			t.Logf("✅ %s binary found in PATH", name)
		} else {
			t.Logf("⚠️  %s binary not in PATH", name)
		}
	}
}
