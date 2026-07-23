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

	"github.com/RollingTheRock/Focus-tui/internal/adapters"
	"github.com/RollingTheRock/Focus-tui/internal/git"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/plugins/editor"
	filebrowser "github.com/RollingTheRock/Focus-tui/internal/plugins/filebrowser"
	gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"
	"github.com/RollingTheRock/Focus-tui/internal/plugins/gitfiletree/graph"
	"github.com/RollingTheRock/Focus-tui/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	rightPaneStash
	rightPaneBranches
)

// overlayFocus controls which pane has keyboard focus.
type overlayFocus int

const (
	focusLeftPane overlayFocus = iota
	focusRightPane
)

// commitInputMode controls the commit input state.
type commitInputMode int

const (
	commitInputNone commitInputMode = iota
	commitInputNormal
	commitInputAmend
)

// gitNodeStatus describes the git status of a file or directory node.
type gitNodeStatus int

const (
	gitNodeClean gitNodeStatus = iota
	gitNodeStaged
	gitNodeUnstaged
	gitNodeUntracked
	gitNodeConflicted
	gitNodeMixed
)

// GitFileTreeNode is a file tree node annotated with git status.
type GitFileTreeNode struct {
	Name             string
	Path             string
	IsDir            bool
	Children         []*GitFileTreeNode
	Parent           *GitFileTreeNode
	Depth            int
	Collapsed        bool
	CompressionLevel int
	Status           gitNodeStatus
	HasStaged        bool
	HasUnstaged      bool
	HasUntracked     bool
	HasConflicted    bool
}

// stashEntry represents a git stash.
type stashEntry struct {
	Index   int
	Message string
}

// commitEntry is a commit for the graph view.
type commitEntry struct {
	Hash    string
	Subject string
	Author  string
	Date    string
	Parents []string
}

// GitFileTreeOverlay is a combined git-annotated file tree overlay.
// Layout: left pane = git file tree, right pane = graph/stash/branches.
type GitFileTreeOverlay struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath string

	// Git status
	status *git.Status

	// Left pane: git-annotated file tree
	treeRoot     *GitFileTreeNode
	treeCursor   int
	treeFlatList []*GitFileTreeNode
	treeLoading  bool
	treeErr      error

	// Right pane mode
	rightMode rightPaneMode

	// Focus: left or right pane
	focus overlayFocus

	// Right pane: commit graph
	commits        []commitEntry
	commitCursor   int
	commitsLoading bool

	// Right pane: stash list
	stashEntries []stashEntry
	stashCursor  int
	stashLoading bool

	// Right pane: branches list
	branches        []git.Branch
	branchCursor    int
	branchesLoading bool

	// Commit input
	commitInput string
	commitMode  commitInputMode

	width  int
	height int
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
	o.treeLoading = true
	o.commitsLoading = true
	o.stashLoading = true
	o.branchesLoading = true
	return tea.Batch(
		o.loadStatusCmd(),
		o.loadCommitsCmd(),
		o.loadStashCmd(),
		o.loadBranchesCmd(),
		o.autoRefreshCmd(),
	)
}

func (o *GitFileTreeOverlay) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	// Commit input mode: intercept most keys
	if o.commitMode != commitInputNone {
		return o.updateCommitInput(msg)
	}

	switch msg := msg.(type) {
	case gitStatusLoadedMsg:
		o.treeLoading = false
		o.treeErr = msg.err
		o.status = msg.status
		o.rebuildTree()
		o.clampTreeCursor()
		return o, nil

	case commitsLoadedMsg:
		o.commitsLoading = false
		if msg.err == nil {
			o.commits = msg.commits
		}
		return o, nil

	case stashLoadedMsg:
		o.stashLoading = false
		if msg.err == nil {
			o.stashEntries = msg.entries
		}
		return o, nil

	case branchesLoadedMsg:
		o.branchesLoading = false
		if msg.err == nil {
			o.branches = msg.branches
		}
		return o, nil

	case commitFinishedMsg:
		if msg.err == nil {
			return o, tea.Batch(o.refreshCmd(), o.loadCommitsCmd())
		}
		return o, nil

	case autoRefreshMsg:
		return o, tea.Batch(
			o.refreshCmd(),
			o.loadStashCmd(),
			o.loadBranchesCmd(),
			o.autoRefreshCmd(),
		)

	case tea.KeyPressMsg:
		logOverlay(fmt.Sprintf("Update received KeyPressMsg keystroke=%s", msg.Keystroke()))
		return o.updateKey(msg)
	}

	return o, nil
}

