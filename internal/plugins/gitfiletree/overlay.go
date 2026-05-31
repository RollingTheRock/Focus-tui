package gitfiletree

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"focus/internal/adapters"
	"focus/internal/git"
	"focus/internal/models"
	"focus/internal/plugins/editor"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

)

// OpenGitFileTreeMsg triggers the GitFileTree overlay.
type OpenGitFileTreeMsg struct {
	RepoPath string
}

// CloseOverlayMsg closes the GitFileTree overlay.
type CloseOverlayMsg struct {
	ID models.PaneID
}

// overlayMode controls which view is active.
type overlayMode int

const (
	modeFiles overlayMode = iota
	modeCommits
)

// GitFileTreeOverlay is a combined file tree + git status + commit graph overlay.
type GitFileTreeOverlay struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath string
	mode     overlayMode

	// Files mode state
	status      *git.Status
	files       []fileEntry
	fileCursor  int
	fileLoading bool
	fileErr     error

	// Commits mode state (populated later)
	commits      []commitEntry
	commitCursor int

	// Diff preview (bottom panel within overlay)
	diffPreview string
	diffPath    string
	diffStaged  bool
	diffLoading bool

	width  int
	height int
}

// fileEntry is a file with its git status for display.
type fileEntry struct {
	file   git.File
	status string // "staged", "unstaged", "untracked", "conflicted"
}

// commitEntry is a commit for the graph view.
type commitEntry struct {
	Hash    string
	Subject string
	Author  string
	Date    string
	Parents []string
}

// NewOverlay creates a new GitFileTree overlay.
func NewOverlay(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter, repoPath string) *GitFileTreeOverlay {
	return &GitFileTreeOverlay{
		id:       id,
		meta:     meta,
		common:   common,
		adapter:  adapter,
		repoPath: repoPath,
		mode:     modeFiles,
	}
}

func (o *GitFileTreeOverlay) Init() tea.Cmd {
	o.fileLoading = true
	return tea.Batch(o.loadStatusCmd(), o.loadFilesCmd())
}

func (o *GitFileTreeOverlay) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case gitStatusLoadedMsg:
		o.fileLoading = false
		o.fileErr = msg.err
		o.status = msg.status
		o.rebuildFileEntries()
		o.clampFileCursor()
		return o, nil

	case gitFilesLoadedMsg:
		// Files loaded from tree walk
		return o, nil

	case diffLoadedMsg:
		o.diffLoading = false
		if msg.err == nil {
			o.diffPreview = msg.diff
			o.diffPath = msg.path
			o.diffStaged = msg.staged
		}
		return o, nil

	case tea.KeyPressMsg:
		return o.updateKey(msg)
	}

	return o, nil
}

func (o *GitFileTreeOverlay) updateKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	if o.mode == modeFiles {
		return o.updateFilesKey(msg)
	}
	return o.updateCommitsKey(msg)
}

func (o *GitFileTreeOverlay) updateFilesKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	count := len(o.files)

	switch msg.Keystroke() {
	case "j", "down":
		if o.fileCursor < count-1 {
			o.fileCursor++
		}
		return o, o.previewSelectedFile()
	case "k", "up":
		if o.fileCursor > 0 {
			o.fileCursor--
		}
		return o, o.previewSelectedFile()
	case "tab":
		o.mode = modeCommits
		if len(o.commits) == 0 {
			return o, o.loadCommitsCmd()
		}
		return o, nil
	case "space":
		return o.handleStageToggle()
	case "a":
		return o, o.handleStageAllToggle()
	case "d":
		return o, o.handleDiff()
	case "c":
		return o, o.handleCommit()
	case "ctrl+d":
		return o, o.handleDiscard()
	case "r":
		return o, o.refreshCmd()
	case "enter":
		return o, o.handleOpenFile()
	case "esc", "q":
		return o, closeOverlayCmd(o.id)
	}

	return o, nil
}

func (o *GitFileTreeOverlay) updateCommitsKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.commitCursor < len(o.commits)-1 {
			o.commitCursor++
		}
	case "k", "up":
		if o.commitCursor > 0 {
			o.commitCursor--
		}
	case "tab":
		o.mode = modeFiles
	case "esc", "q":
		return o, closeOverlayCmd(o.id)
	}
	return o, nil
}

func (o *GitFileTreeOverlay) View() tea.View {
	width := o.width
	if width <= 0 {
		width = 80
	}

	lines := []string{o.renderHeader(width)}

	if o.mode == modeFiles {
		lines = append(lines, o.renderFilesMode(width)...)
	} else {
		lines = append(lines, o.renderCommitsMode(width)...)
	}

	// Clamp to height
	if o.height > 0 && len(lines) > o.height {
		lines = lines[:o.height]
	}

	return tea.NewView(strings.Join(lines, "\n"))
}

