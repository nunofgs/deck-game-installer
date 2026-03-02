package installer

import (
	"context"
)

// Step represents a single discrete operation in the installation workflow.
type Step interface {
	// Name returns a short identifier for the step (used for logging and progress tracking).
	Name() string

	// Description returns a human-readable description of what this step did.
	Description(state *State) string

	// Execute performs the step's operation.
	// It should respect context cancellation and return early if ctx.Done() is signaled.
	// Any data needed by later steps should be stored in state.
	Execute(ctx context.Context, state *State) error
}

// BaseStep can be embedded by steps that don't need any extra behaviour.
type BaseStep struct{}
