package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ models.Panel = (*EditorPane)(nil)

const fileWatchInterval = time.Second

const (
	maxEditableBytes = 512 * 1024
	maxPreviewBytes  = 8 * 1024
	maxBinaryPreview = 256
)

type EditorPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	input textarea.Model

	filePath        string
	originalContent string
	dirty           bool
	changeTick      int64
	confirmClose    bool
	lastDiskModTime time.Time
	externalChange  bool
	readOnly        bool
	previewMode     bool
	previewReason   string
	previewContent  string
	previewLines    []string
	previewScroll   int

	mode        editorMode
	miniInput   textinput.Model
	searchQuery string
	searchHits  []cursorTarget
	searchIndex int
	initialLine int

	width   int
	height  int
	loading bool
	saving  bool
	err     error
	notice  string
}

type editorMode string

const (
	editorModeNormal editorMode = "normal"
	editorModeSearch editorMode = "search"
	editorModeJump   editorMode = "jump"
)

type cursorTarget struct {
	line   int
	column int
}

type previewMatch struct {
	start  int
	end    int
	active bool
}

func (p *EditorPane) FilePath() string { return p.filePath }

func (p *EditorPane) Dirty() bool { return p.dirty }

func (p *EditorPane) DisplayName() string {
	if p.filePath == "" {
		if p.meta.Name != "" {
			return p.meta.Name
		}
		return "untitled"
	}
	return filepath.Base(p.filePath)
}

type editorLoadedMsg struct {
	content        string
	modTime        time.Time
	notice         string
	readOnly       bool
	previewMode    bool
	previewReason  string
	previewContent string
	err            error
}

type saveFinishedMsg struct {
	modTime time.Time
	err     error
}

type fileWatchTickMsg struct{}

type externalFileStateMsg struct {
	modTime time.Time
	err     error
}

func NewEditorPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, filePath string, initialLine int) *EditorPane {
	input := textarea.New()
	input.Placeholder = "Start typing..."
	input.Prompt = ""
	input.ShowLineNumbers = true
	focusedStyle, blurredStyle := textarea.DefaultStyles()
	focusedStyle.Base = lipgloss.NewStyle().Foreground(appstyles.Text)
	focusedStyle.CursorLine = lipgloss.NewStyle().Background(appstyles.Highlight)
	focusedStyle.LineNumber = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	focusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(appstyles.Accent)
	focusedStyle.Placeholder = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	focusedStyle.Text = lipgloss.NewStyle().Foreground(appstyles.Text)
	blurredStyle = focusedStyle
	input.FocusedStyle = focusedStyle
	input.BlurredStyle = blurredStyle
	input.SetWidth(60)
	input.SetHeight(12)
	miniInput := textinput.New()
	miniInput.Prompt = ""
	miniInput.CharLimit = 256
	miniInput.Blur()

	return &EditorPane{
		id:          id,
		meta:        meta,
		common:      common,
		input:       input,
		miniInput:   miniInput,
		mode:        editorModeNormal,
		filePath:    filePath,
		initialLine: initialLine,
		loading:     filePath != "",
	}
}

func (p *EditorPane) Init() tea.Cmd {
	cmds := []tea.Cmd{p.input.Focus(), textarea.Blink}
	if p.filePath != "" {
		cmds = append(cmds, p.loadFileCmd(fileOpenedNotice(p.filePath)), p.watchFileCmd())
	}
	return tea.Batch(cmds...)
}

