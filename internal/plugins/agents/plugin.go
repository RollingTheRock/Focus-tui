package agents

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

func (p *Plugin) Name() string    { return "agents" }
func (p *Plugin) Version() string { return "1.0.0" }

func (p *Plugin) PaneTypes() []models.PaneType {
	return []models.PaneType{models.PaneTypeAgentSession}
}

func (p *Plugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	switch paneType {
	case models.PaneTypeAgentSession:
		return NewSessionPane(id, meta, common), nil
	default:
		return nil, fmt.Errorf("agents plugin: unsupported pane type %q", paneType)
	}
}

func (p *Plugin) Init() error    { return nil }
func (p *Plugin) Destroy() error { return nil }
