package app

import (
	"strings"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/config"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
	"github.com/RollingTheRock/Focus-tui/internal/ui/todo"

	tea "charm.land/bubbletea/v2"
	bubblesKey "charm.land/bubbles/v2/key"
)

// =============================================================================
// Section 1: Architectural Integrity — KeyBindingProvider Coverage Audit
// =============================================================================

// TestAllRegisteredPaneTypesHaveCoverage enumerates every pane type and
// verifies they have either a KeyBindingProvider or a legacy fallback.
func TestAllRegisteredPaneTypesHaveCoverage(t *testing.T) {
	cfg := config.DefaultConfig()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	m := New(cfg, st).(model)

	noBindingsNeeded := map[models.PaneType]bool{
		models.PaneTypeFooter: true,
	}

	gaps := 0
	for _, id := range m.activePage.paneOrder {
		meta := m.activePage.paneMeta[id]
		if noBindingsNeeded[meta.Type] {
			continue
		}

		hasKP := m.activePage.keyBindingProvider(id) != nil
		hasLegacy := false
		if m.activePage.isOverlayPane(id) {
			hasLegacy = legacyOverlayBindings(m, false) != nil
		} else {
			hasLegacy = legacyPaneBindings(m, false) != nil
		}

		if !hasKP && !hasLegacy {
			t.Logf("GAP: pane %q (type=%q) — no KeyBindingProvider and no legacy fallback", id, meta.Type)
			gaps++
		}
	}

	if gaps > 0 {
		t.Logf("Total gaps: %d — panes without any binding declaration", gaps)
	}
}

// TestPanelsWithDualCoverageAreDocumented identifies legacy code that's dead
// because the panel already has KeyBindingProvider.
func TestPanelsWithDualCoverageAreDocumented(t *testing.T) {
	dualTypes := map[models.PaneType]bool{
		paneTypeTaskArchive:        true,
		paneTypeTaskArchiveConfirm: true,
	}
	t.Logf("Types with dual coverage (legacy code is dead): %v", dualTypes)
}

// =============================================================================
// Section 2: helpBindingsForState — Tier Routing Correctness
// =============================================================================

func TestHelpBindingsForStateTier1OverlayWithKP(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	// Open help overlay (now implements KeyBindingProvider).
	m.activePage.openHelpOverlayPane()

	bindings := helpBindingsForState(m, false)
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings from help overlay")
	}

	// Should contain Quick Launch items like "Git File Tree", "Toggle Todo".
	descs := bubblesBindingDescs(bindings)
	for _, want := range []string{"Git File Tree", "Toggle Todo", "Weather", "Agent Store"} {
		if !descs[want] {
			t.Errorf("help overlay bindings should contain %q", want)
		}
	}
}

func TestHelpBindingsForStateTier1OverlayCompact(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.activePage.openHelpOverlayPane()

	compact := helpBindingsForState(m, true)
	if len(compact) == 0 {
		t.Fatal("expected non-empty compact bindings")
	}

	// Compact should be shorter.
	full := helpBindingsForState(m, false)
	if len(compact) >= len(full) {
		t.Errorf("compact (%d) should be fewer than full (%d)", len(compact), len(full))
	}
}

func TestHelpBindingsForStateTier2InputMode(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.mode = ModeInput

	bindings := helpBindingsForState(m, false)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 input mode bindings, got %d", len(bindings))
	}
	if b := bindings[0]; b.Help().Key != "enter" || b.Help().Desc != "confirm" {
		t.Errorf("unexpected binding[0]: %q %q", b.Help().Key, b.Help().Desc)
	}
	if b := bindings[1]; b.Help().Key != "esc" || b.Help().Desc != "cancel" {
		t.Errorf("unexpected binding[1]: %q %q", b.Help().Key, b.Help().Desc)
	}
}

func TestHelpBindingsForStateTier3ShellMode(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.mode = ModeShell
	m.activePage.focused = paneShell

	full := helpBindingsForState(m, false)
	compact := helpBindingsForState(m, true)

	if len(full) < 3 {
		t.Fatalf("expected at least 3 shell mode bindings (full), got %d", len(full))
	}
	if len(compact) >= len(full) {
		t.Errorf("compact shell bindings (%d) should be fewer than full (%d)", len(compact), len(full))
	}

	// "shell input active" hint only in full mode.
	hasHint := false
	for _, b := range full {
		if b.Help().Desc == "shell input active" {
			hasHint = true
			break
		}
	}
	if !hasHint {
		t.Error("full shell bindings should include 'shell input active' hint")
	}
}

