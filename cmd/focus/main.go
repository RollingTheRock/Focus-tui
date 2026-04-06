package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"focus/internal/app"
	"focus/internal/config"
	"focus/internal/store"
)

func main() {
	cfg := config.Load()

	dbPath, err := store.DefaultDBPath()
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

	// Mark overdue todos from previous days.
	_ = st.MarkOverdue()

	m := app.New(cfg, st)
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.Shell.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
