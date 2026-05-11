package app

import (
	"context"
	"encoding/json"
	"fmt"
	"focus/internal/adapters"
	"focus/internal/adapters/ccswitch"
	"focus/internal/agents"
	"focus/internal/avatar"
	"focus/internal/commands"
	"focus/internal/config"
	gitmodel "focus/internal/git"
	"focus/internal/mcp"
	"focus/internal/models"
	"focus/internal/orchestrator"
	"focus/internal/plugins"
	agentsplugin "focus/internal/plugins/agents"
	editorplugin "focus/internal/plugins/editor"
	filebrowser "focus/internal/plugins/filebrowser"
	gitplugin "focus/internal/plugins/git"
	dbstore "focus/internal/store"
	"focus/internal/styles"
	"focus/internal/ui/footer"
	"focus/internal/ui/header"
	"focus/internal/ui/layout"
	"focus/internal/ui/shell"
	"focus/internal/ui/todo"
	"hash/fnv"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
)

const (
	paneHeader                models.PaneID = "header"
	paneShell                 models.PaneID = "shell-main"
	paneWorktree              models.PaneID = "worktree-main"
	paneDAG                   models.PaneID = "dag-main"
	paneWorktreeDetail        models.PaneID = "worktree-detail-main"
	paneGitDiff               models.PaneID = "git-diff-pane"
	paneGitCommit             models.PaneID = "git-commit-overlay"
	paneWorktreeCreate        models.PaneID = "worktree-create-overlay"
	paneTaskEdit              models.PaneID = "task-edit-overlay"
	panePlanEdit              models.PaneID = "plan-edit-overlay"
	paneAgentSelect           models.PaneID = "agent-select-overlay"
	paneProviderSelect        models.PaneID = "provider-select-overlay"
	paneWorktreeHistory       models.PaneID = "worktree-history-overlay"
	paneWorktreeDeleteConfirm models.PaneID = "worktree-delete-confirm-overlay"
	paneADRDetail             models.PaneID = "adr-detail-overlay"
	paneTodoOverlay           models.PaneID = "todo-overlay"
	paneFooter                models.PaneID = "footer"

	paneTypeGitCommit             models.PaneType = "git-commit"
	paneTypeWorktreeCreate        models.PaneType = "worktree-create"
	paneTypeTaskEdit              models.PaneType = "task-edit"
	paneTypePlanEdit              models.PaneType = "plan-edit"
	paneTypeAgentSelect           models.PaneType = "agent-select"
	paneTypeProviderSelect        models.PaneType = "provider-select"
	paneTypeWorktreeHistory       models.PaneType = "worktree-history"
	paneTypeWorktreeDeleteConfirm models.PaneType = "worktree-delete-confirm"
	paneTypeADRDetail             models.PaneType = "adr-detail"
	paneTypeTodoOverlay           models.PaneType = "todo-overlay"
	paneTypeOverviewSummary       models.PaneType = "overview-summary"
	paneTypeOverviewDAG           models.PaneType = "overview-dag"
	paneTypeOverviewDetail        models.PaneType = "overview-detail"

	splitRatioStep = 5

	simplifiedHelpMaxWidth = 40
	hideFooterBelowHeight  = 10
	tinyWindowMinWidth     = 20
	tinyWindowMinHeight    = 5
)

// avatarRenderedMsg carries the pre-rendered avatar string from chafa.
type avatarRenderedMsg struct {
	art string
}

// orchNotificationMsg wraps an orchestrator.Notification for Bubbletea routing.
type orchNotificationMsg orchestrator.Notification

// viewCache holds the cached View() output.
// Using a pointer so it survives value-receiver copies in Bubbletea.
type viewCache struct {
	output string
	gen    uint64
}

// model is the top-level Bubbletea model.
type model struct {
	common         *models.CommonModel
	state          AppState
	mode           AppMode
	overlay        OverlayKind
	avatarRendered string

	activePage *page
	pages      map[string]*page

	viewGen             uint64
	vc                  *viewCache
	currentWorktreePage string

	overlayBaseFocus models.PaneID

	pluginRegistry     *plugins.Registry
	adapterManager     *adapters.Manager
	agentRegistry      *agents.Registry
	resumeSummaryCache map[string]gitmodel.WorktreeResumeSummary
	mcpServer          *mcp.Server

	lastAgentSync time.Time

	orch          *orchestrator.Orchestrator
	notifications []orchestrator.Notification

	disablePiggyback bool

	// cmdBus is the command-layer bus for structured domain writes.
	cmdBus *commands.Bus
}

type editorMetaProvider interface {
	FilePath() string
	Dirty() bool
	DisplayName() string
}

type tabHandler interface {
	HandleTab() bool
}

// New creates and returns the initial application model.
func New(cfg config.Config, store models.Store) tea.Model {
	cm := &models.CommonModel{
		Theme: styles.DefaultTheme(),
		Cfg:   cfg,
		Store: store,
	}

	cwd, _ := os.Getwd()
	repoRoot, _ := gitRepoRoot(cwd)

	m := model{
		common:             cm,
		state:              StateDashboard,
		mode:               ModeNormal,
		overlay:            OverlayNone,
		vc:                 &viewCache{},
		pluginRegistry:     plugins.NewRegistry(),
		adapterManager:     adapters.NewManager(),
		agentRegistry:      agents.NewRegistry(),
		pages:              make(map[string]*page),
		resumeSummaryCache: make(map[string]gitmodel.WorktreeResumeSummary),
		mcpServer:          mcp.NewServer(cfg.Agent.MCPSocket, cfg.Agent.MCPPort),
		disablePiggyback:   os.Getenv("FOCUS_DISABLE_PIGGYBACK") == "1",
	}
	m.registerMCPTools()
	m.registerMCPResources()
	if err := m.mcpServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "focus: mcp server start failed: %v\n", err)
	}
	if url, err := m.mcpServer.StartHTTP(); err != nil {
		fmt.Fprintf(os.Stderr, "focus: mcp http start failed: %v\n", err)
	} else if url != "" {
		m.common.Cfg.Agent.MCPSocket = url
	}

	gitAdapter := adapters.NewGitLocalAdapter()
	_ = m.adapterManager.Register(gitAdapter.Name(), gitAdapter)

	gitPlugin := gitplugin.New(m.adapterManager.Git())
	fileTreePlugin := filebrowser.New()
	editorPlugin := editorplugin.New()
	agentPlugin := agentsplugin.New()
	_ = m.pluginRegistry.Register(gitPlugin)
	_ = m.pluginRegistry.Register(fileTreePlugin)
	_ = m.pluginRegistry.Register(editorPlugin)
	_ = m.pluginRegistry.Register(agentPlugin)

	m.activePage = newOverviewPage(cm, m.pluginRegistry, m.adapterManager, cfg, store, cwd, repoRoot)
	m.pages[""] = m.activePage

	// Phase 3: initialise command bus when backed by the concrete store.
	if st, ok := store.(*dbstore.Store); ok {
		m.cmdBus = commands.NewBus(st)
	}

	m.loadPageSnapshots()
	m.syncWorktreeActivities()

	// Phase 4: start resident orchestrator when event bus is available.
	if bus := store.EventBus(); bus != nil {
		m.orch = orchestrator.New(appOrchestratorStore{model: &m}, bus)
	}

	return m
}

func (m *model) registerMCPTools() {
	if m == nil || m.mcpServer == nil {
		return
	}
	// Schemas with Chinese-first title instructions
	taskCreateSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "任务标题，必须使用中文，简洁明了，采用动宾结构（如「修复登录接口缓存问题」）",
			},
			"goal": map[string]any{
				"type":        "string",
				"description": "任务目标描述（中文优先）",
			},
			"next_step": map[string]any{
				"type":        "string",
				"description": "下一步行动（中文优先）",
			},
			"state": map[string]any{
				"type":        "string",
				"description": "初始状态：active, paused, ready, blocked, done",
			},
			"priority": map[string]any{
				"type":        "string",
				"description": "优先级：low, medium, high, critical",
			},
			"repo_id": map[string]any{
				"type":        "string",
				"description": "仓库路径（可选，默认当前仓库）",
			},
			"preferred_worktree_id": map[string]any{
				"type":        "string",
				"description": "偏好的工作树 ID",
			},
		},
		"required": []string{"title"},
	}
	planCreateSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "计划标题，必须使用中文，概括整个计划的核心目标（如「重构认证模块 v2」）",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "关联的任务 ID",
			},
			"why_now": map[string]any{
				"type":        "string",
				"description": "为什么要现在做这个计划（中文优先）",
			},
			"success": map[string]any{
				"type":        "string",
				"description": "成功标准（中文优先）",
			},
			"out_of_scope": map[string]any{
				"type":        "string",
				"description": "明确排除的范围（中文优先）",
			},
			"known_risks": map[string]any{
				"type":        "string",
				"description": "已知风险（中文优先）",
			},
			"plan_body": map[string]any{
				"type":        "string",
				"description": "计划正文，使用中文描述各步骤",
			},
		},
		"required": []string{"title"},
	}
	planAddStepSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"plan_id": map[string]any{
				"type":        "string",
				"description": "计划 ID",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "步骤标题，必须使用中文，简洁具体（如「编写单元测试覆盖登录流程」）",
			},
			"order_index": map[string]any{
				"type":        "number",
				"description": "步骤顺序索引（可选，默认追加到最后）",
			},
			"notes": map[string]any{
				"type":        "string",
				"description": "步骤备注（中文优先）",
			},
		},
		"required": []string{"plan_id", "title"},
	}

	_ = m.mcpServer.RegisterTool("session.heartbeat", "Report agent session heartbeat", nil, m.withPiggyback(m.mcpSessionHeartbeatTool))
	_ = m.mcpServer.RegisterTool("session.request_intervention", "Request human intervention", nil, m.withPiggyback(m.mcpSessionRequestInterventionTool))
	_ = m.mcpServer.RegisterTool("task.get", "Get task details by ID", nil, m.withPiggyback(m.mcpTaskGetTool))
	_ = m.mcpServer.RegisterTool("task.create", "创建一个新任务。title 必须使用中文，简洁动宾结构", taskCreateSchema, m.withPiggyback(m.mcpTaskCreateTool))
	_ = m.mcpServer.RegisterTool("task.list", "List tasks", nil, m.withPiggyback(m.mcpTaskListTool))
	_ = m.mcpServer.RegisterTool("task.add_dependency", "Add dependency between tasks", nil, m.withPiggyback(m.mcpTaskAddDependencyTool))
	_ = m.mcpServer.RegisterTool("task.create_output", "Create task output/artifact", nil, m.withPiggyback(m.mcpTaskCreateOutputTool))
	_ = m.mcpServer.RegisterTool("task.update_status", "Update task status", nil, m.withPiggyback(m.mcpTaskUpdateStatusTool))
	_ = m.mcpServer.RegisterTool("kg.add_fact", "Add a knowledge graph fact", nil, m.withPiggyback(m.mcpKnowledgeAddFactTool))
	_ = m.mcpServer.RegisterTool("context.get_for_task", "Get full context for a task", nil, m.withPiggyback(m.mcpContextGetForTaskTool))
	_ = m.mcpServer.RegisterTool("plan.create", "创建一个新的任务计划。title 必须使用中文，概括核心目标", planCreateSchema, m.withPiggyback(m.mcpPlanCreateTool))
	_ = m.mcpServer.RegisterTool("plan.get", "Get plan details with steps", nil, m.withPiggyback(m.mcpPlanGetTool))
	_ = m.mcpServer.RegisterTool("plan.list", "List task plans", nil, m.withPiggyback(m.mcpPlanListTool))
	_ = m.mcpServer.RegisterTool("plan.add_step", "为计划添加一个步骤。title 必须使用中文，简洁具体", planAddStepSchema, m.withPiggyback(m.mcpPlanAddStepTool))
	_ = m.mcpServer.RegisterTool("plan.expand_to_tasks", "将计划步骤展开为任务和依赖关系", nil, m.withPiggyback(m.mcpPlanExpandToTasksTool))
	_ = m.mcpServer.RegisterTool("dag.get_status", "获取完整 DAG 状态和拓扑结构", nil, m.withPiggyback(m.mcpDagGetStatusTool))
}

func (m *model) registerMCPResources() {
	if m == nil || m.mcpServer == nil || m.common == nil || m.common.Store == nil {
		return
	}
	_ = m.mcpServer.RegisterResource("context://tasks", "Tasks", "All tasks in the current repository", "application/json", m.mcpTasksResource)
	_ = m.mcpServer.RegisterResource("context://plans", "Plans", "All task plans in the current repository", "application/json", m.mcpPlansResource)
	_ = m.mcpServer.RegisterResource("context://worktrees", "Worktrees", "All known worktrees", "application/json", m.mcpWorktreesResource)
}

func (m *model) mcpTasksResource(uri string) (mcp.ResourceContent, error) {
	repoID := m.gitRepoPath()
	tasks, err := m.common.Store.ListTaskContexts(repoID)
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	b, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	return mcp.ResourceContent{URI: uri, MimeType: "application/json", Text: string(b)}, nil
}

func (m *model) mcpPlansResource(uri string) (mcp.ResourceContent, error) {
	plans, err := m.common.Store.ListTaskPlans("")
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	b, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	return mcp.ResourceContent{URI: uri, MimeType: "application/json", Text: string(b)}, nil
}

func (m *model) mcpWorktreesResource(uri string) (mcp.ResourceContent, error) {
	repoID := m.gitRepoPath()
	worktrees, err := m.common.Store.ListWorktreeContexts(repoID)
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	b, err := json.MarshalIndent(worktrees, "", "  ")
	if err != nil {
		return mcp.ResourceContent{}, err
	}
	return mcp.ResourceContent{URI: uri, MimeType: "application/json", Text: string(b)}, nil
}

