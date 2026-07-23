package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/RollingTheRock/Focus-tui/internal/app"
	"github.com/RollingTheRock/Focus-tui/internal/config"
	"github.com/RollingTheRock/Focus-tui/internal/projections"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

var version = "dev"

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("focus %s\n", versionString())
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("focus - AI-assisted project cockpit for the terminal")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  focus [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  -h, --help     Show this help message")
		fmt.Println("  -v, --version  Print version information")
		os.Exit(0)
	}

	cfg := config.Load()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	projectRoot, err := store.ResolveProjectRoot(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dbPath, err := resolveDBPath(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	st, err := store.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// Redirect log output to a file so that log.Printf does not corrupt
	// the TUI terminal rendering (bubbletea owns stdout/stderr).
	logDir := filepath.Join(projectRoot, ".focus", "journal")
	if err := os.MkdirAll(logDir, 0o755); err == nil {
		logPath := filepath.Join(logDir, "focus.log")
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			log.SetOutput(f)
		}
	}

	// Derive per-project MCP addresses to avoid conflicts when running
	// multiple focus instances across different projects.
	// On Windows, MCP uses HTTP transport (see internal/mcp/socket_windows.go);
	// leave MCPSocket empty so mcp.Server.Start() is a no-op instead of
	// logging a spurious "unix socket listen" error each launch.
	if cfg.Agent.MCPSocket == "" && runtime.GOOS != "windows" {
		cfg.Agent.MCPSocket = filepath.Join(projectRoot, ".focus", "mcp.sock")
	}

	// Mark overdue todos from previous days.
	_ = st.MarkOverdue()

	// Start projection builder when running on PostgreSQL.
	var projBuilder *projections.Builder
	if st.Mode() == "postgresql" {
		if pool := st.PGPool(); pool != nil {
			raw := st.EventStore()
			if evStore, ok := raw.(*store.EventStore); ok {
				projBuilder = projections.NewBuilder(pool, evStore)
				go projBuilder.Run(context.Background())
			}
		}
	}

	m := app.New(cfg, st)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if projBuilder != nil {
		projBuilder.Stop()
	}
}

func resolveDBPath(projectRoot string) (string, error) {
	if override := os.Getenv("FOCUS_DB_PATH"); override != "" {
		return override, nil
	}
	return store.ProjectDBPath(projectRoot)
}
