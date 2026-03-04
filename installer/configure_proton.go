package installer

import (
	"context"
	"fmt"
	"os"
)

// ConfigureProton sets the Proton version for the Steam shortcut in config.vdf.
type ConfigureProton struct {
}

func NewConfigureProton() *ConfigureProton { return &ConfigureProton{} }
func (s *ConfigureProton) Name() string    { return "Configure Proton" }

func (s *ConfigureProton) Execute(ctx context.Context, state *State) error {
	state.UI.Log(fmt.Sprintf("[ConfigureProton] Starting. AppID=%d (raw int32), uint32=%d", state.AppID, uint32(state.AppID)))

	// Dump config.vdf BEFORE any changes so we can inspect the baseline.
	configPath := state.Steam.ConfigPath()
	state.UI.Log(fmt.Sprintf("[ConfigureProton] config.vdf path: %s", configPath))
	if raw, err := os.ReadFile(configPath); err != nil {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] WARNING: could not read config.vdf before changes: %v", err))
	} else {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] config.vdf size before: %d bytes", len(raw)))
		// Print the full file — we need to see it all.
		state.UI.Log("[ConfigureProton] === config.vdf BEFORE ===")
		state.UI.Log(string(raw))
		state.UI.Log("[ConfigureProton] === end config.vdf BEFORE ===")
	}

	// Check if already configured.
	state.UI.Log("[ConfigureProton] Calling GetProtonVersion...")
	existing, err := state.Steam.GetProtonVersion(state.AppID)
	state.UI.Log(fmt.Sprintf("[ConfigureProton] GetProtonVersion returned: version=%q err=%v", existing, err))

	if err == nil && existing != "" {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] SKIPPING — Proton already configured as %q. Will NOT overwrite.", existing))
		state.ProtonVersion = existing
		return nil
	}
	state.UI.Log(fmt.Sprintf("[ConfigureProton] No existing Proton config found (err=%v, version=%q). Will set a new one.", err, existing))

	// List available versions.
	state.UI.Log("[ConfigureProton] Calling GetAvailableProtonVersions...")
	versions := state.Proton.GetAvailableProtonVersions()
	state.UI.Log(fmt.Sprintf("[ConfigureProton] GetAvailableProtonVersions returned %d version(s):", len(versions)))
	for i, v := range versions {
		state.UI.Log(fmt.Sprintf("[ConfigureProton]   [%d] %q", i, v))
	}

	if len(versions) == 0 {
		state.UI.Log("[ConfigureProton] ERROR: no Proton versions found — cannot configure")
		return fmt.Errorf("no Proton versions found")
	}

	protonVersion := versions[0] // GetAvailableProtonVersions sorts Experimental first
	state.UI.Log(fmt.Sprintf("[ConfigureProton] Chosen Proton version: %q (index 0)", protonVersion))

	state.UI.Log(fmt.Sprintf("[ConfigureProton] Calling SetProtonVersion(appID=%d, version=%q)...", state.AppID, protonVersion))
	if err := state.Steam.SetProtonVersion(state.AppID, protonVersion); err != nil {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] ERROR from SetProtonVersion: %v", err))
		return fmt.Errorf("failed to set Proton version: %w", err)
	}
	state.UI.Log("[ConfigureProton] SetProtonVersion succeeded.")

	// Dump config.vdf AFTER changes.
	if raw, err := os.ReadFile(configPath); err != nil {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] WARNING: could not read config.vdf after changes: %v", err))
	} else {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] config.vdf size after: %d bytes", len(raw)))
		state.UI.Log("[ConfigureProton] === config.vdf AFTER ===")
		state.UI.Log(string(raw))
		state.UI.Log("[ConfigureProton] === end config.vdf AFTER ===")
	}

	// Verify the write actually took effect by reading it back.
	state.UI.Log("[ConfigureProton] Verifying by calling GetProtonVersion again...")
	verify, verifyErr := state.Steam.GetProtonVersion(state.AppID)
	state.UI.Log(fmt.Sprintf("[ConfigureProton] Verification: version=%q err=%v", verify, verifyErr))
	if verifyErr != nil || verify == "" {
		state.UI.Log("[ConfigureProton] WARNING: verification failed — the write may not have persisted!")
	} else {
		state.UI.Log(fmt.Sprintf("[ConfigureProton] Verification OK — Proton is now set to %q", verify))
	}

	state.ProtonVersion = protonVersion
	state.UI.Log(fmt.Sprintf("[ConfigureProton] Done. state.ProtonVersion=%q", state.ProtonVersion))
	return nil
}
