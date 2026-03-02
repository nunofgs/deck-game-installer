package installer

import (
	"context"
	"fmt"
)

// FinalRestart restarts Steam one final time to recognize the updated shortcut.
type FinalRestart struct {
}

func NewFinalRestart() *FinalRestart { return &FinalRestart{} }
func (s *FinalRestart) Name() string { return "Finalize" }

func (s *FinalRestart) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Restarting Steam to apply shortcut changes...")

	if err := state.Steam.ShutdownSteam(ctx); err != nil {
		return fmt.Errorf("failed to shut down Steam: %w", err)
	}

	if err := state.Steam.StartSteam(); err != nil {
		return fmt.Errorf("failed to start Steam: %w", err)
	}

	return nil
}
