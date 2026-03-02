package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// UpdateShortcut modifies the Steam shortcut to point to the installed game.
type UpdateShortcut struct {
	BaseStep
}

// NewUpdateShortcut creates a new update shortcut step.
func NewUpdateShortcut() *UpdateShortcut {
	return &UpdateShortcut{}
}

func (s *UpdateShortcut) Name() string {
	return "Update Shortcut"
}


func (s *UpdateShortcut) Execute(ctx context.Context, state *State) error {
	// If no game was found/selected, skip this step
	if state.GameExePath == "" {
		state.UI.Log("No game executable selected, keeping original shortcut")
		return nil
	}

	state.UI.Log("Updating Steam shortcut to point to game...")

	// Remove "(Installer)" suffix from game name
	gameName := strings.TrimSuffix(state.GameName, " (Installer)")

	err := state.Steam.UpdateShortcut(
		state.AppID,
		state.GameExePath,
		state.GameStartDir,
		gameName,
	)

	if err != nil {
		return fmt.Errorf("failed to update shortcut: %w", err)
	}

	state.UI.Log(fmt.Sprintf("Shortcut updated: %s", gameName))
	state.UI.Log(fmt.Sprintf("Game path: %s", filepath.Base(state.GameExePath)))

	return nil
}


