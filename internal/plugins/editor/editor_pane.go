package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ models.Panel = (*EditorPane)(nil)

const fileWatchInterval = time.Second

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

	width   int
	height  int
	loading bool
	saving  bool
	err     error
	notice  string
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
	content string
	modTime time.Time
	notice  string
	err     error
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

func NewEditorPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, filePath string) *EditorPane {
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

	return &EditorPane{
		id:       id,
		meta:     meta,
		common:   common,
		input:    input,
		filePath: filePath,
		loading:  filePath != "",
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

	switch msg := msg.(type) {
	case editorLoadedMsg:
		p.loading = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.originalContent = normalizeContent(msg.content)
		p.lastDiskModTime = msg.modTime
		p.input.SetValue(p.originalContent)
		p.input.CursorEnd()
		p.dirty = false
		p.changeTick = 0
		p.externalChange = false
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
		case tea.KeyEsc:
			if p.dirty {
				p.confirmClose = true
				p.err = nil
				p.notice = ""
				return p, nil
			}
			return p, closeEditorCmd(p.id)
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
	} else {
		lines = append(lines, p.input.View())
	}

	status := "clean"
	if p.dirty {
		status = fmt.Sprintf("modified · changes %d", p.changeTick)
	}
	hint := fmt.Sprintf("%s · Ctrl+S save · Esc close", status)
	lines = append(lines, lipgloss.NewStyle().Foreground(appstyles.Subtle).Render(hint))

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
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
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
		if len(content) > 512*1024 {
			return editorLoadedMsg{err: errors.New("file too large for editor MVP (limit 512KB)")}
		}
		if !isLikelyText(content) {
			return editorLoadedMsg{err: errors.New("binary file preview is not supported yet")}
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

func isLikelyText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}
