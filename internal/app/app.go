package app

import (
	"fmt"
	"focus/internal/a2a"
	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/avatar"
	"focus/internal/config"
	gitmodel "focus/internal/git"
	"focus/internal/mcp"
	"focus/internal/models"
	"focus/internal/orchestrator"
	"focus/internal/plugins"
	agentsplugin "focus/internal/plugins/agents"
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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
)

const (
	paneHeader          models.PaneID = "header"
	paneShell           models.PaneID = "shell-main"
	paneWorktree        models.PaneID = "worktree-main"
	paneOverviewSummary models.PaneID = "overview-summary-main"
	paneOverviewDetail  models.PaneID = "overview-detail-main"
	paneGitStatus       models.PaneID = "git-status-main"
	paneGitDiff         models.PaneID = "git-diff-pane"
	paneGitCommit       models.PaneID = "git-commit-overlay"
	paneWorktreeCreate  models.PaneID = "worktree-create-overlay"
	paneTaskEdit        models.PaneID = "task-edit-overlay"
	panePlanEdit        models.PaneID = "plan-edit-overlay"
	paneTodo            models.PaneID = "todo-main"
	paneFileTree        models.PaneID = "file-tree-main"
	panePomodoro        models.PaneID = "pomodoro-main"
	paneAgentSession    models.PaneID = "agent-session-main"
	paneFooter          models.PaneID = "footer"

	paneTypeGitCommit       models.PaneType = "git-commit"
	paneTypeWorktreeCreate  models.PaneType = "worktree-create"
	paneTypeTaskEdit        models.PaneType = "task-edit"
	paneTypePlanEdit        models.PaneType = "plan-edit"
	paneTypeOverviewSummary models.PaneType = "overview-summary"
	paneTypeOverviewDetail  models.PaneType = "overview-detail"

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

	activePage *page
	pages      map[string]*page

	viewGen             uint64
	vc                  *viewCache
	currentWorktreePage string

	overlayBaseFocus models.PaneID

	pluginRegistry     *plugins.Registry
	adapterManager     *adapters.Manager
	agentRegistry      *agents.Registry
	resumeSummaryCache map[string]gitmodel.WorktreeResumeSummary
	mcpServer          *mcp.Server
	a2aRouter          *a2a.Router

	lastAgentSync time.Time
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
	repoRoot, _ := gitRepoRoot(cwd)

	m := model{
		common:             cm,
		state:              StateDashboard,
		mode:               ModeNormal,
		overlay:            OverlayNone,
		vc:                 &viewCache{},
		pluginRegistry:     plugins.NewRegistry(),
		adapterManager:     adapters.NewManager(),
		agentRegistry:      agents.NewRegistry(),
		pages:              make(map[string]*page),
		resumeSummaryCache: make(map[string]gitmodel.WorktreeResumeSummary),
		mcpServer:          mcp.NewServer(cfg.Agent.MCPSocket),
		a2aRouter:          a2a.NewRouter(cfg.Agent.A2ASocket),
	}
	_ = m.mcpServer.Start()
	_ = m.a2aRouter.Start()

	gitAdapter := adapters.NewGitLocalAdapter()
	_ = m.adapterManager.Register(gitAdapter.Name(), gitAdapter)

	gitPlugin := gitplugin.New(m.adapterManager.Git())
	fileTreePlugin := filebrowser.New()
	editorPlugin := editorplugin.New()
	agentPlugin := agentsplugin.New()
	_ = m.pluginRegistry.Register(gitPlugin)
	_ = m.pluginRegistry.Register(fileTreePlugin)
	_ = m.pluginRegistry.Register(editorPlugin)
	_ = m.pluginRegistry.Register(agentPlugin)

	m.activePage = newOverviewPage(cm, m.pluginRegistry, m.adapterManager, cfg, store, cwd, repoRoot)
	m.pages[""] = m.activePage

	m.loadPageSnapshots()
	m.syncWorktreeActivities()

	return m
}

func (m *model) registerPane(id models.PaneID, panel models.Panel, meta models.PaneMeta) {
	m.activePage.registerPane(id, panel, meta)
}

func (m *model) pane(id models.PaneID) models.Panel {
	return m.activePage.pane(id)
}

func (m *model) setPane(id models.PaneID, panel models.Panel) {
	m.activePage.setPane(id, panel)
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.adapterManager != nil {
		if err := m.adapterManager.Init(); err != nil {
			if meta, ok := m.activePage.paneMeta[paneGitStatus]; ok {
				repoPath := meta.CWD
				cmds = append(cmds, func() tea.Msg {
					return adapters.StatusEvent{RepoPath: repoPath, Error: err}
				})
			}
		}
	}
	for _, id := range m.activePage.paneOrder {
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
		return m, m.routeToPane(panePomodoro, msg)

	case gitplugin.OpenDiffMsg:
		cmd := m.openDiffPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenCommitMsg:
		cmd := m.openCommitPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenCreateWorktreeMsg:
		cmd := m.openCreateWorktreePane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenWorktreeShellMsg:
		cmd := m.openWorktreeShell(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.ResumeWorktreeMsg:
		cmd := m.resumeWorktree(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenTaskEditMsg:
		cmd := m.openTaskEditPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenPlanEditMsg:
		cmd := m.openPlanEditPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.CycleTaskStateMsg:
		cmd := m.cycleTaskState(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case CloseTaskEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case ClosePlanEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case TaskEditorSavedMsg:
		cmd := m.saveTaskEditor(msg)
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case PlanEditorSavedMsg:
		cmd := m.savePlanEditor(msg)
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agents.LaunchAgentMsg:
		cmd := m.launchAgent(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agents.AgentExitedMsg:
		m.handleAgentExited(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case agents.ExternalLaunchResultMsg:
		m.handleExternalLaunchResult(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case agentsplugin.KillSessionMsg:
		cmd := m.killAgent(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agentsplugin.FocusAgentSessionMsg:
		cmd := m.focusAgentSession(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case editorplugin.OpenEditorMsg:
		cmd := m.openEditorPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case editorplugin.CloseEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case editorplugin.SaveCompletedMsg:
		m.syncPaneMeta(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseDiffMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseCommitMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseCreateWorktreeMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CommitCompletedMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneGitStatus]; ok {
			return m, m.routeToPane(paneGitStatus, msg)
		}
		return m, nil

	case gitplugin.WorktreeCreatedMsg:
		m.closePane(msg.ID)
		var cmds []tea.Cmd
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			cmd := m.routeToPane(paneWorktree, gitplugin.RefreshWorktreesMsg{})
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if cmd := m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: msg.Worktree}); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, batchCmds(cmds)

	case gitplugin.RequestRemoveWorktreeMsg:
		cmd := m.removeWorktree(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.RequestPruneWorktreesMsg:
		cmd := m.pruneWorktrees(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.WorktreeRemovedMsg:
		m.closePanesForWorktree(msg.Path)
		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			return m, m.routeToPane(paneWorktree, msg)
		}
		return m, nil

	case gitplugin.WorktreesPrunedMsg, gitplugin.WorktreeActionFailedMsg:
		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			return m, m.routeToPane(paneWorktree, msg)
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
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case shell.RefreshMsg:
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case shell.ExitedMsg:
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case adapters.StatusEvent:
		m.syncWorktreeActivities()
		m.invalidateView()
		var cmds []tea.Cmd
		for _, id := range m.activePage.paneOrder {
			if m.activePage.paneMeta[id].Type != models.PaneTypeGitStatus {
				continue
			}
			newPanel, cmd := m.pane(id).Update(msg)
			m.setPane(id, newPanel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.refreshPaneStatuses()
		return m, batchCmds(cmds)

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		m.updateSizes(msg.Width, msg.Height)
		m.invalidateView()
	}

	var cmds []tea.Cmd
	var dirty bool
	for _, id := range m.activePage.paneOrder {
		newPanel, cmd := m.pane(id).Update(msg)
		if newPanel != m.pane(id) {
			dirty = true
			m.setPane(id, newPanel)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if dirty {
		m.invalidateView()
	}
	m.refreshPaneStatuses()
	return m, batchCmds(cmds)
}

func batchCmds(cmds []tea.Cmd) tea.Cmd {
	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
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
	if clicked != "" && msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown && msg.Button != tea.MouseButtonWheelLeft && msg.Button != tea.MouseButtonWheelRight {
		m.setFocus(clicked)
	}
	if clicked == "" {
		return m, nil
	}
	if m.activePage.paneMeta[clicked].Type != models.PaneTypeShell {
		return m, m.routeToPane(clicked, msg)
	}
	if m.mode != ModeShell {
		return m, nil
	}
	frame, ok := m.activePage.frames[clicked]
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
	return m, m.routeToPane(clicked, tea.Msg(adjusted))
}

// handleKey routes keyboard input based on overlay, mode, and focused pane.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()
	pomo := m.pane(panePomodoro).(*pomodoro.Model)

	if m.overlay == OverlayPicker {
		if pomo.IsPickerActive() {
			cmd := m.routeToPane(panePomodoro, msg)
			if !m.pane(panePomodoro).(*pomodoro.Model).IsPickerActive() {
				m.overlay = OverlayNone
			}
			return m, cmd
		}
		m.overlay = OverlayNone
		return m, nil
	}

	if overlayID := m.activeOverlayPane(); overlayID != "" {
		return m, m.routeToPane(overlayID, msg)
	}

	if m.mode == ModeInput {
		if msg.String() == "ctrl+c" {
			m.closeShellPanes()
			return m, tea.Quit
		}
		return m, m.routeToPane(paneTodo, msg)
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
		case "ctrl+g":
			m.switchToOverviewPage()
			return m, nil
		default:
			if m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
				return m, m.routeToPane(m.activePage.focused, msg)
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
		if m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeTodo {
			return m, m.routeToPane(m.activePage.focused, msg)
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
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusLeft))
		return m, nil
	case "ctrl+l":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusRight))
		return m, nil
	case "ctrl+k":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusUp))
		return m, nil
	case "ctrl+j":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusDown))
		return m, nil
	case "ctrl+t":
		m.restoreZoom()
		m.setFocus(paneTodo)
		return m, nil
	case "ctrl+g":
		m.switchToOverviewPage()
		return m, nil
	case "ctrl+n":
		if m.state == StateWorktreePage {
			return m, m.switchToAdjacentWorktreePage(1)
		}
	case "ctrl+p":
		if m.state == StateWorktreePage {
			return m, m.switchToAdjacentWorktreePage(-1)
		}
	case "z":
		return m.toggleZoom()
	case "enter":
		if m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
			m.mode = ModeShell
			m.refreshPaneStatuses()
			if m.activePage.paneMeta[m.activePage.focused].Status == models.PaneStatusExited {
				return m, m.routeToPane(m.activePage.focused, msg)
			}
			return m, nil
		}
	}

	if m.activePage.focused != "" && m.activePage.paneMeta[m.activePage.focused].Type != models.PaneTypeShell {
		return m, m.routeToPane(m.activePage.focused, msg)
	}

	return m, nil
}

func (m *model) setPaneStatus(id models.PaneID, status models.PaneStatus) {
	meta := m.activePage.paneMeta[id]
	meta.Status = status
	m.activePage.paneMeta[id] = meta
}

func (m *model) setFocus(id models.PaneID) {
	m.activePage.setFocus(id)
}

func (m *model) refreshPaneStatuses() {
	m.activePage.refreshPaneStatuses()
}

func (m *model) syncPaneMeta(id models.PaneID) {
	m.activePage.syncPaneMeta(id)
}

func (m *model) nextShellPaneID() models.PaneID {
	return m.activePage.nextShellPaneID()
}

func (m *model) nextEditorPaneID() models.PaneID {
	return m.activePage.nextEditorPaneID()
}

func (m *model) createShellPane() (models.PaneID, tea.Cmd) {
	return m.activePage.createShellPane()
}

func (m *model) createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot string) (models.PaneID, tea.Cmd) {
	return m.activePage.createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot)
}

