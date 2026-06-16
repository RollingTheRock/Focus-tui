package plugins

import "github.com/RollingTheRock/Focus-tui/internal/models"

// Plugin defines the lifecycle and pane factory contract for a pane plugin.
type Plugin interface {
	Name() string
	Version() string
	PaneTypes() []models.PaneType
	CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error)
	Init() error
	Destroy() error
}
