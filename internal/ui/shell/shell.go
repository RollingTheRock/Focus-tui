package shell

import (
	"image/color"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/models"

	"github.com/RollingTheRock/Focus-tui/internal/x/vt"
	"github.com/RollingTheRock/Focus-tui/internal/x/xpty"
	tea "charm.land/bubbletea/v2"
)

const shellRefreshInterval = 33 * time.Millisecond

// Messages for the Bubbletea event loop.
type StartedMsg struct{ PaneID models.PaneID }
type RefreshMsg struct{ PaneID models.PaneID }
type ExitedMsg struct {
	PaneID models.PaneID
	Err    error
}

// OpenExternalShellMsg is sent when the user requests an external terminal
// (e.g. Alt+z) because the embedded pane is too small for TUI test output.
type OpenExternalShellMsg struct {
	PaneID models.PaneID
	CWD    string
}

// Model implements models.Panel for an embedded terminal shell.
type Model struct {
	id     models.PaneID
	common *models.CommonModel
	width  int
	height int
	cwd    string

	pty   xpty.Pty
	vterm *vt.SafeEmulator
	cmd   *exec.Cmd

	running bool
	exited  bool

	// Output batching: reader goroutine sets dirty, tick clears it.
	dirty atomic.Bool

	// View cache: only re-render when content changed.
	viewDirty  bool
	cachedView string

	// scrollOffset is how many lines we have scrolled back into scrollback history.
	// 0 = live view (bottom of output).
	scrollOffset int

	// Debounced PTY resize: store pending dims atomically so the timer goroutine
	// can read them without a data race.
	pendingW       atomic.Int32
	pendingH       atomic.Int32
	ptyResizeTimer *time.Timer

	autoType string
}

// New creates a new shell panel.
func New(cm *models.CommonModel, id models.PaneID) *Model {
	return NewWithCWD(cm, id, "")
}

func NewWithCWD(cm *models.CommonModel, id models.PaneID, cwd string) *Model {
	return &Model{
		id:     id,
		common: cm,
		width:  80,
		height: 24,
		cwd:    cwd,
	}
}

func NewWithCommand(cm *models.CommonModel, id models.PaneID, cwd, command string) *Model {
	m := NewWithCWD(cm, id, cwd)
	m.autoType = command
	return m
}

// Init implements models.Panel.
func (m *Model) Init() tea.Cmd {
	return m.startShell()
}

func (m *Model) startShell() tea.Cmd {
	return func() tea.Msg {
		p, err := xpty.NewPty(m.width, m.height)
		if err != nil {
			return ExitedMsg{PaneID: m.id, Err: err}
		}

		vtm := vt.NewSafeEmulator(m.width, m.height)

		// Drain terminal query responses (DA, DSR, CPR, color queries, etc.)
		// from the emulator's internal io.Pipe and forward them back to the
		// PTY so nested TUI apps receive proper responses.
		go io.Copy(p, vtm)

		sh := os.Getenv("SHELL")
		if sh == "" {
			sh = "/bin/sh"
		}
		cmd := exec.Command(sh)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		if m.cwd != "" {
			cmd.Dir = m.cwd
		}

		if err := p.Start(cmd); err != nil {
			p.Close()
			return ExitedMsg{PaneID: m.id, Err: err}
		}

		m.pty = p
		m.vterm = vtm
		m.cmd = cmd
		m.running = true
		m.exited = false
		m.scrollOffset = 0
		m.viewDirty = true
		m.cachedView = ""

		if m.autoType != "" {
			go func() {
				time.Sleep(200 * time.Millisecond)
				if !m.running || m.pty == nil {
					return
				}
				if m.pty != nil {
					m.pty.Write([]byte(m.autoType + "\r"))
				}
			}()
		}

		return StartedMsg{PaneID: m.id}
	}
}

// readPtyLoop runs a continuous read loop in a background goroutine.
// It writes PTY output into the vt emulator and sets the dirty flag.
// Only returns when the PTY closes.
func (m *Model) readPtyLoop() tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 32*1024)
		for {
			n, err := m.pty.Read(buf)
			if err != nil {
				return ExitedMsg{PaneID: m.id, Err: err}
			}
			m.vterm.Write(buf[:n])
			m.dirty.Store(true)
		}
	}
}

