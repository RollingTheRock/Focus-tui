package git

import (
	"fmt"

	"focus/internal/adapters"
	"focus/internal/models"
	"focus/internal/plugins"
)

var _ plugins.Plugin = (*Plugin)(nil)

type Plugin struct {
	adapter adapters.GitAdapter
}

func New(adapter adapters.GitAdapter) *Plugin {
	return &Plugin{adapter: adapter}
}

func (p *Plugin) Name() string { return "git" }

func (p *Plugin) Version() string { return "1.0.0" }

func (p *Plugin) PaneTypes() []models.PaneType {
	return []models.PaneType{models.PaneTypeWorktree, models.PaneTypeGitStatus, models.PaneTypeDiffView}
}

func (p *Plugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	switch paneType {
	case models.PaneTypeWorktree:
		return NewWorktreePane(id, meta, common, p.adapter), nil
	case models.PaneTypeGitStatus:
		return NewStatusPane(id, meta, common, p.adapter), nil
	case models.PaneTypeDiffView:
		return NewDiffPane(id, meta, common, p.adapter, "", false), nil
	default:
		return nil, fmt.Errorf("git plugin: unsupported pane type %q", paneType)
	}
}

func (p *Plugin) Init() error {
	if p.adapter == nil {
		return nil
	}
	return p.adapter.Init()
}

func (p *Plugin) Destroy() error {
	if p.adapter == nil {
		return nil
	}
	return p.adapter.Destroy()
}
