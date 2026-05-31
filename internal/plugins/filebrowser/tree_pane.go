package filebrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focus/internal/models"
	editorplugin "focus/internal/plugins/editor"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const refreshInterval = 8 * time.Second

type TreePane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	root     *FileNode
	cursor   int
	flatList []*FileNode
	cwd      string

	width   int
	height  int
	err     error
	loading bool
}

type FileNode struct {
	Name      string
	Path      string
	IsDir     bool
	Children  []*FileNode
	Parent    *FileNode
	Collapsed bool
	Depth     int
}

type treeRefreshMsg struct {
	cwd  string
	root *FileNode
	err  error
}

type treeTickMsg struct{}

// RefreshTreeMsg triggers an immediate tree refresh.
type RefreshTreeMsg struct{}

func NewTreePane(id models.PaneID, meta models.PaneMeta, common models.CommonModel) *TreePane {
	cwd := meta.CWD
	if cwd == "" {
		cwd = "."
	}

	return &TreePane{
		id:      id,
		meta:    meta,
		common:  common,
		cwd:     cwd,
		loading: true,
	}
}

func (p *TreePane) Init() tea.Cmd {
	p.loading = true
	return tea.Batch(p.refreshTreeCmd(), p.refreshTickCmd())
}

func (p *TreePane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case treeRefreshMsg:
		if msg.cwd != p.cwd {
			return p, nil
		}
		p.applyRefresh(msg)
		return p, nil

	case treeTickMsg:
		return p, tea.Batch(p.refreshTreeCmd(), p.refreshTickCmd())

	case RefreshTreeMsg:
		p.loading = true
		return p, p.refreshTreeCmd()

	case tea.KeyPressMsg:
		return p.updateKey(msg)
	}

	return p, nil
}

func (p *TreePane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 40
	}

	lines := []string{treeHeaderStyle.MaxWidth(width).Render(p.cwd)}

	if p.loading && p.root == nil && p.err == nil {
		lines = append(lines, emptyStyle.Render("Loading file tree..."))
		return tea.NewView(p.fitHeight(lines, width))
	}

	if p.err != nil && p.root == nil {
		lines = append(lines, errorStyle.MaxWidth(width).Render("Unable to load file tree: "+p.err.Error()))
		return tea.NewView(p.fitHeight(lines, width))
	}

	if len(p.flatList) == 0 {
		lines = append(lines, emptyStyle.Render("No files found."))
		return tea.NewView(p.fitHeight(lines, width))
	}

	start, end := p.visibleRange()
	lines = append(lines, metaStyle.Render(fmt.Sprintf("  %d/%d", p.cursor+1, len(p.flatList))))
	if start > 0 {
		lines = append(lines, metaStyle.Render(fmt.Sprintf("  ▲ %d hidden", start)))
	}
	for index := start; index < end; index++ {
		lines = append(lines, p.renderNode(index, p.flatList[index], width))
	}
	if end < len(p.flatList) {
		lines = append(lines, metaStyle.Render(fmt.Sprintf("  ▼ %d hidden", len(p.flatList)-end)))
	}

	return tea.NewView(p.fitHeight(lines, width))
}

func (p *TreePane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *TreePane) updateKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	count := len(p.flatList)
	if count == 0 {
		return p, nil
	}

	switch msg.Keystroke() {
	case "j", "down":
		if p.cursor < count-1 {
			p.cursor++}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "o":
		node := p.selectedNode()
		if node == nil || node.IsDir {
			return p, nil
		}
		return p, openEditorCmd(node.Path, editorplugin.OpenBehaviorDefault)
	case "v":
		node := p.selectedNode()
		if node == nil || node.IsDir {
			return p, nil
		}
		return p, openEditorCmd(node.Path, editorplugin.OpenBehaviorVSplit)
	case "r":
		p.loading = true
		return p, p.refreshTreeCmd()
	case "enter", " ", "right", "left":
		node := p.selectedNode()
		if node == nil {
			return p, nil
		}
		if !node.IsDir {
			if msg.String() == "enter" {
				return p, openEditorCmd(node.Path, editorplugin.OpenBehaviorDefault)
			}
			return p, nil
		}

		if msg.String() == "left" && !node.Collapsed && len(node.Children) > 0 {
			node.Collapsed = true
		} else if msg.String() == "left" && node.Collapsed && node.Parent != nil {
			p.selectPath(node.Parent.Path)
			return p, nil
		} else if msg.String() == "right" && node.Collapsed {
			node.Collapsed = false
		} else {
			node.Collapsed = !node.Collapsed
		}

		selectedPath := node.Path
		p.flatList = flattenVisibleNodes(p.root)
		p.selectPath(selectedPath)
	}

	p.clampCursor()
	return p, nil
}