// shellRefreshCmd returns a tick command that fires every 33ms (~30fps).
// On each tick, Bubbletea calls Update → View, picking up any dirty output.
func (m *Model) shellRefreshCmd() tea.Cmd {
	return tea.Tick(shellRefreshInterval, func(t time.Time) tea.Msg {
		return RefreshMsg{PaneID: m.id}
	})
}

// Update implements models.Panel.
func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case StartedMsg:
		if msg.PaneID != m.id {
			return m, nil
		}
		return m, tea.Batch(m.readPtyLoop(), m.shellRefreshCmd())

	case RefreshMsg:
		if msg.PaneID != m.id {
			return m, nil
		}
		if !m.running {
			return m, nil
		}
		// Sync the cached mouse state so FastMouseSequence stays accurate
		// without needing a lock on every scroll event.
		if m.vterm != nil {
			m.vterm.SyncMouseState()
		}
		// Only mark view dirty if the reader goroutine produced new output.
		if m.dirty.CompareAndSwap(true, false) {
			m.viewDirty = true
		}
		return m, m.shellRefreshCmd()

	case ExitedMsg:
		if msg.PaneID != m.id {
			return m, nil
		}
		m.running = false
		m.exited = true
		m.pty = nil
		m.cmd = nil
		m.viewDirty = true
		return m, nil

	case tea.KeyPressMsg:
		if m.exited {
			m.exited = false
			m.running = false
			m.scrollOffset = 0
			m.viewDirty = true
			return m, m.startShell()
		}
		// Alt+z opens an external terminal for the current worktree so TUI tests
		// can run with a full-size terminal instead of the small embedded pane.
		if msg.Keystroke() == "alt+z" {
			return m, func() tea.Msg {
				return OpenExternalShellMsg{PaneID: m.id, CWD: m.cwd}
			}
		}
		m.forwardKey(msg)
		return m, nil

	case tea.PasteMsg:
		if m.pty != nil {
			m.pty.Write([]byte("\x1b[200~" + msg.Content + "\x1b[201~")) //nolint:errcheck
		}
		return m, nil

	case tea.MouseMsg:
		if m.pty == nil || !m.running {
			return m, nil
		}
		m.forwardMouse(msg)
		// forwardMouse already sets viewDirty for scrollback navigation.
		// For mouse-reporting apps the screen only changes after the PTY
		// produces output, which readPtyLoop picks up on the next tick.
		return m, nil
	}
	return m, nil
}

