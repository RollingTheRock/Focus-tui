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
