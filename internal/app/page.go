package app

import (
	"fmt"
	"os"
	"path/filepath"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/plugins"
	editorplugin "focus/internal/plugins/editor"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/render"
	"focus/internal/store"
	"focus/internal/ui/footer"
	"focus/internal/ui/header"
	"focus/internal/ui/layout"
	"focus/internal/ui/pomodoro"
	"focus/internal/ui/shell"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
)

// page holds the pane collection, layout tree, and focus state for a single
// workspace page.
type page struct {
	common         *models.CommonModel
	pluginRegistry *plugins.Registry
	adapterManager *adapters.Manager

	panes       map[models.PaneID]models.Panel
	paneMeta    map[models.PaneID]models.PaneMeta
	paneOrder   []models.PaneID
	bodyTree    *layout.TreeNode
	frames      map[models.PaneID]models.PaneFrame
	focused     models.PaneID
	nextShell   int
	nextEditor  int
	returnFocus map[models.PaneID]models.PaneID

	zoomedPane  models.PaneID
	preZoomTree *layout.TreeNode

	snapshot    *PageSnapshot
	initialized bool
}

type PageSnapshot struct {
	BodyTreeJSON []byte        `json:"bodyTree"`
	Focused      models.PaneID `json:"focused"`
	OpenEditors  []string      `json:"openEditors"`
	ZoomedPane   models.PaneID `json:"zoomedPane,omitempty"`
	PreZoomTree  []byte        `json:"preZoomTree,omitempty"`
}

func pageSnapshotFromStore(s store.PageSnapshot) *PageSnapshot {
	ps := &PageSnapshot{
		Focused:     models.PaneID(s.Focused),
		OpenEditors: s.OpenEditors,
		ZoomedPane:  models.PaneID(s.ZoomedPane),
	}
	if len(s.BodyTreeJSON) > 0 {
		ps.BodyTreeJSON = append([]byte(nil), s.BodyTreeJSON...)
	}
	if len(s.PreZoomTree) > 0 {
		ps.PreZoomTree = append([]byte(nil), s.PreZoomTree...)
	}
	return ps
}

func (p *PageSnapshot) toStore() store.PageSnapshot {
	s := store.PageSnapshot{
		Focused:     string(p.Focused),
		OpenEditors: append([]string(nil), p.OpenEditors...),
		ZoomedPane:  string(p.ZoomedPane),
	}
	if len(p.BodyTreeJSON) > 0 {
		s.BodyTreeJSON = append([]byte(nil), p.BodyTreeJSON...)
	}
	if len(p.PreZoomTree) > 0 {
		s.PreZoomTree = append([]byte(nil), p.PreZoomTree...)
	}
	return s
}

func newPage(common *models.CommonModel, pluginRegistry *plugins.Registry, adapterManager *adapters.Manager) *page {
	return &page{
		common:         common,
		pluginRegistry: pluginRegistry,
		adapterManager: adapterManager,
		panes:          make(map[models.PaneID]models.Panel),
		paneMeta:       make(map[models.PaneID]models.PaneMeta),
		returnFocus:    make(map[models.PaneID]models.PaneID),
		nextShell:      2,
		nextEditor:     1,
	}
}

