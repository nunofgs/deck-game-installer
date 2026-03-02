package installer

import (
	"context"
)

// FinalRestart restarts Steam one final time to recognize the updated shortcut.
type FinalRestart struct {
	BaseStep
}

func NewFinalRestart() *FinalRestart { return &FinalRestart{} }
func (s *FinalRestart) Name() string { return "Finalize" }

func (s *FinalRestart) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Restarting Steam to apply shortcut changes...")

	if err := state.Steam.ShutdownSteam(ctx); err != nil {
		state.UI.Log("Warning: Steam shutdown had issues: " + err.Error())
	}

	state.Steam.StartSteam()

	state.UI.Info("Installation Complete", s.completionMessage(state))
	return nil
}

func (s *FinalRestart) completionMessage(state *State) string {
	if state.GameExePath != "" {
		return "Your game has been added to Steam and is ready to play.\n" +
			"You can find it in your Steam library under Non-Steam games."
	}
	return "The installer shortcut has been created.\n" +
		"You may need to configure the game executable manually in Steam."
}
