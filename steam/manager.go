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
	userID       string
}

// NewManager creates a new Steam manager, auto-detecting Steam paths.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	steamPath := filepath.Join(home, ".local", "share", "Steam")
	userdata := filepath.Join(steamPath, "userdata")
	userID := firstUserID(userdata)
	return &Manager{
		steamPath:    steamPath,
		userdataPath: userdata,
		userID:       userID,
	}
}

// firstUserID returns the first numeric user ID directory found in userdata.
func firstUserID(userdata string) string {
	entries, err := os.ReadDir(userdata)
	if err != nil {
		return "0"
	}
	for _, e := range entries {
		if e.IsDir() && isDigits(e.Name()) {
			return e.Name()
		}
	}
	return "0"
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
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
	u32 := crc32.ChecksumIEEE([]byte(exePath+appName)) | 0x80000000
	return int32(u32)
}

// GetURLAppID converts a signed app ID to the unsigned 64-bit format used in steam:// URLs.
func GetURLAppID(appID int32) string {
	u64 := (uint64(uint32(appID)) << 32) | 0x02000000
	return strconv.FormatUint(u64, 10)
}

// AddShortcut creates a new Steam shortcut, or returns the existing app ID if one already exists for that exe.
func (m *Manager) AddShortcut(appName, exePath, args, startDir string) (int32, error) {
	path := m.ShortcutsPath()

	obj := map[string]KVValue{"shortcuts": map[string]KVValue{}}
	if data, err := os.ReadFile(path); err == nil {
		if parsed, err := ReadBinaryVDF(data); err == nil {
			obj = parsed
		}
	}

	shortcuts, ok := obj["shortcuts"].(map[string]KVValue)
	if !ok {
		shortcuts = map[string]KVValue{}
		obj["shortcuts"] = shortcuts
	}

	// Return early if a shortcut for this exe already exists.
	quoted := "\"" + exePath + "\""
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

	if startDir == "" {
		startDir = filepath.Dir(exePath)
	}

	appid := GenerateAppID(exePath, appName)
	key := strconv.Itoa(nextShortcutIndex(shortcuts))

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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, fmt.Errorf("failed to create shortcuts directory: %w", err)
	}
	data, err := WriteBinaryVDF(obj)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}
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
	for k, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}
		if id, ok := entry["appid"].(int32); ok && id == appID {
			delete(shortcuts, k)
			newData, err := WriteBinaryVDF(obj)
			if err != nil {
				return err
			}
			return os.WriteFile(path, newData, 0o644)
		}
	}
	return errors.New("shortcut not found")
}

// UpdateShortcut modifies an existing shortcut's exe, start directory, and name.
func (m *Manager) UpdateShortcut(appID int32, newExePath, newAppName string) error {
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

	for _, v := range shortcuts {
		entry, ok := v.(map[string]KVValue)
		if !ok {
			continue
		}
		if id, ok := entry["appid"].(int32); !ok || id != appID {
			continue
		}

		entry["Exe"] = "\"" + newExePath + "\""
		entry["StartDir"] = filepath.Dir(newExePath)
		if newAppName != "" {
			entry["AppName"] = newAppName
		} else if name, ok := entry["AppName"].(string); ok {
			entry["AppName"] = strings.TrimSuffix(name, " (Installer)")
		}

		newData, err := WriteBinaryVDF(obj)
		if err != nil {
			return err
		}
		return os.WriteFile(path, newData, 0o644)
	}

	return errors.New("shortcut not found")
}

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
	configPath := m.ConfigPath()
	fmt.Printf("[GetProtonVersion] Reading config.vdf from: %s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("[GetProtonVersion] ERROR reading config.vdf: %v\n", err)
		return "", err
	}
	fmt.Printf("[GetProtonVersion] config.vdf read OK (%d bytes)\n", len(data))

	root, err := vdf.Parse(string(data))
	if err != nil {
		fmt.Printf("[GetProtonVersion] ERROR parsing config.vdf: %v\n", err)
		return "", err
	}
	fmt.Printf("[GetProtonVersion] config.vdf parsed OK, top-level keys: %v\n", mapKeys(root))

	mapping := vdf.GetNestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")
	if mapping == nil {
		fmt.Printf("[GetProtonVersion] CompatToolMapping not found in config.vdf\n")
		return "", errors.New("CompatToolMapping not found")
	}
	fmt.Printf("[GetProtonVersion] CompatToolMapping found, keys: %v\n", mapKeys(mapping))

	key := strconv.FormatUint(uint64(uint32(appID)), 10)
	fmt.Printf("[GetProtonVersion] Looking up app key: %q (appID int32=%d, uint32=%d)\n", key, appID, uint32(appID))

	entry, ok := mapping[key].(map[string]any)
	if !ok {
		fmt.Printf("[GetProtonVersion] No entry found for key %q in CompatToolMapping\n", key)
		return "", errors.New("app entry not found")
	}
	fmt.Printf("[GetProtonVersion] Entry found for key %q: %v\n", key, entry)

	name, _ := entry["name"].(string)
	fmt.Printf("[GetProtonVersion] Entry 'name' field: %q\n", name)
	if name == "" {
		return "", errors.New("Proton version not set")
	}
	return name, nil
}

