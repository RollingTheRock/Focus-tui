package editor

import (
	"fmt"

	"focus/internal/models"
	"focus/internal/plugins"
)

var _ plugins.Plugin = (*Plugin)(nil)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Name() string { return "editor" }

func (p *Plugin) Version() string { return "1.0.0" }

func (p *Plugin) PaneTypes() []models.PaneType { return []models.PaneType{models.PaneTypeEditor} }

func (p *Plugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	switch paneType {
	case models.PaneTypeEditor:
		return NewEditorPane(id, meta, common, ""), nil
	default:
		return nil, fmt.Errorf("editor plugin: unsupported pane type %q", paneType)
	}
}

func (p *Plugin) Init() error { return nil }

func (p *Plugin) Destroy() error { return nil }
