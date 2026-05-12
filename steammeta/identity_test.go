package steammeta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentifySteamAppFromSteamAppID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "steam_appid.txt"), []byte("12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := touchFile(t, filepath.Join(dir, "Game.exe"))

	ident, err := NewIdentifierForTest(t.TempDir(), nil).Identify(t.Context(), exe)
	if err != nil {
		t.Fatal(err)
	}
	if ident.AppID != 12345 || ident.Confidence != 1 {
		t.Fatalf("identity = %#v, want app 12345 with confidence 1", ident)
	}
}

func TestIdentifySteamAppFromManifest(t *testing.T) {
	steamPath := t.TempDir()
	steamapps := filepath.Join(steamPath, "steamapps")
	gameDir := filepath.Join(steamapps, "common", "Cool Game")
	exe := touchFile(t, filepath.Join(gameDir, "Game.exe"))
	manifest := `"AppState"
{
	"appid"		"67890"
	"name"		"Cool Game"
	"installdir"		"Cool Game"
}
`
	if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_67890.acf"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	ident, err := NewIdentifierForTest(steamPath, nil).Identify(t.Context(), exe)
	if err != nil {
		t.Fatal(err)
	}
	if ident.AppID != 67890 || ident.Name != "Cool Game" {
		t.Fatalf("identity = %#v, want manifest app", ident)
	}
}

func TestStoreSearchCandidates(t *testing.T) {
	dir := t.TempDir()
	exe := touchFile(t, filepath.Join(dir, "Puzzle-Agent-2.exe"))
	apps := []AppListEntry{
		{AppID: 1, Name: "Puzzle Agent"},
		{AppID: 94590, Name: "Puzzle Agent 2"},
	}

	candidates, err := NewIdentifierForTest(t.TempDir(), apps).StoreSearchCandidates(t.Context(), exe, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2: %#v", len(candidates), candidates)
	}
	if candidates[0].AppID != 94590 {
		t.Fatalf("first candidate = %#v, want app 94590", candidates[0])
	}
}

func TestIdentifySteamAppUsesHints(t *testing.T) {
	dir := t.TempDir()
	exe := touchFile(t, filepath.Join(dir, "setup.exe"))
	apps := []AppListEntry{
		{AppID: 345, Name: "Excellent Game"},
	}

	ident, err := NewIdentifierForTest(t.TempDir(), apps).IdentifyWithHints(t.Context(), exe, []string{"Excellent Game"})
	if err != nil {
		t.Fatal(err)
	}
	if ident.AppID != 345 {
		t.Fatalf("identity = %#v, want hinted app 345", ident)
	}
}

func TestIdentifySteamAppSkipsAmbiguousStoreSearchMatch(t *testing.T) {
	dir := t.TempDir()
	exe := touchFile(t, filepath.Join(dir, "Doom.exe"))
	apps := []AppListEntry{
		{AppID: 1, Name: "Doom"},
		{AppID: 2, Name: "Doom"},
	}

	ident, err := NewIdentifierForTest(t.TempDir(), apps).Identify(t.Context(), exe)
	if err != nil {
		t.Fatal(err)
	}
	if ident.AppID != 0 {
		t.Fatalf("identity = %#v, want no confident match", ident)
	}
}

func touchFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