func newOverviewPage(common *models.CommonModel, pluginRegistry *plugins.Registry, adapterManager *adapters.Manager, cfg config.Config, store models.Store, cwd, repoRoot string) *page {
	p := newPage(common, pluginRegistry, adapterManager)

	worktreePaneMeta := models.PaneMeta{ID: paneWorktree, Name: "Worktrees", Type: models.PaneTypeWorktree, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	overviewSummaryMeta := models.PaneMeta{ID: paneOverviewSummary, Name: "Overview Summary", Type: paneTypeOverviewSummary, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	overviewDAGMeta := models.PaneMeta{ID: paneOverviewDAG, Name: "Task DAG", Type: paneTypeOverviewDAG, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	overviewDetailMeta := models.PaneMeta{ID: paneOverviewDetail, Name: "Context Detail", Type: paneTypeOverviewDetail, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	agentSessionMeta := models.PaneMeta{ID: paneAgentSession, Name: "Agents", Type: models.PaneTypeAgentSession, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}

	p.registerPane(paneHeader, header.New(cfg, common.Theme, store), models.PaneMeta{ID: paneHeader, Name: "Header", Type: models.PaneTypeHeader, Status: models.PaneStatusPassive, Closable: false})
	p.registerPane(paneShell, shell.New(common, paneShell), models.PaneMeta{ID: paneShell, Name: "Shell", Type: models.PaneTypeShell, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusStarting, Closable: true})
	p.registerPane(paneTodo, todo.New(common), models.PaneMeta{ID: paneTodo, Name: "Todo", Type: models.PaneTypeTodo, Status: models.PaneStatusIdle, Closable: false})
	p.registerPane(panePomodoro, pomodoro.New(common), models.PaneMeta{ID: panePomodoro, Name: "Pomodoro", Type: models.PaneTypePomodoro, Status: models.PaneStatusIdle, Closable: false})
	p.registerPane(paneFooter, footer.New(common), models.PaneMeta{ID: paneFooter, Name: "Footer", Type: models.PaneTypeFooter, Status: models.PaneStatusPassive, Closable: false})

	if repoRoot != "" {
		worktreePaneMeta.CWD = repoRoot
		worktreePaneMeta.RepoID = repoRoot
		worktreePaneMeta.WorktreeID = repoRoot
		overviewSummaryMeta.CWD = repoRoot
		overviewSummaryMeta.RepoID = repoRoot
		overviewSummaryMeta.WorktreeID = repoRoot
		overviewDAGMeta.CWD = repoRoot
		overviewDAGMeta.RepoID = repoRoot
		overviewDAGMeta.WorktreeID = repoRoot
		overviewDetailMeta.CWD = repoRoot
		overviewDetailMeta.RepoID = repoRoot
		overviewDetailMeta.WorktreeID = repoRoot
		agentSessionMeta.CWD = repoRoot
		agentSessionMeta.RepoID = repoRoot
		agentSessionMeta.WorktreeID = repoRoot
		if panel, err := pluginRegistry.CreatePane(models.PaneTypeWorktree, paneWorktree, worktreePaneMeta, *common); err == nil {
			p.registerPane(paneWorktree, panel, worktreePaneMeta)
		}
		if panel, err := pluginRegistry.CreatePane(models.PaneTypeAgentSession, paneAgentSession, agentSessionMeta, *common); err == nil {
			p.registerPane(paneAgentSession, panel, agentSessionMeta)
		}
		overviewRepoID := repoRoot
		if overviewRepoID == "" {
			overviewRepoID = cwd
		}
		overviewProvider := func() workbenchOverviewContext {
			if wp, ok := p.pane(paneWorktree).(*gitplugin.WorktreePane); ok {
				return buildWorkbenchOverviewContext(wp, common.Store, overviewRepoID)
			}
			return workbenchOverviewContext{}
		}
		p.registerPane(paneOverviewSummary, newOverviewSummaryPane(paneOverviewSummary, overviewSummaryMeta, overviewProvider), overviewSummaryMeta)
		p.registerPane(paneOverviewDAG, newOverviewDAGPane(paneOverviewDAG, overviewDAGMeta, overviewProvider), overviewDAGMeta)
		p.registerPane(paneOverviewDetail, newOverviewDetailPane(paneOverviewDetail, overviewDetailMeta, overviewProvider), overviewDetailMeta)
		p.bodyTree = layout.Split(
			layout.SplitVertical,
			74,
			layout.Split(layout.SplitVertical, 20,
				layout.Leaf(paneOverviewSummary),
				layout.Split(layout.SplitHorizontal, 36,
					layout.Leaf(paneWorktree),
					layout.Split(layout.SplitVertical, 54,
						layout.Leaf(paneOverviewDAG),
						layout.Split(layout.SplitHorizontal, 58, layout.Leaf(paneOverviewDetail), layout.Leaf(paneAgentSession)),
					),
				),
			),
			layout.Leaf(paneShell),
		)
	} else {
		p.bodyTree = layout.Leaf(paneShell)
	}
	if _, ok := p.paneMeta[paneWorktree]; ok {
		p.focused = paneWorktree
	} else {
		p.focused = paneShell
	}
	p.refreshPaneStatuses()
	return p
}

func newWorktreePage(common *models.CommonModel, pluginRegistry *plugins.Registry, adapterManager *adapters.Manager, cfg config.Config, store models.Store, worktreeID, repoRoot string) *page {
	p := newPage(common, pluginRegistry, adapterManager)

	gitPaneMeta := models.PaneMeta{ID: paneGitStatus, Name: "Git Status", Type: models.PaneTypeGitStatus, CWD: worktreeID, RepoID: repoRoot, WorktreeID: worktreeID, Status: models.PaneStatusIdle, Closable: false}
	fileTreeMeta := models.PaneMeta{ID: paneFileTree, Name: "File Tree", Type: models.PaneTypeFileTree, CWD: worktreeID, RepoID: repoRoot, WorktreeID: worktreeID, Status: models.PaneStatusIdle, Closable: false}
	agentSessionMeta := models.PaneMeta{ID: paneAgentSession, Name: "Agents", Type: models.PaneTypeAgentSession, CWD: worktreeID, RepoID: repoRoot, WorktreeID: worktreeID, Status: models.PaneStatusIdle, Closable: false}

	p.registerPane(paneHeader, header.New(cfg, common.Theme, store), models.PaneMeta{ID: paneHeader, Name: "Header", Type: models.PaneTypeHeader, Status: models.PaneStatusPassive, Closable: false})
	p.registerPane(paneShell, shell.NewWithCWD(common, paneShell, worktreeID), models.PaneMeta{ID: paneShell, Name: "Shell", Type: models.PaneTypeShell, CWD: worktreeID, RepoID: repoRoot, WorktreeID: worktreeID, Status: models.PaneStatusStarting, Closable: true})
	p.registerPane(paneTodo, todo.New(common), models.PaneMeta{ID: paneTodo, Name: "Todo", Type: models.PaneTypeTodo, Status: models.PaneStatusIdle, Closable: false})
	p.registerPane(panePomodoro, pomodoro.New(common), models.PaneMeta{ID: panePomodoro, Name: "Pomodoro", Type: models.PaneTypePomodoro, Status: models.PaneStatusIdle, Closable: false})
	p.registerPane(paneFooter, footer.New(common), models.PaneMeta{ID: paneFooter, Name: "Footer", Type: models.PaneTypeFooter, Status: models.PaneStatusPassive, Closable: false})

	if panel, err := pluginRegistry.CreatePane(models.PaneTypeFileTree, paneFileTree, fileTreeMeta, *common); err == nil {
		p.registerPane(paneFileTree, panel, fileTreeMeta)
	}

	if panel, err := pluginRegistry.CreatePane(models.PaneTypeAgentSession, paneAgentSession, agentSessionMeta, *common); err == nil {
		p.registerPane(paneAgentSession, panel, agentSessionMeta)
	}

	if repoRoot != "" {
		if panel, err := pluginRegistry.CreatePane(models.PaneTypeGitStatus, paneGitStatus, gitPaneMeta, *common); err == nil {
			p.registerPane(paneGitStatus, panel, gitPaneMeta)
		}
	}
	if _, hasGitStatus := p.paneMeta[paneGitStatus]; hasGitStatus {
		p.bodyTree = layout.Split(
			layout.SplitHorizontal,
			30,
			layout.Split(layout.SplitVertical, 50, layout.Leaf(paneGitStatus), layout.Leaf(paneFileTree)),
			layout.Split(layout.SplitVertical, 50, layout.Leaf(paneAgentSession), layout.Leaf(paneShell)),
		)
	} else {
		p.bodyTree = layout.Split(
			layout.SplitHorizontal,
			30,
			layout.Leaf(paneFileTree),
			layout.Split(layout.SplitVertical, 50, layout.Leaf(paneAgentSession), layout.Leaf(paneShell)),
		)
	}
	p.focused = paneShell
	p.refreshPaneStatuses()
	return p
}

func (p *page) registerPane(id models.PaneID, panel models.Panel, meta models.PaneMeta) {
	p.panes[id] = panel
	p.paneMeta[id] = meta
	p.paneOrder = append(p.paneOrder, id)
}

func (p *page) pane(id models.PaneID) models.Panel {
	return p.panes[id]
}

func (p *page) setPane(id models.PaneID, panel models.Panel) {
	p.panes[id] = panel
	p.syncPaneMeta(id)
}

func (p *page) setFocus(id models.PaneID) {
	if id == "" {
		return
	}
	if _, ok := p.paneMeta[id]; !ok {
		return
	}
	p.focused = id
	p.refreshPaneStatuses()
}

func (p *page) refreshPaneStatuses() {
	for id, meta := range p.paneMeta {
		if editorPane, ok := p.pane(id).(editorMetaProvider); ok {
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
			if sh, ok := p.pane(id).(*shell.Model); ok {
				meta.Status = sh.SessionStatus()
			} else {
				meta.Status = models.PaneStatusStarting
			}
		default:
			if id == p.focused {
				meta.Status = models.PaneStatusReady
			} else {
				meta.Status = models.PaneStatusIdle
			}
		}
		p.paneMeta[id] = meta
	}
}

func (p *page) syncPaneMeta(id models.PaneID) {
	meta, ok := p.paneMeta[id]
	if !ok {
		return
	}
	if editorPane, ok := p.pane(id).(editorMetaProvider); ok {
		meta.Name = editorPane.DisplayName()
		if editorPane.Dirty() {
			meta.Name = "*" + meta.Name
		}
		if filePath := editorPane.FilePath(); filePath != "" {
			meta.CWD = filepath.Dir(filePath)
		}
		p.paneMeta[id] = meta
	}
}

func (p *page) nextShellPaneID() models.PaneID {
	id := models.PaneID(fmt.Sprintf("shell-%d", p.nextShell))
	p.nextShell++
	return id
}

func (p *page) nextEditorPaneID() models.PaneID {
	id := models.PaneID(fmt.Sprintf("editor-%d", p.nextEditor))
	p.nextEditor++
	return id
}

func (p *page) createShellPane() (models.PaneID, tea.Cmd) {
	return p.createShellPaneFor(p.currentCWD(), p.currentRepoID(), p.currentWorktreeID(), p.currentBranchSnapshot())
}

func (p *page) createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot string) (models.PaneID, tea.Cmd) {
	id := p.nextShellPaneID()
	panel := shell.NewWithCWD(p.common, id, cwd)
	meta := models.PaneMeta{
		ID:             id,
		Name:           fmt.Sprintf("Shell %d", p.nextShell-1),
		Type:           models.PaneTypeShell,
		CWD:            cwd,
		RepoID:         repoID,
		WorktreeID:     worktreeID,
		BranchSnapshot: branchSnapshot,
		Status:         models.PaneStatusStarting,
		Closable:       true,
	}
	p.registerPane(id, panel, meta)
	if frame, ok := p.frames[p.focused]; ok {
		contentW := max(frame.W-4, 8)
		contentH := max(frame.H-2, 3)
		panel.SetSize(contentW, contentH)
	}
	return id, panel.Init()
}

func (p *page) currentCWD() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.CWD != "" {
		return meta.CWD
	}
	cwd, _ := os.Getwd()
	return cwd
}

func (p *page) currentRepoID() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.RepoID != "" {
		return meta.RepoID
	}
	if meta, ok := p.paneMeta[paneGitStatus]; ok && meta.RepoID != "" {
		return meta.RepoID
	}
	return p.currentCWD()
}

func (p *page) currentWorktreeID() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.WorktreeID != "" {
		return meta.WorktreeID
	}
	if meta, ok := p.paneMeta[paneGitStatus]; ok && meta.WorktreeID != "" {
		return meta.WorktreeID
	}
	return p.currentCWD()
}

