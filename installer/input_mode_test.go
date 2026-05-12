package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInputMode(t *testing.T) {
	t.Run("installer exe", func(t *testing.T) {
		dir := t.TempDir()
		path := touch(t, filepath.Join(dir, "setup.exe"))

		mode, _, err := DetectInputMode(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode != InputModeInstaller {
			t.Fatalf("mode = %q, want %q", mode, InputModeInstaller)
		}
	})

	t.Run("unity portable exe", func(t *testing.T) {
		dir := t.TempDir()
		path := touch(t, filepath.Join(dir, "CoolGame.exe"))
		touch(t, filepath.Join(dir, "UnityPlayer.dll"))
		touch(t, filepath.Join(dir, "steam_appid.txt"))

		mode, _, err := DetectInputMode(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode != InputModePortable {
			t.Fatalf("mode = %q, want %q", mode, InputModePortable)
		}
	})

	t.Run("unreal portable exe", func(t *testing.T) {
		dir := t.TempDir()
		exeDir := filepath.Join(dir, "Binaries", "Win64")
		if err := os.MkdirAll(filepath.Join(dir, "Content", "Paks"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(exeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := touch(t, filepath.Join(exeDir, "CoolGame-Win64-Shipping.exe"))

		mode, _, err := DetectInputMode(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode != InputModePortable {
			t.Fatalf("mode = %q, want %q", mode, InputModePortable)
		}
	})

	t.Run("folder is unsupported", func(t *testing.T) {
		dir := t.TempDir()
		if _, _, err := DetectInputMode(dir); err == nil {
			t.Fatal("DetectInputMode(folder) returned nil error, want unsupported")
		}
	})

	t.Run("ambiguous exe", func(t *testing.T) {
		dir := t.TempDir()
		path := touch(t, filepath.Join(dir, "Game.exe"))

		mode, _, err := DetectInputMode(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode != InputModeAmbiguous {
			t.Fatalf("mode = %q, want %q", mode, InputModeAmbiguous)
		}
	})
}

func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
