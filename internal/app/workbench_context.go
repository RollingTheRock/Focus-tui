package app

import (
	"fmt"
	"sort"
	"strings"

	gitmodel "focus/internal/git"
	"focus/internal/models"
	gitplugin "focus/internal/plugins/git"
)

type workbenchContextSource interface {
	OrderedContexts() []gitplugin.WorktreeContextView
	SelectedContext() (gitmodel.Worktree, gitmodel.WorktreeResumeSummary, gitmodel.WorktreeActivity, bool)
}

type workbenchQueueItem struct {
	Worktree           gitmodel.Worktree
	Summary            gitmodel.WorktreeResumeSummary
	Activity           gitmodel.WorktreeActivity
	GitSummary         string
	GitPressureSummary string
	PlanSummary        string
	QueuedSummary      string
	AgentSummary       string
	RuntimeSummary     string
	UpstreamSummary    string
	AttentionSummary   string
	NoteSummary        string
	HandoffSummary     string
}

type workbenchOverviewContext struct {
	Items    []workbenchQueueItem
	Selected *workbenchQueueItem
	Stats    overviewSummaryStats
	Detail   overviewDetailSelection
	DAG      overviewDAGProjection
}

type overviewDAGProjection struct {
	NodeCount       int
	EdgeCount       int
	RootCount       int
	FocusTaskID     string
	FocusTaskTitle  string
	LaneLines       []string
	EdgeLines       []string
	UpstreamLines   []string
	DownstreamLines []string
	ContextLines    []string
}

func (p overviewDAGProjection) SummaryLine() string {
	if p.NodeCount == 0 {
		return "dag: no task graph"
	}
	line := fmt.Sprintf("dag: %d tasks · %d edges · %d roots", p.NodeCount, p.EdgeCount, p.RootCount)
	if p.FocusTaskID != "" {
		line += " · focus " + p.FocusTaskID
	}
	return line
}

type dagNode struct {
	ID       string
	Title    string
	State    string
	Priority string
}

type dagEdge struct {
	From string
	To   string
	Type string
}

func buildWorkbenchOverviewContext(source workbenchContextSource, store models.Store, repoID string) workbenchOverviewContext {
	ctx := workbenchOverviewContext{}
	if source == nil {
		return ctx
	}
	for _, item := range source.OrderedContexts() {
		queueItem := buildWorkbenchQueueItem(item.Worktree, item.Summary, item.Activity)
		ctx.Items = append(ctx.Items, queueItem)
		ctx.Stats.Total++
		switch item.Summary.TaskState {
		case "active":
			ctx.Stats.Active++
		case "blocked":
			ctx.Stats.Blocked++
		}
		if item.Summary.QueuedTaskCount > 0 {
			ctx.Stats.Queued += item.Summary.QueuedTaskCount
		}
		if item.Worktree.DirtySummary.IsDirty() {
			ctx.Stats.Dirty++
		}
		if item.Activity.AgentCount > 0 {
			ctx.Stats.RunningAgent += item.Activity.AgentCount
		}
		if item.Worktree.DirtySummary.IsDirty() || item.Summary.ResumeScore >= 60 || item.Summary.LastAgentSummary != "" {
			ctx.Stats.Focus++
		}
		if ctx.Stats.Recent == "" {
			if item.Summary.TaskTitle != "" {
				ctx.Stats.Recent = item.Summary.TaskTitle
			} else {
				ctx.Stats.Recent = item.Worktree.DisplayName()
			}
		}
	}
	if wt, summary, activity, ok := source.SelectedContext(); ok {
		selected := buildWorkbenchQueueItem(wt, summary, activity)
		dag := buildOverviewDAGProjection(store, repoID, summary.TaskID)
		ctx.Selected = &selected
		ctx.Detail = overviewDetailSelection{
			TaskID:            summary.TaskID,
			Title:             selected.DisplayTitle(),
			Branch:            wt.Branch,
			WorktreePath:      wt.Path,
			State:             summary.TaskState,
			Priority:          summary.TaskPriority,
			ResumeReason:      summary.ResumeReason,
			ResumeHint:        summary.LastResumeHint,
			Goal:              summary.TaskGoal,
			WhyNow:            summary.TaskWhyNow,
			SuccessCriteria:   summary.TaskSuccess,
			OutOfScope:        summary.TaskOutOfScope,
			KnownRisks:        summary.TaskKnownRisks,
			NextStep:          summary.NextStep,
			AgentSummary:      selected.AgentSummary,
			QueuedSummary:     selected.QueuedSummary,
			GitSummary:        selected.GitSummary,
			GitPressure:       selected.GitPressureSummary,
			RuntimeSummary:    selected.RuntimeSummary,
			Upstream:          selected.UpstreamSummary,
			Attention:         selected.AttentionSummary,
			PinnedNote:        selected.NoteSummary,
			BlockerNote:       summary.BlockerNote,
			HandoffNote:       summary.HandoffNote,
			PlanTitle:         summary.PlanTitle,
			PlanStatus:        summary.PlanStatus,
			CurrentPlanStep:   summary.CurrentPlanStep,
			PlanBody:          summary.PlanBody,
			PlanSteps:         summary.PlanSteps,
			HandoffEntrypoint: summary.HandoffEntrypoint,
			RecentArtifact:    summary.RecentArtifact,
			DAGSummary:        dag.SummaryLine(),
			DAGLanes:          append([]string(nil), dag.LaneLines...),
			DAGEdges:          append([]string(nil), dag.EdgeLines...),
			UpstreamTasks:     append([]string(nil), dag.UpstreamLines...),
			DownstreamTasks:   append([]string(nil), dag.DownstreamLines...),
			SharedContext:     append([]string(nil), dag.ContextLines...),
			HasSelection:      true,
		}
		ctx.DAG = dag
		return ctx
	}
	ctx.DAG = buildOverviewDAGProjection(store, repoID, "")
	return ctx
}

