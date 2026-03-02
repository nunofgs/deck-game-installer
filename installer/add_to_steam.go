package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// AddToSteam creates a Steam shortcut for the installer.
type AddToSteam struct {
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

// titleCase uppercases the first letter of each word, replacing strings.Title
// which was deprecated in Go 1.18.
func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
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

	name = titleCase(name)
	if name == "" || name == "." || name == "/" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return name
}
