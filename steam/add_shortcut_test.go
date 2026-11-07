package steam

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddShortcutImportsThroughSteam(t *testing.T) {
	for _, scenario := range []string{"import", "failure", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			manager := &Manager{userdataPath: root, userID: "123"}
			if err := os.MkdirAll(filepath.Dir(manager.ShortcutsPath()), 0o755); err != nil {
				t.Fatal(err)
			}

			executable := filepath.Join(root, "Game [test]", "game.exe")
			entry := map[string]KVValue{"appid": int32(-1234), "appname": "Test Game", "exe": `"` + executable + `"`}
			imported, err := WriteBinaryVDF(map[string]KVValue{"shortcuts": map[string]KVValue{"0": entry}})
			if err != nil {
				t.Fatal(err)
			}

			fixture := filepath.Join(root, "imported.vdf")
			request := filepath.Join(root, "request")
			writeTestFile(t, fixture, imported)
			t.Setenv("IMPORT_FIXTURE", fixture)
			t.Setenv("IMPORT_DESTINATION", manager.ShortcutsPath())
			t.Setenv("IMPORT_REQUEST", request)
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			script := "#!/bin/sh\nprintf '%s' \"$1\" > \"$IMPORT_REQUEST\"\n"
			switch scenario {
			case "import":
				script += "cp \"$IMPORT_FIXTURE\" \"$IMPORT_DESTINATION\"\n"
			case "failure":
				script += "exit 7\n"
			}
			writeTestFile(t, filepath.Join(root, "steam"), []byte(script))

			if _, err := os.Stat("/tmp/addnonsteamgamefile"); errors.Is(err, os.ErrNotExist) {
				t.Cleanup(func() { os.Remove("/tmp/addnonsteamgamefile") })
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if scenario == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			}
			defer cancel()

			appID, err := manager.AddShortcut(ctx, "Test Game", executable)
			if scenario != "import" {
				if err == nil {
					t.Fatal("expected import failure")
				}

				if scenario == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline error, got %v", err)
				}

				if _, err := os.Stat(manager.ShortcutsPath()); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed import wrote shortcuts.vdf")
				}

				return
			}

			if err != nil || appID != -1234 {
				t.Fatalf("appID=%d, err=%v", appID, err)
			}

			contents, err := os.ReadFile(manager.ShortcutsPath())
			if err != nil || !bytes.Equal(contents, imported) {
				t.Fatal("import modified Steam's saved shortcut")
			}

			rawURL, err := os.ReadFile(request)
			if err != nil {
				t.Fatal(err)
			}

			launcher, err := url.PathUnescape(strings.TrimPrefix(string(rawURL), "steam://addnonsteamgame/"))
			if err != nil {
				t.Fatal(err)
			}

			desktop, err := os.ReadFile(launcher)
			if err != nil || !strings.Contains(string(desktop), "Name=Test Game\n") || !strings.Contains(string(desktop), `Exec="`+executable+`"`) {
				t.Fatalf("invalid launcher: %q, %v", desktop, err)
			}

			writeTestFile(t, filepath.Join(root, "steam"), []byte("#!/bin/sh\nexit 7\n"))
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			relativePath, err := filepath.Rel(cwd, executable)
			if err != nil {
				t.Fatal(err)
			}

			reusedID, err := manager.AddShortcut(context.Background(), "Different Name", relativePath)
			if err != nil || reusedID != appID {
				t.Fatalf("existing shortcut not reused: %d, %v", reusedID, err)
			}
		})
	}
}

func TestUpdateImportedShortcutPreservesFieldNames(t *testing.T) {
	for _, fields := range [][2]string{{"Exe", "AppName"}, {"exe", "appname"}} {
		t.Run(fields[0], func(t *testing.T) {
			manager := &Manager{userdataPath: t.TempDir(), userID: "123"}
			if err := os.MkdirAll(filepath.Dir(manager.ShortcutsPath()), 0o755); err != nil {
				t.Fatal(err)
			}

			entry := map[string]KVValue{"appid": int32(-1234), fields[0]: `"/setup.exe"`, fields[1]: "Test (Installer)"}
			data, err := WriteBinaryVDF(map[string]KVValue{"shortcuts": map[string]KVValue{"0": entry}})
			if err != nil {
				t.Fatal(err)
			}

			writeTestFile(t, manager.ShortcutsPath(), data)
			if err := manager.UpdateShortcut(-1234, "/games/game.exe", "Test Game"); err != nil {
				t.Fatal(err)
			}

			contents, err := os.ReadFile(manager.ShortcutsPath())
			if err != nil {
				t.Fatal(err)
			}

			parsed, err := ReadBinaryVDF(contents)
			if err != nil {
				t.Fatal(err)
			}

			updated := parsed["shortcuts"].(map[string]KVValue)["0"].(map[string]KVValue)
			if updated[fields[0]] != `"/games/game.exe"` || updated[fields[1]] != "Test Game" || updated["appid"] != int32(-1234) || len(updated) != 4 {
				t.Fatalf("unexpected shortcut after update: %#v", updated)
			}
		})
	}
}

func TestShortcutURLUses64BitGameID(t *testing.T) {
	if got := GetURLAppID(-2059949906); got != "9599326596074831872" {
		t.Fatalf("GameID = %s", got)
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
}