func (o *GitFileTreeOverlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

func (o *GitFileTreeOverlay) String() string {
	return fmt.Sprintf("GitFileTreeOverlay(%s)", o.id)
}

// --- Rendering ---

func (o *GitFileTreeOverlay) renderHeader(width int) string {
	branch := "HEAD"
	if o.status != nil && o.status.Branch != "" {
		branch = o.status.Branch
	}

	parts := []string{branchStyle.Render(branch)}
	if o.status != nil && o.status.Upstream != "" {
		parts = append(parts, upstreamStyle.Render("-> "+o.status.Upstream))
	}
	if o.status != nil && o.status.Ahead > 0 {
		parts = append(parts, aheadStyle.Render(fmt.Sprintf("↑%d", o.status.Ahead)))
	}
	if o.status != nil && o.status.Behind > 0 {
		parts = append(parts, behindStyle.Render(fmt.Sprintf("↓%d", o.status.Behind)))
	}

	modeIndicator := "[1 Files]"
	if o.mode == modeCommits {
		modeIndicator = "[2 Commits]"
	}
	parts = append(parts, tabStyle.Render(modeIndicator))

	return headerStyle.MaxWidth(width).Render(strings.Join(parts, " "))
}

func (o *GitFileTreeOverlay) renderFilesMode(width int) []string {
	if o.fileLoading && o.status == nil {
		return []string{loadingStyle.Render("Loading git status...")}
	}
	if o.fileErr != nil {
		return []string{errorStyle.Render("Error: " + o.fileErr.Error())}
	}
	if o.status == nil {
		return []string{emptyStyle.Render("No git status available.")}
	}

	var lines []string
	lines = append(lines, "")

	// Section: Conflicted
	conflicted := o.filesInSection("conflicted")
	lines = append(lines, o.renderSection("Conflicted", conflicted, width)...)

	// Section: Staged
	staged := o.filesInSection("staged")
	lines = append(lines, o.renderSection("Staged", staged, width)...)

	// Section: Unstaged
	unstaged := o.filesInSection("unstaged")
	lines = append(lines, o.renderSection("Changes", unstaged, width)...)

	// Section: Untracked
	untracked := o.filesInSection("untracked")
	lines = append(lines, o.renderSection("Untracked", untracked, width)...)

	if len(o.files) == 0 {
		lines = append(lines, emptyStyle.Render("  Working tree clean."))
	}

	// Diff preview (bottom area)
	if o.diffPreview != "" && o.height > 20 {
		lines = append(lines, "", diffSeparatorStyle.Render(strings.Repeat("─", width)))
		previewLines := strings.Split(o.diffPreview, "\n")
		maxPreview := (o.height - len(lines)) / 2
		if maxPreview < 3 {
			maxPreview = 3
		}
		if len(previewLines) > maxPreview {
			previewLines = previewLines[:maxPreview]
		}
		for _, pl := range previewLines {
			lines = append(lines, o.renderDiffLine(pl))
		}
	}

	// Footer hint
	lines = append(lines, "", hintStyle.Render(
		"[j/k]nav [space]stage [d]iff [a]ll [c]ommit [ctrl+d]discard [tab]commits [esc]close",
	))

	return lines
}

func (o *GitFileTreeOverlay) renderCommitsMode(width int) []string {
	if len(o.commits) == 0 {
		return []string{"", emptyStyle.Render("  Loading commits...")}
	}

	var lines []string
	lines = append(lines, "")

	for i, c := range o.commits {
		prefix := "  "
		if i == o.commitCursor {
			prefix = "> "
		}
		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		line := fmt.Sprintf("%s%s %s %s", prefix, hashStyle.Render(shortHash), authorStyle.Render(truncate(c.Author, 12)), truncate(c.Subject, width-30))
		if i == o.commitCursor {
			line = selectedStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	lines = append(lines, "", hintStyle.Render("[j/k]nav [d]iff [tab]files [esc]close"))
	return lines
}

func (o *GitFileTreeOverlay) renderSection(title string, entries []fileEntry, width int) []string {
	if len(entries) == 0 {
		return nil
	}

	var lines []string
	lines = append(lines, sectionStyle.Render(title+fmt.Sprintf(" (%d)", len(entries))))

	for _, e := range entries {
		icon, colorStr := statusIconAndColor(e.file, e.status)
		name := filepath.Base(e.file.Path)
		if e.file.OriginalPath != "" && e.file.OriginalPath != e.file.Path {
			name = filepath.Base(e.file.OriginalPath) + " -> " + name
		}

		statusStr := statusStyle.Copy().Foreground(lipgloss.Color(colorStr)).Render(icon)
		line := fmt.Sprintf("  %s %s", statusStr, nameStyle.Render(name))

		idx := o.fileIndex(e.file.Path, e.status)
		if idx == o.fileCursor {
			line = selectedStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	return lines
}

func (o *GitFileTreeOverlay) renderDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return addedLineStyle.Render(line)
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return removedLineStyle.Render(line)
	case strings.HasPrefix(line, "@@ "):
		return hunkHeaderStyle.Render(line)
	default:
		return diffLineStyle.Render(line)
	}
}

// --- Actions ---

func (o *GitFileTreeOverlay) handleStageToggle() (models.Panel, tea.Cmd) {
	entry := o.selectedFile()
	if entry == nil || o.adapter == nil {
		return o, nil
	}

	var err error
	if entry.status == "staged" {
		err = o.adapter.UnstageFile(o.repoPath, entry.file.Path)
	} else {
		err = o.adapter.StageFile(o.repoPath, entry.file.Path)
	}
	if err != nil {
		return o, nil
	}
	return o, o.refreshCmd()
}

func (o *GitFileTreeOverlay) handleStageAllToggle() tea.Cmd {
	if o.adapter == nil || o.status == nil {
		return nil
	}

	shouldStage := len(o.status.UnstagedFiles) > 0 || len(o.status.UntrackedFiles) > 0 || len(o.status.ConflictedFiles) > 0
	if shouldStage {
		if err := o.adapter.StageAll(o.repoPath); err != nil {
			return nil
		}
	} else if len(o.status.StagedFiles) > 0 {
		if err := o.adapter.UnstageAll(o.repoPath); err != nil {
			return nil
		}
	} else {
		return nil
	}

	return o.refreshCmd()
}

func (o *GitFileTreeOverlay) handleDiff() tea.Cmd {
	entry := o.selectedFile()
	if entry == nil || o.adapter == nil {
		return nil
	}

	staged := entry.status == "staged"
	o.diffLoading = true
	return func() tea.Msg {
		diff, err := o.adapter.GetDiff(o.repoPath, entry.file.Path, staged)
		return diffLoadedMsg{diff: diff, path: entry.file.Path, staged: staged, err: err}
	}
}

func (o *GitFileTreeOverlay) handleCommit() tea.Cmd {
	if o.status == nil || len(o.status.StagedFiles) == 0 {
		return nil
	}
	return func() tea.Msg {
		return gitplugin.OpenCommitMsg{RepoPath: o.repoPath, StagedFiles: o.status.StagedFiles}
	}
}

func (o *GitFileTreeOverlay) handleDiscard() tea.Cmd {
	entry := o.selectedFile()
	if entry == nil || o.adapter == nil {
		return nil
	}
	if err := o.adapter.DiscardChanges(o.repoPath, entry.file.Path); err != nil {
		return nil
	}
	return o.refreshCmd()
}

func (o *GitFileTreeOverlay) handleOpenFile() tea.Cmd {
	entry := o.selectedFile()
	if entry == nil {
		return nil
	}
	return func() tea.Msg {
		return editor.OpenEditorMsg{FilePath: entry.file.Path, Behavior: editor.OpenBehaviorDefault}
	}
}

func (o *GitFileTreeOverlay) previewSelectedFile() tea.Cmd {
	entry := o.selectedFile()
	if entry == nil || o.adapter == nil {
		return nil
	}
	staged := entry.status == "staged"
	o.diffLoading = true
	return func() tea.Msg {
		diff, err := o.adapter.GetDiff(o.repoPath, entry.file.Path, staged)
		return diffLoadedMsg{diff: diff, path: entry.file.Path, staged: staged, err: err}
	}
}

// --- Helpers ---

func (o *GitFileTreeOverlay) rebuildFileEntries() {
	if o.status == nil {
		o.files = nil
		return
	}

	var entries []fileEntry
	for _, f := range o.status.ConflictedFiles {
		entries = append(entries, fileEntry{file: f, status: "conflicted"})
	}
	for _, f := range o.status.StagedFiles {
		entries = append(entries, fileEntry{file: f, status: "staged"})
	}
	for _, f := range o.status.UnstagedFiles {
		entries = append(entries, fileEntry{file: f, status: "unstaged"})
	}
	for _, f := range o.status.UntrackedFiles {
		entries = append(entries, fileEntry{file: f, status: "untracked"})
	}

	// Sort by path for stable ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].file.Path < entries[j].file.Path
	})

	o.files = entries
}

func (o *GitFileTreeOverlay) filesInSection(status string) []fileEntry {
	var result []fileEntry
	for _, e := range o.files {
		if e.status == status {
			result = append(result, e)
		}
	}
	return result
}

func (o *GitFileTreeOverlay) selectedFile() *fileEntry {
	if o.fileCursor < 0 || o.fileCursor >= len(o.files) {
		return nil
	}
	return &o.files[o.fileCursor]
}

func (o *GitFileTreeOverlay) fileIndex(path, status string) int {
	idx := 0
	for _, e := range o.files {
		if e.file.Path == path && e.status == status {
			return idx
		}
		idx++
	}
	return -1
}

func (o *GitFileTreeOverlay) clampFileCursor() {
	if len(o.files) == 0 {
		o.fileCursor = 0
		return
	}
	if o.fileCursor < 0 {
		o.fileCursor = 0
	}
	if o.fileCursor >= len(o.files) {
		o.fileCursor = len(o.files) - 1
	}
}

// --- Commands ---

func (o *GitFileTreeOverlay) refreshCmd() tea.Cmd {
	return tea.Batch(o.loadStatusCmd(), o.loadFilesCmd())
}

func (o *GitFileTreeOverlay) loadStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if o.adapter == nil {
			return gitStatusLoadedMsg{err: errors.New("git adapter not configured")}
		}
		status, err := o.adapter.GetStatus(o.repoPath)
		return gitStatusLoadedMsg{status: status, err: err}
	}
}

