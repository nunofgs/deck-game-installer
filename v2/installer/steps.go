package installer

import (
	"context"
)

// Step represents a single discrete operation in the installation workflow.
// Each step should be focused on one specific task (mounting, adding shortcut, etc.)
// and implement rollback logic if the operation is reversible.
type Step interface {
	// Name returns a short identifier for the step (used for logging and step tracking).
	Name() string

	// Description returns a human-readable description of what this step did.
	// This is shown in the rollback dialog to help users understand what will be undone.
	Description(state *State) string

	// Execute performs the step's operation.
	// It should respect context cancellation and return early if ctx.Done() is signaled.
	// Any data needed by later steps should be stored in state.
	Execute(ctx context.Context, state *State) error

	// Rollback undoes the step's operation.
	// This is called when a later step fails and the user chooses to roll back.
	// Steps that cannot be rolled back (e.g., restart Steam) should return nil.
	Rollback(ctx context.Context, state *State) error

	// CanRollback returns true if this step's operation can be undone.
	// Steps that return false will still appear in the rollback dialog but won't be rolled back.
	CanRollback() bool
}

// BaseStep provides a default implementation for steps that cannot roll back.
// Embed this in step implementations that don't need rollback functionality.
type BaseStep struct{}

// Rollback is a no-op for steps that cannot be rolled back.
func (b *BaseStep) Rollback(ctx context.Context, state *State) error {
	return nil
}

// CanRollback returns false by default.
func (b *BaseStep) CanRollback() bool {
	return false
}
