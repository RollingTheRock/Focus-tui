package vt

import (
	"image/color"
	"strings"
	"sync"
	"sync/atomic"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// SafeEmulator is a wrapper around an Emulator that adds concurrency safety.
type SafeEmulator struct {
	*Emulator
	mu sync.RWMutex

	// Cached mouse state to avoid RLock contention with the readPtyLoop
	// writer during high-frequency scroll events.
	mouseReporting atomic.Bool
	mouseSgr       atomic.Bool
}

var _ Terminal = (*SafeEmulator)(nil)

// NewSafeEmulator creates a new SafeEmulator instance.
func NewSafeEmulator(w, h int) *SafeEmulator {
	return &SafeEmulator{
		Emulator: NewEmulator(w, h),
	}
}

// SyncMouseState updates the cached mouse reporting mode and encoding from
// the emulator's current state. Call this periodically (e.g. on the shell
// refresh tick) so that FastMouseSequence and IsMouseReportingFast stay
// accurate without needing a lock on every event.
func (se *SafeEmulator) SyncMouseState() {
	se.mu.RLock()
	defer se.mu.RUnlock()
	se.mouseReporting.Store(
		se.Emulator.isModeSet(ansi.ModeMouseX10) ||
			se.Emulator.isModeSet(ansi.ModeMouseNormal) ||
			se.Emulator.isModeSet(ansi.ModeMouseHighlight) ||
			se.Emulator.isModeSet(ansi.ModeMouseButtonEvent) ||
			se.Emulator.isModeSet(ansi.ModeMouseAnyEvent))
	se.mouseSgr.Store(se.Emulator.isModeSet(ansi.ModeMouseExtSgr))
}

// IsMouseReportingFast returns the cached mouse-reporting state without
// acquiring a lock. The cache is refreshed by SyncMouseState.
func (se *SafeEmulator) IsMouseReportingFast() bool {
	return se.mouseReporting.Load()
}

// FastMouseSequence returns the ANSI escape sequence for a mouse event using
// the cached mouse mode and encoding state. It does not acquire any lock, so
// it can be called from the Bubble Tea event loop during high-frequency
// scrolling without contending with readPtyLoop's Write lock.
// The second return value is false when mouse reporting is disabled.
func (se *SafeEmulator) FastMouseSequence(m Mouse) (string, bool) {
	if !se.mouseReporting.Load() {
		return "", false
	}

	mouse := m.Mouse()
	_, isMotion := m.(MouseMotion)
	_, isRelease := m.(MouseRelease)
	b := ansi.EncodeMouseButton(mouse.Button, isMotion,
		mouse.Mod.Contains(ModShift),
		mouse.Mod.Contains(ModAlt),
		mouse.Mod.Contains(ModCtrl))

	if se.mouseSgr.Load() {
		return ansi.MouseSgr(b, mouse.X, mouse.Y, isRelease), true
	}
	return ansi.MouseX10(b, mouse.X, mouse.Y), true
}

// Write writes data to the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Write(data []byte) (int, error) {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.Emulator.Write(data)
}

// Read reads data from the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Read(p []byte) (int, error) {
	return se.Emulator.Read(p)
}

// Resize resizes the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Resize(w, h int) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.Resize(w, h)
}

// Render renders the emulator's current state in a concurrency-safe manner.
func (se *SafeEmulator) Render() string {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.Render()
}

// SetCell sets a cell in the emulator in a concurrency-safe manner.
func (se *SafeEmulator) SetCell(x, y int, cell *uv.Cell) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetCell(x, y, cell)
}

// CellAt retrieves a cell from the emulator in a concurrency-safe manner.
func (se *SafeEmulator) CellAt(x, y int) *uv.Cell {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.CellAt(x, y)
}

// SendKey sends a key event to the emulator in a concurrency-safe manner.
func (se *SafeEmulator) SendKey(key uv.KeyEvent) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SendKey(key)
}

// SendMouse sends a mouse event to the emulator in a concurrency-safe manner.
func (se *SafeEmulator) SendMouse(mouse uv.MouseEvent) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SendMouse(mouse)
}

// MouseSequence returns the ANSI escape sequence for a mouse event based on
// the terminal's current mouse reporting mode and encoding. The second return
// value is false when mouse reporting is disabled.
func (se *SafeEmulator) MouseSequence(m Mouse) (string, bool) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	var (
		enc  ansi.Mode
		mode ansi.Mode
	)
	for _, mm := range []ansi.DECMode{
		ansi.ModeMouseX10,
		ansi.ModeMouseNormal,
		ansi.ModeMouseHighlight,
		ansi.ModeMouseButtonEvent,
		ansi.ModeMouseAnyEvent,
	} {
		if se.Emulator.isModeSet(mm) {
			mode = mm
		}
	}
	if mode == nil {
		return "", false
	}
	for _, mm := range []ansi.DECMode{
		ansi.ModeMouseExtSgr,
	} {
		if se.Emulator.isModeSet(mm) {
			enc = mm
		}
	}

	mouse := m.Mouse()
	_, isMotion := m.(MouseMotion)
	_, isRelease := m.(MouseRelease)
	b := ansi.EncodeMouseButton(mouse.Button, isMotion,
		mouse.Mod.Contains(ModShift),
		mouse.Mod.Contains(ModAlt),
		mouse.Mod.Contains(ModCtrl))

	switch enc {
	case nil:
		return ansi.MouseX10(b, mouse.X, mouse.Y), true
	case ansi.ModeMouseExtSgr:
		return ansi.MouseSgr(b, mouse.X, mouse.Y, isRelease), true
	}
	return "", false
}

