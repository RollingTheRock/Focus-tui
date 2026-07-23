package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/adapters"
	"github.com/RollingTheRock/Focus-tui/internal/agents"
	"github.com/RollingTheRock/Focus-tui/internal/config"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/plugins"
	editorplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/editor"
	gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"
	gitfiletree "github.com/RollingTheRock/Focus-tui/internal/plugins/gitfiletree"
	"github.com/RollingTheRock/Focus-tui/internal/render"
	"github.com/RollingTheRock/Focus-tui/internal/store"
	"github.com/RollingTheRock/Focus-tui/internal/ui/footer"
	"github.com/RollingTheRock/Focus-tui/internal/ui/header"
	"github.com/RollingTheRock/Focus-tui/internal/ui/layout"
	"github.com/RollingTheRock/Focus-tui/internal/ui/shell"
	"github.com/RollingTheRock/Focus-tui/internal/ui/todo"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// page holds the pane collection, layout tree, and focus state for a single
// workspace page.
type page struct {
	common         *models.CommonModel
	pluginRegistry *plugins.Registry
	adapterManager *adapters.Manager
	repoRoot       string

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

	// overlayPaneSet  and overlayPaneOrder replace the hard-coded
	// activeOverlayPane()/isOverlayPane() if-else chains.
	overlayPaneSet   map[models.PaneID]struct{}
	overlayPaneOrder []models.PaneID

	// Cache for active sessions to avoid DB queries on the render path
	activeSessionCache   map[string]bool
	activeSessionCacheTs time.Time
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

func newPage(common *models.CommonModel, pluginRegistry *plugins.Registry, adapterManager *adapters.Manager, repoRoot string) *page {
	p := &page{
		common:             common,
		pluginRegistry:     pluginRegistry,
		adapterManager:     adapterManager,
		repoRoot:           repoRoot,
		panes:              make(map[models.PaneID]models.Panel),
		paneMeta:           make(map[models.PaneID]models.PaneMeta),
		returnFocus:        make(map[models.PaneID]models.PaneID),
		nextShell:          2,
		nextEditor:         1,
		activeSessionCache: make(map[string]bool),
	}
	p.initOverlayRegistry()
	return p
}

func newOverviewPage(common *models.CommonModel, pluginRegistry *plugins.Registry, adapterManager *adapters.Manager, cfg config.Config, store models.Store, cwd, repoRoot string) *page {
	p := newPage(common, pluginRegistry, adapterManager, repoRoot)

	dagMeta := models.PaneMeta{ID: paneDAG, Name: "DAG", Type: models.PaneTypeWorktree, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	worktreeMeta := models.PaneMeta{ID: paneWorktree, Name: "Worktrees", Type: models.PaneTypeWorktree, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}
	detailMeta := models.PaneMeta{ID: paneWorktreeDetail, Name: "Worktree Detail", Type: models.PaneTypeWorktree, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusIdle, Closable: false}

	p.registerPane(paneHeader, header.New(cfg, common.Theme, store), models.PaneMeta{ID: paneHeader, Name: "Header", Type: models.PaneTypeHeader, Status: models.PaneStatusPassive, Closable: false})
	p.registerPane(paneShell, shell.New(common, paneShell), models.PaneMeta{ID: paneShell, Name: "Shell", Type: models.PaneTypeShell, CWD: cwd, RepoID: cwd, WorktreeID: cwd, Status: models.PaneStatusStarting, Closable: true})
	p.registerPane(paneFooter, footer.New(common), models.PaneMeta{ID: paneFooter, Name: "Footer", Type: models.PaneTypeFooter, Status: models.PaneStatusPassive, Closable: false})

	if repoRoot != "" {
		dagMeta.CWD = repoRoot
		dagMeta.RepoID = repoRoot
		worktreeMeta.CWD = repoRoot
		worktreeMeta.RepoID = repoRoot
		detailMeta.CWD = repoRoot
		detailMeta.RepoID = repoRoot
	}

	// DAG pane (tab container: Tasks + ADRs)
	dag := newDagPane(paneDAG, dagMeta, common, repoRoot, adapterManager.Git())
	adr := newAdrPane(paneDAG+"-adr", dagMeta, common, repoRoot)
	tc := newTabContainer(paneDAG, dagMeta, common, dag, adr)
	p.registerPane(paneDAG, tc, dagMeta)

	// Worktree list pane (via plugin registry so it gets the real GitAdapter)
	if panel, err := pluginRegistry.CreatePane(models.PaneTypeWorktree, paneWorktree, worktreeMeta, *common); err == nil {
		p.registerPane(paneWorktree, panel, worktreeMeta)
	}

	// Worktree detail pane
	p.registerPane(paneWorktreeDetail, newWorktreeDetailPane(paneWorktreeDetail, detailMeta, common, adapterManager.Git(), repoRoot), detailMeta)

	// Todo overlay pane (persistent — visibility-toggled via Ctrl+T).
	todoMeta := models.PaneMeta{
		ID:       paneTodoOverlay,
		Name:     "Todos",
		Type:     paneTypeTodoOverlay,
		CWD:      cwd,
		RepoID:   cwd,
		Status:   models.PaneStatusReady,
		Closable: false,
	}
	p.registerPane(paneTodoOverlay, todo.New(common), todoMeta)

	// Fixed three-pane layout:
	//   Top    : DAG (30%)
	//   Bottom : Worktree (20%) | Detail+Shell (80%)
	//   Detail (62%) over Shell (38%)
	p.bodyTree = layout.Split(
		layout.SplitVertical, 30,
		layout.Leaf(paneDAG),
		layout.Split(
			layout.SplitHorizontal, 20,
			layout.Leaf(paneWorktree),
			layout.Split(
				layout.SplitVertical, 62,
				layout.Leaf(paneWorktreeDetail),
				layout.Leaf(paneShell),
			),
		),
	)

	p.focused = paneDAG
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

// keyBindingProvider returns the panel as a KeyBindingProvider if it implements the interface.
func (p *page) keyBindingProvider(id models.PaneID) models.KeyBindingProvider {
	panel := p.panes[id]
	if panel == nil {
		return nil
	}
	kp, _ := panel.(models.KeyBindingProvider)
	return kp
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
	if meta, ok := p.paneMeta[paneWorktree]; ok && meta.RepoID != "" {
		return meta.RepoID
	}
	return p.currentCWD()
}

func (p *page) currentWorktreeID() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.WorktreeID != "" {
		return meta.WorktreeID
	}
	if meta, ok := p.paneMeta[paneWorktree]; ok && meta.WorktreeID != "" {
		return meta.WorktreeID
	}
	return p.currentCWD()
}

func (p *page) currentBranchSnapshot() string {
	if meta, ok := p.paneMeta[p.focused]; ok && meta.BranchSnapshot != "" {
		return meta.BranchSnapshot
	}
	if meta, ok := p.paneMeta[paneWorktree]; ok {
		return meta.BranchSnapshot
	}
	return ""
}

func (p *page) gitRepoPath() string {
	if meta, ok := p.paneMeta[paneWorktree]; ok && meta.RepoID != "" {
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

	p.setFocus(id)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) findEditorPaneByPath(filePath string) models.PaneID {
	currentWorktreeID := p.currentWorktreeID()
	for id, meta := range p.paneMeta {
		if meta.Type != models.PaneTypeEditor || meta.WorktreeID != currentWorktreeID {
			continue
		}
		panel := p.pane(id)
		if editorPane, ok := panel.(editorMetaProvider); ok && editorPane.FilePath() == filePath {
			return id
		}
	}
	return ""
}

func (p *page) lastEditorPane() models.PaneID {
	for id, meta := range p.paneMeta {
		if meta.Type == models.PaneTypeEditor {
			return id
		}
	}
	return ""
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

// initOverlayRegistry sets up the declarative overlay pane registry.
// Adding a new overlay pane only requires inserting its ID here.
func (p *page) initOverlayRegistry() {
	p.overlayPaneSet = map[models.PaneID]struct{}{
		paneHelpOverlay:           {},
		paneTodoOverlay:           {},
		paneWorktreeCreate:        {},
		paneGitCommit:             {},
		paneTaskEdit:              {},
		panePlanEdit:              {},
		paneAgentSelect:           {},
		paneProviderSelect:        {},
		paneAgentInstallHint:      {},
		paneAgentRegister:         {},
		paneAgentStore:            {},
		paneGitFileTree:           {},
		paneGitDiff:               {},
		paneWorktreeHistory:       {},
		paneWorktreeDeleteConfirm: {},
		paneTaskDeleteConfirm:     {},
		paneDAGMiniOverlay:        {},
		paneADRDetail:             {},
		paneCityPicker:            {},
		paneTaskArchive:           {},
		paneTaskArchiveConfirm:    {},
	}
	// Priority order: highest first (help > todo > editor > everything else).
	p.overlayPaneOrder = []models.PaneID{
		paneHelpOverlay,
		paneTodoOverlay,
		paneWorktreeCreate,
		paneGitCommit,
		paneTaskEdit,
		panePlanEdit,
		paneAgentSelect,
		paneProviderSelect,
		paneAgentInstallHint,
		paneAgentRegister,
		paneAgentStore,
		paneGitFileTree,
		paneGitDiff,
		paneWorktreeHistory,
		paneWorktreeDeleteConfirm,
		paneTaskDeleteConfirm,
		paneDAGMiniOverlay,
		paneADRDetail,
		paneCityPicker,
		paneTaskArchive,
		paneTaskArchiveConfirm,
	}
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
	for _, id := range p.overlayPaneOrder {
		if id == paneWorktreeHistory {
			if editorID := p.activeEditorOverlayPane(); editorID != "" {
				return editorID
			}
		}
		if id == paneTodoOverlay {
			if tp, ok := p.pane(id).(*todo.Model); ok && tp.Visible() {
				return id
			}
			continue
		}
		if _, ok := p.paneMeta[id]; ok {
			return id
		}
	}
	return p.activeEditorOverlayPane()
}

func (p *page) activeEditorOverlayPane() models.PaneID {
	for id, meta := range p.paneMeta {
		if meta.Type == models.PaneTypeEditor {
			return id
		}
	}
	return ""
}

func (p *page) isOverlayPane(id models.PaneID) bool {
	if _, ok := p.overlayPaneSet[id]; ok {
		return true
	}
	if meta, ok := p.paneMeta[id]; ok && meta.Type == models.PaneTypeEditor {
		return true
	}
	return false
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

		meta, ok := p.paneMeta[id]
		if !ok {
			continue
		}
		title := meta.Name

		// Extract description from pane meta if available
		description := ""
		if meta.Status != "" {
			description = meta.Status.String()
		}

		// Broaden dynamic detection: Check if any agent session is active for this worktree
		isRunning := false
		if meta.ID == paneWorktreeDetail && meta.WorktreeID != "" {
			if p.hasActiveSession(meta.WorktreeID) {
				isRunning = true
				if description != "" {
					description += " • "
				}
				description += "running"
			}
		}

		// If status itself is running/starting, mark it
		if strings.Contains(strings.ToLower(meta.Status.String()), "running") ||
			strings.Contains(strings.ToLower(meta.Status.String()), "starting") {
			// Shell pane only shows glitch effect when focused
			if meta.Type != models.PaneTypeShell || active {
				isRunning = true
			}
		}

		if meta.CWD != "" {
			rel, err := filepath.Rel(p.repoRoot, meta.CWD)
			if err == nil && rel != "" {
				if rel == "." {
					rel = "root"
				}
				if description != "" {
					description += " • "
				}
				description += rel
			}
		}

		panelView := layout.RenderHubPanel(p.common.Theme, title, description, content.Content, max(frame.W-4, 8), frame.H, active, isRunning)
		base = layout.OverlayOnBase(base, panelView, frame.X, frame.Y)
	}

	if overlayID := p.activeOverlayPane(); overlayID != "" {
		base = dimCanvas(base)
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
		description := ""
		isRunning := false
		if meta, ok := p.paneMeta[id]; ok {
			if meta.Status != "" {
				description = meta.Status.String()
			}
			if meta.ID == paneWorktreeDetail {
				if meta.WorktreeID != "" && p.hasActiveSession(meta.WorktreeID) {
					isRunning = true
				}
			}
			// If status itself is running/starting, mark it
			if strings.Contains(strings.ToLower(meta.Status.String()), "running") ||
				strings.Contains(strings.ToLower(meta.Status.String()), "starting") {
				// Shell pane only shows glitch effect when focused
				if meta.Type != models.PaneTypeShell || active {
					isRunning = true
				}
			}
		}

		sub := canvas.SubCanvas(frame.X, frame.Y, frame.W, frame.H)
		if renderer, ok := panel.(render.Renderer); ok {
			renderer.Render(sub, frame.W, frame.H)
		} else {
			panel.SetSize(max(frame.W-4, 8), max(frame.H-2, 3))
			content := panel.View()
			// Use the Hub-style renderer for non-native renderers.
			hubView := layout.RenderHubPanel(p.common.Theme, title, description, content.Content, max(frame.W-4, 8), max(frame.H-2, 3), active, isRunning)
			sub.SetString(0, 0, hubView, nil)
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

	if tp, ok := p.pane(paneTodoOverlay).(*todo.Model); ok && tp.Visible() {
		var width int
		switch {
		case bounds.W >= 140:
			width = 84
		case bounds.W >= 100:
			width = 72
		case bounds.W >= 80:
			width = 64
		default:
			width = bounds.W - 4
		}
		if width > bounds.W-2 {
			width = bounds.W - 2
		}
		if width < 30 {
			width = 30
		}

		height := int(float64(bounds.H) * 0.78)
		if height < 14 {
			height = 14
		}
		if height > bounds.H-3 {
			height = bounds.H - 3
		}
		if height < 8 {
			height = 8
		}
		return width, height
	}

	if _, ok := p.paneMeta[paneADRDetail]; ok {
		width := bounds.W - 8
		if width > 100 {
			width = 100
		}
		if width < 40 {
			width = 40
		}
		height := bounds.H - 4
		if height > 32 {
			height = 32
		}
		if height < 10 {
			height = 10
		}
		return width, height
	}

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

func (p *page) largeOverlayContentSize() (int, int) {
	bounds := p.bodyBoundsSize()
	width := bounds.W - 6
	if width > bounds.W {
		width = bounds.W
	}
	if width < 40 {
		width = 40
	}

	height := bounds.H - 4
	if height > bounds.H {
		height = bounds.H
	}
	if height < 8 {
		height = 8
	}

	return width, height
}

func (p *page) gitFileTreeOverlayContentSize() (int, int) {
	bounds := p.bodyBoundsSize()
	width := bounds.W - 4
	if width > bounds.W {
		width = bounds.W
	}
	if width < 60 {
		width = 60
	}

	height := bounds.H - 2
	if height > bounds.H {
		height = bounds.H
	}
	if height < 20 {
		height = 20
	}

	return width, height
}

func (p *page) isLargeOverlayPane(id models.PaneID) bool {
	if id == paneGitDiff {
		return true
	}
	if meta, ok := p.paneMeta[id]; ok && meta.Type == models.PaneTypeEditor {
		return true
	}
	return false
}

// dimCanvas strips existing ANSI colors and re-renders every non-empty line
// with a 'Darkroom Dimming' effect. It uses a very subtle foreground color
// to push the base canvas into the background, ensuring readability of context
// while highlighting the overlay.
func dimCanvas(s string) string {
	lines := strings.Split(s, "\n")
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		plain := ansi.Strip(line)
		lines[i] = dimStyle.Render(plain)
	}
	return strings.Join(lines, "\n")
}

func (p *page) renderOverlayPane(base string, id models.PaneID) string {
	panel := p.pane(id)
	if panel == nil {
		return base
	}

	var overlayW, overlayH int
	if id == paneGitFileTree {
		overlayW, overlayH = p.gitFileTreeOverlayContentSize()
	} else if p.isLargeOverlayPane(id) {
		overlayW, overlayH = p.largeOverlayContentSize()
	} else {
		overlayW, overlayH = p.overlayContentSize()
	}
	panel.SetSize(overlayW, overlayH)

	meta, ok := p.paneMeta[id]
	title := ""
	description := ""
	isRunning := false
	if ok {
		title = meta.Name
		description = meta.Status.String()
		if strings.Contains(strings.ToLower(meta.Status.String()), "running") ||
			strings.Contains(strings.ToLower(meta.Status.String()), "starting") {
			isRunning = true
		}
	}

	overlayView := layout.RenderHubPanel(p.common.Theme, title, description, panel.View().Content, overlayW, overlayH, true, isRunning)
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

	var overlayW, overlayH int
	if id == paneGitFileTree {
		overlayW, overlayH = p.gitFileTreeOverlayContentSize()
	} else if p.isLargeOverlayPane(id) {
		overlayW, overlayH = p.largeOverlayContentSize()
	} else {
		overlayW, overlayH = p.overlayContentSize()
	}
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

		meta, ok := p.paneMeta[id]
		title := ""
		description := ""
		isRunning := false
		if ok {
			title = meta.Name
			description = meta.Status.String()
			if strings.Contains(strings.ToLower(meta.Status.String()), "running") ||
				strings.Contains(strings.ToLower(meta.Status.String()), "starting") {
				isRunning = true
			}
		}

		hubView := layout.RenderHubPanel(p.common.Theme, title, description, content.Content, overlayW, overlayH, true, isRunning)
		sub.SetString(0, 0, hubView, nil)
	}
}

func (p *page) openGitFileTreePane(msg gitfiletree.OpenGitFileTreeMsg) tea.Cmd {
	p.closePane(paneGitFileTree)

	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = p.gitRepoPath()
	}

	meta := models.PaneMeta{
		ID:             paneGitFileTree,
		Name:           "Git",
		Type:           paneTypeGitFileTree,
		CWD:            repoPath,
		RepoID:         p.currentRepoID(),
		WorktreeID:     p.currentWorktreeID(),
		BranchSnapshot: p.currentBranchSnapshot(),
		Status:         models.PaneStatusReady,
		Closable:       true,
	}
	panel := gitfiletree.NewOverlay(meta.ID, meta, *p.common, p.adapterManager.Git(), repoPath)
	p.registerPane(meta.ID, panel, meta)
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openDiffPane(msg gitplugin.OpenDiffMsg) tea.Cmd {
	opener := p.focused
	p.closePane(paneGitCommit)
	p.closePane(paneGitDiff)
	// Close any higher-priority overlays so diff receives keyboard focus.
	p.closePane(paneWorktreeCreate)
	p.closePane(paneTaskEdit)
	p.closePane(panePlanEdit)
	p.closePane(paneAgentSelect)
	p.closePane(paneProviderSelect)
	p.closePane(paneAgentInstallHint)
	p.closePane(paneAgentRegister)
	p.closePane(paneAgentStore)

	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = p.gitRepoPath()
	}

	meta := models.PaneMeta{
		ID:             paneGitDiff,
		Name:           "Diff",
		Type:           models.PaneTypeDiffView,
		CWD:            repoPath,
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
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openCommitPane(msg gitplugin.OpenCommitMsg) tea.Cmd {
	p.closePane(paneGitDiff)
	p.closePane(paneGitCommit)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
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

func (p *page) openProviderSelectPane(worktreeID string, provider agents.Provider, resume bool) tea.Cmd {
	p.closePane(paneProviderSelect)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:         paneProviderSelect,
		Name:       "Select Provider",
		Type:       paneTypeProviderSelect,
		CWD:        worktreeID,
		RepoID:     p.currentRepoID(),
		WorktreeID: worktreeID,
		Status:     models.PaneStatusReady,
		Closable:   true,
	}
	panel := newProviderSelectPane(meta.ID, meta, *p.common, worktreeID, provider, resume)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openAgentStorePane() tea.Cmd {
	p.closePane(paneAgentStore)
	p.closePane(paneAgentInstallHint)
	p.closePane(paneAgentRegister)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneAgentStore,
		Name:     "Agent Store",
		Type:     paneTypeAgentStore,
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newAgentStorePane(meta.ID, meta, *p.common)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openAgentInstallHintPane(agentID string) tea.Cmd {
	p.closePane(paneAgentInstallHint)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneAgentInstallHint,
		Name:     "Install Hint",
		Type:     paneTypeAgentInstallHint,
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newAgentInstallHintPane(meta.ID, meta, *p.common, agentID)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openAgentRegisterPane() tea.Cmd {
	p.closePane(paneAgentRegister)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneAgentRegister,
		Name:     "Register Agent",
		Type:     paneTypeAgentRegister,
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newAgentRegisterPane(meta.ID, meta, *p.common)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openWorktreeHistoryPane() tea.Cmd {
	p.closePane(paneWorktreeHistory)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneWorktreeHistory,
		Name:     "Worktree History",
		Type:     paneTypeWorktreeHistory,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newWorktreeHistoryPane(meta.ID, meta, *p.common)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openHelpOverlayPane() tea.Cmd {
	p.closePane(paneHelpOverlay)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	meta := models.PaneMeta{
		ID:       paneHelpOverlay,
		Name:     "Help",
		Type:     paneTypeHelpOverlay,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newHelpOverlayPane(meta.ID, *p.common, p)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openWorktreeDeleteConfirmPane(msg gitplugin.OpenWorktreeDeleteConfirmMsg) tea.Cmd {
	p.closePane(paneWorktreeDeleteConfirm)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneWorktreeDeleteConfirm,
		Name:     "Delete Worktree",
		Type:     paneTypeWorktreeDeleteConfirm,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newDeleteConfirmPane(meta.ID, msg.Worktree, msg.Force)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openTaskDeleteConfirmPane(taskID, taskTitle, repoID string, clearAll bool) tea.Cmd {
	p.closePane(paneTaskDeleteConfirm)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	meta := models.PaneMeta{
		ID:       paneTaskDeleteConfirm,
		Name:     "Delete Task",
		Type:     paneTypeTaskDeleteConfirm,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newTaskDeleteConfirmPane(meta.ID, taskID, taskTitle, repoID, clearAll)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openTaskArchivePane(repoID string) tea.Cmd {
	p.closePane(paneTaskArchive)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	meta := models.PaneMeta{
		ID:       paneTaskArchive,
		Name:     "Archive",
		Type:     paneTypeTaskArchive,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newTaskArchivePane(*p.common, repoID)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openTaskArchiveConfirmPane(taskID, taskTitle, state string) tea.Cmd {
	p.closePane(paneTaskArchiveConfirm)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	meta := models.PaneMeta{
		ID:       paneTaskArchiveConfirm,
		Name:     "Archive Confirm",
		Type:     paneTypeTaskArchiveConfirm,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newTaskArchiveConfirmPane(meta.ID, taskID, taskTitle, state)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openDAGMiniOverlayPane(phaseID, phaseTitle string) tea.Cmd {
	p.closePane(paneDAGMiniOverlay)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	var steps []models.TaskContextRecord
	if p.common.Store != nil {
		allTasks, _ := p.common.Store.ListTaskContexts(p.currentRepoID())
		for _, t := range allTasks {
			if t.ParentTaskID != nil && *t.ParentTaskID == phaseID {
				steps = append(steps, t)
			}
		}
		sort.Slice(steps, func(i, j int) bool {
			return steps[i].Title < steps[j].Title
		})
	}

	meta := models.PaneMeta{
		ID:       paneDAGMiniOverlay,
		Name:     "Phase Steps",
		Type:     paneTypeDAGMiniOverlay,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newDAGMiniOverlayPane(meta.ID, phaseID, phaseTitle, steps)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) refreshDAGPane() tea.Cmd {
	if tc, ok := p.pane(paneDAG).(*tabContainer); ok {
		tc.refreshDAG()
	}
	return nil
}

func (p *page) openADRDetailOverlay(filePath string) tea.Cmd {
	p.closePane(paneADRDetail)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneDAG
	}

	meta := models.PaneMeta{
		ID:       paneADRDetail,
		Name:     "ADR Detail",
		Type:     paneTypeADRDetail,
		CWD:      p.gitRepoPath(),
		RepoID:   p.currentRepoID(),
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newADRDetailOverlay(meta.ID, meta, *p.common, filePath)
	p.registerPane(meta.ID, panel, meta)
	if p.returnFocus == nil {
		p.returnFocus = make(map[models.PaneID]models.PaneID)
	}
	p.returnFocus[meta.ID] = baseFocus
	p.setFocus(meta.ID)
	p.updateSizes(p.bodyBoundsSize())
	return panel.Init()
}

func (p *page) openCityPickerOverlay() tea.Cmd {
	p.closePane(paneCityPicker)

	baseFocus := p.focused
	if baseFocus == "" {
		baseFocus = paneWorktree
	}

	meta := models.PaneMeta{
		ID:       paneCityPicker,
		Name:     "City Picker",
		Type:     paneTypeCityPicker,
		Status:   models.PaneStatusReady,
		Closable: true,
	}
	panel := newCityPickerOverlay(p.common.Cfg)
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

	if meta, ok := p.paneMeta[paneShell]; ok {
		meta.WorktreeID = worktreeID
		meta.CWD = worktreeID
		p.paneMeta[paneShell] = meta
		if sh, ok := p.pane(paneShell).(*shell.Model); ok {
			sh.SetCWD(worktreeID)
		}
		p.setFocus(paneShell)
		return nil
	}
	return nil
}

func (p *page) focusAgentShell(worktreeID string) tea.Cmd {
	if meta, ok := p.paneMeta[paneShell]; ok && meta.WorktreeID == worktreeID {
		p.setFocus(paneShell)
		return nil
	}
	return nil
}

func (p *page) openAgentShell(worktreeID string, provider agents.Provider, sessionID string) tea.Cmd {
	// In the new single-page design, agents run in external terminals.
	// This method is kept for compatibility but should not create new panes.
	return nil
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

func (p *page) hasActiveSession(worktreeID string) bool {
	if p.common == nil || p.common.Store == nil || worktreeID == "" {
		return false
	}

	// Throttled cache: 1 second TTL
	if time.Since(p.activeSessionCacheTs) < 1*time.Second {
		return p.activeSessionCache[worktreeID]
	}

	// Update cache
	sessions, err := p.common.Store.ListAgentSessions(worktreeID)
	if err != nil {
		return false
	}

	isActive := false
	for _, s := range sessions {
		if s.State == "running" || s.State == "starting" {
			isActive = true
			break
		}
	}

	p.activeSessionCache[worktreeID] = isActive
	p.activeSessionCacheTs = time.Now()
	return isActive
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
	// In the new single-page design, worktree-specific page snapshots are no longer used.
	// Only the overview page snapshot may be restored.
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
	// In the new single-page design, there is no separate overview page.
	// Just ensure the active page exists and focus the worktree pane.
	if m.activePage == nil {
		cwd, _ := os.Getwd()
		repoRoot, _ := gitRepoRoot(cwd)
		m.activePage = newOverviewPage(m.common, m.pluginRegistry, m.adapterManager, m.common.Cfg, m.common.Store, cwd, repoRoot)
		m.pages[""] = m.activePage
	}
	m.state = StateOverviewPage
	m.currentWorktreePage = ""
	m.activePage.setFocus(paneWorktree)
	if m.mode == ModeShell {
		m.mode = ModeNormal
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
}

func (m *model) switchToWorktreePage(worktreeID, preferredPane string) tea.Cmd {
	if worktreeID == "" {
		m.switchToOverviewPage()
		return nil
	}
	// In the new single-page design, we don't create separate pages per worktree.
	// Instead, we update the shell CWD and focus the appropriate pane.
	if m.activePage == nil {
		m.switchToOverviewPage()
	}
	m.state = StateWorktreePage
	m.currentWorktreePage = worktreeID

	// Update shell CWD to the selected worktree
	if sh, ok := m.activePage.pane(paneShell).(*shell.Model); ok {
		sh.SetCWD(worktreeID)
	}

	// Update Trellis bridge worktree ID so BuildAgentContext writes
	// AGENTS.md / CLAUDE.md to the correct worktree directory.
	if m.trellisBridge != nil {
		m.trellisBridge.SetWorktreeID(worktreeID)
	}

	// Notify worktree detail pane of the selection via message routing
	// so Init commands (git watcher, file tree ticker) are returned to the runtime.
	var cmd tea.Cmd
	if _, ok := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane); ok {
		cmd = m.activePage.routeToPane(paneWorktreeDetail, worktreeSelectedMsg{WorktreeID: worktreeID})
	}

	if preferredPane != "" {
		m.activePage.setFocus(models.PaneID(preferredPane))
	} else {
		m.activePage.setFocus(paneWorktreeDetail)
	}
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return cmd
}