// forwardKey writes raw bytes directly to the PTY, bypassing the vt emulator's
// internal pipe. We translate tea.KeyPressMsg → raw ANSI escape sequences and write
// them straight to the PTY master fd.
func (m *Model) forwardKey(msg tea.KeyPressMsg) {
	if m.pty == nil {
		return
	}

	var seq string
	ks := msg.String()
	isAlt := strings.HasPrefix(ks, "alt+")
	if isAlt {
		ks = strings.TrimPrefix(ks, "alt+")
	}

	switch ks {
	case "enter":
		seq = "\r"
	case "tab":
		seq = "\t"
	case "backspace":
		seq = "\x7f"
	case "esc":
		seq = "\x1b"
	case "space":
		if isAlt {
			seq = "\x1b "
		} else {
			seq = " "
		}

	case "up":
		if isAlt {
			seq = "\x1b[1;3A"
		} else {
			seq = "\x1b[A"
		}
	case "down":
		if isAlt {
			seq = "\x1b[1;3B"
		} else {
			seq = "\x1b[B"
		}
	case "right":
		if isAlt {
			seq = "\x1b[1;3C"
		} else {
			seq = "\x1b[C"
		}
	case "left":
		if isAlt {
			seq = "\x1b[1;3D"
		} else {
			seq = "\x1b[D"
		}
	case "home":
		if isAlt {
			seq = "\x1b[1;3H"
		} else {
			seq = "\x1b[H"
		}
	case "end":
		if isAlt {
			seq = "\x1b[1;3F"
		} else {
			seq = "\x1b[F"
		}
	case "pgup":
		seq = "\x1b[5~"
	case "pgdown":
		seq = "\x1b[6~"
	case "delete":
		seq = "\x1b[3~"
	case "insert":
		seq = "\x1b[2~"
	case "shift+tab":
		seq = "\x1b[Z"

	case "f1":
		seq = "\x1bOP"
	case "f2":
		seq = "\x1bOQ"
	case "f3":
		seq = "\x1bOR"
	case "f4":
		seq = "\x1bOS"
	case "f5":
		seq = "\x1b[15~"
	case "f6":
		seq = "\x1b[17~"
	case "f7":
		seq = "\x1b[18~"
	case "f8":
		seq = "\x1b[19~"
	case "f9":
		seq = "\x1b[20~"
	case "f10":
		seq = "\x1b[21~"
	case "f11":
		seq = "\x1b[23~"
	case "f12":
		seq = "\x1b[24~"

	default:
		// Ctrl+letter keys.
		if len(ks) == 6 && strings.HasPrefix(ks, "ctrl+") && ks[5] >= 'a' && ks[5] <= 'z' {
			seq = string(rune(ks[5]-'a'+1))
		} else if isAlt && len(ks) == 1 {
			seq = "\x1b" + ks
		} else if len(ks) == 1 {
			seq = ks
		}
	}

	if seq != "" {
		m.pty.Write([]byte(seq)) //nolint:errcheck
	}
}

// forwardMouse handles mouse events. When the inner PTY application has enabled
// mouse reporting, events are forwarded directly to the PTY using the cached
// mouse mode state (IsMouseReportingFast / FastMouseSequence) to avoid lock
// contention with readPtyLoop's Write. When mouse reporting is off, wheel
// events scroll the view through the scrollback buffer instead.
func (m *Model) forwardMouse(msg tea.MouseMsg) {
	if m.pty == nil {
		return
	}

	e := msg.Mouse()

	// When the inner app has NOT enabled mouse reporting, use wheel events for
	// scrollback navigation rather than forwarding them as garbage bytes.
	if !m.vterm.IsMouseReportingFast() {
		switch e.Button {
		case tea.MouseWheelUp:
			m.scrollOffset += 3
			if max := m.vterm.ScrollbackLen(); m.scrollOffset > max {
				m.scrollOffset = max
			}
			m.viewDirty = true
		case tea.MouseWheelDown:
			m.scrollOffset -= 3
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			m.viewDirty = true
		}
		return
	}

	// Convert Bubble Tea mouse message to ultraviolet mouse event, preserving
	// the original message type so that FastMouseSequence can correctly determine
	// motion vs release vs click.
	var uvMouse vt.Mouse
	switch msg.(type) {
	case tea.MouseWheelMsg:
		uvMouse = vt.MouseWheel{X: e.X, Y: e.Y, Button: e.Button, Mod: e.Mod}
	case tea.MouseMotionMsg:
		uvMouse = vt.MouseMotion{X: e.X, Y: e.Y, Button: e.Button, Mod: e.Mod}
	case tea.MouseReleaseMsg:
		uvMouse = vt.MouseRelease{X: e.X, Y: e.Y, Button: e.Button, Mod: e.Mod}
	case tea.MouseClickMsg:
		uvMouse = vt.MouseClick{X: e.X, Y: e.Y, Button: e.Button, Mod: e.Mod}
	default:
		uvMouse = vt.MouseClick{X: e.X, Y: e.Y, Button: e.Button, Mod: e.Mod}
	}

	// Write directly to the PTY using the cached mouse state. This avoids
	// RLock contention with readPtyLoop's Write lock, which is the root cause
	// of the escalating latency during continuous scrolling.
	if seq, ok := m.vterm.FastMouseSequence(uvMouse); ok {
		m.pty.Write([]byte(seq)) //nolint:errcheck
	}
}

