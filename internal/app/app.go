package app

import (
	"fmt"
	"os"
	"strings"

	"focus/internal/avatar"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/styles"
	"focus/internal/ui/footer"
	"focus/internal/ui/header"
	"focus/internal/ui/layout"
	"focus/internal/ui/pomodoro"
	"focus/internal/ui/shell"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	paneHeader   models.PaneID = "header"
	paneShell    models.PaneID = "shell-main"
	paneTodo     models.PaneID = "todo-main"
	panePomodoro models.PaneID = "pomodoro-main"
	paneFooter   models.PaneID = "footer"

	splitRatioStep = 5
)

// avatarRenderedMsg carries the pre-rendered avatar string from chafa.
type avatarRenderedMsg struct {
	art string
}

// viewCache holds the cached View() output.
// Using a pointer so it survives value-receiver copies in Bubbletea.
type viewCache struct {
	output string
	gen    uint64
}

// model is the top-level Bubbletea model.
type model struct {
	common         *models.CommonModel
	state          AppState
	mode           AppMode
	overlay        OverlayKind
	avatarRendered string

	panes     map[models.PaneID]models.Panel
	paneMeta  map[models.PaneID]models.PaneMeta
	paneOrder []models.PaneID
	bodyTree  *layout.TreeNode
	frames    map[models.PaneID]models.PaneFrame
	focused   models.PaneID
	nextShell int

	viewGen uint64
	vc      *viewCache
}

// New creates and returns the initial application model.
func New(cfg config.Config, store models.Store) tea.Model {
	cm := &models.CommonModel{
		Theme: styles.DefaultTheme(),
		Cfg:   cfg,
		Store: store,
	}

	cwd, _ := os.Getwd()
	m := model{
		common:   cm,
		state:    StateDashboard,
		mode:     ModeNormal,
		overlay:  OverlayNone,
		panes:    make(map[models.PaneID]models.Panel),
		paneMeta: make(map[models.PaneID]models.PaneMeta),
		bodyTree: layout.Split(
			layout.SplitHorizontal,
			70,
			layout.Leaf(paneShell),
			layout.Split(layout.SplitVertical, 55, layout.Leaf(paneTodo), layout.Leaf(panePomodoro)),
		),
		focused:   paneShell,
		nextShell: 2,
		vc:        &viewCache{},
	}

	m.registerPane(paneHeader, header.New(cfg, cm.Theme), models.PaneMeta{ID: paneHeader, Name: "Header", Type: models.PaneTypeHeader, Status: models.PaneStatusPassive, Closable: false})
	m.registerPane(paneShell, shell.New(cm), models.PaneMeta{ID: paneShell, Name: "Shell", Type: models.PaneTypeShell, CWD: cwd, Status: models.PaneStatusReady, Closable: true})
	m.registerPane(paneTodo, todo.New(cm), models.PaneMeta{ID: paneTodo, Name: "Todo", Type: models.PaneTypeTodo, Status: models.PaneStatusIdle, Closable: false})
	m.registerPane(panePomodoro, pomodoro.New(cm), models.PaneMeta{ID: panePomodoro, Name: "Pomodoro", Type: models.PaneTypePomodoro, Status: models.PaneStatusIdle, Closable: false})
	m.registerPane(paneFooter, footer.New(cm), models.PaneMeta{ID: paneFooter, Name: "Footer", Type: models.PaneTypeFooter, Status: models.PaneStatusPassive, Closable: false})
	m.refreshPaneStatuses()

	return m
}

func (m *model) registerPane(id models.PaneID, panel models.Panel, meta models.PaneMeta) {
	m.panes[id] = panel
	m.paneMeta[id] = meta
	m.paneOrder = append(m.paneOrder, id)
}

func (m *model) pane(id models.PaneID) models.Panel {
	return m.panes[id]
}

