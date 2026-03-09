package installer

import (
	"context"
	"fmt"
	"path/filepath"
)

// FindGame scans the Proton prefix for installed game executables.
type FindGame struct {
}

// NewFindGame creates a new find game step.
func NewFindGame() *FindGame {
	return &FindGame{}
}

func (s *FindGame) Name() string {
	return "Find Game"
}

func (s *FindGame) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Scanning Proton prefix for installed game...")

	executables, err := state.Proton.ScanPrefixForExecutables(state.AppID)
	if err != nil {
		return fmt.Errorf("failed to scan for game executables: %w", err)
	}

	for len(executables) == 0 {
		retry := state.UI.ConfirmRetry(
			"No Game Executables Found",
			"Could not find any game executables in the Proton prefix.\n\n"+
				"This might happen if:\n"+
				"• The installer is still running\n"+
				"• The installer didn't complete successfully\n"+
				"• The game installed to a non-standard location\n\n"+
				"Finish the installation and click \"Scan Again\", or cancel to stop here.\n"+
				"The Steam shortcut will be left in place — to remove it, right-click\n"+
				"the game in Steam → Manage → Remove from your library.",
		)
		if !retry {
			state.UI.Log("Cancelled — Steam shortcut left in place.")
			state.UI.Log("To remove it: right-click the game in Steam → Manage → Remove from your library.")
			return fmt.Errorf("no game executables found — shortcut left in place")
		}

		state.UI.Log("Scanning again...")
		executables, err = state.Proton.ScanPrefixForExecutables(state.AppID)
		if err != nil {
			return fmt.Errorf("failed to scan for game executables: %w", err)
		}
	}

	state.UI.Log(fmt.Sprintf("Found %d potential game executable(s)", len(executables)))

	// Always show selection dialog - let user confirm even with one option
	selected, ok := state.UI.Select(
		"Select Game Executable",
		"Select the game executable:",
		executables,
	)

	if !ok {
		state.UI.Log("Selection cancelled — Steam shortcut left in place.")
		state.UI.Log("To remove it: right-click the game in Steam → Manage → Remove from your library.")
		return fmt.Errorf("executable selection cancelled — shortcut left in place")
	}

	state.GameExePath = selected
	state.UI.Log("Selected: " + filepath.Base(selected))
	return nil
}