func buildWorkbenchQueueItem(wt gitmodel.Worktree, summary gitmodel.WorktreeResumeSummary, activity gitmodel.WorktreeActivity) workbenchQueueItem {
	item := workbenchQueueItem{
		Worktree: wt,
		Summary:  summary,
		Activity: activity,
	}
	if wt.DirtySummary.IsDirty() {
		item.GitSummary = "git: " + gitplugin.FormatDirtySummaryForUI(wt.DirtySummary)
	}
	if summary.GitPressure != "" {
		item.GitPressureSummary = "git pressure: " + summary.GitPressure
	}
	if summary.LastAgentSummary != "" {
		item.AgentSummary = "agent: " + summary.LastAgentSummary
	}
	if summary.QueuedTaskTitle != "" {
		queued := "queued: " + summary.QueuedTaskTitle
		if summary.QueuedTaskCount > 1 {
			queued += fmt.Sprintf(" (+%d more)", summary.QueuedTaskCount-1)
		}
		item.QueuedSummary = queued
	}
	if summary.AttentionAnchor != "" || summary.RecentArtifact != "" {
		item.AttentionSummary = compactWorkbenchParts(summary.AttentionAnchor, summary.RecentArtifact)
	}
	if summary.PinnedNote != "" {
		item.NoteSummary = "note: " + summary.PinnedNote
	}
	if summary.BlockerNote != "" {
		if item.NoteSummary != "" {
			item.NoteSummary += " · "
		}
		item.NoteSummary += "blocker: " + summary.BlockerNote
	}
	if summary.HandoffNote != "" {
		item.HandoffSummary = "handoff: " + summary.HandoffNote
	}
	if summary.PlanTitle != "" {
		item.PlanSummary = compactWorkbenchParts(summary.PlanTitle, summary.PlanStatus, summary.CurrentPlanStep)
	}
	item.RuntimeSummary = fmt.Sprintf("runtime: shell=%t edits=%d agents=%d", activity.HasShell, activity.OpenEditors, activity.AgentCount)
	if wt.AheadBehind.Ahead > 0 || wt.AheadBehind.Behind > 0 || wt.Upstream != "" {
		item.UpstreamSummary = fmt.Sprintf("upstream: %s ↑%d ↓%d", wt.Upstream, wt.AheadBehind.Ahead, wt.AheadBehind.Behind)
	}
	return item
}

func compactWorkbenchParts(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " · ")
}

func (i workbenchQueueItem) DisplayTitle() string {
	if i.Summary.TaskTitle != "" {
		return i.Summary.TaskTitle
	}
	return i.Worktree.DisplayName()
}