// SendText sends text input to the emulator in a concurrency-safe manner.
func (se *SafeEmulator) SendText(text string) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SendText(text)
}

// Paste pastes text into the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Paste(text string) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.Paste(text)
}

// SetForegroundColor sets the foreground color in a concurrency-safe manner.
func (se *SafeEmulator) SetForegroundColor(color color.Color) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetForegroundColor(color)
}

// SetBackgroundColor sets the background color in a concurrency-safe manner.
func (se *SafeEmulator) SetBackgroundColor(color color.Color) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetBackgroundColor(color)
}

// SetCursorColor sets the cursor color in a concurrency-safe manner.
func (se *SafeEmulator) SetCursorColor(color color.Color) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetCursorColor(color)
}

// SetIndexedColor sets an indexed color in a concurrency-safe manner.
func (se *SafeEmulator) SetIndexedColor(index int, color color.Color) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetIndexedColor(index, color)
}

// IndexedColor retrieves an indexed color in a concurrency-safe manner.
func (se *SafeEmulator) IndexedColor(index int) color.Color {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.IndexedColor(index)
}

// Touched returns the touched lines in a concurrency-safe manner.
func (se *SafeEmulator) Touched() []*uv.LineData {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.Touched()
}

// Height returns the height of the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Height() int {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.Height()
}

// Width returns the width of the emulator in a concurrency-safe manner.
func (se *SafeEmulator) Width() int {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.Width()
}

// ForegroundColor returns the foreground color in a concurrency-safe manner.
func (se *SafeEmulator) ForegroundColor() color.Color {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.ForegroundColor()
}

// BackgroundColor returns the background color in a concurrency-safe manner.
func (se *SafeEmulator) BackgroundColor() color.Color {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.BackgroundColor()
}

// CursorColor returns the cursor color in a concurrency-safe manner.
func (se *SafeEmulator) CursorColor() color.Color {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.CursorColor()
}

// CursorPosition returns the cursor position in a concurrency-safe manner.
func (se *SafeEmulator) CursorPosition() uv.Position {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.CursorPosition()
}

// CursorState returns the cursor's position, visibility, style, steady state,
// and color in a concurrency-safe manner.
func (se *SafeEmulator) CursorState() (x, y int, visible bool, style CursorStyle, steady bool, curColor color.Color) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	if se.Emulator.scr == nil {
		return 0, 0, false, CursorBlock, true, nil
	}
	cur := se.Emulator.scr.Cursor()
	return cur.X, cur.Y, !cur.Hidden, cur.Style, cur.Steady, se.Emulator.CursorColor()
}

// Draw draws the emulator's content onto a given surface in a concurrency-safe manner.
func (se *SafeEmulator) Draw(s uv.Screen, a uv.Rectangle) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	se.Emulator.Draw(s, a)
}

// Scrollback returns the scrollback buffer in a concurrency-safe manner.
func (se *SafeEmulator) Scrollback() *Scrollback {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.Scrollback()
}

// ScrollbackLen returns the number of lines in the scrollback buffer in a concurrency-safe manner.
func (se *SafeEmulator) ScrollbackLen() int {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.ScrollbackLen()
}

// ScrollbackCellAt returns a cell from the scrollback buffer in a concurrency-safe manner.
func (se *SafeEmulator) ScrollbackCellAt(x, y int) *uv.Cell {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.ScrollbackCellAt(x, y)
}

// SetScrollbackSize sets the scrollback buffer size in a concurrency-safe manner.
func (se *SafeEmulator) SetScrollbackSize(maxLines int) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.SetScrollbackSize(maxLines)
}

// ClearScrollback clears the scrollback buffer in a concurrency-safe manner.
func (se *SafeEmulator) ClearScrollback() {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.Emulator.ClearScrollback()
}

// IsAltScreen returns whether in alternate screen mode in a concurrency-safe manner.
func (se *SafeEmulator) IsAltScreen() bool {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.IsAltScreen()
}

func (se *SafeEmulator) IsMouseReporting() bool {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.isModeSet(ansi.ModeMouseX10) ||
		se.Emulator.isModeSet(ansi.ModeMouseNormal) ||
		se.Emulator.isModeSet(ansi.ModeMouseHighlight) ||
		se.Emulator.isModeSet(ansi.ModeMouseButtonEvent) ||
		se.Emulator.isModeSet(ansi.ModeMouseAnyEvent)
}

func (se *SafeEmulator) IsCursorHidden() bool {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.Emulator.scr.cur.Hidden
}

func (se *SafeEmulator) RenderScrolled(offset, width, height int) string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	sb := se.Emulator.Scrollback()
	maxOffset := 0
	if sb != nil {
		maxOffset = sb.Len()
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	if offset == 0 {
		return se.Emulator.Render()
	}

	var b strings.Builder
	b.Grow(width * height * 2) // rough pre-allocation, leaves headroom for ANSI

	scrollbackLines := offset
	if scrollbackLines > height {
		scrollbackLines = height
	}

	if sb != nil {
		startIdx := maxOffset - offset
		for i := 0; i < scrollbackLines && (startIdx+i) < sb.Len(); i++ {
			if i > 0 {
				b.WriteByte('\n')
			}
			line := sb.Line(startIdx + i)
			if line != nil {
				b.WriteString(line.Render())
			}
		}
	}

	screenLines := height - scrollbackLines
	if screenLines > 0 {
		if scrollbackLines > 0 {
			b.WriteByte('\n')
		}
		lines := se.Emulator.scr.buf.Lines
		for i := 0; i < screenLines && i < len(lines); i++ {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(lines[i].Render())
		}
	}

	return b.String()
}
