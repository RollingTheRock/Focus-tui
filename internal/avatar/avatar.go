package avatar

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Backend represents the image rendering method.
type Backend int

const (
	BackendNone   Backend = iota
	BackendSixel          // chafa --format=sixel (true pixels)
	BackendKitty          // chafa --format=kitty (true pixels)
	BackendSymbol         // chafa half-block/symbol chars (fallback)
)

// Render converts an image file into terminal-displayable output.
// It auto-detects the best available backend:
//
//	1. Sixel (via chafa --format=sixel) — true pixel rendering, broad support
//	2. Kitty (via chafa --format=kitty) — true pixel rendering for kitty-compatible terminals
//	3. Symbols (via chafa half-block chars) — universal fallback
//
// Returns empty string if chafa is not installed or the image can't be rendered.
func Render(imagePath string, width int) string {
	if imagePath == "" {
		return ""
	}
	if width <= 0 {
		width = 32
	}

	// Expand ~ to home directory.
	if strings.HasPrefix(imagePath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			imagePath = filepath.Join(home, imagePath[2:])
		}
	}

	// Verify file exists.
	if _, err := os.Stat(imagePath); err != nil {
		return ""
	}

	// Check chafa is available.
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return ""
	}

	height := width / 2
	sizeArg := strconv.Itoa(width) + "x" + strconv.Itoa(height)

	// Try backends in preference order.
	for _, backend := range detectBackends() {
		var result string
		switch backend {
		case BackendSixel:
			result = runChafa(chafaPath, imagePath, sizeArg, "sixel")
		case BackendKitty:
			result = runChafa(chafaPath, imagePath, sizeArg, "kitty")
		case BackendSymbol:
			result = runChafa(chafaPath, imagePath, sizeArg, "symbols")
		}
		if result != "" {
			return result
		}
	}

	return ""
}

// detectBackends returns backends to try in order, based on terminal capabilities.
func detectBackends() []Backend {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	kittyPID := os.Getenv("KITTY_PID")

	// Terminals known to NOT support any image protocol — skip straight to symbols.
	if termProgram == "WarpTerminal" || termProgram == "Warp" ||
		termProgram == "Apple_Terminal" ||
		termProgram == "Alacritty" || strings.Contains(term, "alacritty") {
		return []Backend{BackendSymbol}
	}

	var backends []Backend

	// Kitty terminal detection.
	if kittyPID != "" || strings.Contains(term, "kitty") {
		backends = append(backends, BackendKitty)
	}

	// Ghostty supports kitty graphics protocol.
	if termProgram == "ghostty" || os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		backends = append(backends, BackendKitty)
	}

	// iTerm2 / WezTerm / foot / mlterm etc support sixel.
	if termProgram == "WezTerm" ||
		termProgram == "iTerm.app" ||
		strings.Contains(term, "foot") ||
		strings.Contains(term, "mlterm") ||
		os.Getenv("WEZTERM_EXECUTABLE") != "" {
		backends = append(backends, BackendSixel)
	}

	// If no specific terminal detected, still try sixel — many terminals
	// support it silently (e.g. Windows Terminal, Contour, etc).
	// chafa will fail gracefully if unsupported.
	if len(backends) == 0 {
		backends = append(backends, BackendSixel)
	}

	// Always add symbol fallback last.
	backends = append(backends, BackendSymbol)
	return backends
}

// runChafa executes chafa with the given format.
func runChafa(chafaPath, imagePath, sizeArg, format string) string {
	var args []string

	switch format {
	case "sixel":
		args = []string{
			"--format", "sixels",
			"--size", sizeArg,
			"--animate", "off",
			imagePath,
		}
	case "kitty":
		args = []string{
			"--format", "kitty",
			"--size", sizeArg,
			"--animate", "off",
			imagePath,
		}
	default: // symbols
		args = []string{
			"--size", sizeArg,
			"--symbols", "block+border+space-wide",
			"--color-space", "din99d",
			"--animate", "off",
			imagePath,
		}
	}

	cmd := exec.Command(chafaPath, args...)
	// Inherit terminal environment for protocol detection.
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		// chafa exits non-zero if format is unsupported.
		return ""
	}

	result := strings.TrimRight(string(out), "\n\r ")
	if result == "" {
		return ""
	}

	return result
}

// BackendName returns a human-readable name for the active backend.
// Useful for debugging/status display.
func BackendName(imagePath string) string {
	if imagePath == "" {
		return "none"
	}

	for _, b := range detectBackends() {
		switch b {
		case BackendSixel:
			return "sixel"
		case BackendKitty:
			return "kitty"
		case BackendSymbol:
			return "symbols"
		}
	}
	return "none"
}

func init() {
	// Suppress "chafa not found" warnings by checking early.
	if _, err := exec.LookPath("chafa"); err != nil {
		log.Printf("focus: hint: install chafa for avatar display (apt install chafa)")
	}
}
