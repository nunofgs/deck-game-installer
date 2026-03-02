package installer

import (
	"context"
	"fmt"
)

// ShutdownSteam shuts down the Steam client.
type ShutdownSteam struct {
	BaseStep
}

func NewShutdownSteam() *ShutdownSteam { return &ShutdownSteam{} }
func (s *ShutdownSteam) Name() string  { return "Shutdown Steam" }

func (s *ShutdownSteam) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Shutting down Steam...")
	if err := state.Steam.ShutdownSteam(ctx); err != nil {
		return fmt.Errorf("failed to shut down Steam: %w", err)
	}
	state.UI.Log("Steam shut down")
	return nil
}
