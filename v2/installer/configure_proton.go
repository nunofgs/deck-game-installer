package installer

import (
	"context"
	"fmt"
	"strings"
)

// ConfigureProton sets the Proton version for the Steam shortcut.
type ConfigureProton struct {
	BaseStep
}

// NewConfigureProton creates a new configure Proton step.
func NewConfigureProton() *ConfigureProton {
	return &ConfigureProton{}
}

func (s *ConfigureProton) Name() string {
	return "Configure Proton"
}

func (s *ConfigureProton) Description(state *State) string {
	if state.ProtonVersion != "" {
		return fmt.Sprintf("Proton configured: %s", state.ProtonVersion)
	}
	return "Proton configuration"
}

func (s *ConfigureProton) Execute(ctx context.Context, state *State) error {
	// Check if Proton is already configured
	existing, err := state.Steam.GetProtonVersion(state.AppID)
	if err == nil && existing != "" {
		state.UI.Log(fmt.Sprintf("Proton already configured: %s", existing))
		state.ProtonVersion = existing
		state.ProtonConfigChanged = false
		return nil
	}

	// Get available Proton versions
	versions := state.Proton.GetAvailableProtonVersions()
	if len(versions) == 0 {
		return fmt.Errorf("no Proton versions found")
	}

	// Automatically pick Proton Experimental, or first available
	protonVersion := pickDefaultProton(versions)

	state.UI.Log(fmt.Sprintf("Setting Proton version to: %s", protonVersion))

	if err := state.Steam.SetProtonVersion(state.AppID, protonVersion); err != nil {
		return fmt.Errorf("failed to set Proton version: %w", err)
	}

	state.ProtonVersion = protonVersion
	state.ProtonConfigChanged = true
	state.SteamRestartRequired = true

	state.UI.Log("Proton configuration updated")
	return nil
}

// pickDefaultProton selects Proton Experimental if available, otherwise the first version.
func pickDefaultProton(versions []string) string {
	for _, v := range versions {
		if strings.Contains(strings.ToLower(v), "experimental") {
			return v
		}
	}
	if len(versions) > 0 {
		return versions[0]
	}
	return ""
}

// CanRollback returns false - Proton config changes are harmless to leave.
func (s *ConfigureProton) CanRollback() bool {
	return false
}
