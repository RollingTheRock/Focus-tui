package plugins

import (
	"errors"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

var (
	ErrNilPlugin         = errors.New("plugins: nil plugin")
	ErrDuplicatePlugin   = errors.New("plugins: duplicate plugin")
	ErrDuplicatePaneType = errors.New("plugins: duplicate pane type")
	ErrUnknownPaneType   = errors.New("plugins: unknown pane type")
)

// Registry stores registered plugins and resolves pane types to their owners.
type Registry struct {
	plugins   map[string]Plugin
	paneTypes map[models.PaneType]Plugin
}

// NewRegistry returns an initialized plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:   make(map[string]Plugin),
		paneTypes: make(map[models.PaneType]Plugin),
	}
}

// Register adds a plugin and reserves all pane types it owns.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return ErrNilPlugin
	}

	r.ensureMaps()

	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicatePlugin, p.Name())
	}

	for _, paneType := range p.PaneTypes() {
		if existing, exists := r.paneTypes[paneType]; exists {
			return fmt.Errorf("%w: %q already owned by %q", ErrDuplicatePaneType, paneType, existing.Name())
		}
	}

	r.plugins[p.Name()] = p
	for _, paneType := range p.PaneTypes() {
		r.paneTypes[paneType] = p
	}

	return nil
}

// CreatePane resolves a plugin for the pane type and delegates pane creation.
func (r *Registry) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	p, ok := r.GetPluginForType(paneType)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownPaneType, paneType)
	}

	return p.CreatePane(paneType, id, meta, common)
}

// GetPluginForType returns the plugin registered for a pane type.
func (r *Registry) GetPluginForType(paneType models.PaneType) (Plugin, bool) {
	if r == nil {
		return nil, false
	}

	p, ok := r.paneTypes[paneType]
	return p, ok
}

func (r *Registry) ensureMaps() {
	if r.plugins == nil {
		r.plugins = make(map[string]Plugin)
	}
	if r.paneTypes == nil {
		r.paneTypes = make(map[models.PaneType]Plugin)
	}
}
