package installer

import (
	"context"
	"fmt"

	"deck-game-installer/steam"
)

// RunInstaller launches the installer by starting Steam with the launch URL.
type RunInstaller struct {
	BaseStep
}

func NewRunInstaller() *RunInstaller { return &RunInstaller{} }
func (s *RunInstaller) Name() string { return "Run Installer" }

func (s *RunInstaller) Execute(ctx context.Context, state *State) error {
	urlAppID := steam.GetURLAppID(state.AppID)
	url := "steam://rungameid/" + urlAppID

	state.UI.Log("Starting Steam with installer URL: " + url)
	if err := state.Steam.StartSteam(url); err != nil {
		return fmt.Errorf("failed to start Steam: %w", err)
	}
	state.UI.Log("Steam launched")
	return nil
}
