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
	       // Log AppID before configuring Proton
	       state.UI.Log(fmt.Sprintf("Configuring Proton for AppID: %d", state.AppID))
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
	       state.UI.Log(fmt.Sprintf("Available Proton versions: %v", versions))
	       if len(versions) == 0 {
		       state.UI.Log("No Proton versions found!")
		       return fmt.Errorf("no Proton versions found")
	       }

	       // Automatically pick Proton Experimental, or first available
	       protonVersion := pickDefaultProton(versions)
	       state.UI.Log(fmt.Sprintf("Setting Proton version to: %s", protonVersion))

	       err = state.Steam.SetProtonVersion(state.AppID, protonVersion)
	       if err != nil {
		       state.UI.Log(fmt.Sprintf("Failed to set Proton version: %v", err))
		       return fmt.Errorf("failed to set Proton version: %w", err)
	       }
	       state.UI.Log(fmt.Sprintf("Successfully set Proton version to: %s for AppID: %d", protonVersion, state.AppID))

	       state.ProtonVersion = protonVersion
	       state.ProtonConfigChanged = true

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


