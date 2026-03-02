package installer

import "context"

// Step represents a single discrete operation in the installation workflow.
type Step interface {
	// Name returns a short identifier used for logging and progress display.
	Name() string

	// Execute performs the step. It should respect context cancellation and
	// store any results needed by later steps in state.
	Execute(ctx context.Context, state *State) error
}


