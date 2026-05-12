package installer

import (
	"context"
	"fmt"
)

// StartSteamForRedists starts Steam and waits for it to load shortcuts so
// protontricks can find the newly-added shortcut when installing redists.
type StartSteamForRedists struct{}

func NewStartSteamForRedists() *StartSteamForRedists { return &StartSteamForRedists{} }
func (s *StartSteamForRedists) Name() string         { return "Start Steam" }

func (s *StartSteamForRedists) Execute(ctx context.Context, state *State) error {
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