func (m *model) setPane(id models.PaneID, panel models.Panel) {
	m.panes[id] = panel
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, id := range m.paneOrder {
		if cmd := m.pane(id).Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cfg := m.common.Cfg; cfg.Avatar.Image != "" {
		cmds = append(cmds, func() tea.Msg {
			art := avatar.Render(cfg.Avatar.Image, cfg.Avatar.Width)
			return avatarRenderedMsg{art: art}
		})
	}
	return tea.Batch(cmds...)
}

func (m *model) invalidateView() {
	m.viewGen++
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case avatarRenderedMsg:
		m.avatarRendered = msg.art
		m.invalidateView()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case pomodoro.PickerLoadedMsg:
		m.overlay = OverlayPicker
		m.setFocus(panePomodoro)
		m.invalidateView()
		return m.routeToPane(panePomodoro, msg)

	case todo.ModeChangeMsg:
		m.setFocus(paneTodo)
		if msg.InputActive {
			m.mode = ModeInput
		} else {
			m.mode = ModeNormal
		}
		m.refreshPaneStatuses()
		m.invalidateView()

	case pomodoro.SessionCompleteMsg, models.StatsRefreshMsg:
		m.invalidateView()
		if ft, ok := m.pane(paneFooter).(*footer.Model); ok {
			return m, ft.Refresh()
		}

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		m.updateSizes(msg.Width, msg.Height)
		m.invalidateView()
	}

	m.invalidateView()
	var cmds []tea.Cmd
	for _, id := range m.paneOrder {
		newPanel, cmd := m.pane(id).Update(msg)
		m.setPane(id, newPanel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()
	dims := layout.ComputeBanner(m.common.Width, m.common.Height)
	bodyY := msg.Y - dims.HeaderH
	bodyX := msg.X
	clicked := m.paneAt(bodyX, bodyY)
	if clicked != "" {
		m.setFocus(clicked)
	}
	if m.mode != ModeShell || clicked == "" || m.paneMeta[clicked].Type != models.PaneTypeShell {
		return m, nil
	}
	frame, ok := m.frames[clicked]
	if !ok {
		return m, nil
	}
	adjusted := msg
	adjusted.X = msg.X - frame.X - 2
	adjusted.Y = bodyY - frame.Y - 1
	contentW := frame.W - 4
	contentH := frame.H - 2
	if adjusted.X < 0 || adjusted.X >= contentW || adjusted.Y < 0 || adjusted.Y >= contentH {
		return m, nil
	}
	return m.routeToPane(clicked, tea.Msg(adjusted))
}

// handleKey routes keyboard input based on overlay, mode, and focused pane.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()
	pomo := m.pane(panePomodoro).(*pomodoro.Model)

	if m.overlay == OverlayPicker {
		if pomo.IsPickerActive() {
			updated, cmd := m.routeToPane(panePomodoro, msg)
			m = updated.(model)
			if !m.pane(panePomodoro).(*pomodoro.Model).IsPickerActive() {
				m.overlay = OverlayNone
			}
			return m, cmd
		}
		m.overlay = OverlayNone
		return m, nil
	}

	if m.mode == ModeInput {
		if msg.String() == "ctrl+c" {
			m.closeShellPanes()
			return m, tea.Quit
		}
		return m.routeToPane(paneTodo, msg)
	}

	if m.mode == ModeShell {
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			m.refreshPaneStatuses()
			return m, nil
		case "ctrl+t":
			m.mode = ModeNormal
			m.setFocus(paneTodo)
			return m, nil
		default:
			if m.paneMeta[m.focused].Type == models.PaneTypeShell {
				return m.routeToPane(m.focused, msg)
			}
			m.mode = ModeNormal
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.closeShellPanes()
		return m, tea.Quit
	case "ctrl+shift+left":
		return m.adjustFocusedSplit(layout.FocusLeft)
	case "ctrl+shift+right":
		return m.adjustFocusedSplit(layout.FocusRight)
	case "ctrl+shift+up":
		return m.adjustFocusedSplit(layout.FocusUp)
	case "ctrl+shift+down":
		return m.adjustFocusedSplit(layout.FocusDown)
	case "tab":
		m.focusCycle(1)
		return m, nil
	case "shift+tab":
		if m.paneMeta[m.focused].Type == models.PaneTypeTodo {
			return m.routeToPane(m.focused, msg)
		}
		m.focusCycle(-1)
		return m, nil
	case "ctrl+\\":
		return m.splitFocused(layout.SplitHorizontal)
	case "ctrl+-":
		return m.splitFocused(layout.SplitVertical)
	case "ctrl+w":
		return m.closeFocusedPane()
	case "ctrl+h":
		m.setFocus(layout.MoveFocus(m.focused, m.frames, layout.FocusLeft))
		return m, nil
	case "ctrl+l":
		m.setFocus(layout.MoveFocus(m.focused, m.frames, layout.FocusRight))
		return m, nil
	case "ctrl+k":
		m.setFocus(layout.MoveFocus(m.focused, m.frames, layout.FocusUp))
		return m, nil
	case "ctrl+j":
		m.setFocus(layout.MoveFocus(m.focused, m.frames, layout.FocusDown))
		return m, nil
	case "ctrl+t":
		m.setFocus(paneTodo)
		return m, nil
	case "enter":
		if m.paneMeta[m.focused].Type == models.PaneTypeShell {
			m.mode = ModeShell
			m.refreshPaneStatuses()
			return m, nil
		}
	}

	if m.focused != "" && m.paneMeta[m.focused].Type != models.PaneTypeShell {
		return m.routeToPane(m.focused, msg)
	}

	return m, nil
}

func (m *model) setPaneStatus(id models.PaneID, status models.PaneStatus) {
	meta := m.paneMeta[id]
	meta.Status = status
	m.paneMeta[id] = meta
}

func (m *model) setFocus(id models.PaneID) {
	if id == "" {
		return
	}
	if _, ok := m.paneMeta[id]; !ok {
		return
	}
	m.focused = id
	m.refreshPaneStatuses()
}

func (m *model) refreshPaneStatuses() {
	for id, meta := range m.paneMeta {
		switch meta.Type {
		case models.PaneTypeHeader, models.PaneTypeFooter:
			meta.Status = models.PaneStatusPassive
		case models.PaneTypeShell:
			if id == m.focused && m.mode == ModeShell {
				meta.Status = models.PaneStatusActive
			} else if id == m.focused {
				meta.Status = models.PaneStatusReady
			} else {
				meta.Status = models.PaneStatusIdle
			}
		default:
			if id == m.focused {
				meta.Status = models.PaneStatusReady
			} else {
				meta.Status = models.PaneStatusIdle
			}
		}
		m.paneMeta[id] = meta
	}
}

func (m *model) nextShellPaneID() models.PaneID {
	id := models.PaneID(fmt.Sprintf("shell-%d", m.nextShell))
	m.nextShell++
	return id
}

func (m *model) createShellPane() (models.PaneID, tea.Cmd) {
	id := m.nextShellPaneID()
	panel := shell.New(m.common)
	meta := models.PaneMeta{
		ID:       id,
		Name:     fmt.Sprintf("Shell %d", m.nextShell-1),
		Type:     models.PaneTypeShell,
		CWD:      m.currentCWD(),
		Status:   models.PaneStatusIdle,
		Closable: true,
	}
	m.registerPane(id, panel, meta)
	if frame, ok := m.frames[m.focused]; ok {
		contentW := max(frame.W-4, 8)
		contentH := max(frame.H-2, 3)
		panel.SetSize(contentW, contentH)
	}
	return id, panel.Init()
}

func (m *model) currentCWD() string {
	if meta, ok := m.paneMeta[m.focused]; ok && meta.CWD != "" {
		return meta.CWD
	}
	cwd, _ := os.Getwd()
	return cwd
}

func (m model) splitFocused(direction layout.SplitDirection) (tea.Model, tea.Cmd) {
	newID, cmd := m.createShellPane()
	m.bodyTree = layout.SplitLeaf(m.bodyTree, m.focused, newID, direction, true)
	m.setFocus(newID)
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, cmd
}

func (m model) adjustFocusedSplit(direction layout.FocusDirection) (tea.Model, tea.Cmd) {
	if m.focused == "" {
		return m, nil
	}

	splitDirection := layout.SplitHorizontal
	delta := splitRatioStep
	switch direction {
	case layout.FocusLeft:
		delta = -splitRatioStep
	case layout.FocusRight:
		delta = splitRatioStep
	case layout.FocusUp:
		splitDirection = layout.SplitVertical
		delta = -splitRatioStep
	case layout.FocusDown:
		splitDirection = layout.SplitVertical
		delta = splitRatioStep
	}

	if !layout.AdjustSplitRatio(m.bodyTree, m.bodyBounds(), m.focused, splitDirection, delta) {
		return m, nil
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m model) closeFocusedPane() (tea.Model, tea.Cmd) {
	meta, ok := m.paneMeta[m.focused]
	if !ok || !meta.Closable {
		return m, nil
	}
	order := layout.LeafOrder(m.bodyTree)
	if len(order) <= 1 {
		return m, nil
	}
	closing := m.focused
	if sh, ok := m.pane(closing).(*shell.Model); ok {
		_ = sh.Close()
	}
	m.bodyTree = layout.RemoveLeaf(m.bodyTree, closing)
	delete(m.panes, closing)
	delete(m.paneMeta, closing)
	m.removePaneOrder(closing)
	remaining := layout.LeafOrder(m.bodyTree)
	if len(remaining) > 0 {
		m.setFocus(remaining[0])
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m *model) removePaneOrder(id models.PaneID) {
	filtered := m.paneOrder[:0]
	for _, existing := range m.paneOrder {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	m.paneOrder = filtered
}

func (m *model) focusCycle(delta int) {
	order := layout.LeafOrder(m.bodyTree)
	if len(order) == 0 {
		return
	}
	idx := 0
	for i, id := range order {
		if id == m.focused {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(order)) % len(order)
	m.setFocus(order[idx])
}

func (m *model) paneAt(x, y int) models.PaneID {
	for _, id := range layout.LeafOrder(m.bodyTree) {
		frame, ok := m.frames[id]
		if !ok {
			continue
		}
		if x >= frame.X && x < frame.X+frame.W && y >= frame.Y && y < frame.Y+frame.H {
			return id
		}
	}
	return ""
}

func (m model) routeToPane(id models.PaneID, msg tea.Msg) (tea.Model, tea.Cmd) {
	panel := m.pane(id)
	if panel == nil {
		return m, nil
	}
	newPanel, cmd := panel.Update(msg)
	m.setPane(id, newPanel)
	return m, cmd
}

func (m *model) updateSizes(w, h int) {
	dims := layout.ComputeBanner(w, h)
	m.pane(paneHeader).SetSize(w, dims.HeaderH)
	m.pane(paneFooter).SetSize(w, 1)

	m.frames = layout.ComputeFrames(m.bodyTree, m.bodyBounds())
	for id, frame := range m.frames {
		contentW := frame.W - 4
		if contentW < 8 {
			contentW = 8
		}
		contentH := frame.H - 2
		if contentH < 3 {
			contentH = 3
		}
		m.pane(id).SetSize(contentW, contentH)
	}
}

func (m model) bodyBounds() models.PaneFrame {
	w := m.common.Width
	h := m.common.Height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	dims := layout.ComputeBanner(w, h)
	bodyHeight := h - dims.HeaderH - 1 - 1
	if bodyHeight < 6 {
		bodyHeight = 6
	}
	return models.PaneFrame{X: 0, Y: 0, W: w, H: bodyHeight}
}

// View implements tea.Model.
func (m model) View() string {
	w := m.common.Width
	h := m.common.Height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	dims := layout.ComputeBanner(w, h)

	var result string
	if m.vc.gen == m.viewGen && m.vc.output != "" {
		result = m.vc.output
	} else {
		result = m.buildView(dims, w, h)
		m.vc.output = result
		m.vc.gen = m.viewGen
	}

	if m.mode == ModeShell && m.paneMeta[m.focused].Type == models.PaneTypeShell {
		if sh, ok := m.pane(m.focused).(*shell.Model); ok {
			if frame, exists := m.frames[m.focused]; exists {
				if cx, cy, vis := sh.CursorPos(); vis {
					termRow := dims.HeaderH + frame.Y + 1 + cy + 1
					termCol := frame.X + 2 + cx + 1
					result += fmt.Sprintf("\033[?25h\033[%d;%dH", termRow, termCol)
				}
			}
		}
	}

	return result
}

func (m model) buildView(dims layout.Dimensions, w, h int) string {
	hdr := m.pane(paneHeader).(*header.Model)
	pomo := m.pane(panePomodoro).(*pomodoro.Model)

	var headerView string
	if dims.UseBanner {
		pomoTimer := ""
		pomoPhase := ""
		linkedTodo := ""
		if pomo.IsRunning() {
			pomoTimer = pomo.TimerDisplay()
			pomoPhase = pomo.PhaseLabel()
			linkedTodo = pomo.LinkedTodoText()
			if linkedTodo != "" {
				linkedTodo = "-> " + linkedTodo
			}
		}
		headerView = hdr.ViewBanner(w, pomoTimer, pomoPhase, linkedTodo)
	} else {
		headerView = hdr.ViewCompact(w, dims.ShowQuote)
	}

	bodyView := m.renderBody(w, h-dims.HeaderH-1-1)
	statsView := m.pane(paneFooter).View()
	helpLine := m.renderHelpLine(w)

	return lipgloss.JoinVertical(lipgloss.Left, headerView, bodyView, statsView, helpLine)
}

func (m model) renderBody(w, h int) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 6
	}
	base := blankCanvas(w, h)
	for _, id := range layout.LeafOrder(m.bodyTree) {
		frame, ok := m.frames[id]
		if !ok {
			continue
		}
		panel := m.pane(id)
		active := id == m.focused
		content := panel.View()
		title := m.renderPaneTitle(id)
		panelView := layout.RenderPanel(title, content, max(frame.W-4, 8), max(frame.H-2, 3), active)
		base = layout.OverlayOnBase(base, panelView, frame.X, frame.Y)
	}

	if m.overlay == OverlayPicker {
		if frame, ok := m.frames[panePomodoro]; ok {
			pomo := m.pane(panePomodoro).(*pomodoro.Model)
			overlayW := frame.W - 8
			if overlayW > 50 {
				overlayW = 50
			}
			if overlayW < 20 {
				overlayW = 20
			}
			overlayH := frame.H - 4
			if overlayH > 15 {
				overlayH = 15
			}
			if overlayH < 4 {
				overlayH = 4
			}
			overlayView := layout.RenderPanel("SELECT TASK", pomo.PickerView(), overlayW, overlayH, true)
			x := frame.X + (frame.W-(overlayW+4))/2
			y := frame.Y + (frame.H-(overlayH+2))/2
			base = layout.OverlayOnBase(base, overlayView, x, y)
		}
	}

	return base
}

func (m model) renderPaneTitle(id models.PaneID) string {
	meta := m.paneMeta[id]
	title := strings.ToUpper(meta.Name)
	if !meta.Closable && (meta.Type == models.PaneTypeTodo || meta.Type == models.PaneTypePomodoro) {
		title += " [fixed]"
	}
	if meta.CWD != "" && meta.Type == models.PaneTypeShell {
		title += " [" + meta.CWD + "]"
	}
	if id == m.focused {
		if m.mode == ModeShell && meta.Type == models.PaneTypeShell {
			title += " [active]"
		} else {
			title += " [focus]"
		}
	}
	return title
}

func (m model) renderHelpLine(w int) string {
	helpStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	pomo := m.pane(panePomodoro).(*pomodoro.Model)
	todoModel := m.pane(paneTodo).(*todo.Model)
	focusedType := m.paneMeta[m.focused].Type

	if m.overlay == OverlayPicker {
		return helpStyle.Render("  [enter]select  [esc]skip")
	}
	if m.mode == ModeInput {
		return helpStyle.Render("  [enter]confirm  [esc]cancel")
	}
	if m.mode == ModeShell {
		return helpStyle.Render("  [esc]normal  [ctrl+t]todo")
	}

	left := "[tab]next  [ctrl+h/j/k/l]focus  [enter]activate"
	switch focusedType {
	case models.PaneTypeTodo:
		if todoModel.IsConfirmingDelete() {
			left = "[j/k]move  [y/n]delete"
		} else {
			left = "[j/k]move  [a]dd  [e]dit  [d]el  [space]done  [shift+tab]list"
		}
	case models.PaneTypePomodoro:
		if pomo.CurrentPhase() == pomodoro.PhaseIdle {
			left = "[s]tart pomo"
		} else if pomo.IsPaused() {
			left = "[p]resume  [r]eset"
		} else {
			left = "[p]ause  [n]ext  [r]eset"
		}
	case models.PaneTypeShell:
		left = "[tab]next  [ctrl+h/j/k/l]focus  [enter]shell"
	}
	right := "[ctrl+\\/ctrl+-]split  [ctrl+shift+arrows]resize"
	if meta, ok := m.paneMeta[m.focused]; ok && meta.Closable {
		right += "  [ctrl+w]close"
	}
	right += "  [q]uit"
	return renderHelpBar(helpStyle, left, right, w)
}

func renderHelpBar(helpStyle lipgloss.Style, left, right string, w int) string {
	leftRendered := helpStyle.Render("  " + left)
	rightRendered := helpStyle.Render(right + "  ")
	gap := w - lipgloss.Width(leftRendered) - lipgloss.Width(rightRendered)
	if gap < 1 {
		gap = 1
	}
	return leftRendered + strings.Repeat(" ", gap) + rightRendered
}

func (m *model) closeShellPanes() {
	for id, meta := range m.paneMeta {
		if meta.Type != models.PaneTypeShell {
			continue
		}
		if sh, ok := m.pane(id).(*shell.Model); ok {
			_ = sh.Close()
		}
	}
}

func blankCanvas(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", w)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