func (p *EditorPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	if p.confirmClose {
		if key, ok := msg.(tea.KeyMsg); ok {
			return p.updateCloseConfirm(key)
		}
	}
	if p.mode != editorModeNormal {
		if key, ok := msg.(tea.KeyMsg); ok {
			return p.updateMiniInput(key)
		}
	}

	switch msg := msg.(type) {
	case OpenEditorMsg:
		if msg.LineNumber > 0 {
			return p.executeJump(strconv.Itoa(msg.LineNumber))
		}
		return p, nil

	case editorLoadedMsg:
		p.loading = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.originalContent = normalizeContent(msg.content)
		p.lastDiskModTime = msg.modTime
		p.readOnly = msg.readOnly
		p.previewMode = msg.previewMode
		p.previewReason = msg.previewReason
		p.previewContent = msg.previewContent
		p.previewLines = splitPreviewLines(msg.previewContent)
		p.previewScroll = 0
		p.input.SetValue(p.originalContent)
		p.input.CursorEnd()
		p.dirty = false
		p.changeTick = 0
		p.externalChange = false
		if p.initialLine > 0 {
			_, _ = p.executeJump(strconv.Itoa(p.initialLine))
			p.initialLine = 0
		}
		p.err = nil
		p.notice = msg.notice
		return p, nil

	case saveFinishedMsg:
		p.saving = false
		if msg.err != nil {
			p.err = msg.err
			p.notice = ""
			return p, nil
		}
		p.originalContent = normalizeContent(p.input.Value())
		p.lastDiskModTime = msg.modTime
		p.dirty = false
		p.externalChange = false
		p.err = nil
		p.notice = fmt.Sprintf("saved %s", filepath.Base(p.filePath))
		return p, saveCompletedCmd(p.id, p.filePath, p.dirty)

	case fileWatchTickMsg:
		if p.filePath == "" {
			return p, nil
		}
		return p, tea.Batch(p.checkExternalFileCmd(), p.watchFileCmd())

	case externalFileStateMsg:
		if p.filePath == "" || p.loading || p.saving {
			return p, nil
		}
		if msg.err != nil {
			p.err = msg.err
			p.notice = ""
			return p, nil
		}
		if !p.lastDiskModTime.IsZero() && !msg.modTime.After(p.lastDiskModTime) {
			return p, nil
		}
		if p.dirty {
			p.externalChange = true
			p.err = nil
			p.notice = "file changed on disk; save will overwrite newer content"
			return p, nil
		}
		p.loading = true
		p.err = nil
		p.notice = ""
		return p, p.loadFileCmd(fileReloadedNotice(p.filePath))

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlS:
			return p.submitSave()
		case tea.KeyCtrlF:
			return p.startSearchMode()
		case tea.KeyEsc:
			if p.dirty {
				p.confirmClose = true
				p.err = nil
				p.notice = ""
				return p, nil
			}
			return p, closeEditorCmd(p.id)
		}
		switch msg.String() {
		case "/":
			return p.startSearchMode()
		case ":":
			return p.startJumpMode()
		case "n":
			return p.advanceSearch(1)
		case "N":
			return p.advanceSearch(-1)
		}
		if p.readOnly {
			return p.updatePreviewKey(msg)
		}
	}

	before := normalizeContent(p.input.Value())
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	after := normalizeContent(p.input.Value())
	if before != after {
		p.changeTick++
		p.dirty = after != p.originalContent
		p.notice = ""
		if p.err != nil {
			p.err = nil
		}
	}
	return p, cmd
}

func (p *EditorPane) View() string {
	width := p.width
	if width <= 0 {
		width = 60
	}

	name := p.filePath
	if name == "" {
		name = p.meta.Name
	}
	if name == "" {
		name = "untitled"
	}

	header := lipgloss.NewStyle().Foreground(appstyles.Accent).Bold(true).Render(name)
	lines := []string{header}
	if p.loading {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("Loading file..."))
	} else if p.previewMode {
		if p.previewReason != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Warning).Render(p.previewReason))
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Text).Render(p.visiblePreviewContent()))
	} else {
		lines = append(lines, p.input.View())
	}

	status := "clean"
	if p.readOnly {
		status = "read-only preview"
	}
	if p.dirty {
		status = fmt.Sprintf("modified · changes %d", p.changeTick)
	}
	hint := fmt.Sprintf("%s · Ctrl+S save · Esc close", status)
	if p.readOnly {
		hint = fmt.Sprintf("%s · j/k scroll · / search · : line · n/N result · Esc close", status)
	} else {
		hint = fmt.Sprintf("%s · Ctrl+S save · Ctrl+F search · : line · n/N result · Esc close", status)
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Subtle).Render(hint))
	if p.mode != editorModeNormal {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Accent).Render(p.miniInput.View()))
	}
	if status := p.searchStatusLine(); status != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Accent).Render(status))
	}

	if p.confirmClose {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Warning).Render("Discard unsaved changes? [y/n]"))
	}
	if p.externalChange {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Warning).Render("Disk changed outside editor; reload or save carefully"))
	}
	if p.notice != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Success).Render(p.notice))
	}
	if p.err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Overdue).Render("Editor error: "+p.err.Error()))
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = appstyles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (p *EditorPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	inputWidth := width - 4
	if inputWidth < 20 {
		inputWidth = 20
	}
	p.input.SetWidth(inputWidth)
	inputHeight := height - 4
	if inputHeight < 4 {
		inputHeight = 4
	}
	p.input.SetHeight(inputHeight)
}

