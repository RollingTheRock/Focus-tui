package trellis

import (

	"focus/internal/models"
)

// MCPBridge provides hooks for Focus MCP tool handlers to sync with Trellis.
type MCPBridge struct {
	bridge *Bridge
}

// NewMCPBridge creates an MCPBridge wrapping the given Bridge.
func NewMCPBridge(bridge *Bridge) *MCPBridge {
	return &MCPBridge{bridge: bridge}
}

// OnTaskCreate is called after a Focus task is created via MCP.
func (b *MCPBridge) OnTaskCreate(task models.TaskContextRecord, plan *models.TaskPlanRecord) error {
	if b.bridge == nil {
		return nil
	}
	_, err := b.bridge.SyncTaskCreate(task, plan)
	return err
}

// OnTaskStatusChange is called after a Focus task state is updated via MCP.
func (b *MCPBridge) OnTaskStatusChange(taskID string, prevState, nextState string, sessionID string) error {
	if b.bridge == nil {
		return nil
	}
	switch nextState {
	case "active":
		return b.bridge.SyncTaskStart(taskID)
	case "done":
		return b.bridge.SyncTaskFinish(taskID)
	case "archived":
		return b.bridge.SyncTaskArchive(taskID)
	}
	return nil
}

// OnTaskOutput is called after a task output is recorded via MCP.
func (b *MCPBridge) OnTaskOutput(taskID, output, actor string) error {
	if b.bridge == nil {
		return nil
	}
	// Append to journal via add_session.py.
	return b.bridge.client.AddSession(output, []string{})
}

// OnKnowledgeFact is called after a knowledge fact is added via MCP.
func (b *MCPBridge) OnKnowledgeFact(domain, subject, predicate, object string) error {
	if b.bridge == nil {
		return nil
	}
	return b.bridge.WriteSpecFact(domain, subject, predicate, object)
}

// OnPlanCreate is called after a Focus plan is created via MCP.
func (b *MCPBridge) OnPlanCreate(plan models.TaskPlanRecord, steps []models.PlanStepRecord) error {
	if b.bridge == nil {
		return nil
	}
	// PRD is written when the associated task is synced.
	return nil
}

// OnPlanAddStep is called after a plan step is added via MCP.
func (b *MCPBridge) OnPlanAddStep(planID string, step models.PlanStepRecord) error {
	if b.bridge == nil {
		return nil
	}
	return b.bridge.UpdateWorkflowState(planID, step)
}

// OnContextGetForTask enriches the context response with Trellis data.
func (b *MCPBridge) OnContextGetForTask(taskID string, result map[string]any) error {
	if b.bridge == nil {
		return nil
	}
	extCtx, err := b.bridge.GetTaskContextExtended(taskID)
	if err != nil {
		return err
	}
	if extCtx == nil {
		return nil
	}
	if extCtx.Task != nil {
		result["trellis_task"] = extCtx.Task
	}
	result["trellis_prd"] = extCtx.PRD
	result["trellis_specs"] = extCtx.Specs
	result["trellis_handoff"] = extCtx.Handoff
	result["trellis_journal"] = extCtx.Journal
	result["trellis_workflow_state"] = extCtx.WorkflowState
	return nil
}
