package git

import (
	"errors"
	"fmt"
	"strings"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatusPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	status   *gitmodel.Status
	cursor   int
	loading  bool
	repoPath string

	width  int
	height int
	err    error

	confirmDiscard bool
}

type initialStatusMsg struct {
	event adapters.StatusEvent
}

type statusRow struct {
	icon string
	path string
}

func NewStatusPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter) *StatusPane {
	repoPath := meta.CWD
	if repoPath == "" {
		repoPath = "."
	}

	return &StatusPane{
		id:       id,
		meta:     meta,
		common:   common,
		adapter:  adapter,
		loading:  true,
		repoPath: repoPath,
	}
}

func (p *StatusPane) Init() tea.Cmd {
	p.loading = true
	return tea.Batch(p.loadStatusCmd(), p.watchStatusCmd())
}

func (p *StatusPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	if p.confirmDiscard {
		if _, ok := msg.(tea.KeyMsg); ok {
			return p.updateDiscardConfirm(msg)
		}
	}

	switch msg := msg.(type) {
	case initialStatusMsg:
		p.applyStatus(msg.event)
		if p.status != nil {
			return p, notifyStatsRefreshCmd()
		}
		return p, nil

	case adapters.StatusEvent:
		if msg.RepoPath != "" && msg.RepoPath != p.repoPath {
			return p, nil
		}

		p.applyStatus(msg)
		cmds := []tea.Cmd{p.watchStatusCmd()}
		if p.status != nil {
			cmds = append(cmds, notifyStatsRefreshCmd())
		}
		return p, tea.Batch(cmds...)

	case tea.KeyMsg:
		return p.updateKey(msg)

	case models.StatsRefreshMsg:
		return p, nil
	}

	return p, nil
}

func (p *StatusPane) View() string {
	width := p.width
	if width <= 0 {
		width = 40
	}

	if p.loading && p.status == nil && p.err == nil {
		return p.renderLoading(width)
	}

	if p.err != nil && p.status == nil {
		return errorStyle.MaxWidth(width).Render("Unable to load git status: " + p.err.Error())
	}

	if p.status == nil {
		return emptyStyle.Render("No git status available.")
	}

	lines := []string{p.renderHeader()}
	summary := p.renderSummary()
	if summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, "")

	rows := p.rows()
	if len(rows) == 0 {
		lines = append(lines, emptyStyle.Render("Working tree clean."))
	} else {
		lines = append(lines, p.renderSections(rows)...)
	}

	if p.confirmDiscard {
		lines = append(lines, "", errorStyle.Render("Discard selected changes for "+p.getSelectedPath()+"? [y/n]"))
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}

	for i := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
	}

	return strings.Join(lines, "\n")
}

func (p *StatusPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *StatusPane) updateKey(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	count := len(p.rows())
	if count == 0 {
		return p, nil
	}

	switch msg.String() {
	case "j", "down":
		if p.cursor < count-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		file, staged := p.getSelectedFile()
		if file == nil {
			return p, nil
		}
		return p, openDiffCmd(file.Path, staged)
	case " ":
		return p.handleStageToggle()
	case "ctrl+d":
		if p.getSelectedPath() == "" {
			return p, nil
		}
		p.confirmDiscard = true
		return p, nil
	}

	return p, nil
}

func openDiffCmd(path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		return OpenDiffMsg{FilePath: path, Staged: staged}
	}
}

func (p *StatusPane) handleStageToggle() (models.Panel, tea.Cmd) {
	if p.status == nil || p.adapter == nil {
		return p, nil
	}

	file, staged := p.getSelectedFile()
	if file == nil {
		return p, nil
	}

	if staged {
		if err := p.adapter.UnstageFile(p.repoPath, file.Path); err != nil {
			return p, nil
		}
	} else {
		if err := p.adapter.StageFile(p.repoPath, file.Path); err != nil {
			return p, nil
		}
	}

	return p, p.refreshCmd()
}