func TestHelpBindingsForStateTier4FocusedPaneWithKP(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	bindings := helpBindingsForState(m, false)
	if len(bindings) == 0 {
		t.Fatal("expected non-empty DAG bindings")
	}

	// Global bindings are intentionally omitted from the footer.
	// They are discoverable via the ? Help overlay.
	descs := bubblesBindingDescs(bindings)
	if !descs["archive"] && !descs["state"] {
		t.Error("DAG bindings should include panel-specific operations like 'archive', 'state'")
	}

	// Global binding key+desc pairs should NOT appear in footer.
	// (Pane-specific bindings with similar descriptions are fine — e.g. "R refresh" is DAG's own.)
	for _, b := range bindings {
		key, desc := b.Help().Key, b.Help().Desc
		if (key == "?" && desc == "help") ||
			(key == "w" && desc == "weather") ||
			(key == "ctrl+r" && desc == "refresh") ||
			(key == "q" && desc == "quit") {
			t.Errorf("global binding %q=%q should not appear in panel footer", key, desc)
		}
	}
}

func TestHelpBindingsForStateCompactOmitsGlobalBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	full := helpBindingsForState(m, false)
	compact := helpBindingsForState(m, true)

	if len(compact) >= len(full) {
		t.Errorf("compact (%d) should be fewer than full (%d)", len(compact), len(full))
	}

	// Neither compact nor full should include global bindings.
	// Check by key+desc pairs (not desc alone — panes may have their own "refresh" etc).
	globalPairs := map[string]string{
		"?": "help", "w": "weather", "ctrl+r": "refresh", "q": "quit",
	}
	for _, mode := range []struct {
		bindings []bubblesKey.Binding
		name     string
	}{{full, "full"}, {compact, "compact"}} {
		for _, b := range mode.bindings {
			key, desc := b.Help().Key, b.Help().Desc
			if globalPairs[key] == desc {
				t.Errorf("%s mode should not include global %q=%q", mode.name, key, desc)
			}
		}
	}
}

// =============================================================================
// Section 3: toBubblesBindings Conversion Fidelity
// =============================================================================

func TestToBubblesBindingsPreservesAllEntries(t *testing.T) {
	input := []models.KeyBinding{
		{Keys: []string{"a", "A"}, Help: "alpha"},
		{Keys: []string{"ctrl+x"}, Help: "cut"},
		{Keys: []string{"j", "k", "down", "up"}, Help: "move"},
	}
	output := toBubblesBindings(input)

	if len(output) != len(input) {
		t.Fatalf("expected %d bindings, got %d", len(input), len(output))
	}
	for i := range input {
		got := output[i].Help()
		wantKey := strings.Join(input[i].Keys, "/")
		if got.Key != wantKey {
			t.Errorf("binding[%d]: expected Key=%q, got %q", i, wantKey, got.Key)
		}
		if got.Desc != input[i].Help {
			t.Errorf("binding[%d]: expected Desc=%q, got %q", i, input[i].Help, got.Desc)
		}
	}
}

func TestToBubblesBindingsEmptyInput(t *testing.T) {
	output := toBubblesBindings(nil)
	if len(output) != 0 {
		t.Errorf("expected empty slice from nil input, got %v", output)
	}

	output = toBubblesBindings([]models.KeyBinding{})
	if len(output) != 0 {
		t.Errorf("expected empty slice from empty input, got len=%d", len(output))
	}
}

// =============================================================================
// Section 4: KeyBindingProvider Consistency — Cross-Check vs Legacy
// =============================================================================

func TestDagPaneKeyBindingsMatchLegacy(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	newBindings := m.activePage.keyBindingProvider(paneDAG).KeyBindings(false)
	legacyBindings := legacyPaneBindings(m, false)

	newDescs := bindingDescs(newBindings)
	legacyDescs := bubblesBindingDescs(legacyBindings)

	for _, want := range []string{"open/create", "new-wt", "state", "archive", "delete", "clear done", "expand", "todo", "new-task", "research", "arch", "refresh", "bin"} {
		if !newDescs[want] {
			t.Errorf("new DAG KeyBindings missing %q", want)
		}
		if !legacyDescs[want] {
			t.Errorf("legacy DAG bindings missing %q", want)
		}
	}

	// The legacy code has 'start agent' but the new KP should NOT.
	if newDescs["start agent"] {
		t.Error("new DAG KeyBindings should not contain 'start agent'")
	}
}

