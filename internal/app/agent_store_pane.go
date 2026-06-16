package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	appstyles "github.com/RollingTheRock/Focus-tui/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// OpenAgentStoreMsg opens the agent store overlay.
type OpenAgentStoreMsg struct{}

// CloseAgentStoreMsg closes the agent store overlay.
type CloseAgentStoreMsg struct{}

// AgentStoreToggleEnabledMsg signals that an agent's enabled state was toggled.
type AgentStoreToggleEnabledMsg struct {
	AgentID string
}

// AgentStoreAgentSelectedMsg signals an agent was chosen for launch.
type AgentStoreAgentSelectedMsg struct {
	AgentID    string
	WorktreeID string
}

// AgentStoreOpenHintMsg requests the install hint overlay.
type AgentStoreOpenHintMsg struct {
	AgentID string
}

// OpenAgentRegisterPaneMsg opens the custom agent registration overlay.
type OpenAgentRegisterPaneMsg struct{}

type agentStoreTab string

const (
	storeTabAll        agentStoreTab = "all"
	storeTabInstalled  agentStoreTab = "installed"
	storeTabCoding     agentStoreTab = "coding"
	storeTabDevOps     agentStoreTab = "devops"
	storeTabAutomation agentStoreTab = "automation"
)

var storeTabs = []agentStoreTab{
	storeTabAll,
	storeTabInstalled,
	storeTabCoding,
	storeTabDevOps,
	storeTabAutomation,
}

type agentStorePane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	width   int
	height  int
	tab     agentStoreTab
	cursor  int
	items   []models.AgentDefinition
	loading bool
	errMsg  string
}

func newAgentStorePane(id models.PaneID, meta models.PaneMeta, common models.CommonModel) *agentStorePane {
	p := &agentStorePane{
		id:     id,
		meta:   meta,
		common: common,
		tab:    storeTabAll,
	}
	p.reload()
	return p
}

func (p *agentStorePane) reload() {
	p.loading = true
	p.errMsg = ""
	defs, err := p.common.Store.ListAgentDefinitions()
	if err != nil {
		p.errMsg = err.Error()
		p.items = nil
	} else {
		p.items = defs
	}
	p.loading = false
	p.cursor = 0
	p.clampCursor()
}

func (p *agentStorePane) Init() tea.Cmd {
	return nil
}

func (p *agentStorePane) KeyBindings(compact bool) []models.KeyBinding {
	if compact {
		return []models.KeyBinding{
			{Keys: []string{"j", "k"}, Help: "nav"},
			{Keys: []string{"enter"}, Help: "toggle"},
			{Keys: []string{"tab"}, Help: "tabs"},
			{Keys: []string{"q", "esc"}, Help: "close"},
		}
	}
	return []models.KeyBinding{
		{Keys: []string{"j", "k"}, Help: "nav"},
		{Keys: []string{"enter"}, Help: "toggle/install"},
		{Keys: []string{"i"}, Help: "install hint"},
		{Keys: []string{"tab"}, Help: "tabs"},
		{Keys: []string{"d"}, Help: "delete"},
		{Keys: []string{"a"}, Help: "register"},
		{Keys: []string{"r"}, Help: "reload"},
		{Keys: []string{"q", "esc"}, Help: "close"},
	}
}