func (m *model) openWorktreeShell(msg gitplugin.OpenWorktreeShellMsg) tea.Cmd {
	worktreeID := msg.Worktree.Path
	if worktreeID == "" {
		worktreeID = m.currentWorktreeID()
	}
	pageCmd := m.switchToWorktreePage(worktreeID, "")
	cmd := m.activePage.openWorktreeShell(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && cmd != nil {
		return tea.Batch(pageCmd, cmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return cmd
}

func (m *model) resumeWorktree(msg gitplugin.ResumeWorktreeMsg) tea.Cmd {
	worktreeID := msg.Worktree.Path
	if worktreeID == "" {
		worktreeID = m.currentWorktreeID()
	}
	cmd := m.switchToWorktreePage(worktreeID, "")
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) launchAgent(msg agents.LaunchAgentMsg) tea.Cmd {
	worktreeID := msg.WorktreeID
	provider := msg.Provider
	if provider == "" {
		provider = agents.DefaultProvider()
	}

	if m.agentRegistry != nil && m.agentRegistry.HasRunning(worktreeID, provider) {
		pageCmd := m.switchToWorktreePage(worktreeID, "")
		var focusCmd tea.Cmd
		if m.activePage != nil {
			focusCmd = m.activePage.focusAgentShell(worktreeID)
		}
		m.updateSizes(m.common.Width, m.common.Height)
		if pageCmd != nil && focusCmd != nil {
			return tea.Batch(pageCmd, focusCmd)
		}
		if pageCmd != nil {
			return pageCmd
		}
		return focusCmd
	}

	session := m.newAgentSession(worktreeID, provider)
	m.saveAgentSession(session)
	if m.agentRegistry != nil {
		m.agentRegistry.Register(session)
	}

	pageCmd := m.switchToWorktreePage(worktreeID, "")
	if m.common != nil && m.common.Cfg.Agent.ExternalTerminal {
		launchCmd := m.launchExternalAgent(session)
		m.updateSizes(m.common.Width, m.common.Height)
		if pageCmd != nil && launchCmd != nil {
			return tea.Batch(pageCmd, launchCmd)
		}
		if pageCmd != nil {
			return pageCmd
		}
		return launchCmd
	}
	var launchCmd tea.Cmd
	if m.activePage != nil {
		launchCmd = m.activePage.openAgentShell(worktreeID, provider, session.ID)
	}
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && launchCmd != nil {
		return tea.Batch(pageCmd, launchCmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return launchCmd
}

func (m *model) launchExternalAgent(session *agents.Session) tea.Cmd {
	if session == nil {
		return nil
	}
	title := fmt.Sprintf("Focus:%s:%s", session.PlanID, session.ID)
	if session.TaskID != "" {
		title = fmt.Sprintf("Focus:%s:%s", session.PlanID, session.TaskID)
	}
	envVars := []string{
		agents.SessionIDEnvVar + "=" + session.ID,
		agents.LegacySessionIDEnvVar + "=" + session.ID,
	}
	if session.TaskID != "" {
		envVars = append(envVars, agents.TaskIDEnvVar+"="+session.TaskID)
	}
	if session.PlanID != "" {
		envVars = append(envVars, agents.PlanIDEnvVar+"="+session.PlanID)
	}
	if socket := strings.TrimSpace(m.common.Cfg.Agent.MCPSocket); socket != "" {
		envVars = append(envVars, agents.MCPSocketEnvVar+"="+socket)
	}
	if socket := strings.TrimSpace(m.common.Cfg.Agent.A2ASocket); socket != "" {
		envVars = append(envVars, agents.A2ASocketEnvVar+"="+socket)
	}
	session.State = agents.SessionWaiting
	session.EnvSnapshot = strings.Join(envVars, " ")
	m.saveAgentSession(session)
	return agents.LaunchExternalCommand(agents.ExternalLaunchRequest{
		SessionID:        session.ID,
		Title:            title,
		WorktreeID:       session.WorktreeID,
		Provider:         session.Provider,
		TerminalEmulator: strings.TrimSpace(m.common.Cfg.Agent.TerminalEmulator),
		EnvVars:          envVars,
	})
}

func (m *model) handleExternalLaunchResult(msg agents.ExternalLaunchResultMsg) {
	record := m.agentSessionRecord(msg.SessionID)
	if record.ID == "" || record.WorktreeID == "" {
		return
	}
	now := time.Now()
	if msg.Err != nil {
		record.State = string(agents.SessionFailed)
		record.StopReason = msg.Err.Error()
		record.EndedAt = &now
		record.PID = 0
		record.UpdatedAt = now
		record.LastActivityAt = &now
		m.saveAgentSessionRecord(record)
		return
	}
	record.PID = msg.PID
	record.State = string(agents.SessionRunning)
	record.StopReason = ""
	record.EndedAt = nil
	record.UpdatedAt = now
	record.LastActivityAt = &now
	record.LastHeartbeat = &now
	m.saveAgentSessionRecord(record)
}

func (m *model) killAgent(msg agentsplugin.KillSessionMsg) tea.Cmd {
	if m.agentRegistry != nil {
		m.agentRegistry.Remove(msg.SessionID)
	}
	if msg.SessionID != "" {
		now := time.Now()
		record := m.agentSessionRecord(msg.SessionID)
		record.PID = 0
		record.State = string(agents.SessionExited)
		record.EndedAt = &now
		record.UpdatedAt = now
		m.saveAgentSessionRecord(record)
	}
	if msg.PID > 0 {
		_ = exec.Command("kill", "-TERM", strconv.Itoa(msg.PID)).Run()
	}
	return nil
}

func (m *model) handleAgentExited(msg agents.AgentExitedMsg) {
	now := time.Now()
	for _, record := range m.listAgentSessionRecords(msg.WorktreeID) {
		if record.Provider != string(msg.Provider) || record.State != string(agents.SessionRunning) {
			continue
		}
		record.PID = 0
		record.State = string(agents.SessionExited)
		record.EndedAt = &now
		record.UpdatedAt = now
		m.saveAgentSessionRecord(record)
		m.backflowAgentSession(record)
		if m.agentRegistry != nil {
			m.agentRegistry.Remove(record.ID)
		}
	}
}

func (m *model) focusAgentSession(msg agentsplugin.FocusAgentSessionMsg) tea.Cmd {
	worktreeID := msg.WorktreeID
	pageCmd := m.switchToWorktreePage(worktreeID, "")
	var focusCmd tea.Cmd
	if m.activePage != nil {
		focusCmd = m.activePage.focusAgentShell(worktreeID)
	}
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && focusCmd != nil {
		return tea.Batch(pageCmd, focusCmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return focusCmd
}

func (m *model) currentCWD() string {
	return m.activePage.currentCWD()
}

func (m *model) currentRepoID() string {
	return m.activePage.currentRepoID()
}

func (m *model) currentWorktreeID() string {
	return m.activePage.currentWorktreeID()
}

func (m *model) currentBranchSnapshot() string {
	return m.activePage.currentBranchSnapshot()
}

func (m *model) openEditorPane(msg editorplugin.OpenEditorMsg) tea.Cmd {
	cmd := m.activePage.openEditorPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m model) findEditorPaneByPath(filePath string) models.PaneID {
	return m.activePage.findEditorPaneByPath(filePath)
}

func (m model) editorHostPaneTarget(opener models.PaneID, behavior editorplugin.OpenBehavior) models.PaneID {
	return m.activePage.editorHostPaneTarget(opener, behavior)
}

func (m model) lastEditorPane() models.PaneID {
	return m.activePage.lastEditorPane()
}

func (m model) editorSplitDirection(target models.PaneID, behavior editorplugin.OpenBehavior) layout.SplitDirection {
	return m.activePage.editorSplitDirection(target, behavior)
}

func (m model) splitFocused(direction layout.SplitDirection) (tea.Model, tea.Cmd) {
	cmd := m.activePage.splitFocused(direction)
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, cmd
}

func (m model) adjustFocusedSplit(direction layout.FocusDirection) (tea.Model, tea.Cmd) {
	if m.activePage.adjustFocusedSplit(direction) {
		m.updateSizes(m.common.Width, m.common.Height)
		m.invalidateView()
	}
	return m, nil
}

func (m model) closeFocusedPane() (tea.Model, tea.Cmd) {
	m.activePage.closeFocusedPane()
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m *model) focusCycle(delta int) {
	m.activePage.focusCycle(delta)
}

func (m model) toggleZoom() (tea.Model, tea.Cmd) {
	m.activePage.toggleZoom()
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m *model) restoreZoom() {
	m.activePage.restoreZoom()
}

func (m *model) removePaneOrder(id models.PaneID) {
	m.activePage.removePaneOrder(id)
}

func (m model) activeOverlayPane() models.PaneID {
	return m.activePage.activeOverlayPane()
}

func (m model) isOverlayPane(id models.PaneID) bool {
	return id == paneGitCommit || id == paneWorktreeCreate
}

func (m *model) paneAt(x, y int) models.PaneID {
	return m.activePage.paneAt(x, y)
}

func (m model) routeToPane(id models.PaneID, msg tea.Msg) tea.Cmd {
	return m.activePage.routeToPane(id, msg)
}

func (m *model) updateSizes(w, h int) {
	dims := layout.ComputeBanner(w, h)
	m.activePage.pane(paneHeader).SetSize(w, dims.HeaderH)
	footerHeight := 0
	if footerVisible(h) {
		footerHeight = 1
	}
	m.activePage.pane(paneFooter).SetSize(w, footerHeight)
	m.activePage.updateSizes(m.activePage.bodyBounds(w, h))
	if overlayID := m.activePage.activeOverlayPane(); overlayID != "" {
		overlayW, overlayH := m.overlayContentSize()
		if panel := m.activePage.pane(overlayID); panel != nil {
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
	return m.activePage.bodyBounds(m.common.Width, m.common.Height)
}

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

	if m.mode == ModeShell && m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
		if sh, ok := m.pane(m.activePage.focused).(*shell.Model); ok {
			if frame, exists := m.activePage.frames[m.activePage.focused]; exists {
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
	return m.activePage.renderBody(w, h, m.overlay)
}

func (m model) renderPaneTitle(id models.PaneID, contentWidth int) string {
	return m.activePage.renderPaneTitle(id, m.activePage.focused, m.mode, contentWidth)
}

func (m model) renderHelpLine(w int) string {
	if w <= 0 {
		return ""
	}
	helpStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	pomo := m.pane(panePomodoro).(*pomodoro.Model)
	todoModel := m.pane(paneTodo).(*todo.Model)
	focusedType := m.activePage.paneMeta[m.activePage.focused].Type

	if m.overlay == OverlayPicker {
		return renderCompactHelpLine(helpStyle, "[enter]select  [esc]skip", w)
	}
	if overlayID := m.activeOverlayPane(); overlayID != "" {
		switch m.activePage.paneMeta[overlayID].Type {
		case paneTypeGitCommit:
			return renderCompactHelpLine(helpStyle, "[ctrl+s]commit  [ctrl+j]fallback  [esc]cancel", w)
		case paneTypeWorktreeCreate:
			return renderCompactHelpLine(helpStyle, "[tab]next  [enter]next/create  [ctrl+s]create  [esc]cancel", w)
		case paneTypeTaskEdit:
			return renderCompactHelpLine(helpStyle, "[tab]next  [enter]next/save  [ctrl+s]save  [esc]cancel", w)
		case paneTypePlanEdit:
			return renderCompactHelpLine(helpStyle, "[tab]switch  [enter]into body  [ctrl+s]save  [esc]cancel", w)
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
	case models.PaneTypeWorktree:
		left = "[1-4]/[]tabs  [j/k]move  [enter]resume  [o]shell  [e]task  [f]ollow-up  [p]lan  [P]rune  [s]tate  [d]el  [n]ew worktree  [r]efresh  [ctrl+g]overview"
		compact = "[1-4]tabs  [enter]resume  [e]task  [p]lan"
	case models.PaneTypeGitStatus:
		if m.state == StateWorktreePage {
			left = "[j/k]move  [enter]review  [d]iff file  [space]stage  [a]all  [f]etch  [p]ull  [c]ommit  [P]push  [ctrl+n/p]worktree  [ctrl+g]overview"
			compact = "[enter]review  [d]iff  [a]all  [ctrl+g]overview"
		} else {
			left = "[j/k]move  [enter]review  [d]iff file  [space]stage  [a]all  [f]etch  [p]ull  [c]ommit  [P]push"
			compact = "[enter]review  [d]iff  [a]all"
		}
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
		switch m.activePage.paneMeta[m.activePage.focused].Status {
		case models.PaneStatusExited:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [enter]restart shell"
			compact = "[tab]next  [enter]restart  [q]uit"
		case models.PaneStatusStarting:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [shell starting]"
			compact = "[tab]next  [shell starting]"
		default:
			left = "[tab]next  [ctrl+h/j/k/l]focus  [enter]shell  [ctrl+n/p]worktree  [ctrl+g]overview"
			compact = "[tab]next  [enter]shell  [ctrl+g]overview"
		}
	case models.PaneTypeEditor:
		left = "[ctrl+s]save  [ctrl+f /]search  [:]line  [n/N]result  [esc]close"
		compact = "[ctrl+s]save  [/]search  [:]line"
	case models.PaneTypeDiffView:
		left = "[enter]open file  [s]toggle staged  [[]/[]]files  [j/k]scroll  [wheel]scroll  [q/esc]close review"
		compact = "[enter]open  [s]toggle  [wheel]scroll"
	case paneTypeOverviewDetail:
		left = "[tab]next  [1-4]/[]tabs  [enter]resume  [e]edit task  [f]follow-up  [s]cycle state"
		compact = "[tab]next  [1-4]tabs  [enter]resume"
	case models.PaneTypeAgentSession:
		left = "[j/k]move  [enter]focus worktree  [a]relaunch  [x]kill  [r]refresh"
		compact = "[enter]focus  [a]relaunch  [x]kill"
	}
	if w < simplifiedHelpMaxWidth {
		return renderCompactHelpLine(helpStyle, compact, w)
	}
	right := "[ctrl+\\/ctrl+-]split  [ctrl+arrows]resize"
	if meta, ok := m.activePage.paneMeta[m.activePage.focused]; ok && meta.Closable {
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
	m.activePage.closeShellPanes()
}

func (m *model) openDiffPane(msg gitplugin.OpenDiffMsg) tea.Cmd {
	cmd := m.activePage.openDiffPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m model) reviewHostPaneTarget(opener models.PaneID) models.PaneID {
	return m.activePage.reviewHostPaneTarget(opener)
}

func (m model) reviewSplitDirection(target models.PaneID) layout.SplitDirection {
	return m.activePage.reviewSplitDirection(target)
}

func (m *model) openCommitPane(msg gitplugin.OpenCommitMsg) tea.Cmd {
	cmd := m.activePage.openCommitPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openCreateWorktreePane(msg gitplugin.OpenCreateWorktreeMsg) tea.Cmd {
	cmd := m.activePage.openCreateWorktreePane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openTaskEditPane(msg gitplugin.OpenTaskEditMsg) tea.Cmd {
	seed := taskEditorSeed{WorktreeID: msg.WorktreeID, State: "active", Priority: "medium", RelationType: msg.RelationType, ParentTaskID: msg.ParentTaskID}
	if m.common != nil && m.common.Store != nil {
		if wc, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID); wc != nil {
			if msg.RelationType != "queued" {
				seed.Title = wc.TaskName
			}
			targetTaskID := msg.TaskID
			if targetTaskID == "" && msg.RelationType != "queued" && wc.PrimaryTaskID != nil {
				targetTaskID = *wc.PrimaryTaskID
			}
			if targetTaskID != "" {
				if task, _ := m.common.Store.GetTaskContext(targetTaskID); task != nil {
					seed.TaskID = task.ID
					seed.Title = task.Title
					seed.Goal = task.Goal
					seed.NextStep = task.NextStep
					seed.State = task.State
					seed.Priority = task.Priority
					if brief, _ := m.common.Store.GetTaskBrief(task.ID); brief != nil {
						seed.WhyNow = brief.WhyNow
						seed.Success = brief.SuccessCriteria
						seed.OutOfScope = brief.OutOfScope
						seed.KnownRisks = brief.KnownRisks
					}
				}
			}
		}
	}
	cmd := m.activePage.openTaskEditPane(seed)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openPlanEditPane(msg gitplugin.OpenPlanEditMsg) tea.Cmd {
	seed := planEditorSeed{TaskID: msg.TaskID, WorktreeID: msg.WorktreeID, Status: "draft"}
	if m.common != nil && m.common.Store != nil {
		var plan *models.TaskPlanRecord
		if msg.TaskID != "" {
			plans := m.listTaskPlans(msg.TaskID)
			if len(plans) > 0 {
				plan = &plans[0]
			} else if task, _ := m.common.Store.GetTaskContext(msg.TaskID); task != nil {
				seed.Title = task.Title
			}
		} else if wc, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID); wc != nil && wc.CurrentPlanID != nil && *wc.CurrentPlanID != "" {
			plan, _ = m.common.Store.GetTaskPlan(*wc.CurrentPlanID)
		}
		if plan != nil {
			seed.PlanID = plan.ID
			seed.Title = plan.Title
			seed.WhyNow = plan.WhyNow
			seed.Success = plan.Success
			seed.OutOfScope = plan.OutOfScope
			seed.KnownRisks = plan.KnownRisks
			seed.PlanBody = plan.PlanBody
			seed.Status = plan.Status
			seed.CurrentStep = plan.CurrentStep
			if seed.TaskID == "" {
				seed.TaskID = plan.TaskID
			}
			if steps := m.listPlanSteps(plan.ID); len(steps) > 0 {
				seed.PlanBody = renderPlanSteps(steps)
			}
		}
	}
	cmd := m.activePage.openPlanEditPane(seed)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) saveTaskEditor(msg TaskEditorSavedMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil || msg.WorktreeID == "" {
		return nil
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(msg.WorktreeID)
	}
	if repoID == "" {
		repoID = msg.WorktreeID
	}
	worktreeContext, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID)
	relationType := msg.RelationType
	if relationType == "" {
		relationType = "primary"
	}
	taskID := msg.TaskID
	if taskID == "" {
		taskID = uuid.NewString()
	}
	if relationType == "primary" && taskID == "" && worktreeContext != nil && worktreeContext.PrimaryTaskID != nil && *worktreeContext.PrimaryTaskID != "" {
		taskID = *worktreeContext.PrimaryTaskID
	}
	now := time.Now()
	_ = m.common.Store.SaveTaskContext(models.TaskContextRecord{
		ID:                  taskID,
		RepoID:              repoID,
		Title:               msg.Title,
		Goal:                msg.Goal,
		NextStep:            msg.NextStep,
		State:               msg.State,
		Priority:            msg.Priority,
		ParentTaskID:        stringPtrOrNil(msg.ParentTaskID),
		PreferredWorktreeID: msg.WorktreeID,
	})
	_ = m.common.Store.SaveTaskBrief(models.TaskBriefRecord{
		TaskID:          taskID,
		WhyNow:          msg.WhyNow,
		SuccessCriteria: msg.Success,
		OutOfScope:      msg.OutOfScope,
		KnownRisks:      msg.KnownRisks,
	})
	if worktreeContext != nil && worktreeContext.CurrentPlanID != nil && *worktreeContext.CurrentPlanID != "" {
		if plan, _ := m.common.Store.GetTaskPlan(*worktreeContext.CurrentPlanID); plan != nil && plan.TaskID == "" {
			plan.TaskID = taskID
			_ = m.common.Store.SaveTaskPlan(*plan)
		}
	}
	branchSnapshot := ""
	if worktreeContext != nil {
		branchSnapshot = worktreeContext.BranchSnapshot
	}
	primaryTaskID := (*string)(nil)
	currentPlanID := (*string)(nil)
	taskMode := "single"
	taskName := msg.Title
	if worktreeContext != nil {
		primaryTaskID = worktreeContext.PrimaryTaskID
		currentPlanID = worktreeContext.CurrentPlanID
		taskMode = worktreeContext.TaskMode
		if worktreeContext.TaskName != "" {
			taskName = worktreeContext.TaskName
		}
	}
	if relationType == "primary" {
		primaryTaskID = &taskID
		taskMode = "single"
		taskName = msg.Title
	} else {
		if primaryTaskID != nil && *primaryTaskID != "" {
			taskMode = "mixed"
		}
	}
	_ = m.common.Store.SaveWorktreeContext(models.WorktreeContextRecord{
		WorktreeID:     msg.WorktreeID,
		RepoID:         repoID,
		PrimaryTaskID:  primaryTaskID,
		CurrentPlanID:  currentPlanID,
		TaskMode:       taskMode,
		TaskName:       taskName,
		BranchSnapshot: branchSnapshot,
		LastActiveAt:   now,
		LastOpenedAt:   &now,
		LastAgentAt:    lastAgentAtForWorktree(m.listAgentSessionRecords(msg.WorktreeID)),
	})
	_ = m.common.Store.SaveTaskWorktreeLink(models.TaskWorktreeLinkRecord{
		ID:           taskID + "::" + msg.WorktreeID + "::" + relationType,
		TaskID:       taskID,
		WorktreeID:   msg.WorktreeID,
		RelationType: relationType,
	})
	return nil
}

func (m *model) savePlanEditor(msg PlanEditorSavedMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	planID := msg.PlanID
	if planID == "" {
		planID = uuid.NewString()
	}
	status := msg.Status
	if status == "" {
		status = "draft"
	}
	currentStep := msg.CurrentStep
	if currentStep == "" {
		currentStep = inferCurrentPlanStep(msg.PlanBody)
	}
	_ = m.common.Store.SaveTaskPlan(models.TaskPlanRecord{
		ID:          planID,
		TaskID:      msg.TaskID,
		Title:       msg.Title,
		WhyNow:      msg.WhyNow,
		Success:     msg.Success,
		OutOfScope:  msg.OutOfScope,
		KnownRisks:  msg.KnownRisks,
		Status:      status,
		CurrentStep: currentStep,
		PlanBody:    msg.PlanBody,
	})
	_ = m.common.Store.DeletePlanSteps(planID)
	for i, step := range parsePlanSteps(msg.PlanBody) {
		_ = m.common.Store.SavePlanStep(models.PlanStepRecord{
			ID:         fmt.Sprintf("%s::step::%03d", planID, i),
			PlanID:     planID,
			OrderIndex: i,
			Title:      step.Title,
			State:      stepStateForIndex(i, step.Kind),
			Notes:      step.Kind,
		})
	}
	if msg.WorktreeID != "" {
		m.attachPlanToWorktree(msg.WorktreeID, msg.Title, planID)
	}
	if msg.ExpandToTasks {
		m.expandPlanToTasks(planID, msg.WorktreeID)
	}
	return nil
}

func inferCurrentPlanStep(planBody string) string {
	for _, step := range parsePlanSteps(planBody) {
		if step.Title != "" && step.Kind != "blocked" && step.Kind != "follow-up" {
			return step.Title
		}
	}
	return ""
}

type parsedPlanStep struct {
	Title string
	Kind  string
}

func parsePlanSteps(planBody string) []parsedPlanStep {
	lines := strings.Split(planBody, "\n")
	steps := make([]parsedPlanStep, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•0123456789. "))
		if line == "" {
			continue
		}
		kind := "main"
		switch {
		case strings.HasPrefix(strings.ToLower(line), "blocked:"):
			kind = "blocked"
			line = strings.TrimSpace(line[len("blocked:"):])
		case strings.HasPrefix(strings.ToLower(line), "follow-up:"):
			kind = "follow-up"
			line = strings.TrimSpace(line[len("follow-up:"):])
		case strings.HasPrefix(strings.ToLower(line), "worktree:"):
			kind = "worktree-candidate"
			line = strings.TrimSpace(line[len("worktree:"):])
		case strings.HasPrefix(strings.ToLower(line), "validation:"):
			kind = "validation"
			line = strings.TrimSpace(line[len("validation:"):])
		case strings.HasPrefix(strings.ToLower(line), "risk:"):
			kind = "risk"
			line = strings.TrimSpace(line[len("risk:"):])
		}
		if line == "" {
			continue
		}
		steps = append(steps, parsedPlanStep{Title: line, Kind: kind})
	}
	return steps
}

func renderPlanSteps(steps []models.PlanStepRecord) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		line := strings.TrimSpace(step.Title)
		if line == "" {
			continue
		}
		switch step.Notes {
		case "blocked":
			line = "blocked: " + line
		case "follow-up":
			line = "follow-up: " + line
		case "worktree-candidate":
			line = "worktree: " + line
		case "validation":
			line = "validation: " + line
		case "risk":
			line = "risk: " + line
		}
		lines = append(lines, "- "+line)
	}
	return strings.Join(lines, "\n")
}

func stepStateForIndex(index int, kind string) string {
	if kind == "blocked" {
		return "blocked"
	}
	if index == 0 && kind != "follow-up" {
		return "in_progress"
	}
	return "pending"
}

func (m *model) attachPlanToWorktree(worktreeID, fallbackTitle, planID string) {
	if m.common == nil || m.common.Store == nil || worktreeID == "" || planID == "" {
		return
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if repoID == "" {
		repoID = worktreeID
	}
	now := time.Now()
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	record := models.WorktreeContextRecord{
		WorktreeID:    worktreeID,
		RepoID:        repoID,
		CurrentPlanID: &planID,
		TaskMode:      "single",
		TaskName:      fallbackTitle,
		LastActiveAt:  now,
		LastOpenedAt:  &now,
	}
	if wc != nil {
		record.PrimaryTaskID = wc.PrimaryTaskID
		record.TaskMode = wc.TaskMode
		record.TaskName = wc.TaskName
		record.BranchSnapshot = wc.BranchSnapshot
		record.LastAgentAt = wc.LastAgentAt
		if record.TaskName == "" {
			record.TaskName = fallbackTitle
		}
	}
	_ = m.common.Store.SaveWorktreeContext(record)
}

func (m *model) expandPlanToTasks(planID, worktreeID string) {
	if m.common == nil || m.common.Store == nil || planID == "" || worktreeID == "" {
		return
	}
	plan, err := m.common.Store.GetTaskPlan(planID)
	if err != nil || plan == nil {
		return
	}
	steps := m.listPlanSteps(planID)
	if len(steps) == 0 {
		return
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if repoID == "" {
		repoID = worktreeID
	}
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	var primaryTaskID *string
	if wc != nil {
		primaryTaskID = wc.PrimaryTaskID
	}
	createdPrimary := primaryTaskID == nil || *primaryTaskID == ""
	for i, step := range steps {
		if step.ExpandedTaskID != "" {
			continue
		}
		taskID := uuid.NewString()
		state := "paused"
		relationType := "queued"
		if step.Notes == "blocked" {
			state = "blocked"
		}
		if i == 0 && createdPrimary {
			state = "active"
			relationType = "primary"
			primaryTaskID = &taskID
		} else if step.State == "in_progress" {
			state = "active"
		}
		if step.Notes == "worktree-candidate" {
			relationType = "secondary"
		}
		_ = m.common.Store.SaveTaskContext(models.TaskContextRecord{
			ID:                  taskID,
			RepoID:              repoID,
			Title:               step.Title,
			Goal:                plan.Title,
			NextStep:            step.Title,
			State:               state,
			Priority:            "medium",
			PreferredWorktreeID: worktreeID,
		})
		_ = m.common.Store.SaveTaskBrief(models.TaskBriefRecord{TaskID: taskID, WhyNow: plan.WhyNow, SuccessCriteria: plan.Success, OutOfScope: plan.OutOfScope, KnownRisks: plan.KnownRisks})
		step.ExpandedTaskID = taskID
		_ = m.common.Store.SavePlanStep(step)
		_ = m.common.Store.SaveTaskWorktreeLink(models.TaskWorktreeLinkRecord{ID: taskID + "::" + worktreeID + "::" + relationType, TaskID: taskID, WorktreeID: worktreeID, RelationType: relationType})
		if plan.TaskID == "" && relationType == "primary" {
			plan.TaskID = taskID
		}
	}
	for i := 0; i+1 < len(steps); i++ {
		fromTaskID := strings.TrimSpace(steps[i].ExpandedTaskID)
		toTaskID := strings.TrimSpace(steps[i+1].ExpandedTaskID)
		if fromTaskID == "" || toTaskID == "" || fromTaskID == toTaskID {
			continue
		}
		_ = m.common.Store.SaveTaskDependency(models.TaskDependencyRecord{
			FromTaskID:     fromTaskID,
			ToTaskID:       toTaskID,
			DependencyType: "hard",
		})
	}
	if plan.TaskID != "" {
		_ = m.common.Store.SaveTaskPlan(*plan)
	}
	if wc != nil {
		wc.PrimaryTaskID = primaryTaskID
		if wc.TaskName == "" {
			wc.TaskName = plan.Title
		}
		if wc.TaskMode == "" {
			wc.TaskMode = "mixed"
		}
		_ = m.common.Store.SaveWorktreeContext(*wc)
	}
}

func (m *model) cycleTaskState(msg gitplugin.CycleTaskStateMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil || msg.TaskID == "" {
		return nil
	}
	task, err := m.common.Store.GetTaskContext(msg.TaskID)
	if err != nil || task == nil {
		return nil
	}
	prevState := task.State
	task.State = nextTaskState(task.State)
	_ = m.common.Store.SaveTaskContext(*task)
	if prevState != "done" && task.State == "done" {
		return m.launchDownstreamTasks(task.ID)
	}
	return nil
}

func (m *model) launchDownstreamTasks(taskID string) tea.Cmd {
	if m.common == nil || m.common.Store == nil || taskID == "" {
		return nil
	}
	launcher := &appOrchestratorLauncher{model: m}
	engine := orchestrator.New(appOrchestratorStore{model: m}, launcher)
	if _, err := engine.OnTaskCompleted(taskID); err != nil {
		return nil
	}
	if len(launcher.cmds) == 0 {
		return nil
	}
	return tea.Batch(launcher.cmds...)
}

type appOrchestratorStore struct {
	model *model
}

func (s appOrchestratorStore) GetDownstreamTasks(taskID string) ([]orchestrator.Task, error) {
	records, err := s.model.common.Store.ListDownstreamTaskContexts(taskID)
	if err != nil {
		return nil, err
	}
	out := make([]orchestrator.Task, 0, len(records))
	for _, record := range records {
		out = append(out, orchestrator.Task{
			ID:         record.ID,
			Name:       record.Title,
			WorktreeID: strings.TrimSpace(record.PreferredWorktreeID),
			Provider:   string(agents.DefaultProvider()),
		})
	}
	return out, nil
}

func (s appOrchestratorStore) AllPrerequisitesMet(taskID string) (bool, error) {
	return s.model.common.Store.AreTaskPrerequisitesMet(taskID)
}

func (s appOrchestratorStore) MarkSessionDisconnected(sessionID string) error {
	return s.model.common.Store.MarkAgentSessionDisconnected(sessionID, "heartbeat timeout")
}

type appOrchestratorLauncher struct {
	model *model
	cmds  []tea.Cmd
}

func (l *appOrchestratorLauncher) LaunchTask(task orchestrator.Task) error {
	if strings.TrimSpace(task.WorktreeID) == "" {
		return nil
	}
	provider := agents.Provider(task.Provider)
	if provider == "" {
		provider = agents.DefaultProvider()
	}
	cmd := l.model.launchAgent(agents.LaunchAgentMsg{
		WorktreeID: task.WorktreeID,
		Provider:   provider,
	})
	if cmd != nil {
		l.cmds = append(l.cmds, cmd)
	}
	return nil
}

func nextTaskState(state string) string {
	switch state {
	case "active":
		return "paused"
	case "paused":
		return "blocked"
	case "blocked":
		return "done"
	case "done":
		return "active"
	default:
		return "active"
	}
}

func (m *model) removeWorktree(msg gitplugin.RequestRemoveWorktreeMsg) tea.Cmd {
	adapter := m.adapterManager.Git()
	repoPath := m.gitRepoPath()
	return func() tea.Msg {
		if adapter == nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "remove", Err: fmt.Errorf("git adapter unavailable")}
		}
		if err := adapter.RemoveWorktree(repoPath, msg.Worktree.Path, gitmodel.RemoveWorktreeOptions{Force: msg.Force}); err != nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "remove", Err: err}
		}
		return gitplugin.WorktreeRemovedMsg{Path: msg.Worktree.Path, Force: msg.Force}
	}
}

func (m *model) pruneWorktrees(msg gitplugin.RequestPruneWorktreesMsg) tea.Cmd {
	adapter := m.adapterManager.Git()
	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = m.gitRepoPath()
	}
	return func() tea.Msg {
		if adapter == nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "prune", Err: fmt.Errorf("git adapter unavailable")}
		}
		if err := adapter.PruneWorktrees(repoPath); err != nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "prune", Err: err}
		}
		return gitplugin.WorktreesPrunedMsg{}
	}
}

func (m *model) closePanesForWorktree(worktreePath string) {
	m.activePage.closePanesForWorktree(worktreePath)
}

func (m *model) closePane(id models.PaneID) {
	m.activePage.closePane(id)
}

func (m model) gitRepoPath() string {
	return m.activePage.gitRepoPath()
}

func (m model) overlayContentSize() (int, int) {
	return m.activePage.overlayContentSize()
}

func (m model) renderOverlayPane(base string, id models.PaneID) string {
	return m.activePage.renderOverlayPane(base, id)
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

func (m *model) worktreeList() []gitmodel.Worktree {
	adapter := m.adapterManager.Git()
	repoPath := m.gitRepoPath()
	if adapter == nil || repoPath == "" {
		return nil
	}
	wts, _ := adapter.ListWorktrees(repoPath)
	return wts
}

func (m *model) switchToAdjacentWorktreePage(delta int) tea.Cmd {
	wts := m.worktreeList()
	if len(wts) == 0 {
		return nil
	}
	idx := -1
	for i, wt := range wts {
		if wt.Path == m.currentWorktreePage {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	nextIdx := (idx + delta + len(wts)) % len(wts)
	return m.switchToWorktreePage(wts[nextIdx].Path, "")
}

func (m *model) syncWorktreeActivities() {
	if m.pages == nil {
		return
	}

	shouldDiscover := time.Since(m.lastAgentSync) >= time.Second
	if shouldDiscover {
		m.lastAgentSync = time.Now()
	}

	persisted := m.persistedAgentSessions()
	if shouldDiscover {
		persisted = m.reconcileDiscoveredAgentSessions(persisted, agents.DiscoverRunningAgents())
	}
	m.refreshResumeSummaryCache(persisted)

	runningSessions := make(map[string][]agents.Session)
	visibleSessions := m.sortedAgentSessions(persisted)
	if m.agentRegistry != nil {
		m.agentRegistry.Clear()
		for _, session := range visibleSessions {
			if session.State != agents.SessionRunning {
				continue
			}
			s := *session
			m.agentRegistry.Register(&s)
			runningSessions[session.WorktreeID] = append(runningSessions[session.WorktreeID], s)
		}
	}

	for _, p := range m.pages {
		if ap, ok := p.pane(paneAgentSession).(*agentsplugin.SessionPane); ok {
			ap.SetSessions(visibleSessions)
		}
	}

	for worktreeID, p := range m.pages {
		if worktreeID == "" {
			continue
		}
		activity := gitmodel.WorktreeActivity{}
		for id, meta := range p.paneMeta {
			if meta.Type == models.PaneTypeEditor {
				activity.OpenEditors++
			}
			if meta.Type == models.PaneTypeShell {
				if sh, ok := p.pane(id).(*shell.Model); ok {
					if sh.SessionStatus() == models.PaneStatusReady || sh.SessionStatus() == models.PaneStatusStarting {
						activity.HasShell = true
					}
				}
			}
		}
		if p == m.activePage {
			activity.LastActive = "now"
		} else if p.snapshot != nil && p.snapshot.Focused != "" {
			activity.LastActive = "recent"
		}
		activity.AgentCount = len(runningSessions[worktreeID])
		if wp, ok := m.pages[""].pane(paneWorktree).(*gitplugin.WorktreePane); ok {
			wp.SetActivity(worktreeID, activity)
			wp.SetAgentSessions(runningSessions)
			wp.SetResumeSummaries(m.resumeSummaryCache)
		}
	}
}

func (m *model) refreshResumeSummaryCache(agentSessions map[string]agents.Session) {
	if m.resumeSummaryCache == nil {
		m.resumeSummaryCache = make(map[string]gitmodel.WorktreeResumeSummary)
	}
	for key := range m.resumeSummaryCache {
		delete(m.resumeSummaryCache, key)
	}

	repoID := m.gitRepoPath()
	if repoID == "" {
		cwd, _ := os.Getwd()
		repoID, _ = gitRepoRoot(cwd)
	}
	worktreeContexts := m.listWorktreeContexts(repoID)
	taskContexts := m.listTaskContexts("")
	tasksByID := make(map[string]models.TaskContextRecord, len(taskContexts))
	for _, task := range taskContexts {
		tasksByID[task.ID] = task
	}

	for _, wc := range worktreeContexts {
		summary := gitmodel.WorktreeResumeSummary{
			TaskMode:        wc.TaskMode,
			TaskTitle:       wc.TaskName,
			LastActiveLabel: formatRelativeLabel("active", wc.LastActiveAt),
		}
		if wc.PrimaryTaskID != nil {
			summary.TaskID = *wc.PrimaryTaskID
			if task, ok := tasksByID[*wc.PrimaryTaskID]; ok {
				summary.TaskTitle = task.Title
				summary.TaskGoal = task.Goal
				summary.NextStep = task.NextStep
				summary.TaskState = task.State
				summary.TaskPriority = task.Priority
				if brief := m.getTaskBrief(task.ID); brief != nil {
					summary.TaskWhyNow = brief.WhyNow
					summary.TaskSuccess = brief.SuccessCriteria
					summary.TaskOutOfScope = brief.OutOfScope
					summary.TaskKnownRisks = brief.KnownRisks
				}
			}
		}
		if summary.TaskTitle == "" {
			summary.TaskTitle = wc.TaskName
		}
		if wc.LastAgentAt != nil {
			summary.LastAgentLabel = formatRelativeLabel("agent", *wc.LastAgentAt)
		}
		plan := m.resolveCurrentPlan(wc.CurrentPlanID, summary.TaskID)
		if plan != nil {
			summary.PlanTitle = plan.Title
			summary.PlanStatus = plan.Status
			summary.CurrentPlanStep = plan.CurrentStep
			summary.PlanBody = plan.PlanBody
			if summary.TaskWhyNow == "" {
				summary.TaskWhyNow = plan.WhyNow
			}
			if summary.TaskSuccess == "" {
				summary.TaskSuccess = plan.Success
			}
			if summary.TaskOutOfScope == "" {
				summary.TaskOutOfScope = plan.OutOfScope
			}
			if summary.TaskKnownRisks == "" {
				summary.TaskKnownRisks = plan.KnownRisks
			}
			steps := m.listPlanSteps(plan.ID)
			if len(steps) > 0 {
				summary.PlanSteps = formatPlanStepSummaries(steps)
				if summary.PlanBody == "" {
					summary.PlanBody = renderPlanSteps(steps)
				}
			}
		}
		if summary.TaskID != "" {
			handoffs := m.listSessionHandoffs(summary.TaskID)
			if len(handoffs) > 0 {
				h := handoffs[0]
				if summary.HandoffNote == "" {
					summary.HandoffNote = h.RemainingSummary
				}
				summary.HandoffEntrypoint = h.Entrypoint
				if summary.BlockerNote == "" {
					summary.BlockerNote = h.BlockerSummary
				}
			}
		}
		summary.AttentionAnchor, summary.RecentArtifact = m.deriveWorktreeSignals(wc.WorktreeID)
		summary.PinnedNote, summary.BlockerNote, summary.HandoffNote = m.deriveWorktreeNotes(summary.TaskID, wc.WorktreeID)
		summary.GitPressure = deriveGitPressure(wc.BranchSnapshot, wc.WorktreeID, m.pages[""].pane(paneWorktree))
		for _, link := range m.listWorktreeTaskLinks(wc.WorktreeID) {
			if link.RelationType != "queued" {
				continue
			}
			queuedTask, ok := tasksByID[link.TaskID]
			if !ok {
				continue
			}
			summary.QueuedTaskCount++
			if summary.QueuedTaskTitle == "" {
				summary.QueuedTaskTitle = queuedTask.Title
			}
		}
		if s := mostRelevantAgentSession(wc.WorktreeID, agentSessions); s != nil {
			summary.LastAgentSummary = formatAgentSummary(*s)
			if summary.LastAgentLabel == "" {
				summary.LastAgentLabel = formatRelativeLabel("agent", sessionRelevantTime(*s))
			}
		}
		summary.ResumeReason, summary.ResumeScore = computeResumeReason(summary)
		summary.LastResumeHint = buildResumeHint(summary)
		m.resumeSummaryCache[wc.WorktreeID] = summary
	}

	for worktreeID, page := range m.pages {
		if worktreeID == "" {
			continue
		}
		if _, ok := m.resumeSummaryCache[worktreeID]; ok {
			continue
		}
		summary := gitmodel.WorktreeResumeSummary{}
		if page == m.activePage {
			summary.LastActiveLabel = "active now"
			summary.ResumeScore += 100
		} else if page.snapshot != nil {
			summary.LastActiveLabel = "resume available"
			summary.ResumeScore += 40
		}
		if s := mostRelevantAgentSession(worktreeID, agentSessions); s != nil {
			summary.LastAgentSummary = formatAgentSummary(*s)
			summary.LastAgentLabel = formatRelativeLabel("agent", sessionRelevantTime(*s))
		}
		summary.AttentionAnchor, summary.RecentArtifact = m.deriveWorktreeSignals(worktreeID)
		plan := m.resolveCurrentPlan(nil, summary.TaskID)
		if plan != nil {
			summary.PlanTitle = plan.Title
			summary.PlanStatus = plan.Status
			summary.CurrentPlanStep = plan.CurrentStep
			summary.PlanBody = plan.PlanBody
			if summary.TaskWhyNow == "" {
				summary.TaskWhyNow = plan.WhyNow
			}
			if summary.TaskSuccess == "" {
				summary.TaskSuccess = plan.Success
			}
			if summary.TaskOutOfScope == "" {
				summary.TaskOutOfScope = plan.OutOfScope
			}
			if summary.TaskKnownRisks == "" {
				summary.TaskKnownRisks = plan.KnownRisks
			}
			steps := m.listPlanSteps(plan.ID)
			if len(steps) > 0 {
				summary.PlanSteps = formatPlanStepSummaries(steps)
				if summary.PlanBody == "" {
					summary.PlanBody = renderPlanSteps(steps)
				}
			}
		}
		if summary.TaskID != "" {
			handoffs := m.listSessionHandoffs(summary.TaskID)
			if len(handoffs) > 0 {
				h := handoffs[0]
				if summary.HandoffNote == "" {
					summary.HandoffNote = h.RemainingSummary
				}
				summary.HandoffEntrypoint = h.Entrypoint
				if summary.BlockerNote == "" {
					summary.BlockerNote = h.BlockerSummary
				}
			}
		}
		summary.PinnedNote, summary.BlockerNote, summary.HandoffNote = m.deriveWorktreeNotes(summary.TaskID, worktreeID)
		summary.GitPressure = deriveGitPressure("", worktreeID, m.pages[""].pane(paneWorktree))
		summary.ResumeReason, summary.ResumeScore = computeResumeReason(summary)
		summary.LastResumeHint = buildResumeHint(summary)
		m.resumeSummaryCache[worktreeID] = summary
	}
}

func (m *model) listTaskContexts(repoID string) []models.TaskContextRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListTaskContexts(repoID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listWorktreeContexts(repoID string) []models.WorktreeContextRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListWorktreeContexts(repoID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) getTaskBrief(taskID string) *models.TaskBriefRecord {
	if m.common == nil || m.common.Store == nil || taskID == "" {
		return nil
	}
	record, err := m.common.Store.GetTaskBrief(taskID)
	if err != nil {
		return nil
	}
	return record
}

func (m *model) listWorktreeTaskLinks(worktreeID string) []models.TaskWorktreeLinkRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListWorktreeTaskLinks(worktreeID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listTaskPlans(taskID string) []models.TaskPlanRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListTaskPlans(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listPlanSteps(planID string) []models.PlanStepRecord {
	if m.common == nil || m.common.Store == nil || planID == "" {
		return nil
	}
	records, err := m.common.Store.ListPlanSteps(planID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) resolveCurrentPlan(currentPlanID *string, taskID string) *models.TaskPlanRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	if currentPlanID != nil && *currentPlanID != "" {
		record, err := m.common.Store.GetTaskPlan(*currentPlanID)
		if err == nil && record != nil {
			return record
		}
	}
	if taskID == "" {
		return nil
	}
	plans := m.listTaskPlans(taskID)
	if len(plans) == 0 {
		return nil
	}
	return &plans[0]
}

func formatPlanStepSummaries(steps []models.PlanStepRecord) []string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		state := strings.TrimSpace(step.State)
		if state == "" {
			state = "pending"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", state, step.Title))
	}
	return lines
}

func (m *model) listSessionHandoffs(taskID string) []models.SessionHandoffRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListSessionHandoffs(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) deriveWorktreeNotes(taskID string, worktreeID string) (string, string, string) {
	if m.common == nil || m.common.Store == nil {
		return "", "", ""
	}
	notes, err := m.common.Store.ListContextNotes(taskID, worktreeID)
	if err != nil {
		return "", "", ""
	}
	var pinned, blocker, handoff string
	for _, note := range notes {
		if blocker == "" && note.NoteType == "blocker" {
			blocker = note.Body
		}
		if handoff == "" && note.NoteType == "handoff" {
			handoff = note.Body
		}
		if pinned == "" && note.Pinned {
			pinned = note.Body
		}
		if pinned != "" && blocker != "" && handoff != "" {
			break
		}
	}
	return pinned, blocker, handoff
}

func deriveGitPressure(branchSnapshot, worktreeID string, pane models.Panel) string {
	wp, ok := pane.(*gitplugin.WorktreePane)
	if !ok || wp == nil {
		return ""
	}
	for _, item := range wp.OrderedContexts() {
		if item.Worktree.Path != worktreeID {
			continue
		}
		wt := item.Worktree
		if wt.DirtySummary.Conflicted > 0 {
			return "conflicted"
		}
		if wt.AheadBehind.Behind > 0 && wt.DirtySummary.IsDirty() {
			return "diverged+dirty"
		}
		if wt.AheadBehind.Behind > 0 {
			return "behind"
		}
		if wt.DirtySummary.IsDirty() {
			return "dirty"
		}
		if branchSnapshot != "" || wt.Branch != "" {
			return "clean"
		}
	}
	return ""
}

func (m *model) deriveWorktreeSignals(worktreeID string) (string, string) {
	if worktreeID == "" {
		return "", ""
	}
	if page, ok := m.pages[worktreeID]; ok && page != nil {
		if anchor, artifact := pageSignals(page); anchor != "" || artifact != "" {
			return anchor, artifact
		}
	}
	if page, ok := m.pages[worktreeID]; ok && page != nil && page.snapshot != nil {
		if anchor, artifact := snapshotSignals(page.snapshot); anchor != "" || artifact != "" {
			return anchor, artifact
		}
	}
	return "", ""
}

func pageSignals(p *page) (string, string) {
	if p == nil {
		return "", ""
	}
	if meta, ok := p.paneMeta[p.focused]; ok {
		switch meta.Type {
		case models.PaneTypeEditor:
			return "editing", meta.CWD
		case models.PaneTypeDiffView:
			return "reviewing diff", meta.CWD
		case models.PaneTypeGitStatus:
			return "checking git status", meta.CWD
		case models.PaneTypeShell:
			return "working in shell", meta.CWD
		}
	}
	for _, id := range p.paneOrder {
		meta := p.paneMeta[id]
		if meta.Type == models.PaneTypeEditor && meta.CWD != "" {
			return "editing", meta.CWD
		}
	}
	return "", ""
}

func snapshotSignals(snapshot *PageSnapshot) (string, string) {
	if snapshot == nil {
		return "", ""
	}
	if len(snapshot.OpenEditors) > 0 {
		return "resume available", snapshot.OpenEditors[0]
	}
	if snapshot.Focused != "" {
		return "resume available", string(snapshot.Focused)
	}
	return "", ""
}

func mostRelevantAgentSession(worktreeID string, sessions map[string]agents.Session) *agents.Session {
	var best *agents.Session
	for _, session := range sessions {
		if session.WorktreeID != worktreeID {
			continue
		}
		s := session
		if best == nil {
			best = &s
			continue
		}
		if best.State != s.State {
			if s.State == agents.SessionRunning {
				best = &s
			}
			continue
		}
		if sessionRelevantTime(s).After(sessionRelevantTime(*best)) {
			best = &s
		}
	}
	return best
}

func sessionRelevantTime(session agents.Session) time.Time {
	if session.UpdatedAt.IsZero() {
		return session.StartedAt
	}
	return session.UpdatedAt
}

func formatRelativeLabel(prefix string, ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	age := time.Since(ts)
	if age < time.Minute {
		return prefix + " now"
	}
	if age < time.Hour {
		return fmt.Sprintf("%s %dm ago", prefix, int(age.Minutes()))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%s %dh ago", prefix, int(age.Hours()))
	}
	return fmt.Sprintf("%s %dd ago", prefix, int(age.Hours()/24))
}

func formatAgentSummary(session agents.Session) string {
	label := string(session.Provider)
	if session.State == agents.SessionRunning {
		return label + " running"
	}
	if session.EndedAt != nil {
		return label + " finished"
	}
	return label + " recent"
}

func computeResumeReason(summary gitmodel.WorktreeResumeSummary) (string, int) {
	score := 0
	parts := []string{}
	if summary.TaskState == "active" {
		score += 50
		parts = append(parts, "active task")
	}
	if summary.NextStep != "" {
		score += 20
		parts = append(parts, "next step ready")
	}
	if summary.BlockerNote != "" {
		score += 25
		parts = append(parts, "blocker noted")
	}
	if summary.GitPressure == "conflicted" {
		score += 35
		parts = append(parts, "git conflicted")
	} else if summary.GitPressure == "diverged+dirty" {
		score += 20
		parts = append(parts, "git diverged")
	}
	if summary.LastAgentSummary != "" {
		score += 15
		parts = append(parts, summary.LastAgentSummary)
	}
	if summary.QueuedTaskCount > 0 {
		score += 10
		parts = append(parts, fmt.Sprintf("%d queued", summary.QueuedTaskCount))
	}
	if summary.LastActiveLabel != "" {
		score += 10
	}
	return strings.Join(parts, " · "), score
}

func buildResumeHint(summary gitmodel.WorktreeResumeSummary) string {
	if summary.NextStep != "" {
		return "Continue: " + summary.NextStep
	}
	if summary.BlockerNote != "" {
		return "Blocked: " + summary.BlockerNote
	}
	if summary.HandoffNote != "" {
		return "Handoff: " + summary.HandoffNote
	}
	if summary.AttentionAnchor != "" {
		return summary.AttentionAnchor
	}
	if summary.QueuedTaskTitle != "" {
		return "Queued: " + summary.QueuedTaskTitle
	}
	if summary.TaskGoal != "" {
		return "Goal: " + summary.TaskGoal
	}
	if summary.LastAgentSummary != "" {
		return summary.LastAgentSummary
	}
	return ""
}

func lastAgentAtForWorktree(records []models.AgentSessionRecord) *time.Time {
	var latest *time.Time
	for _, record := range records {
		candidate := record.UpdatedAt
		if record.LastActivityAt != nil {
			candidate = *record.LastActivityAt
		}
		if latest == nil || candidate.After(*latest) {
			t := candidate
			latest = &t
		}
	}
	return latest
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (m *model) touchActiveWorktreeContext() {
	for worktreeID, page := range m.pages {
		if worktreeID == "" || page != m.activePage {
			continue
		}
		m.touchWorktreeContext(worktreeID)
		return
	}
}

func (m *model) touchWorktreeContext(worktreeID string) {
	if worktreeID == "" || m.common == nil || m.common.Store == nil {
		return
	}
	repoID := ""
	branchSnapshot := ""
	taskName := filepath.Base(worktreeID)
	if page, ok := m.pages[worktreeID]; ok && page != nil {
		repoID = page.currentRepoID()
		branchSnapshot = page.currentBranchSnapshot()
		if branchSnapshot != "" {
			taskName = branchSnapshot
		}
	}
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if branchSnapshot == "" && m.adapterManager != nil && m.adapterManager.Git() != nil {
		if status, err := m.adapterManager.Git().GetWorktreeStatus(worktreeID); err == nil && status != nil {
			branchSnapshot = status.Branch
			if taskName == filepath.Base(worktreeID) && status.Branch != "" {
				taskName = status.Branch
			}
		}
	}
	existing, _ := m.common.Store.GetWorktreeContext(worktreeID)
	record := models.WorktreeContextRecord{
		WorktreeID:     worktreeID,
		RepoID:         repoID,
		TaskMode:       "single",
		TaskName:       taskName,
		BranchSnapshot: branchSnapshot,
		LastActiveAt:   time.Now(),
	}
	if existing != nil {
		record.PrimaryTaskID = existing.PrimaryTaskID
		record.TaskMode = existing.TaskMode
		if existing.TaskName != "" {
			record.TaskName = existing.TaskName
		}
		if existing.BranchSnapshot != "" {
			record.BranchSnapshot = existing.BranchSnapshot
		}
		if existing.LastOpenedAt != nil {
			record.LastOpenedAt = existing.LastOpenedAt
		}
		if existing.LastAgentAt != nil {
			record.LastAgentAt = existing.LastAgentAt
		}
	}
	_ = m.common.Store.SaveWorktreeContext(record)
}

func (m *model) persistedAgentSessions() map[string]agents.Session {
	records := m.listAgentSessionRecords("")
	sessions := make(map[string]agents.Session, len(records))
	for _, record := range records {
		sessions[record.ID] = agents.Session{
			ID:             record.ID,
			Provider:       agents.Provider(record.Provider),
			WorktreeID:     record.WorktreeID,
			RepoID:         record.RepoID,
			TaskID:         record.TaskID,
			PlanID:         record.PlanID,
			StepID:         record.StepID,
			BranchSnapshot: record.BranchSnapshot,
			PID:            record.PID,
			State:          agents.SessionState(record.State),
			LaunchSource:   record.LaunchSource,
			Summary:        record.Summary,
			EnvSnapshot:    record.EnvSnapshot,
			StartedAt:      record.StartedAt,
			EndedAt:        record.EndedAt,
			LastActivityAt: record.LastActivityAt,
			UpdatedAt:      record.UpdatedAt,
		}
	}
	return sessions
}

func (m *model) reconcileDiscoveredAgentSessions(existing map[string]agents.Session, discovered []agents.Session) map[string]agents.Session {
	now := time.Now()
	seen := make(map[string]struct{}, len(discovered))
	for _, session := range discovered {
		record, ok := existing[session.ID]
		if !ok {
			record = session
			record.StartedAt = now
		} else if record.StartedAt.IsZero() {
			record.StartedAt = now
		}
		record.Provider = session.Provider
		record.WorktreeID = session.WorktreeID
		record.PID = session.PID
		record.State = agents.SessionRunning
		record.LaunchSource = session.LaunchSource
		if record.LaunchSource == "" {
			record.LaunchSource = "discovered"
		}
		record.EndedAt = nil
		record.LastActivityAt = &now
		record.UpdatedAt = now
		if record.RepoID == "" {
			record.RepoID, _ = gitRepoRoot(session.WorktreeID)
		}
		if record.BranchSnapshot == "" && m.adapterManager != nil && m.adapterManager.Git() != nil {
			if status, err := m.adapterManager.Git().GetWorktreeStatus(session.WorktreeID); err == nil && status != nil {
				record.BranchSnapshot = status.Branch
			}
		}
		existing[record.ID] = record
		seen[record.ID] = struct{}{}
		m.saveAgentSession(&record)
		_ = m.common.Store.UpdateAgentSessionHeartbeat(record.ID, now, string(agents.SessionRunning))
	}

	for id, session := range existing {
		if session.State != agents.SessionRunning {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if session.PID == 0 && now.Sub(session.StartedAt) < 5*time.Second {
			continue
		}
		session.State = agents.SessionDisconnected
		session.PID = 0
		session.UpdatedAt = now
		if session.EndedAt == nil {
			endedAt := now
			session.EndedAt = &endedAt
		}
		existing[id] = session
		m.saveAgentSession(&session)
		_ = m.common.Store.MarkAgentSessionDisconnected(session.ID, "heartbeat timeout")
	}

	return existing
}

func (m *model) sortedAgentSessions(sessionMap map[string]agents.Session) []*agents.Session {
	list := make([]*agents.Session, 0, len(sessionMap))
	for _, session := range sessionMap {
		s := session
		list = append(list, &s)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].State != list[j].State {
			return list[i].State == agents.SessionRunning
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

func (m *model) newAgentSession(worktreeID string, provider agents.Provider) *agents.Session {
	now := time.Now()
	repoID := ""
	if m.activePage != nil {
		repoID = m.activePage.currentRepoID()
	}
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	branchSnapshot := ""
	if m.adapterManager != nil && m.adapterManager.Git() != nil {
		if status, err := m.adapterManager.Git().GetWorktreeStatus(worktreeID); err == nil && status != nil {
			branchSnapshot = status.Branch
		}
	}
	taskID, planID, stepID := m.currentExecutionSlice(worktreeID)
	return &agents.Session{
		ID:             agents.NewSessionID(),
		Provider:       provider,
		WorktreeID:     worktreeID,
		RepoID:         repoID,
		TaskID:         taskID,
		PlanID:         planID,
		StepID:         stepID,
		BranchSnapshot: branchSnapshot,
		State:          agents.SessionWaiting,
		LaunchSource:   "focus",
		StartedAt:      now,
		LastActivityAt: &now,
		UpdatedAt:      now,
	}
}

func (m *model) saveAgentSession(session *agents.Session) {
	if session == nil {
		return
	}
	m.saveAgentSessionRecord(models.AgentSessionRecord{
		ID:             session.ID,
		Provider:       string(session.Provider),
		WorktreeID:     session.WorktreeID,
		RepoID:         session.RepoID,
		TaskID:         session.TaskID,
		PlanID:         session.PlanID,
		StepID:         session.StepID,
		BranchSnapshot: session.BranchSnapshot,
		PID:            session.PID,
		State:          string(session.State),
		LaunchSource:   session.LaunchSource,
		Summary:        session.Summary,
		EnvSnapshot:    session.EnvSnapshot,
		StartedAt:      session.StartedAt,
		EndedAt:        session.EndedAt,
		LastActivityAt: session.LastActivityAt,
		UpdatedAt:      session.UpdatedAt,
	})
}

func (m *model) currentExecutionSlice(worktreeID string) (string, string, string) {
	if m.common == nil || m.common.Store == nil || worktreeID == "" {
		return "", "", ""
	}
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	var taskID, planID string
	if wc != nil {
		if wc.PrimaryTaskID != nil {
			taskID = *wc.PrimaryTaskID
		}
		if wc.CurrentPlanID != nil {
			planID = *wc.CurrentPlanID
		}
	}
	if planID == "" && taskID != "" {
		plan := m.resolveCurrentPlan(nil, taskID)
		if plan != nil {
			planID = plan.ID
		}
	}
	if planID == "" {
		return taskID, "", ""
	}
	for _, step := range m.listPlanSteps(planID) {
		if step.State == "in_progress" {
			if taskID == "" {
				taskID = step.ExpandedTaskID
			}
			return taskID, planID, step.ID
		}
	}
	return taskID, planID, ""
}

func (m *model) backflowAgentSession(record models.AgentSessionRecord) {
	if m.common == nil || m.common.Store == nil || record.StepID == "" || record.PlanID == "" {
		return
	}
	steps := m.listPlanSteps(record.PlanID)
	if len(steps) == 0 {
		return
	}
	var currentTitle, nextTitle string
	for i := range steps {
		if steps[i].ID != record.StepID {
			continue
		}
		steps[i].State = "done"
		currentTitle = steps[i].Title
		_ = m.common.Store.SavePlanStep(steps[i])
		if i+1 < len(steps) {
			steps[i+1].State = "in_progress"
			nextTitle = steps[i+1].Title
			_ = m.common.Store.SavePlanStep(steps[i+1])
		}
		break
	}
	plan, _ := m.common.Store.GetTaskPlan(record.PlanID)
	if plan != nil {
		plan.CurrentStep = nextTitle
		if plan.CurrentStep == "" {
			plan.Status = "completed"
		}
		_ = m.common.Store.SaveTaskPlan(*plan)
	}
	if record.TaskID != "" {
		task, _ := m.common.Store.GetTaskContext(record.TaskID)
		if task != nil {
			task.NextStep = nextTitle
			if nextTitle == "" {
				task.State = "done"
			}
			_ = m.common.Store.SaveTaskContext(*task)
		}
		planID := record.PlanID
		_ = m.common.Store.SaveSessionHandoff(models.SessionHandoffRecord{
			ID:               record.ID + "::handoff",
			TaskID:           record.TaskID,
			PlanID:           &planID,
			SessionID:        record.ID,
			DoneSummary:      currentTitle,
			RemainingSummary: nextTitle,
			DecisionSummary:  "advance to next plan step",
			Entrypoint:       "Open plan detail and continue current step",
		})
	}
}

func (m *model) saveAgentSessionRecord(record models.AgentSessionRecord) {
	if m.common == nil || m.common.Store == nil {
		return
	}
	_ = m.common.Store.SaveAgentSession(record)
}

func (m *model) listAgentSessionRecords(worktreeID string) []models.AgentSessionRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListAgentSessions(worktreeID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) agentSessionRecord(sessionID string) models.AgentSessionRecord {
	for _, record := range m.listAgentSessionRecords("") {
		if record.ID == sessionID {
			return record
		}
	}
	return models.AgentSessionRecord{ID: sessionID}
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
