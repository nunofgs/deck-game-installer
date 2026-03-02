package installer

import (
	"context"

)

// FinalRestart restarts Steam one final time to recognize the updated shortcut.
type FinalRestart struct {
	BaseStep
}

// NewFinalRestart creates a new final restart step.
func NewFinalRestart() *FinalRestart {
	return &FinalRestart{}
}

func (s *FinalRestart) Name() string {
	return "Finalize"
}

func (s *FinalRestart) Description(state *State) string {
	return "Steam restarted with updated configuration"
}

func (s *FinalRestart) Execute(ctx context.Context, state *State) error {
	if !state.SteamRestartRequired {
		state.UI.Log("No Steam restart required")
		s.showCompletionMessage(state)
		return nil
	}

	state.UI.Log("Restarting Steam to apply shortcut changes...")

	if err := state.Steam.RestartSteam(ctx); err != nil {
		state.UI.Log("Warning: Steam restart had issues: " + err.Error())
		// Continue anyway - user can restart Steam manually
	}

	s.showCompletionMessage(state)
	return nil
}

func (s *FinalRestart) showCompletionMessage(state *State) {
	msg := "Installation complete!\n\n"

	if state.GameExePath != "" {
		msg += "Your game has been added to Steam and is ready to play.\n"
		msg += "You can find it in your Steam library under Non-Steam games."
	} else {
		msg += "The installer shortcut has been created.\n"
		msg += "You may need to configure the game executable manually in Steam."
	}

	state.UI.Info("Installation Complete", msg)
}

// CanRollback returns false - final step.
func (s *FinalRestart) CanRollback() bool {
	return false
}