func openEditorCmd(path string, behavior editorplugin.OpenBehavior) tea.Cmd {
	return func() tea.Msg {
		return editorplugin.OpenEditorMsg{FilePath: path, Behavior: behavior}
	}
}

func (p *TreePane) selectedNode() *FileNode {
	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return nil
	}
	return p.flatList[p.cursor]
}

func (p *TreePane) applyRefresh(msg treeRefreshMsg) {
	p.loading = false
	if msg.err != nil {
		p.err = msg.err
		return
	}

	collapsed := map[string]bool{}
	if p.root != nil {
		collectCollapsed(p.root, collapsed)
	}

	selectedPath := ""
	if node := p.selectedNode(); node != nil {
		selectedPath = node.Path
	}

	applyCollapsed(msg.root, collapsed)
	p.err = nil
	p.root = msg.root
	p.flatList = flattenVisibleNodes(p.root)
	if selectedPath != "" {
		p.selectPath(selectedPath)
	}
	p.clampCursor()
}

func (p *TreePane) refreshTreeCmd() tea.Cmd {
	cwd := p.cwd
	return func() tea.Msg {
		root, err := buildTree(cwd, nil, 0)
		return treeRefreshMsg{cwd: cwd, root: root, err: err}
	}
}

func (p *TreePane) refreshTickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return treeTickMsg{}
	})
}

func (p *TreePane) renderNode(index int, node *FileNode, width int) string {
	prefix := "  "
	if index == p.cursor {
		prefix = "> "
	}

	indent := strings.Repeat("  ", node.Depth)
	marker := "  "
	if node.IsDir {
		if node.Collapsed {
			marker = "▸ "
		} else {
			marker = "▾ "
		}
	}

	line := prefix + indent + marker + renderNodeIcon(node) + " " + nameStyle.Render(node.Name)
	line = ansi.Truncate(line, width, "")
	if index == p.cursor {
		return selectedStyle.Width(width).Render(line)
	}
	return styles.StyleCache.MaxWidth(width).Render(line)
}

func (p *TreePane) fitHeight(lines []string, width int) string {
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = styles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (p *TreePane) visibleRange() (int, int) {
	if p.height <= 3 {
		return 0, len(p.flatList)
	}

	bodyHeight := p.height - 3 // cwd + position + at least one list row
	if bodyHeight <= 0 || len(p.flatList) <= bodyHeight {
		return 0, len(p.flatList)
	}

	start := p.cursor - bodyHeight + 1
	if start < 0 {
		start = 0
	}
	end := start + bodyHeight
	if end > len(p.flatList) {
		end = len(p.flatList)
		start = end - bodyHeight
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func (p *TreePane) clampCursor() {
	if len(p.flatList) == 0 {
		p.cursor = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.flatList) {
		p.cursor = len(p.flatList) - 1
	}
}

func (p *TreePane) selectPath(path string) {
	for i, node := range p.flatList {
		if node.Path == path {
			p.cursor = i
			return
		}
	}
	p.clampCursor()
}

func buildTree(path string, parent *FileNode, depth int) (*FileNode, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, err
	}

	name := info.Name()
	if depth == 0 {
		name = filepath.Base(cleanPath)
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = cleanPath
		}
	}

	node := &FileNode{
		Name:   name,
		Path:   cleanPath,
		IsDir:  info.IsDir(),
		Parent: parent,
		Depth:  depth,
	}

	if !node.IsDir {
		return node, nil
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, err
	}

	children := make([]*FileNode, 0, len(entries))
	for _, entry := range entries {
		child, err := buildTree(filepath.Join(cleanPath, entry.Name()), node, depth+1)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	node.Children = children
	return node, nil
}

func flattenVisibleNodes(root *FileNode) []*FileNode {
	if root == nil {
		return nil
	}

	flat := []*FileNode{root}
	if root.IsDir && !root.Collapsed {
		for _, child := range root.Children {
			flat = append(flat, flattenVisibleNodes(child)...)
		}
	}
	return flat
}

func collectCollapsed(node *FileNode, out map[string]bool) {
	if node == nil {
		return
	}
	if node.IsDir {
		out[node.Path] = node.Collapsed
	}
	for _, child := range node.Children {
		collectCollapsed(child, out)
	}
}

func applyCollapsed(node *FileNode, collapsed map[string]bool) {
	if node == nil {
		return
	}
	if node.IsDir {
		if value, ok := collapsed[node.Path]; ok {
			node.Collapsed = value
		}
	}
	for _, child := range node.Children {
		applyCollapsed(child, collapsed)
	}
}

func (p *TreePane) String() string {
	return fmt.Sprintf("TreePane(%s)", p.id)
}