func (p *agentStorePane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return p, func() tea.Msg { return CloseAgentStoreMsg{} }
		case "s":
			return p, func() tea.Msg { return CloseAgentStoreMsg{} }
		case "j", "down":
			if p.cursor < p.filteredCount()-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "tab":
			p.nextTab()
		case "shift+tab":
			p.prevTab()
		case "enter":
			if def := p.currentDef(); def != nil {
				if def.IsInstalled {
					return p, func() tea.Msg {
						return AgentStoreToggleEnabledMsg{AgentID: def.ID}
					}
				}
				return p, func() tea.Msg {
					return AgentStoreOpenHintMsg{AgentID: def.ID}
				}
			}
		case "i":
			if def := p.currentDef(); def != nil {
				return p, func() tea.Msg {
					return AgentStoreOpenHintMsg{AgentID: def.ID}
				}
			}
		case "a":
			return p, func() tea.Msg { return OpenAgentRegisterPaneMsg{} }
		case "r":
			p.reload()
		case "d":
			if def := p.currentDef(); def != nil && def.Category == "registered" {
				_ = p.common.Store.DeleteAgentDefinition(def.ID)
				p.reload()
			}
		}
	case AgentStoreToggleEnabledMsg:
		_ = p.common.Store.ToggleAgentDefinitionEnabled(msg.AgentID)
		p.reload()
	}
	return p, nil
}

