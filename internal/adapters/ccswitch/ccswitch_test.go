package ccswitch

import (
	"os/exec"
	"sort"
	"strings"
	"testing"

	"focus/internal/agents"
)

// ---- LaunchCommand -----------------------------------------------------------

func TestLaunchCommand_EmptyConfigID(t *testing.T) {
	for _, p := range []agents.Provider{
		agents.ProviderClaude,
		agents.ProviderCodex,
		agents.ProviderOpenCode,
		agents.ProviderGemini,
		agents.ProviderKimi,
		agents.ProviderGeneric,
	} {
		bin, args := LaunchCommand(p, "")
		if bin != "" || args != nil {
			t.Errorf("%s: expected empty when configID=\"\", got bin=%q args=%v", p, bin, args)
		}
	}
}

func TestLaunchCommand_Claude(t *testing.T) {
	bin, args := LaunchCommand(agents.ProviderClaude, "my-provider")
	if bin != "cc-switch" {
		t.Errorf("expected cc-switch, got %q", bin)
	}
	if len(args) != 3 || args[0] != "start" || args[1] != "claude" || args[2] != "my-provider" {
		t.Errorf("wrong args: %v", args)
	}
}

func TestLaunchCommand_Codex(t *testing.T) {
	bin, args := LaunchCommand(agents.ProviderCodex, "my-provider")
	if bin != "cc-switch" {
		t.Errorf("expected cc-switch, got %q", bin)
	}
	if len(args) != 3 || args[0] != "start" || args[1] != "codex" || args[2] != "my-provider" {
		t.Errorf("wrong args: %v", args)
	}
}

func TestLaunchCommand_GeminiAndOpenCode_NoOverride(t *testing.T) {
	// Gemini and OpenCode cannot be launched via cc-switch start.
	// LaunchCommand should return empty so the caller uses the default binary
	// and injects env vars separately via GetProviderEnvVars.
	for _, p := range []agents.Provider{agents.ProviderGemini, agents.ProviderOpenCode} {
		bin, args := LaunchCommand(p, "my-provider")
		if bin != "" || args != nil {
			t.Errorf("%s: expected empty (no cc-switch override), got bin=%q args=%v", p, bin, args)
		}
	}
}

func TestLaunchCommand_Unsupported(t *testing.T) {
	for _, p := range []agents.Provider{agents.ProviderKimi, agents.ProviderGeneric, agents.Provider("unknown")} {
		bin, args := LaunchCommand(p, "test")
		if bin != "" || args != nil {
			t.Errorf("%s: expected empty, got bin=%q args=%v", p, bin, args)
		}
	}
}

func TestLaunchCommand_CCSwitchProviders_IncludeConfigID(t *testing.T) {
	// Only Claude and Codex use cc-switch start.  The configID must be
	// passed as the last positional arg so cc-switch can identify the provider.
	tests := []struct {
		provider agents.Provider
		wantBin  string
	}{
		{agents.ProviderClaude, "cc-switch"},
		{agents.ProviderCodex, "cc-switch"},
	}
	for _, tt := range tests {
		bin, args := LaunchCommand(tt.provider, "x-config-id")
		if bin != tt.wantBin {
			t.Errorf("%s: bin=%q want=%q", tt.provider, bin, tt.wantBin)
		}
		if !containsStr(args, "x-config-id") {
			t.Errorf("%s: args %v missing configID", tt.provider, args)
		}
	}
}

// ---- toCCSwitchAppType -------------------------------------------------------

