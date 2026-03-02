package steam

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"deck-game-installer/vdf"
)

// Manager handles Steam integration: shortcuts, configuration, and process management.
type Manager struct {
	steamPath    string
	userdataPath string
	userIDs      []string
	userID       string
}

// NewManager creates a new Steam manager, auto-detecting Steam paths.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	steamPath := filepath.Join(home, ".local", "share", "Steam")
	userdata := filepath.Join(steamPath, "userdata")

	userIDs := findUserIDs(userdata)
	userID := "0"
	if len(userIDs) > 0 {
		userID = userIDs[0]
	}

	return &Manager{
		steamPath:    steamPath,
		userdataPath: userdata,
		userIDs:      userIDs,
		userID:       userID,
	}
}

// findUserIDs returns all numeric user ID directories in the userdata folder.
func findUserIDs(userdata string) []string {
	entries, err := os.ReadDir(userdata)
	if err != nil {
		return nil
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() && isDigits(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	return ids
}

// isDigits returns true if the string contains only digits.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// SteamPath returns the root Steam installation path.
func (m *Manager) SteamPath() string {
	return m.steamPath
}

// ShortcutsPath returns the path to shortcuts.vdf for the current user.
func (m *Manager) ShortcutsPath() string {
	return filepath.Join(m.userdataPath, m.userID, "config", "shortcuts.vdf")
}

// ConfigPath returns the path to config.vdf.
func (m *Manager) ConfigPath() string {
	return filepath.Join(m.steamPath, "config", "config.vdf")
}

// GenerateAppID creates a deterministic app ID from the exe path and app name.
// This matches Steam's algorithm for non-Steam game shortcuts.
func GenerateAppID(exePath, appName string) int32 {
	key := exePath + appName
	u32 := crc32.ChecksumIEEE([]byte(key)) | 0x80000000
	return int32(u32)
}

// GetURLAppID converts a signed app ID to the unsigned 64-bit format used in steam:// URLs.
func GetURLAppID(appID int32) string {
	unsigned := uint32(appID)
	u64 := (uint64(unsigned) << 32) | 0x02000000
	return strconv.FormatUint(u64, 10)
}

// FindAppIDByPath searches all user shortcuts for a matching exe path.
func (m *Manager) FindAppIDByPath(exePath string) (int32, error) {
	quoted := "\"" + exePath + "\""

	for _, uid := range m.userIDs {
		path := filepath.Join(m.userdataPath, uid, "config", "shortcuts.vdf")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		obj, err := ReadBinaryVDF(data)
		if err != nil {
			continue
		}

		shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
		if !ok {
			continue
		}

		for _, v := range shortcuts {
			entry, ok := v.(map[string]KVValue)
			if !ok {
				continue
			}

			exe, _ := entry["Exe"].(string)
			if exe == quoted || exe == exePath {
				if appid, ok := entry["appid"].(int32); ok {
					return appid, nil
				}
			}
		}
	}

	return 0, errors.New("shortcut not found")
}

// AddShortcut creates a new Steam shortcut or returns the existing app ID if already present.
func (m *Manager) AddShortcut(appName, exePath, args, startDir string) (int32, error) {
	path := m.ShortcutsPath()
    fmt.Printf("[AddShortcut] Path: %s\n", path)

	// Load existing shortcuts or create new structure
	obj := map[string]KVValue{"shortcuts": map[string]KVValue{}}
	if data, err := os.ReadFile(path); err == nil {
		if parsed, err := ReadBinaryVDF(data); err == nil {
			obj = parsed
            fmt.Printf("[AddShortcut] Loaded existing shortcuts from %s\n", path)
		}
	}

	shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
	if !ok {
		shortcuts = map[string]KVValue{}
		obj["shortcuts"] = shortcuts
	}

	// Check if shortcut already exists
	quoted := "\"" + exePath + "\""
	var existingKey string

	for k, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}

		exe, _ := entry["Exe"].(string)
		if exe == quoted || exe == exePath {
			if appid, ok := entry["appid"].(int32); ok {
                    fmt.Printf("[AddShortcut] Shortcut already exists: AppID=%d, Exe=%s\n", appid, exe)
				return appid, nil // Already exists
			}
		}

		// Check if name matches (for updating existing entry)
		name, _ := entry["AppName"].(string)
		if name == appName {
			existingKey = k
		}
	}

	// Generate app ID and determine key
	appid := GenerateAppID(exePath, appName)
	var key string

	if existingKey != "" {
		key = existingKey
	} else {
		idx := nextShortcutIndex(shortcuts)
		key = strconv.Itoa(idx)
	}

	if startDir == "" {
		startDir = filepath.Dir(exePath)
	}

	// Create shortcut entry
    fmt.Printf("[AddShortcut] Creating/updating shortcut: AppID=%d, AppName=%s, Exe=%s, Args=%s, StartDir=%s\n", appid, appName, exePath, args, startDir)
	shortcuts[key] = map[string]KVValue{
		"appid":               appid,
		"AppName":             appName,
		"Exe":                 quoted,
		"StartDir":            startDir,
		"icon":                "",
		"ShortcutPath":        "",
		"LaunchOptions":       args,
		"IsHidden":            int32(0),
		"AllowDesktopConfig":  int32(1),
		"AllowOverlay":        int32(1),
		"OpenVR":              int32(0),
		"Devkit":              int32(0),
		"DevkitGameID":        "",
		"DevkitOverrideAppID": int32(0),
		"LastPlayTime":        int32(0),
		"FlatpakAppID":        "",
		"sortas":              "",
		"tags":                map[string]KVValue{},
	}

	// Write back to file
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := WriteBinaryVDF(obj)
	if err != nil {
        fmt.Printf("[AddShortcut] Error serializing shortcuts: %v\n", err)
		return 0, err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
        fmt.Printf("[AddShortcut] Error writing shortcuts file: %v\n", err)
		return 0, err
	}
    fmt.Printf("[AddShortcut] Successfully wrote shortcuts to %s\n", path)

	return appid, nil
}

