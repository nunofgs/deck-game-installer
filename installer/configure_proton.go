package installer

import (
	"context"
	"fmt"
)

// ConfigureProton sets the Proton version for the Steam shortcut in config.vdf.
type ConfigureProton struct {
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

	protonVersion := versions[0] // GetAvailableProtonVersions sorts Experimental first
	state.UI.Log("Setting Proton version: " + protonVersion)

	if err := state.Steam.SetProtonVersion(state.AppID, protonVersion); err != nil {
		return fmt.Errorf("failed to set Proton version: %w", err)
	}

	state.ProtonVersion = protonVersion
	state.UI.Log("Proton configured: " + protonVersion)
	return nil
}


