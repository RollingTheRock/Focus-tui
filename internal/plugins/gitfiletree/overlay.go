package gitfiletree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"focus/internal/adapters"
	"focus/internal/git"
	"focus/internal/models"
	"focus/internal/plugins/editor"
	filebrowser "focus/internal/plugins/filebrowser"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/plugins/gitfiletree/graph"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func logOverlay(s string) {
	f, _ := os.OpenFile("/tmp/overlay-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		_, _ = f.WriteString(time.Now().Format("15:04:05") + " " + s + "\n")
		_ = f.Close()
	}
}

// OpenGitFileTreeMsg triggers the GitFileTree overlay.
type OpenGitFileTreeMsg struct {
	RepoPath string
}

// CloseOverlayMsg closes the GitFileTree overlay.
type CloseOverlayMsg struct {
	ID models.PaneID
}

// rightPaneMode controls what the right pane displays.
type rightPaneMode int

const (
	rightPaneGraph rightPaneMode = iota
	rightPaneDiff
)

// GitFileTreeOverlay is a combined file tree + git status + commit graph overlay.
// Layout: left pane = file list, right pane = diff preview or commit graph.
type GitFileTreeOverlay struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath string

	// Left pane: file list
	status      *git.Status
	files       []fileEntry
	fileCursor  int
	fileLoading bool
	fileErr     error

	// Right pane: commit graph
	commits        []commitEntry
	commitCursor   int
	commitsLoading bool

	// Right pane: diff preview
	rightMode   rightPaneMode
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
		id:        id,
		meta:      meta,
		common:    common,
		adapter:   adapter,
		repoPath:  repoPath,
		rightMode: rightPaneGraph,
	}
}

func (o *GitFileTreeOverlay) Init() tea.Cmd {
	o.fileLoading = true
	o.commitsLoading = true
	return tea.Batch(o.loadStatusCmd(), o.loadCommitsCmd(), o.autoRefreshCmd())
}

func (o *GitFileTreeOverlay) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case gitStatusLoadedMsg:
		o.fileLoading = false
		o.fileErr = msg.err
		o.status = msg.status
		o.rebuildFileEntries()
		o.clampFileCursor()
		return o, o.previewSelectedFile()

	case gitFilesLoadedMsg:
		return o, nil

	case diffLoadedMsg:
		o.diffLoading = false
		if msg.err == nil {
			o.diffPreview = msg.diff
			o.diffPath = msg.path
			o.diffStaged = msg.staged
			if o.diffPath != "" {
				o.rightMode = rightPaneDiff
			}
		}
		return o, nil

	case commitsLoadedMsg:
		o.commitsLoading = false
		if msg.err == nil {
			o.commits = msg.commits
		}
		return o, nil

	case autoRefreshMsg:
		return o, tea.Batch(o.refreshCmd(), o.autoRefreshCmd())

	case tea.KeyPressMsg:
		logOverlay(fmt.Sprintf("Update received KeyPressMsg keystroke=%s", msg.Keystroke()))
		return o.updateKey(msg)
	}

	return o, nil
}

func (o *GitFileTreeOverlay) updateKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	count := len(o.files)
	logOverlay(fmt.Sprintf("updateKey keystroke=%s files=%d cursor=%d", msg.Keystroke(), count, o.fileCursor))

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
	case "space":
		return o.handleStageToggle()
	case "a":
		return o, o.handleStageAllToggle()
	case "d":
		return o, o.handleDiff()
	case "g":
		o.rightMode = rightPaneGraph
		return o, nil
	case "c":
		return o, o.handleCommit()
	case "ctrl+d":
		return o, o.handleDiscard()
	case "r":
		return o, o.refreshCmd()
	case "enter":
		return o, o.handleOpenFile()
	case "esc", "q":
		logOverlay("updateKey: returning closeOverlayCmd")
		return o, closeOverlayCmd(o.id)
	}

	return o, nil
}

