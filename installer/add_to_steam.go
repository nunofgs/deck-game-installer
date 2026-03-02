package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// AddToSteam creates a Steam shortcut for the installer.
type AddToSteam struct {
	BaseStep
}

func NewAddToSteam() *AddToSteam { return &AddToSteam{} }
func (s *AddToSteam) Name() string { return "Add to Steam" }

func (s *AddToSteam) Execute(ctx context.Context, state *State) error {
	if state.GameName == "" {
		state.GameName = DeriveGameName(state.InstallerPath)
	}

	appName := state.GameName + " (Installer)"
	startDir := filepath.Dir(state.InstallerPath)

	state.UI.Log("Creating Steam shortcut: " + appName)

	appID, err := state.Steam.AddShortcut(appName, state.InstallerPath, "", startDir)
	if err != nil {
		return fmt.Errorf("failed to create Steam shortcut: %w", err)
	}

	state.AppID = appID
	state.UI.Log(fmt.Sprintf("Shortcut created with app ID: %d", appID))
	return nil
}

// DeriveGameName extracts a human-readable game name from an installer path.
// It tries the parent directory name first, falling back to the filename.
func DeriveGameName(path string) string {
	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	name = strings.TrimSuffix(name, "_files")
	name = strings.TrimSuffix(name, "-files")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	genericNames := map[string]bool{
		"disc1": true, "disc2": true, "disk1": true, "disk2": true,
		"cd1": true, "cd2": true, "dvd1": true, "dvd2": true,
		"setup": true, "install": true, "installer": true,
		".": true, "/": true, "home": true, "downloads": true,
	}

	if genericNames[strings.ToLower(name)] {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.ReplaceAll(name, "-", " ")
	}

	name = strings.Title(strings.ToLower(name))
	if name == "" || name == "." || name == "/" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return name
}
