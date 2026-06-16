package git

import (
	"errors"
	"fmt"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/adapters"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	editorplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/editor"
	"github.com/RollingTheRock/Focus-tui/internal/plugins/git/diffview"
	appstyles "github.com/RollingTheRock/Focus-tui/internal/styles"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

var _ models.Panel = (*DiffPane)(nil)

type OpenDiffMsg struct {
	FilePath string
	Staged   bool
}

type CloseDiffMsg struct {
	ID models.PaneID
}

type diffFile struct {
	path   string
	before string
	after  string
}

type DiffPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	filePath string
	staged   bool

	repoPath string
	width    int
	height   int
	scroll   int

	// review mode data
	files       []diffFile
	reviewIndex int

	// diff view layout: "unified" or "split"
	layout string

	loading bool
	spinner spinner.Model
	err     error

	vp viewport.Model

	// Render cache: diff rendering + syntax highlighting is expensive.
	// We cache the rendered lines and only recompute when files/layout/width change.
	renderedLines   []string
	cacheValid      bool
	cacheWidth      int
	cacheLayout     string
}

type diffLoadedMsg struct {
	files []diffFile
	err   error
}

func NewDiffPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter, filePath string, staged bool) *DiffPane {
	repoPath := meta.CWD
	if repoPath == "" {
		repoPath = "."
	}

	return &DiffPane{
		id:       id,
		meta:     meta,
		common:   common,
		adapter:  adapter,
		filePath: filePath,
		staged:   staged,
		repoPath: repoPath,
		layout:   "unified",
		vp:       viewport.New(),
	}
}

func (p *DiffPane) Init() tea.Cmd {
	p.loading = p.adapter != nil
	if !p.loading {
		return nil
	}
	p.spinner = spinner.New()
	p.spinner.Spinner = spinner.Dot
	p.spinner.Style = lipgloss.NewStyle().Foreground(appstyles.Accent)
	p.vp.SetWidth(p.width)
	p.vp.SetHeight(p.contentHeight())
	return tea.Batch(p.loadDiffCmd(), p.spinner.Tick)
}

func (p *DiffPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case diffLoadedMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.files = msg.files
		}
		p.cacheValid = false
		p.refreshViewportContent()
		return p, nil

	case spinner.TickMsg:
		if !p.loading {
			return p, nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd

	case tea.KeyPressMsg:
		return p.updateKey(msg)
	case tea.MouseWheelMsg:
		return p.updateMouse(msg)
	}

	return p, nil
}

func (p *DiffPane) refreshViewportContent() {
	yOffset := p.scroll
	p.vp.SetContent(strings.Join(p.renderedDiffLines(), "\n"))
	p.vp.SetYOffset(yOffset)
	p.scroll = p.vp.YOffset()
}

func (p *DiffPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 80
	}

	var b strings.Builder
	b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(p.renderHeader()))

	switch {
	case p.loading && p.err == nil && len(p.files) == 0:
		b.WriteByte('\n')
		b.WriteString(p.spinner.View())
	case p.err != nil:
		b.WriteByte('\n')
		b.WriteString(errorStyle.MaxWidth(width).Render("Unable to load diff: " + p.err.Error()))
	default:
		p.vp.SetWidth(width)
		p.vp.SetHeight(p.contentHeight())
		p.vp.SetYOffset(p.scroll)
		content := p.vp.View()
		if content == "" {
			b.WriteByte('\n')
			b.WriteString(emptyStyle.Render("No diff available."))
		} else {
			b.WriteByte('\n')
			b.WriteString(content)
		}
	}

	return tea.NewView(b.String())
}

func (p *DiffPane) SetSize(width, height int) {
	if p.width == width && p.height == height {
		return
	}
	p.width = width
	p.height = height
	p.vp.SetWidth(width)
	p.vp.SetHeight(p.contentHeight())
	if len(p.files) > 0 {
		p.refreshViewportContent()
	} else {
		p.scroll = p.vp.YOffset()
	}
}

func (p *DiffPane) updateKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "q", "esc":
		return p, closeDiffCmd(p.id)
	case "enter":
		if path := p.currentFilePath(); path != "" {
			return p, openEditorCmd(path, p.currentTargetLine())
		}
	case "s":
		p.staged = !p.staged
		p.scroll = 0
		p.vp.SetYOffset(0)
		p.loading = true
		p.err = nil
		p.files = nil
		p.reviewIndex = 0
		return p, p.loadDiffCmd()
	case "v":
		if p.layout == "unified" {
			p.layout = "split"
		} else {
			p.layout = "unified"
		}
		p.refreshViewportContent()
	case "]":
		p.jumpFile(1)
	case "[":
		p.jumpFile(-1)
	default:
		if isScrollKey(msg) {
			p.vp.SetYOffset(p.scroll)
			var cmd tea.Cmd
			p.vp, cmd = p.vp.Update(msg)
			p.scroll = p.vp.YOffset()
			return p, cmd
		}
	}

	return p, nil
}

func isScrollKey(msg tea.KeyPressMsg) bool {
	switch msg.Keystroke() {
	case "j", "k", "up", "down", "pgup", "pgdown", "home", "end", "ctrl+d", "ctrl+u":
		return true
	}
	return false
}

func (p *DiffPane) updateMouse(msg tea.MouseWheelMsg) (models.Panel, tea.Cmd) {
	p.vp.SetYOffset(p.scroll)
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	p.scroll = p.vp.YOffset()
	return p, cmd
}

