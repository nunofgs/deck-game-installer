package installer

import (
	"context"
)

// ShutdownSteam shuts down the Steam client.
type ShutdownSteam struct {
	BaseStep
}

func NewShutdownSteam() *ShutdownSteam { return &ShutdownSteam{} }
func (s *ShutdownSteam) Name() string  { return "Shutdown Steam" }
func (s *ShutdownSteam) Description(state *State) string {
	return "Steam client shut down"
}
func (s *ShutdownSteam) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Shutting down Steam...")
	if err := state.Steam.ShutdownSteam(ctx); err != nil {
		state.UI.Log("Warning: Steam shutdown had issues, continuing...")
	}
	state.UI.Log("Steam shut down")
	return nil
}

// StartSteam launches the Steam client.
type StartSteam struct {
	BaseStep
}

func NewStartSteam() *StartSteam { return &StartSteam{} }
func (s *StartSteam) Name() string {
	return "Start Steam"
}
func (s *StartSteam) Description(state *State) string {
	return "Steam client started"
}
func (s *StartSteam) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Starting Steam...")
	state.Steam.StartSteam()
	state.UI.Log("Steam started")
	return nil
}
