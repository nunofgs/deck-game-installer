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

	// List available versions first — we need them to validate any existing value.
	versions := state.Proton.GetAvailableProtonVersions()
	if len(versions) == 0 {
		return fmt.Errorf("no Proton versions found")
	}
	state.UI.Log(fmt.Sprintf("Available Proton versions: %v", versions))

	// Skip if already configured with a valid/installed version.
	if existing, err := state.Steam.GetProtonVersion(state.AppID); err == nil && existing != "" {
		if isInstalledVersion(existing, versions) {
			state.UI.Log("Proton already configured: " + existing)
			state.ProtonVersion = existing
			return nil
		}
		state.UI.Log(fmt.Sprintf("Existing Proton value %q is not installed, overwriting", existing))
	}

	protonVersion := versions[0] // GetAvailableProtonVersions sorts Experimental first
	state.UI.Log("Setting Proton version: " + protonVersion)

	if err := state.Steam.SetProtonVersion(state.AppID, protonVersion); err != nil {
		return fmt.Errorf("failed to set Proton version: %w", err)
	}

	state.ProtonVersion = protonVersion
	state.UI.Log("Proton configured: " + protonVersion)
	return nil
}

// isInstalledVersion returns true if name matches one of the installed Proton versions.
func isInstalledVersion(name string, installed []string) bool {
	for _, v := range installed {
		if v == name {
			return true
		}
	}
	return false
}