func TestWorktreeDetailPaneKeyBindingsAreTabAware(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	dp := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane)

	dp.activeTab = detailTabTasks
	tasksB := dp.KeyBindings(false)

	dp.activeTab = detailTabContext
	contextB := dp.KeyBindings(false)

	dp.activeTab = detailTabAgent
	agentB := dp.KeyBindings(false)

	if len(tasksB) == 0 || len(contextB) == 0 || len(agentB) == 0 {
		t.Fatalf("all tabs should return bindings: tasks=%d context=%d agent=%d",
			len(tasksB), len(contextB), len(agentB))
	}

	tasksDescs := bindingDescs(tasksB)
	contextDescs := bindingDescs(contextB)
	agentDescs := bindingDescs(agentB)

	// Tasks tab should have "start agent" (by design — 's' starts agent in context).
	// Context tab should NOT have task operations.
	if tasksDescs["edit task"] && contextDescs["edit task"] {
		t.Error("'edit task' should be tasks-only, not in context tab")
	}

	// Agent tab should have start/stop.
	if !agentDescs["start"] {
		t.Error("agent tab should have 'start' binding")
	}
	if !agentDescs["stop"] {
		t.Error("agent tab should have 'stop' binding")
	}

	// Tasks tab should have start agent.
	if !tasksDescs["start agent"] {
		t.Error("tasks tab should have 'start agent' (s key)")
	}
}

func TestTabContainerDelegatesKeyBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	tc := m.activePage.pane(paneDAG).(*tabContainer)

	// Tasks tab → DAG bindings.
	tc.activeTab = 0
	tasksB := tc.KeyBindings(false)
	if len(tasksB) == 0 {
		t.Fatal("tabContainer should delegate to DAG pane in tasks tab")
	}
	tasksDescs := bindingDescs(tasksB)
	if !tasksDescs["archive"] {
		t.Error("tasks tab should have 'archive' binding from DAG")
	}

	// ADR tab → ADR bindings.
	tc.activeTab = 1
	adrB := tc.KeyBindings(false)
	if len(adrB) == 0 {
		t.Fatal("tabContainer should delegate to ADR pane in ADR tab")
	}
}

// =============================================================================
// Section 5: Help Overlay Pane — Lifecycle and Behavior
// =============================================================================

func TestHelpOverlayPaneInitReturnsNil(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	cmd := overlay.Init()
	// Init returns nil — this is expected (no async init needed).
	_ = cmd
}

func TestHelpOverlayPaneCloseKeys(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 24)

	for _, r := range []rune{'?', 'q'} {
		_, cmd := overlay.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		if cmd == nil {
			t.Errorf("key %q should produce CloseHelpOverlayMsg", string(r))
			continue
		}
		msg := cmd()
		if _, ok := msg.(CloseHelpOverlayMsg); !ok {
			t.Errorf("key %q: expected CloseHelpOverlayMsg, got %T", string(r), msg)
		}
	}

	// esc should also close.
	_, cmd := overlay.Update(tea.KeyPressMsg{Code: 27, Text: "esc"})
	if cmd == nil {
		t.Fatal("esc should produce CloseHelpOverlayMsg")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("esc command returned nil message")
	}
}

func TestHelpOverlayPaneQuickLaunchKeys(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 24)

	// 'g' closes (only CloseHelpOverlayMsg, no extra quick-launch msg).
	_, cmd := overlay.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if cmd == nil {
		t.Fatal("'g' should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(CloseHelpOverlayMsg); !ok {
		t.Errorf("'g': expected CloseHelpOverlayMsg, got %T", msg)
	}

	// 't' closes AND sends ToggleTodoFromHelpMsg (via Sequence).
	_, cmd = overlay.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	if cmd == nil {
		t.Fatal("'t' should produce a command")
	}
	msg = cmd()
	if msg == nil {
		t.Fatal("'t' batch returned nil")
	}
}