func (o *GitFileTreeOverlay) updateKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	logOverlay(fmt.Sprintf("updateKey keystroke=%s focus=%d treeCursor=%d", msg.Keystroke(), o.focus, o.treeCursor))

	switch msg.Keystroke() {
	case "tab":
		if o.focus == focusLeftPane {
			o.focus = focusRightPane
		} else {
			o.focus = focusLeftPane
		}
		return o, nil
	case "g":
		o.rightMode = rightPaneGraph
		return o, nil
	case "s":
		o.rightMode = rightPaneStash
		return o, nil
	case "b":
		o.rightMode = rightPaneBranches
		return o, nil
	case "r":
		return o, tea.Batch(
			o.refreshCmd(),
			o.loadStashCmd(),
			o.loadBranchesCmd(),
		)
	case "esc", "q":
		logOverlay("updateKey: returning closeOverlayCmd")
		return o, closeOverlayCmd(o.id)
	}

	if o.focus == focusLeftPane {
		return o.updateKeyLeftPane(msg)
	}
	return o.updateKeyRightPane(msg)
}

func (o *GitFileTreeOverlay) updateKeyLeftPane(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.treeCursor < len(o.treeFlatList)-1 {
			o.treeCursor++
		}
		return o, nil
	case "k", "up":
		if o.treeCursor > 0 {
			o.treeCursor--
		}
		return o, nil
	case "enter":
		return o.handleEnter()
	case "left", "right":
		return o.handleTreeToggle(msg.Keystroke())
	case "space":
		return o.handleStageToggle()
	case "o":
		return o, o.handleOpenFile()
	case "a":
		return o, o.handleStageAllToggle()
	case "c":
		return o.handleCommit()
	case "d":
		return o, o.handleDiscard()
	}
	return o, nil
}

func (o *GitFileTreeOverlay) updateKeyRightPane(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch o.rightMode {
	case rightPaneGraph:
		return o.updateKeyRightPaneGraph(msg)
	case rightPaneStash:
		return o.updateKeyRightPaneStash(msg)
	case rightPaneBranches:
		return o.updateKeyRightPaneBranches(msg)
	}
	return o, nil
}

func (o *GitFileTreeOverlay) updateKeyRightPaneGraph(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.commitCursor < len(o.commits)-1 {
			o.commitCursor++
		}
		return o, nil
	case "k", "up":
		if o.commitCursor > 0 {
			o.commitCursor--
		}
		return o, nil
	}
	return o, nil
}

func (o *GitFileTreeOverlay) updateKeyRightPaneStash(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.stashCursor < len(o.stashEntries)-1 {
			o.stashCursor++
		}
		return o, nil
	case "k", "up":
		if o.stashCursor > 0 {
			o.stashCursor--
		}
		return o, nil
	case " ", "enter":
		return o.handleStashApply()
	case "p":
		return o.handleStashPop()
	case "d":
		return o.handleStashDrop()
	}
	return o, nil
}

func (o *GitFileTreeOverlay) updateKeyRightPaneBranches(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.branchCursor < len(o.branches)-1 {
			o.branchCursor++
		}
		return o, nil
	case "k", "up":
		if o.branchCursor > 0 {
			o.branchCursor--
		}
		return o, nil
	case " ", "enter":
		return o.handleBranchCheckout()
	case "d":
		return o.handleBranchDelete()
	}
	return o, nil
}

// --- Commit Input ---

func (o *GitFileTreeOverlay) updateCommitInput(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "enter":
			return o.handleCommitConfirm()
		case "esc":
			return o.handleCommitCancel()
		case "backspace":
			if len(o.commitInput) > 0 {
				o.commitInput = o.commitInput[:len(o.commitInput)-1]
			}
			return o, nil
		default:
			// Append printable characters
			if len(msg.Text) == 1 && msg.Text >= " " && msg.Text != "\x7f" {
				o.commitInput += msg.Text
			}
			return o, nil
		}
	}
	return o, nil
}

func (o *GitFileTreeOverlay) handleCommit() (models.Panel, tea.Cmd) {
	if o.status == nil || len(o.status.StagedFiles) == 0 {
		return o, nil
	}
	o.commitMode = commitInputNormal
	o.commitInput = ""
	return o, nil
}

func (o *GitFileTreeOverlay) handleCommitConfirm() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.commitInput == "" {
		o.commitMode = commitInputNone
		return o, nil
	}
	msg := o.commitInput
	o.commitMode = commitInputNone
	o.commitInput = ""
	return o, func() tea.Msg {
		if err := o.adapter.Commit(o.repoPath, msg); err != nil {
			return commitFinishedMsg{err: err}
		}
		return commitFinishedMsg{err: nil}
	}
}

// commitFinishedMsg signals that a commit attempt finished.
type commitFinishedMsg struct {
	err error
}

func (o *GitFileTreeOverlay) handleCommitCancel() (models.Panel, tea.Cmd) {
	o.commitMode = commitInputNone
	o.commitInput = ""
	return o, nil
}