func buildOverviewDAGProjection(store models.Store, repoID string, focusTaskID string) overviewDAGProjection {
	projection := overviewDAGProjection{}
	if store == nil {
		return projection
	}
	tasks, err := store.ListTaskContexts(repoID)
	if err != nil || len(tasks) == 0 {
		return projection
	}

	nodes := make(map[string]dagNode, len(tasks))
	for _, task := range tasks {
		nodes[task.ID] = dagNode{
			ID:       task.ID,
			Title:    task.Title,
			State:    task.State,
			Priority: task.Priority,
		}
	}

	edges := make([]dagEdge, 0)
	adjacency := make(map[string][]dagEdge, len(tasks))
	indegree := make(map[string]int, len(tasks))
	edgeSeen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		downstream, err := store.ListDownstreamTaskContexts(task.ID)
		if err != nil {
			continue
		}
		for _, down := range downstream {
			if repoID != "" && strings.TrimSpace(down.RepoID) != "" && down.RepoID != repoID {
				continue
			}
			if _, ok := nodes[down.ID]; !ok {
				continue
			}
			key := task.ID + "->" + down.ID
			if _, dup := edgeSeen[key]; dup {
				continue
			}
			edgeSeen[key] = struct{}{}
			edge := dagEdge{From: task.ID, To: down.ID, Type: "hard"}
			edges = append(edges, edge)
			adjacency[task.ID] = append(adjacency[task.ID], edge)
			indegree[down.ID]++
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	projection.NodeCount = len(nodes)
	projection.EdgeCount = len(edges)

	levels := computeDAGLevels(nodes, adjacency, indegree)
	maxLevel := 0
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	layerIDs := make(map[int][]string, maxLevel+1)
	for id, level := range levels {
		layerIDs[level] = append(layerIDs[level], id)
	}
	for _, value := range indegree {
		if value == 0 {
			projection.RootCount++
		}
	}
	for level := 0; level <= maxLevel; level++ {
		ids := layerIDs[level]
		if len(ids) == 0 {
			continue
		}
		sort.Slice(ids, func(i, j int) bool {
			left := nodes[ids[i]]
			right := nodes[ids[j]]
			if left.Title == right.Title {
				return left.ID < right.ID
			}
			return left.Title < right.Title
		})
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, formatDAGNodeChip(nodes[id]))
		}
		projection.LaneLines = append(projection.LaneLines, fmt.Sprintf("L%02d %s", level, strings.Join(parts, "  ")))
	}
	for _, edge := range edges {
		fromNode := nodes[edge.From]
		toNode := nodes[edge.To]
		projection.EdgeLines = append(projection.EdgeLines, fmt.Sprintf("%s -> %s", compactTaskLabel(fromNode), compactTaskLabel(toNode)))
	}
	if len(projection.LaneLines) == 0 {
		projection.LaneLines = append(projection.LaneLines, "No dependency lanes yet.")
	}
	if len(projection.EdgeLines) == 0 {
		projection.EdgeLines = append(projection.EdgeLines, "No dependency edges yet.")
	}

	if focusTaskID == "" {
		return projection
	}
	focusNode, ok := nodes[focusTaskID]
	if !ok {
		return projection
	}
	projection.FocusTaskID = focusNode.ID
	projection.FocusTaskTitle = focusNode.Title

	upstream, _ := store.ListUpstreamTaskContexts(focusTaskID)
	for _, task := range upstream {
		projection.UpstreamLines = append(projection.UpstreamLines, formatTaskSummaryLine(task))
	}
	sort.Strings(projection.UpstreamLines)
	downstream, _ := store.ListDownstreamTaskContexts(focusTaskID)
	for _, task := range downstream {
		projection.DownstreamLines = append(projection.DownstreamLines, formatTaskSummaryLine(task))
	}
	sort.Strings(projection.DownstreamLines)

	projection.ContextLines = buildContextRelayLines(store, focusTaskID, upstream)
	return projection
}

