package git

import (
	"errors"
	"fmt"
	"strings"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ models.Panel = (*CommitPane)(nil)

type OpenCommitMsg struct {
	RepoPath    string
	StagedFiles []gitmodel.File
}

type CommitCompletedMsg struct {
	ID       models.PaneID
	RepoPath string
}

type CloseCommitMsg struct {
	ID models.PaneID
}

type CommitPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	input       textarea.Model
	stagedFiles []gitmodel.File

	repoPath   string
	width      int
	height     int
	submitting bool
	err        error
}

type commitFinishedMsg struct {
	err error
}

func NewCommitPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter, stagedFiles []gitmodel.File) *CommitPane {
	repoPath := meta.CWD
	if repoPath == "" {
		repoPath = "."
	}

	input := textarea.New()
	input.Placeholder = "Subject on the first line, details below"
	input.Prompt = "│ "
	input.ShowLineNumbers = false

	focusedStyle, blurredStyle := textarea.DefaultStyles()
	focusedStyle.Base = lipgloss.NewStyle().Foreground(appstyles.Text)
	focusedStyle.CursorLine = lipgloss.NewStyle()
	focusedStyle.Prompt = lipgloss.NewStyle().Foreground(appstyles.Accent)
	focusedStyle.Placeholder = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	focusedStyle.Text = lipgloss.NewStyle().Foreground(appstyles.Text)
	blurredStyle = focusedStyle
	blurredStyle.Prompt = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	input.FocusedStyle = focusedStyle
	input.BlurredStyle = blurredStyle
	input.SetWidth(56)
	input.SetHeight(6)

	files := append([]gitmodel.File(nil), stagedFiles...)

	return &CommitPane{
		id:          id,
		meta:        meta,
		common:      common,
		adapter:     adapter,
		input:       input,
		stagedFiles: files,
		repoPath:    repoPath,
	}
}

func (p *CommitPane) Init() tea.Cmd {
	return tea.Batch(p.input.Focus(), textarea.Blink)
}

func (p *CommitPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case commitFinishedMsg:
		p.submitting = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}

		p.err = nil
		return p, commitCompletedCmd(p.id, p.repoPath)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return p, closeCommitCmd(p.id)
		case tea.KeyCtrlS:
			return p.submit()
		case tea.KeyCtrlJ:
			return p.submit()
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *CommitPane) View() string {
	width := p.width
	if width <= 0 {
		width = 60
	}

	lines := []string{commitHeaderStyle.Render("Commit")}
	if len(p.stagedFiles) == 0 {
		lines = append(lines, emptyStyle.Render("No staged files to commit."))
	} else {
		lines = append(lines, upstreamStyle.Render(fmt.Sprintf("%d staged file(s)", len(p.stagedFiles))))
		lines = append(lines, p.renderStagedFiles()...)
	}

	lines = append(lines, "", commitInputStyle.Render(p.input.View()), p.renderHint())
	if p.submitting {
		lines = append(lines, upstreamStyle.Render("Committing..."))
	}
	if p.err != nil {
		lines = append(lines, errorStyle.Render("Commit failed: "+p.err.Error()))
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}

	for i := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
	}

	return strings.Join(lines, "\n")
}

func (p *CommitPane) SetSize(width, height int) {
	p.width = width
	p.height = height

	inputWidth := width - 6
	if inputWidth < 20 {
		inputWidth = 20
	}
	p.input.SetWidth(inputWidth)

	inputHeight := 6
	if height > 0 {
		inputHeight = max(3, min(8, height/3))
	}
	p.input.SetHeight(inputHeight)
}

func (p *CommitPane) renderStagedFiles() []string {
	lines := make([]string, 0, len(p.stagedFiles))
	for _, file := range p.stagedFiles {
		icon := renderStatusIcon(statusIcon(file.StagedStatus, "M"))
		lines = append(lines, "  "+icon+" "+commitFileStyle.Render(fileLabel(file)))
	}
	return lines
}

func (p *CommitPane) renderHint() string {
	subjectLength := len([]rune(p.subject()))
	hint := fmt.Sprintf("Subject %d/50 chars · Ctrl+S commit · Ctrl+J fallback · Esc cancel", subjectLength)
	if subjectLength > 50 {
		return commitHintWarningStyle.Render(hint)
	}
	return commitHintStyle.Render(hint)
}

func (p *CommitPane) subject() string {
	message := strings.ReplaceAll(p.input.Value(), "\r\n", "\n")
	parts := strings.SplitN(message, "\n", 2)
	return strings.TrimSpace(parts[0])
}

func (p *CommitPane) submit() (models.Panel, tea.Cmd) {
	if p.submitting {
		return p, nil
	}
	if len(p.stagedFiles) == 0 {
		p.err = errors.New("no staged changes to commit")
		return p, nil
	}

	message := strings.TrimSpace(p.input.Value())
	if message == "" {
		p.err = errors.New("commit message cannot be empty")
		return p, nil
	}
	if p.adapter == nil {
		p.err = errors.New("git adapter is not configured")
		return p, nil
	}

	p.submitting = true
	p.err = nil
	return p, p.commitCmd(message)
}

func (p *CommitPane) commitCmd(message string) tea.Cmd {
	return func() tea.Msg {
		return commitFinishedMsg{err: p.adapter.Commit(p.repoPath, message)}
	}
}

func commitCompletedCmd(id models.PaneID, repoPath string) tea.Cmd {
	return func() tea.Msg {
		return CommitCompletedMsg{ID: id, RepoPath: repoPath}
	}
}

func closeCommitCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg {
		return CloseCommitMsg{ID: id}
	}
}
