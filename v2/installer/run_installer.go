package installer

import (
	"context"
	"fmt"
	"time"

)

// RunInstaller launches the installer via Steam.
type RunInstaller struct {
	BaseStep
}

// NewRunInstaller creates a new run installer step.
func NewRunInstaller() *RunInstaller {
	return &RunInstaller{}
}

func (s *RunInstaller) Name() string {
	return "Run Installer"
}

func (s *RunInstaller) Description(state *State) string {
	return "Installer launched via Steam"
}

func (s *RunInstaller) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Launching installer via Steam...")

	// Give Steam a moment to be ready
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}

	if err := state.Steam.LaunchApp(state.AppID); err != nil {
		return fmt.Errorf("failed to launch installer: %w", err)
	}

	state.UI.Log("Installer launched")
	return nil
}

// CanRollback returns false - can't undo running an installer.
func (s *RunInstaller) CanRollback() bool {
	return false
}