// DeleteShortcut removes a shortcut by app ID.
func (m *Manager) DeleteShortcut(appID int32) error {
	path := m.ShortcutsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	obj, err := ReadBinaryVDF(data)
	if err != nil {
		return err
	}

	shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
	if !ok {
		return errors.New("shortcuts not found")
	}

	// Find and delete the shortcut
	var foundKey string
	for k, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}

		if existingID, ok := entry["appid"].(int32); ok && existingID == appID {
			foundKey = k
			break
		}
	}

	if foundKey == "" {
		return errors.New("shortcut not found")
	}

	delete(shortcuts, foundKey)

	// Write back to file
	newData, err := WriteBinaryVDF(obj)
	if err != nil {
		return err
	}

	return os.WriteFile(path, newData, 0o644)
}

// GetShortcut retrieves shortcut info by app ID.
func (m *Manager) GetShortcut(appID int32) (exePath, startDir, appName string, err error) {
	path := m.ShortcutsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", err
	}

	obj, err := ReadBinaryVDF(data)
	if err != nil {
		return "", "", "", err
	}

	shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
	if !ok {
		return "", "", "", errors.New("shortcuts not found")
	}

	for _, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}

		if existingID, ok := entry["appid"].(int32); ok && existingID == appID {
			exePath, _ = entry["Exe"].(string)
			startDir, _ = entry["StartDir"].(string)
			appName, _ = entry["AppName"].(string)

			// Remove quotes from exe path if present
			exePath = strings.Trim(exePath, "\"")
			return exePath, startDir, appName, nil
		}
	}

	return "", "", "", errors.New("shortcut not found")
}

// UpdateShortcut modifies an existing shortcut's exe and start directory.
func (m *Manager) UpdateShortcut(appID int32, newExePath, newStartDir, newAppName string) error {
	path := m.ShortcutsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	obj, err := ReadBinaryVDF(data)
	if err != nil {
		return err
	}

	shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
	if !ok {
		return errors.New("shortcuts not found")
	}

	// Find and update the shortcut
	for _, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}

		if existingID, ok := entry["appid"].(int32); ok && existingID == appID {
			entry["Exe"] = "\"" + newExePath + "\""

			if newStartDir == "" {
				newStartDir = filepath.Dir(newExePath)
			}
			entry["StartDir"] = newStartDir

			if newAppName != "" {
				entry["AppName"] = newAppName
			} else {
				// Remove "(Installer)" suffix if present
				if name, ok := entry["AppName"].(string); ok {
					if strings.HasSuffix(name, " (Installer)") {
						entry["AppName"] = strings.TrimSuffix(name, " (Installer)")
					}
				}
			}

			// Write back to file
			newData, err := WriteBinaryVDF(obj)
			if err != nil {
				return err
			}
			return os.WriteFile(path, newData, 0o644)
		}
	}

	return errors.New("shortcut not found")
}