func (o *GitFileTreeOverlay) View() tea.View {
	w := o.width
	if w <= 0 {
		w = 80
	}
	h := o.height
	if h <= 0 {
		h = 24
	}

	// Header
	header := o.renderHeader(w)

	// Content area
	contentH := h - 2 // header + footer
	if contentH < 1 {
		contentH = 1
	}

	leftW := w / 3
	if leftW < 28 {
		leftW = 28
	}
	if leftW > 45 {
		leftW = 45
	}
	rightW := w - leftW - 1 // 1 col separator
	if rightW < 20 {
		rightW = 20
		leftW = w - rightW - 1
	}

	leftContent := o.renderLeftPane(leftW, contentH)
	rightContent := o.renderRightPane(rightW, contentH)

	// Pad both panes to same height
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")
	for len(leftLines) < contentH {
		leftLines = append(leftLines, strings.Repeat(" ", leftW))
	}
	for len(rightLines) < contentH {
		rightLines = append(rightLines, strings.Repeat(" ", rightW))
	}
	if len(leftLines) > contentH {
		leftLines = leftLines[:contentH]
	}
	if len(rightLines) > contentH {
		rightLines = rightLines[:contentH]
	}

	// Join horizontally line by line
	sep := lipgloss.NewStyle().Foreground(styles.Subtle).Render("│")
	var contentLines []string
	for i := 0; i < contentH; i++ {
		ll := lipgloss.NewStyle().Width(leftW).Render(leftLines[i])
		rl := lipgloss.NewStyle().Width(rightW).Render(rightLines[i])
		contentLines = append(contentLines, ll+sep+rl)
	}

	// Footer hint
	footer := o.renderFooter(w)

	allLines := append([]string{header}, contentLines...)
	allLines = append(allLines, footer)

	return tea.NewView(strings.Join(allLines, "\n"))
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

	rightLabel := "[G]raph"
	if o.rightMode == rightPaneDiff {
		rightLabel = "[D]iff"
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(styles.Subtle).Render(rightLabel))

	return headerStyle.MaxWidth(width).Render(strings.Join(parts, " "))
}