func (p *StatusPane) getSelectedFile() (*gitmodel.File, bool) {
	staged, unstaged, untracked, conflicted := p.sectionedRows()

	idx := p.cursor

	if idx < len(staged) {
		if idx < len(p.status.StagedFiles) {
			return &p.status.StagedFiles[idx], true
		}
	}
	idx -= len(staged)

	if idx < len(unstaged) {
		if idx < len(p.status.UnstagedFiles) {
			return &p.status.UnstagedFiles[idx], false
		}
	}
	idx -= len(unstaged)

	if idx < len(untracked) {
		if idx < len(p.status.UntrackedFiles) {
			return &p.status.UntrackedFiles[idx], false
		}
	}
	idx -= len(untracked)

	if idx < len(conflicted) {
		if idx < len(p.status.ConflictedFiles) {
			return &p.status.ConflictedFiles[idx], false
		}
	}

	return nil, false
}

func (p *StatusPane) getSelectedPath() string {
	file, _ := p.getSelectedFile()
	if file == nil {
		return ""
	}
	return file.Path
}

func (p *StatusPane) updateDiscardConfirm(msg tea.Msg) (models.Panel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	switch keyMsg.String() {
	case "y", "enter":
		path := p.getSelectedPath()
		p.confirmDiscard = false
		if path == "" || p.adapter == nil {
			return p, nil
		}
		if err := p.adapter.DiscardChanges(p.repoPath, path); err != nil {
			p.err = err
			return p, nil
		}
		p.err = nil
		return p, p.refreshCmd()
	default:
		p.confirmDiscard = false
		return p, nil
	}
}

func (p *StatusPane) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return adapters.StatusEvent{RepoPath: p.repoPath, Error: errors.New("git adapter is not configured")}
		}
		status, err := p.adapter.GetStatus(p.repoPath)
		return adapters.StatusEvent{RepoPath: p.repoPath, Status: status, Error: err}
	}
}

func (p *StatusPane) applyStatus(event adapters.StatusEvent) {
	p.loading = false
	if event.Error != nil {
		p.err = event.Error
		return
	}

	p.err = nil
	p.status = event.Status
	p.clampCursor()
}

func (p *StatusPane) loadStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return initialStatusMsg{event: adapters.StatusEvent{RepoPath: p.repoPath, Error: errors.New("git adapter is not configured")}}
		}

		status, err := p.adapter.GetStatus(p.repoPath)
		return initialStatusMsg{event: adapters.StatusEvent{RepoPath: p.repoPath, Status: status, Error: err}}
	}
}

func (p *StatusPane) watchStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return adapters.StatusEvent{RepoPath: p.repoPath, Error: errors.New("git adapter is not configured")}
		}

		ch, err := p.adapter.WatchStatus(p.repoPath)
		if err != nil {
			return adapters.StatusEvent{RepoPath: p.repoPath, Error: err}
		}

		event, ok := <-ch
		if !ok {
			return adapters.StatusEvent{RepoPath: p.repoPath, Error: errors.New("git status watcher closed")}
		}

		return event
	}
}

func notifyStatsRefreshCmd() tea.Cmd {
	return func() tea.Msg { return models.StatsRefreshMsg{} }
}

func (p *StatusPane) renderLoading(width int) string {
	lines := []string{
		renderLoadingLine(width),
		renderLoadingLine(width),
		"",
		renderLoadingLine(width),
		renderLoadingLine(width),
		renderLoadingLine(width),
	}
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	return strings.Join(lines, "\n")
}

func (p *StatusPane) renderHeader() string {
	branch := p.status.Branch
	if branch == "" {
		branch = "HEAD"
	}

	parts := []string{branchStyle.Render(branch)}
	if p.status.Upstream != "" {
		parts = append(parts, upstreamStyle.Render("-> "+p.status.Upstream))
	}
	if p.status.Ahead > 0 {
		parts = append(parts, aheadStyle.Render(fmt.Sprintf("↑%d", p.status.Ahead)))
	}
	if p.status.Behind > 0 {
		parts = append(parts, behindStyle.Render(fmt.Sprintf("↓%d", p.status.Behind)))
	}
	return strings.Join(parts, " ")
}

