// Package proton provides Proton version detection and prefix scanning.
package proton

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"deck-game-installer/vdf"
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

// GetAvailableProtonVersions returns all installed Proton versions,
// sorted with Experimental first, then descending by version number.
func (m *Manager) GetAvailableProtonVersions() []string {
	var versions []string

	// Official Proton installs in steamapps/common
	if entries, err := os.ReadDir(m.commonPath); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "proton") {
				versions = append(versions, folderToInternalName(e.Name()))
			}
		}
	}

	// Custom builds (Proton-GE etc.) in compatibilitytools.d
	if entries, err := os.ReadDir(m.compatToolsPath); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			vdfPath := filepath.Join(m.compatToolsPath, e.Name(), "compatibilitytool.vdf")
			if data, err := os.ReadFile(vdfPath); err == nil {
				if name := internalNameFromVDF(string(data)); name != "" {
					versions = append(versions, name)
					continue
				}
			}
			versions = append(versions, folderToInternalName(e.Name()))
		}
	}

	versions = unique(versions)
	sort.Slice(versions, func(i, j int) bool {
		vi, vj := versions[i], versions[j]
		if strings.Contains(strings.ToLower(vi), "experimental") {
			return true
		}
		if strings.Contains(strings.ToLower(vj), "experimental") {
			return false
		}
		return vi > vj
	})
	return versions
}

// PrefixPath returns the drive_c path inside the Proton prefix for a given app ID.
func (m *Manager) PrefixPath(appID int32) string {
	return filepath.Join(m.compatDataPath, strconv.FormatUint(uint64(uint32(appID)), 10), "pfx", "drive_c")
}

// ScanPrefixForExecutables scans the Proton prefix for game executables,
// filtering out common system/installer binaries. Results are sorted by
// modification time, newest first.
func (m *Manager) ScanPrefixForExecutables(appID int32) []string {
	prefix := m.PrefixPath(appID)
	if _, err := os.Stat(prefix); err != nil {
		return nil
	}

	excludeDirs := map[string]struct{}{
		"windows":           {},
		"common files":      {},
		"internet explorer": {},
		"steam":             {},
		"users":             {},
	}

	excludePatterns := []*regexp.Regexp{
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
		regexp.MustCompile(`.*crashhandler.*\.exe`),
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
	}

	var results []string

	_ = filepath.WalkDir(prefix, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(prefix, path)
			for _, part := range strings.Split(strings.ToLower(rel), string(filepath.Separator)) {
				if _, skip := excludeDirs[part]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".exe") {
			return nil
		}
		for _, re := range excludePatterns {
			if re.MatchString(name) {
				return nil
			}
		}
		results = append(results, path)
		return nil
	})

	sort.Slice(results, func(i, j int) bool {
		ii, _ := os.Stat(results[i])
		ji, _ := os.Stat(results[j])
		if ii == nil || ji == nil {
			return results[i] > results[j]
		}
		return ii.ModTime().After(ji.ModTime())
	})

	return results
}

// folderToInternalName converts a Proton folder name to Steam's internal name.
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

// internalNameFromVDF extracts the internal Proton name from a compatibility tool VDF.
func internalNameFromVDF(data string) string {
	root, err := vdf.Parse(data)
	if err != nil {
		return ""
	}
	compat := vdf.GetNestedMap(root, "compatibilitytools", "compat_tools")
	if compat == nil {
		return ""
	}
	for k := range compat {
		return k
	}
	return ""
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