func TestToCCSwitchAppType(t *testing.T) {
	tests := []struct{ in, want string }{
		{"opencode", "open-code"},
		{"openclaw", "open-claw"},
		{"claude", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := toCCSwitchAppType(tt.in)
		if got != tt.want {
			t.Errorf("toCCSwitchAppType(%q)=%q want=%q", tt.in, got, tt.want)
		}
	}
}

// ---- envVars -----------------------------------------------------------------

func TestProviderEnvVars_NilSettings(t *testing.T) {
	p := Provider{SettingsConfig: nil}
	if v := p.envVars(); v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

func TestProviderEnvVars_EmptySettings(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{}}
	if v := p.envVars(); v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

func TestProviderEnvVars_NoEnvKey(t *testing.T) {
	// No env key AND no options key → nil
	p := Provider{SettingsConfig: map[string]any{
		"models": map[string]any{"gpt-5": map[string]any{"name": "GPT-5"}},
	}}
	if v := p.envVars(); v != nil {
		t.Errorf("expected nil when no 'env' or 'options' key, got %v", v)
	}
}

func TestProviderEnvVars_OptionsFallback(t *testing.T) {
	// OpenCode-style: options block maps apiKey → OPENAI_API_KEY etc.
	p := Provider{SettingsConfig: map[string]any{
		"options": map[string]any{
			"apiKey":  "sk-test123",
			"baseURL": "https://api.example.com/v1",
		},
	}}
	got := p.envVars()
	if len(got) != 2 {
		t.Fatalf("expected 2 vars from options, got %d: %v", len(got), got)
	}
	sort.Strings(got)
	if got[0] != "OPENAI_API_KEY=sk-test123" {
		t.Errorf("var 0: %q", got[0])
	}
	if got[1] != "OPENAI_BASE_URL=https://api.example.com/v1" {
		t.Errorf("var 1: %q", got[1])
	}
}

func TestProviderEnvVars_OptionsOnlyKnownKeys(t *testing.T) {
	// Unknown option keys are silently skipped.
	p := Provider{SettingsConfig: map[string]any{
		"options": map[string]any{
			"apiKey":      "sk-a",
			"unknownKey":  "ignored",
			"setCacheKey": "true",
			"baseURL":     "https://x.com",
		},
	}}
	got := p.envVars()
	if len(got) != 2 {
		t.Fatalf("expected 2 mapped vars, got %d: %v", len(got), got)
	}
}

func TestProviderEnvVars_EnvTakesPriorityOverOptions(t *testing.T) {
	// When both env and options exist, env wins.
	p := Provider{SettingsConfig: map[string]any{
		"env": map[string]any{"GEMINI_API_KEY": "sk-gemini"},
		"options": map[string]any{
			"apiKey": "sk-openai",
		},
	}}
	got := p.envVars()
	if len(got) != 1 {
		t.Fatalf("expected 1 var from env (ignoring options), got %d: %v", len(got), got)
	}
	if got[0] != "GEMINI_API_KEY=sk-gemini" {
		t.Errorf("got %q", got[0])
	}
}

func TestProviderEnvVars_OptionsEmptyValuesSkipped(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{
		"options": map[string]any{
			"apiKey":  "",
			"baseURL": "https://x.com",
		},
	}}
	got := p.envVars()
	if len(got) != 1 {
		t.Fatalf("expected 1 var (empty apiKey skipped), got %d: %v", len(got), got)
	}
	if got[0] != "OPENAI_BASE_URL=https://x.com" {
		t.Errorf("got %q", got[0])
	}
}

func TestProviderEnvVars_EnvNotMap(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{
		"env": "not-a-map",
	}}
	if v := p.envVars(); v != nil {
		t.Errorf("expected nil when env is not map, got %v", v)
	}
}

func TestProviderEnvVars_ValidEnv(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{
		"env": map[string]any{
			"GEMINI_API_KEY":         "sk-abc123",
			"GOOGLE_GEMINI_BASE_URL": "https://api.example.com",
			"GEMINI_MODEL":           "gemini-pro",
		},
	}}
	got := p.envVars()
	if len(got) != 3 {
		t.Fatalf("expected 3 vars, got %d: %v", len(got), got)
	}
	// Sort for deterministic comparison since map iteration order is random.
	sort.Strings(got)
	if got[0] != "GEMINI_API_KEY=sk-abc123" {
		t.Errorf("var 0: %q", got[0])
	}
	if got[1] != "GEMINI_MODEL=gemini-pro" {
		t.Errorf("var 1: %q", got[1])
	}
	if got[2] != "GOOGLE_GEMINI_BASE_URL=https://api.example.com" {
		t.Errorf("var 2: %q", got[2])
	}
}

func TestProviderEnvVars_SkipsEmptyValues(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{
		"env": map[string]any{
			"SET":   "value",
			"EMPTY": "",
			"ZERO":  "0", // "0" is non-empty, should be included
		},
	}}
	got := p.envVars()
	if len(got) != 2 {
		t.Fatalf("expected 2 vars (empty skipped), got %d: %v", len(got), got)
	}
}

func TestProviderEnvVars_SkipsNonStringValues(t *testing.T) {
	p := Provider{SettingsConfig: map[string]any{
		"env": map[string]any{
			"STRING": "value",
			"NUMBER": 12345,
			"BOOL":   true,
		},
	}}
	got := p.envVars()
	if len(got) != 1 {
		t.Fatalf("expected 1 var (only string), got %d: %v", len(got), got)
	}
	if got[0] != "STRING=value" {
		t.Errorf("got %q", got[0])
	}
}

func TestProviderEnvVars_FuzzLikeValues(t *testing.T) {
	// Values that look like they might be valid but aren't.
	p := Provider{SettingsConfig: map[string]any{
		"env": map[string]any{
			"KEY_WITH_EQUALS":  "val=ue",
			"KEY_WITH_NEWLINE": "val\nue",
			"KEY_WITH_SPACES":  " val ",
			"UNICODE_KEY":      "välüe",
		},
	}}
	got := p.envVars()
	if len(got) != 4 {
		t.Fatalf("expected 4 vars, got %d", len(got))
	}
	// Verify values are passed through as-is.
	m := make(map[string]string, len(got))
	for _, v := range got {
		parts := strings.SplitN(v, "=", 2)
		m[parts[0]] = parts[1]
	}
	if m["KEY_WITH_EQUALS"] != "val=ue" {
		t.Errorf("equals in value mangled: %q", m["KEY_WITH_EQUALS"])
	}
}

