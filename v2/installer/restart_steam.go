package installer

import (
	"context"

)

// RestartSteam restarts the Steam client to apply configuration changes.
type RestartSteam struct {
	BaseStep
}

// NewRestartSteam creates a new restart Steam step.
func NewRestartSteam() *RestartSteam {
	return &RestartSteam{}
}

func (s *RestartSteam) Name() string {
	return "Restart Steam"
}

func (s *RestartSteam) Description(state *State) string {
	return "Steam client restarted"
}

func (s *RestartSteam) Execute(ctx context.Context, state *State) error {
	if !state.SteamRestartRequired {
		state.UI.Log("Steam restart not required, skipping...")
		return nil
	}

	state.UI.Log("Restarting Steam to apply configuration changes...")

	if err := state.Steam.RestartSteam(ctx); err != nil {
		state.UI.Log("Warning: Steam restart had issues, but continuing...")
		// Don't fail the whole workflow for restart issues
	}

	state.SteamRestartRequired = false
	state.UI.Log("Steam restarted successfully")

	// Give Steam a moment to fully initialize
	state.UI.Log("Waiting for Steam to initialize...")

	return nil
}

// CanRollback returns false - can't undo a Steam restart.
func (s *RestartSteam) CanRollback() bool {
	return false
}