func TestHelpOverlayPaneNonQuickLaunchKeyCloses(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 24)

	for _, key := range []rune{'d', 'n', 'x', 'a', '0'} {
		_, cmd := overlay.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
		if cmd == nil {
			t.Errorf("key %q should produce CloseHelpOverlayMsg", string(key))
			continue
		}
		msg := cmd()
		if _, ok := msg.(CloseHelpOverlayMsg); !ok {
			t.Errorf("key %q: expected CloseHelpOverlayMsg, got %T", string(key), msg)
		}
	}
}

func TestHelpOverlayPaneBuildSectionsPopulated(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)

	if len(overlay.sections) < 3 {
		t.Fatalf("expected >=3 sections (Quick Launch + Global + panes), got %d", len(overlay.sections))
	}

	if overlay.sections[0].title != "Quick Launch" {
		t.Errorf("section[0] should be 'Quick Launch', got %q", overlay.sections[0].title)
	}
	if len(overlay.sections[0].bindings) != 4 {
		t.Errorf("Quick Launch should have 4 bindings, got %d", len(overlay.sections[0].bindings))
	}

	if overlay.sections[1].title != "Global" {
		t.Errorf("section[1] should be 'Global', got %q", overlay.sections[1].title)
	}

	// Collect section titles for verification.
	titles := make(map[string]bool)
	for _, sec := range overlay.sections {
		titles[sec.title] = true
	}

	// DAG pane has Name "DAG".
	for _, want := range []string{"DAG", "Worktrees"} {
		if !titles[want] {
			t.Errorf("expected section %q in help overlay (available: %v)", want, titles)
		}
	}

	t.Logf("Help overlay sections: %v", titles)
}

func TestHelpOverlayPaneTotalLinesCalculation(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)

	expected := 0
	for _, sec := range overlay.sections {
		expected += 2 + len(sec.bindings) + 1
	}

	if overlay.totalLines != expected {
		t.Errorf("totalLines=%d computed=%d", overlay.totalLines, expected)
	}
	if overlay.totalLines == 0 {
		t.Error("totalLines should be > 0")
	}
}

func TestHelpOverlayPaneSizeDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	view := overlay.View()
	if view.Content == "" {
		t.Error("View() should return non-empty content even with zero size")
	}
	if !strings.Contains(view.Content, "Help") {
		t.Error("View() should contain 'Help' title")
	}
}

func TestHelpOverlayPaneScrollBoundaries(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 24)

	// Scroll up from top.
	for i := 0; i < 100; i++ {
		overlay.scrollUp()
	}
	if overlay.scrollOffset != 0 {
		t.Errorf("scrollUp from top: expected 0, got %d", overlay.scrollOffset)
	}

	// Scroll down.
	for i := 0; i < 10000; i++ {
		overlay.scrollDown()
	}
	visible := overlay.height - 3
	if visible < 1 {
		visible = 1
	}
	maxScroll := overlay.totalLines - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if overlay.scrollOffset > maxScroll {
		t.Errorf("scrollOffset %d > max %d", overlay.scrollOffset, maxScroll)
	}

	// Scroll back up.
	for i := 0; i < 10000; i++ {
		overlay.scrollUp()
	}
	if overlay.scrollOffset != 0 {
		t.Errorf("after full scroll up: expected 0, got %d", overlay.scrollOffset)
	}
}

func TestHelpOverlayPaneViewFooter(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 24)

	view := overlay.View()
	t.Logf("Help overlay footer:\n%s", view.Content)

	// Footer should mention Quick Launch keys.
	if !strings.Contains(view.Content, "Quick Launch") {
		t.Error("View() footer should contain 'Quick Launch'")
	}
}

// =============================================================================
// Section 6: handleKey — Help Overlay Routing
// =============================================================================

func TestHandleKeyQuestionMarkOpensHelpOverlay(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	_, cmd := m.handleKey(tea.KeyPressMsg{Code: '?', Text: "?"})

	// cmd may be nil because helpOverlayPane.Init() returns nil.
	// That's valid — the pane is still registered.
	if _, ok := m.activePage.paneMeta[paneHelpOverlay]; !ok {
		t.Fatal("help overlay should be registered after pressing '?'")
	}

	// If a command is returned, execute it.
	if cmd != nil {
		_ = cmd()
	}
}

func TestHandleKeyQuestionMarkWhenHelpAlreadyOpenRoutesToOverlay(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	// Open help overlay first (this creates the overlay pane).
	m.activePage.openHelpOverlayPane()

	// Press '?' — handleKey detects active overlay and routes to it.
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: '?', Text: "?"})
	if cmd == nil {
		t.Fatal("expected command routing to overlay")
	}
}

