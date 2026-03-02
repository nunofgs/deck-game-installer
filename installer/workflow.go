package installer

import (
	"context"
	"fmt"
)

// Runner executes a sequence of steps.
type Runner struct {
	steps []Step
	state *State
}

// NewRunner creates a new workflow runner with the given state.
func NewRunner(state *State) *Runner {
	return &Runner{state: state}
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

// State returns the current workflow state.
func (r *Runner) State() *State {
	return r.state
}

// Run executes all steps in order, stopping on the first error.
func (r *Runner) Run(ctx context.Context) error {
	stepNames := make([]string, len(r.steps))
	for i, step := range r.steps {
		stepNames[i] = step.Name()
	}
	r.state.UI.ConfigureSteps(stepNames)

	for _, step := range r.steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.state.UI.StepStarted(step.Name())
		r.state.UI.Log(fmt.Sprintf("Starting: %s", step.Name()))

		if err := step.Execute(ctx, r.state); err != nil {
			r.state.UI.Log(fmt.Sprintf("Failed: %s - %v", step.Name(), err))
			r.state.UI.StepCompleted(step.Name(), err)
			r.cleanup()
			return err
		}

		r.state.UI.StepCompleted(step.Name(), nil)
		r.state.UI.Log(fmt.Sprintf("Completed: %s", step.Name()))
	}

	return nil
}

// cleanup silently unmounts any ISO or SMB share we mounted.
func (r *Runner) cleanup() {
	s := r.state
	if s.ISOManager != nil && !s.ISOManager.WasExisting() {
		s.UI.Log("Unmounting ISO...")
		s.ISOManager.Unmount()
		s.ISOManager = nil
	}
	if s.SMBMount != nil && !s.SMBMount.WasExisting() {
		s.UI.Log("Unmounting network share...")
		if err := s.SMBMount.Unmount(); err != nil {
			s.UI.Log("Warning: failed to unmount SMB share: " + err.Error())
		}
		s.SMBMount = nil
	}
}
