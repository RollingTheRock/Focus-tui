package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"focus/internal/adapters"
	"focus/internal/avatar"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/plugins"
	editorplugin "focus/internal/plugins/editor"
	filebrowser "focus/internal/plugins/filebrowser"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/styles"
	"focus/internal/ui/footer"
	"focus/internal/ui/header"
	"focus/internal/ui/layout"
	"focus/internal/ui/pomodoro"
	"focus/internal/ui/shell"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	paneHeader    models.PaneID = "header"
	paneShell     models.PaneID = "shell-main"
	paneGitStatus models.PaneID = "git-status-main"
	paneGitDiff   models.PaneID = "git-diff-pane"
	paneGitCommit models.PaneID = "git-commit-overlay"
	paneTodo      models.PaneID = "todo-main"
	paneFileTree  models.PaneID = "file-tree-main"
	panePomodoro  models.PaneID = "pomodoro-main"
	paneFooter    models.PaneID = "footer"

	paneTypeGitCommit models.PaneType = "git-commit"

	splitRatioStep = 5

	simplifiedHelpMaxWidth = 40
	hideFooterBelowHeight  = 10
	tinyWindowMinWidth     = 20
	tinyWindowMinHeight    = 5
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

	panes       map[models.PaneID]models.Panel
	paneMeta    map[models.PaneID]models.PaneMeta
	paneOrder   []models.PaneID
	bodyTree    *layout.TreeNode
	frames      map[models.PaneID]models.PaneFrame
	focused     models.PaneID
	nextShell   int
	nextEditor  int
	returnFocus map[models.PaneID]models.PaneID

	viewGen uint64
	vc      *viewCache

	zoomedPane  models.PaneID
	preZoomTree *layout.TreeNode

	overlayBaseFocus models.PaneID

	pluginRegistry *plugins.Registry
	adapterManager *adapters.Manager
}