func (m *model) mcpSessionHeartbeatTool(params map[string]any) (map[string]any, error) {
	sessionID := toolStringParam(params, "session_id")
	if sessionID == "" {
		return nil, fmt.Errorf("session_id required")
	}
	state := toolStringParam(params, "status")
	if state == "" {
		state = toolStringParam(params, "state")
	}
	now := time.Now()
	if atRaw := toolStringParam(params, "at"); atRaw != "" {
		if parsed, err := time.Parse(time.RFC3339, atRaw); err == nil {
			now = parsed
		}
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	if err := m.cmdBus.Send(context.Background(), &commands.HeartbeatSession{
		SessionID: sessionID,
		At:        now,
		State:     state,
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"success":    true,
		"session_id": sessionID,
		"status":     state,
		"at":         now.UTC().Format(time.RFC3339),
	}, nil
}

func (m *model) mcpTaskGetTool(params map[string]any) (map[string]any, error) {
	taskID := toolStringParam(params, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	record, err := m.common.Store.GetTaskContext(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	task := map[string]any{
		"id":                    record.ID,
		"repo_id":               record.RepoID,
		"title":                 record.Title,
		"goal":                  record.Goal,
		"next_step":             record.NextStep,
		"state":                 record.State,
		"priority":              record.Priority,
		"preferred_worktree_id": record.PreferredWorktreeID,
	}
	return map[string]any{
		"task": task,
	}, nil
}

func (m *model) mcpTaskCreateOutputTool(params map[string]any) (map[string]any, error) {
	taskID := toolStringParam(params, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}
	output := strings.TrimSpace(toolStringParam(params, "output"))
	if output == "" {
		output = strings.TrimSpace(toolStringParam(params, "content"))
	}
	if output == "" {
		return nil, fmt.Errorf("output required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	record, err := m.common.Store.GetTaskContext(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	outputID := "out-" + uuid.NewString()
	actor := resolveToolActor(params)
	if err := m.cmdBus.Send(context.Background(), &commands.CreateTaskOutput{
		ID:      outputID,
		TaskID:  taskID,
		Content: output,
		Actor:   actor,
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"success":    true,
		"task_id":    taskID,
		"output_id":  outputID,
		"output_ref": fmt.Sprintf("mcp://task-board/%s/output/%s", taskID, outputID),
	}, nil
}

func (m *model) mcpTaskUpdateStatusTool(params map[string]any) (map[string]any, error) {
	taskID := toolStringParam(params, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}
	nextState := normalizeToolTaskState(toolStringParam(params, "state"))
	if nextState == "" {
		return nil, fmt.Errorf("state required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	record, err := m.common.Store.GetTaskContext(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	prevState := record.State
	if err := m.cmdBus.Send(context.Background(), &commands.UpdateTaskState{
		TaskID:   taskID,
		NewState: nextState,
	}); err != nil {
		return nil, err
	}
	createdOutputID := ""
	summary := strings.TrimSpace(toolStringParam(params, "summary"))
	if summary != "" {
		created, err := m.mcpTaskCreateOutputTool(map[string]any{
			"task_id": taskID,
			"output":  summary,
			"actor":   resolveToolActor(params),
		})
		if err != nil {
			return nil, err
		}
		createdOutputID = toolStringParam(created, "output_id")
	}
	launchedTaskIDs := make([]string, 0)
	if prevState != "done" && nextState == "done" {
		triggered, err := m.launchDownstreamTasksProtocol(taskID)
		if err != nil {
			return nil, err
		}
		launchedTaskIDs = append(launchedTaskIDs, triggered...)
	}
	if sessionID := toolStringParam(params, "session_id"); sessionID != "" && nextState == "done" {
		m.markSessionCompleted(sessionID, toolStringParam(params, "summary"))
	}
	return map[string]any{
		"success":             true,
		"task_id":             taskID,
		"previous_state":      prevState,
		"state":               nextState,
		"summary_output_id":   createdOutputID,
		"launched_task_ids":   launchedTaskIDs,
		"launched_task_count": len(launchedTaskIDs),
	}, nil
}

func (m *model) mcpKnowledgeAddFactTool(params map[string]any) (map[string]any, error) {
	subject := strings.TrimSpace(toolStringParam(params, "subject"))
	predicate := strings.TrimSpace(toolStringParam(params, "predicate"))
	object := strings.TrimSpace(toolStringParam(params, "object"))
	if subject == "" || predicate == "" || object == "" {
		return nil, fmt.Errorf("subject/predicate/object required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}

	planID := strings.TrimSpace(toolStringParam(params, "plan_id"))
	if planID == "" {
		planID = m.resolvePlanIDForTask(toolStringParam(params, "task_id"))
	}
	factID := "fact-" + uuid.NewString()
	if err := m.cmdBus.Send(context.Background(), &commands.AddKnowledgeFact{
		ID:         factID,
		PlanID:     planID,
		Subject:    subject,
		Predicate:  predicate,
		Object:     object,
		Source:     resolveToolActor(params),
		Confidence: toolFloatParam(params, "confidence", 1),
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"success": true,
		"fact_id": factID,
		"plan_id": planID,
	}, nil
}

func (m *model) mcpSessionRequestInterventionTool(params map[string]any) (map[string]any, error) {
	sessionID := strings.TrimSpace(toolStringParam(params, "session_id"))
	reason := strings.TrimSpace(toolStringParam(params, "reason"))
	if sessionID == "" {
		return nil, fmt.Errorf("session_id required")
	}
	if reason == "" {
		return nil, fmt.Errorf("reason required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	interventionID := "int-" + uuid.NewString()
	if err := m.cmdBus.Send(context.Background(), &commands.RequestIntervention{
		ID:        interventionID,
		SessionID: sessionID,
		Reason:    reason,
		Actor:     resolveToolActor(params),
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"success":         true,
		"intervention_id": interventionID,
	}, nil
}

func (m *model) mcpContextGetForTaskTool(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(toolStringParam(params, "task_id"))
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	task, err := m.common.Store.GetTaskContext(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	brief, _ := m.common.Store.GetTaskBrief(taskID)
	upstreamTasks, _ := m.common.Store.ListUpstreamTaskContexts(taskID)

	upstreamOutputs := make([]map[string]any, 0)
	for _, upstream := range upstreamTasks {
		outputs, err := m.common.Store.ListTaskOutputs(upstream.ID)
		if err != nil {
			continue
		}
		for _, output := range outputs {
			upstreamOutputs = append(upstreamOutputs, map[string]any{
				"id":         output.ID,
				"task_id":    output.TaskID,
				"task_title": upstream.Title,
				"content":    output.Content,
				"actor":      output.Actor,
				"created_at": output.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
	}

	planID := m.resolvePlanIDForTask(taskID)
	facts, _ := m.common.Store.ListKnowledgeFacts(planID)
	knowledgeFacts := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		knowledgeFacts = append(knowledgeFacts, map[string]any{
			"id":         fact.ID,
			"plan_id":    fact.PlanID,
			"subject":    fact.Subject,
			"predicate":  fact.Predicate,
			"object":     fact.Object,
			"source":     fact.Source,
			"confidence": fact.Confidence,
		})
	}

	worktreeInfo := map[string]any{}
	if task.PreferredWorktreeID != "" {
		if wc, err := m.common.Store.GetWorktreeContext(task.PreferredWorktreeID); err == nil && wc != nil {
			worktreeInfo["worktree_id"] = wc.WorktreeID
			worktreeInfo["repo_id"] = wc.RepoID
			worktreeInfo["task_mode"] = wc.TaskMode
			worktreeInfo["task_name"] = wc.TaskName
			worktreeInfo["branch_snapshot"] = wc.BranchSnapshot
		}
	}

	taskBrief := map[string]any{}
	if brief != nil {
		taskBrief["why_now"] = brief.WhyNow
		taskBrief["success_criteria"] = brief.SuccessCriteria
		taskBrief["out_of_scope"] = brief.OutOfScope
		taskBrief["known_risks"] = brief.KnownRisks
	}

	upstreamTaskPayload := make([]map[string]any, 0, len(upstreamTasks))
	for _, upstream := range upstreamTasks {
		upstreamTaskPayload = append(upstreamTaskPayload, map[string]any{
			"id":        upstream.ID,
			"title":     upstream.Title,
			"state":     upstream.State,
			"next_step": upstream.NextStep,
		})
	}

	result := map[string]any{
		"task": map[string]any{
			"id":                    task.ID,
			"repo_id":               task.RepoID,
			"title":                 task.Title,
			"goal":                  task.Goal,
			"next_step":             task.NextStep,
			"state":                 task.State,
			"priority":              task.Priority,
			"preferred_worktree_id": task.PreferredWorktreeID,
		},
		"task_brief":       taskBrief,
		"plan_id":          planID,
		"upstream_tasks":   upstreamTaskPayload,
		"upstream_outputs": upstreamOutputs,
		"knowledge_facts":  knowledgeFacts,
		"worktree_info":    worktreeInfo,
		"adr_constraints":  []any{},
	}
	return result, nil
}

func (m *model) mcpTaskCreateTool(params map[string]any) (map[string]any, error) {
	repoID := strings.TrimSpace(toolStringParam(params, "repo_id"))
	if repoID == "" {
		repoID = m.gitRepoPath()
	}
	if repoID == "" {
		return nil, fmt.Errorf("repo_id required")
	}
	title := strings.TrimSpace(toolStringParam(params, "title"))
	if title == "" {
		return nil, fmt.Errorf("title required")
	}
	if m.cmdBus == nil {
		return nil, fmt.Errorf("command bus unavailable")
	}
	cmd := &commands.CreateTask{
		ID:                  uuid.NewString(),
		RepoID:              repoID,
		Title:               title,
		Goal:                strings.TrimSpace(toolStringParam(params, "goal")),
		NextStep:            strings.TrimSpace(toolStringParam(params, "next_step")),
		State:               normalizeToolTaskState(toolStringParam(params, "state")),
		Priority:            toolStringParam(params, "priority"),
		PreferredWorktreeID: strings.TrimSpace(toolStringParam(params, "preferred_worktree_id")),
	}
	if err := m.cmdBus.Send(context.Background(), cmd); err != nil {
		return nil, err
	}
	// Refresh DAG if visible
	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.refreshDAG()
	}
	m.invalidateView()
	m.syncWorktreeActivities()
	return map[string]any{
		"success": true,
		"task_id": cmd.ID,
		"title":   cmd.Title,
		"state":   cmd.State,
	}, nil
}

func (m *model) mcpTaskAddDependencyTool(params map[string]any) (map[string]any, error) {
	fromTaskID := strings.TrimSpace(toolStringParam(params, "from_task_id"))
	toTaskID := strings.TrimSpace(toolStringParam(params, "to_task_id"))
	if fromTaskID == "" || toTaskID == "" {
		return nil, fmt.Errorf("from_task_id and to_task_id required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	depType := toolStringParam(params, "dependency_type")
	if depType == "" {
		depType = "hard"
	}
	if err := m.cmdBus.Send(context.Background(), &commands.AddTaskDependency{
		FromTaskID:     fromTaskID,
		ToTaskID:       toTaskID,
		DependencyType: depType,
	}); err != nil {
		return nil, err
	}
	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.refreshDAG()
	}
	m.invalidateView()
	return map[string]any{
		"success":         true,
		"from_task_id":    fromTaskID,
		"to_task_id":      toTaskID,
		"dependency_type": depType,
	}, nil
}

func (m *model) mcpTaskListTool(params map[string]any) (map[string]any, error) {
	repoID := strings.TrimSpace(toolStringParam(params, "repo_id"))
	if repoID == "" {
		repoID = m.gitRepoPath()
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	records, err := m.common.Store.ListTaskContexts(repoID)
	if err != nil {
		return nil, err
	}
	tasks := make([]map[string]any, 0, len(records))
	for _, t := range records {
		tasks = append(tasks, map[string]any{
			"id":                    t.ID,
			"repo_id":               t.RepoID,
			"title":                 t.Title,
			"state":                 t.State,
			"priority":              t.Priority,
			"preferred_worktree_id": t.PreferredWorktreeID,
		})
	}
	return map[string]any{
		"success": true,
		"count":   len(tasks),
		"tasks":   tasks,
	}, nil
}

func (m *model) mcpPlanCreateTool(params map[string]any) (map[string]any, error) {
	title := strings.TrimSpace(toolStringParam(params, "title"))
	if title == "" {
		return nil, fmt.Errorf("title required")
	}
	if m.cmdBus == nil {
		return nil, fmt.Errorf("command bus unavailable")
	}
	cmd := &commands.CreatePlan{
		ID:         uuid.NewString(),
		TaskID:     strings.TrimSpace(toolStringParam(params, "task_id")),
		Title:      title,
		WhyNow:     strings.TrimSpace(toolStringParam(params, "why_now")),
		Success:    strings.TrimSpace(toolStringParam(params, "success")),
		OutOfScope: strings.TrimSpace(toolStringParam(params, "out_of_scope")),
		KnownRisks: strings.TrimSpace(toolStringParam(params, "known_risks")),
		PlanBody:   strings.TrimSpace(toolStringParam(params, "plan_body")),
	}
	if err := m.cmdBus.Send(context.Background(), cmd); err != nil {
		return nil, err
	}
	return map[string]any{
		"success": true,
		"plan_id": cmd.ID,
		"title":   cmd.Title,
		"status":  "draft",
	}, nil
}

func (m *model) mcpPlanGetTool(params map[string]any) (map[string]any, error) {
	planID := strings.TrimSpace(toolStringParam(params, "plan_id"))
	if planID == "" {
		return nil, fmt.Errorf("plan_id required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	plan, err := m.common.Store.GetTaskPlan(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("plan %q not found", planID)
	}
	steps, err := m.common.Store.ListPlanSteps(planID)
	if err != nil {
		return nil, err
	}
	stepItems := make([]map[string]any, 0, len(steps))
	for _, s := range steps {
		stepItems = append(stepItems, map[string]any{
			"id":               s.ID,
			"order_index":      s.OrderIndex,
			"title":            s.Title,
			"state":            s.State,
			"expanded_task_id": s.ExpandedTaskID,
			"notes":            s.Notes,
		})
	}
	return map[string]any{
		"success": true,
		"plan": map[string]any{
			"id":           plan.ID,
			"task_id":      plan.TaskID,
			"title":        plan.Title,
			"why_now":      plan.WhyNow,
			"success":      plan.Success,
			"out_of_scope": plan.OutOfScope,
			"known_risks":  plan.KnownRisks,
			"status":       plan.Status,
			"current_step": plan.CurrentStep,
			"plan_body":    plan.PlanBody,
		},
		"steps": stepItems,
	}, nil
}

func (m *model) mcpPlanListTool(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(toolStringParam(params, "task_id"))
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	records, err := m.common.Store.ListTaskPlans(taskID)
	if err != nil {
		return nil, err
	}
	plans := make([]map[string]any, 0, len(records))
	for _, p := range records {
		plans = append(plans, map[string]any{
			"id":           p.ID,
			"task_id":      p.TaskID,
			"title":        p.Title,
			"status":       p.Status,
			"current_step": p.CurrentStep,
		})
	}
	return map[string]any{
		"success": true,
		"count":   len(plans),
		"plans":   plans,
	}, nil
}

func (m *model) mcpPlanAddStepTool(params map[string]any) (map[string]any, error) {
	planID := strings.TrimSpace(toolStringParam(params, "plan_id"))
	if planID == "" {
		return nil, fmt.Errorf("plan_id required")
	}
	title := strings.TrimSpace(toolStringParam(params, "title"))
	if title == "" {
		return nil, fmt.Errorf("title required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	// Verify plan exists
	plan, err := m.common.Store.GetTaskPlan(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("plan %q not found", planID)
	}
	// Determine order_index if not provided
	orderIndex := int(toolFloatParam(params, "order_index", -1))
	if orderIndex < 0 {
		steps, _ := m.common.Store.ListPlanSteps(planID)
		orderIndex = len(steps)
	}
	stepID := uuid.NewString()
	if err := m.cmdBus.Send(context.Background(), &commands.AddPlanStep{
		ID:         stepID,
		PlanID:     planID,
		Title:      title,
		Notes:      strings.TrimSpace(toolStringParam(params, "notes")),
		OrderIndex: orderIndex,
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"success":     true,
		"step_id":     stepID,
		"plan_id":     planID,
		"order_index": orderIndex,
	}, nil
}

func (m *model) mcpPlanExpandToTasksTool(params map[string]any) (map[string]any, error) {
	planID := strings.TrimSpace(toolStringParam(params, "plan_id"))
	if planID == "" {
		return nil, fmt.Errorf("plan_id required")
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	plan, err := m.common.Store.GetTaskPlan(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("plan %q not found", planID)
	}
	steps, err := m.common.Store.ListPlanSteps(planID)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("plan %q has no steps to expand", planID)
	}
	repoID := strings.TrimSpace(toolStringParam(params, "repo_id"))
	if repoID == "" {
		repoID = m.gitRepoPath()
	}
	if repoID == "" {
		return nil, fmt.Errorf("repo_id required")
	}

	// Create a task for each step and build dependency chain
	taskIDs := make([]string, 0, len(steps))
	taskIDMap := make(map[int]string) // step order_index -> task_id
	dependencies := make([]map[string]any, 0)
	var firstTaskID string

	for i, step := range steps {
		taskID := uuid.NewString()
		taskIDMap[step.OrderIndex] = taskID
		taskIDs = append(taskIDs, taskID)
		if i == 0 {
			firstTaskID = taskID
		}

		state := "blocked"
		if i == 0 {
			state = "active"
		}
		if err := m.cmdBus.Send(context.Background(), &commands.CreateTask{
			ID:     taskID,
			RepoID: repoID,
			Title:  step.Title,
			Goal:   step.Notes,
			State:  state,
		}); err != nil {
			return nil, fmt.Errorf("save task for step %q: %w", step.Title, err)
		}

		// Update step with expanded task ID
		step.ExpandedTaskID = taskID
		if err := m.cmdBus.Send(context.Background(), &commands.UpdatePlanStep{Record: step}); err != nil {
			return nil, fmt.Errorf("update step %q: %w", step.Title, err)
		}
	}

	// Create sequential dependencies: step[i] -> step[i+1]
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].OrderIndex < steps[j].OrderIndex
	})
	for i := 0; i < len(steps)-1; i++ {
		fromID := taskIDMap[steps[i].OrderIndex]
		toID := taskIDMap[steps[i+1].OrderIndex]
		if err := m.cmdBus.Send(context.Background(), &commands.AddTaskDependency{
			FromTaskID:     fromID,
			ToTaskID:       toID,
			DependencyType: "hard",
		}); err != nil {
			return nil, fmt.Errorf("save dependency: %w", err)
		}
		dependencies = append(dependencies, map[string]any{
			"from_task_id": fromID,
			"to_task_id":   toID,
		})
	}

	// Update plan status to active
	plan.Status = "active"
	if err := m.cmdBus.Send(context.Background(), &commands.UpdatePlan{Record: *plan}); err != nil {
		return nil, err
	}

	// Refresh DAG if visible
	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.refreshDAG()
	}
	m.invalidateView()
	m.syncWorktreeActivities()

	return map[string]any{
		"success":       true,
		"plan_id":       planID,
		"task_ids":      taskIDs,
		"first_task_id": firstTaskID,
		"dependencies":  dependencies,
		"task_count":    len(taskIDs),
	}, nil
}

func (m *model) mcpDagGetStatusTool(params map[string]any) (map[string]any, error) {
	repoID := strings.TrimSpace(toolStringParam(params, "repo_id"))
	if repoID == "" {
		repoID = m.gitRepoPath()
	}
	if m.common == nil || m.common.Store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	tasks, err := m.common.Store.ListTaskContexts(repoID)
	if err != nil {
		return nil, err
	}

	nodes := make([]map[string]any, 0, len(tasks))
	nodeMap := make(map[string]map[string]any)
	for _, t := range tasks {
		node := map[string]any{
			"id":       t.ID,
			"title":    t.Title,
			"state":    t.State,
			"priority": t.Priority,
		}
		nodes = append(nodes, node)
		nodeMap[t.ID] = node
	}

	edges := make([]map[string]any, 0)
	adjacency := make(map[string][]string)
	indegree := make(map[string]int)
	seen := make(map[string]struct{})

	for _, t := range tasks {
		downstream, err := m.common.Store.ListDownstreamTaskContexts(t.ID)
		if err != nil {
			continue
		}
		for _, d := range downstream {
			if _, ok := nodeMap[d.ID]; !ok {
				continue
			}
			key := t.ID + "->" + d.ID
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, map[string]any{
				"from": t.ID,
				"to":   d.ID,
				"type": "hard",
			})
			adjacency[t.ID] = append(adjacency[t.ID], d.ID)
			indegree[d.ID]++
		}
	}

	// Compute levels via topological BFS
	levels := make(map[string]int)
	queue := make([]string, 0)
	for _, t := range tasks {
		if indegree[t.ID] == 0 {
			queue = append(queue, t.ID)
			levels[t.ID] = 0
		}
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[curr] {
			if levels[next] < levels[curr]+1 {
				levels[next] = levels[curr] + 1
			}
			queue = append(queue, next)
		}
	}
	for _, n := range nodes {
		id := n["id"].(string)
		if lv, ok := levels[id]; ok {
			n["level"] = lv
		} else {
			n["level"] = 0
		}
	}

	readyCount, doneCount := 0, 0
	for _, t := range tasks {
		switch t.State {
		case "ready":
			readyCount++
		case "done":
			doneCount++
		}
	}

	return map[string]any{
		"success":      true,
		"repo_id":      repoID,
		"nodes":        nodes,
		"edges":        edges,
		"total_count":  len(tasks),
		"ready_count":  readyCount,
		"done_count":   doneCount,
		"active_count": len(tasks) - readyCount - doneCount,
	}, nil
}

func (m *model) launchDownstreamTasksProtocol(taskID string) ([]string, error) {
	// Phase 4: Orchestrator is now resident. Downstream readiness is
	// detected via the event bus and surfaced as TUI notifications.
	// Auto-launch is intentionally removed per ADR-0000 Human Sovereignty.
	return nil, nil
}

func (m *model) markSessionCompleted(sessionID, summary string) {
	if sessionID == "" {
		return
	}
	record := m.agentSessionRecord(sessionID)
	if record.ID == "" || record.WorktreeID == "" {
		return
	}
	now := time.Now()
	record.State = string(agents.SessionExited)
	record.PID = 0
	record.StopReason = ""
	record.EndedAt = &now
	record.LastActivityAt = &now
	record.LastHeartbeat = &now
	record.UpdatedAt = now
	if strings.TrimSpace(summary) != "" {
		record.Summary = strings.TrimSpace(summary)
	}
	m.saveAgentSessionRecord(record)
}

func (m *model) resolvePlanIDForTask(taskID string) string {
	if m == nil || m.common == nil || m.common.Store == nil || strings.TrimSpace(taskID) == "" {
		return ""
	}
	plans, err := m.common.Store.ListTaskPlans(taskID)
	if err != nil || len(plans) == 0 {
		return ""
	}
	return plans[0].ID
}

func resolveToolActor(params map[string]any) string {
	for _, key := range []string{"actor", "source", "session_id"} {
		if value := strings.TrimSpace(toolStringParam(params, key)); value != "" {
			return value
		}
	}
	return "unknown"
}

func toolFloatParam(params map[string]any, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	value, exists := params[key]
	if !exists || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func toJSONString(payload map[string]any) string {
	if payload == nil {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func toolStringParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	value, exists := params[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func normalizeToolTaskState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "completed":
		return "done"
	case "in_progress":
		return "active"
	default:
		return strings.TrimSpace(state)
	}
}

func (m *model) registerPane(id models.PaneID, panel models.Panel, meta models.PaneMeta) {
	m.activePage.registerPane(id, panel, meta)
}

func (m *model) pane(id models.PaneID) models.Panel {
	return m.activePage.pane(id)
}

func (m *model) setPane(id models.PaneID, panel models.Panel) {
	m.activePage.setPane(id, panel)
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.adapterManager != nil {
		if err := m.adapterManager.Init(); err != nil {
			if meta, ok := m.activePage.paneMeta[paneWorktree]; ok {
				repoPath := meta.CWD
				cmds = append(cmds, func() tea.Msg {
					return adapters.StatusEvent{RepoPath: repoPath, Error: err}
				})
			}
		}
	}
	for _, id := range m.activePage.paneOrder {
		if cmd := m.pane(id).Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cfg := m.common.Cfg; cfg.Avatar.Image != "" {
		cmds = append(cmds, func() tea.Msg {
			art := avatar.Render(cfg.Avatar.Image, cfg.Avatar.Width)
			return avatarRenderedMsg{art: art}
		})
	}
	if m.orch != nil {
		m.orch.Start(context.Background())
		cmds = append(cmds, m.orchestratorCmd())
	}
	return tea.Batch(cmds...)
}

func (m *model) invalidateView() {
	m.viewGen++
}

// orchestratorCmd blocks until the next orchestrator notification arrives.
func (m *model) orchestratorCmd() tea.Cmd {
	return func() tea.Msg {
		if m.orch == nil {
			return nil
		}
		n, ok := <-m.orch.Notifications()
		if !ok {
			return nil
		}
		return orchNotificationMsg(n)
	}
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case avatarRenderedMsg:
		m.avatarRendered = msg.art
		m.invalidateView()
		return m, nil

	case orchNotificationMsg:
		m.notifications = append(m.notifications, orchestrator.Notification(msg))
		m.invalidateView()
		// Re-subscribe to the next notification.
		return m, m.orchestratorCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case dagNodeSelectedMsg:
		if msg.PreferredWorktreeID != "" {
			if wc, _ := m.common.Store.GetWorktreeContext(msg.PreferredWorktreeID); wc != nil {
				cmd := m.switchToWorktreePage(msg.PreferredWorktreeID, string(paneWorktreeDetail))
				m.syncWorktreeActivities()
				m.invalidateView()
				return m, cmd
			}
		}
		cmd := m.openCreateWorktreePane(gitplugin.OpenCreateWorktreeMsg{
			RepoPath:  m.gitRepoPath(),
			TaskTitle: msg.TaskTitle,
			TaskID:    msg.TaskID,
			BaseRef:   "master",
		})
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case dagCreateWorktreeMsg:
		cmd := m.openCreateWorktreePane(gitplugin.OpenCreateWorktreeMsg{
			RepoPath:  m.gitRepoPath(),
			TaskTitle: msg.TaskTitle,
			TaskID:    msg.TaskID,
			BaseRef:   "master",
		})
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case dagLaunchAgentMsg:
		var cmds []tea.Cmd
		if pageCmd := m.switchToWorktreePage(msg.WorktreeID, string(paneWorktreeDetail)); pageCmd != nil {
			cmds = append(cmds, pageCmd)
		}
		extraArgs := msg.ExtraArgs
		if msg.Provider == agents.ProviderClaude && agents.HasResumableClaudeSession(msg.WorktreeID) {
			extraArgs = append(extraArgs, "--continue")
		}
		session := m.newAgentSession(msg.WorktreeID, msg.Provider)
		session.ExtraArgs = extraArgs
		m.saveAgentSession(session)
		_ = m.prepareAgentProfile(session)
		if m.agentRegistry != nil {
			m.agentRegistry.Register(session)
		}
		if launchCmd := m.launchExternalAgent(session); launchCmd != nil {
			cmds = append(cmds, launchCmd)
		}
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, batchCmds(cmds)

	case dagAddTaskToTodoMsg:
		if m.common.Store != nil {
			taskID := msg.TaskID
			_, err := m.common.Store.CreateTodo(msg.TaskTitle, "today", &taskID)
			if err == nil {
				m.notifications = append(m.notifications, orchestrator.Notification{
					Title:    "Todo",
					Body:     fmt.Sprintf("Added \"%s\" to today", msg.TaskTitle),
					Severity: "info",
				})
				m.invalidateView()
			}
		}
		return m, nil

	case dagTaskCreatedMsg:
		if m.common == nil || m.common.Store == nil || m.cmdBus == nil {
			return m, nil
		}
		repoID := msg.RepoID
		if repoID == "" {
			repoID = m.gitRepoPath()
		}
		if repoID == "" {
			return m, nil
		}
		taskID := uuid.NewString()
		now := time.Now()
		_ = m.cmdBus.Send(context.Background(), &commands.UpdateTask{
			Record: models.TaskContextRecord{
				ID:        taskID,
				RepoID:    repoID,
				Title:     msg.Title,
				Goal:      msg.Goal,
				State:     "active",
				Priority:  "medium",
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
		m.invalidateView()
		return m, func() tea.Msg {
			return dagRefreshMsg{repoID: repoID}
		}

	case gitplugin.OpenDiffMsg:
		cmd := m.openDiffPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenCommitMsg:
		cmd := m.openCommitPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenCreateWorktreeMsg:
		cmd := m.openCreateWorktreePane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenWorktreeShellMsg:
		cmd := m.openWorktreeShell(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenWorktreeHistoryMsg:
		cmd := m.openWorktreeHistoryPane()
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenWorktreeDeleteConfirmMsg:
		cmd := m.openWorktreeDeleteConfirmPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case CloseWorktreeHistoryMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case CloseDeleteConfirmMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.ResumeWorktreeMsg:
		cmd := m.resumeWorktree(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenTaskEditMsg:
		cmd := m.openTaskEditPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.OpenPlanEditMsg:
		cmd := m.openPlanEditPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.CycleTaskStateMsg:
		cmd := m.cycleTaskState(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		repoID := m.gitRepoPath()
		return m, tea.Batch(cmd, func() tea.Msg { return dagRefreshMsg{repoID: repoID} })

	case CloseTaskEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case ClosePlanEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case todo.OverlayVisibleMsg:
		if !msg.Visible {
			if prev, ok := m.activePage.returnFocus[paneTodoOverlay]; ok {
				m.setFocus(prev)
				delete(m.activePage.returnFocus, paneTodoOverlay)
			}
			m.invalidateView()
		}
		return m, nil

	case todo.ModeChangeMsg:
		if msg.InputActive {
			m.mode = ModeInput
		} else {
			m.mode = ModeNormal
		}
		m.invalidateView()
		return m, nil

	case closeADRDetailMsg:
		m.closePane(paneADRDetail)
		return m, nil

	case TaskEditorSavedMsg:
		cmd := m.saveTaskEditor(msg)
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case PlanEditorSavedMsg:
		cmd := m.savePlanEditor(msg)
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case OpenAgentSelectMsg:
		cmd := m.openAgentSelectPane(msg.WorktreeID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agents.LaunchAgentMsg:
		cmd := m.launchAgent(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agents.AgentExitedMsg:
		m.handleAgentExited(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case agents.ExternalLaunchResultMsg:
		m.handleExternalLaunchResult(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case agents.ExternalShellLaunchedMsg:
		if msg.Err != nil {
			m.common.Notice = fmt.Sprintf("external shell failed: %v", msg.Err)
		} else {
			m.common.Notice = fmt.Sprintf("external shell opened at %s (pid %d)", msg.CWD, msg.PID)
		}
		m.invalidateView()
		return m, nil

	case agentsplugin.KillSessionMsg:
		cmd := m.killAgent(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case agentsplugin.FocusAgentSessionMsg:
		cmd := m.focusAgentSession(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case editorplugin.OpenEditorMsg:
		cmd := m.openEditorPane(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case openADRDetailMsg:
		cmd := m.activePage.openADRDetailOverlay(msg.FilePath)
		return m, cmd

	case editorplugin.CloseEditorMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case editorplugin.SaveCompletedMsg:
		m.syncPaneMeta(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseDiffMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseCommitMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CloseCreateWorktreeMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case gitplugin.CommitCompletedMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneWorktreeDetail]; ok {
			return m, m.routeToPane(paneWorktreeDetail, msg)
		}
		return m, nil

	case gitplugin.WorktreeCreatedMsg:
		m.closePane(msg.ID)
		var cmds []tea.Cmd
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			cmd := m.routeToPane(paneWorktree, gitplugin.RefreshWorktreesMsg{})
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if msg.TaskTitle != "" {
			if taskCmd := m.createTaskForWorktree(msg); taskCmd != nil {
				cmds = append(cmds, taskCmd)
			}
		}
		if msg.OpenExternal {
			// Open agent selection overlay instead of launching directly.
			if selectCmd := m.openAgentSelectPane(msg.Worktree.Path); selectCmd != nil {
				cmds = append(cmds, selectCmd)
			}
		} else {
			if cmd := m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: msg.Worktree}); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, batchCmds(cmds)

	case CloseAgentSelectMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case OpenProviderSelectMsg:
		m.closePane(msg.PaneID)
		if selectCmd := m.openProviderSelectPane(msg.WorktreeID, msg.Provider, msg.Resume); selectCmd != nil {
			m.syncWorktreeActivities()
			m.invalidateView()
			return m, selectCmd
		}
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case CloseProviderSelectMsg:
		m.closePane(msg.ID)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, nil

	case AgentSelectedMsg:
		m.closePane(msg.PaneID)
		var cmds []tea.Cmd
		if pageCmd := m.switchToWorktreePage(msg.WorktreeID, string(paneWorktreeDetail)); pageCmd != nil {
			cmds = append(cmds, pageCmd)
		}
		session := m.newAgentSession(msg.WorktreeID, msg.Provider)
		session.ProviderConfigID = msg.ProviderConfigID
		if msg.Resume {
			session.ExtraArgs = append(session.ExtraArgs, "--continue")
		}
		m.saveAgentSession(session)
		_ = m.prepareAgentProfile(session)
		if m.agentRegistry != nil {
			m.agentRegistry.Register(session)
		}
		if launchCmd := m.launchExternalAgent(session); launchCmd != nil {
			cmds = append(cmds, launchCmd)
		}
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, batchCmds(cmds)

	case gitplugin.RequestRemoveWorktreeMsg:
		m.closePane(paneWorktreeDeleteConfirm)
		cmd := m.removeWorktree(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.RequestPruneWorktreesMsg:
		cmd := m.pruneWorktrees(msg)
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, cmd

	case gitplugin.WorktreeRemovedMsg:
		m.closePanesForWorktree(msg.Path)

		// Clean up orphaned SQLite records now that the filesystem removal succeeded.
		if m.cmdBus != nil && msg.Path != "" {
			if err := m.cmdBus.Send(context.Background(), &commands.DeleteWorktree{WorktreePath: msg.Path}); err != nil {
				log.Printf("worktree removed but SQLite cleanup failed: %v", err)
			}
		}

		// Clean up agent registry and terminate any external agent processes.
		if m.agentRegistry != nil && msg.Path != "" {
			for _, s := range m.agentRegistry.ByWorktree(msg.Path) {
				m.agentRegistry.Remove(s.ID)
				if s.PID > 0 {
					_ = exec.Command("kill", "-TERM", strconv.Itoa(s.PID)).Run()
				}
			}
		}

		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			return m, m.routeToPane(paneWorktree, msg)
		}
		return m, nil

	case gitplugin.WorktreesPrunedMsg, gitplugin.WorktreeActionFailedMsg:
		m.syncWorktreeActivities()
		m.invalidateView()
		if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
			return m, m.routeToPane(paneWorktree, msg)
		}
		return m, nil

	case models.StatsRefreshMsg:
		m.invalidateView()
		if ft, ok := m.pane(paneFooter).(*footer.Model); ok {
			return m, ft.Refresh()
		}

	case shell.StartedMsg:
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case shell.RefreshMsg:
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case shell.ExitedMsg:
		m.syncWorktreeActivities()
		m.invalidateView()
		return m, m.routeToPane(msg.PaneID, msg)

	case shell.OpenExternalShellMsg:
		cmd := m.launchExternalShell(msg.CWD)
		return m, cmd

	case adapters.StatusEvent:
		m.syncWorktreeActivities()
		m.invalidateView()
		var cmds []tea.Cmd
		for _, id := range m.activePage.paneOrder {
			if m.activePage.paneMeta[id].Type == models.PaneTypeGitStatus {
				newPanel, cmd := m.pane(id).Update(msg)
				m.setPane(id, newPanel)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			// The worktree detail pane has a nested git status sub-pane.
			if id == paneWorktreeDetail {
				newPanel, cmd := m.pane(id).Update(msg)
				m.setPane(id, newPanel)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		m.refreshPaneStatuses()
		return m, batchCmds(cmds)

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		m.updateSizes(msg.Width, msg.Height)
		m.invalidateView()
	}

	var cmds []tea.Cmd
	var dirty bool
	for _, id := range m.activePage.paneOrder {
		newPanel, cmd := m.pane(id).Update(msg)
		if newPanel != m.pane(id) {
			dirty = true
			m.setPane(id, newPanel)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if dirty {
		m.invalidateView()
	}
	m.refreshPaneStatuses()
	return m, batchCmds(cmds)
}

func batchCmds(cmds []tea.Cmd) tea.Cmd {
	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()
	if m.activeOverlayPane() != "" {
		return m, nil
	}
	dims := layout.ComputeBanner(m.common.Width, m.common.Height)
	bodyY := msg.Y - dims.HeaderH
	bodyX := msg.X
	clicked := m.paneAt(bodyX, bodyY)
	if clicked != "" && msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown && msg.Button != tea.MouseButtonWheelLeft && msg.Button != tea.MouseButtonWheelRight {
		m.setFocus(clicked)
	}
	if clicked == "" {
		return m, nil
	}
	if m.activePage.paneMeta[clicked].Type != models.PaneTypeShell {
		return m, m.routeToPane(clicked, msg)
	}
	if m.mode != ModeShell {
		return m, nil
	}
	frame, ok := m.activePage.frames[clicked]
	if !ok {
		return m, nil
	}
	adjusted := msg
	adjusted.X = msg.X - frame.X - 2
	adjusted.Y = bodyY - frame.Y - 1
	contentW := frame.W - 4
	contentH := frame.H - 2
	if adjusted.X < 0 || adjusted.X >= contentW || adjusted.Y < 0 || adjusted.Y >= contentH {
		return m, nil
	}
	return m, m.routeToPane(clicked, tea.Msg(adjusted))
}

// handleKey routes keyboard input based on overlay, mode, and focused pane.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.invalidateView()

	if overlayID := m.activeOverlayPane(); overlayID != "" {
		return m, m.routeToPane(overlayID, msg)
	}

	if m.mode == ModeInput {
		if msg.String() == "ctrl+c" {
			m.closeShellPanes()
			return m, tea.Quit
		}
		return m, m.routeToPane(m.activePage.focused, msg)
	}

	if m.mode == ModeShell {
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			m.refreshPaneStatuses()
			return m, nil
		case "ctrl+t":
			m.mode = ModeNormal
			m.setFocus(paneShell)
			return m, nil
		case "ctrl+g":
			m.switchToOverviewPage()
			return m, nil
		default:
			if m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
				return m, m.routeToPane(m.activePage.focused, msg)
			}
			m.mode = ModeNormal
		}
	}

	switch msg.String() {
	case "ctrl+r":
		return m, m.globalRefreshCmd()
	case "q", "ctrl+c":
		m.closeShellPanes()
		return m, tea.Quit
	case "d":
		// Non-shell panes (e.g. worktree pane) may use 'd' for their own actions.
		// Route to focused pane first; only consume for notification dismissal if
		// the pane does not handle it.
		if m.activePage.focused != "" && m.activePage.paneMeta[m.activePage.focused].Type != models.PaneTypeShell {
			cmd := m.routeToPane(m.activePage.focused, msg)
			if cmd != nil {
				return m, cmd
			}
		}
		if len(m.notifications) > 0 {
			m.notifications = m.notifications[1:]
		}
		return m, nil
	case "ctrl+left", "ctrl+shift+left":
		return m.adjustFocusedSplit(layout.FocusLeft)
	case "ctrl+right", "ctrl+shift+right":
		return m.adjustFocusedSplit(layout.FocusRight)
	case "ctrl+up", "ctrl+shift+up":
		return m.adjustFocusedSplit(layout.FocusUp)
	case "ctrl+down", "ctrl+shift+down":
		return m.adjustFocusedSplit(layout.FocusDown)
	case "ctrl+t":
		if tp, ok := m.activePage.pane(paneTodoOverlay).(*todo.Model); ok {
			var cmd tea.Cmd
			if tp.Visible() {
				cmd = tp.SetVisible(false)
				if prev, ok := m.activePage.returnFocus[paneTodoOverlay]; ok {
					m.setFocus(prev)
					delete(m.activePage.returnFocus, paneTodoOverlay)
				}
			} else {
				if m.activePage.returnFocus == nil {
					m.activePage.returnFocus = make(map[models.PaneID]models.PaneID)
				}
				m.activePage.returnFocus[paneTodoOverlay] = m.activePage.focused
				cmd = tp.SetVisible(true)
				m.setFocus(paneTodoOverlay)
			}
			m.invalidateView()
			return m, cmd
		}
		return m, nil
	case "tab":
		if th, ok := m.activePage.pane(m.activePage.focused).(tabHandler); ok {
			if th.HandleTab() {
				return m, nil
			}
		}
		m.focusCycle(1)
		return m, nil
	case "shift+tab":
		m.focusCycle(-1)
		return m, nil
	case "ctrl+h":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusLeft))
		return m, nil
	case "ctrl+l":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusRight))
		return m, nil
	case "ctrl+k":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusUp))
		return m, nil
	case "ctrl+j":
		m.setFocus(layout.MoveFocus(m.activePage.focused, m.activePage.frames, layout.FocusDown))
		return m, nil
	case "ctrl+g":
		m.switchToOverviewPage()
		return m, nil
	case "enter":
		if m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
			m.mode = ModeShell
			m.refreshPaneStatuses()
			if m.activePage.paneMeta[m.activePage.focused].Status == models.PaneStatusExited {
				return m, m.routeToPane(m.activePage.focused, msg)
			}
			return m, nil
		}
	}

	if m.activePage.focused != "" && m.activePage.paneMeta[m.activePage.focused].Type != models.PaneTypeShell {
		return m, m.routeToPane(m.activePage.focused, msg)
	}

	return m, nil
}

func (m model) globalRefreshCmd() tea.Cmd {
	var cmds []tea.Cmd

	repoID := m.currentRepoID()
	if meta, ok := m.activePage.paneMeta[paneDAG]; ok && strings.TrimSpace(meta.RepoID) != "" {
		repoID = meta.RepoID
	}
	if repoID == "" {
		repoID = m.gitRepoPath()
	}

	if _, ok := m.activePage.paneMeta[paneDAG]; ok {
		if cmd := m.routeToPane(paneDAG, dagRefreshMsg{repoID: repoID}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if _, ok := m.activePage.paneMeta[paneWorktree]; ok {
		if cmd := m.routeToPane(paneWorktree, gitplugin.RefreshWorktreesMsg{}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if _, ok := m.activePage.paneMeta[paneWorktreeDetail]; ok {
		if cmd := m.routeToPane(paneWorktreeDetail, refreshWorktreeDetailMsg{}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if _, ok := m.activePage.paneMeta[paneTodoOverlay]; ok {
		if cmd := m.routeToPane(paneTodoOverlay, todo.RefreshTodosMsg{}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if ft, ok := m.pane(paneFooter).(*footer.Model); ok {
		if cmd := ft.Refresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return batchCmds(cmds)
}

func (m *model) setPaneStatus(id models.PaneID, status models.PaneStatus) {
	meta := m.activePage.paneMeta[id]
	meta.Status = status
	m.activePage.paneMeta[id] = meta
}

func (m *model) setFocus(id models.PaneID) {
	m.activePage.setFocus(id)
}

func (m *model) refreshPaneStatuses() {
	m.activePage.refreshPaneStatuses()
}

func (m *model) syncPaneMeta(id models.PaneID) {
	m.activePage.syncPaneMeta(id)
}

func (m *model) nextShellPaneID() models.PaneID {
	return m.activePage.nextShellPaneID()
}

func (m *model) nextEditorPaneID() models.PaneID {
	return m.activePage.nextEditorPaneID()
}

func (m *model) createShellPane() (models.PaneID, tea.Cmd) {
	return m.activePage.createShellPane()
}

func (m *model) createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot string) (models.PaneID, tea.Cmd) {
	return m.activePage.createShellPaneFor(cwd, repoID, worktreeID, branchSnapshot)
}

func (m *model) openWorktreeShell(msg gitplugin.OpenWorktreeShellMsg) tea.Cmd {
	worktreeID := msg.Worktree.Path
	if worktreeID == "" {
		worktreeID = m.currentWorktreeID()
	}
	pageCmd := m.switchToWorktreePage(worktreeID, "")
	cmd := m.activePage.openWorktreeShell(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && cmd != nil {
		return tea.Batch(pageCmd, cmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return cmd
}

func (m *model) resumeWorktree(msg gitplugin.ResumeWorktreeMsg) tea.Cmd {
	worktreeID := msg.Worktree.Path
	if worktreeID == "" {
		worktreeID = m.currentWorktreeID()
	}
	cmd := m.switchToWorktreePage(worktreeID, "")
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) createTaskForWorktree(msg gitplugin.WorktreeCreatedMsg) tea.Cmd {
	if msg.TaskTitle == "" {
		return nil
	}
	if m.common == nil || m.common.Store == nil || m.cmdBus == nil {
		return nil
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		return nil
	}
	now := time.Now()

	// If TaskID is set, link the existing task to the new worktree instead
	// of creating a duplicate.
	if msg.TaskID != "" {
		task, err := m.common.Store.GetTaskContext(msg.TaskID)
		if err != nil || task == nil {
			log.Printf("createTaskForWorktree: task %q not found: %v", msg.TaskID, err)
			return nil
		}
		task.PreferredWorktreeID = msg.Worktree.Path
		if err := m.cmdBus.Send(context.Background(), &commands.UpdateTask{Record: *task}); err != nil {
			log.Printf("createTaskForWorktree: update task preferred worktree: %v", err)
			return nil
		}
		if err := m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{
			Record: models.WorktreeContextRecord{
				WorktreeID:    msg.Worktree.Path,
				RepoID:        repoID,
				PrimaryTaskID: &msg.TaskID,
				TaskMode:      "single",
				TaskName:      msg.TaskTitle,
				LastActiveAt:  now,
			},
		}); err != nil {
			log.Printf("createTaskForWorktree: update worktree context: %v", err)
			return nil
		}
		linkID := msg.TaskID + "::" + msg.Worktree.Path + "::primary"
		if err := m.cmdBus.Send(context.Background(), &commands.LinkTaskToWorktree{
			ID:           linkID,
			TaskID:       msg.TaskID,
			WorktreeID:   msg.Worktree.Path,
			RelationType: "primary",
		}); err != nil {
			log.Printf("createTaskForWorktree: link task to worktree: %v", err)
			return nil
		}
		return func() tea.Msg {
			return dagRefreshMsg{repoID: repoID}
		}
	}

	// No existing task — create one (manual worktree creation flow).
	taskID := uuid.NewString()
	if err := m.cmdBus.Send(context.Background(), &commands.CreateTask{
		ID:     taskID,
		RepoID: repoID,
		Title:  msg.TaskTitle,
		State:  "active",
	}); err != nil {
		log.Printf("createTaskForWorktree: create task %q: %v", msg.TaskTitle, err)
		return nil
	}
	if err := m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{
		Record: models.WorktreeContextRecord{
			WorktreeID:    msg.Worktree.Path,
			RepoID:        repoID,
			PrimaryTaskID: &taskID,
			TaskMode:      "single",
			TaskName:      msg.TaskTitle,
			LastActiveAt:  now,
		},
	}); err != nil {
		log.Printf("createTaskForWorktree: update worktree context: %v", err)
		return nil
	}
	linkID := taskID + "::" + msg.Worktree.Path + "::primary"
	if err := m.cmdBus.Send(context.Background(), &commands.LinkTaskToWorktree{
		ID:           linkID,
		TaskID:       taskID,
		WorktreeID:   msg.Worktree.Path,
		RelationType: "primary",
	}); err != nil {
		log.Printf("createTaskForWorktree: link task to worktree: %v", err)
		return nil
	}
	return func() tea.Msg {
		return dagRefreshMsg{repoID: repoID}
	}
}

func (m *model) launchAgent(msg agents.LaunchAgentMsg) tea.Cmd {
	worktreeID := msg.WorktreeID
	provider := msg.Provider
	if provider == "" {
		provider = agents.DefaultProvider()
	}

	if m.agentRegistry != nil && m.agentRegistry.HasRunning(worktreeID, provider) {
		pageCmd := m.switchToWorktreePage(worktreeID, "")
		var focusCmd tea.Cmd
		if m.activePage != nil {
			focusCmd = m.activePage.focusAgentShell(worktreeID)
		}
		m.updateSizes(m.common.Width, m.common.Height)
		if pageCmd != nil && focusCmd != nil {
			return tea.Batch(pageCmd, focusCmd)
		}
		if pageCmd != nil {
			return pageCmd
		}
		return focusCmd
	}

	session := m.newAgentSession(worktreeID, provider)
	m.saveAgentSession(session)
	if m.agentRegistry != nil {
		m.agentRegistry.Register(session)
	}

	// Trellis-style: auto-inject per-worktree agent profile.
	_ = m.prepareAgentProfile(session)

	pageCmd := m.switchToWorktreePage(worktreeID, "")
	if m.common != nil && m.common.Cfg.Agent.ExternalTerminal {
		launchCmd := m.launchExternalAgent(session)
		m.updateSizes(m.common.Width, m.common.Height)
		if pageCmd != nil && launchCmd != nil {
			return tea.Batch(pageCmd, launchCmd)
		}
		if pageCmd != nil {
			return pageCmd
		}
		return launchCmd
	}
	var launchCmd tea.Cmd
	if m.activePage != nil {
		launchCmd = m.activePage.openAgentShell(worktreeID, provider, session.ID)
	}
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && launchCmd != nil {
		return tea.Batch(pageCmd, launchCmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return launchCmd
}

func (m *model) launchExternalAgent(session *agents.Session) tea.Cmd {
	if session == nil {
		return nil
	}
	title := "Focus: " + session.DisplayTitle
	if session.DisplayTitle == "" {
		if session.TaskID != "" {
			title = "Focus: " + session.TaskID
		} else {
			title = fmt.Sprintf("Focus:%s:%s", session.PlanID, session.ID)
		}
	}
	envVars := []string{
		agents.SessionIDEnvVar + "=" + session.ID,
		agents.LegacySessionIDEnvVar + "=" + session.ID,
	}
	if session.TaskID != "" {
		envVars = append(envVars, agents.TaskIDEnvVar+"="+session.TaskID)
	}
	if session.PlanID != "" {
		envVars = append(envVars, agents.PlanIDEnvVar+"="+session.PlanID)
	}
	if url := strings.TrimSpace(m.mcpServer.HTTPURL()); url != "" {
		envVars = append(envVars, agents.MCPURLEnvVar+"="+url)
	}
	session.State = agents.SessionWaiting
	session.EnvSnapshot = strings.Join(envVars, " ")
	m.saveAgentSession(session)

	emulator := strings.TrimSpace(m.common.Cfg.Agent.TerminalEmulator)
	if emulator != "" {
		if _, err := exec.LookPath(emulator); err != nil {
			if detected := agents.DetectTerminalEmulator(); detected != "" {
				emulator = detected
			}
		}
	} else {
		emulator = agents.DetectTerminalEmulator()
	}

	bin, args := agents.ProviderCommand(session.Provider)
	if session.ProviderConfigID != "" {
		if ccsBin, ccsArgs := ccswitch.LaunchCommand(session.Provider, session.ProviderConfigID); ccsBin != "" {
			bin, args = ccsBin, ccsArgs
		}
	}
	return agents.LaunchExternalCommand(agents.ExternalLaunchRequest{
		SessionID:        session.ID,
		Title:            title,
		WorktreeID:       session.WorktreeID,
		Provider:         session.Provider,
		TerminalEmulator: emulator,
		EnvVars:          envVars,
		ExtraArgs:        session.ExtraArgs,
		OverrideBinary:   bin,
		OverrideArgs:     args,
	})
}

func (m *model) handleExternalLaunchResult(msg agents.ExternalLaunchResultMsg) {
	record := m.agentSessionRecord(msg.SessionID)
	if record.ID == "" || record.WorktreeID == "" {
		return
	}
	now := time.Now()
	if msg.Err != nil {
		record.State = string(agents.SessionFailed)
		record.StopReason = msg.Err.Error()
		record.EndedAt = &now
		record.PID = 0
		record.UpdatedAt = now
		record.LastActivityAt = &now
		m.saveAgentSessionRecord(record)
		return
	}
	record.PID = msg.PID
	record.State = string(agents.SessionRunning)
	record.StopReason = ""
	record.EndedAt = nil
	record.UpdatedAt = now
	record.LastActivityAt = &now
	record.LastHeartbeat = &now
	m.saveAgentSessionRecord(record)
}

// launchExternalShell opens the user's preferred terminal emulator with an
// interactive shell at the given working directory. This is used when the
// embedded shell pane is too small for TUI test output.
func (m *model) launchExternalShell(cwd string) tea.Cmd {
	if cwd == "" {
		cwd = m.currentWorktreeID()
	}
	emulator := strings.TrimSpace(m.common.Cfg.Agent.TerminalEmulator)
	if emulator != "" {
		if _, err := exec.LookPath(emulator); err != nil {
			if detected := agents.DetectTerminalEmulator(); detected != "" {
				emulator = detected
			}
		}
	} else {
		emulator = agents.DetectTerminalEmulator()
	}
	title := m.resolveWorktreeDisplayTitle(cwd, "")
	if title == "" {
		title = filepath.Base(cwd)
	}
	return agents.LaunchExternalShell(cwd, title, emulator, 1.2)
}

func (m *model) killAgent(msg agentsplugin.KillSessionMsg) tea.Cmd {
	if m.agentRegistry != nil {
		m.agentRegistry.Remove(msg.SessionID)
	}
	if msg.SessionID != "" {
		now := time.Now()
		record := m.agentSessionRecord(msg.SessionID)
		record.PID = 0
		record.State = string(agents.SessionExited)
		record.EndedAt = &now
		record.UpdatedAt = now
		m.saveAgentSessionRecord(record)
	}
	if msg.PID > 0 {
		_ = exec.Command("kill", "-TERM", strconv.Itoa(msg.PID)).Run()
	}
	return nil
}

func (m *model) handleAgentExited(msg agents.AgentExitedMsg) {
	now := time.Now()
	for _, record := range m.listAgentSessionRecords(msg.WorktreeID) {
		if record.Provider != string(msg.Provider) || record.State != string(agents.SessionRunning) {
			continue
		}
		record.PID = 0
		record.State = string(agents.SessionExited)
		record.EndedAt = &now
		record.UpdatedAt = now
		m.saveAgentSessionRecord(record)
		m.backflowAgentSession(record)
		m.archiveAgentSession(record)
		if m.agentRegistry != nil {
			m.agentRegistry.Remove(record.ID)
		}
	}
}

// archiveAgentSession builds a session digest from the external agent
// transcript and git diff, then persists it into the store.
// TODO: re-enable when sessiondigest package is implemented.
func (m *model) archiveAgentSession(record models.AgentSessionRecord) {
	if m.common == nil || m.common.Store == nil {
		return
	}
	// Session digest archiving is currently disabled.
}

func (m *model) focusAgentSession(msg agentsplugin.FocusAgentSessionMsg) tea.Cmd {
	worktreeID := msg.WorktreeID
	pageCmd := m.switchToWorktreePage(worktreeID, "")
	var focusCmd tea.Cmd
	if m.activePage != nil {
		focusCmd = m.activePage.focusAgentShell(worktreeID)
	}
	m.updateSizes(m.common.Width, m.common.Height)
	if pageCmd != nil && focusCmd != nil {
		return tea.Batch(pageCmd, focusCmd)
	}
	if pageCmd != nil {
		return pageCmd
	}
	return focusCmd
}

func (m *model) currentCWD() string {
	return m.activePage.currentCWD()
}

func (m *model) currentRepoID() string {
	return m.activePage.currentRepoID()
}

func (m *model) currentWorktreeID() string {
	return m.activePage.currentWorktreeID()
}

func (m *model) currentBranchSnapshot() string {
	return m.activePage.currentBranchSnapshot()
}

func (m *model) openEditorPane(msg editorplugin.OpenEditorMsg) tea.Cmd {
	cmd := m.activePage.openEditorPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m model) findEditorPaneByPath(filePath string) models.PaneID {
	return m.activePage.findEditorPaneByPath(filePath)
}

func (m model) lastEditorPane() models.PaneID {
	return m.activePage.lastEditorPane()
}

func (m model) splitFocused(direction layout.SplitDirection) (tea.Model, tea.Cmd) {
	cmd := m.activePage.splitFocused(direction)
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, cmd
}

func (m model) adjustFocusedSplit(direction layout.FocusDirection) (tea.Model, tea.Cmd) {
	if m.activePage.adjustFocusedSplit(direction) {
		m.updateSizes(m.common.Width, m.common.Height)
		m.invalidateView()
	}
	return m, nil
}

func (m model) closeFocusedPane() (tea.Model, tea.Cmd) {
	m.activePage.closeFocusedPane()
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m *model) focusCycle(delta int) {
	m.activePage.focusCycle(delta)
}

func (m model) toggleZoom() (tea.Model, tea.Cmd) {
	m.activePage.toggleZoom()
	m.updateSizes(m.common.Width, m.common.Height)
	m.invalidateView()
	return m, nil
}

func (m *model) restoreZoom() {
	m.activePage.restoreZoom()
}

func (m *model) removePaneOrder(id models.PaneID) {
	m.activePage.removePaneOrder(id)
}

func (m model) activeOverlayPane() models.PaneID {
	return m.activePage.activeOverlayPane()
}

func (m model) isOverlayPane(id models.PaneID) bool {
	return m.activePage.isOverlayPane(id)
}

func (m *model) paneAt(x, y int) models.PaneID {
	return m.activePage.paneAt(x, y)
}

func (m model) routeToPane(id models.PaneID, msg tea.Msg) tea.Cmd {
	return m.activePage.routeToPane(id, msg)
}

func (m *model) updateSizes(w, h int) {
	dims := layout.ComputeBanner(w, h)
	m.activePage.pane(paneHeader).SetSize(w, dims.HeaderH)
	footerHeight := 0
	if footerVisible(h) {
		footerHeight = 1
	}
	m.activePage.pane(paneFooter).SetSize(w, footerHeight)
	m.activePage.updateSizes(m.activePage.bodyBounds(w, h))
	if overlayID := m.activePage.activeOverlayPane(); overlayID != "" {
		var overlayW, overlayH int
		if m.activePage.isLargeOverlayPane(overlayID) {
			overlayW, overlayH = m.activePage.largeOverlayContentSize()
		} else {
			overlayW, overlayH = m.overlayContentSize()
		}
		if panel := m.activePage.pane(overlayID); panel != nil {
			panel.SetSize(overlayW, overlayH)
		}
	}
}

func gitRepoRoot(path string) (string, bool) {
	output, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", false
	}

	return root, true
}

func (m model) bodyBounds() models.PaneFrame {
	return m.activePage.bodyBounds(m.common.Width, m.common.Height)
}

func (m model) View() string {
	w := m.common.Width
	h := m.common.Height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	dims := layout.ComputeBanner(w, h)

	var result string
	if m.vc.gen == m.viewGen && m.vc.output != "" {
		result = m.vc.output
	} else {
		result = m.buildView(dims, w, h)
		m.vc.output = result
		m.vc.gen = m.viewGen
	}

	if m.mode == ModeShell && m.activePage.paneMeta[m.activePage.focused].Type == models.PaneTypeShell {
		if sh, ok := m.pane(m.activePage.focused).(*shell.Model); ok {
			if frame, exists := m.activePage.frames[m.activePage.focused]; exists {
				if cx, cy, vis := sh.CursorPos(); vis {
					termRow := dims.HeaderH + frame.Y + 1 + cy + 1
					termCol := frame.X + 2 + cx + 1
					result += fmt.Sprintf("\033[?25h\033[%d;%dH", termRow, termCol)
				}
			}
		}
	}

	return result
}

func (m model) buildView(dims layout.Dimensions, w, h int) string {
	hdr := m.pane(paneHeader).(*header.Model)

	var headerView string
	timerSummary := ""
	if tp, ok := m.activePage.pane(paneTodoOverlay).(*todo.Model); ok {
		timerSummary = tp.TimerSummary()
	}
	if dims.UseBanner {
		headerView = hdr.ViewBanner(w, timerSummary, "", "")
	} else {
		headerView = hdr.ViewCompact(w, dims.ShowQuote)
	}

	notifBar := m.renderNotificationBar(w)
	notifH := 0
	if notifBar != "" {
		notifH = 1
	}

	bodyHeight := h - dims.HeaderH - notifH - 1
	if footerVisible(h) {
		bodyHeight--
	}
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	bodyView := m.renderBody(w, bodyHeight)
	if windowTooSmall(w, h) {
		bodyView = renderWindowTooSmallBody(w, bodyHeight)
	}
	helpLine := m.renderHelpLine(w)

	sections := []string{headerView}
	if notifBar != "" {
		sections = append(sections, notifBar)
	}
	sections = append(sections, bodyView)
	if footerVisible(h) {
		sections = append(sections, m.pane(paneFooter).View())
	}
	sections = append(sections, helpLine)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m model) renderNotificationBar(w int) string {
	if len(m.notifications) == 0 || w <= 0 {
		return ""
	}
	n := m.notifications[0]
	icon := "ℹ"
	bgColor := lipgloss.Color("#3B82F6") // blue for info
	switch n.Severity {
	case "warning":
		icon = "⚠"
		bgColor = lipgloss.Color("#F59E0B") // amber
	case "critical":
		icon = "✖"
		bgColor = lipgloss.Color("#EF4444") // red
	}
	var countHint string
	if len(m.notifications) > 1 {
		countHint = fmt.Sprintf(" [%d more]", len(m.notifications)-1)
	}
	text := fmt.Sprintf("%s %s: %s%s [d]dismiss", icon, n.Title, n.Body, countHint)
	// Keep notification bar strictly single-line so body height math remains stable.
	text = truncateToWidth(text, max(0, w-2))
	style := lipgloss.NewStyle().
		Width(w).
		Background(bgColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)
	return style.Render(text)
}

func (m model) renderBody(w, h int) string {
	return m.activePage.renderBody(w, h, m.overlay)
}

func (m model) renderPaneTitle(id models.PaneID, contentWidth int) string {
	return m.activePage.renderPaneTitle(id, m.activePage.focused, m.mode, contentWidth)
}

func (m model) renderHelpLine(w int) string {
	if w <= 0 {
		return ""
	}
	helpStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	focusedType := m.activePage.paneMeta[m.activePage.focused].Type
	if overlayID := m.activeOverlayPane(); overlayID != "" {
		switch m.activePage.paneMeta[overlayID].Type {
		case paneTypeGitCommit:
			return renderCompactHelpLine(helpStyle, "[ctrl+s]commit  [ctrl+j]fallback  [esc]cancel", w)
		case paneTypeWorktreeCreate:
			return renderCompactHelpLine(helpStyle, "[tab]next  [enter]next/create  [ctrl+s]create  [esc]cancel", w)
		case paneTypeTaskEdit:
			return renderCompactHelpLine(helpStyle, "[tab]next  [enter]next/save  [ctrl+s]save  [esc]cancel", w)
		case paneTypePlanEdit:
			return renderCompactHelpLine(helpStyle, "[tab]switch  [enter]into body  [ctrl+s]save  [esc]cancel", w)
		}
	}
	if m.mode == ModeInput {
		return renderCompactHelpLine(helpStyle, "[enter]confirm  [esc]cancel", w)
	}
	if m.mode == ModeShell {
		text := "[esc]normal  [ctrl+t]shell  [alt+z]ext  [shell input active]"
		if w < simplifiedHelpMaxWidth {
			text = "[esc]normal  [ctrl+t]shell  [alt+z]ext"
		}
		return renderCompactHelpLine(helpStyle, text, w)
	}

	left := []helpAction{
		{Key: "tab", Label: "cycle focus"},
		{Key: "enter", Label: "activate"},
	}
	compact := []helpAction{
		{Key: "tab", Label: "cycle"},
		{Key: "enter", Label: "open"},
		{Key: "q", Label: "quit"},
	}
	switch m.activePage.focused {
	case paneDAG:
		left = []helpAction{
			{Key: "j/k", Label: "move"},
			{Key: "h/l", Label: "level"},
			{Key: "enter", Label: "open/create"},
			{Key: "c", Label: "new-wt"},
			{Key: "s", Label: "state"},
			{Key: "t", Label: "todo"},
			{Key: "n", Label: "new-task"},
			{Key: "r", Label: "research"},
			{Key: "a", Label: "arch"},
			{Key: "R", Label: "refresh"},
		}
		compact = []helpAction{
			{Key: "j/k", Label: "move"},
			{Key: "h/l", Label: "level"},
			{Key: "enter/c", Label: "open/wt"},
			{Key: "s", Label: "state"},
			{Key: "t", Label: "todo"},
		}
	case paneWorktree:
		left = []helpAction{
			{Key: "j/k", Label: "nav"},
			{Key: "enter", Label: "select"},
			{Key: "n", Label: "new"},
			{Key: "d", Label: "el"},
			{Key: "o", Label: "shell"},
			{Key: "tab", Label: "cycle focus"},
		}
		compact = []helpAction{
			{Key: "j/k", Label: "nav"},
			{Key: "enter", Label: "select"},
			{Key: "n", Label: "new"},
			{Key: "d", Label: "del"},
		}
	case paneWorktreeDetail:
		if dp, ok := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane); ok {
			leftText, compactText := dp.helpText()
			left = []helpAction{{Key: leftText, Label: ""}}
			compact = []helpAction{{Key: compactText, Label: ""}}
		} else {
			left = []helpAction{
				{Key: "1-3", Label: "tabs"},
				{Key: "j/k", Label: "nav"},
				{Key: "enter", Label: "open"},
				{Key: "tab", Label: "cycle focus"},
			}
			compact = []helpAction{
				{Key: "1-3", Label: "tabs"},
				{Key: "j/k", Label: "nav"},
				{Key: "enter", Label: "open"},
			}
		}
	case paneShell:
		switch m.activePage.paneMeta[m.activePage.focused].Status {
		case models.PaneStatusExited:
			left = []helpAction{
				{Key: "tab", Label: "cycle focus"},
				{Key: "enter", Label: "restart shell"},
			}
			compact = []helpAction{
				{Key: "enter", Label: "restart"},
				{Key: "q", Label: "quit"},
			}
		case models.PaneStatusStarting:
			left = []helpAction{{Key: "shell starting", Label: ""}}
			compact = []helpAction{{Key: "shell starting", Label: ""}}
		default:
			left = []helpAction{
				{Key: "tab", Label: "cycle focus"},
				{Key: "enter", Label: "shell"},
				{Key: "alt+z", Label: "external"},
				{Key: "ctrl+g", Label: "overview"},
			}
			compact = []helpAction{
				{Key: "enter", Label: "shell"},
				{Key: "alt+z", Label: "ext"},
				{Key: "ctrl+g", Label: "overview"},
			}
		}
	default:
		switch focusedType {
		case models.PaneTypeEditor:
			left = []helpAction{
				{Key: "ctrl+s", Label: "save"},
				{Key: "ctrl+f /", Label: "search"},
				{Key: ":", Label: "line"},
				{Key: "n/N", Label: "result"},
				{Key: "esc", Label: "close"},
			}
			compact = []helpAction{
				{Key: "ctrl+s", Label: "save"},
				{Key: "/", Label: "search"},
				{Key: ":", Label: "line"},
			}
		case models.PaneTypeDiffView:
			left = []helpAction{
				{Key: "enter", Label: "open file"},
				{Key: "s", Label: "toggle staged"},
				{Key: "[]/[]", Label: "files"},
				{Key: "j/k", Label: "scroll"},
				{Key: "wheel", Label: "scroll"},
				{Key: "q/esc", Label: "close review"},
			}
			compact = []helpAction{
				{Key: "enter", Label: "open"},
				{Key: "s", Label: "toggle"},
				{Key: "wheel", Label: "scroll"},
			}
		}
	}
	if w < simplifiedHelpMaxWidth {
		return renderCompactHelpLine(helpStyle, joinHelpActions(compact), w)
	}
	right := "[ctrl+r]refresh  [q]uit"
	return renderHelpBar(helpStyle, joinHelpActions(left), right, w)
}
func renderHelpBar(helpStyle lipgloss.Style, left, right string, w int) string {
	if w <= 0 {
		return ""
	}
	if w <= 2 {
		return helpStyle.Render(strings.Repeat(" ", w))
	}
	leftRendered := helpStyle.Render("  " + left)
	rightRendered := helpStyle.Render(right + "  ")
	if lipgloss.Width(leftRendered)+lipgloss.Width(rightRendered) >= w {
		compact := truncateToWidth(left+"  "+right, w-2)
		return helpStyle.Render("  " + compact)
	}
	gap := w - lipgloss.Width(leftRendered) - lipgloss.Width(rightRendered)
	if gap < 1 {
		gap = 1
	}
	return leftRendered + strings.Repeat(" ", gap) + rightRendered
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(text, width, "…")
}

type helpAction struct {
	Key   string
	Label string
}

func joinHelpActions(actions []helpAction) string {
	if len(actions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		key := strings.TrimSpace(a.Key)
		label := strings.TrimSpace(a.Label)
		if key == "" && label == "" {
			continue
		}
		if label == "" {
			if strings.Contains(key, "[") {
				parts = append(parts, key)
				continue
			}
			parts = append(parts, "["+key+"]")
			continue
		}
		parts = append(parts, "["+key+"]"+label)
	}
	return strings.Join(parts, "  ")
}

func renderCompactHelpLine(helpStyle lipgloss.Style, text string, w int) string {
	if w <= 0 {
		return ""
	}
	if w <= 2 {
		return helpStyle.Render(strings.Repeat(" ", w))
	}
	content := text
	if ansi.StringWidth(content) > w-2 {
		content = ansi.Truncate(content, w-2, "")
	}
	return helpStyle.Render("  " + content)
}

func footerVisible(h int) bool {
	return h >= hideFooterBelowHeight
}

func windowTooSmall(w, h int) bool {
	return w < tinyWindowMinWidth || h < tinyWindowMinHeight
}

func renderWindowTooSmallBody(w, h int) string {
	base := blankCanvas(w, h)
	if base == "" {
		return ""
	}
	message := "Window too small"
	if w < ansi.StringWidth(message) {
		message = ansi.Truncate(message, w, "")
	}
	x := (w - ansi.StringWidth(message)) / 2
	if x < 0 {
		x = 0
	}
	y := h / 2
	if y >= h {
		y = h - 1
	}
	if y < 0 {
		y = 0
	}
	return layout.OverlayOnBase(base, message, x, y)
}

func (m *model) closeShellPanes() {
	m.activePage.closeShellPanes()
	if m.mcpServer != nil {
		_ = m.mcpServer.Stop()
	}
	if m.orch != nil {
		m.orch.Stop()
	}
}

func (m *model) openDiffPane(msg gitplugin.OpenDiffMsg) tea.Cmd {
	cmd := m.activePage.openDiffPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openCommitPane(msg gitplugin.OpenCommitMsg) tea.Cmd {
	cmd := m.activePage.openCommitPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openCreateWorktreePane(msg gitplugin.OpenCreateWorktreeMsg) tea.Cmd {
	cmd := m.activePage.openCreateWorktreePane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openAgentSelectPane(worktreeID string) tea.Cmd {
	cmd := m.activePage.openAgentSelectPane(worktreeID)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openProviderSelectPane(worktreeID string, provider agents.Provider, resume bool) tea.Cmd {
	cmd := m.activePage.openProviderSelectPane(worktreeID, provider, resume)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openWorktreeHistoryPane() tea.Cmd {
	cmd := m.activePage.openWorktreeHistoryPane()
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openWorktreeDeleteConfirmPane(msg gitplugin.OpenWorktreeDeleteConfirmMsg) tea.Cmd {
	cmd := m.activePage.openWorktreeDeleteConfirmPane(msg)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openTaskEditPane(msg gitplugin.OpenTaskEditMsg) tea.Cmd {
	seed := taskEditorSeed{WorktreeID: msg.WorktreeID, State: "active", Priority: "medium", RelationType: msg.RelationType, ParentTaskID: msg.ParentTaskID}
	if m.common != nil && m.common.Store != nil {
		if wc, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID); wc != nil {
			if msg.RelationType != "queued" {
				seed.Title = wc.TaskName
			}
			targetTaskID := msg.TaskID
			if targetTaskID == "" && msg.RelationType != "queued" && wc.PrimaryTaskID != nil {
				targetTaskID = *wc.PrimaryTaskID
			}
			if targetTaskID != "" {
				if task, _ := m.common.Store.GetTaskContext(targetTaskID); task != nil {
					seed.TaskID = task.ID
					seed.Title = task.Title
					seed.Goal = task.Goal
					seed.NextStep = task.NextStep
					seed.State = task.State
					seed.Priority = task.Priority
					if brief, _ := m.common.Store.GetTaskBrief(task.ID); brief != nil {
						seed.WhyNow = brief.WhyNow
						seed.Success = brief.SuccessCriteria
						seed.OutOfScope = brief.OutOfScope
						seed.KnownRisks = brief.KnownRisks
					}
				}
			}
		}
	}
	cmd := m.activePage.openTaskEditPane(seed)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) openPlanEditPane(msg gitplugin.OpenPlanEditMsg) tea.Cmd {
	seed := planEditorSeed{TaskID: msg.TaskID, WorktreeID: msg.WorktreeID, Status: "draft"}
	if m.common != nil && m.common.Store != nil {
		var plan *models.TaskPlanRecord
		if msg.TaskID != "" {
			plans := m.listTaskPlans(msg.TaskID)
			if len(plans) > 0 {
				plan = &plans[0]
			} else if task, _ := m.common.Store.GetTaskContext(msg.TaskID); task != nil {
				seed.Title = task.Title
			}
		} else if wc, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID); wc != nil && wc.CurrentPlanID != nil && *wc.CurrentPlanID != "" {
			plan, _ = m.common.Store.GetTaskPlan(*wc.CurrentPlanID)
		}
		if plan != nil {
			seed.PlanID = plan.ID
			seed.Title = plan.Title
			seed.WhyNow = plan.WhyNow
			seed.Success = plan.Success
			seed.OutOfScope = plan.OutOfScope
			seed.KnownRisks = plan.KnownRisks
			seed.PlanBody = plan.PlanBody
			seed.Status = plan.Status
			seed.CurrentStep = plan.CurrentStep
			if seed.TaskID == "" {
				seed.TaskID = plan.TaskID
			}
			if steps := m.listPlanSteps(plan.ID); len(steps) > 0 {
				seed.PlanBody = renderPlanSteps(steps)
			}
		}
	}
	cmd := m.activePage.openPlanEditPane(seed)
	m.updateSizes(m.common.Width, m.common.Height)
	return cmd
}

func (m *model) saveTaskEditor(msg TaskEditorSavedMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil || msg.WorktreeID == "" {
		return nil
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(msg.WorktreeID)
	}
	if repoID == "" {
		repoID = msg.WorktreeID
	}
	worktreeContext, _ := m.common.Store.GetWorktreeContext(msg.WorktreeID)
	relationType := msg.RelationType
	if relationType == "" {
		relationType = "primary"
	}
	taskID := msg.TaskID
	if taskID == "" {
		taskID = uuid.NewString()
	}
	if relationType == "primary" && taskID == "" && worktreeContext != nil && worktreeContext.PrimaryTaskID != nil && *worktreeContext.PrimaryTaskID != "" {
		taskID = *worktreeContext.PrimaryTaskID
	}
	now := time.Now()
	_ = m.cmdBus.Send(context.Background(), &commands.UpdateTask{
		Record: models.TaskContextRecord{
			ID:                  taskID,
			RepoID:              repoID,
			Title:               msg.Title,
			Goal:                msg.Goal,
			NextStep:            msg.NextStep,
			State:               msg.State,
			Priority:            msg.Priority,
			ParentTaskID:        stringPtrOrNil(msg.ParentTaskID),
			PreferredWorktreeID: msg.WorktreeID,
		},
	})
	_ = m.cmdBus.Send(context.Background(), &commands.UpdateTaskBrief{
		TaskID:          taskID,
		WhyNow:          msg.WhyNow,
		SuccessCriteria: msg.Success,
		OutOfScope:      msg.OutOfScope,
		KnownRisks:      msg.KnownRisks,
	})
	if worktreeContext != nil && worktreeContext.CurrentPlanID != nil && *worktreeContext.CurrentPlanID != "" {
		if plan, _ := m.common.Store.GetTaskPlan(*worktreeContext.CurrentPlanID); plan != nil && plan.TaskID == "" {
			plan.TaskID = taskID
			_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlan{Record: *plan})
		}
	}
	branchSnapshot := ""
	if worktreeContext != nil {
		branchSnapshot = worktreeContext.BranchSnapshot
	}
	primaryTaskID := (*string)(nil)
	currentPlanID := (*string)(nil)
	taskMode := "single"
	taskName := msg.Title
	if worktreeContext != nil {
		primaryTaskID = worktreeContext.PrimaryTaskID
		currentPlanID = worktreeContext.CurrentPlanID
		taskMode = worktreeContext.TaskMode
		if worktreeContext.TaskName != "" {
			taskName = worktreeContext.TaskName
		}
	}
	if relationType == "primary" {
		primaryTaskID = &taskID
		taskMode = "single"
		taskName = msg.Title
	} else {
		if primaryTaskID != nil && *primaryTaskID != "" {
			taskMode = "mixed"
		}
	}
	wtRecord := models.WorktreeContextRecord{
		WorktreeID:     msg.WorktreeID,
		RepoID:         repoID,
		PrimaryTaskID:  primaryTaskID,
		CurrentPlanID:  currentPlanID,
		TaskMode:       taskMode,
		TaskName:       taskName,
		BranchSnapshot: branchSnapshot,
		LastActiveAt:   now,
		LastOpenedAt:   &now,
		LastAgentAt:    lastAgentAtForWorktree(m.listAgentSessionRecords(msg.WorktreeID)),
	}
	_ = m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{Record: wtRecord})
	_ = m.cmdBus.Send(context.Background(), &commands.LinkTaskToWorktree{
		ID:           taskID + "::" + msg.WorktreeID + "::" + relationType,
		TaskID:       taskID,
		WorktreeID:   msg.WorktreeID,
		RelationType: relationType,
	})
	return nil
}

func (m *model) savePlanEditor(msg PlanEditorSavedMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	planID := msg.PlanID
	if planID == "" {
		planID = uuid.NewString()
	}
	status := msg.Status
	if status == "" {
		status = "draft"
	}
	currentStep := msg.CurrentStep
	if currentStep == "" {
		currentStep = inferCurrentPlanStep(msg.PlanBody)
	}
	planRecord := models.TaskPlanRecord{
		ID:          planID,
		TaskID:      msg.TaskID,
		Title:       msg.Title,
		WhyNow:      msg.WhyNow,
		Success:     msg.Success,
		OutOfScope:  msg.OutOfScope,
		KnownRisks:  msg.KnownRisks,
		Status:      status,
		CurrentStep: currentStep,
		PlanBody:    msg.PlanBody,
	}
	_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlan{Record: planRecord})
	_ = m.cmdBus.Send(context.Background(), &commands.DeletePlanSteps{PlanID: planID})
	for i, step := range parsePlanSteps(msg.PlanBody) {
		_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlanStep{
			Record: models.PlanStepRecord{
				ID:         fmt.Sprintf("%s::step::%03d", planID, i),
				PlanID:     planID,
				OrderIndex: i,
				Title:      step.Title,
				State:      stepStateForIndex(i, step.Kind),
				Notes:      step.Kind,
			},
		})
	}
	if msg.WorktreeID != "" {
		m.attachPlanToWorktree(msg.WorktreeID, msg.Title, planID)
	}
	if msg.ExpandToTasks {
		m.expandPlanToTasks(planID, msg.WorktreeID)
	}
	return nil
}

func inferCurrentPlanStep(planBody string) string {
	for _, step := range parsePlanSteps(planBody) {
		if step.Title != "" && step.Kind != "blocked" && step.Kind != "follow-up" {
			return step.Title
		}
	}
	return ""
}

type parsedPlanStep struct {
	Title string
	Kind  string
}

func parsePlanSteps(planBody string) []parsedPlanStep {
	lines := strings.Split(planBody, "\n")
	steps := make([]parsedPlanStep, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•0123456789. "))
		if line == "" {
			continue
		}
		kind := "main"
		switch {
		case strings.HasPrefix(strings.ToLower(line), "blocked:"):
			kind = "blocked"
			line = strings.TrimSpace(line[len("blocked:"):])
		case strings.HasPrefix(strings.ToLower(line), "follow-up:"):
			kind = "follow-up"
			line = strings.TrimSpace(line[len("follow-up:"):])
		case strings.HasPrefix(strings.ToLower(line), "worktree:"):
			kind = "worktree-candidate"
			line = strings.TrimSpace(line[len("worktree:"):])
		case strings.HasPrefix(strings.ToLower(line), "validation:"):
			kind = "validation"
			line = strings.TrimSpace(line[len("validation:"):])
		case strings.HasPrefix(strings.ToLower(line), "risk:"):
			kind = "risk"
			line = strings.TrimSpace(line[len("risk:"):])
		}
		if line == "" {
			continue
		}
		steps = append(steps, parsedPlanStep{Title: line, Kind: kind})
	}
	return steps
}

func renderPlanSteps(steps []models.PlanStepRecord) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		line := strings.TrimSpace(step.Title)
		if line == "" {
			continue
		}
		switch step.Notes {
		case "blocked":
			line = "blocked: " + line
		case "follow-up":
			line = "follow-up: " + line
		case "worktree-candidate":
			line = "worktree: " + line
		case "validation":
			line = "validation: " + line
		case "risk":
			line = "risk: " + line
		}
		lines = append(lines, "- "+line)
	}
	return strings.Join(lines, "\n")
}

func stepStateForIndex(index int, kind string) string {
	if kind == "blocked" {
		return "blocked"
	}
	if index == 0 && kind != "follow-up" {
		return "in_progress"
	}
	return "pending"
}

func (m *model) attachPlanToWorktree(worktreeID, fallbackTitle, planID string) {
	if m.common == nil || m.common.Store == nil || worktreeID == "" || planID == "" {
		return
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if repoID == "" {
		repoID = worktreeID
	}
	now := time.Now()
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	record := models.WorktreeContextRecord{
		WorktreeID:    worktreeID,
		RepoID:        repoID,
		CurrentPlanID: &planID,
		TaskMode:      "single",
		TaskName:      fallbackTitle,
		LastActiveAt:  now,
		LastOpenedAt:  &now,
	}
	if wc != nil {
		record.PrimaryTaskID = wc.PrimaryTaskID
		record.TaskMode = wc.TaskMode
		record.TaskName = wc.TaskName
		record.BranchSnapshot = wc.BranchSnapshot
		record.LastAgentAt = wc.LastAgentAt
		if record.TaskName == "" {
			record.TaskName = fallbackTitle
		}
	}
	_ = m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{Record: record})
}

func (m *model) expandPlanToTasks(planID, worktreeID string) {
	if m.common == nil || m.common.Store == nil || planID == "" || worktreeID == "" {
		return
	}
	plan, err := m.common.Store.GetTaskPlan(planID)
	if err != nil || plan == nil {
		return
	}
	steps := m.listPlanSteps(planID)
	if len(steps) == 0 {
		return
	}
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if repoID == "" {
		repoID = worktreeID
	}
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	var primaryTaskID *string
	if wc != nil {
		primaryTaskID = wc.PrimaryTaskID
	}
	createdPrimary := primaryTaskID == nil || *primaryTaskID == ""
	for i, step := range steps {
		if step.ExpandedTaskID != "" {
			continue
		}
		taskID := uuid.NewString()
		state := "blocked"
		relationType := "queued"
		if i == 0 && createdPrimary {
			state = "active"
			relationType = "primary"
			primaryTaskID = &taskID
		} else if step.State == "in_progress" {
			state = "active"
		}
		if step.Notes == "worktree-candidate" {
			relationType = "secondary"
		}
		_ = m.cmdBus.Send(context.Background(), &commands.CreateTask{
			ID:                  taskID,
			RepoID:              repoID,
			Title:               step.Title,
			Goal:                plan.Title,
			NextStep:            step.Title,
			State:               state,
			Priority:            "medium",
			PreferredWorktreeID: worktreeID,
		})
		_ = m.cmdBus.Send(context.Background(), &commands.UpdateTaskBrief{
			TaskID:          taskID,
			WhyNow:          plan.WhyNow,
			SuccessCriteria: plan.Success,
			OutOfScope:      plan.OutOfScope,
			KnownRisks:      plan.KnownRisks,
		})
		step.ExpandedTaskID = taskID
		_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlanStep{Record: step})
		_ = m.cmdBus.Send(context.Background(), &commands.LinkTaskToWorktree{
			ID:           taskID + "::" + worktreeID + "::" + relationType,
			TaskID:       taskID,
			WorktreeID:   worktreeID,
			RelationType: relationType,
		})
		if plan.TaskID == "" && relationType == "primary" {
			plan.TaskID = taskID
		}
	}
	for i := 0; i+1 < len(steps); i++ {
		fromTaskID := strings.TrimSpace(steps[i].ExpandedTaskID)
		toTaskID := strings.TrimSpace(steps[i+1].ExpandedTaskID)
		if fromTaskID == "" || toTaskID == "" || fromTaskID == toTaskID {
			continue
		}
		_ = m.cmdBus.Send(context.Background(), &commands.AddTaskDependency{
			FromTaskID:     fromTaskID,
			ToTaskID:       toTaskID,
			DependencyType: "hard",
		})
	}
	if plan.TaskID != "" {
		_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlan{Record: *plan})
	}
	if wc != nil {
		wc.PrimaryTaskID = primaryTaskID
		if wc.TaskName == "" {
			wc.TaskName = plan.Title
		}
		if wc.TaskMode == "" {
			wc.TaskMode = "mixed"
		}
		_ = m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{Record: *wc})
	}
}

func (m *model) cycleTaskState(msg gitplugin.CycleTaskStateMsg) tea.Cmd {
	if m.common == nil || m.common.Store == nil || msg.TaskID == "" {
		return nil
	}
	task, err := m.common.Store.GetTaskContext(msg.TaskID)
	if err != nil || task == nil {
		return nil
	}
	prevState := task.State
	nextState := nextTaskState(task.State)
	if err := m.cmdBus.Send(context.Background(), &commands.UpdateTaskState{
		TaskID:   msg.TaskID,
		NewState: nextState,
	}); err != nil {
		return nil
	}
	if prevState != "done" && nextState == "done" {
		return m.launchDownstreamTasks(msg.TaskID)
	}
	return nil
}

func (m *model) launchDownstreamTasks(taskID string) tea.Cmd {
	// Phase 4: Orchestrator is now resident. Downstream readiness is
	// detected via the event bus and surfaced as TUI notifications.
	// Auto-launch is intentionally removed per ADR-0000 Human Sovereignty.
	return nil
}

type appOrchestratorStore struct {
	model *model
}

func (s appOrchestratorStore) GetDownstreamTasks(taskID string) ([]orchestrator.Task, error) {
	records, err := s.model.common.Store.ListDownstreamTaskContexts(taskID)
	if err != nil {
		return nil, err
	}
	out := make([]orchestrator.Task, 0, len(records))
	for _, record := range records {
		out = append(out, orchestrator.Task{
			ID:         record.ID,
			Name:       record.Title,
			WorktreeID: strings.TrimSpace(record.PreferredWorktreeID),
			Provider:   "",
		})
	}
	return out, nil
}

func (s appOrchestratorStore) AllPrerequisitesMet(taskID string) (bool, error) {
	return s.model.common.Store.AreTaskPrerequisitesMet(taskID)
}

func (s appOrchestratorStore) MarkSessionDisconnected(sessionID string, reason string) error {
	return s.model.common.Store.MarkAgentSessionDisconnected(sessionID, reason)
}

func (s appOrchestratorStore) ListActiveAgentSessions() ([]orchestrator.AgentSession, error) {
	records, err := s.model.common.Store.ListAgentSessions("")
	if err != nil {
		return nil, err
	}
	var out []orchestrator.AgentSession
	for _, r := range records {
		if r.State == "active" || r.State == "running" {
			out = append(out, orchestrator.AgentSession{
				ID:            r.ID,
				TaskID:        r.TaskID,
				WorktreeID:    r.WorktreeID,
				Provider:      r.Provider,
				LastHeartbeat: r.LastHeartbeat,
				State:         r.State,
			})
		}
	}
	return out, nil
}

func (s appOrchestratorStore) GetTaskContext(taskID string) (*orchestrator.TaskContext, error) {
	record, err := s.model.common.Store.GetTaskContext(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	return &orchestrator.TaskContext{
		ID:    record.ID,
		Title: record.Title,
		State: record.State,
	}, nil
}

func (s appOrchestratorStore) ListPlanSteps(planID string) ([]orchestrator.PlanStep, error) {
	records, err := s.model.common.Store.ListPlanSteps(planID)
	if err != nil {
		return nil, err
	}
	steps := make([]orchestrator.PlanStep, len(records))
	for i, r := range records {
		steps[i] = orchestrator.PlanStep{
			ID:    r.ID,
			Title: r.Title,
			State: r.State,
		}
	}
	return steps, nil
}

func (s appOrchestratorStore) UpdateTaskState(taskID string, newState string) error {
	return s.model.cmdBus.Send(context.Background(), &commands.UpdateTaskState{
		TaskID:   taskID,
		NewState: newState,
	})
}

type appOrchestratorLauncher struct {
	model *model
	cmds  []tea.Cmd
}

func (l *appOrchestratorLauncher) LaunchTask(task orchestrator.Task) error {
	if strings.TrimSpace(task.WorktreeID) == "" {
		return nil
	}
	provider := l.model.resolveOrchestratedProvider(task)
	cmd := l.model.launchAgent(agents.LaunchAgentMsg{
		WorktreeID: task.WorktreeID,
		Provider:   provider,
	})
	if cmd != nil {
		l.cmds = append(l.cmds, cmd)
	}
	return nil
}

type protocolOrchestratorLauncher struct {
	model *model
}

func (l *protocolOrchestratorLauncher) LaunchTask(task orchestrator.Task) error {
	if l == nil || l.model == nil {
		return nil
	}
	worktreeID := strings.TrimSpace(task.WorktreeID)
	if worktreeID == "" {
		return nil
	}
	provider := l.model.resolveOrchestratedProvider(task)
	session := l.model.newProtocolAgentSession(task.ID, worktreeID, provider)
	l.model.saveAgentSession(session)
	_ = l.model.prepareAgentProfile(session)
	if l.model.common != nil && l.model.common.Cfg.Agent.ExternalTerminal {
		cmd := l.model.launchExternalAgent(session)
		if cmd != nil {
			if result, ok := cmd().(agents.ExternalLaunchResultMsg); ok {
				l.model.handleExternalLaunchResult(result)
			}
		}
		return nil
	}
	now := time.Now()
	session.State = agents.SessionRunning
	session.LastActivityAt = &now
	session.UpdatedAt = now
	l.model.saveAgentSession(session)
	return nil
}

func (m *model) resolveOrchestratedProvider(task orchestrator.Task) agents.Provider {
	if provider := parseProvider(task.Provider); provider != "" {
		return provider
	}
	if m == nil || m.common == nil {
		return agents.DefaultProvider()
	}
	taskName := strings.ToLower(strings.TrimSpace(task.Name))
	switch {
	case containsAny(taskName, "research", "调研", "investigate", "analysis", "分析"):
		if provider := parseProvider(m.common.Cfg.Agent.ResearchProvider); provider != "" {
			return provider
		}
	case containsAny(taskName, "architecture", "架构", "design", "设计", "adr"):
		if provider := parseProvider(m.common.Cfg.Agent.ArchitectureProvider); provider != "" {
			return provider
		}
	default:
		if providers := parseProviderList(m.common.Cfg.Agent.CodingProvider); len(providers) > 0 {
			seed := strings.TrimSpace(task.ID)
			if seed == "" {
				seed = taskName
			}
			return pickProviderBySeed(seed, providers)
		}
		if provider := parseProvider(m.common.Cfg.Agent.CodingProvider); provider != "" {
			return provider
		}
	}
	return agents.DefaultProvider()
}

func parseProvider(value string) agents.Provider {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(agents.ProviderOpenCode):
		return agents.ProviderOpenCode
	case string(agents.ProviderClaude):
		return agents.ProviderClaude
	case string(agents.ProviderKimi):
		return agents.ProviderKimi
	case string(agents.ProviderCodex):
		return agents.ProviderCodex
	case string(agents.ProviderGeneric):
		return agents.ProviderGeneric
	default:
		return ""
	}
}

func containsAny(input string, keywords ...string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(input, keyword) {
			return true
		}
	}
	return false
}

func parseProviderList(value string) []agents.Provider {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\t' || r == '\n'
	})
	seen := make(map[agents.Provider]struct{})
	out := make([]agents.Provider, 0, len(parts))
	for _, part := range parts {
		provider := parseProvider(part)
		if provider == "" {
			continue
		}
		if _, exists := seen[provider]; exists {
			continue
		}
		seen[provider] = struct{}{}
		out = append(out, provider)
	}
	return out
}

func pickProviderBySeed(seed string, providers []agents.Provider) agents.Provider {
	if len(providers) == 0 {
		return agents.DefaultProvider()
	}
	if strings.TrimSpace(seed) == "" {
		return providers[0]
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(seed))
	index := int(hasher.Sum32() % uint32(len(providers)))
	return providers[index]
}

func nextTaskState(state string) string {
	switch state {
	case "active":
		return "paused"
	case "paused":
		return "blocked"
	case "blocked":
		return "done"
	case "done":
		return "active"
	default:
		return "active"
	}
}

func (m *model) removeWorktree(msg gitplugin.RequestRemoveWorktreeMsg) tea.Cmd {
	adapter := m.adapterManager.Git()
	repoPath := m.gitRepoPath()
	worktreePath := msg.Worktree.Path

	// Archive worktree metadata before removal.
	if m.common != nil && m.common.Store != nil && worktreePath != "" {
		wc, _ := m.common.Store.GetWorktreeContext(worktreePath)
		sessions, _ := m.common.Store.ListAgentSessions(worktreePath)

		var provider, summary string
		var createdAt time.Time
		if len(sessions) > 0 {
			provider = sessions[0].Provider
			summary = sessions[0].Summary
			createdAt = sessions[0].StartedAt
		}
		if wc != nil {
			if summary == "" && wc.TaskName != "" {
				summary = wc.TaskName
			}
			if createdAt.IsZero() {
				createdAt = wc.LastActiveAt
			}
		}

		duration := 0
		if !createdAt.IsZero() {
			duration = int(time.Since(createdAt).Minutes())
		}

		repoID := repoPath
		if wc != nil && wc.RepoID != "" {
			repoID = wc.RepoID
		}

		branch := msg.Worktree.Branch
		if branch == "" && wc != nil {
			branch = wc.BranchSnapshot
		}

		historyID := "hst-" + uuid.NewString()
		var taskID, planID *string
		if wc != nil && wc.PrimaryTaskID != nil && *wc.PrimaryTaskID != "" {
			taskID = wc.PrimaryTaskID
		}
		if wc != nil && wc.CurrentPlanID != nil && *wc.CurrentPlanID != "" {
			planID = wc.CurrentPlanID
		}

		_ = m.cmdBus.Send(context.Background(), &commands.RecordWorktreeHistory{
			ID:              historyID,
			RepoID:          repoID,
			Branch:          branch,
			Path:            worktreePath,
			CreatedAt:       createdAt,
			RemovedAt:       time.Now(),
			TaskID:          taskID,
			PlanID:          planID,
			Provider:        provider,
			Summary:         summary,
			DurationMinutes: duration,
		})

	}

	return func() tea.Msg {
		if adapter == nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "remove", Err: fmt.Errorf("git adapter unavailable")}
		}
		if err := adapter.RemoveWorktree(repoPath, worktreePath, gitmodel.RemoveWorktreeOptions{Force: msg.Force}); err != nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "remove", Err: err}
		}
		return gitplugin.WorktreeRemovedMsg{Path: worktreePath, Force: msg.Force}
	}
}

func (m *model) pruneWorktrees(msg gitplugin.RequestPruneWorktreesMsg) tea.Cmd {
	adapter := m.adapterManager.Git()
	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = m.gitRepoPath()
	}
	return func() tea.Msg {
		if adapter == nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "prune", Err: fmt.Errorf("git adapter unavailable")}
		}
		if err := adapter.PruneWorktrees(repoPath); err != nil {
			return gitplugin.WorktreeActionFailedMsg{Action: "prune", Err: err}
		}
		return gitplugin.WorktreesPrunedMsg{}
	}
}

func (m *model) closePanesForWorktree(worktreePath string) {
	m.activePage.closePanesForWorktree(worktreePath)
}

func (m *model) closePane(id models.PaneID) {
	m.activePage.closePane(id)
}

func (m model) gitRepoPath() string {
	return m.activePage.gitRepoPath()
}

func (m model) overlayContentSize() (int, int) {
	return m.activePage.overlayContentSize()
}

func (m model) renderOverlayPane(base string, id models.PaneID) string {
	return m.activePage.renderOverlayPane(base, id)
}

func blankCanvas(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", w)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatPaneTitle(meta models.PaneMeta, focused bool, shellActive bool, contentWidth int) string {
	name := strings.ToUpper(meta.Name)
	badge := paneStatusBadge(meta)
	focusBadge := ""
	if focused {
		if shellActive {
			focusBadge = "active"
		} else {
			focusBadge = "focus"
		}
	}

	var candidates []string
	if meta.Type == models.PaneTypeShell && meta.CWD != "" {
		cwd := shortenCWD(meta.CWD)
		candidates = append(candidates,
			composePaneTitle(name, cwd, badge, focusBadge),
			composePaneTitle(name, "", badge, focusBadge),
			composePaneTitle(name, "", "", focusBadge),
			name,
		)
	} else {
		candidates = append(candidates,
			composePaneTitle(name, "", badge, focusBadge),
			composePaneTitle(name, "", "", focusBadge),
			name,
		)
	}

	maxTitleWidth := contentWidth + 1
	if maxTitleWidth < 8 {
		maxTitleWidth = 8
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= maxTitleWidth {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func composePaneTitle(name, cwd, badge, focusBadge string) string {
	title := name
	if cwd != "" {
		title += " [" + cwd + "]"
	}
	if badge != "" {
		title += " [" + badge + "]"
	}
	if focusBadge != "" {
		title += " [" + focusBadge + "]"
	}
	return title
}

func paneStatusBadge(meta models.PaneMeta) string {
	switch meta.Type {
	case models.PaneTypeShell:
		switch meta.Status {
		case models.PaneStatusStarting, models.PaneStatusRunning, models.PaneStatusExited:
			return string(meta.Status)
		}
	case models.PaneTypeTodo, models.PaneTypePomodoro:
		if !meta.Closable {
			return "fixed"
		}
	case models.PaneTypeEditor:
		if strings.HasPrefix(meta.Name, "*") {
			return "modified"
		}
	}
	return ""
}

func shortenCWD(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return shortenPath(cwd, home)
}

func (m *model) worktreeList() []gitmodel.Worktree {
	adapter := m.adapterManager.Git()
	repoPath := m.gitRepoPath()
	if adapter == nil || repoPath == "" {
		return nil
	}
	wts, _ := adapter.ListWorktrees(repoPath)
	return wts
}

func (m *model) switchToAdjacentWorktreePage(delta int) tea.Cmd {
	wts := m.worktreeList()
	if len(wts) == 0 {
		return nil
	}
	idx := -1
	for i, wt := range wts {
		if wt.Path == m.currentWorktreePage {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	nextIdx := (idx + delta + len(wts)) % len(wts)
	return m.switchToWorktreePage(wts[nextIdx].Path, "")
}

func (m *model) syncWorktreeActivities() {
	if m.pages == nil {
		return
	}

	shouldDiscover := time.Since(m.lastAgentSync) >= time.Second
	if shouldDiscover {
		m.lastAgentSync = time.Now()
	}

	persisted := m.persistedAgentSessions()
	if shouldDiscover {
		persisted = m.reconcileDiscoveredAgentSessions(persisted, agents.DiscoverRunningAgents())
	}
	m.refreshResumeSummaryCache(persisted)

	runningSessions := make(map[string][]agents.Session)
	visibleSessions := m.sortedAgentSessions(persisted)
	if m.agentRegistry != nil {
		m.agentRegistry.Clear()
		for _, session := range visibleSessions {
			if session.State != agents.SessionRunning {
				continue
			}
			s := *session
			m.agentRegistry.Register(&s)
			runningSessions[session.WorktreeID] = append(runningSessions[session.WorktreeID], s)
		}
	}

	repoPath := m.gitRepoPath()
	if repoPath == "" {
		cwd, _ := os.Getwd()
		repoPath, _ = gitRepoRoot(cwd)
	}
	var worktreeList []gitmodel.Worktree
	if m.adapterManager != nil && m.adapterManager.Git() != nil {
		worktreeList, _ = m.adapterManager.Git().ListWorktrees(repoPath)
	}
	if len(worktreeList) == 0 && m.activePage != nil {
		if wp, ok := m.activePage.pane(paneWorktree).(*gitplugin.WorktreePane); ok {
			wp.SetAgentSessions(runningSessions)
			wp.SetResumeSummaries(m.resumeSummaryCache)
		}
		return
	}

	for _, wt := range worktreeList {
		worktreeID := wt.Path
		activity := gitmodel.WorktreeActivity{}
		if m.activePage != nil {
			for id, meta := range m.activePage.paneMeta {
				if meta.Type == models.PaneTypeEditor {
					activity.OpenEditors++
				}
				if meta.Type == models.PaneTypeShell {
					if sh, ok := m.activePage.pane(id).(*shell.Model); ok {
						if sh.SessionStatus() == models.PaneStatusReady || sh.SessionStatus() == models.PaneStatusStarting {
							activity.HasShell = true
						}
					}
				}
			}
		}
		if worktreeID == m.currentWorktreePage {
			activity.LastActive = "now"
		} else {
			activity.LastActive = ""
		}
		activity.AgentCount = len(runningSessions[worktreeID])
		if wp, ok := m.activePage.pane(paneWorktree).(*gitplugin.WorktreePane); ok {
			wp.SetActivity(worktreeID, activity)
		}
	}
	if wp, ok := m.activePage.pane(paneWorktree).(*gitplugin.WorktreePane); ok {
		wp.SetAgentSessions(runningSessions)
		wp.SetResumeSummaries(m.resumeSummaryCache)
	}
}

func (m *model) refreshResumeSummaryCache(agentSessions map[string]agents.Session) {
	if m.resumeSummaryCache == nil {
		m.resumeSummaryCache = make(map[string]gitmodel.WorktreeResumeSummary)
	}
	for key := range m.resumeSummaryCache {
		delete(m.resumeSummaryCache, key)
	}

	repoID := m.gitRepoPath()
	if repoID == "" {
		cwd, _ := os.Getwd()
		repoID, _ = gitRepoRoot(cwd)
	}
	worktreeContexts := m.listWorktreeContexts(repoID)
	taskContexts := m.listTaskContexts("")
	tasksByID := make(map[string]models.TaskContextRecord, len(taskContexts))
	for _, task := range taskContexts {
		tasksByID[task.ID] = task
	}

	for _, wc := range worktreeContexts {
		summary := gitmodel.WorktreeResumeSummary{
			TaskMode:        wc.TaskMode,
			TaskTitle:       wc.TaskName,
			LastActiveLabel: formatRelativeLabel("active", wc.LastActiveAt),
		}
		if wc.PrimaryTaskID != nil {
			summary.TaskID = *wc.PrimaryTaskID
			if task, ok := tasksByID[*wc.PrimaryTaskID]; ok {
				summary.TaskTitle = task.Title
				summary.TaskGoal = task.Goal
				summary.NextStep = task.NextStep
				summary.TaskState = task.State
				summary.TaskPriority = task.Priority
				if brief := m.getTaskBrief(task.ID); brief != nil {
					summary.TaskWhyNow = brief.WhyNow
					summary.TaskSuccess = brief.SuccessCriteria
					summary.TaskOutOfScope = brief.OutOfScope
					summary.TaskKnownRisks = brief.KnownRisks
				}
			}
		}
		if summary.TaskTitle == "" {
			summary.TaskTitle = wc.TaskName
		}
		if wc.LastAgentAt != nil {
			summary.LastAgentLabel = formatRelativeLabel("agent", *wc.LastAgentAt)
		}
		plan := m.resolveCurrentPlan(wc.CurrentPlanID, summary.TaskID)
		if plan != nil {
			summary.PlanTitle = plan.Title
			summary.PlanStatus = plan.Status
			summary.CurrentPlanStep = plan.CurrentStep
			summary.PlanBody = plan.PlanBody
			if summary.TaskWhyNow == "" {
				summary.TaskWhyNow = plan.WhyNow
			}
			if summary.TaskSuccess == "" {
				summary.TaskSuccess = plan.Success
			}
			if summary.TaskOutOfScope == "" {
				summary.TaskOutOfScope = plan.OutOfScope
			}
			if summary.TaskKnownRisks == "" {
				summary.TaskKnownRisks = plan.KnownRisks
			}
			steps := m.listPlanSteps(plan.ID)
			if len(steps) > 0 {
				summary.PlanSteps = formatPlanStepSummaries(steps)
				if summary.PlanBody == "" {
					summary.PlanBody = renderPlanSteps(steps)
				}
			}
		}
		if summary.TaskID != "" {
			handoffs := m.listSessionHandoffs(summary.TaskID)
			if len(handoffs) > 0 {
				h := handoffs[0]
				if summary.HandoffNote == "" {
					summary.HandoffNote = h.RemainingSummary
				}
				summary.HandoffEntrypoint = h.Entrypoint
				if summary.BlockerNote == "" {
					summary.BlockerNote = h.BlockerSummary
				}
			}
		}
		summary.AttentionAnchor, summary.RecentArtifact = m.deriveWorktreeSignals(wc.WorktreeID)
		summary.PinnedNote, summary.BlockerNote, summary.HandoffNote = m.deriveWorktreeNotes(summary.TaskID, wc.WorktreeID)
		summary.GitPressure = deriveGitPressure(wc.BranchSnapshot, wc.WorktreeID, m.pages[""].pane(paneWorktree))
		for _, link := range m.listWorktreeTaskLinks(wc.WorktreeID) {
			if link.RelationType != "queued" {
				continue
			}
			queuedTask, ok := tasksByID[link.TaskID]
			if !ok {
				continue
			}
			summary.QueuedTaskCount++
			if summary.QueuedTaskTitle == "" {
				summary.QueuedTaskTitle = queuedTask.Title
			}
		}
		if s := mostRelevantAgentSession(wc.WorktreeID, agentSessions); s != nil {
			summary.LastAgentSummary = formatAgentSummary(*s)
			if summary.LastAgentLabel == "" {
				summary.LastAgentLabel = formatRelativeLabel("agent", sessionRelevantTime(*s))
			}
		}
		summary.ResumeReason, summary.ResumeScore = computeResumeReason(summary)
		summary.LastResumeHint = buildResumeHint(summary)
		m.resumeSummaryCache[wc.WorktreeID] = summary
	}

	for worktreeID, page := range m.pages {
		if worktreeID == "" {
			continue
		}
		if _, ok := m.resumeSummaryCache[worktreeID]; ok {
			continue
		}
		summary := gitmodel.WorktreeResumeSummary{}
		if page == m.activePage {
			summary.LastActiveLabel = "active now"
			summary.ResumeScore += 100
		} else if page.snapshot != nil {
			summary.LastActiveLabel = "resume available"
			summary.ResumeScore += 40
		}
		if s := mostRelevantAgentSession(worktreeID, agentSessions); s != nil {
			summary.LastAgentSummary = formatAgentSummary(*s)
			summary.LastAgentLabel = formatRelativeLabel("agent", sessionRelevantTime(*s))
		}
		summary.AttentionAnchor, summary.RecentArtifact = m.deriveWorktreeSignals(worktreeID)
		plan := m.resolveCurrentPlan(nil, summary.TaskID)
		if plan != nil {
			summary.PlanTitle = plan.Title
			summary.PlanStatus = plan.Status
			summary.CurrentPlanStep = plan.CurrentStep
			summary.PlanBody = plan.PlanBody
			if summary.TaskWhyNow == "" {
				summary.TaskWhyNow = plan.WhyNow
			}
			if summary.TaskSuccess == "" {
				summary.TaskSuccess = plan.Success
			}
			if summary.TaskOutOfScope == "" {
				summary.TaskOutOfScope = plan.OutOfScope
			}
			if summary.TaskKnownRisks == "" {
				summary.TaskKnownRisks = plan.KnownRisks
			}
			steps := m.listPlanSteps(plan.ID)
			if len(steps) > 0 {
				summary.PlanSteps = formatPlanStepSummaries(steps)
				if summary.PlanBody == "" {
					summary.PlanBody = renderPlanSteps(steps)
				}
			}
		}
		if summary.TaskID != "" {
			handoffs := m.listSessionHandoffs(summary.TaskID)
			if len(handoffs) > 0 {
				h := handoffs[0]
				if summary.HandoffNote == "" {
					summary.HandoffNote = h.RemainingSummary
				}
				summary.HandoffEntrypoint = h.Entrypoint
				if summary.BlockerNote == "" {
					summary.BlockerNote = h.BlockerSummary
				}
			}
		}
		summary.PinnedNote, summary.BlockerNote, summary.HandoffNote = m.deriveWorktreeNotes(summary.TaskID, worktreeID)
		summary.GitPressure = deriveGitPressure("", worktreeID, m.pages[""].pane(paneWorktree))
		summary.ResumeReason, summary.ResumeScore = computeResumeReason(summary)
		summary.LastResumeHint = buildResumeHint(summary)
		m.resumeSummaryCache[worktreeID] = summary
	}
}

func (m *model) listTaskContexts(repoID string) []models.TaskContextRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListTaskContexts(repoID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listWorktreeContexts(repoID string) []models.WorktreeContextRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListWorktreeContexts(repoID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) getTaskBrief(taskID string) *models.TaskBriefRecord {
	if m.common == nil || m.common.Store == nil || taskID == "" {
		return nil
	}
	record, err := m.common.Store.GetTaskBrief(taskID)
	if err != nil {
		return nil
	}
	return record
}

func (m *model) listWorktreeTaskLinks(worktreeID string) []models.TaskWorktreeLinkRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListWorktreeTaskLinks(worktreeID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listTaskPlans(taskID string) []models.TaskPlanRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListTaskPlans(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) listPlanSteps(planID string) []models.PlanStepRecord {
	if m.common == nil || m.common.Store == nil || planID == "" {
		return nil
	}
	records, err := m.common.Store.ListPlanSteps(planID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) resolveCurrentPlan(currentPlanID *string, taskID string) *models.TaskPlanRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	if currentPlanID != nil && *currentPlanID != "" {
		record, err := m.common.Store.GetTaskPlan(*currentPlanID)
		if err == nil && record != nil {
			return record
		}
	}
	if taskID == "" {
		return nil
	}
	plans := m.listTaskPlans(taskID)
	if len(plans) == 0 {
		return nil
	}
	return &plans[0]
}

func formatPlanStepSummaries(steps []models.PlanStepRecord) []string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		state := strings.TrimSpace(step.State)
		if state == "" {
			state = "pending"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", state, step.Title))
	}
	return lines
}

func (m *model) listSessionHandoffs(taskID string) []models.SessionHandoffRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListSessionHandoffs(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) deriveWorktreeNotes(taskID string, worktreeID string) (string, string, string) {
	if m.common == nil || m.common.Store == nil {
		return "", "", ""
	}
	notes, err := m.common.Store.ListContextNotes(taskID, worktreeID)
	if err != nil {
		return "", "", ""
	}
	var pinned, blocker, handoff string
	for _, note := range notes {
		if blocker == "" && note.NoteType == "blocker" {
			blocker = note.Body
		}
		if handoff == "" && note.NoteType == "handoff" {
			handoff = note.Body
		}
		if pinned == "" && note.Pinned {
			pinned = note.Body
		}
		if pinned != "" && blocker != "" && handoff != "" {
			break
		}
	}
	return pinned, blocker, handoff
}

func deriveGitPressure(branchSnapshot, worktreeID string, pane models.Panel) string {
	wp, ok := pane.(*gitplugin.WorktreePane)
	if !ok || wp == nil {
		return ""
	}
	for _, item := range wp.OrderedContexts() {
		if item.Worktree.Path != worktreeID {
			continue
		}
		wt := item.Worktree
		if wt.DirtySummary.Conflicted > 0 {
			return "conflicted"
		}
		if wt.AheadBehind.Behind > 0 && wt.DirtySummary.IsDirty() {
			return "diverged+dirty"
		}
		if wt.AheadBehind.Behind > 0 {
			return "behind"
		}
		if wt.DirtySummary.IsDirty() {
			return "dirty"
		}
		if branchSnapshot != "" || wt.Branch != "" {
			return "clean"
		}
	}
	return ""
}

func (m *model) deriveWorktreeSignals(worktreeID string) (string, string) {
	if worktreeID == "" {
		return "", ""
	}
	if page, ok := m.pages[worktreeID]; ok && page != nil {
		if anchor, artifact := pageSignals(page); anchor != "" || artifact != "" {
			return anchor, artifact
		}
	}
	if page, ok := m.pages[worktreeID]; ok && page != nil && page.snapshot != nil {
		if anchor, artifact := snapshotSignals(page.snapshot); anchor != "" || artifact != "" {
			return anchor, artifact
		}
	}
	return "", ""
}

func pageSignals(p *page) (string, string) {
	if p == nil {
		return "", ""
	}
	if meta, ok := p.paneMeta[p.focused]; ok {
		switch meta.Type {
		case models.PaneTypeEditor:
			return "editing", meta.CWD
		case models.PaneTypeDiffView:
			return "reviewing diff", meta.CWD
		case models.PaneTypeGitStatus:
			return "checking git status", meta.CWD
		case models.PaneTypeShell:
			return "working in shell", meta.CWD
		}
	}
	for _, id := range p.paneOrder {
		meta := p.paneMeta[id]
		if meta.Type == models.PaneTypeEditor && meta.CWD != "" {
			return "editing", meta.CWD
		}
	}
	return "", ""
}

func snapshotSignals(snapshot *PageSnapshot) (string, string) {
	if snapshot == nil {
		return "", ""
	}
	if len(snapshot.OpenEditors) > 0 {
		return "resume available", snapshot.OpenEditors[0]
	}
	if snapshot.Focused != "" {
		return "resume available", string(snapshot.Focused)
	}
	return "", ""
}

func mostRelevantAgentSession(worktreeID string, sessions map[string]agents.Session) *agents.Session {
	var best *agents.Session
	for _, session := range sessions {
		if session.WorktreeID != worktreeID {
			continue
		}
		s := session
		if best == nil {
			best = &s
			continue
		}
		if best.State != s.State {
			if s.State == agents.SessionRunning {
				best = &s
			}
			continue
		}
		if sessionRelevantTime(s).After(sessionRelevantTime(*best)) {
			best = &s
		}
	}
	return best
}

func sessionRelevantTime(session agents.Session) time.Time {
	if session.UpdatedAt.IsZero() {
		return session.StartedAt
	}
	return session.UpdatedAt
}

func formatRelativeLabel(prefix string, ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	age := time.Since(ts)
	if age < time.Minute {
		return prefix + " now"
	}
	if age < time.Hour {
		return fmt.Sprintf("%s %dm ago", prefix, int(age.Minutes()))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%s %dh ago", prefix, int(age.Hours()))
	}
	return fmt.Sprintf("%s %dd ago", prefix, int(age.Hours()/24))
}

func formatAgentSummary(session agents.Session) string {
	label := string(session.Provider)
	if session.State == agents.SessionRunning {
		return label + " running"
	}
	if session.EndedAt != nil {
		return label + " finished"
	}
	return label + " recent"
}

func computeResumeReason(summary gitmodel.WorktreeResumeSummary) (string, int) {
	score := 0
	parts := []string{}
	if summary.TaskState == "active" {
		score += 50
		parts = append(parts, "active task")
	}
	if summary.NextStep != "" {
		score += 20
		parts = append(parts, "next step ready")
	}
	if summary.BlockerNote != "" {
		score += 25
		parts = append(parts, "blocker noted")
	}
	if summary.GitPressure == "conflicted" {
		score += 35
		parts = append(parts, "git conflicted")
	} else if summary.GitPressure == "diverged+dirty" {
		score += 20
		parts = append(parts, "git diverged")
	}
	if summary.LastAgentSummary != "" {
		score += 15
		parts = append(parts, summary.LastAgentSummary)
	}
	if summary.QueuedTaskCount > 0 {
		score += 10
		parts = append(parts, fmt.Sprintf("%d queued", summary.QueuedTaskCount))
	}
	if summary.LastActiveLabel != "" {
		score += 10
	}
	return strings.Join(parts, " · "), score
}

func buildResumeHint(summary gitmodel.WorktreeResumeSummary) string {
	if summary.NextStep != "" {
		return "Continue: " + summary.NextStep
	}
	if summary.BlockerNote != "" {
		return "Blocked: " + summary.BlockerNote
	}
	if summary.HandoffNote != "" {
		return "Handoff: " + summary.HandoffNote
	}
	if summary.AttentionAnchor != "" {
		return summary.AttentionAnchor
	}
	if summary.QueuedTaskTitle != "" {
		return "Queued: " + summary.QueuedTaskTitle
	}
	if summary.TaskGoal != "" {
		return "Goal: " + summary.TaskGoal
	}
	if summary.LastAgentSummary != "" {
		return summary.LastAgentSummary
	}
	return ""
}

func lastAgentAtForWorktree(records []models.AgentSessionRecord) *time.Time {
	var latest *time.Time
	for _, record := range records {
		candidate := record.UpdatedAt
		if record.LastActivityAt != nil {
			candidate = *record.LastActivityAt
		}
		if latest == nil || candidate.After(*latest) {
			t := candidate
			latest = &t
		}
	}
	return latest
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (m *model) touchActiveWorktreeContext() {
	for worktreeID, page := range m.pages {
		if worktreeID == "" || page != m.activePage {
			continue
		}
		m.touchWorktreeContext(worktreeID)
		return
	}
}

func (m *model) touchWorktreeContext(worktreeID string) {
	if worktreeID == "" || m.common == nil || m.common.Store == nil {
		return
	}
	repoID := ""
	branchSnapshot := ""
	taskName := filepath.Base(worktreeID)
	if page, ok := m.pages[worktreeID]; ok && page != nil {
		repoID = page.currentRepoID()
		branchSnapshot = page.currentBranchSnapshot()
		if branchSnapshot != "" {
			taskName = branchSnapshot
		}
	}
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	if branchSnapshot == "" && m.adapterManager != nil && m.adapterManager.Git() != nil {
		if status, err := m.adapterManager.Git().GetWorktreeStatus(worktreeID); err == nil && status != nil {
			branchSnapshot = status.Branch
			if taskName == filepath.Base(worktreeID) && status.Branch != "" {
				taskName = status.Branch
			}
		}
	}
	existing, _ := m.common.Store.GetWorktreeContext(worktreeID)
	record := models.WorktreeContextRecord{
		WorktreeID:     worktreeID,
		RepoID:         repoID,
		TaskMode:       "single",
		TaskName:       taskName,
		BranchSnapshot: branchSnapshot,
		LastActiveAt:   time.Now(),
	}
	if existing != nil {
		record.PrimaryTaskID = existing.PrimaryTaskID
		record.TaskMode = existing.TaskMode
		if existing.TaskName != "" {
			record.TaskName = existing.TaskName
		}
		if existing.BranchSnapshot != "" {
			record.BranchSnapshot = existing.BranchSnapshot
		}
		if existing.LastOpenedAt != nil {
			record.LastOpenedAt = existing.LastOpenedAt
		}
		if existing.LastAgentAt != nil {
			record.LastAgentAt = existing.LastAgentAt
		}
	}
	_ = m.cmdBus.Send(context.Background(), &commands.UpdateWorktreeContext{Record: record})
}

func (m *model) persistedAgentSessions() map[string]agents.Session {
	records := m.listAgentSessionRecords("")
	sessions := make(map[string]agents.Session, len(records))
	for _, record := range records {
		sessions[record.ID] = agents.Session{
			ID:             record.ID,
			Provider:       agents.Provider(record.Provider),
			WorktreeID:     record.WorktreeID,
			RepoID:         record.RepoID,
			TaskID:         record.TaskID,
			PlanID:         record.PlanID,
			StepID:         record.StepID,
			BranchSnapshot: record.BranchSnapshot,
			PID:            record.PID,
			State:          agents.SessionState(record.State),
			LaunchSource:   record.LaunchSource,
			Summary:        record.Summary,
			EnvSnapshot:    record.EnvSnapshot,
			StartedAt:      record.StartedAt,
			EndedAt:        record.EndedAt,
			LastActivityAt: record.LastActivityAt,
			UpdatedAt:      record.UpdatedAt,
		}
	}
	return sessions
}

func (m *model) reconcileDiscoveredAgentSessions(existing map[string]agents.Session, discovered []agents.Session) map[string]agents.Session {
	now := time.Now()
	seen := make(map[string]struct{}, len(discovered))
	for _, session := range discovered {
		record, ok := existing[session.ID]
		if !ok {
			record = session
			record.StartedAt = now
		} else if record.StartedAt.IsZero() {
			record.StartedAt = now
		}
		record.Provider = session.Provider
		record.WorktreeID = session.WorktreeID
		record.PID = session.PID
		record.State = agents.SessionRunning
		record.LaunchSource = session.LaunchSource
		if record.LaunchSource == "" {
			record.LaunchSource = "discovered"
		}
		record.EndedAt = nil
		record.LastActivityAt = &now
		record.UpdatedAt = now
		if record.RepoID == "" {
			record.RepoID, _ = gitRepoRoot(session.WorktreeID)
		}
		if record.BranchSnapshot == "" && m.adapterManager != nil && m.adapterManager.Git() != nil {
			if status, err := m.adapterManager.Git().GetWorktreeStatus(session.WorktreeID); err == nil && status != nil {
				record.BranchSnapshot = status.Branch
			}
		}
		existing[record.ID] = record
		seen[record.ID] = struct{}{}
		m.saveAgentSession(&record)
		_ = m.cmdBus.Send(context.Background(), &commands.HeartbeatSession{
			SessionID: record.ID,
			At:        now,
			State:     string(agents.SessionRunning),
		})
	}

	for id, session := range existing {
		if session.State != agents.SessionRunning {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if session.PID == 0 && now.Sub(session.StartedAt) < 5*time.Second {
			continue
		}
		session.State = agents.SessionDisconnected
		session.PID = 0
		session.UpdatedAt = now
		if session.EndedAt == nil {
			endedAt := now
			session.EndedAt = &endedAt
		}
		existing[id] = session
		m.saveAgentSession(&session)
		_ = m.common.Store.MarkAgentSessionDisconnected(session.ID, "heartbeat timeout")
	}

	return existing
}

func (m *model) sortedAgentSessions(sessionMap map[string]agents.Session) []*agents.Session {
	list := make([]*agents.Session, 0, len(sessionMap))
	for _, session := range sessionMap {
		s := session
		list = append(list, &s)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].State != list[j].State {
			return list[i].State == agents.SessionRunning
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

// resolveWorktreeDisplayTitle returns a human-readable title for a worktree.
func (m *model) resolveWorktreeDisplayTitle(worktreeID, taskID string) string {
	if m.common == nil || m.common.Store == nil {
		return ""
	}
	if taskID != "" {
		if task, err := m.common.Store.GetTaskContext(taskID); err == nil && task != nil {
			return task.Title
		}
	}
	if wc, err := m.common.Store.GetWorktreeContext(worktreeID); err == nil && wc != nil && wc.TaskName != "" {
		return wc.TaskName
	}
	return ""
}

func (m *model) newAgentSession(worktreeID string, provider agents.Provider) *agents.Session {
	now := time.Now()
	repoID := ""
	if m.activePage != nil {
		repoID = m.activePage.currentRepoID()
	}
	if repoID == "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	branchSnapshot := ""
	if m.adapterManager != nil && m.adapterManager.Git() != nil {
		if status, err := m.adapterManager.Git().GetWorktreeStatus(worktreeID); err == nil && status != nil {
			branchSnapshot = status.Branch
		}
	}
	taskID, planID, stepID := m.currentExecutionSlice(worktreeID)
	displayTitle := m.resolveWorktreeDisplayTitle(worktreeID, taskID)
	return &agents.Session{
		ID:             agents.NewSessionID(),
		Provider:       provider,
		WorktreeID:     worktreeID,
		RepoID:         repoID,
		TaskID:         taskID,
		PlanID:         planID,
		StepID:         stepID,
		DisplayTitle:   displayTitle,
		BranchSnapshot: branchSnapshot,
		State:          agents.SessionWaiting,
		LaunchSource:   "focus",
		StartedAt:      now,
		LastActivityAt: &now,
		UpdatedAt:      now,
	}
}

// prepareAgentProfile generates the Trellis-style per-worktree agent context
// (AGENTS.md, handoff, journal) before the agent starts.  Errors are logged
// but not fatal — the agent can still launch without a profile.
func (m *model) prepareAgentProfile(session *agents.Session) error {
	if session == nil || session.WorktreeID == "" {
		return nil
	}

	pm := agents.NewProfileManager(session.WorktreeID)
	if err := pm.Prepare(); err != nil {
		return err
	}

	// Build spec load context from task/plan/step.
	ctx := agents.SpecLoadContext{}
	if m.common != nil && m.common.Store != nil {
		if session.TaskID != "" {
			if task, err := m.common.Store.GetTaskContext(session.TaskID); err == nil && task != nil {
				ctx.Task = task
			}
		}
		if session.PlanID != "" {
			if plan, err := m.common.Store.GetTaskPlan(session.PlanID); err == nil && plan != nil {
				ctx.Plan = plan
			}
			if steps, err := m.common.Store.ListPlanSteps(session.PlanID); err == nil {
				for _, s := range steps {
					if s.ID == session.StepID {
						ctx.Step = &s
						break
					}
				}
			}
		}
		// Load latest handoff for this task.
		if handoffs, err := m.common.Store.ListSessionHandoffs(session.TaskID); err == nil && len(handoffs) > 0 {
			ctx.Handoff = &handoffs[0]
		}
	}

	// Populate DAG dependency context.
	if session.TaskID != "" && m.common != nil && m.common.Store != nil {
		ctx.UpstreamTasks, _ = m.common.Store.ListUpstreamTaskContexts(session.TaskID)
		ctx.DownstreamTasks, _ = m.common.Store.ListDownstreamTaskContexts(session.TaskID)
	}

	// Populate git status.
	ctx.GitBranch = session.BranchSnapshot
	if m.adapterManager != nil && m.adapterManager.Git() != nil {
		if status, err := m.adapterManager.Git().GetWorktreeStatus(session.WorktreeID); err == nil && status != nil {
			ctx.GitBranch = status.Branch
			ctx.GitDirty = formatGitDirty(status)
		}
	}

	// Determine repo-level spec directory.
	repoSpecDir := ""
	if repoRoot, ok := gitRepoRoot(session.WorktreeID); ok {
		repoSpecDir = filepath.Join(repoRoot, ".focus", "spec")
	}

	loader := agents.NewSpecLoader(repoSpecDir, pm.Paths.SpecDir)
	spec, err := loader.Load(ctx)
	if err != nil {
		return err
	}

	// Write AGENTS.md.
	if err := pm.WriteRootFiles(spec); err != nil {
		return err
	}

	return nil
}

// formatGitDirty builds a compact human-readable summary of staged,
// unstaged, and untracked file counts from a git status.
func formatGitDirty(s *gitmodel.Status) string {
	staged := len(s.StagedFiles)
	unstaged := len(s.UnstagedFiles)
	untracked := len(s.UntrackedFiles)
	if staged == 0 && unstaged == 0 && untracked == 0 {
		return ""
	}
	var parts []string
	if staged > 0 {
		parts = append(parts, fmt.Sprintf("+%d staged", staged))
	}
	if unstaged > 0 {
		parts = append(parts, fmt.Sprintf("~%d unstaged", unstaged))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("?%d untracked", untracked))
	}
	return strings.Join(parts, ", ")
}

func (m *model) newProtocolAgentSession(taskID, worktreeID string, provider agents.Provider) *agents.Session {
	now := time.Now()
	repoID := ""
	planID := ""
	branchSnapshot := ""
	if m.common != nil && m.common.Store != nil {
		if taskID != "" {
			if task, err := m.common.Store.GetTaskContext(taskID); err == nil && task != nil {
				repoID = task.RepoID
			}
			if plans, err := m.common.Store.ListTaskPlans(taskID); err == nil && len(plans) > 0 {
				planID = plans[0].ID
			}
		}
		if repoID == "" && worktreeID != "" {
			if wc, err := m.common.Store.GetWorktreeContext(worktreeID); err == nil && wc != nil {
				repoID = wc.RepoID
				branchSnapshot = wc.BranchSnapshot
			}
		}
	}
	if repoID == "" && worktreeID != "" {
		repoID, _ = gitRepoRoot(worktreeID)
	}
	return &agents.Session{
		ID:             agents.NewSessionID(),
		Provider:       provider,
		WorktreeID:     worktreeID,
		RepoID:         repoID,
		TaskID:         taskID,
		PlanID:         planID,
		BranchSnapshot: branchSnapshot,
		State:          agents.SessionWaiting,
		LaunchSource:   "orchestrator",
		StartedAt:      now,
		LastActivityAt: &now,
		UpdatedAt:      now,
	}
}

func (m *model) saveAgentSession(session *agents.Session) {
	if session == nil {
		return
	}
	m.saveAgentSessionRecord(models.AgentSessionRecord{
		ID:             session.ID,
		Provider:       string(session.Provider),
		WorktreeID:     session.WorktreeID,
		RepoID:         session.RepoID,
		TaskID:         session.TaskID,
		PlanID:         session.PlanID,
		StepID:         session.StepID,
		BranchSnapshot: session.BranchSnapshot,
		PID:            session.PID,
		State:          string(session.State),
		LaunchSource:   session.LaunchSource,
		Summary:        session.Summary,
		EnvSnapshot:    session.EnvSnapshot,
		StartedAt:      session.StartedAt,
		EndedAt:        session.EndedAt,
		LastActivityAt: session.LastActivityAt,
		UpdatedAt:      session.UpdatedAt,
	})
}

func (m *model) currentExecutionSlice(worktreeID string) (string, string, string) {
	if m.common == nil || m.common.Store == nil || worktreeID == "" {
		return "", "", ""
	}
	wc, _ := m.common.Store.GetWorktreeContext(worktreeID)
	var taskID, planID string
	if wc != nil {
		if wc.PrimaryTaskID != nil {
			taskID = *wc.PrimaryTaskID
		}
		if wc.CurrentPlanID != nil {
			planID = *wc.CurrentPlanID
		}
	}
	if planID == "" && taskID != "" {
		plan := m.resolveCurrentPlan(nil, taskID)
		if plan != nil {
			planID = plan.ID
		}
	}
	if planID == "" {
		return taskID, "", ""
	}
	for _, step := range m.listPlanSteps(planID) {
		if step.State == "in_progress" {
			if taskID == "" {
				taskID = step.ExpandedTaskID
			}
			return taskID, planID, step.ID
		}
	}
	return taskID, planID, ""
}

func (m *model) backflowAgentSession(record models.AgentSessionRecord) {
	if m.common == nil || m.common.Store == nil || record.StepID == "" || record.PlanID == "" {
		return
	}
	steps := m.listPlanSteps(record.PlanID)
	if len(steps) == 0 {
		return
	}
	var currentTitle, nextTitle string
	for i := range steps {
		if steps[i].ID != record.StepID {
			continue
		}
		steps[i].State = "done"
		currentTitle = steps[i].Title
		if i+1 < len(steps) {
			steps[i+1].State = "in_progress"
			nextTitle = steps[i+1].Title
		}
		break
	}
	plan, _ := m.common.Store.GetTaskPlan(record.PlanID)
	if plan != nil {
		plan.CurrentStep = nextTitle
		if plan.CurrentStep == "" {
			plan.Status = "completed"
		}
	}
	var updatedTask *models.TaskContextRecord
	if record.TaskID != "" {
		task, _ := m.common.Store.GetTaskContext(record.TaskID)
		if task != nil {
			task.NextStep = nextTitle
			if nextTitle == "" {
				task.State = "done"
			}
			updatedTask = task
		}
	}
	for i := range steps {
		_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlanStep{Record: steps[i]})
	}
	if plan != nil {
		_ = m.cmdBus.Send(context.Background(), &commands.UpdatePlan{Record: *plan})
	}
	if updatedTask != nil {
		_ = m.cmdBus.Send(context.Background(), &commands.UpdateTask{Record: *updatedTask})
		planID := record.PlanID
		doneSummary := "Completed: " + currentTitle
		remainingSummary := "Plan complete"
		if nextTitle != "" {
			remainingSummary = "Next: " + nextTitle
		}
		decisionSummary := "Auto-advanced to next plan step"
		entrypoint := record.Summary
		if entrypoint == "" {
			entrypoint = "Continue from plan step: " + nextTitle
		}
		_ = m.cmdBus.Send(context.Background(), &commands.CreateSessionHandoff{
			ID:               record.ID + "::handoff",
			TaskID:           record.TaskID,
			PlanID:           &planID,
			SessionID:        record.ID,
			DoneSummary:      doneSummary,
			RemainingSummary: remainingSummary,
			DecisionSummary:  decisionSummary,
			Entrypoint:       entrypoint,
		})

		// Write per-worktree handoff file for the next agent session.
		pm := agents.NewProfileManager(record.WorktreeID)
		handoffMD := "# Session Handoff\n\n" +
			"## Completed\n" + doneSummary + "\n\n" +
			"## Key Decisions\n" + decisionSummary + "\n\n" +
			"## Remaining\n" + remainingSummary + "\n\n" +
			"## Entrypoint\n" + entrypoint + "\n"
		if err := pm.WriteHandoff(record.ID, handoffMD); err != nil {
			log.Printf("backflow: write handoff for session %s: %v", record.ID, err)
		}

		// Append workspace journal entry.
		taskTitle := currentTitle
		if record.TaskID != "" {
			if task, err := m.common.Store.GetTaskContext(record.TaskID); err == nil && task != nil {
				taskTitle = task.Title
			}
		}
		entry := agents.JournalEntry{
			SessionID:    record.ID,
			TaskTitle:    taskTitle,
			StepTitle:    currentTitle,
			Completed:    doneSummary,
			NextSteps:    remainingSummary,
			KeyDecisions: []string{decisionSummary},
			Timestamp:    time.Now(),
		}
		if err := pm.WriteJournal("", entry); err != nil {
			log.Printf("backflow: write journal for session %s: %v", record.ID, err)
		}
	}
}

func (m *model) saveAgentSessionRecord(record models.AgentSessionRecord) {
	if m.common == nil || m.common.Store == nil {
		return
	}
	_ = m.cmdBus.Send(context.Background(), &commands.CreateAgentSession{Record: record})
}

func (m *model) listAgentSessionRecords(worktreeID string) []models.AgentSessionRecord {
	if m.common == nil || m.common.Store == nil {
		return nil
	}
	records, err := m.common.Store.ListAgentSessions(worktreeID)
	if err != nil {
		return nil
	}
	return records
}

func (m *model) agentSessionRecord(sessionID string) models.AgentSessionRecord {
	for _, record := range m.listAgentSessionRecords("") {
		if record.ID == sessionID {
			return record
		}
	}
	return models.AgentSessionRecord{ID: sessionID}
}

func shortenPath(path, home string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	if home != "" {
		home = filepath.Clean(home)
		if cleaned == home {
			cleaned = "~"
		} else if strings.HasPrefix(cleaned, home+string(os.PathSeparator)) {
			cleaned = "~" + strings.TrimPrefix(cleaned, home)
		}
	}
	if cleaned == string(os.PathSeparator) || cleaned == "~" {
		return cleaned
	}
	if strings.HasPrefix(cleaned, "~"+string(os.PathSeparator)) {
		return shortenSegments("~", strings.TrimPrefix(cleaned, "~"+string(os.PathSeparator)))
	}
	if strings.HasPrefix(cleaned, string(os.PathSeparator)) {
		return shortenSegments(string(os.PathSeparator), strings.TrimPrefix(cleaned, string(os.PathSeparator)))
	}
	return shortenSegments("", cleaned)
}

func shortenSegments(prefix, rest string) string {
	parts := strings.Split(rest, string(os.PathSeparator))
	if len(parts) <= 2 {
		if prefix == "" {
			return rest
		}
		return prefix + string(os.PathSeparator) + rest
	}
	short := filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	if prefix == "" {
		return short
	}
	if prefix == "~" {
		return filepath.Join("~", short)
	}
	return prefix + short
}
