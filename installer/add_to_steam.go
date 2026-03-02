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

// NewAddToSteam creates a new add to Steam step.
func NewAddToSteam() *AddToSteam {
	return &AddToSteam{}
}

func (s *AddToSteam) Name() string {
	return "Add to Steam"
}

func (s *AddToSteam) Description(state *State) string {
	if state.GameName != "" {
		return fmt.Sprintf("Steam shortcut created: %s (Installer)", state.GameName)
	}
	return "Steam shortcut created"
}

func (s *AddToSteam) Execute(ctx context.Context, state *State) error {
	// Derive game name from installer path if not set
	if state.GameName == "" {
		state.GameName = deriveGameName(state.InstallerPath)
	}

	appName := state.GameName + " (Installer)"
	startDir := filepath.Dir(state.InstallerPath)

	state.UI.Log(fmt.Sprintf("Creating Steam shortcut: %s", appName))

	appID, err := state.Steam.AddShortcut(appName, state.InstallerPath, "", startDir)
	if err != nil {
		return fmt.Errorf("failed to create Steam shortcut: %w", err)
	}

	state.AppID = appID
	state.OriginalShortcutExe = state.InstallerPath
	state.OriginalShortcutDir = startDir

	state.UI.Log(fmt.Sprintf("Shortcut created with App ID: %d", appID))
	return nil
}

func (s *AddToSteam) Rollback(ctx context.Context, state *State) error {
	if state.AppID != 0 {
		state.UI.Log("Removing Steam shortcut...")
		return state.Steam.DeleteShortcut(state.AppID)
	}
	return nil
}

func (s *AddToSteam) CanRollback() bool {
	return true
}

// deriveGameName extracts a game name from the installer path.
func deriveGameName(path string) string {
	// Try to get a meaningful name from the path
	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	// Clean up common patterns
	name = strings.TrimSuffix(name, "_files")
	name = strings.TrimSuffix(name, "-files")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// If the directory name is generic (like "disc1"), try parent
	genericNames := map[string]bool{
		"disc1": true, "disc2": true, "disk1": true, "disk2": true,
		"cd1": true, "cd2": true, "dvd1": true, "dvd2": true,
		"setup": true, "install": true, "installer": true,
	}

	if genericNames[strings.ToLower(name)] {
		parentDir := filepath.Dir(dir)
		name = filepath.Base(parentDir)
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.ReplaceAll(name, "-", " ")
	}

	// Title case the name
	name = strings.Title(strings.ToLower(name))

	if name == "" || name == "." || name == "/" {
		// Fallback to installer filename
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = strings.Title(strings.ToLower(name))
	}

	return name
}