func (p *StatusPane) renderSummary() string {
	parts := make([]string, 0, 4)
	if count := len(p.status.StagedFiles); count > 0 {
		parts = append(parts, fmt.Sprintf("staged %d", count))
	}
	if count := len(p.status.UnstagedFiles); count > 0 {
		parts = append(parts, fmt.Sprintf("unstaged %d", count))
	}
	if count := len(p.status.UntrackedFiles); count > 0 {
		parts = append(parts, fmt.Sprintf("untracked %d", count))
	}
	if count := len(p.status.ConflictedFiles); count > 0 {
		parts = append(parts, fmt.Sprintf("conflicted %d", count))
	}
	if len(parts) == 0 {
		return emptyStyle.Render("No pending changes")
	}
	return upstreamStyle.Render(strings.Join(parts, " · "))
}

func (p *StatusPane) renderSections(rows []statusRow) []string {
	lines := make([]string, 0, len(rows)+4)
	index := 0

	appendSection := func(title string, sectionRows []statusRow) {
		if len(sectionRows) == 0 {
			return
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(title))
		for _, row := range sectionRows {
			lines = append(lines, p.renderRow(index, row))
			index++
		}
	}

	staged, unstaged, untracked, conflicted := p.sectionedRows()
	appendSection("Staged", staged)
	appendSection("Unstaged", unstaged)
	appendSection("Untracked", untracked)
	appendSection("Conflicted", conflicted)

	return lines
}

func (p *StatusPane) renderRow(index int, row statusRow) string {
	prefix := "  "
	if index == p.cursor {
		prefix = "> "
	}

	line := prefix + renderStatusIcon(row.icon) + " " + pathStyle.Render(row.path)
	if index == p.cursor {
		return selectedRowStyle.Render(line)
	}
	return line
}

func (p *StatusPane) rows() []statusRow {
	if p.status == nil {
		return nil
	}

	staged, unstaged, untracked, conflicted := p.sectionedRows()
	rows := make([]statusRow, 0, len(staged)+len(unstaged)+len(untracked)+len(conflicted))
	rows = append(rows, staged...)
	rows = append(rows, unstaged...)
	rows = append(rows, untracked...)
	rows = append(rows, conflicted...)
	return rows
}

func (p *StatusPane) sectionedRows() ([]statusRow, []statusRow, []statusRow, []statusRow) {
	if p.status == nil {
		return nil, nil, nil, nil
	}

	staged := make([]statusRow, 0, len(p.status.StagedFiles))
	for _, file := range p.status.StagedFiles {
		staged = append(staged, statusRow{icon: statusIcon(file.StagedStatus, "M"), path: fileLabel(file)})
	}

	unstaged := make([]statusRow, 0, len(p.status.UnstagedFiles))
	for _, file := range p.status.UnstagedFiles {
		unstaged = append(unstaged, statusRow{icon: statusIcon(file.WorktreeStatus, "M"), path: fileLabel(file)})
	}

	untracked := make([]statusRow, 0, len(p.status.UntrackedFiles))
	for _, file := range p.status.UntrackedFiles {
		untracked = append(untracked, statusRow{icon: "?", path: fileLabel(file)})
	}

	conflicted := make([]statusRow, 0, len(p.status.ConflictedFiles))
	for _, file := range p.status.ConflictedFiles {
		conflicted = append(conflicted, statusRow{icon: "!", path: fileLabel(file)})
	}

	return staged, unstaged, untracked, conflicted
}

func (p *StatusPane) clampCursor() {
	count := len(p.rows())
	if count == 0 {
		p.cursor = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= count {
		p.cursor = count - 1
	}
}

func statusIcon(status gitmodel.FileStatus, fallback string) string {
	switch status {
	case gitmodel.Modified:
		return "M"
	case gitmodel.Added:
		return "A"
	case gitmodel.Deleted:
		return "D"
	case gitmodel.Renamed:
		return "R"
	case gitmodel.Untracked:
		return "?"
	case gitmodel.Updated:
		return "!"
	default:
		return fallback
	}
}

func fileLabel(file gitmodel.File) string {
	if file.OriginalPath != "" && file.Path != "" && file.OriginalPath != file.Path {
		return file.OriginalPath + " -> " + file.Path
	}
	if file.Path != "" {
		return file.Path
	}
	return "(unknown)"
}
