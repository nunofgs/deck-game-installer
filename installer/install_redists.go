package installer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"deck-game-installer/redist"
	"deck-game-installer/steammeta"
)

var errInstallationCancelled = errors.New("installation cancelled")

// InstallSteamRedists installs Steam-declared common redistributables into the shortcut prefix.
type InstallSteamRedists struct{}

func NewInstallSteamRedists() *InstallSteamRedists { return &InstallSteamRedists{} }
func (s *InstallSteamRedists) Name() string        { return "Install Redistributables" }

func (s *InstallSteamRedists) Execute(ctx context.Context, state *State) error {
	source := redistSourcePath(state)
	if source == "" {
		state.UI.Log("No source path available for Steam metadata lookup; skipping redistributables.")
		return nil
	}

	state.UI.Log("Identifying matching Steam app...")
	identity, err := s.identifySteamApp(ctx, state, source)
	if err != nil {
		return err
	}
	if identity.AppID == 0 {
		state.UI.Log("Steam metadata not identified; skipping redistributables.")
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
		state.UI.Log("Could not fetch Steam redistributable metadata: " + err.Error())
		return nil
	}
	if len(redists) == 0 {
		state.UI.Log("No Steam common redistributables declared for this app.")
		return nil
	}

	for _, item := range redists {
		if len(item.Verbs) == 0 {
			if !steammeta.IsKnownRedistDepot(item.DepotID) {
				state.UI.Log(fmt.Sprintf("WARNING: unrecognised redistributable depot %s (%s) — no winetricks mapping known, skipping.", item.DepotID, item.Name))
			} else {
				state.UI.Log(fmt.Sprintf("Redistributable %s (%s) has no winetricks mapping; skipping.", item.Name, item.DepotID))
			}
			continue
		}
		state.UI.Log(fmt.Sprintf("Redistributable %s (%s) -> %s", item.Name, item.DepotID, strings.Join(item.Verbs, ", ")))
	}

	verbs := steammeta.WinetricksVerbs(redists)
	if len(verbs) == 0 {
		state.UI.Log("No installable winetricks verbs resolved from Steam redistributables.")
		return nil
	}

	protonDir := state.Proton.ProtonDirectory(state.ProtonVersion)
	prefixRoot := state.Proton.PrefixRoot(state.AppID)
	if protonDir != "" {
		state.UI.Log(fmt.Sprintf("Proton binary: %s (%s)", state.ProtonVersion, protonDir))
	}
	state.UI.Log("Game prefix: " + prefixRoot)

	installer := redist.NewInstaller()
	for {
		err := installer.InstallRedists(ctx, state.AppID, prefixRoot, protonDir, verbs, state.UI.Log)
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
		if isFlatpakPermissionError(err) {
			if !s.handleFlatpakPermission(state, err) {
				return nil
			}
			continue
		}
		choice, ok := state.UI.Select(
			"Redistributable Installation Failed",
			"Could not install the Steam redistributables:\n\n"+err.Error(),
			[]string{"Retry", "Skip Dependencies", "Cancel"},
		)
		if !ok || choice == "Cancel" {
			return errInstallationCancelled
		}
		if choice == "Skip Dependencies" {
			state.UI.Log("Skipping redistributable installation after failure.")
			return nil
		}
	}
}

func (s *InstallSteamRedists) identifySteamApp(ctx context.Context, state *State, source string) (steammeta.Identification, error) {
	identifier := steammeta.NewIdentifier()

	if identity, ok, err := identifier.IdentifyLocal(source); err != nil || ok {
		return identity, err
	}

	var candidates []steammeta.Identification
	for {
		var err error
		candidates, err = identifier.StoreSearchCandidates(ctx, source, []string{state.GameName})
		if err == nil {
			break
		}
		state.UI.Log("Steam Store search failed: " + err.Error())
		choice, ok := state.UI.Select(
			"Steam Store Search Failed",
			"Could not search the Steam Store:\n\n"+err.Error(),
			[]string{"Retry", "Skip"},
		)
		if !ok || choice == "Skip" {
			state.UI.Log("Skipping redistributable installation.")
			return steammeta.Identification{}, nil
		}
	}

	switch len(candidates) {
	case 0:
		return steammeta.Identification{}, nil
	case 1:
		return candidates[0], nil
	}

	options := make([]string, len(candidates))
	byOption := make(map[string]steammeta.Identification, len(candidates))
	for i, candidate := range candidates {
		option := fmt.Sprintf("%s (AppID %d)", candidate.Name, candidate.AppID)
		options[i] = option
		byOption[option] = candidate
	}

	selected, ok := state.UI.Select(
		"Select Steam App",
		"Multiple Steam Store results matched this game. Select the Steam app to use for redistributables:",
		options,
	)
	if !ok {
		return steammeta.Identification{}, errInstallationCancelled
	}
	return byOption[selected], nil
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
			"Steam redistributables were found, but dependency installation needs Protontricks.\n\n"+
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

func isFlatpakPermissionError(err error) bool {
	return strings.Contains(err.Error(), "does not appear to have access to")
}

func (s *InstallSteamRedists) handleFlatpakPermission(state *State, cause error) bool {
	grantCmd := flatpakOverrideCmd(cause.Error())

	for {
		choice, ok := state.UI.Select(
			"Protontricks Needs Filesystem Access",
			"Protontricks does not have permission to access your Steam directory.\n\n"+
				"Click \"Grant Access\" to fix this automatically, then the installation will continue.",
			[]string{"Grant Access", "Skip Dependencies"},
		)
		if !ok || choice == "Skip Dependencies" {
			state.UI.Log("Skipping redistributable installation.")
			return false
		}
		parts := strings.Fields(grantCmd)
		if out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput(); err != nil {
			state.UI.Log("Could not grant Flatpak access: " + err.Error() + ": " + strings.TrimSpace(string(out)))
			continue
		}
		state.UI.Log("Granted Flatpak filesystem access.")
		return true
	}
}

func flatpakOverrideCmd(errMsg string) string {
	for _, line := range strings.Split(errMsg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "flatpak override") {
			return strings.TrimSpace(line)
		}
	}
	return "flatpak override --user --filesystem=home"
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