func computeDAGLevels(nodes map[string]dagNode, adjacency map[string][]dagEdge, indegree map[string]int) map[string]int {
	queue := make([]string, 0, len(nodes))
	indegreeLeft := make(map[string]int, len(indegree))
	for id := range nodes {
		indegreeLeft[id] = indegree[id]
	}
	for id := range nodes {
		if indegreeLeft[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	levels := make(map[string]int, len(nodes))
	processed := make(map[string]bool, len(nodes))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed[current] = true
		for _, edge := range adjacency[current] {
			if levels[edge.To] < levels[current]+1 {
				levels[edge.To] = levels[current] + 1
			}
			indegreeLeft[edge.To]--
			if indegreeLeft[edge.To] == 0 {
				queue = append(queue, edge.To)
			}
		}
		sort.Strings(queue)
	}

	for id := range nodes {
		if processed[id] {
			continue
		}
		levels[id] = 0
	}
	return levels
}

func formatDAGNodeChip(node dagNode) string {
	label := compactTaskLabel(node)
	state := node.State
	if state == "" {
		state = "pending"
	}
	return fmt.Sprintf("[%s] %s", state, clipText(label, 34))
}

func compactTaskLabel(node dagNode) string {
	title := strings.TrimSpace(node.Title)
	if title == "" {
		title = node.ID
	}
	return fmt.Sprintf("%s (%s)", title, shortTaskID(node.ID))
}

func shortTaskID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 10 {
		return id
	}
	return id[:10]
}

func formatTaskSummaryLine(task models.TaskContextRecord) string {
	state := strings.TrimSpace(task.State)
	if state == "" {
		state = "pending"
	}
	priority := strings.TrimSpace(task.Priority)
	if priority == "" {
		priority = "medium"
	}
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = task.ID
	}
	line := fmt.Sprintf("%s · %s · %s", shortTaskID(task.ID), state, clipText(title, 48))
	if strings.TrimSpace(task.NextStep) != "" {
		line += " · next: " + clipText(task.NextStep, 44)
	}
	line += " · p:" + priority
	return line
}

func buildContextRelayLines(store models.Store, taskID string, upstream []models.TaskContextRecord) []string {
	lines := make([]string, 0, 12)
	outputs, _ := store.ListTaskOutputs(taskID)
	for i, output := range outputs {
		if i >= 3 {
			break
		}
		actor := strings.TrimSpace(output.Actor)
		if actor == "" {
			actor = "unknown"
		}
		lines = append(lines, fmt.Sprintf("output/%s: %s", actor, clipText(output.Content, 72)))
	}

	upstreamLimit := 0
	for _, up := range upstream {
		if upstreamLimit >= 3 {
			break
		}
		records, _ := store.ListTaskOutputs(up.ID)
		if len(records) == 0 {
			continue
		}
		upstreamLimit++
		title := strings.TrimSpace(up.Title)
		if title == "" {
			title = up.ID
		}
		lines = append(lines, fmt.Sprintf("upstream/%s: %s", clipText(title, 22), clipText(records[0].Content, 58)))
	}

	planID := resolveFirstPlanID(store, taskID)
	if planID != "" {
		facts, _ := store.ListKnowledgeFacts(planID)
		for i, fact := range facts {
			if i >= 3 {
				break
			}
			subject := strings.TrimSpace(fact.Subject)
			predicate := strings.TrimSpace(fact.Predicate)
			object := strings.TrimSpace(fact.Object)
			if subject == "" && predicate == "" && object == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("fact: %s %s %s", clipText(subject, 18), clipText(predicate, 18), clipText(object, 24)))
		}
	}

	messages, _ := store.ListAgentMessages("", "", 60)
	msgCount := 0
	for _, message := range messages {
		if msgCount >= 3 {
			break
		}
		if !isTaskRelatedMessage(taskID, message) {
			continue
		}
		msgCount++
		payload := clipText(message.Payload, 56)
		lines = append(lines, fmt.Sprintf("msg/%s: %s -> %s :: %s", message.MsgType, clipText(message.FromAgent, 12), clipText(message.ToAgent, 12), payload))
	}

	if len(lines) == 0 {
		lines = append(lines, "No shared context artifacts yet.")
	}
	return lines
}

func resolveFirstPlanID(store models.Store, taskID string) string {
	if store == nil || strings.TrimSpace(taskID) == "" {
		return ""
	}
	plans, err := store.ListTaskPlans(taskID)
	if err != nil || len(plans) == 0 {
		return ""
	}
	return plans[0].ID
}

func isTaskRelatedMessage(taskID string, message models.AgentMessageRecord) bool {
	if strings.TrimSpace(taskID) == "" {
		return false
	}
	if message.FromAgent == taskID || message.ToAgent == taskID {
		return true
	}
	return strings.Contains(message.Payload, taskID)
}

func clipText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxLen <= 0 {
		return ""
	}
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}
