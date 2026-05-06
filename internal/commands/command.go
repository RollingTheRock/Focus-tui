package commands

import (
	"context"

	"focus/internal/store"
)

// Executor is the interface for all domain commands.
// Validate performs pre-flight business-rule checks.
// Execute carries out the side effects (typically via store methods).
type Executor interface {
	Validate() error
	Execute(ctx context.Context, s *store.Store) error
}

// Bus is a thin coordinator that validates and dispatches commands.
// It can be extended later with logging, metrics, or async dispatch.
type Bus struct {
	store *store.Store
}

// NewBus creates a command bus backed by the given store.
func NewBus(s *store.Store) *Bus {
	return &Bus{store: s}
}

// Send validates the command and, if valid, executes it.
func (b *Bus) Send(ctx context.Context, cmd Executor) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	return cmd.Execute(ctx, b.store)
}
