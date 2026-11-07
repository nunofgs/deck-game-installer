package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, test := range []struct {
		candidate, current string
		want               bool
	}{
		{"v1.0.9", "v1.0.8", true},
		{"v1.0.10", "v1.0.9", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.8", "v1.0.8", false},
		{"v1.0.7", "v1.0.8", false},
		{"v1.0.9-rc1", "v1.0.8", false},
		{"invalid", "v1.0.8", false},
		{"v1.0.9", "dev", false},
	} {
		if got := newer(test.candidate, test.current); got != test.want {
			t.Errorf("newer(%q, %q) = %v, want %v", test.candidate, test.current, got, test.want)
		}
	}
}

func TestUpdate(t *testing.T) {
	for _, scenario := range []string{"update", "current", "older", "prerelease", "offline", "bad checksum", "corrupt zip", "missing executable", "missing asset"} {
		t.Run(scenario, func(t *testing.T) {
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			name := "deck-game-installer"
			if scenario == "missing executable" {
				name = "../deck-game-installer"
			}

			file, err := writer.Create(name)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := file.Write([]byte("#!/bin/sh\nprintf 'updated:%s' \"$1\"\n")); err != nil {
				t.Fatal(err)
			}

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			data := archive.Bytes()
			if scenario == "corrupt zip" {
				data = []byte("not a zip")
			}

			digest := sha256.Sum256(data)
			checksum := "sha256:" + hex.EncodeToString(digest[:])
			if scenario == "bad checksum" {
				checksum = "sha256:wrong"
			}

			tag := "v1.0.9"
			if scenario == "current" {
				tag = "v1.0.8"
			}

			if scenario == "older" {
				tag = "v1.0.7"
			}

			downloads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "offline" {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}

				if r.URL.Path == "/archive" {
					downloads++
					w.Write(data)
					return
				}

				assets := []map[string]string{{"name": "deck-game-installer.zip", "browser_download_url": "http://" + r.Host + "/archive", "digest": checksum}}
				if scenario == "missing asset" {
					assets = nil
				}

				json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "prerelease": scenario == "prerelease", "assets": assets})
			}))
			defer server.Close()

			executable := filepath.Join(t.TempDir(), "deck-game-installer")
			if err := os.WriteFile(executable, []byte("original"), 0o755); err != nil {
				t.Fatal(err)
			}

			updated, err := update(context.Background(), server.Client(), server.URL, executable, "v1.0.8", func(message string) { t.Log(message) })
			wantError := scenario != "update" && scenario != "current" && scenario != "older" && scenario != "prerelease"
			if (err != nil) != wantError || updated != (scenario == "update") {
				t.Fatalf("updated=%v, err=%v", updated, err)
			}

			if updated {
				output, err := exec.Command(executable, "game with spaces.exe").Output()
				if err != nil || string(output) != "updated:game with spaces.exe" {
					t.Fatalf("replacement failed to execute: %q, %v", output, err)
				}
			} else {
				contents, err := os.ReadFile(executable)
				if err != nil || string(contents) != "original" {
					t.Fatalf("original executable changed: %q, %v", contents, err)
				}
			}

			if (scenario == "current" || scenario == "older" || scenario == "prerelease") && downloads != 0 {
				t.Fatal("downloaded an unnecessary update")
			}

			files, err := os.ReadDir(filepath.Dir(executable))
			if err != nil || len(files) != 1 {
				t.Fatalf("temporary files remain: %v, %v", files, err)
			}
		})
	}
}
