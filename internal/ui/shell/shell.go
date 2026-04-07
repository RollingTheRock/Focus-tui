package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// Messages for the Bubbletea event loop.
type shellStartedMsg struct{}
type shellRefreshMsg struct{}
type shellExitedMsg struct{ err error }

// Model implements models.Panel for an embedded terminal shell.
type Model struct {
	common *models.CommonModel
	width  int
	height int

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
}

// New creates a new shell panel.
func New(cm *models.CommonModel) *Model {
	return &Model{
		common: cm,
		width:  80,
		height: 24,
	}
}

// Init implements models.Panel.
func (m *Model) Init() tea.Cmd {
	return m.startShell()
}

func (m *Model) startShell() tea.Cmd {
	return func() tea.Msg {
		p, err := xpty.NewPty(m.width, m.height)
		if err != nil {
			return shellExitedMsg{err: err}
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

		if err := p.Start(cmd); err != nil {
			p.Close()
			return shellExitedMsg{err: err}
		}

		m.pty = p
		m.vterm = vtm
		m.cmd = cmd
		m.running = true
		m.exited = false
		m.viewDirty = true
		m.cachedView = ""

		return shellStartedMsg{}
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
				return shellExitedMsg{err: err}
			}
			m.vterm.Write(buf[:n])
			m.dirty.Store(true)
		}
	}
}

// shellRefreshCmd returns a tick command that fires every 16ms (~62fps).
// On each tick, Bubbletea calls Update → View, picking up any dirty output.
func (m *Model) shellRefreshCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return shellRefreshMsg{}
	})
}

// Update implements models.Panel.
func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case shellStartedMsg:
		return m, tea.Batch(m.readPtyLoop(), m.shellRefreshCmd())

	case shellRefreshMsg:
		if !m.running {
			return m, nil
		}
		// Only mark view dirty if the reader goroutine produced new output.
		if m.dirty.CompareAndSwap(true, false) {
			m.viewDirty = true
		}
		return m, m.shellRefreshCmd()

	case shellExitedMsg:
		m.running = false
		m.exited = true
		m.viewDirty = true
		return m, nil

	case tea.KeyMsg:
		if m.exited {
			return m, m.startShell()
		}
		m.forwardKey(msg)
		return m, nil

	case tea.MouseMsg:
		if m.pty == nil || !m.running {
			return m, nil
		}
		m.forwardMouse(msg)
		return m, nil
	}
	return m, nil
}

// forwardKey writes raw bytes directly to the PTY, bypassing the vt emulator's
// internal pipe. We translate tea.KeyMsg → raw ANSI escape sequences and write
// them straight to the PTY master fd.
func (m *Model) forwardKey(msg tea.KeyMsg) {
	if m.pty == nil {
		return
	}

	var seq string

	switch msg.Type {
	case tea.KeyRunes:
		s := string(msg.Runes)
		if msg.Paste {
			seq = "\x1b[200~" + s + "\x1b[201~"
		} else if msg.Alt {
			seq = "\x1b" + s
		} else {
			seq = s
		}

	case tea.KeyEnter:
		seq = "\r"
	case tea.KeyTab:
		seq = "\t"
	case tea.KeyBackspace:
		seq = "\x7f"
	case tea.KeyEscape:
		seq = "\x1b"
	case tea.KeySpace:
		if msg.Alt {
			seq = "\x1b "
		} else {
			seq = " "
		}

	case tea.KeyUp:
		if msg.Alt {
			seq = "\x1b[1;3A"
		} else {
			seq = "\x1b[A"
		}
	case tea.KeyDown:
		if msg.Alt {
			seq = "\x1b[1;3B"
		} else {
			seq = "\x1b[B"
		}
	case tea.KeyRight:
		if msg.Alt {
			seq = "\x1b[1;3C"
		} else {
			seq = "\x1b[C"
		}
	case tea.KeyLeft:
		if msg.Alt {
			seq = "\x1b[1;3D"
		} else {
			seq = "\x1b[D"
		}
	case tea.KeyHome:
		if msg.Alt {
			seq = "\x1b[1;3H"
		} else {
			seq = "\x1b[H"
		}
	case tea.KeyEnd:
		if msg.Alt {
			seq = "\x1b[1;3F"
		} else {
			seq = "\x1b[F"
		}
	case tea.KeyPgUp:
		seq = "\x1b[5~"
	case tea.KeyPgDown:
		seq = "\x1b[6~"
	case tea.KeyDelete:
		seq = "\x1b[3~"
	case tea.KeyInsert:
		seq = "\x1b[2~"
	case tea.KeyShiftTab:
		seq = "\x1b[Z"

	case tea.KeyF1:
		seq = "\x1bOP"
	case tea.KeyF2:
		seq = "\x1bOQ"
	case tea.KeyF3:
		seq = "\x1bOR"
	case tea.KeyF4:
		seq = "\x1bOS"
	case tea.KeyF5:
		seq = "\x1b[15~"
	case tea.KeyF6:
		seq = "\x1b[17~"
	case tea.KeyF7:
		seq = "\x1b[18~"
	case tea.KeyF8:
		seq = "\x1b[19~"
	case tea.KeyF9:
		seq = "\x1b[20~"
	case tea.KeyF10:
		seq = "\x1b[21~"
	case tea.KeyF11:
		seq = "\x1b[23~"
	case tea.KeyF12:
		seq = "\x1b[24~"

	default:
		// Ctrl+letter keys: tea.KeyCtrlA (=1) through tea.KeyCtrlZ (=26).
		if msg.Type >= tea.KeyCtrlA && msg.Type <= tea.KeyCtrlZ {
			seq = string(rune(msg.Type))
		}
	}

	if seq != "" {
		m.pty.Write([]byte(seq)) //nolint:errcheck
	}
}

