package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"focus/internal/app"
	"focus/internal/config"
	"focus/internal/projections"
	"focus/internal/store"
)

func main() {
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
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	p := tea.NewProgram(m, opts...)
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
