package app

import (
	"fmt"
	"strings"

	gitmodel "focus/internal/git"
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
}

func buildWorkbenchOverviewContext(source workbenchContextSource) workbenchOverviewContext {
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
		ctx.Selected = &selected
		ctx.Detail = overviewDetailSelection{
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
			HasSelection:      true,
		}
	}
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