func (p *agentStorePane) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 60
	}
	h := p.height
	if h <= 0 {
		h = 24
	}

	var b strings.Builder

	// Title.
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	b.WriteString(titleStyle.Render("Agent Store"))
	b.WriteByte('\n')

	// Tabs.
	b.WriteString(p.renderTabs(w))
	b.WriteByte('\n')

	if p.loading {
		b.WriteString(appstyles.StyleCache.MaxWidth(w).Render("Loading..."))
	} else if p.errMsg != "" {
		b.WriteString(appstyles.StyleCache.MaxWidth(w).Render("Error: " + p.errMsg))
	} else if len(p.items) == 0 {
		b.WriteString(appstyles.StyleCache.MaxWidth(w).Render("No agents registered."))
	} else {
		b.WriteString(p.renderList(w, h-4))
	}

	// Footer help — dynamic based on the current agent.
	b.WriteByte('\n')
	hintStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	b.WriteString(hintStyle.Render(p.renderHelp()))

	result := b.String()
	lines := strings.Split(result, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *agentStorePane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// ---- internal helpers ----

func (p *agentStorePane) nextTab() {
	for i, t := range storeTabs {
		if t == p.tab {
			p.tab = storeTabs[(i+1)%len(storeTabs)]
			p.cursor = 0
			return
		}
	}
}

func (p *agentStorePane) prevTab() {
	for i, t := range storeTabs {
		if t == p.tab {
			p.tab = storeTabs[(i-1+len(storeTabs))%len(storeTabs)]
			p.cursor = 0
			return
		}
	}
}

func (p *agentStorePane) filteredItems() []models.AgentDefinition {
	var out []models.AgentDefinition
	for _, def := range p.items {
		if p.matchesTab(def) {
			out = append(out, def)
		}
	}
	// Sort: enabled first, then installed, then by name.
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsEnabled != out[j].IsEnabled {
			return out[i].IsEnabled
		}
		if out[i].IsInstalled != out[j].IsInstalled {
			return out[i].IsInstalled
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (p *agentStorePane) matchesTab(def models.AgentDefinition) bool {
	switch p.tab {
	case storeTabAll:
		return true
	case storeTabInstalled:
		return def.IsInstalled
	case storeTabCoding:
		return hasTag(def.Tags, "coding") || hasTag(def.Tags, "pair-programming") || hasTag(def.Tags, "architecture")
	case storeTabDevOps:
		return hasTag(def.Tags, "devops") || hasTag(def.Tags, "mcp-native") || hasTag(def.Tags, "local")
	case storeTabAutomation:
		return hasTag(def.Tags, "automation") || hasTag(def.Tags, "tui") || hasTag(def.Tags, "multi-model") || hasTag(def.Tags, "open-weights")
	}
	return true
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

func (p *agentStorePane) filteredCount() int {
	return len(p.filteredItems())
}

func (p *agentStorePane) currentDef() *models.AgentDefinition {
	items := p.filteredItems()
	if p.cursor < 0 || p.cursor >= len(items) {
		return nil
	}
	return &items[p.cursor]
}

func (p *agentStorePane) clampCursor() {
	if n := p.filteredCount(); p.cursor >= n {
		if n > 0 {
			p.cursor = n - 1
		} else {
			p.cursor = 0
		}
	}
}

func (p *agentStorePane) renderTabs(w int) string {
	var parts []string
	for _, t := range storeTabs {
		label := string(t)
		if t == p.tab {
			parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent).Render("["+label+"]"))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(appstyles.Subtle).Render(label))
		}
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(strings.Join(parts, "  "))
}

func (p *agentStorePane) renderList(w, maxH int) string {
	items := p.filteredItems()
	if len(items) == 0 {
		return lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("No agents match this filter.")
	}

	var sections []string
	enabled := filterDefs(items, func(d models.AgentDefinition) bool { return d.IsInstalled && d.IsEnabled })
	available := filterDefs(items, func(d models.AgentDefinition) bool { return !(d.IsInstalled && d.IsEnabled) })

	if len(enabled) > 0 {
		sections = append(sections, p.renderSection(w, "Enabled", enabled))
	}
	if len(available) > 0 {
		sections = append(sections, p.renderSection(w, "Available", available))
	}

	result := strings.Join(sections, "\n")
	lines := strings.Split(result, "\n")
	if len(lines) > maxH {
		lines = lines[:maxH]
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  ..."))
	}
	return strings.Join(lines, "\n")
}

func (p *agentStorePane) renderSection(w int, title string, defs []models.AgentDefinition) string {
	var b strings.Builder
	sectionStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	b.WriteString(sectionStyle.Render(title))
	b.WriteByte('\n')

	items := p.filteredItems()
	for _, def := range defs {
		idx := -1
		for i, it := range items {
			if it.ID == def.ID {
				idx = i
				break
			}
		}
		isCursor := idx == p.cursor
		b.WriteString(p.renderDefLine(w, def, isCursor))
		b.WriteByte('\n')
	}
	return b.String()
}

func (p *agentStorePane) renderDefLine(w int, def models.AgentDefinition, isCursor bool) string {
	var status string
	if def.IsInstalled && def.IsEnabled {
		status = lipgloss.NewStyle().Foreground(appstyles.Success).Render("●")
	} else if def.IsInstalled && !def.IsEnabled {
		status = lipgloss.NewStyle().Foreground(appstyles.Warning).Render("○ disabled")
	} else {
		status = lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("○")
	}

	cursor := "  "
	if isCursor {
		cursor = "▸ "
	}

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(appstyles.Text)
	if isCursor {
		nameStyle = nameStyle.Background(appstyles.Highlight)
	}

	descStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	tagStyle := lipgloss.NewStyle().Foreground(appstyles.StateReady)

	tags := strings.Join(def.Tags, ", ")

	// Layout: cursor + status + name + desc + tags + action hint
	left := fmt.Sprintf("%s%s %s  %s", cursor, status, nameStyle.Render(def.Name), descStyle.Render(def.Description))
	if tags != "" {
		left += "  " + tagStyle.Render(tags)
	}

	// Right-aligned action hint for the current item.
	if isCursor {
		actionStyle := lipgloss.NewStyle().Foreground(appstyles.AccentDim)
		var hint string
		if !def.IsInstalled {
			hint = " [Enter/i]"
		} else if def.IsInstalled && !def.IsEnabled {
			hint = " [Enter]"
		} else if def.Category == "registered" {
			hint = " [Enter/d]"
		} else {
			hint = " [Enter]"
		}
		left += "  " + actionStyle.Render(hint)
	}

	return appstyles.StyleCache.MaxWidth(w).Render(left)
}

// renderHelp returns the footer hint line tailored to the currently selected agent.
func (p *agentStorePane) renderHelp() string {
	return ""
}

func filterDefs(items []models.AgentDefinition, fn func(models.AgentDefinition) bool) []models.AgentDefinition {
	var out []models.AgentDefinition
	for _, it := range items {
		if fn(it) {
			out = append(out, it)
		}
	}
	return out
}