// forwardMouse encodes a tea.MouseMsg as an SGR mouse sequence and writes
// it to the PTY. Format: ESC [ < Cb ; Cx ; Cy M/m
//
// SGR sequences are only forwarded when the PTY application has enabled mouse
// reporting (DEC modes 9/1000/1001/1002/1003). Without this guard, a plain
// bash/zsh prompt would receive unintelligible escape bytes and show them as
// garbage in the readline buffer.
func (m *Model) forwardMouse(msg tea.MouseMsg) {
	if m.pty == nil {
		return
	}
	// Only forward if the inner PTY app has enabled mouse reporting.
	if m.vterm == nil || !m.vterm.IsMouseReporting() {
		return
	}

	e := tea.MouseEvent(msg)

	var cb int
	switch e.Button {
	case tea.MouseButtonLeft:
		cb = 0
	case tea.MouseButtonMiddle:
		cb = 1
	case tea.MouseButtonRight:
		cb = 2
	case tea.MouseButtonWheelUp:
		cb = 64
	case tea.MouseButtonWheelDown:
		cb = 65
	case tea.MouseButtonWheelLeft:
		cb = 66
	case tea.MouseButtonWheelRight:
		cb = 67
	case tea.MouseButtonBackward:
		cb = 128
	case tea.MouseButtonForward:
		cb = 129
	case tea.MouseButtonNone:
		cb = 3
	default:
		return
	}

	if e.Shift {
		cb |= 4
	}
	if e.Alt {
		cb |= 8
	}
	if e.Ctrl {
		cb |= 16
	}
	if e.Action == tea.MouseActionMotion {
		cb |= 32
	}

	suffix := "M"
	if e.Action == tea.MouseActionRelease {
		suffix = "m"
	}
	seq := fmt.Sprintf("\x1b[<%d;%d;%d%s", cb, e.X+1, e.Y+1, suffix)
	m.pty.Write([]byte(seq)) //nolint:errcheck
}

// View implements models.Panel.
func (m *Model) View() string {
	if m.vterm == nil {
		return "Starting shell..."
	}
	if m.exited {
		return "Shell exited. Press any key to restart."
	}
	if m.viewDirty || m.cachedView == "" {
		m.cachedView = m.vterm.Render()
		m.viewDirty = false
	}
	return m.cachedView
}

// SetSize implements models.Panel.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewDirty = true
	if m.vterm != nil {
		m.vterm.Resize(width, height)
	}
	if m.pty != nil {
		_ = m.pty.Resize(width, height)
	}
}

// Close cleans up the PTY and emulator resources.
func (m *Model) Close() error {
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
