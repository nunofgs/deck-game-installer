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

	if len(executables) == 0 {
		keep := state.UI.Confirm(
			"No Game Found",
			"Could not find any game executables in the Proton prefix.\n\n"+
				"This might happen if:\n"+
				"• The installer didn't complete successfully\n"+
				"• The game installed to a non-standard location\n\n"+
				"Keep the Steam shortcut so you can configure it manually later?",
		)
		if keep {
			state.UI.Log("Keeping shortcut for manual configuration")
			return nil
		}
		state.UI.Log("Removing Steam shortcut...")
		if err := state.Steam.DeleteShortcut(state.AppID); err != nil {
			return fmt.Errorf("no game executables found; also failed to remove shortcut: %w", err)
		}
		state.UI.Log("Restarting Steam to apply shortcut removal...")
		if err := state.Steam.ShutdownSteam(ctx); err != nil {
			return fmt.Errorf("shortcut removed but failed to restart Steam: %w", err)
		}
		if err := state.Steam.StartSteam(); err != nil {
			return fmt.Errorf("shortcut removed but failed to start Steam: %w", err)
		}
		return fmt.Errorf("no game executables found — shortcut removed")
	}

	state.UI.Log(fmt.Sprintf("Found %d potential game executable(s)", len(executables)))

	// Always show selection dialog - let user confirm even with one option
	selected, ok := state.UI.Select(
		"Select Game Executable",
		"Select the game executable:",
		executables,
	)

	if !ok {
		// User cancelled, keep original shortcut
		state.UI.Log("Selection cancelled, keeping original shortcut")
		return nil
	}

	// Find the selected path
	for _, exe := range executables {
		if exe == selected {
			state.GameExePath = exe
			state.GameStartDir = filepath.Dir(exe)
			break
		}
	}

	state.UI.Log("Selected: " + filepath.Base(state.GameExePath))
	return nil
}