func (p *EditorPane) loadFileCmd(notice string) tea.Cmd {
	path := p.filePath
	return func() tea.Msg {
		content, err := os.ReadFile(path)
		if err != nil {
			return editorLoadedMsg{err: err}
		}
		info, err := os.Stat(path)
		if err != nil {
			return editorLoadedMsg{err: err}
		}
		if len(content) > maxEditableBytes {
			previewContent := string(content[:minInt(len(content), maxPreviewBytes)])
			return editorLoadedMsg{
				content:        previewContent,
				modTime:        info.ModTime(),
				notice:         notice,
				readOnly:       true,
				previewMode:    true,
				previewReason:  fmt.Sprintf("Large file preview (%d bytes). Editing is disabled.", len(content)),
				previewContent: truncateWithNotice(normalizeContent(previewContent), len(content) > maxPreviewBytes, "\n\n[preview truncated]"),
			}
		}
		if !isLikelyText(content) {
			preview := renderBinaryPreview(content)
			return editorLoadedMsg{
				content:        preview,
				modTime:        info.ModTime(),
				notice:         notice,
				readOnly:       true,
				previewMode:    true,
				previewReason:  "Binary file preview. Editing is disabled.",
				previewContent: preview,
			}
		}
		return editorLoadedMsg{content: string(content), modTime: info.ModTime(), notice: notice}
	}
}

func (p *EditorPane) submitSave() (models.Panel, tea.Cmd) {
	if p.loading || p.saving {
		return p, nil
	}
	if p.filePath == "" {
		p.err = errors.New("cannot save file without a path")
		p.notice = ""
		return p, nil
	}
	if p.readOnly {
		p.err = nil
		p.notice = "preview mode is read-only"
		return p, nil
	}
	content := normalizeContent(p.input.Value())
	if content == p.originalContent {
		p.notice = "no changes to save"
		p.err = nil
		return p, nil
	}
	p.saving = true
	p.err = nil
	p.notice = ""
	return p, p.saveFileCmd(content)
}

func (p *EditorPane) saveFileCmd(content string) tea.Cmd {
	path := p.filePath
	return func() tea.Msg {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return saveFinishedMsg{err: err}
		}
		info, err := os.Stat(path)
		if err != nil {
			return saveFinishedMsg{err: err}
		}
		return saveFinishedMsg{modTime: info.ModTime()}
	}
}

func (p *EditorPane) watchFileCmd() tea.Cmd {
	return tea.Tick(fileWatchInterval, func(time.Time) tea.Msg {
		return fileWatchTickMsg{}
	})
}

func (p *EditorPane) checkExternalFileCmd() tea.Cmd {
	path := p.filePath
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return externalFileStateMsg{err: err}
		}
		return externalFileStateMsg{modTime: info.ModTime()}
	}
}

func (p *EditorPane) updateCloseConfirm(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		p.confirmClose = false
		return p, closeEditorCmd(p.id)
	case "n", "esc":
		p.confirmClose = false
		return p, nil
	default:
		return p, nil
	}
}

func closeEditorCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return CloseEditorMsg{ID: id} }
}

func saveCompletedCmd(id models.PaneID, filePath string, dirty bool) tea.Cmd {
	return func() tea.Msg { return SaveCompletedMsg{ID: id, FilePath: filePath, Dirty: dirty} }
}

func normalizeContent(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func fileOpenedNotice(path string) string {
	return fmt.Sprintf("opened %s", filepath.Base(path))
}

func fileReloadedNotice(path string) string {
	return fmt.Sprintf("reloaded %s after external change", filepath.Base(path))
}

func (p *EditorPane) startSearchMode() (models.Panel, tea.Cmd) {
	p.mode = editorModeSearch
	p.miniInput.SetValue(p.searchQuery)
	p.miniInput.Prompt = "/ "
	p.miniInput.Placeholder = "search"
	return p, p.miniInput.Focus()
}

func (p *EditorPane) startJumpMode() (models.Panel, tea.Cmd) {
	p.mode = editorModeJump
	p.miniInput.SetValue("")
	p.miniInput.Prompt = ": "
	p.miniInput.Placeholder = "line number"
	return p, p.miniInput.Focus()
}

func (p *EditorPane) updateMiniInput(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		p.mode = editorModeNormal
		p.miniInput.Blur()
		return p, nil
	case tea.KeyEnter:
		value := strings.TrimSpace(p.miniInput.Value())
		p.mode = editorModeNormal
		p.miniInput.Blur()
		if value == "" {
			p.notice = ""
			return p, nil
		}
		if p.miniInput.Prompt == "/ " {
			return p.executeSearch(value)
		}
		return p.executeJump(value)
	}
	var cmd tea.Cmd
	p.miniInput, cmd = p.miniInput.Update(msg)
	return p, cmd
}

