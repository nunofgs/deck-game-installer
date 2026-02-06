package proton

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"deck-game-installer/internal/vdf"
)

type Manager struct {
	steamPath       string
	commonPath      string
	compatToolsPath string
	compatDataPath  string
}

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

func (m *Manager) GetAvailableProtonVersions() []string {
	versions := []string{}
	if entries, err := os.ReadDir(m.commonPath); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "proton") {
				versions = append(versions, folderToInternalName(e.Name()))
			}
		}
	}

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
		vi := versions[i]
		vj := versions[j]
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

func internalNameFromVDF(data string) string {
	root, err := vdf.Parse(data)
	if err != nil {
		return ""
	}

	compat := nestedMap(root, "compatibilitytools", "compat_tools")
	if compat == nil {
		return ""
	}
	for k := range compat {
		return k
	}
	return ""
}

func nestedMap(root map[string]any, keys ...string) map[string]any {
	current := root
	for _, k := range keys {
		v, ok := current[k]
		if !ok {
			return nil
		}
		next, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

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

func (m *Manager) ScanPrefixForExecutables(appID int32) []string {
	unsigned := uint32(appID)
	prefix := filepath.Join(m.compatDataPath, strconv.FormatUint(uint64(unsigned), 10), "pfx", "drive_c")
	if _, err := os.Stat(prefix); err != nil {
		return nil
	}

	excludeDirs := map[string]struct{}{
		"windows": {}, "common files": {}, "internet explorer": {}, "steam": {}, "users": {},
	}
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
	}

	var executables []string
	_ = filepath.WalkDir(prefix, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
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
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".exe") {
			return nil
		}
		for _, re := range patterns {
			if re.MatchString(name) {
				return nil
			}
		}
		executables = append(executables, path)
		return nil
	})

	sort.Slice(executables, func(i, j int) bool {
		iInfo, _ := os.Stat(executables[i])
		jInfo, _ := os.Stat(executables[j])
		if iInfo == nil || jInfo == nil {
			return executables[i] > executables[j]
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	return executables
}
