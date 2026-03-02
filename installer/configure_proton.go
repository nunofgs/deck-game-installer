package installer

import (
	"context"
	"fmt"
	"strings"
)

// ConfigureProton sets the Proton version for the Steam shortcut in config.vdf.
type ConfigureProton struct {
	BaseStep
}

func NewConfigureProton() *ConfigureProton { return &ConfigureProton{} }
func (s *ConfigureProton) Name() string    { return "Configure Proton" }

func (s *ConfigureProton) Execute(ctx context.Context, state *State) error {
	state.UI.Log(fmt.Sprintf("Configuring Proton for app ID: %d", state.AppID))

	// Skip if already configured.
	if existing, err := state.Steam.GetProtonVersion(state.AppID); err == nil && existing != "" {
		state.UI.Log("Proton already configured: " + existing)
		state.ProtonVersion = existing
		return nil
	}

	versions := state.Proton.GetAvailableProtonVersions()
	if len(versions) == 0 {
		return fmt.Errorf("no Proton versions found")
	}
	state.UI.Log(fmt.Sprintf("Available Proton versions: %v", versions))

	protonVersion := pickDefaultProton(versions)
	state.UI.Log("Setting Proton version: " + protonVersion)

	if err := state.Steam.SetProtonVersion(state.AppID, protonVersion); err != nil {
		return fmt.Errorf("failed to set Proton version: %w", err)
	}

	state.ProtonVersion = protonVersion
	state.UI.Log("Proton configured: " + protonVersion)
	return nil
}

// pickDefaultProton selects Proton Experimental if available, otherwise the first version.
func pickDefaultProton(versions []string) string {
	for _, v := range versions {
		if strings.Contains(strings.ToLower(v), "experimental") {
			return v
		}
	}
	return versions[0]
}
