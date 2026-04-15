package app

import (
	"focus/internal/models"
	"focus/internal/ui/layout"
)

// workspacePageState holds the page-local layout identity that will eventually
// replace the remaining global workspace fields on model.
type workspacePageState struct {
	WorktreeID  string
	BodyTree    *layout.TreeNode
	Frames      map[models.PaneID]models.PaneFrame
	Focused     models.PaneID
	ZoomedPane  models.PaneID
	PreZoomTree *layout.TreeNode
	ReturnFocus map[models.PaneID]models.PaneID
}

func (m model) currentPageLabel() string {
	if m.state == StateWorktreePage {
		if m.currentWorktreePage != "" {
			return "WORKTREE"
		}
		return "WORKTREE"
	}
	return "OVERVIEW"
}

func (m *model) switchToOverviewPage() {
	m.state = StateOverviewPage
	m.currentWorktreePage = ""
	if m.paneMeta[paneWorktree].ID != "" {
		m.setFocus(paneWorktree)
	}
	if m.mode == ModeShell {
		m.mode = ModeNormal
	}
	if m.zoomedPane != "" {
		m.restoreZoom()
	}
	m.invalidateView()
}

func (m *model) switchToWorktreePage(worktreeID, preferredPane string) {
	m.state = StateWorktreePage
	m.currentWorktreePage = worktreeID
	if preferredPane != "" {
		m.setFocus(models.PaneID(preferredPane))
	}
	m.invalidateView()
}

func (m model) pageStateSnapshot() workspacePageState {
	return workspacePageState{
		WorktreeID:  m.currentWorktreePage,
		BodyTree:    m.bodyTree,
		Frames:      m.frames,
		Focused:     m.focused,
		ZoomedPane:  m.zoomedPane,
		PreZoomTree: m.preZoomTree,
		ReturnFocus: m.returnFocus,
	}
}