func (o *GitFileTreeOverlay) loadFilesCmd() tea.Cmd {
	return func() tea.Msg {
		// Status already contains all files; this is a no-op for now
		return gitFilesLoadedMsg{}
	}
}

func (o *GitFileTreeOverlay) loadCommitsCmd() tea.Cmd {
	return func() tea.Msg {
		commits, err := loadCommits(o.repoPath)
		return commitsLoadedMsg{commits: commits, err: err}
	}
}

func closeOverlayCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg {
		return CloseOverlayMsg{ID: id}
	}
}

// --- Message types ---

type gitStatusLoadedMsg struct {
	status *git.Status
	err    error
}

type gitFilesLoadedMsg struct{}

type diffLoadedMsg struct {
	diff   string
	path   string
	staged bool
	err    error
}

type commitsLoadedMsg struct {
	commits []commitEntry
	err     error
}

// --- External helpers ---

func loadCommits(repoPath string) ([]commitEntry, error) {
	cmd := exec.Command("git", "-C", repoPath, "log", "--oneline", "--format=%H|%s|%an|%ar", "-50")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []commitEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, commitEntry{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits, nil
}

func statusIconAndColor(f git.File, section string) (string, string) {
	switch section {
	case "staged":
		return "A", "#34D399" // green
	case "unstaged":
		return "M", "#FBBF24" // yellow
	case "untracked":
		return "?", "#6B7280" // gray
	case "conflicted":
		return "!", "#EF4444" // red
	default:
		return " ", "#9CA3AF"
	}
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// --- Styles ---

var (
	headerStyle        = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	branchStyle        = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	upstreamStyle      = lipgloss.NewStyle().Foreground(styles.Subtle)
	aheadStyle         = lipgloss.NewStyle().Foreground(styles.Success)
	behindStyle        = lipgloss.NewStyle().Foreground(styles.Warning)
	tabStyle           = lipgloss.NewStyle().Foreground(styles.Subtle)
	sectionStyle       = lipgloss.NewStyle().Bold(true).Foreground(styles.Text)
	nameStyle          = lipgloss.NewStyle().Foreground(styles.Text)
	statusStyle        = lipgloss.NewStyle().Bold(true)
	selectedStyle      = lipgloss.NewStyle().Background(styles.Highlight).Bold(true)
	loadingStyle       = lipgloss.NewStyle().Foreground(styles.Subtle)
	emptyStyle         = lipgloss.NewStyle().Foreground(styles.Subtle)
	errorStyle         = lipgloss.NewStyle().Foreground(styles.Overdue)
	hintStyle          = lipgloss.NewStyle().Foreground(styles.Subtle)
	hashStyle          = lipgloss.NewStyle().Foreground(styles.Accent)
	authorStyle        = lipgloss.NewStyle().Foreground(styles.Subtle)
	diffSeparatorStyle = lipgloss.NewStyle().Foreground(styles.Subtle)
	addedLineStyle     = lipgloss.NewStyle().Foreground(styles.Success)
	removedLineStyle   = lipgloss.NewStyle().Foreground(styles.Overdue)
	hunkHeaderStyle    = lipgloss.NewStyle().Foreground(styles.Accent)
	diffLineStyle      = lipgloss.NewStyle().Foreground(styles.Subtle)
)
