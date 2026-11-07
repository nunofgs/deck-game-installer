package steam

import (
	"context"
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AddShortcut asks Steam to import a launcher and returns Steam's assigned app ID.
func (m *Manager) AddShortcut(ctx context.Context, appName, exePath string) (int32, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return 0, err
	}

	if appID, _, found, _, err := m.FindShortcutByExe(exePath); err != nil {
		return 0, err
	} else if found {
		return appID, nil
	}

	launcher, err := m.writeLauncher(appName, exePath)
	if err != nil {
		return 0, err
	}

	sentinel, err := os.OpenFile("/tmp/addnonsteamgamefile", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}

	if err := sentinel.Close(); err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	steamURL := "steam://addnonsteamgame/" + url.PathEscape(launcher)
	if output, err := exec.CommandContext(ctx, "steam", steamURL).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("Steam could not import shortcut: %w: %s", err, strings.TrimSpace(string(output)))
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		appID, _, found, _, err := m.FindShortcutByExe(exePath)
		if err == nil && found {
			return appID, nil
		}

		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("waiting for Steam to save shortcut: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *Manager) writeLauncher(appName, exePath string) (string, error) {
	if strings.ContainsAny(appName, "\x00\r\n") || strings.TrimSpace(appName) == "" {
		return "", fmt.Errorf("shortcut name must be a nonempty single line")
	}

	if !filepath.IsAbs(exePath) || strings.ContainsAny(exePath, "\x00\r\n") {
		return "", fmt.Errorf("shortcut executable must be an absolute path on one line")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	directory := filepath.Join(home, ".local", "share", "deck-game-installer", "shortcuts")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(directory, fmt.Sprintf("%08x.desktop", crc32.ChecksumIEEE([]byte(exePath+"\x00"+appName))))
	escapedExe := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`, "%", "%%").Replace(exePath)
	contents := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s\nExec=%s\nPath=%s\nTerminal=false\n",
		desktopValue(appName), desktopValue(`"`+escapedExe+`"`), desktopValue(filepath.Dir(exePath)))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return "", err
	}

	return path, nil
}

func desktopValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
}
