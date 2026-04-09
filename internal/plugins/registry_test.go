package plugins

import (
	"errors"
	"testing"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRegisterRejectsDuplicatePluginName(t *testing.T) {
	r := NewRegistry()
	first := &stubPlugin{name: "core", paneTypes: []models.PaneType{models.PaneTypeTodo}}
	second := &stubPlugin{name: "core", paneTypes: []models.PaneType{models.PaneTypePomodoro}}

	if err := r.Register(first); err != nil {
		t.Fatalf("register first plugin: %v", err)
	}

	err := r.Register(second)
	if !errors.Is(err, ErrDuplicatePlugin) {
		t.Fatalf("expected ErrDuplicatePlugin, got %v", err)
	}
}

func TestRegisterRejectsDuplicatePaneType(t *testing.T) {
	r := NewRegistry()
	first := &stubPlugin{name: "todo", paneTypes: []models.PaneType{models.PaneTypeTodo}}
	second := &stubPlugin{name: "alt-todo", paneTypes: []models.PaneType{models.PaneTypeTodo}}

	if err := r.Register(first); err != nil {
		t.Fatalf("register first plugin: %v", err)
	}

	err := r.Register(second)
	if !errors.Is(err, ErrDuplicatePaneType) {
		t.Fatalf("expected ErrDuplicatePaneType, got %v", err)
	}
}

func TestGetPluginForType(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "todo", paneTypes: []models.PaneType{models.PaneTypeTodo}}

	if err := r.Register(p); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	got, ok := r.GetPluginForType(models.PaneTypeTodo)
	if !ok {
		t.Fatal("expected plugin lookup to succeed")
	}
	if got.Name() != p.Name() {
		t.Fatalf("expected plugin %q, got %q", p.Name(), got.Name())
	}

	if _, ok := r.GetPluginForType(models.PaneTypeShell); ok {
		t.Fatal("expected missing plugin lookup to fail")
	}
}

func TestCreatePaneDelegatesToPlugin(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "todo", paneTypes: []models.PaneType{models.PaneTypeTodo}}

	if err := r.Register(p); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	meta := models.PaneMeta{ID: "pane-1", Type: models.PaneTypeTodo, Name: "Todo"}
	panel, err := r.CreatePane(models.PaneTypeTodo, "pane-1", meta, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}
	if panel == nil {
		t.Fatal("expected panel to be created")
	}

	if p.lastPaneType != models.PaneTypeTodo {
		t.Fatalf("expected pane type %q, got %q", models.PaneTypeTodo, p.lastPaneType)
	}
	if p.lastID != "pane-1" {
		t.Fatalf("expected pane id %q, got %q", models.PaneID("pane-1"), p.lastID)
	}
	if p.lastMeta != meta {
		t.Fatalf("expected pane meta %+v, got %+v", meta, p.lastMeta)
	}
}

func TestCreatePaneReturnsErrorForUnknownType(t *testing.T) {
	r := NewRegistry()

	_, err := r.CreatePane(models.PaneTypeShell, "pane-1", models.PaneMeta{}, models.CommonModel{})
	if !errors.Is(err, ErrUnknownPaneType) {
		t.Fatalf("expected ErrUnknownPaneType, got %v", err)
	}
}

type stubPlugin struct {
	name         string
	paneTypes    []models.PaneType
	lastPaneType models.PaneType
	lastID       models.PaneID
	lastMeta     models.PaneMeta
	lastCommon   models.CommonModel
	panel        models.Panel
	createErr    error
}

func (p *stubPlugin) Name() string {
	return p.name
}

func (p *stubPlugin) Version() string {
	return "v0.0.0"
}

func (p *stubPlugin) PaneTypes() []models.PaneType {
	return p.paneTypes
}

func (p *stubPlugin) CreatePane(paneType models.PaneType, id models.PaneID, meta models.PaneMeta, common models.CommonModel) (models.Panel, error) {
	p.lastPaneType = paneType
	p.lastID = id
	p.lastMeta = meta
	p.lastCommon = common

	if p.panel != nil || p.createErr != nil {
		return p.panel, p.createErr
	}

	return stubPanel{}, nil
}

func (p *stubPlugin) Init() error {
	return nil
}

func (p *stubPlugin) Destroy() error {
	return nil
}

type stubPanel struct{}

func (stubPanel) Init() tea.Cmd {
	return nil
}

func (stubPanel) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	return stubPanel{}, nil
}

func (stubPanel) View() string {
	return ""
}

func (stubPanel) SetSize(width, height int) {}
