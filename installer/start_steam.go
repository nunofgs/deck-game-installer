package installer

import (
	"context"
	"fmt"
)

// StartSteam loads the updated shortcut after the installer finishes.
type StartSteam struct{}

func NewStartSteam() *StartSteam   { return &StartSteam{} }
func (s *StartSteam) Name() string { return "Start Steam" }

func (s *StartSteam) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Starting Steam to load shortcut...")
	if err := state.Steam.StartSteam(); err != nil {
		return fmt.Errorf("failed to start Steam: %w", err)
	}
	state.UI.Log("Waiting for Steam to load...")
	if err := state.Steam.WaitForSteamReady(ctx); err != nil {
		return fmt.Errorf("Steam did not become ready: %w", err)
	}
	state.UI.Log("Steam ready.")
	return nil
}