func (p *DiffPane) loadDiffCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return diffLoadedMsg{err: errors.New("git adapter is not configured")}
		}

		if p.filePath != "" {
			files, err := p.loadSingleFile(p.filePath)
			return diffLoadedMsg{files: files, err: err}
		}

		// Review mode: use GetDiff to discover changed files, then load each
		diff, err := p.adapter.GetDiff(p.repoPath, "", p.staged)
		if err != nil {
			return diffLoadedMsg{err: err}
		}

		paths := extractDiffFilePaths(diff)
		if len(paths) == 0 {
			return diffLoadedMsg{files: nil}
		}

		var files []diffFile
		for _, path := range paths {
			f, err := p.loadSingleFile(path)
			if err != nil {
				return diffLoadedMsg{err: err}
			}
			files = append(files, f...)
		}
		return diffLoadedMsg{files: files}
	}
}

func (p *DiffPane) loadSingleFile(path string) ([]diffFile, error) {
	beforeRef := "HEAD"
	afterRef := ""
	if p.staged {
		afterRef = ":0"
	}

	before, _ := p.adapter.GetFileContent(p.repoPath, path, beforeRef)
	after, _ := p.adapter.GetFileContent(p.repoPath, path, afterRef)

	return []diffFile{{path: path, before: before, after: after}}, nil
}

func extractDiffFilePaths(diff string) []string {
	var paths []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if path, ok := parseDiffFilePath(line); ok {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func (p *DiffPane) renderedDiffLines() []string {
	if len(p.files) == 0 {
		return nil
	}

	if p.cacheValid && p.cacheWidth == p.width && p.cacheLayout == p.layout {
		return p.renderedLines
	}

	var result []string
	for i, f := range p.files {
		if i > 0 {
			result = append(result, "")
		}
		dv := diffview.New().
			Before(f.path, f.before).
			After(f.path, f.after).
			FileName(f.path).
			Width(p.width).
			Height(0).
			ChromaStyle(chromastyles.Get("catppuccin-macchiato"))
		if p.layout == "split" {
			dv.Split()
		}
		result = append(result, strings.Split(dv.String(), "\n")...)
	}

	p.renderedLines = result
	p.cacheValid = true
	p.cacheWidth = p.width
	p.cacheLayout = p.layout
	return result
}

func (p *DiffPane) renderHeader() string {
	mode := "unstaged"
	if p.staged {
		mode = "staged"
	}

	layoutLabel := ""
	if p.layout == "split" {
		layoutLabel = " · split"
	}

	if p.filePath != "" {
		return diffHeaderStyle.Render(fmt.Sprintf("%s · %s%s", p.filePath, mode, layoutLabel))
	}

	if len(p.files) > 0 {
		current := p.currentFilePath()
		if current != "" {
			return diffHeaderStyle.Render(fmt.Sprintf("%s · %d/%d · %s%s", current, p.reviewIndex+1, len(p.files), mode, layoutLabel))
		}
	}

	return diffHeaderStyle.Render(fmt.Sprintf("Review · %s%s", mode, layoutLabel))
}

func (p *DiffPane) contentHeight() int {
	if p.height <= 1 {
		return 1
	}
	return p.height - 1
}

func closeDiffCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg {
		return CloseDiffMsg{ID: id}
	}
}

func openEditorCmd(path string, lineNumber int) tea.Cmd {
	return func() tea.Msg {
		return editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault, LineNumber: lineNumber}
	}
}

func parseDiffFilePath(line string) (string, bool) {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	parts := strings.Fields(strings.TrimPrefix(line, prefix))
	if len(parts) < 2 {
		return "", false
	}
	right := strings.TrimPrefix(parts[1], "b/")
	if right == "" {
		return strings.TrimPrefix(parts[0], "a/"), true
	}
	return right, true
}

func (p *DiffPane) currentFilePath() string {
	if p.filePath != "" {
		return p.filePath
	}
	if p.reviewIndex >= 0 && p.reviewIndex < len(p.files) {
		return p.files[p.reviewIndex].path
	}
	return ""
}

func (p *DiffPane) currentTargetLine() int {
	// Return the starting line number of the current file's first hunk.
	if p.reviewIndex >= 0 && p.reviewIndex < len(p.files) {
		f := p.files[p.reviewIndex]
		return firstHunkLine(f.before, f.after)
	}
	return 1
}

func firstHunkLine(before, after string) int {
	edits := diffview.New().Before("", before).After("", after)
	_ = edits // TODO: expose hunk info from diffview if needed
	return 1
}

func (p *DiffPane) jumpFile(delta int) {
	if len(p.files) <= 1 {
		return
	}
	p.reviewIndex += delta
	if p.reviewIndex < 0 {
		p.reviewIndex = 0
	}
	if p.reviewIndex >= len(p.files) {
		p.reviewIndex = len(p.files) - 1
	}
	p.scrollToFile(p.reviewIndex)
}

func (p *DiffPane) scrollToFile(index int) {
	lines := p.renderedDiffLines()
	fileCount := 0
	for i, line := range lines {
		if strings.Contains(line, "@@") {
			if fileCount == index {
				p.scroll = i
				p.vp.SetYOffset(i)
				return
			}
			fileCount++
		}
	}
}