// --- View ---

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

	// Footer or commit input
	var footer string
	if o.commitMode != commitInputNone {
		footer = o.renderCommitPanel(w)
	} else {
		footer = o.renderFooter(w)
	}

	// Content area
	footerH := lipgloss.Height(footer)
	headerH := lipgloss.Height(header)
	contentH := h - headerH - footerH
	if contentH < 1 {
		contentH = 1
	}

	leftW := w / 3
	if leftW < 32 {
		leftW = 32
	}
	if leftW > 50 {
		leftW = 50
	}
	rightW := w - leftW - 1 // 1 col separator
	if rightW < 25 {
		rightW = 25
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
	sepColor := styles.Subtle
	if o.focus == focusLeftPane {
		sepColor = styles.Accent
	}
	sep := lipgloss.NewStyle().Foreground(sepColor).Render("│")
	var contentLines []string
	for i := 0; i < contentH; i++ {
		ll := lipgloss.NewStyle().Width(leftW).Render(leftLines[i])
		rl := lipgloss.NewStyle().Width(rightW).Render(rightLines[i])
		contentLines = append(contentLines, ll+sep+rl)
	}

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

// --- Rendering: Header ---

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

	modeLabel := "[G]raph"
	switch o.rightMode {
	case rightPaneStash:
		modeLabel = "[S]tash"
	case rightPaneBranches:
		modeLabel = "[B]ranches"
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(styles.Subtle).Render(modeLabel))

	return headerStyle.MaxWidth(width).Render(strings.Join(parts, " "))
}

// --- Rendering: Left Pane (Git File Tree) ---