func (p *page) currentBranchSnapshot() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.BranchSnapshot != "" {
		return meta.BranchSnapshot
	}
	if meta, ok := p.paneMeta[paneGitStatus]; ok {
		return meta.BranchSnapshot
	}
	return ""
}

func (p *page) gitRepoPath() string {
	if meta, ok := p.paneMeta[paneGitStatus]; ok && meta.RepoID != "" {
		return meta.RepoID
	}
	return p.currentCWD()
}

func (p *page) openEditorPane(msg editorplugin.OpenEditorMsg) tea.Cmd {
	opener := p.focused
	filePath := filepath.Clean(msg.FilePath)
	if existing := p.findEditorPaneByPath(filePath); existing != "" {
		p.setFocus(existing)
		if msg.LineNumber > 0 {
			newPanel, cmd := p.pane(existing).Update(editorplugin.OpenEditorMsg{FilePath: filePath, LineNumber: msg.LineNumber})
			p.setPane(existing, newPanel)
			p.syncPaneMeta(existing)
			return cmd
		}
		return nil
	}
	id := p.nextEditorPaneID()
	meta := models.PaneMeta{
		ID:             id,
		Name:           filepath.Base(filePath),
		Type:           models.PaneTypeEditor,
		CWD:            filepath.Dir(filePath),
		RepoID:         p.currentRepoID(),
		WorktreeID:     p.currentWorktreeID(),
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := editorplugin.NewEditorPane(id, meta, *p.common, filePath, msg.LineNumber)
	p.registerPane(id, panel, meta)
	if opener != "" && opener != id {
		if p.returnFocus == nil {
			p.returnFocus = make(map[models.PaneID]models.PaneID)
		}
		p.returnFocus[id] = opener
	}

	target := p.editorHostPaneTarget(opener, msg.Behavior)
	direction := p.editorSplitDirection(target, msg.Behavior)
	p.bodyTree = layout.SplitLeaf(p.bodyTree, target, id, direction, true)
	p.setFocus(id)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) findEditorPaneByPath(filePath string) models.PaneID {
	currentWorktreeID := p.currentWorktreeID()
	for _, id := range layout.LeafOrder(p.bodyTree) {
		panel := p.pane(id)
		editorPane, ok := panel.(editorMetaProvider)
		if !ok {
			continue
		}
		meta := p.paneMeta[id]
		if editorPane.FilePath() == filePath && meta.WorktreeID == currentWorktreeID {
			return id
		}
	}
	return ""
}

func (p *page) editorHostPaneTarget(opener models.PaneID, behavior editorplugin.OpenBehavior) models.PaneID {
	if meta, ok := p.paneMeta[opener]; ok && (meta.Type == models.PaneTypeEditor || meta.Type == models.PaneTypeShell) {
		return opener
	}
	if editor := p.lastEditorPane(); editor != "" {
		return editor
	}
	if _, ok := p.paneMeta[paneShell]; ok {
		return paneShell
	}
	if behavior == editorplugin.OpenBehaviorVSplit {
		return opener
	}
	return opener
}

func (p *page) lastEditorPane() models.PaneID {
	order := layout.LeafOrder(p.bodyTree)
	for idx := len(order) - 1; idx >= 0; idx-- {
		id := order[idx]
		if meta, ok := p.paneMeta[id]; ok && meta.Type == models.PaneTypeEditor {
			return id
		}
	}
	return ""
}

func (p *page) editorSplitDirection(target models.PaneID, behavior editorplugin.OpenBehavior) layout.SplitDirection {
	if behavior == editorplugin.OpenBehaviorVSplit {
		return layout.SplitHorizontal
	}
	if meta, ok := p.paneMeta[target]; ok && meta.Type == models.PaneTypeEditor {
		return layout.SplitVertical
	}
	return layout.SplitHorizontal
}

func (p *page) splitFocused(direction layout.SplitDirection) tea.Cmd {
	newID, cmd := p.createShellPane()
	p.bodyTree = layout.SplitLeaf(p.bodyTree, p.focused, newID, direction, true)
	p.setFocus(newID)
	p.updateSizes(p.bodyBoundsSize())
	return cmd
}

func (p *page) adjustFocusedSplit(direction layout.FocusDirection) bool {
	if p.focused == "" {
		return false
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

	if !layout.AdjustSplitRatio(p.bodyTree, p.bodyBoundsSize(), p.focused, splitDirection, delta) {
		return false
	}
	p.updateSizes(p.bodyBoundsSize())
	return true
}

func (p *page) closeFocusedPane() {
	meta, ok := p.paneMeta[p.focused]
	if !ok || !meta.Closable {
		return
	}
	p.closePane(p.focused)
}

func (p *page) focusCycle(delta int) {
	if p.zoomedPane != "" {
		p.restoreZoom()
	}
	order := layout.LeafOrder(p.bodyTree)
	if len(order) == 0 {
		return
	}
	idx := 0
	for i, id := range order {
		if id == p.focused {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(order)) % len(order)
	p.setFocus(order[idx])
}

func (p *page) toggleZoom() {
	if p.zoomedPane != "" {
		p.restoreZoom()
	} else {
		if p.focused == "" || p.focused == paneHeader || p.focused == paneFooter {
			return
		}
		p.preZoomTree = p.bodyTree
		p.bodyTree = layout.Leaf(p.focused)
		p.zoomedPane = p.focused
		p.updateSizes(p.bodyBoundsSize())
	}
}

func (p *page) restoreZoom() {
	if p.zoomedPane == "" {
		return
	}
	if p.preZoomTree != nil {
		p.bodyTree = p.preZoomTree
		p.preZoomTree = nil
	}
	p.zoomedPane = ""
	p.updateSizes(p.bodyBoundsSize())
}

func (p *page) removePaneOrder(id models.PaneID) {
	filtered := p.paneOrder[:0]
	for _, existing := range p.paneOrder {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	p.paneOrder = filtered
}

func (p *page) activeOverlayPane() models.PaneID {
	if _, ok := p.paneMeta[paneWorktreeCreate]; ok {
		return paneWorktreeCreate
	}
	if _, ok := p.paneMeta[paneGitCommit]; ok {
		return paneGitCommit
	}
	if _, ok := p.paneMeta[paneTaskEdit]; ok {
		return paneTaskEdit
	}
	if _, ok := p.paneMeta[panePlanEdit]; ok {
		return panePlanEdit
	}
	if _, ok := p.paneMeta[paneAgentSelect]; ok {
		return paneAgentSelect
	}
	return ""
}

func (p *page) isOverlayPane(id models.PaneID) bool {
	return id == paneGitCommit || id == paneWorktreeCreate || id == paneTaskEdit || id == panePlanEdit || id == paneAgentSelect
}

func (p *page) paneAt(x, y int) models.PaneID {
	for _, id := range layout.LeafOrder(p.bodyTree) {
		frame, ok := p.frames[id]
		if !ok {
			continue
		}
		if x >= frame.X && x < frame.X+frame.W && y >= frame.Y && y < frame.Y+frame.H {
			return id
		}
	}
	return ""
}

func (p *page) routeToPane(id models.PaneID, msg tea.Msg) tea.Cmd {
	panel := p.pane(id)
	if panel == nil {
		return nil
	}
	newPanel, cmd := panel.Update(msg)
	p.setPane(id, newPanel)
	p.refreshPaneStatuses()
	return cmd
}

func (p *page) updateSizes(bounds models.PaneFrame) {
	p.frames = layout.ComputeFrames(p.bodyTree, bounds)
	for id, frame := range p.frames {
		contentW := frame.W - 4
		if contentW < 8 {
			contentW = 8
		}
		contentH := frame.H - 2
		if contentH < 3 {
			contentH = 3
		}
		p.pane(id).SetSize(contentW, contentH)
	}
}

func (p *page) bodyBounds(w, h int) models.PaneFrame {
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

func (p *page) bodyBoundsSize() models.PaneFrame {
	w := p.common.Width
	h := p.common.Height
	return p.bodyBounds(w, h)
}

func (p *page) renderBody(w, h int, overlay OverlayKind) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 6
	}
	if p.common.Cfg.Experimental.UseCanvasCompositor {
		return p.renderBodyCanvas(w, h, overlay)
	}
	base := blankCanvas(w, h)
	for _, id := range layout.LeafOrder(p.bodyTree) {
		frame, ok := p.frames[id]
		if !ok {
			continue
		}
		panel := p.pane(id)
		active := id == p.focused
		content := panel.View()
		title := p.renderPaneTitle(id, p.focused, ModeNormal, max(frame.W-4, 8))
		panelView := layout.RenderPanel(title, content, max(frame.W-4, 8), max(frame.H-2, 3), active)
		base = layout.OverlayOnBase(base, panelView, frame.X, frame.Y)
	}

	if overlay == OverlayPicker {
		if frame, ok := p.frames[panePomodoro]; ok {
			pomo := p.pane(panePomodoro).(*pomodoro.Model)
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

	if overlayID := p.activeOverlayPane(); overlayID != "" {
		base = p.renderOverlayPane(base, overlayID)
	}

	return base
}

func (p *page) renderBodyCanvas(w, h int, overlay OverlayKind) string {
	canvas := render.NewCanvas(w, h)
	canvas.Clear()
	for _, id := range layout.LeafOrder(p.bodyTree) {
		frame, ok := p.frames[id]
		if !ok {
			continue
		}
		panel := p.pane(id)
		active := id == p.focused
		title := p.renderPaneTitle(id, p.focused, ModeNormal, max(frame.W-4, 8))
		sub := canvas.SubCanvas(frame.X, frame.Y, frame.W, frame.H)
		if renderer, ok := panel.(render.Renderer); ok {
			renderer.Render(sub, frame.W, frame.H)
		} else {
			panel.SetSize(max(frame.W-4, 8), max(frame.H-2, 3))
			content := panel.View()
			render.RenderPane(sub, title, content, active)
		}
	}

	if overlay == OverlayPicker {
		if frame, ok := p.frames[panePomodoro]; ok {
			pomo := p.pane(panePomodoro).(*pomodoro.Model)
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
			x := frame.X + (frame.W-(overlayW+4))/2
			y := frame.Y + (frame.H-(overlayH+2))/2
			overlaySub := canvas.SubCanvas(x, y, overlayW+4, overlayH+2)
			render.RenderPane(overlaySub, "SELECT TASK", pomo.PickerView(), true)
		}
	}

	if overlayID := p.activeOverlayPane(); overlayID != "" {
		p.renderOverlayPaneToCanvas(canvas, overlayID)
	}

	return canvas.Render()
}

func (p *page) renderPaneTitle(id models.PaneID, focused models.PaneID, mode AppMode, contentWidth int) string {
	meta := p.paneMeta[id]
	shellActive := mode == ModeShell && meta.Type == models.PaneTypeShell
	return formatPaneTitle(meta, id == focused, shellActive, contentWidth)
}

func (p *page) overlayContentSize() (int, int) {
	bounds := p.bodyBoundsSize()
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

func (p *page) renderOverlayPane(base string, id models.PaneID) string {
	panel := p.pane(id)
	if panel == nil {
		return base
	}

	overlayW, overlayH := p.overlayContentSize()
	overlayView := layout.RenderPanel(p.renderPaneTitle(id, p.focused, ModeNormal, overlayW), panel.View(), overlayW, overlayH, true)
	bounds := p.bodyBoundsSize()
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

func (p *page) renderOverlayPaneToCanvas(canvas *render.Canvas, id models.PaneID) {
	panel := p.pane(id)
	if panel == nil {
		return
	}

	overlayW, overlayH := p.overlayContentSize()
	bounds := p.bodyBoundsSize()
	x := bounds.X + (bounds.W-(overlayW+4))/2
	y := bounds.Y + (bounds.H-(overlayH+2))/2
	if x < bounds.X {
		x = bounds.X
	}
	if y < bounds.Y {
		y = bounds.Y
	}

	sub := canvas.SubCanvas(x, y, overlayW+4, overlayH+2)
	if renderer, ok := panel.(render.Renderer); ok {
		renderer.Render(sub, overlayW+4, overlayH+2)
	} else {
		panel.SetSize(overlayW, overlayH)
		content := panel.View()
		render.RenderPane(sub, p.renderPaneTitle(id, p.focused, ModeNormal, overlayW), content, true)
	}
}

func (p *page) openDiffPane(msg gitplugin.OpenDiffMsg) tea.Cmd {
	opener := p.focused
	target := p.reviewHostPaneTarget(opener)
	p.closePane(paneGitCommit)
	p.closePane(paneGitDiff)

	meta := models.PaneMeta{
		ID:             paneGitDiff,
		Name:           "Diff",
		Type:           models.PaneTypeDiffView,
		CWD:            p.gitRepoPath(),
		RepoID:         p.currentRepoID(),
		WorktreeID:     p.currentWorktreeID(),
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := gitplugin.NewDiffPane(meta.ID, meta, *p.common, p.adapterManager.Git(), msg.FilePath, msg.Staged)
	p.registerPane(meta.ID, panel, meta)
	if opener != "" && opener != paneGitDiff {
		if p.returnFocus == nil {
			p.returnFocus = make(map[models.PaneID]models.PaneID)
		}
		p.returnFocus[paneGitDiff] = opener
	}
	p.bodyTree = layout.SplitLeaf(p.bodyTree, target, paneGitDiff, p.reviewSplitDirection(target), true)
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) reviewHostPaneTarget(opener models.PaneID) models.PaneID {
	if meta, ok := p.paneMeta[opener]; ok && (meta.Type == models.PaneTypeEditor || meta.Type == models.PaneTypeShell) {
		return opener
	}
	if editor := p.lastEditorPane(); editor != "" {
		return editor
	}
	if _, ok := p.paneMeta[paneShell]; ok {
		return paneShell
	}
	if opener != "" {
		return opener
	}
	return paneGitStatus
}

func (p *page) reviewSplitDirection(target models.PaneID) layout.SplitDirection {
	if meta, ok := p.paneMeta[target]; ok && meta.Type == models.PaneTypeEditor {
		return layout.SplitVertical
	}
	return layout.SplitHorizontal
}

func (p *page) openCommitPane(msg gitplugin.OpenCommitMsg) tea.Cmd {
	p.closePane(paneGitDiff)
	p.closePane(paneGitCommit)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneGitStatus
	}

	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = p.gitRepoPath()
	}

	meta := models.PaneMeta{
		ID:             paneGitCommit,
		Name:           "Commit",
		Type:           paneTypeGitCommit,
		CWD:            repoPath,
		RepoID:         p.currentRepoID(),
		WorktreeID:     p.currentWorktreeID(),
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := gitplugin.NewCommitPane(meta.ID, meta, *p.common, p.adapterManager.Git(), msg.StagedFiles)
	p.registerPane(meta.ID, panel, meta)
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openCreateWorktreePane(msg gitplugin.OpenCreateWorktreeMsg) tea.Cmd {
	p.closePane(paneWorktreeCreate)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = p.gitRepoPath()
	}

	meta := models.PaneMeta{
		ID:             paneWorktreeCreate,
		Name:           "Create Worktree",
		Type:           paneTypeWorktreeCreate,
		CWD:            repoPath,
		RepoID:         p.currentRepoID(),
		WorktreeID:     p.currentWorktreeID(),
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := gitplugin.NewWorktreeCreatePane(meta.ID, meta, *p.common, p.adapterManager.Git(), msg)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openAgentSelectPane(worktreeID string) tea.Cmd {
	p.closePane(paneAgentSelect)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:         paneAgentSelect,
		Name:       "Select Agent",
		Type:       paneTypeAgentSelect,
		CWD:        worktreeID,
		RepoID:     p.currentRepoID(),
		WorktreeID: worktreeID,
		Status:     models.PaneStatusReady,
		Closable:   true,
	}
	panel := newAgentSelectPane(meta.ID, meta, *p.common, worktreeID)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openTaskEditPane(seed taskEditorSeed) tea.Cmd {
	p.closePane(paneTaskEdit)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:             paneTaskEdit,
		Name:           "Task Context",
		Type:           paneTypeTaskEdit,
		CWD:            p.gitRepoPath(),
		RepoID:         p.currentRepoID(),
		WorktreeID:     seed.WorktreeID,
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := NewTaskEditPane(meta.ID, meta, *p.common, seed)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openPlanEditPane(seed planEditorSeed) tea.Cmd {
	p.closePane(panePlanEdit)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:             panePlanEdit,
		Name:           "Plan Draft",
		Type:           paneTypePlanEdit,
		CWD:            p.gitRepoPath(),
		RepoID:         p.currentRepoID(),
		WorktreeID:     seed.WorktreeID,
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := NewPlanEditPane(meta.ID, meta, *p.common, seed)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) closePanesForWorktree(worktreePath string) {
	if worktreePath == "" {
		return
	}
	ids := append([]models.PaneID(nil), p.paneOrder...)
	for _, id := range ids {
		meta, ok := p.paneMeta[id]
		if !ok || !meta.Closable {
			continue
		}
		if meta.WorktreeID != worktreePath {
			continue
		}
		p.closePane(id)
	}
}

func (p *page) closePane(id models.PaneID) {
	meta, ok := p.paneMeta[id]
	if !ok || !meta.Closable {
		return
	}
	if p.zoomedPane == id {
		p.restoreZoom()
	}

	if sh, ok := p.pane(id).(*shell.Model); ok {
		_ = sh.Close()
	}
	if st, ok := p.pane(id).(*gitplugin.StatusPane); ok {
		st.StopWatch()
	}

	wasFocused := p.focused == id
	preferred := p.returnFocus[id]
	var fallback models.PaneID
	leafOrder := layout.LeafOrder(p.bodyTree)
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
			fallback = layout.CloseFocusFallback(id, p.frames)
		}
		p.bodyTree = layout.RemoveLeaf(p.bodyTree, id)
	}

	delete(p.panes, id)
	delete(p.paneMeta, id)
	p.removePaneOrder(id)
	if p.returnFocus != nil {
		delete(p.returnFocus, id)
		for paneID, target := range p.returnFocus {
			if target == id {
				delete(p.returnFocus, paneID)
			}
		}
	}

	if wasFocused {
		restoredPreferred := false
		if preferred != "" {
			if _, ok := p.paneMeta[preferred]; ok && preferred != id {
				p.setFocus(preferred)
				restoredPreferred = true
			}
		}
		if !restoredPreferred && fallback != "" && fallback != id {
			p.setFocus(fallback)
		} else if !restoredPreferred {
			remaining := layout.LeafOrder(p.bodyTree)
			if len(remaining) > 0 {
				p.setFocus(remaining[0])
			}
		}
	}

	p.updateSizes(p.bodyBoundsSize())
	p.refreshPaneStatuses()
}

func (p *page) closeShellPanes() {
	for id, meta := range p.paneMeta {
		if meta.Type != models.PaneTypeShell {
			continue
		}
		if sh, ok := p.pane(id).(*shell.Model); ok {
			_ = sh.Close()
		}
	}
}

func (p *page) openWorktreeShell(msg gitplugin.OpenWorktreeShellMsg) tea.Cmd {
	cwd := msg.Worktree.Path
	if cwd == "" {
		cwd = p.currentCWD()
	}
	worktreeID := cwd

	if meta, ok := p.paneMeta[paneShell]; ok && meta.WorktreeID == worktreeID {
		p.setFocus(paneShell)
		return nil
	}
	for id, meta := range p.paneMeta {
		if meta.Type == models.PaneTypeShell && meta.WorktreeID == worktreeID {
			p.setFocus(id)
			return nil
		}
	}

	repoID := p.currentRepoID()
	if repoID == "" {
		repoID = p.gitRepoPath()
	}
	branchSnapshot := msg.Worktree.Branch

	id, cmd := p.createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot)
	p.bodyTree = layout.SplitLeaf(p.bodyTree, p.focused, id, layout.SplitHorizontal, true)
	p.setFocus(id)
	p.updateSizes(p.bodyBoundsSize())
	return cmd
}

func (p *page) focusAgentShell(worktreeID string) tea.Cmd {
	for id, meta := range p.paneMeta {
		if meta.Type == models.PaneTypeShell && meta.WorktreeID == worktreeID {
			p.setFocus(id)
			return nil
		}
	}
	return nil
}

func (p *page) openAgentShell(worktreeID string, provider agents.Provider, sessionID string) tea.Cmd {
	repoID := p.currentRepoID()
	if repoID == "" {
		repoID = p.gitRepoPath()
	}

	cmdStr := agents.AutoTypeCommandWithSession(provider, sessionID)
	id := p.nextShellPaneID()
	panel := shell.NewWithCommand(p.common, id, worktreeID, cmdStr)
	meta := models.PaneMeta{
		ID:         id,
		Name:       fmt.Sprintf("Shell %d", p.nextShell-1),
		Type:       models.PaneTypeShell,
		CWD:        worktreeID,
		RepoID:     repoID,
		WorktreeID: worktreeID,
		Status:     models.PaneStatusStarting,
		Closable:   true,
	}
	p.registerPane(id, panel, meta)
	if frame, ok := p.frames[p.focused]; ok {
		contentW := max(frame.W-4, 8)
		contentH := max(frame.H-2, 3)
		panel.SetSize(contentW, contentH)
	}
	p.bodyTree = layout.SplitLeaf(p.bodyTree, p.focused, id, layout.SplitHorizontal, true)
	p.setFocus(id)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) captureSnapshot() *PageSnapshot {
	bodyJSON, _ := layout.SerializeTree(p.bodyTree)
	preZoomJSON, _ := layout.SerializeTree(p.preZoomTree)
	var editors []string
	for _, id := range layout.LeafOrder(p.bodyTree) {
		if meta, ok := p.paneMeta[id]; ok && meta.Type == models.PaneTypeEditor {
			if ep, ok := p.pane(id).(editorMetaProvider); ok {
				editors = append(editors, ep.FilePath())
			}
		}
	}
	return &PageSnapshot{
		BodyTreeJSON: bodyJSON,
		Focused:      p.focused,
		OpenEditors:  editors,
		ZoomedPane:   p.zoomedPane,
		PreZoomTree:  preZoomJSON,
	}
}

func (p *page) restoreSnapshot() {
	if p.snapshot == nil {
		return
	}
	if len(p.snapshot.BodyTreeJSON) > 0 {
		if tree, err := layout.DeserializeTree(p.snapshot.BodyTreeJSON); err == nil && tree != nil {
			p.bodyTree = tree
		}
	}
	for _, id := range layout.LeafOrder(p.bodyTree) {
		if _, ok := p.panes[id]; !ok {
			p.bodyTree = layout.RemoveLeaf(p.bodyTree, id)
		}
	}
	if p.snapshot.Focused != "" {
		if _, ok := p.paneMeta[p.snapshot.Focused]; ok {
			p.focused = p.snapshot.Focused
		}
	}
	if p.snapshot.ZoomedPane != "" {
		if preZoom, err := layout.DeserializeTree(p.snapshot.PreZoomTree); err == nil && preZoom != nil {
			p.zoomedPane = p.snapshot.ZoomedPane
			p.preZoomTree = preZoom
		}
	}
}

func (m *model) loadPageSnapshots() {
	if m.common.Store == nil {
		return
	}
	records, err := m.common.Store.ListPageSnapshots()
	if err != nil {
		return
	}
	for _, r := range records {
		if r.WorktreeID == "" {
			continue
		}
		ss, err := store.UnmarshalPageSnapshot([]byte(r.SnapshotJSON))
		if err != nil {
			continue
		}
		repoRoot := m.gitRepoPath()
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, _ = gitRepoRoot(cwd)
		}
		p := newWorktreePage(m.common, m.pluginRegistry, m.adapterManager, m.common.Cfg, m.common.Store, r.WorktreeID, repoRoot)
		p.snapshot = pageSnapshotFromStore(ss)
		m.pages[r.WorktreeID] = p
	}
}

func (m *model) persistActivePageSnapshot() {
	if m.common.Store == nil || m.activePage == nil {
		return
	}
	var worktreeID string
	for id, p := range m.pages {
		if p == m.activePage && id != "" {
			worktreeID = id
			break
		}
	}
	if worktreeID == "" {
		return
	}
	m.activePage.snapshot = m.activePage.captureSnapshot()
	data, err := store.MarshalPageSnapshot(m.activePage.snapshot.toStore())
	if err != nil {
		return
	}
	_ = m.common.Store.SavePageSnapshot(worktreeID, data)
}

func (m *model) switchToOverviewPage() {
	m.persistActivePageSnapshot()
	m.touchActiveWorktreeContext()
	if m.activePage != nil {
		m.activePage.snapshot = m.activePage.captureSnapshot()
	}
	m.state = StateOverviewPage
	m.currentWorktreePage = ""
	m.activePage = m.pages[""]
	if m.activePage == nil {
		cwd, _ := os.Getwd()
		repoRoot, _ := gitRepoRoot(cwd)
		m.activePage = newOverviewPage(m.common, m.pluginRegistry, m.adapterManager, m.common.Cfg, m.common.Store, cwd, repoRoot)
		m.pages[""] = m.activePage
	}
	if m.activePage.snapshot != nil {
		m.activePage.restoreSnapshot()
	}
	if m.activePage.paneMeta[paneWorktree].ID != "" {
		m.activePage.setFocus(paneWorktree)
	}
	if m.mode == ModeShell {
		m.mode = ModeNormal
	}
	if m.activePage.zoomedPane != "" {
		m.activePage.restoreZoom()
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
}

func (m *model) switchToWorktreePage(worktreeID, preferredPane string) tea.Cmd {
	if worktreeID == "" {
		m.switchToOverviewPage()
		return nil
	}
	m.persistActivePageSnapshot()
	m.touchActiveWorktreeContext()
	if m.activePage != nil {
		m.activePage.snapshot = m.activePage.captureSnapshot()
	}
	m.state = StateWorktreePage
	m.currentWorktreePage = worktreeID
	var initCmds []tea.Cmd
	if p, ok := m.pages[worktreeID]; ok && p != nil {
		m.activePage = p
	} else {
		repoRoot := m.gitRepoPath()
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, _ = gitRepoRoot(cwd)
		}
		p = newWorktreePage(m.common, m.pluginRegistry, m.adapterManager, m.common.Cfg, m.common.Store, worktreeID, repoRoot)
		m.pages[worktreeID] = p
		m.activePage = p
	}
	if !m.activePage.initialized {
		m.activePage.initialized = true
		for _, id := range m.activePage.paneOrder {
			if cmd := m.activePage.pane(id).Init(); cmd != nil {
				initCmds = append(initCmds, cmd)
			}
		}
	}
	m.touchWorktreeContext(worktreeID)
	if m.activePage.snapshot != nil {
		m.activePage.restoreSnapshot()
	}
	if preferredPane != "" {
		m.activePage.setFocus(models.PaneID(preferredPane))
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	if len(initCmds) == 0 {
		return nil
	}
	return tea.Batch(initCmds...)
}
