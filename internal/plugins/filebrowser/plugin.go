package filebrowser

import (
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/plugins"
)

var _ plugins.Plugin = (*Plugin)(nil)

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string { return "filebrowser" }

func (p *Plugin) Version() string { return "1.0.0" }

func (p *Plugin) PaneTypes() []models.PaneType {
	return []models.PaneType{models.PaneTypeFileTree}
}

func (p *Plugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	switch paneType {
	case models.PaneTypeFileTree:
		return NewTreePane(id, meta, common), nil
	default:
		return nil, fmt.Errorf("filebrowser plugin: unsupported pane type %q", paneType)
	}
}

func (p *Plugin) Init() error {
	return nil
}

func (p *Plugin) Destroy() error {
	return nil
}