// SetProtonVersion configures the Proton version for an app ID in config.vdf.
func (m *Manager) SetProtonVersion(appID int32, protonVersion string) error {
	configPath := m.ConfigPath()
	fmt.Printf("[SetProtonVersion] appID int32=%d uint32=%d protonVersion=%q\n", appID, uint32(appID), protonVersion)
	fmt.Printf("[SetProtonVersion] Reading config.vdf from: %s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("[SetProtonVersion] ERROR reading config.vdf: %v\n", err)
		return err
	}
	fmt.Printf("[SetProtonVersion] config.vdf read OK (%d bytes)\n", len(data))

	root, err := vdf.Parse(string(data))
	if err != nil {
		fmt.Printf("[SetProtonVersion] ERROR parsing config.vdf: %v\n", err)
		return err
	}
	fmt.Printf("[SetProtonVersion] config.vdf parsed OK\n")

	mapping := ensureNestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")
	fmt.Printf("[SetProtonVersion] CompatToolMapping keys before update: %v\n", mapKeys(mapping))

	key := strconv.FormatUint(uint64(uint32(appID)), 10)
	fmt.Printf("[SetProtonVersion] Writing entry for key %q: name=%q config=\"\" priority=\"250\"\n", key, protonVersion)

	mapping[key] = map[string]any{
		"name":     protonVersion,
		"config":   "",
		"priority": "250",
	}
	fmt.Printf("[SetProtonVersion] CompatToolMapping keys after update: %v\n", mapKeys(mapping))

	dumped := vdf.Dump(root)
	fmt.Printf("[SetProtonVersion] Dumped VDF size: %d bytes\n", len(dumped))
	fmt.Printf("[SetProtonVersion] Writing to: %s\n", configPath)

	if err := os.WriteFile(configPath, []byte(dumped), 0o644); err != nil {
		fmt.Printf("[SetProtonVersion] ERROR writing config.vdf: %v\n", err)
		return err
	}
	fmt.Printf("[SetProtonVersion] config.vdf written successfully\n")
	return nil
}

// mapKeys returns a sorted slice of keys from a map[string]any, for debug logging.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ensureNestedMap traverses or creates nested maps along a key path.
func ensureNestedMap(root map[string]any, keys ...string) map[string]any {
	cur := root
	for _, k := range keys {
		v, ok := cur[k]
		if !ok {
			child := map[string]any{}
			cur[k] = child
			cur = child
			continue
		}
		next, ok := v.(map[string]any)
		if !ok {
			child := map[string]any{}
			cur[k] = child
			cur = child
			continue
		}
		cur = next
	}
	return cur
}

// ShutdownSteam gracefully shuts down Steam and waits for it to exit.
func (m *Manager) ShutdownSteam(ctx context.Context) error {
	// steam -shutdown exit code is unreliable; ignore it and rely on pgrep.
	exec.CommandContext(ctx, "steam", "-shutdown").Run() //nolint:errcheck

	if err := m.waitForSteamShutdown(ctx); err != nil {
		// Graceful shutdown timed out — force kill.
		if killErr := exec.Command("pkill", "-x", "steam").Run(); killErr != nil {
			// pkill returns non-zero if no process matched, which is fine.
			// Only return an error if Steam is genuinely still running.
			if pgrep := exec.Command("pgrep", "-x", "steam").Run(); pgrep == nil {
				return fmt.Errorf("failed to shut down Steam: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

// StartSteam launches Steam in the background, detached from this process.
// Any args (e.g. a steam:// URL) are passed directly to Steam.
func (m *Manager) StartSteam(args ...string) error {
	cmd := exec.Command("steam", args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Steam: %w", err)
	}
	// Release detaches the process from this one; the error is not actionable.
	cmd.Process.Release() //nolint:errcheck
	return nil
}

// waitForSteamShutdown polls until the steam process is no longer running.
func (m *Manager) waitForSteamShutdown(ctx context.Context) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(20 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return errors.New("timeout waiting for Steam to shut down")
		case <-ticker.C:
			if err := exec.Command("pgrep", "-x", "steam").Run(); err != nil {
				// pgrep returned non-zero: process not found
				time.Sleep(300 * time.Millisecond)
				return nil
			}
		}
	}
}