func (o *GitFileTreeOverlay) renderLeftPane(width, height int) string {
	if o.fileLoading && o.status == nil {
		return loadingStyle.Render("Loading...")
	}
	if o.fileErr != nil {
		return errorStyle.Render("Err: " + truncate(o.fileErr.Error(), width-4))
	}
	if o.status == nil {
		return emptyStyle.Render("No git status.")
	}

	var lines []string
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

	// Truncate or pad to height
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderRightPane(width, height int) string {
	if o.rightMode == rightPaneDiff {
		return o.renderDiffPane(width, height)
	}
	return o.renderGraphPane(width, height)
}

func (o *GitFileTreeOverlay) renderDiffPane(width, height int) string {
	if o.diffLoading {
		return loadingStyle.Render("Loading diff...")
	}
	if o.diffPreview == "" {
		return emptyStyle.Render("Select a file to preview diff. Press [d] to load, [g] for graph.")
	}

	var lines []string
	if o.diffPath != "" {
		dir := filepath.Dir(o.diffPath)
		if dir == "." {
			dir = ""
		}
		name := filepath.Base(o.diffPath)
		label := name
		if dir != "" {
			label = dir + "/" + name
		}
		if o.diffStaged {
			label += " (staged)"
		}
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(styles.Accent).Render(truncate(label, width)))
	}

	previewLines := strings.Split(o.diffPreview, "\n")
	available := height - len(lines)
	if available < 3 {
		available = 3
	}
	for i, pl := range previewLines {
		if i >= available {
			remaining := len(previewLines) - available
			if remaining > 0 {
				lines = append(lines, hintStyle.Render(fmt.Sprintf("  ... %d more lines", remaining)))
			}
			break
		}
		lines = append(lines, o.renderDiffLine(pl))
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderGraphPane(width, height int) string {
	if o.commitsLoading {
		return loadingStyle.Render("Loading commits...")
	}
	if len(o.commits) == 0 {
		return emptyStyle.Render("No commits.")
	}

	// Convert commitEntry to graph.Commit
	gcommits := make([]graph.Commit, len(o.commits))
	for i, c := range o.commits {
		gcommits[i] = graph.Commit{
			Hash:    c.Hash,
			Subject: c.Subject,
			Author:  c.Author,
			Date:    c.Date,
			Parents: c.Parents,
		}
	}

	var selectedHash string
	if o.commitCursor >= 0 && o.commitCursor < len(o.commits) {
		selectedHash = o.commits[o.commitCursor].Hash
	}

	renderer := graph.NewRenderer(gcommits, selectedHash)
	graphWidth := width / 3
	if graphWidth < 12 {
		graphWidth = 12
	}
	if graphWidth > 30 {
		graphWidth = 30
	}
	graphLines := renderer.Render(graphWidth)

	// Right side: commit info (hash author subject)
	infoWidth := width - graphWidth - 1
	if infoWidth < 10 {
		infoWidth = 10
	}

	var lines []string
	for i := 0; i < len(graphLines) && i < height; i++ {
		var info string
		if i < len(o.commits) {
			c := o.commits[i]
			shortHash := c.Hash
			if len(shortHash) > 7 {
				shortHash = shortHash[:7]
			}
			hashPart := hashStyle.Render(shortHash)
			authorPart := authorStyle.Render(graph.Truncate(c.Author, 10))
			subjectPart := graph.Truncate(c.Subject, infoWidth-22)
			info = fmt.Sprintf(" %s %s %s", hashPart, authorPart, subjectPart)
		}
		if i == o.commitCursor {
			info = selectedStyle.Render(info)
		}
		// Pad info to fixed width
		if lipgloss.Width(info) < infoWidth {
			info += strings.Repeat(" ", infoWidth-lipgloss.Width(info))
		} else if lipgloss.Width(info) > infoWidth {
			info = info[:infoWidth]
		}
		lines = append(lines, graphLines[i]+info)
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderFooter(width int) string {
	return hintStyle.MaxWidth(width).Render(
		"[j/k]nav [space]stage [a]ll [d]iff [g]raph [c]ommit [ctrl+d]discard [enter]open [esc]close",
	)
}

func (o *GitFileTreeOverlay) renderSection(title string, entries []fileEntry, width int) []string {
	if len(entries) == 0 {
		return nil
	}

	var lines []string
	lines = append(lines, sectionStyle.Render(title+fmt.Sprintf(" (%d)", len(entries))))

	for _, e := range entries {
		statusIcon, colorStr := statusIconAndColor(e.file, e.status)
		name := filepath.Base(e.file.Path)
		if e.file.OriginalPath != "" && e.file.OriginalPath != e.file.Path {
			name = filepath.Base(e.file.OriginalPath) + " -> " + name
		}

		// File type icon via Nerd Fonts
		fileTypeIcon, _ := filebrowser.FileIcon(name, false, filebrowser.IconModeNerd)
		if fileTypeIcon == "" {
			fileTypeIcon = " "
		}

		statusStr := statusStyle.Copy().Foreground(lipgloss.Color(colorStr)).Render(statusIcon)
		// line: "  status fileIcon name"
		line := fmt.Sprintf("  %s %s %s", statusStr, fileTypeIcon, nameStyle.Render(name))

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

func (o *GitFileTreeOverlay) autoRefreshCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
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

type autoRefreshMsg struct{}

// --- External helpers ---

func loadCommits(repoPath string) ([]commitEntry, error) {
	// %H = hash, %P = parent hashes (space separated), %s = subject, %an = author, %ar = relative date
	cmd := exec.Command("git", "-C", repoPath, "log", "--format=%H|%P|%s|%an|%ar", "-50")
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
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		var parents []string
		if parts[1] != "" {
			parents = strings.Split(parts[1], " ")
		}
		commits = append(commits, commitEntry{
			Hash:    parts[0],
			Parents: parents,
			Subject: parts[2],
			Author:  parts[3],
			Date:    parts[4],
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
	addedLineStyle     = lipgloss.NewStyle().Foreground(styles.Success)
	removedLineStyle   = lipgloss.NewStyle().Foreground(styles.Overdue)
	hunkHeaderStyle    = lipgloss.NewStyle().Foreground(styles.Accent)
	diffLineStyle      = lipgloss.NewStyle().Foreground(styles.Subtle)
)