func (o *GitFileTreeOverlay) renderLeftPane(width, height int) string {
	if o.treeLoading && o.status == nil {
		return loadingStyle.Render("Loading...")
	}
	if o.treeErr != nil {
		return errorStyle.Render("Err: " + truncate(o.treeErr.Error(), width-4))
	}
	if o.status == nil {
		return emptyStyle.Render("No git status.")
	}
	if len(o.treeFlatList) == 0 {
		return emptyStyle.Render("Working tree clean.")
	}

	var lines []string

	start, end := o.visibleTreeRange(height)
	if start > 0 {
		lines = append(lines, metaStyle.Render(fmt.Sprintf("  ▲ %d hidden", start)))
	}
	for idx := start; idx < end && idx < len(o.treeFlatList); idx++ {
		lines = append(lines, o.renderTreeNode(idx, o.treeFlatList[idx], width))
	}
	if end < len(o.treeFlatList) {
		lines = append(lines, metaStyle.Render(fmt.Sprintf("  ▼ %d hidden", len(o.treeFlatList)-end)))
	}

	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderTreeNode(index int, node *GitFileTreeNode, width int) string {
	prefix := "  "
	if index == o.treeCursor {
		prefix = "> "
	}

	// Visual indent based on depth and compression
	visualDepth := node.Depth
	if node.CompressionLevel > 0 {
		visualDepth = node.Depth - node.CompressionLevel + 1
	}
	indent := strings.Repeat("  ", visualDepth)

	// Expansion marker for directories
	marker := "  "
	if node.IsDir {
		if node.Collapsed {
			marker = "▸ "
		} else {
			marker = "▾ "
		}
	}

	// File type icon (Unicode mode for terminal compatibility)
	icon, iconStyle := filebrowser.FileIcon(node.Name, node.IsDir, filebrowser.IconModeUnicode)
	if icon != "" {
		icon = iconStyle.Render(icon) + " "
	}

	// Git status icon
	statusIcon, statusColor := statusIconForNode(node)
	statusStr := ""
	if statusIcon != "" {
		statusStr = statusStyle.Copy().Foreground(lipgloss.Color(statusColor)).Render(statusIcon) + " "
	}

	line := prefix + indent + marker + statusStr + icon + nameStyle.Render(node.Name)
	line = ansi.Truncate(line, width, "")

	if index == o.treeCursor {
		return selectedStyle.Width(width).Render(line)
	}
	return styles.StyleCache.MaxWidth(width).Render(line)
}

func (o *GitFileTreeOverlay) visibleTreeRange(maxLines int) (int, int) {
	if maxLines <= 0 || len(o.treeFlatList) == 0 {
		return 0, 0
	}
	if len(o.treeFlatList) <= maxLines {
		return 0, len(o.treeFlatList)
	}

	start := o.treeCursor - maxLines + 1
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(o.treeFlatList) {
		end = len(o.treeFlatList)
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// --- Rendering: Right Pane ---

func (o *GitFileTreeOverlay) renderRightPane(width, height int) string {
	switch o.rightMode {
	case rightPaneStash:
		return o.renderStashPane(width, height)
	case rightPaneBranches:
		return o.renderBranchesPane(width, height)
	default:
		return o.renderGraphPane(width, height)
	}
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

func (o *GitFileTreeOverlay) renderStashPane(width, height int) string {
	if o.stashLoading {
		return loadingStyle.Render("Loading stash...")
	}
	if len(o.stashEntries) == 0 {
		return emptyStyle.Render("No stash entries.")
	}

	var lines []string
	for i, entry := range o.stashEntries {
		if i >= height {
			break
		}
		label := fmt.Sprintf("stash@{%d} %s", entry.Index, entry.Message)
		line := truncate(label, width)
		if i == o.stashCursor {
			line = selectedStyle.Width(width).Render(line)
		} else {
			line = nameStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderBranchesPane(width, height int) string {
	if o.branchesLoading {
		return loadingStyle.Render("Loading branches...")
	}
	if len(o.branches) == 0 {
		return emptyStyle.Render("No branches.")
	}

	var lines []string
	for i, b := range o.branches {
		if i >= height {
			break
		}
		marker := "  "
		if b.Current {
			marker = "* "
		}
		label := marker + b.Name
		if b.Upstream != "" {
			label += " -> " + b.Upstream
		}
		if b.Ahead > 0 || b.Behind > 0 {
			label += fmt.Sprintf(" [↑%d↓%d]", b.Ahead, b.Behind)
		}
		line := truncate(label, width)
		if i == o.branchCursor {
			line = selectedStyle.Width(width).Render(line)
		} else if b.Current {
			line = branchStyle.Width(width).Render(line)
		} else {
			line = nameStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// --- Rendering: Footer ---

func (o *GitFileTreeOverlay) renderFooter(width int) string {
	var parts []string

	// Focus indicator
	if o.focus == focusLeftPane {
		parts = append(parts, lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render("[Files]"))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(styles.Subtle).Render("[Files]"))
	}

	if o.focus == focusRightPane {
		parts = append(parts, lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render("["+o.rightPaneLabel()+"]"))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(styles.Subtle).Render("["+o.rightPaneLabel()+"]"))
	}

	return hintStyle.MaxWidth(width).Render(strings.Join(parts, " "))
}

func (o *GitFileTreeOverlay) rightPaneLabel() string {
	switch o.rightMode {
	case rightPaneStash:
		return "Stash"
	case rightPaneBranches:
		return "Branches"
	default:
		return "Graph"
	}
}

func (o *GitFileTreeOverlay) renderCommitPanel(width int) string {
	if o.status == nil {
		return o.renderCommitInput(width)
	}

	var lines []string

	// Title line: Commit — N file(s) staged
	title := fmt.Sprintf("Commit — %d file(s) staged", len(o.status.StagedFiles))
	lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render(title))

	// Staged files list
	if len(o.status.StagedFiles) > 0 {
		var parts []string
		for _, f := range o.status.StagedFiles {
			icon := string(f.StagedStatus)
			if f.StagedStatus == 0 || f.StagedStatus == git.Unmodified {
				icon = "A"
			}
			// color by status
			var iconStyle lipgloss.Style
			switch f.StagedStatus {
			case git.Added:
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
			case git.Modified:
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
			case git.Deleted:
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
			case git.Renamed:
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
			default:
				iconStyle = lipgloss.NewStyle().Foreground(styles.Text)
			}
			rendered := iconStyle.Render(icon) +
				" " + lipgloss.NewStyle().Foreground(styles.Text).Render(f.Path)
			parts = append(parts, rendered)
		}
		// Build file list lines, wrapping at width
		fileLines := wrapRenderedItems(parts, width, "  ")
		// Show max 2 lines of files
		for i := 0; i < len(fileLines) && i < 2; i++ {
			lines = append(lines, "  "+fileLines[i])
		}
	}

	// Separator
	lines = append(lines, lipgloss.NewStyle().Foreground(styles.Subtle).Render(strings.Repeat("─", width)))

	// Message input line
	prompt := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render("Message: ")
	msg := lipgloss.NewStyle().Foreground(styles.Text).Render(o.commitInput)
	hint := lipgloss.NewStyle().Foreground(styles.Subtle).Render(" [↵]commit [esc]cancel")
	lines = append(lines, prompt+msg+hint)

	return strings.Join(lines, "\n")
}

func (o *GitFileTreeOverlay) renderCommitInput(width int) string {
	prompt := "Commit message: "
	return lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render(prompt) +
		lipgloss.NewStyle().Foreground(styles.Text).Render(o.commitInput) +
		lipgloss.NewStyle().Foreground(styles.Subtle).Render(" [↵]commit [esc]cancel")
}

// --- Actions ---

func (o *GitFileTreeOverlay) handleEnter() (models.Panel, tea.Cmd) {
	node := o.selectedTreeNode()
	if node == nil {
		return o, nil
	}
	if !node.IsDir {
		return o, o.handleDiff()
	}
	// Directory: toggle collapse
	node.Collapsed = !node.Collapsed
	selectedPath := node.Path
	o.treeFlatList = flattenVisibleGitNodes(o.treeRoot, true)
	o.selectTreeNodeByPath(selectedPath)
	return o, nil
}

func (o *GitFileTreeOverlay) handleDiff() tea.Cmd {
	node := o.selectedTreeNode()
	if node == nil || node.IsDir {
		return nil
	}
	path := node.Path
	staged := o.fileGitStatus(path) == "staged"
	return func() tea.Msg {
		return gitplugin.OpenDiffMsg{FilePath: path, Staged: staged}
	}
}

func (o *GitFileTreeOverlay) handleTreeToggle(keystroke string) (models.Panel, tea.Cmd) {
	node := o.selectedTreeNode()
	if node == nil {
		return o, nil
	}

	if !node.IsDir {
		// File: right to open editor, left to jump to parent
		if keystroke == "right" {
			return o, o.handleOpenFile()
		}
		if keystroke == "left" && node.Parent != nil {
			o.selectTreeNodeByPath(node.Parent.Path)
		}
		return o, nil
	}

	// Directory: toggle collapse
	if keystroke == "left" && !node.Collapsed && len(node.Children) > 0 {
		node.Collapsed = true
	} else if keystroke == "left" && node.Collapsed && node.Parent != nil {
		// Move cursor to parent
		o.selectTreeNodeByPath(node.Parent.Path)
		return o, nil
	} else if keystroke == "right" && node.Collapsed {
		node.Collapsed = false
	}

	selectedPath := node.Path
	o.treeFlatList = flattenVisibleGitNodes(o.treeRoot, true)
	o.selectTreeNodeByPath(selectedPath)
	return o, nil
}

func (o *GitFileTreeOverlay) handleStageToggle() (models.Panel, tea.Cmd) {
	node := o.selectedTreeNode()
	if node == nil || o.adapter == nil || o.status == nil {
		return o, nil
	}

	if node.IsDir {
		return o.handleStageDir(node)
	}

	// File: determine current status and toggle
	status := o.fileGitStatus(node.Path)
	var err error
	if status == "staged" {
		err = o.adapter.UnstageFile(o.repoPath, node.Path)
	} else {
		err = o.adapter.StageFile(o.repoPath, node.Path)
	}
	if err != nil {
		return o, nil
	}
	return o, o.refreshCmd()
}

func (o *GitFileTreeOverlay) handleStageDir(node *GitFileTreeNode) (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.status == nil {
		return o, nil
	}

	// Collect all dirty file paths under this directory
	paths := o.collectDirtyPaths(node)
	if len(paths) == 0 {
		return o, nil
	}

	// Check if majority are already staged
	stagedCount := 0
	for _, p := range paths {
		if o.fileGitStatus(p) == "staged" {
			stagedCount++
		}
	}

	shouldStage := stagedCount < len(paths)
	for _, p := range paths {
		if shouldStage {
			_ = o.adapter.StageFile(o.repoPath, p)
		} else {
			_ = o.adapter.UnstageFile(o.repoPath, p)
		}
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

func (o *GitFileTreeOverlay) handleDiscard() tea.Cmd {
	node := o.selectedTreeNode()
	if node == nil || o.adapter == nil {
		return nil
	}

	if node.IsDir {
		paths := o.collectDirtyPaths(node)
		for _, p := range paths {
			_ = o.adapter.DiscardChanges(o.repoPath, p)
		}
	} else {
		if err := o.adapter.DiscardChanges(o.repoPath, node.Path); err != nil {
			return nil
		}
	}
	return o.refreshCmd()
}

func (o *GitFileTreeOverlay) handleOpenFile() tea.Cmd {
	node := o.selectedTreeNode()
	if node == nil || node.IsDir {
		return nil
	}
	path := node.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(o.repoPath, path)
	}
	return func() tea.Msg {
		return editor.OpenEditorMsg{FilePath: path, Behavior: editor.OpenBehaviorDefault}
	}
}

// --- Right Pane Actions ---

func (o *GitFileTreeOverlay) handleStashApply() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.stashCursor < 0 || o.stashCursor >= len(o.stashEntries) {
		return o, nil
	}
	idx := o.stashEntries[o.stashCursor].Index
	return o, func() tea.Msg {
		if err := o.adapter.StashApply(o.repoPath, idx); err != nil {
			return commitFinishedMsg{err: err}
		}
		return commitFinishedMsg{err: nil}
	}
}

func (o *GitFileTreeOverlay) handleStashPop() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.stashCursor < 0 || o.stashCursor >= len(o.stashEntries) {
		return o, nil
	}
	idx := o.stashEntries[o.stashCursor].Index
	return o, func() tea.Msg {
		if err := o.adapter.StashPop(o.repoPath, idx); err != nil {
			return commitFinishedMsg{err: err}
		}
		return commitFinishedMsg{err: nil}
	}
}

func (o *GitFileTreeOverlay) handleStashDrop() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.stashCursor < 0 || o.stashCursor >= len(o.stashEntries) {
		return o, nil
	}
	idx := o.stashEntries[o.stashCursor].Index
	return o, func() tea.Msg {
		if err := o.adapter.StashDrop(o.repoPath, idx); err != nil {
			return commitFinishedMsg{err: err}
		}
		return commitFinishedMsg{err: nil}
	}
}

func (o *GitFileTreeOverlay) handleBranchCheckout() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.branchCursor < 0 || o.branchCursor >= len(o.branches) {
		return o, nil
	}
	branch := o.branches[o.branchCursor].Name
	return o, func() tea.Msg {
		if err := o.adapter.CheckoutBranch(o.repoPath, branch); err != nil {
			return commitFinishedMsg{err: err}
		}
		return commitFinishedMsg{err: nil}
	}
}

func (o *GitFileTreeOverlay) handleBranchDelete() (models.Panel, tea.Cmd) {
	if o.adapter == nil || o.branchCursor < 0 || o.branchCursor >= len(o.branches) {
		return o, nil
	}
	// For now, just return without implementing actual delete
	// This would need confirmation in a real implementation
	return o, nil
}

// --- Tree Helpers ---

func (o *GitFileTreeOverlay) selectedTreeNode() *GitFileTreeNode {
	if o.treeCursor < 0 || o.treeCursor >= len(o.treeFlatList) {
		return nil
	}
	return o.treeFlatList[o.treeCursor]
}

func (o *GitFileTreeOverlay) selectTreeNodeByPath(path string) {
	for i, node := range o.treeFlatList {
		if node.Path == path {
			o.treeCursor = i
			return
		}
	}
	o.clampTreeCursor()
}

func (o *GitFileTreeOverlay) clampTreeCursor() {
	if len(o.treeFlatList) == 0 {
		o.treeCursor = 0
		return
	}
	if o.treeCursor < 0 {
		o.treeCursor = 0
	}
	if o.treeCursor >= len(o.treeFlatList) {
		o.treeCursor = len(o.treeFlatList) - 1
	}
}

func (o *GitFileTreeOverlay) fileGitStatus(path string) string {
	if o.status == nil {
		return ""
	}
	for _, f := range o.status.ConflictedFiles {
		if f.Path == path {
			return "conflicted"
		}
	}
	for _, f := range o.status.StagedFiles {
		if f.Path == path {
			return "staged"
		}
	}
	for _, f := range o.status.UnstagedFiles {
		if f.Path == path {
			return "unstaged"
		}
	}
	for _, f := range o.status.UntrackedFiles {
		if f.Path == path {
			return "untracked"
		}
	}
	return ""
}

func (o *GitFileTreeOverlay) collectDirtyPaths(node *GitFileTreeNode) []string {
	var paths []string
	if !node.IsDir {
		if o.fileGitStatus(node.Path) != "" {
			paths = append(paths, node.Path)
		}
		return paths
	}
	for _, child := range node.Children {
		paths = append(paths, o.collectDirtyPaths(child)...)
	}
	return paths
}

// --- Tree Building ---

func (o *GitFileTreeOverlay) rebuildTree() {
	if o.status == nil {
		o.treeRoot = nil
		o.treeFlatList = nil
		return
	}

	// Save collapsed states before rebuild
	collapsedPaths := make(map[string]bool)
	if o.treeRoot != nil {
		collectCollapsedGitNodes(o.treeRoot, collapsedPaths)
	}

	// Collect dirty files with status
	dirtyFiles := make(map[string]gitNodeStatus)
	for _, f := range o.status.ConflictedFiles {
		dirtyFiles[f.Path] = gitNodeConflicted
	}
	for _, f := range o.status.StagedFiles {
		dirtyFiles[f.Path] = gitNodeStaged
	}
	for _, f := range o.status.UnstagedFiles {
		dirtyFiles[f.Path] = gitNodeUnstaged
	}
	for _, f := range o.status.UntrackedFiles {
		dirtyFiles[f.Path] = gitNodeUntracked
	}

	if len(dirtyFiles) == 0 {
		o.treeRoot = nil
		o.treeFlatList = nil
		return
	}

	root := &GitFileTreeNode{
		Name:      filepath.Base(o.repoPath),
		Path:      "", // relative paths from repo root
		IsDir:     true,
		Depth:     0,
		Collapsed: false,
	}

	for path, status := range dirtyFiles {
		parts := strings.Split(path, string(filepath.Separator))
		o.insertFileIntoTree(root, parts, status)
	}

	// Compute directory statuses
	o.computeDirStatus(root)

	// Sort children alphabetically: dirs first, then files
	o.sortTreeChildren(root)

	// Compress single-child directory chains
	root.compress()

	// Restore collapsed states
	applyCollapsedGitNodes(root, collapsedPaths)

	o.treeRoot = root
	o.treeFlatList = flattenVisibleGitNodes(root, true)
}

func (o *GitFileTreeOverlay) insertFileIntoTree(parent *GitFileTreeNode, parts []string, status gitNodeStatus) {
	if len(parts) == 0 {
		return
	}

	name := parts[0]
	var node *GitFileTreeNode
	for _, child := range parent.Children {
		if child.Name == name {
			node = child
			break
		}
	}

	if node == nil {
		node = &GitFileTreeNode{
			Name:      name,
			Path:      filepath.Join(parent.Path, name),
			IsDir:     len(parts) > 1,
			Parent:    parent,
			Depth:     parent.Depth + 1,
			Collapsed: false, // expand directories by default (all shown dirs have changes)
		}
		parent.Children = append(parent.Children, node)
	}

	if len(parts) == 1 {
		node.Status = status
		node.IsDir = false
	} else {
		node.IsDir = true
		o.insertFileIntoTree(node, parts[1:], status)
	}
}

func (o *GitFileTreeOverlay) computeDirStatus(node *GitFileTreeNode) gitNodeStatus {
	if !node.IsDir {
		switch node.Status {
		case gitNodeStaged:
			node.HasStaged = true
		case gitNodeUnstaged:
			node.HasUnstaged = true
		case gitNodeUntracked:
			node.HasUntracked = true
		case gitNodeConflicted:
			node.HasConflicted = true
		}
		return node.Status
	}

	var statuses []gitNodeStatus
	for _, child := range node.Children {
		s := o.computeDirStatus(child)
		if s != gitNodeClean {
			statuses = append(statuses, s)
		}
		node.HasStaged = node.HasStaged || child.HasStaged
		node.HasUnstaged = node.HasUnstaged || child.HasUnstaged
		node.HasUntracked = node.HasUntracked || child.HasUntracked
		node.HasConflicted = node.HasConflicted || child.HasConflicted
	}

	if len(statuses) == 0 {
		node.Status = gitNodeClean
		return gitNodeClean
	}

	allSame := true
	for i := 1; i < len(statuses); i++ {
		if statuses[i] != statuses[0] {
			allSame = false
			break
		}
	}
	if allSame {
		node.Status = statuses[0]
	} else {
		node.Status = gitNodeMixed
	}
	return node.Status
}

func (o *GitFileTreeOverlay) sortTreeChildren(node *GitFileTreeNode) {
	if !node.IsDir || len(node.Children) == 0 {
		return
	}
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir // dirs first
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		o.sortTreeChildren(child)
	}
}

func (n *GitFileTreeNode) compress() {
	if !n.IsDir || len(n.Children) == 0 {
		return
	}

	for i := range n.Children {
		child := n.Children[i]
		if !child.IsDir || len(child.Children) != 1 || !child.Children[0].IsDir {
			child.compress()
			continue
		}

		grandchild := child.Children[0]
		merged := &GitFileTreeNode{
			Name:             child.Name + "/" + grandchild.Name,
			Path:             grandchild.Path,
			IsDir:            true,
			Children:         grandchild.Children,
			Parent:           n,
			Depth:            child.Depth,
			Collapsed:        grandchild.Collapsed,
			CompressionLevel: child.CompressionLevel + 1,
			Status:           grandchild.Status,
			HasStaged:        grandchild.HasStaged,
			HasUnstaged:      grandchild.HasUnstaged,
			HasUntracked:     grandchild.HasUntracked,
			HasConflicted:    grandchild.HasConflicted,
		}
		for _, gc := range merged.Children {
			gc.Parent = merged
		}
		n.Children[i] = merged
		merged.compress()
	}
}

func flattenVisibleGitNodes(root *GitFileTreeNode, skipRoot bool) []*GitFileTreeNode {
	if root == nil {
		return nil
	}
	if skipRoot {
		var flat []*GitFileTreeNode
		if root.IsDir && !root.Collapsed {
			for _, child := range root.Children {
				flat = append(flat, flattenVisibleGitNodes(child, false)...)
			}
		}
		return flat
	}
	flat := []*GitFileTreeNode{root}
	if root.IsDir && !root.Collapsed {
		for _, child := range root.Children {
			flat = append(flat, flattenVisibleGitNodes(child, false)...)
		}
	}
	return flat
}

func collectCollapsedGitNodes(node *GitFileTreeNode, out map[string]bool) {
	if node == nil {
		return
	}
	if node.IsDir {
		out[node.Path] = node.Collapsed
	}
	for _, child := range node.Children {
		collectCollapsedGitNodes(child, out)
	}
}

func applyCollapsedGitNodes(node *GitFileTreeNode, collapsed map[string]bool) {
	if node == nil {
		return
	}
	if node.IsDir {
		if value, ok := collapsed[node.Path]; ok {
			node.Collapsed = value
		}
	}
	for _, child := range node.Children {
		applyCollapsedGitNodes(child, collapsed)
	}
}

// --- Commands ---

func (o *GitFileTreeOverlay) refreshCmd() tea.Cmd {
	return o.loadStatusCmd()
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

func (o *GitFileTreeOverlay) loadCommitsCmd() tea.Cmd {
	return func() tea.Msg {
		commits, err := loadCommits(o.repoPath)
		return commitsLoadedMsg{commits: commits, err: err}
	}
}

func (o *GitFileTreeOverlay) loadStashCmd() tea.Cmd {
	return func() tea.Msg {
		if o.adapter == nil {
			return stashLoadedMsg{err: errors.New("git adapter not configured")}
		}
		entries, err := o.adapter.GetStashList(o.repoPath)
		var stash []stashEntry
		for _, e := range entries {
			stash = append(stash, stashEntry{Index: e.Index, Message: e.Message})
		}
		return stashLoadedMsg{entries: stash, err: err}
	}
}

func (o *GitFileTreeOverlay) loadBranchesCmd() tea.Cmd {
	return func() tea.Msg {
		if o.adapter == nil {
			return branchesLoadedMsg{err: errors.New("git adapter not configured")}
		}
		branches, err := o.adapter.GetBranches(o.repoPath)
		return branchesLoadedMsg{branches: branches, err: err}
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

type commitsLoadedMsg struct {
	commits []commitEntry
	err     error
}

type stashLoadedMsg struct {
	entries []stashEntry
	err     error
}

type branchesLoadedMsg struct {
	branches []git.Branch
	err      error
}

type autoRefreshMsg struct{}

// --- External helpers ---

func loadCommits(repoPath string) ([]commitEntry, error) {
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

func statusIconForNode(node *GitFileTreeNode) (string, string) {
	if node.IsDir {
		switch node.Status {
		case gitNodeStaged:
			return "A", "#34D399"
		case gitNodeUnstaged:
			return "M", "#FBBF24"
		case gitNodeUntracked:
			return "?", "#6B7280"
		case gitNodeConflicted:
			return "!", "#EF4444"
		case gitNodeMixed:
			// Show the most prevalent status or a summary
			if node.HasConflicted {
				return "!", "#EF4444"
			}
			if node.HasStaged && node.HasUnstaged {
				return "~", "#FBBF24"
			}
			if node.HasStaged {
				return "A", "#34D399"
			}
			if node.HasUnstaged {
				return "M", "#FBBF24"
			}
			if node.HasUntracked {
				return "?", "#6B7280"
			}
		}
		return "", ""
	}

	switch node.Status {
	case gitNodeStaged:
		return "A", "#34D399"
	case gitNodeUnstaged:
		return "M", "#FBBF24"
	case gitNodeUntracked:
		return "?", "#6B7280"
	case gitNodeConflicted:
		return "!", "#EF4444"
	}
	return "", ""
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

// wrapRenderedItems wraps a list of rendered strings into lines that fit within maxWidth.
// Each item is separated by the given separator string.
func wrapRenderedItems(items []string, maxWidth int, sep string) []string {
	if len(items) == 0 {
		return nil
	}
	var lines []string
	var currentLine string
	sepW := lipgloss.Width(sep)
	for _, item := range items {
		itemW := lipgloss.Width(item)
		if currentLine == "" {
			if itemW > maxWidth {
				lines = append(lines, item)
			} else {
				currentLine = item
			}
		} else {
			lineW := lipgloss.Width(currentLine)
			if lineW+sepW+itemW > maxWidth {
				lines = append(lines, currentLine)
				currentLine = item
			} else {
				currentLine += sep + item
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	branchStyle   = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	upstreamStyle = lipgloss.NewStyle().Foreground(styles.Subtle)
	aheadStyle    = lipgloss.NewStyle().Foreground(styles.Success)
	behindStyle   = lipgloss.NewStyle().Foreground(styles.Warning)
	nameStyle     = lipgloss.NewStyle().Foreground(styles.Text)
	statusStyle   = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Background(styles.Highlight).Bold(true)
	loadingStyle  = lipgloss.NewStyle().Foreground(styles.Subtle)
	emptyStyle    = lipgloss.NewStyle().Foreground(styles.Subtle)
	errorStyle    = lipgloss.NewStyle().Foreground(styles.Overdue)
	hintStyle     = lipgloss.NewStyle().Foreground(styles.Subtle)
	hashStyle     = lipgloss.NewStyle().Foreground(styles.Accent)
	authorStyle   = lipgloss.NewStyle().Foreground(styles.Subtle)
	metaStyle     = lipgloss.NewStyle().Foreground(styles.Subtle)
)
