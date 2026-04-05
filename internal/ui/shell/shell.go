package shell

import (
	"os"
	"os/exec"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// Messages for the Bubbletea event loop.
type shellStartedMsg struct{}
type shellOutputMsg struct{}
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

		// No io.Copy(pty, vterm) goroutine needed:
		// We write directly to PTY in forwardKey, bypassing the emulator's
		// internal io.Pipe which would deadlock Bubbletea's synchronous Update.

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

		return shellStartedMsg{}
	}
}

// readPtyLoop returns a tea.Cmd that reads PTY output into the emulator.
// Each read triggers a Bubbletea re-render, then re-issues itself.
func (m *Model) readPtyLoop() tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 32*1024)
		n, err := m.pty.Read(buf)
		if err != nil {
			return shellExitedMsg{err: err}
		}
		m.vterm.Write(buf[:n])
		return shellOutputMsg{}
	}
}

// Update implements models.Panel.
func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case shellStartedMsg:
		return m, m.readPtyLoop()

	case shellOutputMsg:
		return m, m.readPtyLoop()

	case shellExitedMsg:
		m.running = false
		m.exited = true
		return m, nil

	case tea.KeyMsg:
		if m.exited {
			return m, m.startShell()
		}
		m.forwardKey(msg)
		return m, nil
	}
	return m, nil
}

// forwardKey writes raw bytes directly to the PTY, bypassing the vt emulator's
// internal pipe. This avoids the io.Pipe deadlock: SendKey/SendText write to a
// synchronous pipe that blocks until a reader (io.Copy goroutine) consumes it,
// but that blocks Bubbletea's main Update goroutine.
//
// We translate tea.KeyMsg → raw ANSI escape sequences and write them straight
// to the PTY master fd. The shell process receives the input on its stdin.
// The echo comes back through PTY → readPtyLoop → vterm.Write → Render.
func (m *Model) forwardKey(msg tea.KeyMsg) {
	if m.pty == nil {
		return
	}

	var seq string

	switch msg.Type {
	case tea.KeyRunes:
		s := string(msg.Runes)
		if msg.Alt {
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
		seq = " "

	case tea.KeyUp:
		seq = "\x1b[A"
	case tea.KeyDown:
		seq = "\x1b[B"
	case tea.KeyRight:
		seq = "\x1b[C"
	case tea.KeyLeft:
		seq = "\x1b[D"
	case tea.KeyHome:
		seq = "\x1b[H"
	case tea.KeyEnd:
		seq = "\x1b[F"
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

// View implements models.Panel.
func (m *Model) View() string {
	if m.vterm == nil {
		return "Starting shell..."
	}
	if m.exited {
		return "Shell exited. Press any key to restart."
	}
	return m.vterm.Render()
}

// SetSize implements models.Panel.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
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