// nextShortcutIndex returns the next available index for a new shortcut.
func nextShortcutIndex(shortcuts map[string]KVValue) int {
	max := -1
	for k := range shortcuts {
		if n, err := strconv.Atoi(k); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// GetProtonVersion returns the configured Proton version for an app ID.
func (m *Manager) GetProtonVersion(appID int32) (string, error) {
	path := m.ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	root, err := vdf.Parse(string(data))
	if err != nil {
		return "", err
	}

	mapping := vdf.GetNestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")
	if mapping == nil {
		return "", errors.New("CompatToolMapping not found")
	}

	unsigned := uint32(appID)
	key := strconv.FormatUint(uint64(unsigned), 10)
	entry, ok := mapping[key].(map[string]any)
	if !ok {
		return "", errors.New("app entry not found")
	}

	name, _ := entry["name"].(string)
	if name == "" {
		return "", errors.New("Proton version not found")
	}

	return name, nil
}

// SetProtonVersion configures the Proton version for an app ID.
func (m *Manager) SetProtonVersion(appID int32, protonVersion string) error {
	   path := m.ConfigPath()
	   fmt.Printf("[SetProtonVersion] Path: %s\n", path)
	   data, err := os.ReadFile(path)
	   if err != nil {
		   fmt.Printf("[SetProtonVersion] Error reading config: %v\n", err)
		   return err
	   }

	   root, err := vdf.Parse(string(data))
	   if err != nil {
		   fmt.Printf("[SetProtonVersion] Error parsing VDF: %v\n", err)
		   return err
	   }

	   // Ensure the nested path exists
	   mapping := ensureNestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")

	   unsigned := uint32(appID)
	   key := strconv.FormatUint(uint64(unsigned), 10)

	   fmt.Printf("[SetProtonVersion] Setting Proton for AppID=%d to '%s'\n", appID, protonVersion)
	   mapping[key] = map[string]any{
		   "name":     protonVersion,
		   "config":   "",
		   "priority": "250",
	   }

	   out := vdf.Dump(root)
	   if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		   fmt.Printf("[SetProtonVersion] Error writing config: %v\n", err)
		   return err
	   }
	   fmt.Printf("[SetProtonVersion] Successfully set Proton version for AppID=%d to '%s' in %s\n", appID, protonVersion, path)
	   return nil
}

// ensureNestedMap traverses/creates nested maps along a key path.
func ensureNestedMap(root map[string]any, keys ...string) map[string]any {
	current := root
	for _, k := range keys {
		v, ok := current[k]
		if !ok {
			child := map[string]any{}
			current[k] = child
			current = child
			continue
		}

		next, ok := v.(map[string]any)
		if !ok {
			child := map[string]any{}
			current[k] = child
			current = child
			continue
		}

		current = next
	}
	return current
}

// ShutdownSteam gracefully shuts down Steam and waits for it to exit.
func (m *Manager) ShutdownSteam(ctx context.Context) error {
	_ = exec.CommandContext(ctx, "steam", "-shutdown").Run()

	if err := m.waitForSteamShutdown(ctx); err != nil {
		// Fall back to force kill
		_ = exec.Command("pkill", "-x", "steam").Run()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil
}

// StartSteam launches Steam in the background, detached from this process.
// If a steam:// URL is provided it is passed as an argument so Steam handles
// it as soon as it finishes initialising.
func (m *Manager) StartSteam(args ...string) {
	cmd := exec.Command("steam", args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}

// waitForSteamShutdown monitors Steam's log to detect when it has shut down.
func (m *Manager) waitForSteamShutdown(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logPath := filepath.Join(home, ".local", "share", "Steam", "logs", "console-linux.txt")

	// Get initial file size
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return err
	}
	offset := fileInfo.Size()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(15 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			return errors.New("timeout waiting for Steam shutdown")

		case <-ticker.C:
			// Check if steam process is still running
			if err := exec.Command("pgrep", "-x", "steam").Run(); err != nil {
				// Process not found, Steam has shut down
				time.Sleep(500 * time.Millisecond)
				return nil
			}

			// Read new log content
			file, err := os.Open(logPath)
			if err != nil {
				continue
			}

			file.Seek(offset, 0)

			buf := make([]byte, 8192)
			n, _ := file.Read(buf)
			if n > 0 {
				offset += int64(n)
				content := string(buf[:n])

				if strings.Contains(content, "Exiting") ||
					strings.Contains(content, "Shutdown") ||
					strings.Contains(content, "Steam_Shutdown") {
					file.Close()
					time.Sleep(500 * time.Millisecond)
					return nil
				}
			}
			file.Close()
		}
	}
}


