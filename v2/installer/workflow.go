package installer

import (
	"context"
	"fmt"
)

// Runner executes a sequence of steps and handles rollback on failure.
type Runner struct {
	steps          []Step
	completedSteps []Step
	state          *State
}

// NewRunner creates a new workflow runner with the given state.
func NewRunner(state *State) *Runner {
	return &Runner{
		steps:          make([]Step, 0),
		completedSteps: make([]Step, 0),
		state:          state,
	}
}

// AddStep adds a step to the workflow pipeline.
func (r *Runner) AddStep(step Step) *Runner {
	r.steps = append(r.steps, step)
	return r
}

// AddSteps adds multiple steps to the workflow pipeline.
func (r *Runner) AddSteps(stepList ...Step) *Runner {
	r.steps = append(r.steps, stepList...)
	return r
}

// Run executes all steps in order.
// If a step fails, it prompts the user to roll back completed steps.
// Returns the error that caused the failure, or nil if all steps succeeded.
func (r *Runner) Run(ctx context.Context) error {
	// Configure UI with step names for progress display
	stepNames := make([]string, len(r.steps))
	for i, step := range r.steps {
		stepNames[i] = step.Name()
	}
	r.state.UI.ConfigureSteps(stepNames)

	// Execute each step
	for _, step := range r.steps {
		// Check for cancellation before starting each step
		select {
		case <-ctx.Done():
			r.handleFailure(ctx, fmt.Errorf("cancelled"))
			return ctx.Err()
		default:
		}

		// Update UI to show current step
		r.state.UI.SetStep(step.Name())
		r.state.UI.Log(fmt.Sprintf("Starting: %s", step.Name()))

		// Execute the step
		if err := step.Execute(ctx, r.state); err != nil {
			r.state.UI.Log(fmt.Sprintf("Failed: %s - %v", step.Name(), err))
			r.handleFailure(ctx, err)
			return err
		}

		// Track completed steps for potential rollback
		r.completedSteps = append(r.completedSteps, step)
		r.state.UI.Log(fmt.Sprintf("Completed: %s", step.Name()))
	}

	r.state.UI.Log("Installation completed successfully!")
	return nil
}

// handleFailure prompts the user to roll back completed steps.
func (r *Runner) handleFailure(ctx context.Context, err error) {
	// Build list of completed operations that can be rolled back
	var rollbackableOps []string
	var rollbackableSteps []Step

	for _, step := range r.completedSteps {
		desc := step.Description(r.state)
		if step.CanRollback() {
			rollbackableOps = append(rollbackableOps, desc)
			rollbackableSteps = append(rollbackableSteps, step)
		}
	}

	// If nothing to roll back, just show the error
	if len(rollbackableSteps) == 0 {
		r.state.UI.Error("Installation Failed", err.Error())
		return
	}

	// Ask user if they want to roll back
	if r.state.UI.RollbackPrompt(err.Error(), rollbackableOps) {
		r.rollback(ctx, rollbackableSteps)
	}
}

// rollback undoes completed steps in reverse order.
func (r *Runner) rollback(ctx context.Context, stepsToRollback []Step) {
	r.state.UI.Log("Rolling back changes...")

	// Roll back in reverse order
	for i := len(stepsToRollback) - 1; i >= 0; i-- {
		step := stepsToRollback[i]
		r.state.UI.Log(fmt.Sprintf("Undoing: %s", step.Description(r.state)))

		if err := step.Rollback(ctx, r.state); err != nil {
			r.state.UI.Log(fmt.Sprintf("Warning: Failed to roll back %s: %v", step.Name(), err))
			// Continue rolling back other steps even if one fails
		}
	}

	r.state.UI.Log("Rollback complete.")
}

// State returns the current workflow state.
func (r *Runner) State() *State {
	return r.state
}
