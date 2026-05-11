package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"deck-game-installer/redist"
	"deck-game-installer/steammeta"
)

// InstallSteamRedists installs Steam-declared common redistributables into the shortcut prefix.
type InstallSteamRedists struct{}

func NewInstallSteamRedists() *InstallSteamRedists { return &InstallSteamRedists{} }
func (s *InstallSteamRedists) Name() string        { return "Install Redists" }

func (s *InstallSteamRedists) Execute(ctx context.Context, state *State) error {
	source := redistSourcePath(state)
	if source == "" {
		state.UI.Log("No source path available for Steam metadata lookup; skipping redists.")
		return nil
	}

	state.UI.Log("Identifying matching Steam app...")
	identity, err := steammeta.NewIdentifier().IdentifyWithHints(ctx, source, []string{state.GameName})
	if err != nil {
		state.UI.Log("Steam metadata not identified: " + err.Error())
		return nil
	}
	if identity.AppID == 0 || identity.Confidence < 0.92 {
		state.UI.Log("Steam metadata not identified with high confidence; skipping redists.")
		return nil
	}

	name := identity.Name
	if name == "" {
		name = fmt.Sprintf("app %d", identity.AppID)
	}
	state.UI.Log(fmt.Sprintf("Matched Steam %s (%d): %s", name, identity.AppID, identity.Reason))

	state.UI.Log("Fetching Steam common redistributables from appinfo...")
	redists, err := steammeta.ResolveCommonRedists(ctx, identity.AppID)
	if err != nil {
		state.UI.Log("Could not fetch Steam redist metadata: " + err.Error())
		return nil
	}
	if len(redists) == 0 {
		state.UI.Log("No Steam common redistributables declared for this app.")
		return nil
	}

	for _, item := range redists {
		if len(item.Verbs) == 0 {
			state.UI.Log(fmt.Sprintf("Redist %s (%s) has no winetricks mapping; skipping it.", item.Name, item.DepotID))
			continue
		}
		state.UI.Log(fmt.Sprintf("Redist %s (%s) -> %s", item.Name, item.DepotID, strings.Join(item.Verbs, ", ")))
	}

	verbs := steammeta.WinetricksVerbs(redists)
	if len(verbs) == 0 {
		state.UI.Log("No installable winetricks verbs were resolved from Steam redists.")
		return nil
	}

	installer := redist.NewInstaller()
	for {
		err := installer.InstallRedists(ctx, state.AppID, state.Proton.PrefixRoot(state.AppID), verbs, state.UI.Log)
		if err == nil {
			state.UI.Log("Redistributables installed.")
			return nil
		}
		if redist.IsMissingTool(err) {
			if !s.handleMissingTool(state, err) {
				state.UI.Log("Skipping redistributable installation.")
				return nil
			}
			continue
		}
		choice, ok := state.UI.Select(
			"Redist Installation Failed",
			"Could not install the Steam redistributables:\n\n"+err.Error(),
			[]string{"Retry", "Skip Dependencies"},
		)
		if !ok || choice != "Retry" {
			state.UI.Log("Skipping redistributable installation after failure.")
			return nil
		}
	}
}

func redistSourcePath(state *State) string {
	switch {
	case state.GameExePath != "":
		return state.GameExePath
	case state.InstallerPath != "":
		return state.InstallerPath
	default:
		return state.InputPath
	}
}

func (s *InstallSteamRedists) handleMissingTool(state *State, cause error) bool {
	for {
		choice, ok := state.UI.Select(
			"Install Protontricks",
			"Steam redists were found, but dependency installation needs Protontricks.\n\n"+
				"Reason: "+cause.Error()+"\n\n"+
				"Install Protontricks from Discover/Flathub, then choose Retry.",
			[]string{"Open Protontricks in Discover", "Retry", "Skip Dependencies"},
		)
		if !ok || choice == "Skip Dependencies" {
			return false
		}
		if choice == "Retry" {
			return true
		}
		openProtontricksPage(state)
	}
}

func openProtontricksPage(state *State) {
	const appstreamURL = "appstream://com.github.Matoking.protontricks"
	if err := exec.Command("xdg-open", appstreamURL).Start(); err != nil {
		const flathubURL = "https://flathub.org/apps/com.github.Matoking.protontricks"
		if fallbackErr := exec.Command("xdg-open", flathubURL).Start(); fallbackErr != nil {
			state.UI.Log("Could not open Protontricks page: " + err.Error())
			return
		}
		state.UI.Log("Opened Protontricks install page.")
		return
	}
	state.UI.Log("Opened Protontricks in Discover.")
}
