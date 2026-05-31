package gitfiletree

import (
	"fmt"

	"focus/internal/models"
	"focus/internal/plugins"
)

var _ plugins.Plugin = (*Plugin)(nil)

// Plugin provides the GitFileTree overlay.
type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string    { return "gitfiletree" }
func (p *Plugin) Version() string { return "1.0.0" }

func (p *Plugin) PaneTypes() []models.PaneType {
	return []models.PaneType{models.PaneTypeGitStatus}
}

func (p *Plugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	return nil, fmt.Errorf("gitfiletree plugin: use NewOverlay directly, not CreatePane")
}

func (p *Plugin) Init() error    { return nil }
func (p *Plugin) Destroy() error { return nil }
