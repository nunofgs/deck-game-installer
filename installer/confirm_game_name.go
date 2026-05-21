package installer

import (
	"context"
	"fmt"
	"strings"
)

// ConfirmGameName prompts the user to enter the game name when it cannot be
// determined automatically from the file path.
type ConfirmGameName struct{}

func NewConfirmGameName() *ConfirmGameName { return &ConfirmGameName{} }
func (s *ConfirmGameName) Name() string    { return "Game Name" }

func (s *ConfirmGameName) Execute(ctx context.Context, state *State) error {
	if state.GameName != "" {
		state.UI.Log("Game name: " + state.GameName)
		return nil
	}

	name, ok := state.UI.PromptText(
		"Enter Game Name",
		"Could not determine the game name from the file path.\nPlease enter the name of the game:",
		"",
	)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("game name is required to continue")
	}

	state.GameName = strings.TrimSpace(name)
	state.UI.Log("Game name: " + state.GameName)
	return nil
}
