package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InputMode describes how the selected input should be handled.
type InputMode string

const (
	InputModeInstaller InputMode = "installer"
	InputModePortable  InputMode = "portable"
	InputModeAmbiguous InputMode = "ambiguous"
)

// DetectInputMode inspects the selected path and decides whether it looks like
// an installer or a portable game.
func DetectInputMode(path string) (InputMode, []string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}

	lower := strings.ToLower(path)
	switch {
	case info.IsDir():
		return "", nil, fmt.Errorf("directories are not supported; select the portable game's .exe")
	case strings.HasSuffix(lower, ".iso"):
		return InputModeInstaller, []string{"ISO images use the installer workflow"}, nil
	case !strings.HasSuffix(lower, ".exe"):
		return "", nil, fmt.Errorf("unsupported file type: %s", path)
	}

	dir := filepath.Dir(path)
	name := strings.ToLower(filepath.Base(path))
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	installerScore := 0
	portableScore := 0
	var reasons []string

	if looksLikeInstallerName(stem) {
		installerScore += 4
		reasons = append(reasons, "executable name looks like an installer")
	}
	if hasInstallerSiblings(dir) {
		installerScore += 2
		reasons = append(reasons, "nearby files look like installer media")
	}
	if hasPortableMarkersNear(dir) {
		portableScore += 4
		reasons = append(reasons, "nearby files look like a portable game")
	}
	if looksLikeGameExecutable(path) {
		portableScore += 2
		reasons = append(reasons, "executable name looks like a game/launcher")
	}

	switch {
	case installerScore >= 4 && installerScore > portableScore:
		return InputModeInstaller, reasons, nil
	case portableScore >= 4 && portableScore > installerScore:
		return InputModePortable, reasons, nil
	default:
		if len(reasons) == 0 {
			reasons = append(reasons, "not enough evidence to classify this executable")
		}
		return InputModeAmbiguous, reasons, nil
	}
}

func looksLikeInstallerName(stem string) bool {
	stem = normalizeName(stem)
	installerWords := []string{
		"setup", "install", "installer", "autorun", "launcher setup",
		"redist", "vcredist", "dxsetup", "directx", "unins", "uninstall",
	}
	for _, word := range installerWords {
		if stem == word || strings.Contains(stem, word) {
			return true
		}
	}
	return false
}

func looksLikeGameExecutable(path string) bool {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if looksLikeInstallerName(name) {
		return false
	}
	if strings.Contains(name, "crash") || strings.Contains(name, "unitycrashhandler") {
		return false
	}
	return true
}

func hasInstallerSiblings(dir string) bool {
	names, err := readLowerDirNames(dir)
	if err != nil {
		return false
	}
	for _, name := range names {
		switch name {
		case "autorun.inf", "setup.ini", "setup.dat", "setup.bin", "install.ini":
			return true
		}
		if strings.HasPrefix(name, "data") && strings.HasSuffix(name, ".cab") {
			return true
		}
	}
	return false
}

func hasPortableMarkers(dir string) bool {
	names, err := readLowerDirNames(dir)
	if err != nil {
		return false
	}
	for _, name := range names {
		if name == "steam_appid.txt" ||
			name == "unityplayer.dll" ||
			name == "gameassembly.dll" ||
			strings.HasPrefix(name, "steam_api") && strings.HasSuffix(name, ".dll") ||
			strings.HasSuffix(name, "_data") {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Binaries", "Win64")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "Content", "Paks")); err == nil {
		return true
	}
	return false
}

func hasPortableMarkersNear(dir string) bool {
	for depth := 0; depth < 5 && dir != "" && dir != string(filepath.Separator); depth++ {
		if hasPortableMarkers(dir) {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

func readLowerDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.ToLower(entry.Name()))
	}
	return names, nil
}

// ResolveInputMode records the detected mode in State, asking the user only
// when a selected .exe is genuinely ambiguous.
type ResolveInputMode struct{}

func NewResolveInputMode() *ResolveInputMode { return &ResolveInputMode{} }
func (s *ResolveInputMode) Name() string     { return "Detect Input" }

func (s *ResolveInputMode) Execute(ctx context.Context, state *State) error {
	mode, reasons, err := DetectInputMode(state.InputPath)
	if err != nil {
		return err
	}

	state.UI.Log("Input analysis: " + strings.Join(reasons, "; "))
	if mode == InputModeAmbiguous {
		selected, ok := state.UI.Select(
			"Choose Input Type",
			"This executable could be either an installer or a portable game. How should it be handled?",
			[]string{"Installer", "Portable Game"},
		)
		if !ok {
			return fmt.Errorf("input type selection cancelled")
		}
		if selected == "Portable Game" {
			mode = InputModePortable
		} else {
			mode = InputModeInstaller
		}
	}

	state.InputMode = mode
	state.UI.Log("Input mode: " + string(mode))
	return nil
}

// FindPortableGame records the selected .exe as the final game executable.
type FindPortableGame struct{}

func NewFindPortableGame() *FindPortableGame { return &FindPortableGame{} }
func (s *FindPortableGame) Name() string     { return "Find Game" }

func (s *FindPortableGame) Execute(ctx context.Context, state *State) error {
	info, err := os.Stat(state.InputPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("portable mode requires selecting the game's .exe, not a folder")
	}

	state.GameExePath = state.InputPath
	state.InstallerPath = state.InputPath
	if state.GameName == "" {
		state.GameName = DeriveGameName(state.InputPath)
	}
	state.UI.Log("Selected: " + filepath.Base(state.GameExePath))
	return nil
}

func normalizeName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	lastSpace := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}