// ---- GetProviderEnvVars (integration with real cc-switch) ---------------------

func skipRealCCSwitchInShortMode(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("real cc-switch integration test skipped in short mode")
	}
}

func TestGetProviderEnvVars_Gemini_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	vars := GetProviderEnvVars("gemini", "gemini-official")
	// gemini-official has empty env, so expect nil.
	if vars != nil {
		t.Logf("gemini-official env vars: %v", vars)
	}
}

func TestGetProviderEnvVars_OpenCode_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	for _, configID := range []string{"kimi-web", "xfusion", "xmapi"} {
		t.Run(configID, func(t *testing.T) {
			vars := GetProviderEnvVars("opencode", configID)
			if len(vars) == 0 {
				t.Fatal("expected env vars from options fallback, got none")
			}
			found := make(map[string]bool)
			for _, v := range vars {
				parts := strings.SplitN(v, "=", 2)
				found[parts[0]] = true
			}
			if !found["OPENAI_API_KEY"] {
				t.Errorf("OPENAI_API_KEY not found in %v", vars)
			}
			if !found["OPENAI_BASE_URL"] {
				t.Errorf("OPENAI_BASE_URL not found in %v", vars)
			}
		})
	}
}

func TestGetProviderEnvVars_Gemini_Cubence_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	vars := GetProviderEnvVars("gemini", "cubence")
	if len(vars) == 0 {
		t.Fatal("cubence gemini provider should have env vars, got none")
	}
	found := make(map[string]bool)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		found[parts[0]] = true
	}
	for _, key := range []string{"GEMINI_API_KEY", "GOOGLE_GEMINI_BASE_URL", "GEMINI_MODEL"} {
		if !found[key] {
			t.Errorf("expected env var %s not found in %v", key, vars)
		}
	}
}

func TestGetProviderEnvVars_NonexistentProvider(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	vars := GetProviderEnvVars("gemini", "this-provider-does-not-exist-zzz")
	if vars != nil {
		t.Errorf("expected nil for nonexistent provider, got %v", vars)
	}
}

func TestGetProviderEnvVars_NonexistentApp(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	vars := GetProviderEnvVars("nonexistent-app", "test")
	if vars != nil {
		t.Errorf("expected nil for nonexistent app, got %v", vars)
	}
}

func TestGetProviderEnvVars_UnsupportedApp(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	// "hermes" is a valid cc-switch app but Focus doesn't have it in ListProviders switch.
	vars := GetProviderEnvVars("hermes", "any")
	if vars != nil {
		t.Errorf("expected nil for unsupported app, got %v", vars)
	}
}

// ---- IsInstalled --------------------------------------------------------------

func TestIsInstalled(t *testing.T) {
	_, err := exec.LookPath("cc-switch")
	want := err == nil
	got := IsInstalled()
	if got != want {
		t.Errorf("IsInstalled=%v, LookPath cc-switch err=%v", got, err)
	}
}

// ---- ListProviders - real cc-switch integration -------------------------------

func TestListProviders_Claude_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	providers, err := ListProviders("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one Claude provider")
	}
	for _, p := range providers {
		if p.ID == "" {
			t.Error("provider has empty ID")
		}
		if p.Name == "" {
			t.Errorf("provider %s has empty Name", p.ID)
		}
		if p.AppType != "claude" {
			t.Errorf("provider %s: AppType=%q, want claude", p.ID, p.AppType)
		}
	}
}

func TestListProviders_Gemini_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	providers, err := ListProviders("gemini")
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one Gemini provider")
	}
	for _, p := range providers {
		if p.AppType != "gemini" {
			t.Errorf("provider %s: AppType=%q, want gemini", p.ID, p.AppType)
		}
	}
}

func TestListProviders_OpenCode_RealCCSwitch(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	providers, err := ListProviders("opencode")
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one OpenCode provider")
	}
}

func TestListProviders_AllAppTypes_RoundTrip(t *testing.T) {
	skipRealCCSwitchInShortMode(t)
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	for _, appType := range []string{"claude", "codex", "opencode", "gemini"} {
		t.Run(appType, func(t *testing.T) {
			providers, err := ListProviders(appType)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range providers {
				envVars := p.envVars()
				for _, ev := range envVars {
					if !strings.Contains(ev, "=") {
						t.Errorf("%s/%s: malformed env var %q", appType, p.ID, ev)
					}
				}
			}
		})
	}
}

func TestListProviders_UnsupportedApp(t *testing.T) {
	if !IsInstalled() {
		t.Skip("cc-switch not installed")
	}
	_, err := ListProviders("unsupported-app-xyz")
	if err == nil {
		t.Error("expected error for unsupported app")
	}
}

// ---- helpers -----------------------------------------------------------------

func containsStr(slice []string, s string) bool {
	for _, e := range slice {
		if e == s {
			return true
		}
	}
	return false
}