func (p *EditorPane) executeSearch(query string) (models.Panel, tea.Cmd) {
	p.searchQuery = query
	p.searchHits = p.findSearchHits(query)
	p.searchIndex = 0
	if len(p.searchHits) == 0 {
		p.notice = fmt.Sprintf("no matches for %q", query)
		return p, nil
	}
	target := p.searchHits[p.searchIndex]
	p.notice = fmt.Sprintf("match %d/%d for %q at line %d, col %d", p.searchIndex+1, len(p.searchHits), query, target.line+1, target.column+1)
	p.moveToTarget(target)
	return p, nil
}

func (p *EditorPane) executeJump(value string) (models.Panel, tea.Cmd) {
	lineNumber, err := strconv.Atoi(value)
	if err != nil || lineNumber <= 0 {
		p.notice = fmt.Sprintf("invalid line %q", value)
		return p, nil
	}
	target := cursorTarget{line: lineNumber - 1, column: 0}
	if p.previewMode {
		if target.line >= len(p.previewLines) {
			p.notice = fmt.Sprintf("line %d out of range", lineNumber)
			return p, nil
		}
	} else if target.line >= len(strings.Split(normalizeContent(p.input.Value()), "\n")) {
		p.notice = fmt.Sprintf("line %d out of range", lineNumber)
		return p, nil
	}
	p.moveToTarget(target)
	p.notice = fmt.Sprintf("jumped to line %d", lineNumber)
	return p, nil
}

func (p *EditorPane) findSearchHits(query string) []cursorTarget {
	needle := strings.ToLower(query)
	lines := p.searchableLines()
	hits := make([]cursorTarget, 0)
	for lineIdx, line := range lines {
		lower := strings.ToLower(line)
		for start := strings.Index(lower, needle); start >= 0; {
			hits = append(hits, cursorTarget{line: lineIdx, column: start})
			nextStart := start + len(needle)
			if nextStart >= len(lower) {
				break
			}
			rest := lower[nextStart:]
			offset := strings.Index(rest, needle)
			if offset < 0 {
				break
			}
			start = nextStart + offset
		}
	}
	return hits
}

func (p *EditorPane) advanceSearch(delta int) (models.Panel, tea.Cmd) {
	if len(p.searchHits) == 0 {
		return p, nil
	}
	p.searchIndex = (p.searchIndex + delta + len(p.searchHits)) % len(p.searchHits)
	target := p.searchHits[p.searchIndex]
	p.moveToTarget(target)
	p.notice = fmt.Sprintf("match %d/%d for %q at line %d, col %d", p.searchIndex+1, len(p.searchHits), p.searchQuery, target.line+1, target.column+1)
	return p, nil
}

func (p *EditorPane) searchableLines() []string {
	if p.previewMode {
		return p.previewLines
	}
	return strings.Split(normalizeContent(p.input.Value()), "\n")
}

func (p *EditorPane) moveToTarget(target cursorTarget) {
	if p.previewMode {
		p.previewScroll = clampInt(target.line, 0, p.maxPreviewScroll())
		return
	}
	currentLine := p.input.Line()
	for currentLine > target.line {
		p.input.CursorUp()
		currentLine--
	}
	for currentLine < target.line {
		p.input.CursorDown()
		currentLine++
	}
	p.input.CursorStart()
}

func (p *EditorPane) updatePreviewKey(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		p.previewScroll = clampInt(p.previewScroll+1, 0, p.maxPreviewScroll())
	case "k", "up":
		p.previewScroll = clampInt(p.previewScroll-1, 0, p.maxPreviewScroll())
	case "pgdown", "ctrl+f":
		p.previewScroll = clampInt(p.previewScroll+p.previewPageStep(), 0, p.maxPreviewScroll())
	case "pgup", "ctrl+b":
		p.previewScroll = clampInt(p.previewScroll-p.previewPageStep(), 0, p.maxPreviewScroll())
	case "g", "home":
		p.previewScroll = 0
	case "G", "end":
		p.previewScroll = p.maxPreviewScroll()
	}
	return p, nil
}

func (p *EditorPane) visiblePreviewContent() string {
	if len(p.previewLines) == 0 {
		return ""
	}
	height := p.previewViewportHeight()
	start := clampInt(p.previewScroll, 0, maxInt(len(p.previewLines)-1, 0))
	end := start + height
	if end > len(p.previewLines) {
		end = len(p.previewLines)
	}
	activeLine := -1
	if target, ok := p.currentSearchTarget(); ok {
		activeLine = target.line
	}
	visible := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		prefix := "  "
		if idx == activeLine {
			prefix = "› "
		}
		visible = append(visible, prefix+p.renderPreviewLine(idx))
	}
	return strings.Join(visible, "\n")
}