type editorMetaProvider interface {
	FilePath() string
	Dirty() bool
	DisplayName() string
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
			30,
			layout.Split(layout.SplitVertical, 50, layout.Leaf(paneGitStatus), layout.Leaf(paneFileTree)),
			layout.Leaf(paneShell),
		),
		focused:        paneShell,
		nextShell:      2,
		nextEditor:     1,
		returnFocus:    make(map[models.PaneID]models.PaneID),
		vc:             &viewCache{},
		pluginRegistry: plugins.NewRegistry(),
		adapterManager: adapters.NewManager(),
	}

	gitAdapter := adapters.NewGitLocalAdapter()
	_ = m.adapterManager.Register(gitAdapter.Name(), gitAdapter)

	gitPaneMeta := models.PaneMeta{ID: paneGitStatus, Name: "Git Status", Type: models.PaneTypeGitStatus, CWD: cwd, Status: models.PaneStatusIdle, Closable: false}
	fileTreeMeta := models.PaneMeta{ID: paneFileTree, Name: "File Tree", Type: models.PaneTypeFileTree, CWD: cwd, Status: models.PaneStatusIdle, Closable: false}

	gitPlugin := gitplugin.New(m.adapterManager.Git())
	fileTreePlugin := filebrowser.New()
	editorPlugin := editorplugin.New()
	_ = m.pluginRegistry.Register(gitPlugin)
	_ = m.pluginRegistry.Register(fileTreePlugin)
	_ = m.pluginRegistry.Register(editorPlugin)

	m.registerPane(paneHeader, header.New(cfg, cm.Theme, store), models.PaneMeta{ID: paneHeader, Name: "Header", Type: models.PaneTypeHeader, Status: models.PaneStatusPassive, Closable: false})
	m.registerPane(paneShell, shell.New(cm, paneShell), models.PaneMeta{ID: paneShell, Name: "Shell", Type: models.PaneTypeShell, CWD: cwd, Status: models.PaneStatusStarting, Closable: true})
	m.registerPane(paneTodo, todo.New(cm), models.PaneMeta{ID: paneTodo, Name: "Todo", Type: models.PaneTypeTodo, Status: models.PaneStatusIdle, Closable: false})
	m.registerPane(panePomodoro, pomodoro.New(cm), models.PaneMeta{ID: panePomodoro, Name: "Pomodoro", Type: models.PaneTypePomodoro, Status: models.PaneStatusIdle, Closable: false})
	m.registerPane(paneFooter, footer.New(cm), models.PaneMeta{ID: paneFooter, Name: "Footer", Type: models.PaneTypeFooter, Status: models.PaneStatusPassive, Closable: false})

	if panel, err := m.pluginRegistry.CreatePane(models.PaneTypeFileTree, paneFileTree, fileTreeMeta, *cm); err == nil {
		m.registerPane(paneFileTree, panel, fileTreeMeta)
	}

	if repoRoot, ok := gitRepoRoot(cwd); ok {
		gitPaneMeta.CWD = repoRoot
		if panel, err := m.pluginRegistry.CreatePane(models.PaneTypeGitStatus, paneGitStatus, gitPaneMeta, *cm); err == nil {
			m.registerPane(paneGitStatus, panel, gitPaneMeta)
			m.bodyTree = layout.Split(
				layout.SplitHorizontal,
				30,
				layout.Split(layout.SplitVertical, 50, layout.Leaf(paneGitStatus), layout.Leaf(paneFileTree)),
				layout.Leaf(paneShell),
			)
		}
	} else {
		m.bodyTree = layout.Split(
			layout.SplitHorizontal,
			30,
			layout.Leaf(paneFileTree),
			layout.Leaf(paneShell),
		)
	}
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
	m.syncPaneMeta(id)
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.adapterManager != nil {
		if err := m.adapterManager.Init(); err != nil {
			if meta, ok := m.paneMeta[paneGitStatus]; ok {
				repoPath := meta.CWD
				cmds = append(cmds, func() tea.Msg {
					return adapters.StatusEvent{RepoPath: repoPath, Error: err}
				})
			}
		}
	}
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

	case gitplugin.OpenDiffMsg:
		cmd := m.openDiffPane(msg)
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenCommitMsg:
		cmd := m.openCommitPane(msg)
		m.invalidateView()
		return m, cmd

	case editorplugin.OpenEditorMsg:
		cmd := m.openEditorPane(msg)
		m.invalidateView()
		return m, cmd

	case editorplugin.CloseEditorMsg:
		m.closePane(msg.ID)
		m.invalidateView()
		return m, nil

	case editorplugin.SaveCompletedMsg:
		m.syncPaneMeta(msg.ID)
		m.invalidateView()
		return m, nil

	case gitplugin.CloseDiffMsg:
		m.closePane(msg.ID)
		m.invalidateView()
		return m, nil

	case gitplugin.CloseCommitMsg:
		m.closePane(msg.ID)
		m.invalidateView()
		return m, nil

	case gitplugin.CommitCompletedMsg:
		m.closePane(msg.ID)
		m.invalidateView()
		if _, ok := m.paneMeta[paneGitStatus]; ok {
			return m.routeToPane(paneGitStatus, msg)
		}
		return m, nil

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

	case shell.StartedMsg:
		m.invalidateView()
		return m.routeToPane(msg.PaneID, msg)

	case shell.RefreshMsg:
		m.invalidateView()
		return m.routeToPane(msg.PaneID, msg)

	case shell.ExitedMsg:
		m.invalidateView()
		return m.routeToPane(msg.PaneID, msg)

	case adapters.StatusEvent:
		m.invalidateView()
		var cmds []tea.Cmd
		for _, id := range m.paneOrder {
			if m.paneMeta[id].Type != models.PaneTypeGitStatus {
				continue
			}
			newPanel, cmd := m.pane(id).Update(msg)
			m.setPane(id, newPanel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.refreshPaneStatuses()
		return m, tea.Batch(cmds...)

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
	m.refreshPaneStatuses()
	return m, tea.Batch(cmds...)
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()
	if m.activeOverlayPane() != "" {
		return m, nil
	}
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

	if overlayID := m.activeOverlayPane(); overlayID != "" {
		return m.routeToPane(overlayID, msg)
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
	case "ctrl+left", "ctrl+shift+left":
		return m.adjustFocusedSplit(layout.FocusLeft)
	case "ctrl+right", "ctrl+shift+right":
		return m.adjustFocusedSplit(layout.FocusRight)
	case "ctrl+up", "ctrl+shift+up":
		return m.adjustFocusedSplit(layout.FocusUp)
	case "ctrl+down", "ctrl+shift+down":
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
	case "ctrl+-", "ctrl+_":
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
		m.restoreZoom()
		m.setFocus(paneTodo)
		return m, nil
	case "z":
		return m.toggleZoom()
	case "enter":
		if m.paneMeta[m.focused].Type == models.PaneTypeShell {
			m.mode = ModeShell
			m.refreshPaneStatuses()
			if m.paneMeta[m.focused].Status == models.PaneStatusExited {
				return m.routeToPane(m.focused, msg)
			}
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
		if editorPane, ok := m.pane(id).(editorMetaProvider); ok {
			meta.Name = editorPane.DisplayName()
			if editorPane.Dirty() {
				meta.Name = "*" + meta.Name
			}
			meta.CWD = filepath.Dir(editorPane.FilePath())
		}
		switch meta.Type {
		case models.PaneTypeHeader, models.PaneTypeFooter:
			meta.Status = models.PaneStatusPassive
		case models.PaneTypeShell:
			if sh, ok := m.pane(id).(*shell.Model); ok {
				meta.Status = sh.SessionStatus()
			} else {
				meta.Status = models.PaneStatusStarting
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

func (m *model) syncPaneMeta(id models.PaneID) {
	meta, ok := m.paneMeta[id]
	if !ok {
		return
	}
	if editorPane, ok := m.pane(id).(editorMetaProvider); ok {
		meta.Name = editorPane.DisplayName()
		if editorPane.Dirty() {
			meta.Name = "*" + meta.Name
		}
		if filePath := editorPane.FilePath(); filePath != "" {
			meta.CWD = filepath.Dir(filePath)
		}
		m.paneMeta[id] = meta
	}
}

func (m *model) nextShellPaneID() models.PaneID {
	id := models.PaneID(fmt.Sprintf("shell-%d", m.nextShell))
	m.nextShell++
	return id
}

func (m *model) nextEditorPaneID() models.PaneID {
	id := models.PaneID(fmt.Sprintf("editor-%d", m.nextEditor))
	m.nextEditor++
	return id
}

func (m *model) createShellPane() (models.PaneID, tea.Cmd) {
	id := m.nextShellPaneID()
	panel := shell.New(m.common, id)
	meta := models.PaneMeta{
		ID:       id,
		Name:     fmt.Sprintf("Shell %d", m.nextShell-1),
		Type:     models.PaneTypeShell,
		CWD:      m.currentCWD(),
		Status:   models.PaneStatusStarting,
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

func (m *model) openEditorPane(msg editorplugin.OpenEditorMsg) tea.Cmd {
	opener := m.focused
	filePath := filepath.Clean(msg.FilePath)
	if existing := m.findEditorPaneByPath(filePath); existing != "" {
		m.setFocus(existing)
		return nil
	}
	id := m.nextEditorPaneID()
	meta := models.PaneMeta{
		ID:       id,
		Name:     filepath.Base(filePath),
		Type:     models.PaneTypeEditor,
		CWD:      filepath.Dir(filePath),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := editorplugin.NewEditorPane(id, meta, *m.common, filePath)
	m.registerPane(id, panel, meta)
	if opener != "" && opener != id {
		if m.returnFocus == nil {
			m.returnFocus = make(map[models.PaneID]models.PaneID)
		}
		m.returnFocus[id] = opener
	}

	target := m.editorHostPaneTarget(opener, msg.Behavior)
	direction := m.editorSplitDirection(target, msg.Behavior)
	m.bodyTree = layout.SplitLeaf(m.bodyTree, target, id, direction, true)
	m.setFocus(id)
	m.updateSizes(m.common.Width, m.common.Height)
	return panel.Init()
}

func (m model) findEditorPaneByPath(filePath string) models.PaneID {
	for _, id := range layout.LeafOrder(m.bodyTree) {
		panel := m.pane(id)
		editorPane, ok := panel.(editorMetaProvider)
		if !ok {
			continue
		}
		if editorPane.FilePath() == filePath {
			return id
		}
	}
	return ""
}

func (m model) editorHostPaneTarget(opener models.PaneID, behavior editorplugin.OpenBehavior) models.PaneID {
	if meta, ok := m.paneMeta[opener]; ok && (meta.Type == models.PaneTypeEditor || meta.Type == models.PaneTypeShell) {
		return opener
	}
	if editor := m.lastEditorPane(); editor != "" {
		return editor
	}
	if _, ok := m.paneMeta[paneShell]; ok {
		return paneShell
	}
	if behavior == editorplugin.OpenBehaviorVSplit {
		return opener
	}
	return opener
}

func (m model) lastEditorPane() models.PaneID {
	order := layout.LeafOrder(m.bodyTree)
	for idx := len(order) - 1; idx >= 0; idx-- {
		id := order[idx]
		if meta, ok := m.paneMeta[id]; ok && meta.Type == models.PaneTypeEditor {
			return id
		}
	}
	return ""
}

func (m model) editorSplitDirection(target models.PaneID, behavior editorplugin.OpenBehavior) layout.SplitDirection {
	if behavior == editorplugin.OpenBehaviorVSplit {
		return layout.SplitHorizontal
	}
	if meta, ok := m.paneMeta[target]; ok && meta.Type == models.PaneTypeEditor {
		return layout.SplitVertical
	}
	return layout.SplitHorizontal
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
	m.closePane(m.focused)
	m.invalidateView()
	return m, nil
}

func (m *model) focusCycle(delta int) {
	if m.zoomedPane != "" {
		m.restoreZoom()
	}
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

func (m model) toggleZoom() (tea.Model, tea.Cmd) {
	if m.zoomedPane != "" {
		m.restoreZoom()
	} else {
		if m.focused == "" || m.focused == paneHeader || m.focused == paneFooter {
			return m, nil
		}
		m.preZoomTree = m.bodyTree
		m.bodyTree = layout.Leaf(m.focused)
		m.zoomedPane = m.focused
		m.updateSizes(m.common.Width, m.common.Height)
		m.invalidateView()
	}
	return m, nil
}

func (m *model) restoreZoom() {
	if m.zoomedPane == "" {
		return
	}
	if m.preZoomTree != nil {
		m.bodyTree = m.preZoomTree
		m.preZoomTree = nil
	}
	m.zoomedPane = ""
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
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

func (m model) activeOverlayPane() models.PaneID {
	if _, ok := m.paneMeta[paneGitCommit]; ok {
		return paneGitCommit
	}
	return ""
}

func (m model) isOverlayPane(id models.PaneID) bool {
	return id == paneGitCommit
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
	m.refreshPaneStatuses()
	return m, cmd
}

func (m *model) updateSizes(w, h int) {
	dims := layout.ComputeBanner(w, h)
	m.pane(paneHeader).SetSize(w, dims.HeaderH)
	footerHeight := 0
	if footerVisible(h) {
		footerHeight = 1
	}
	m.pane(paneFooter).SetSize(w, footerHeight)

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
	if overlayID := m.activeOverlayPane(); overlayID != "" {
		overlayW, overlayH := m.overlayContentSize()
		if panel := m.pane(overlayID); panel != nil {
			panel.SetSize(overlayW, overlayH)
		}
	}
}

func gitRepoRoot(path string) (string, bool) {
	output, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", false
	}

	return root, true
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
	bodyHeight := h - dims.HeaderH - 1
	if footerVisible(h) {
		bodyHeight--
	}
	if bodyHeight < 0 {
		bodyHeight = 0
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

	bodyHeight := h - dims.HeaderH - 1
	if footerVisible(h) {
		bodyHeight--
	}
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	bodyView := m.renderBody(w, bodyHeight)
	if windowTooSmall(w, h) {
		bodyView = renderWindowTooSmallBody(w, bodyHeight)
	}
	helpLine := m.renderHelpLine(w)

	sections := []string{headerView, bodyView}
	if footerVisible(h) {
		sections = append(sections, m.pane(paneFooter).View())
	}
	sections = append(sections, helpLine)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
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
		title := m.renderPaneTitle(id, max(frame.W-4, 8))
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

	if overlayID := m.activeOverlayPane(); overlayID != "" {
		base = m.renderOverlayPane(base, overlayID)
	}

	return base
}

func (m model) renderPaneTitle(id models.PaneID, contentWidth int) string {
	meta := m.paneMeta[id]
	return formatPaneTitle(meta, id == m.focused, m.mode == ModeShell && meta.Type == models.PaneTypeShell, contentWidth)
}

func (m model) renderHelpLine(w int) string {
	if w <= 0 {
		return ""
	}
	helpStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	pomo := m.pane(panePomodoro).(*pomodoro.Model)
	todoModel := m.pane(paneTodo).(*todo.Model)
	focusedType := m.paneMeta[m.focused].Type

	if m.overlay == OverlayPicker {
		return renderCompactHelpLine(helpStyle, "[enter]select  [esc]skip", w)
	}
	if overlayID := m.activeOverlayPane(); overlayID != "" {
		switch m.paneMeta[overlayID].Type {
		case paneTypeGitCommit:
			return renderCompactHelpLine(helpStyle, "[ctrl+s]commit  [ctrl+j]fallback  [esc]cancel", w)
		}
	}
	if m.mode == ModeInput {
		return renderCompactHelpLine(helpStyle, "[enter]confirm  [esc]cancel", w)
	}
	if m.mode == ModeShell {
		text := "[esc]normal  [ctrl+t]todo  [shell input active]"
		if w < simplifiedHelpMaxWidth {
			text = "[esc]normal  [ctrl+t]todo"
		}
		return renderCompactHelpLine(helpStyle, text, w)
	}

	left := "[tab]next  [ctrl+h/j/k/l]focus  [enter]activate"
	compact := "[tab]next  [enter]open  [q]uit"
	switch focusedType {
	case models.PaneTypeGitStatus:
		left = "[j/k]move  [enter]review  [d]iff file  [space]stage  [a]all  [f]etch  [p]ull  [c]ommit  [P]push"
		compact = "[enter]review  [d]iff  [a]all"
	case models.PaneTypeTodo:
		if todoModel.IsConfirmingDelete() {
			left = "[j/k]move  [y/n]delete"
			compact = "[j/k]move  [y/n]delete"
		} else {
			left = "[j/k]move  [a]dd  [e]dit  [d]el  [space]done  [shift+tab]list"
			compact = "[j/k]move  [a]dd  [space]done"
		}
	case models.PaneTypePomodoro:
		if pomo.CurrentPhase() == pomodoro.PhaseIdle {
			left = "[s]tart pomo"
			compact = "[s]tart pomo  [q]uit"
		} else if pomo.IsPaused() {
			left = "[p]resume  [r]eset"
			compact = "[p]resume  [r]eset"
		} else {
			left = "[p]ause  [n]ext  [r]eset"
			compact = "[p]ause  [r]eset"
		}
	case models.PaneTypeShell:
		switch m.paneMeta[m.focused].Status {
		case models.PaneStatusExited:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [enter]restart shell"
			compact = "[tab]next  [enter]restart  [q]uit"
		case models.PaneStatusStarting:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [shell starting]"
			compact = "[tab]next  [shell starting]"
		default:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [enter]shell"
			compact = "[tab]next  [enter]shell  [q]uit"
		}
	case models.PaneTypeEditor:
		left = "[ctrl+s]save  [ctrl+f /]search  [:]line  [n/N]result  [esc]close"
		compact = "[ctrl+s]save  [/]search  [:]line"
	case models.PaneTypeDiffView:
		left = "[j/k]scroll  [q/esc]close review"
		compact = "[j/k]scroll  [esc]close"
	}
	if w < simplifiedHelpMaxWidth {
		return renderCompactHelpLine(helpStyle, compact, w)
	}
	right := "[ctrl+\\/ctrl+-]split  [ctrl+arrows]resize"
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

func renderCompactHelpLine(helpStyle lipgloss.Style, text string, w int) string {
	if w <= 0 {
		return ""
	}
	if w <= 2 {
		return helpStyle.Render(strings.Repeat(" ", w))
	}
	content := text
	if ansi.StringWidth(content) > w-2 {
		content = ansi.Truncate(content, w-2, "")
	}
	return helpStyle.Render("  " + content)
}

func footerVisible(h int) bool {
	return h >= hideFooterBelowHeight
}

func windowTooSmall(w, h int) bool {
	return w < tinyWindowMinWidth || h < tinyWindowMinHeight
}

func renderWindowTooSmallBody(w, h int) string {
	base := blankCanvas(w, h)
	if base == "" {
		return ""
	}
	message := "Window too small"
	if w < ansi.StringWidth(message) {
		message = ansi.Truncate(message, w, "")
	}
	x := (w - ansi.StringWidth(message)) / 2
	if x < 0 {
		x = 0
	}
	y := h / 2
	if y >= h {
		y = h - 1
	}
	if y < 0 {
		y = 0
	}
	return layout.OverlayOnBase(base, message, x, y)
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

func (m *model) openDiffPane(msg gitplugin.OpenDiffMsg) tea.Cmd {
	opener := m.focused
	target := m.reviewHostPaneTarget(opener)
	m.closePane(paneGitCommit)
	m.closePane(paneGitDiff)

	meta := models.PaneMeta{
		ID:       paneGitDiff,
		Name:     "Diff",
		Type:     models.PaneTypeDiffView,
		CWD:      m.gitRepoPath(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := gitplugin.NewDiffPane(meta.ID, meta, *m.common, m.adapterManager.Git(), msg.FilePath, msg.Staged)
	m.registerPane(meta.ID, panel, meta)
	if opener != "" && opener != paneGitDiff {
		if m.returnFocus == nil {
			m.returnFocus = make(map[models.PaneID]models.PaneID)
		}
		m.returnFocus[paneGitDiff] = opener
	}
	m.bodyTree = layout.SplitLeaf(m.bodyTree, target, paneGitDiff, m.reviewSplitDirection(target), true)
	m.setFocus(meta.ID)
	m.updateSizes(m.common.Width, m.common.Height)
	return panel.Init()
}

func (m model) reviewHostPaneTarget(opener models.PaneID) models.PaneID {
	if meta, ok := m.paneMeta[opener]; ok && (meta.Type == models.PaneTypeEditor || meta.Type == models.PaneTypeShell) {
		return opener
	}
	if editor := m.lastEditorPane(); editor != "" {
		return editor
	}
	if _, ok := m.paneMeta[paneShell]; ok {
		return paneShell
	}
	if opener != "" {
		return opener
	}
	return paneGitStatus
}

func (m model) reviewSplitDirection(target models.PaneID) layout.SplitDirection {
	if meta, ok := m.paneMeta[target]; ok && meta.Type == models.PaneTypeEditor {
		return layout.SplitVertical
	}
	return layout.SplitHorizontal
}

func (m *model) openCommitPane(msg gitplugin.OpenCommitMsg) tea.Cmd {
	m.closePane(paneGitDiff)
	m.closePane(paneGitCommit)

	baseFocus := m.focused
	if baseFocus == "" {
		baseFocus = paneGitStatus
	}

	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = m.gitRepoPath()
	}

	meta := models.PaneMeta{
		ID:       paneGitCommit,
		Name:     "Commit",
		Type:     paneTypeGitCommit,
		CWD:      repoPath,
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := gitplugin.NewCommitPane(meta.ID, meta, *m.common, m.adapterManager.Git(), msg.StagedFiles)
	m.registerPane(meta.ID, panel, meta)
	m.overlayBaseFocus = baseFocus
	m.setFocus(meta.ID)
	m.updateSizes(m.common.Width, m.common.Height)
	return panel.Init()
}

func (m *model) closePane(id models.PaneID) {
	meta, ok := m.paneMeta[id]
	if !ok || !meta.Closable {
		return
	}
	if m.zoomedPane == id {
		m.restoreZoom()
	}

	if sh, ok := m.pane(id).(*shell.Model); ok {
		_ = sh.Close()
	}

	wasFocused := m.focused == id
	wasOverlay := m.isOverlayPane(id)
	preferred := m.returnFocus[id]
	var fallback models.PaneID
	leafOrder := layout.LeafOrder(m.bodyTree)
	leafCount := len(leafOrder)
	leafPresent := false
	for _, leafID := range leafOrder {
		if leafID == id {
			leafPresent = true
			break
		}
	}
	if leafPresent {
		if leafCount <= 1 {
			return
		}
		if wasFocused {
			fallback = layout.CloseFocusFallback(id, m.frames)
		}
		m.bodyTree = layout.RemoveLeaf(m.bodyTree, id)
	}

	delete(m.panes, id)
	delete(m.paneMeta, id)
	m.removePaneOrder(id)
	if m.returnFocus != nil {
		delete(m.returnFocus, id)
		for paneID, target := range m.returnFocus {
			if target == id {
				delete(m.returnFocus, paneID)
			}
		}
	}

	if wasOverlay {
		restore := m.overlayBaseFocus
		m.overlayBaseFocus = ""
		if restore != "" {
			if _, ok := m.paneMeta[restore]; ok {
				m.setFocus(restore)
			} else if _, ok := m.paneMeta[paneGitStatus]; ok {
				m.setFocus(paneGitStatus)
			}
		} else if _, ok := m.paneMeta[paneGitStatus]; ok {
			m.setFocus(paneGitStatus)
		}
	} else if wasFocused {
		restoredPreferred := false
		if preferred != "" {
			if _, ok := m.paneMeta[preferred]; ok && preferred != id {
				m.setFocus(preferred)
				restoredPreferred = true
			}
		}
		if !restoredPreferred && fallback != "" && fallback != id {
			m.setFocus(fallback)
		} else if !restoredPreferred {
			remaining := layout.LeafOrder(m.bodyTree)
			if len(remaining) > 0 {
				m.setFocus(remaining[0])
			}
		}
	}

	m.updateSizes(m.common.Width, m.common.Height)
	m.refreshPaneStatuses()
}

func (m model) gitRepoPath() string {
	if meta, ok := m.paneMeta[paneGitStatus]; ok && meta.CWD != "" {
		return meta.CWD
	}
	return m.currentCWD()
}

func (m model) overlayContentSize() (int, int) {
	bounds := m.bodyBounds()
	width := bounds.W - 12
	if width > 96 {
		width = 96
	}
	if width < 20 {
		width = 20
	}

	height := bounds.H - 6
	if height > 18 {
		height = 18
	}
	if height < 4 {
		height = 4
	}

	return width, height
}

func (m model) renderOverlayPane(base string, id models.PaneID) string {
	panel := m.pane(id)
	if panel == nil {
		return base
	}

	bounds := m.bodyBounds()
	overlayW, overlayH := m.overlayContentSize()
	overlayView := layout.RenderPanel(m.renderPaneTitle(id, overlayW), panel.View(), overlayW, overlayH, true)
	x := bounds.X + (bounds.W-(overlayW+4))/2
	y := bounds.Y + (bounds.H-(overlayH+2))/2
	if x < bounds.X {
		x = bounds.X
	}
	if y < bounds.Y {
		y = bounds.Y
	}
	return layout.OverlayOnBase(base, overlayView, x, y)
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

func formatPaneTitle(meta models.PaneMeta, focused bool, shellActive bool, contentWidth int) string {
	name := strings.ToUpper(meta.Name)
	badge := paneStatusBadge(meta)
	focusBadge := ""
	if focused {
		if shellActive {
			focusBadge = "active"
		} else {
			focusBadge = "focus"
		}
	}

	var candidates []string
	if meta.Type == models.PaneTypeShell && meta.CWD != "" {
		cwd := shortenCWD(meta.CWD)
		candidates = append(candidates,
			composePaneTitle(name, cwd, badge, focusBadge),
			composePaneTitle(name, "", badge, focusBadge),
			composePaneTitle(name, "", "", focusBadge),
			name,
		)
	} else {
		candidates = append(candidates,
			composePaneTitle(name, "", badge, focusBadge),
			composePaneTitle(name, "", "", focusBadge),
			name,
		)
	}

	maxTitleWidth := contentWidth + 1
	if maxTitleWidth < 8 {
		maxTitleWidth = 8
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= maxTitleWidth {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func composePaneTitle(name, cwd, badge, focusBadge string) string {
	title := name
	if cwd != "" {
		title += " [" + cwd + "]"
	}
	if badge != "" {
		title += " [" + badge + "]"
	}
	if focusBadge != "" {
		title += " [" + focusBadge + "]"
	}
	return title
}

func paneStatusBadge(meta models.PaneMeta) string {
	switch meta.Type {
	case models.PaneTypeShell:
		switch meta.Status {
		case models.PaneStatusStarting, models.PaneStatusRunning, models.PaneStatusExited:
			return string(meta.Status)
		}
	case models.PaneTypeTodo, models.PaneTypePomodoro:
		if !meta.Closable {
			return "fixed"
		}
	case models.PaneTypeEditor:
		if strings.HasPrefix(meta.Name, "*") {
			return "modified"
		}
	}
	return ""
}

func shortenCWD(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return shortenPath(cwd, home)
}

func shortenPath(path, home string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	if home != "" {
		home = filepath.Clean(home)
		if cleaned == home {
			cleaned = "~"
		} else if strings.HasPrefix(cleaned, home+string(os.PathSeparator)) {
			cleaned = "~" + strings.TrimPrefix(cleaned, home)
		}
	}
	if cleaned == string(os.PathSeparator) || cleaned == "~" {
		return cleaned
	}
	if strings.HasPrefix(cleaned, "~"+string(os.PathSeparator)) {
		return shortenSegments("~", strings.TrimPrefix(cleaned, "~"+string(os.PathSeparator)))
	}
	if strings.HasPrefix(cleaned, string(os.PathSeparator)) {
		return shortenSegments(string(os.PathSeparator), strings.TrimPrefix(cleaned, string(os.PathSeparator)))
	}
	return shortenSegments("", cleaned)
}

func shortenSegments(prefix, rest string) string {
	parts := strings.Split(rest, string(os.PathSeparator))
	if len(parts) <= 2 {
		if prefix == "" {
			return rest
		}
		return prefix + string(os.PathSeparator) + rest
	}
	short := filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	if prefix == "" {
		return short
	}
	if prefix == "~" {
		return filepath.Join("~", short)
	}
	return prefix + short
}
