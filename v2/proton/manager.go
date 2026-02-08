// Package proton provides Proton version detection and prefix scanning.
package proton

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"deck-game-installer/v2/vdf"
)

// Manager handles Proton-related operations.
type Manager struct {
	steamPath       string
	commonPath      string
	compatToolsPath string
	compatDataPath  string
}

// NewManager creates a new Proton manager, auto-detecting Steam paths.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	steamPath := filepath.Join(home, ".local", "share", "Steam")
	return &Manager{
		steamPath:       steamPath,
		commonPath:      filepath.Join(steamPath, "steamapps", "common"),
		compatToolsPath: filepath.Join(steamPath, "compatibilitytools.d"),
		compatDataPath:  filepath.Join(steamPath, "steamapps", "compatdata"),
	}
}

// GetAvailableProtonVersions returns all installed Proton versions.
// The list is sorted with Experimental first, then by version descending.
func (m *Manager) GetAvailableProtonVersions() []string {
	versions := []string{}

	// Check Steam's common folder for official Proton installs
	if entries, err := os.ReadDir(m.commonPath); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "proton") {
				versions = append(versions, folderToInternalName(e.Name()))
			}
		}
	}

	// Check compatibilitytools.d for custom Proton builds (GE, TKG, etc.)
	if entries, err := os.ReadDir(m.compatToolsPath); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			// Try to read the internal name from the VDF file
			vdfPath := filepath.Join(m.compatToolsPath, e.Name(), "compatibilitytool.vdf")
			if data, err := os.ReadFile(vdfPath); err == nil {
				if name := internalNameFromVDF(string(data)); name != "" {
					versions = append(versions, name)
					continue
				}
			}

			// Fall back to folder name
			versions = append(versions, folderToInternalName(e.Name()))
		}
	}

	// Remove duplicates and sort
	versions = unique(versions)
	sort.Slice(versions, func(i, j int) bool {
		vi := versions[i]
		vj := versions[j]

		// Experimental always first
		if strings.Contains(strings.ToLower(vi), "experimental") {
			return true
		}
		if strings.Contains(strings.ToLower(vj), "experimental") {
			return false
		}

		// Otherwise sort descending (newer versions first)
		return vi > vj
	})

	return versions
}

// GetPreferredProton returns the best Proton version to use.
// Prefers Experimental, then the newest numbered version.
func (m *Manager) GetPreferredProton() string {
	versions := m.GetAvailableProtonVersions()
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}

// folderToInternalName converts a folder name to Steam's internal Proton name.
func folderToInternalName(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "experimental") {
		return "proton-experimental"
	}
	if m := regexp.MustCompile(`proton\s*(\d+)`).FindStringSubmatch(lower); len(m) > 1 {
		return "proton_" + m[1]
	}
	return strings.ReplaceAll(strings.ReplaceAll(lower, " - ", "-"), " ", "_")
}

// internalNameFromVDF extracts the internal Proton name from a compatibility tool VDF file.
func internalNameFromVDF(data string) string {
	root, err := vdf.Parse(data)
	if err != nil {
		return ""
	}

	compat := vdf.GetNestedMap(root, "compatibilitytools", "compat_tools")
	if compat == nil {
		return ""
	}

	// Return the first key (there's usually only one)
	for k := range compat {
		return k
	}
	return ""
}

// unique removes duplicate strings from a slice.
func unique(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// PrefixPath returns the path to the Proton prefix for a given app ID.
func (m *Manager) PrefixPath(appID int32) string {
	unsigned := uint32(appID)
	return filepath.Join(m.compatDataPath, strconv.FormatUint(uint64(unsigned), 10), "pfx", "drive_c")
}

// ScanPrefixForExecutables scans a Proton prefix for game executables.
// Filters out common non-game executables (uninstallers, redistributables, etc.)
// Returns executables sorted by modification time (newest first).
func (m *Manager) ScanPrefixForExecutables(appID int32) []string {
	prefix := m.PrefixPath(appID)
	logPath := "/tmp/deck-game-installer-v2.log"
	logFile, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer logFile.Close()

	fmt.Fprintf(logFile, "\n[ScanPrefixForExecutables] Scanning prefix: %s\n", prefix)

	if _, err := os.Stat(prefix); err != nil {
		fmt.Fprintf(logFile, "Prefix does not exist or is not accessible.\n")
		return nil
	}

	// Directories to exclude
	excludeDirs := map[string]struct{}{
		"windows":           {},
		"common files":      {},
		"internet explorer": {},
		"steam":             {},
		"users":             {},
	}

	// Patterns for non-game executables to exclude
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`unins.*\.exe`),
		regexp.MustCompile(`uninst.*\.exe`),
		regexp.MustCompile(`vcredist.*\.exe`),
		regexp.MustCompile(`directx.*\.exe`),
		regexp.MustCompile(`dxsetup\.exe`),
		regexp.MustCompile(`setup\.exe`),
		regexp.MustCompile(`install\.exe`),
		regexp.MustCompile(`installer\.exe`),
		regexp.MustCompile(`.*redist.*\.exe`),
		regexp.MustCompile(`.*crash.*reporter.*\.exe`),
		regexp.MustCompile(`wordpad\.exe`),
		regexp.MustCompile(`notepad\.exe`),
		regexp.MustCompile(`wmplayer\.exe`),
		regexp.MustCompile(`mspaint\.exe`),
		regexp.MustCompile(`explorer\.exe`),
		regexp.MustCompile(`iexplore\.exe`),
		regexp.MustCompile(`cmd\.exe`),
		regexp.MustCompile(`powershell\.exe`),
		regexp.MustCompile(`regedit\.exe`),
		regexp.MustCompile(`taskmgr\.exe`),
		regexp.MustCompile(`control\.exe`),
		regexp.MustCompile(`.*crashhandler.*\.exe`),
	}

	var allExecutables []string
	var excludedExecutables []string
	var includedExecutables []string

	_ = filepath.WalkDir(prefix, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Check if we should skip this directory
		if d.IsDir() {
			rel, _ := filepath.Rel(prefix, path)
			parts := strings.Split(strings.ToLower(rel), string(filepath.Separator))
			for _, p := range parts {
				if _, ok := excludeDirs[p]; ok {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if it's an executable
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".exe") {
			return nil
		}

		allExecutables = append(allExecutables, path)

		// Filter out known non-game executables
		for _, re := range patterns {
			if re.MatchString(name) {
				excludedExecutables = append(excludedExecutables, path)
				return nil
			}
		}

		includedExecutables = append(includedExecutables, path)
		return nil
	})

	fmt.Fprintf(logFile, "All .exe files found:\n")
	for _, exe := range allExecutables {
		fmt.Fprintf(logFile, "  %s\n", exe)
	}
	fmt.Fprintf(logFile, "\nIncluded executables (offered to user):\n")
	for _, exe := range includedExecutables {
		fmt.Fprintf(logFile, "  %s\n", exe)
	}
	fmt.Fprintf(logFile, "\nExcluded executables (filtered out):\n")
	for _, exe := range excludedExecutables {
		fmt.Fprintf(logFile, "  %s\n", exe)
	}
	fmt.Fprintf(logFile, "Done scanning for executables.\n")

	// Sort by modification time (newest first)
	sort.Slice(includedExecutables, func(i, j int) bool {
		iInfo, _ := os.Stat(includedExecutables[i])
		jInfo, _ := os.Stat(includedExecutables[j])
		if iInfo == nil || jInfo == nil {
			return includedExecutables[i] > includedExecutables[j]
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	return includedExecutables
}