func (p *EditorPane) previewViewportHeight() int {
	height := p.height - 6
	if p.mode != editorModeNormal {
		height--
	}
	if p.previewReason != "" {
		height--
	}
	if height < 1 {
		height = 1
	}
	return height
}

func (p *EditorPane) maxPreviewScroll() int {
	return maxInt(len(p.previewLines)-p.previewViewportHeight(), 0)
}

func (p *EditorPane) previewPageStep() int {
	step := p.previewViewportHeight() - 1
	if step < 1 {
		step = 1
	}
	return step
}

func (p *EditorPane) searchStatusLine() string {
	if p.searchQuery == "" {
		return ""
	}
	if len(p.searchHits) == 0 {
		return fmt.Sprintf("search %q · 0 matches", p.searchQuery)
	}
	target := p.searchHits[p.searchIndex]
	return fmt.Sprintf("search %q · %d/%d · line %d, col %d", p.searchQuery, p.searchIndex+1, len(p.searchHits), target.line+1, target.column+1)
}

func (p *EditorPane) currentSearchTarget() (cursorTarget, bool) {
	if len(p.searchHits) == 0 || p.searchIndex < 0 || p.searchIndex >= len(p.searchHits) {
		return cursorTarget{}, false
	}
	return p.searchHits[p.searchIndex], true
}

func (p *EditorPane) renderPreviewLine(lineIdx int) string {
	if lineIdx < 0 || lineIdx >= len(p.previewLines) {
		return ""
	}
	line := p.previewLines[lineIdx]
	if p.searchQuery == "" {
		return line
	}
	matches := p.previewLineMatches(lineIdx)
	if len(matches) == 0 {
		return line
	}
	base := lipgloss.NewStyle().Foreground(appstyles.Text)
	match := lipgloss.NewStyle().Foreground(appstyles.Text).Background(appstyles.Highlight)
	activeMatch := lipgloss.NewStyle().Foreground(appstyles.Text).Background(appstyles.Accent).Bold(true)

	var rendered strings.Builder
	pos := 0
	for _, hit := range matches {
		start := hit.start
		end := hit.end
		rendered.WriteString(base.Render(line[pos:start]))
		segment := line[start:end]
		if hit.active {
			rendered.WriteString(activeMatch.Render(segment))
		} else {
			rendered.WriteString(match.Render(segment))
		}
		pos = end
	}
	rendered.WriteString(base.Render(line[pos:]))
	return rendered.String()
}

func (p *EditorPane) previewLineMatches(lineIdx int) []previewMatch {
	if p.searchQuery == "" || lineIdx < 0 || lineIdx >= len(p.previewLines) {
		return nil
	}
	line := p.previewLines[lineIdx]
	needle := strings.ToLower(p.searchQuery)
	if needle == "" {
		return nil
	}
	lower := strings.ToLower(line)
	active, hasActive := p.currentSearchTarget()
	matches := make([]previewMatch, 0)
	for start := strings.Index(lower, needle); start >= 0; {
		end := start + len(needle)
		matches = append(matches, previewMatch{
			start:  start,
			end:    end,
			active: hasActive && active.line == lineIdx && active.column == start,
		})
		if end >= len(lower) {
			break
		}
		rest := lower[end:]
		offset := strings.Index(rest, needle)
		if offset < 0 {
			break
		}
		start = end + offset
	}
	return matches
}

func truncateWithNotice(content string, truncated bool, suffix string) string {
	if !truncated {
		return content
	}
	return content + suffix
}

func renderBinaryPreview(content []byte) string {
	limit := minInt(len(content), maxBinaryPreview)
	lines := []string{fmt.Sprintf("Binary preview (%d bytes shown)", limit)}
	for i := 0; i < limit; i += 16 {
		end := minInt(i+16, limit)
		chunk := content[i:end]
		parts := make([]string, 0, len(chunk))
		for _, b := range chunk {
			parts = append(parts, fmt.Sprintf("%02x", b))
		}
		lines = append(lines, fmt.Sprintf("%04x: %s", i, strings.Join(parts, " ")))
	}
	if len(content) > limit {
		lines = append(lines, "", "[binary preview truncated]")
	}
	return strings.Join(lines, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func splitPreviewLines(content string) []string {
	if content == "" {
		return []string{""}
	}
	return strings.Split(content, "\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isLikelyText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}
