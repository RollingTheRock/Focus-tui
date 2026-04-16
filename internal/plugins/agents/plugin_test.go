package agents

import (
	"testing"

	"focus/internal/models"
)

func TestPluginName(t *testing.T) {
	p := New()
	if p.Name() != "agents" {
		t.Errorf("Name() = %q, want agents", p.Name())
	}
}

func TestPluginVersion(t *testing.T) {
	p := New()
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want 1.0.0", p.Version())
	}
}

func TestPluginPaneTypes(t *testing.T) {
	p := New()
	types := p.PaneTypes()
	if len(types) != 1 || types[0] != models.PaneTypeAgentSession {
		t.Errorf("PaneTypes() = %v, want [%s]", types, models.PaneTypeAgentSession)
	}
}

func TestPluginCreatePaneAgentSession(t *testing.T) {
	p := New()
	common := models.CommonModel{Width: 80, Height: 24}
	meta := models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}
	panel, err := p.CreatePane(models.PaneTypeAgentSession, "agent-session", meta, common)
	if err != nil {
		t.Fatalf("CreatePane error: %v", err)
	}
	if panel == nil {
		t.Fatal("expected non-nil panel")
	}
	sp, ok := panel.(*SessionPane)
	if !ok {
		t.Fatalf("expected *SessionPane, got %T", panel)
	}
	if sp.id != "agent-session" {
		t.Errorf("pane id = %q, want agent-session", sp.id)
	}
}

func TestPluginCreatePaneUnsupported(t *testing.T) {
	p := New()
	common := models.CommonModel{Width: 80, Height: 24}
	meta := models.PaneMeta{ID: "x", Name: "X", Type: models.PaneTypeShell}
	panel, err := p.CreatePane(models.PaneTypeShell, "x", meta, common)
	if err == nil {
		t.Fatal("expected error for unsupported pane type")
	}
	if panel != nil {
		t.Error("expected nil panel for unsupported pane type")
	}
}

func TestPluginInitDestroy(t *testing.T) {
	p := New()
	if err := p.Init(); err != nil {
		t.Errorf("Init() error: %v", err)
	}
	if err := p.Destroy(); err != nil {
		t.Errorf("Destroy() error: %v", err)
	}
}