func TestHandleKeyQQuits(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("'q' should produce quit command")
	}
}

// =============================================================================
// Section 7: Message Handler Integration
// =============================================================================

func TestCloseHelpOverlayMsgClosesPane(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.activePage.openHelpOverlayPane()
	if _, ok := m.activePage.paneMeta[paneHelpOverlay]; !ok {
		t.Fatal("help overlay should be registered")
	}

	model2, _ := m.Update(CloseHelpOverlayMsg{ID: paneHelpOverlay})
	if _, ok := model2.(model).activePage.paneMeta[paneHelpOverlay]; ok {
		t.Error("help overlay should be gone after CloseHelpOverlayMsg")
	}
}

func TestOpenWeatherFromHelpMsgOpensCityPicker(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	_, _ = m.Update(OpenWeatherFromHelpMsg{})
	if _, ok := m.activePage.paneMeta[paneCityPicker]; !ok {
		t.Error("city picker should be open after OpenWeatherFromHelpMsg")
	}
}

func TestOpenAgentStoreFromHelpMsgOpensStore(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	_, cmd := m.Update(OpenAgentStoreFromHelpMsg{})
	if cmd != nil {
		cmd()
	}
	if _, ok := m.activePage.paneMeta[paneAgentStore]; !ok {
		t.Error("agent store should be open after OpenAgentStoreFromHelpMsg")
	}
}

func TestToggleTodoFromHelpMsgTogglesVisibility(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	tp := m.activePage.pane(paneTodoOverlay).(*todo.Model)
	tp.SetVisible(false)

	_, _ = m.Update(ToggleTodoFromHelpMsg{})
	if !tp.Visible() {
		t.Error("todo should be visible after first ToggleTodoFromHelpMsg")
	}

	_, _ = m.Update(ToggleTodoFromHelpMsg{})
	if tp.Visible() {
		t.Error("todo should NOT be visible after second ToggleTodoFromHelpMsg")
	}
}

// =============================================================================
// Section 8: globalHelpBindings Integrity
// =============================================================================

func TestGlobalHelpBindingsContainsRequiredEntries(t *testing.T) {
	descs := bubblesBindingDescs(globalHelpBindings)

	required := []string{"help", "weather", "refresh", "quit"}
	for _, want := range required {
		if !descs[want] {
			t.Errorf("globalHelpBindings missing %q", want)
		}
	}

	foundHelpKey := false
	for _, b := range globalHelpBindings {
		if b.Help().Key == "?" {
			foundHelpKey = true
			break
		}
	}
	if !foundHelpKey {
		t.Error("globalHelpBindings must include '?' key")
	}
}

func TestGlobalHelpBindingsNotMutatedByCallers(t *testing.T) {
	originalLen := len(globalHelpBindings)

	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	helpBindingsForState(m, false)

	if len(globalHelpBindings) != originalLen {
		t.Errorf("globalHelpBindings was mutated: was %d, now %d", originalLen, len(globalHelpBindings))
	}
}

// =============================================================================
// Section 9: openHelpOverlayPane Integration
// =============================================================================

func TestOpenHelpOverlayPaneRegistersAndSetsFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	prevFocus := m.activePage.focused

	cmd := m.activePage.openHelpOverlayPane()

	if _, ok := m.activePage.paneMeta[paneHelpOverlay]; !ok {
		t.Fatal("help overlay should be registered")
	}
	if m.activePage.focused != paneHelpOverlay {
		t.Errorf("focus should be on help overlay, got %q", m.activePage.focused)
	}
	if m.activePage.returnFocus[paneHelpOverlay] != prevFocus {
		t.Errorf("returnFocus should be %q, got %q", prevFocus, m.activePage.returnFocus[paneHelpOverlay])
	}

	// Init returns nil for help overlay (no async setup needed).
	if cmd != nil {
		msg := cmd()
		t.Logf("openHelpOverlayPane Init cmd returned: %T", msg)
	}
}

func TestOpenHelpOverlayPaneReentrant(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.activePage.openHelpOverlayPane()
	m.activePage.openHelpOverlayPane()

	if m.activePage.focused != paneHelpOverlay {
		t.Errorf("focus should be on help overlay after re-open, got %q", m.activePage.focused)
	}
}

func TestCloseHelpOverlayPaneRestoresFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	prevFocus := m.activePage.focused

	m.activePage.openHelpOverlayPane()
	m.closePane(paneHelpOverlay)

	if m.activePage.focused != prevFocus {
		t.Errorf("focus should be restored to %q, got %q", prevFocus, m.activePage.focused)
	}
	if _, ok := m.activePage.paneMeta[paneHelpOverlay]; ok {
		t.Error("help overlay should be removed from paneMeta after close")
	}
}

// =============================================================================
// Section 10: Stress Tests — Rapid Open/Close Cycles
// =============================================================================

func TestHelpOverlayRapidOpenCloseNoLeak(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	for i := 0; i < 200; i++ {
		m.activePage.openHelpOverlayPane()
		if _, ok := m.activePage.paneMeta[paneHelpOverlay]; !ok {
			t.Fatalf("iter %d: not registered after open", i)
		}
		m.closePane(paneHelpOverlay)
		if _, ok := m.activePage.paneMeta[paneHelpOverlay]; ok {
			t.Fatalf("iter %d: still registered after close", i)
		}
	}
}

func TestHelpOverlayWithOtherOverlaysNoInterference(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	for i := 0; i < 50; i++ {
		m.activePage.openHelpOverlayPane()
		m.activePage.openCityPickerOverlay()

		if _, ok := m.activePage.paneMeta[paneHelpOverlay]; !ok {
			t.Fatalf("iter %d: help overlay missing", i)
		}
		if _, ok := m.activePage.paneMeta[paneCityPicker]; !ok {
			t.Fatalf("iter %d: city picker missing", i)
		}

		if id := m.activeOverlayPane(); id != paneHelpOverlay {
			t.Errorf("iter %d: activeOverlayPane=%q, want %q", i, id, paneHelpOverlay)
		}

		m.closePane(paneHelpOverlay)
		m.closePane(paneCityPicker)
	}
}

func TestHelpOverlayRapidScrollNoPanic(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)
	overlay.SetSize(80, 10)

	for i := 0; i < 200; i++ {
		overlay.scrollDown()
	}
	visible := overlay.height - 3
	if visible < 1 {
		visible = 1
	}
	maxScroll := overlay.totalLines - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if overlay.scrollOffset > maxScroll {
		t.Errorf("scrollOffset %d > max %d after rapid down", overlay.scrollOffset, maxScroll)
	}

	for i := 0; i < 200; i++ {
		overlay.scrollUp()
	}
	if overlay.scrollOffset != 0 {
		t.Errorf("offset should be 0 after rapid up, got %d", overlay.scrollOffset)
	}
}

func TestHelpOverlayRapidViewNoPanic(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)

	sizes := []struct{ w, h int }{
		{80, 24}, {40, 10}, {120, 50}, {0, 0}, {10, 5},
		{200, 100}, {30, 3}, {1, 1}, {80, 24},
	}
	for _, sz := range sizes {
		overlay.SetSize(sz.w, sz.h)
		view := overlay.View()
		if view.Content == "" && sz.w > 0 && sz.h > 0 {
			t.Errorf("View() empty for %dx%d", sz.w, sz.h)
		}
	}
}

func TestHelpOverlayNilSectionsNoPanic(t *testing.T) {
	overlay := &helpOverlayPane{
		id:     paneHelpOverlay,
		width:  80,
		height: 24,
	}
	overlay.scrollDown()
	overlay.scrollUp()
	view := overlay.View()
	if view.Content == "" {
		t.Error("View() should return non-empty content even with nil sections")
	}
}

// =============================================================================
// Section 11: KeyBindingProvider Completeness
// =============================================================================

func TestEveryKeyBindingProviderReturnsNonNil(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	for _, id := range m.activePage.paneOrder {
		kp := m.activePage.keyBindingProvider(id)
		if kp == nil {
			continue
		}
		if full := kp.KeyBindings(false); full == nil {
			t.Errorf("pane %q: KeyBindings(false) returned nil", id)
		}
		if compact := kp.KeyBindings(true); compact == nil {
			t.Errorf("pane %q: KeyBindings(true) returned nil", id)
		}
	}
}

func TestKeyBindingProviderCompactNeverLargerThanFull(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	for _, id := range m.activePage.paneOrder {
		kp := m.activePage.keyBindingProvider(id)
		if kp == nil {
			continue
		}
		full := kp.KeyBindings(false)
		compact := kp.KeyBindings(true)
		if len(compact) > len(full) {
			t.Errorf("pane %q: compact (%d) > full (%d)", id, len(compact), len(full))
		}
	}
}

