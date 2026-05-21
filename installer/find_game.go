package installer

import (
	"context"
	"fmt"
	"path/filepath"

	"deck-game-installer/ui"
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
		action := state.UI.ConfirmRetryOrWait(
			"No Game Executables Found",
			"Could not find any game executables in the Proton prefix.\n\n"+
				"This might happen if:\n"+
				"• The installer is still running\n"+
				"• The installer didn't complete successfully\n"+
				"• The game installed to a non-standard location\n\n"+
				"If the installer is still running, click \"Keep Waiting\" and then\n"+
				"click the button once it finishes. Or click \"Scan Again\" to retry now.\n"+
				"The Steam shortcut will be left in place if you cancel — to remove it,\n"+
				"right-click the game in Steam → Manage → Remove from your library.",
		)

		switch action {
		case ui.ActionCancel:
			state.UI.Log("Cancelled — Steam shortcut left in place.")
			state.UI.Log("To remove it: right-click the game in Steam → Manage → Remove from your library.")
			return fmt.Errorf("no game executables found — shortcut left in place")

		case ui.ActionKeepWaiting:
			state.UI.Log("Waiting for installer to finish...")
			manualCh := state.UI.WaitWithManualOverride()
			select {
			case <-manualCh:
				state.UI.Log("Proceeding...")
			case <-ctx.Done():
				return ctx.Err()
			}
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