// CursorPos returns the cursor's column (x) and row (y, 0-indexed) inside the
// shell panel content area, plus whether the cursor should be shown. Returns
// visible=false when the shell is not running, the cursor is hidden by the inner
// app, or the view is scrolled back into history.
func (m *Model) CursorPos() (x, y int, visible bool) {
	if m.vterm == nil || !m.running {
		return 0, 0, false
	}
	if m.scrollOffset > 0 {
		return 0, 0, false
	}
	if m.vterm.IsCursorHidden() {
		return 0, 0, false
	}
	pos := m.vterm.CursorPosition()
	return pos.X, pos.Y, true
}

// CursorInfo returns the cursor's position, visibility, style, steady state,
// and color so that the outer Bubble Tea view can render a matching hardware
// cursor.
func (m *Model) CursorInfo() (x, y int, visible bool, style vt.CursorStyle, steady bool, curColor color.Color) {
	if m.vterm == nil || !m.running {
		return 0, 0, false, vt.CursorBlock, true, nil
	}
	if m.scrollOffset > 0 {
		return 0, 0, false, vt.CursorBlock, true, nil
	}
	return m.vterm.CursorState()
}

// SessionStatus reports the shell lifecycle state for pane metadata.
func (m *Model) SessionStatus() models.PaneStatus {
	if m.exited {
		return models.PaneStatusExited
	}
	if m.running {
		return models.PaneStatusRunning
	}
	return models.PaneStatusStarting
}

// View implements models.Panel.
func (m *Model) View() tea.View {
	if m.vterm == nil {
		return tea.NewView("Starting shell...")
	}
	if m.exited {
		return tea.NewView("Shell exited. Press any key to restart.")
	}
	if m.viewDirty || m.cachedView == "" {
		if m.scrollOffset > 0 {
			m.cachedView = m.vterm.RenderScrolled(m.scrollOffset, m.width, m.height)
		} else {
			m.cachedView = m.vterm.Render()
		}
		m.viewDirty = false
	}
	return tea.NewView(m.cachedView)
}

// SetSize implements models.Panel.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.scrollOffset = 0 // reset scroll on resize
	m.viewDirty = true
	if m.vterm != nil {
		m.vterm.Resize(width, height)
	}

	// Debounce PTY resize: store the target dimensions atomically and (re)start a
	// short timer. Only the final size triggers SIGWINCH, preventing the shell from
	// being interrupted on every pixel the user drags the window.
	m.pendingW.Store(int32(width))
	m.pendingH.Store(int32(height))
	if m.ptyResizeTimer != nil {
		m.ptyResizeTimer.Stop()
	}
	if m.pty != nil {
		pty := m.pty // capture pointer for timer goroutine
		m.ptyResizeTimer = time.AfterFunc(80*time.Millisecond, func() {
			w := int(m.pendingW.Load())
			h := int(m.pendingH.Load())
			_ = pty.Resize(w, h)
			m.dirty.Store(true) // trigger a view refresh after the shell redraws
		})
	}
}

// SetCWD updates the shell's working directory. If the shell is running, it
// sends a cd command to the PTY.
func (m *Model) SetCWD(cwd string) {
	m.cwd = cwd
	if m.pty != nil && m.running && cwd != "" {
		m.pty.Write([]byte("cd " + cwd + "\r")) //nolint:errcheck
	}
}

// SendCommand writes a command string directly to the shell's PTY followed by
// Enter. Returns false if the PTY is not running.
//
// NOTE: If the shell is running a foreground process, the command may be
// consumed by that process rather than interpreted by the shell. The caller
// should ensure the shell is at a prompt before invoking this.
func (m *Model) SendCommand(cmd string) bool {
	if m.pty == nil || !m.running {
		return false
	}
	m.pty.Write([]byte(cmd + "\r")) //nolint:errcheck
	return true
}

// Close cleans up the PTY and emulator resources.
func (m *Model) Close() error {
	if m.ptyResizeTimer != nil {
		m.ptyResizeTimer.Stop()
	}
	if m.vterm != nil {
		m.vterm.Close()
	}
	if m.pty != nil {
		m.pty.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
	return nil
}