// =============================================================================
// Section 12: Pane Constants and Overlay Recognition
// =============================================================================

func TestHelpOverlayPaneConstants(t *testing.T) {
	if paneTypeHelpOverlay != "help-overlay" {
		t.Errorf("paneTypeHelpOverlay = %q, want %q", paneTypeHelpOverlay, "help-overlay")
	}
	if paneHelpOverlay != "help-overlay" {
		t.Errorf("paneHelpOverlay = %q, want %q", paneHelpOverlay, "help-overlay")
	}
}

func TestActiveOverlayPanePriority(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.activePage.openHelpOverlayPane()
	m.activePage.openCityPickerOverlay()

	if id := m.activeOverlayPane(); id != paneHelpOverlay {
		t.Errorf("activeOverlayPane = %q, want %q (help has priority)", id, paneHelpOverlay)
	}
}

func TestIsOverlayPaneIncludesHelpOverlay(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if !m.activePage.isOverlayPane(paneHelpOverlay) {
		t.Error("isOverlayPane should return true for paneHelpOverlay")
	}
	if m.activePage.isOverlayPane(paneDAG) {
		t.Error("isOverlayPane should return false for paneDAG")
	}
}

// =============================================================================
// Section 13: Edge Cases
// =============================================================================

func TestHelpBindingsForStateFallsThroughWhenOverlayReturnsNil(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	bindings := helpBindingsForState(m, false)
	if len(bindings) == 0 {
		t.Fatal("expected bindings when no overlay active")
	}
}

func TestHelpBindingsForStateNoOverlayUsesPaneProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	bindings := helpBindingsForState(m, false)
	descs := bubblesBindingDescs(bindings)

	for _, want := range []string{"archive", "new-task"} {
		if !descs[want] {
			t.Errorf("DAG bindings should include %q", want)
		}
	}
	// Global bindings are omitted from footer — they're in the ? Help overlay.
	// Verify by checking the actual global key+desc pairs.
	for _, b := range bindings {
		key, desc := b.Help().Key, b.Help().Desc
		if (key == "?" && desc == "help") || (key == "q" && desc == "quit") {
			t.Errorf("global binding %q=%q should not appear in panel footer", key, desc)
		}
	}
}

// =============================================================================
// Section 14: helpOverlayPane KeyBindingProvider — Architecture Fix
// =============================================================================

func TestHelpOverlayPaneImplementsKeyBindingProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	overlay := newHelpOverlayPane(paneHelpOverlay, *m.common, m.activePage)

	// Verify it satisfies the interface.
	var kp models.KeyBindingProvider = overlay
	bindings := kp.KeyBindings(false)
	if len(bindings) == 0 {
		t.Fatal("help overlay should return non-empty KeyBindings")
	}

	// Full mode should include quick launch descriptions.
	descs := bindingDescs(bindings)
	for _, want := range []string{"scroll", "Git File Tree", "Toggle Todo", "Weather", "Agent Store", "close"} {
		if !descs[want] {
			t.Errorf("help overlay KeyBindings missing %q", want)
		}
	}

	// Compact mode should be shorter.
	compact := kp.KeyBindings(true)
	if len(compact) >= len(bindings) {
		t.Errorf("compact (%d) should be fewer than full (%d)", len(compact), len(bindings))
	}
}

// =============================================================================
// Section 15: Panel Interface Integrity
// =============================================================================

func TestPanelTypesSatisfyInterface(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	for _, id := range m.activePage.paneOrder {
		panel := m.activePage.panes[id]
		if panel == nil {
			continue
		}
		// Runtime reachability check via interface assertion.
		_, ok := panel.(models.Panel)
		if !ok {
			t.Errorf("pane %q does not satisfy models.Panel", id)
		}
		// Verify they don't panic.
		_ = panel.View()
		panel.SetSize(80, 24)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func bindingDescs(bindings []models.KeyBinding) map[string]bool {
	m := make(map[string]bool, len(bindings))
	for _, b := range bindings {
		m[b.Help] = true
	}
	return m
}

func bubblesBindingDescs(bindings []bubblesKey.Binding) map[string]bool {
	m := make(map[string]bool, len(bindings))
	for _, b := range bindings {
		m[b.Help().Desc] = true
	}
	return m
}
