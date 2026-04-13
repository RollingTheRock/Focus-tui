package git

import (
	"errors"
	"fmt"
	"strings"

	"focus/internal/adapters"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ models.Panel = (*DiffPane)(nil)

type OpenDiffMsg struct {
	FilePath string
	Staged   bool
}

type CloseDiffMsg struct {
	ID models.PaneID
}

type DiffPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	filePath string
	staged   bool
	diff     string
	scroll   int

	repoPath string
	width    int
	height   int
	loading  bool
	err      error
}

type diffLoadedMsg struct {
	diff string
	err  error
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
	}
}

func (p *DiffPane) Init() tea.Cmd {
	p.loading = p.adapter != nil
	if !p.loading {
		return nil
	}
	return p.loadDiffCmd()
}

func (p *DiffPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case diffLoadedMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.diff = msg.diff
		}
		p.clampScroll()
		return p, nil

	case tea.KeyMsg:
		return p.updateKey(msg)
	}

	return p, nil
}

func (p *DiffPane) View() string {
	width := p.width
	if width <= 0 {
		width = 80
	}

	lines := []string{p.renderHeader()}

	switch {
	case p.loading && p.err == nil && p.diff == "":
		lines = append(lines, renderLoadingLine(width), renderLoadingLine(width), renderLoadingLine(width))
	case p.err != nil:
		lines = append(lines, errorStyle.MaxWidth(width).Render("Unable to load diff: "+p.err.Error()))
	default:
		content := p.visibleContent()
		if len(content) == 0 {
			lines = append(lines, emptyStyle.Render("No diff available."))
		} else {
			lines = append(lines, content...)
		}
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}

	for i := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
	}

	return strings.Join(lines, "\n")
}

func (p *DiffPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.clampScroll()
}

func (p *DiffPane) updateKey(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return p, closeDiffCmd(p.id)
	case "j", "down":
		if p.scroll < p.maxScroll() {
			p.scroll++
		}
	case "k", "up":
		if p.scroll > 0 {
			p.scroll--
		}
	}

	return p, nil
}

func (p *DiffPane) loadDiffCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return diffLoadedMsg{err: errors.New("git adapter is not configured")}
		}

		diff, err := p.adapter.GetDiff(p.repoPath, p.filePath, p.staged)
		return diffLoadedMsg{diff: diff, err: err}
	}
}

func (p *DiffPane) renderHeader() string {
	mode := "unstaged"
	if p.staged {
		mode = "staged"
	}

	label := "Diff"
	if p.filePath != "" {
		label = fmt.Sprintf("%s · %s", p.filePath, mode)
	} else {
		label = fmt.Sprintf("Review · %s", mode)
	}

	return diffHeaderStyle.Render(label)
}

func (p *DiffPane) visibleContent() []string {
	rendered := p.renderedDiffLines()
	if len(rendered) == 0 {
		return nil
	}

	start := p.scroll
	if start > len(rendered) {
		start = len(rendered)
	}

	if p.height <= 0 {
		return rendered[start:]
	}

	visibleHeight := p.contentHeight()
	end := start + visibleHeight
	if end > len(rendered) {
		end = len(rendered)
	}

	return rendered[start:end]
}

func (p *DiffPane) renderedDiffLines() []string {
	lines := p.diffLines()
	if len(lines) == 0 {
		return nil
	}

	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if len(rendered) > 0 {
				rendered = append(rendered, "")
			}
			rendered = append(rendered, p.renderFileHeaderLine(line))
			continue
		}
		rendered = append(rendered, p.renderDiffLine(line))
	}
	return rendered
}

func (p *DiffPane) diffLines() []string {
	if p.diff == "" {
		return nil
	}

	trimmed := strings.TrimRight(p.diff, "\n")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "\n")
}

func (p *DiffPane) renderDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return addedLineStyle.Render(line)
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return removedLineStyle.Render(line)
	case strings.HasPrefix(line, "@@ "):
		return hunkHeaderStyle.Render(line)
	case isDiffHeaderLine(line):
		return diffHeaderStyle.Render(line)
	default:
		return line
	}
}

func (p *DiffPane) renderFileHeaderLine(line string) string {
	label := line
	if filePath, ok := parseDiffFilePath(line); ok {
		label = fmt.Sprintf("File · %s", filePath)
	}
	return diffFileStyle.Render(label)
}

func (p *DiffPane) contentHeight() int {
	if p.height <= 1 {
		return 1
	}
	return p.height - 1
}

func (p *DiffPane) maxScroll() int {
	count := len(p.diffLines())
	if count == 0 {
		return 0
	}

	if p.height <= 0 {
		return count - 1
	}

	maxScroll := count - p.contentHeight()
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (p *DiffPane) clampScroll() {
	if p.scroll < 0 {
		p.scroll = 0
	}
	if p.scroll > p.maxScroll() {
		p.scroll = p.maxScroll()
	}
}

func closeDiffCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg {
		return CloseDiffMsg{ID: id}
	}
}

func isDiffHeaderLine(line string) bool {
	for _, prefix := range []string{"diff --git ", "index ", "@@ ", "--- ", "+++ ", "rename from ", "rename to ", "new file mode ", "deleted file mode ", "similarity index ", "Binary files "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	return false
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
