package steam

import (
	"errors"
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

type Manager struct {
	steamPath    string
	userdataPath string
	userIDs      []string
	userID       string
}

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

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func (m *Manager) ShortcutsPath() string {
	return filepath.Join(m.userdataPath, m.userID, "config", "shortcuts.vdf")
}

func (m *Manager) ConfigPath() string {
	return filepath.Join(m.steamPath, "config", "config.vdf")
}

func GenerateAppID(exePath, appName string) int32 {
	key := exePath + appName
	u32 := crc32.ChecksumIEEE([]byte(key)) | 0x80000000
	return int32(u32)
}

func GetURLAppIDFromU32(u32 int32) string {
	unsigned := uint32(u32)
	u64 := (uint64(unsigned) << 32) | 0x02000000
	return strconv.FormatUint(u64, 10)
}

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
		shortcuts, ok := obj["shortcuts"].(map[string]kvValue)
		if !ok {
			continue
		}
		for _, v := range shortcuts {
			entry, ok := v.(map[string]kvValue)
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
	return 0, errors.New("not found")
}

func (m *Manager) AddShortcut(appName, exePath, args, startDir string) (int32, error) {
	path := m.ShortcutsPath()

	obj := map[string]kvValue{"shortcuts": map[string]kvValue{}}
	if data, err := os.ReadFile(path); err == nil {
		if parsed, err := ReadBinaryVDF(data); err == nil {
			obj = parsed
		}
	}

	shortcuts, ok := obj["shortcuts"].(map[string]kvValue)
	if !ok {
		shortcuts = map[string]kvValue{}
		obj["shortcuts"] = shortcuts
	}

	quoted := "\"" + exePath + "\""
	var existingKey string

	for k, v := range shortcuts {
		entry, ok := v.(map[string]kvValue)
		if !ok {
			continue
		}
		exe, _ := entry["Exe"].(string)
		if exe == quoted || exe == exePath {
			if appid, ok := entry["appid"].(int32); ok {
				return appid, nil
			}
		}
		
		// If name matches, we'll maintain this entry instead of creating a new one
		name, _ := entry["AppName"].(string)
		if name == appName {
			existingKey = k
		}
	}

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

	shortcuts[key] = map[string]kvValue{
		"appid":             appid,
		"AppName":           appName,
		"Exe":               quoted,
		"StartDir":          startDir,
		"icon":              "",
		"ShortcutPath":      "",
		"LaunchOptions":     args,
		"IsHidden":          int32(0),
		"AllowDesktopConfig": int32(1),
		"AllowOverlay":      int32(1),
		"OpenVR":            int32(0),
		"Devkit":            int32(0),
		"DevkitGameID":      "",
		"DevkitOverrideAppID": int32(0),
		"LastPlayTime":      int32(0),
		"FlatpakAppID":      "",
		"sortas":            "",
		"tags":              map[string]kvValue{},
	}

	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := WriteBinaryVDF(obj)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}

	return appid, nil
}

func (m *Manager) UpdateShortcut(appID int32, newExePath, newStartDir string) error {
	path := m.ShortcutsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	obj, err := ReadBinaryVDF(data)
	if err != nil {
		return err
	}

	shortcuts, ok := obj["shortcuts"].(map[string]kvValue)
	if !ok {
		return errors.New("shortcuts not found")
	}

	// Find the shortcut with matching appID
	for _, v := range shortcuts {
		entry, ok := v.(map[string]kvValue)
		if !ok {
			continue
		}
		if existingID, ok := entry["appid"].(int32); ok && existingID == appID {
			// Update the exe and start directory
			entry["Exe"] = "\"" + newExePath + "\""
			if newStartDir == "" {
				newStartDir = filepath.Dir(newExePath)
			}
			entry["StartDir"] = newStartDir
			
			// Remove "(Installer)" suffix from the app name if present
			if appName, ok := entry["AppName"].(string); ok {
				if len(appName) > 12 && appName[len(appName)-12:] == " (Installer)" {
					entry["AppName"] = appName[:len(appName)-12]
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

func nextShortcutIndex(shortcuts map[string]kvValue) int {
	max := -1
	for k := range shortcuts {
		if n, err := strconv.Atoi(k); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

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

	mapping := nestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")
	if mapping == nil {
		return "", errors.New("mapping not found")
	}

	unsigned := uint32(appID)
	key := strconv.FormatUint(uint64(unsigned), 10)
	entry, ok := mapping[key].(map[string]any)
	if !ok {
		return "", errors.New("entry not found")
	}
	name, _ := entry["name"].(string)
	if name == "" {
		return "", errors.New("name not found")
	}
	return name, nil
}

func (m *Manager) SetProtonVersion(appID int32, protonVersion string) error {
	path := m.ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	root, err := vdf.Parse(string(data))
	if err != nil {
		return err
	}

	mapping := ensureNestedMap(root, "InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping")
	unsigned := uint32(appID)
	key := strconv.FormatUint(uint64(unsigned), 10)

	mapping[key] = map[string]any{
		"name":     protonVersion,
		"config":   "",
		"priority": "250",
	}

	out := vdf.Dump(root)
	return os.WriteFile(path, []byte(out), 0o644)
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

func (m *Manager) RestartSteam() {
	// Kill Steam gracefully first
	_ = exec.Command("steam", "-shutdown").Run()
	
	// Wait for Steam to actually shut down by monitoring its log
	if err := m.waitForSteamShutdown(); err != nil {
		// If monitoring fails, fall back to force kill
		_ = exec.Command("pkill", "-x", "steam").Run()
		time.Sleep(2 * time.Second)
	}
	
	// Start Steam in background, fully detached from this process
	cmd := exec.Command("steam")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session to detach from parent
	}
	_ = cmd.Start()
	
	// Release the process so it's not tied to this parent
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}

func (m *Manager) waitForSteamShutdown() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	
	// Path to Steam's console log
	logPath := filepath.Join(home, ".local", "share", "Steam", "logs", "console-linux.txt")
	
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return errors.New("Steam log file not found")
	}
	
	// Get initial file size to start reading from the end
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return err
	}
	offset := fileInfo.Size()
	
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	
	timeout := time.After(15 * time.Second) // 15 second timeout for shutdown
	
	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for Steam shutdown")
			
		case <-ticker.C:
			// Check if steam process is still running
			cmd := exec.Command("pgrep", "-x", "steam")
			if err := cmd.Run(); err != nil {
				// Process not found, Steam has shut down
				time.Sleep(500 * time.Millisecond)
				return nil
			}
			
			file, err := os.Open(logPath)
			if err != nil {
				continue
			}
			
			// Seek to last known position
			file.Seek(offset, 0)
			
			// Read new content
			buf := make([]byte, 8192)
			n, _ := file.Read(buf)
			if n > 0 {
				offset += int64(n)
				content := string(buf[:n])
				
				// Look for shutdown messages in Steam log
				if strings.Contains(content, "Exiting") || 
				   strings.Contains(content, "Shutdown") ||
				   strings.Contains(content, "Steam_Shutdown") {
					file.Close()
					// Give it a moment to fully terminate
					time.Sleep(500 * time.Millisecond)
					return nil
				}
			}
			file.Close()
		}
	}
}

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
