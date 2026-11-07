package installer

import (
	"context"
	"fmt"
	"path/filepath"
)

// AddToSteam creates a Steam shortcut for the installer or portable game.
type AddToSteam struct {
	installer bool
}

func NewAddToSteam() *AddToSteam     { return &AddToSteam{installer: true} }
func NewAddGameToSteam() *AddToSteam { return &AddToSteam{} }
func (s *AddToSteam) Name() string   { return "Add to Steam" }

func (s *AddToSteam) Execute(ctx context.Context, state *State) error {
	if state.GameName == "" {
		state.GameName = DeriveGameName(state.InstallerPath)
	}

	appName := state.GameName
	if s.installer {
		appName += " (Installer)"
	}
	startDir := filepath.Dir(state.InstallerPath)

	state.UI.Log("Creating Steam shortcut: " + appName)

	appID, err := state.Steam.AddShortcut(appName, state.InstallerPath, "", startDir)
	if err != nil {
		return fmt.Errorf("failed to create Steam shortcut: %w", err)
	}

	state.AppID = appID
	state.UI.Log(fmt.Sprintf("Shortcut created with app ID: %d", appID))
	return nil
}
