// Package steammeta resolves Steam app identity and common redistributable
// metadata from Valve/Steam data sources.
package steammeta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"deck-game-installer/vdf"
)

// Identification is the best matching Steam app for a local game path.
type Identification struct {
	AppID      int
	Name       string
	Confidence float64
	Reason     string
}

// AppListEntry is one app candidate returned by Steam Store search.
type AppListEntry struct {
	AppID int
	Name  string
}

// Identifier identifies the Steam app that most likely matches a local path.
type Identifier struct {
	steamPath string
	apps      []AppListEntry
	client    *http.Client
}

// NewIdentifier creates an Identifier using the default Steam install path.
func NewIdentifier() *Identifier {
	home, _ := os.UserHomeDir()
	return &Identifier{
		steamPath: filepath.Join(home, ".local", "share", "Steam"),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NewIdentifierForTest creates an Identifier with explicit Steam path/app list.
func NewIdentifierForTest(steamPath string, apps []AppListEntry) *Identifier {
	return &Identifier{
		steamPath: steamPath,
		apps:      apps,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// IdentifySteamApp resolves the likely Steam app for path.
func IdentifySteamApp(ctx context.Context, path string) (Identification, error) {
	return NewIdentifier().Identify(ctx, path)
}

// Identify resolves the likely Steam app for path.
func (i *Identifier) Identify(ctx context.Context, path string) (Identification, error) {
	return i.IdentifyWithHints(ctx, path, nil)
}

// IdentifyWithHints resolves the likely Steam app for path, using extra names
// such as an ISO-derived game title when filename/path clues are weak.
func (i *Identifier) IdentifyWithHints(ctx context.Context, path string, hints []string) (Identification, error) {
	root, err := identityRoot(path)
	if err != nil {
		return Identification{}, err
	}

	if appID, from := readSteamAppID(root); appID != 0 {
		return Identification{
			AppID:      appID,
			Confidence: 1,
			Reason:     "found " + from,
		}, nil
	}

	if ident, ok := i.identifyFromManifest(path); ok {
		return ident, nil
	}

	return i.identifyFromStoreSearch(ctx, path, root, hints)
}

func identityRoot(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return path, nil
	}
	return filepath.Dir(path), nil
}

func readSteamAppID(root string) (int, string) {
	seen := map[string]struct{}{}
	dir := root
	for depth := 0; depth < 5 && dir != "" && dir != string(filepath.Separator); depth++ {
		if _, ok := seen[dir]; ok {
			break
		}
		seen[dir] = struct{}{}
		path := filepath.Join(dir, "steam_appid.txt")
		if data, err := os.ReadFile(path); err == nil {
			id, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if id > 0 {
				return id, path
			}
		}
		dir = filepath.Dir(dir)
	}
	return 0, ""
}

func (i *Identifier) identifyFromManifest(path string) (Identification, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for _, steamapps := range i.steamAppsDirs() {
		common := filepath.Join(steamapps, "common")
		rel, err := filepath.Rel(common, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 0 || parts[0] == "." || parts[0] == "" {
			continue
		}
		folder := parts[0]

		manifests, err := filepath.Glob(filepath.Join(steamapps, "appmanifest_*.acf"))
		if err != nil {
			continue
		}
		for _, manifest := range manifests {
			appID, name, installdir := parseAppManifest(manifest)
			if appID == 0 {
				continue
			}
			if strings.EqualFold(installdir, folder) {
				return Identification{
					AppID:      appID,
					Name:       name,
					Confidence: 1,
					Reason:     "matched Steam appmanifest installdir",
				}, true
			}
		}
	}
	return Identification{}, false
}

func parseAppManifest(path string) (int, string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", ""
	}
	root, err := vdf.Parse(string(data))
	if err != nil {
		return 0, "", ""
	}
	appState := vdf.GetNestedMap(root, "AppState")
	if appState == nil {
		return 0, "", ""
	}
	appID, _ := strconv.Atoi(stringValue(appState["appid"]))
	return appID, stringValue(appState["name"]), stringValue(appState["installdir"])
}

func (i *Identifier) steamAppsDirs() []string {
	paths := []string{filepath.Join(i.steamPath, "steamapps")}
	data, err := os.ReadFile(filepath.Join(i.steamPath, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return dedupeStrings(paths)
	}
	root, err := vdf.Parse(string(data))
	if err != nil {
		return dedupeStrings(paths)
	}
	libraries := vdf.GetNestedMap(root, "libraryfolders")
	for key, raw := range libraries {
		if key == "contentstatsid" {
			continue
		}
		switch value := raw.(type) {
		case string:
			if value != "" {
				paths = append(paths, filepath.Join(value, "steamapps"))
			}
		case map[string]any:
			if p := stringValue(value["path"]); p != "" {
				paths = append(paths, filepath.Join(p, "steamapps"))
			}
		}
	}
	return dedupeStrings(paths)
}

func (i *Identifier) identifyFromStoreSearch(ctx context.Context, path, root string, hints []string) (Identification, error) {
	names := append(candidateNames(path, root), hints...)
	names = cleanCandidateNames(names)
	if len(names) == 0 {
		return Identification{}, nil
	}

	type match struct {
		app   AppListEntry
		score float64
		name  string
	}
	var matches []match
	for _, candidate := range names {
		apps, err := i.searchApps(ctx, candidate)
		if err != nil {
			return Identification{}, err
		}
		for _, app := range apps {
			if app.Name == "" {
				continue
			}
			score := nameScore(candidate, app.Name)
			if score <= 0 {
				continue
			}
			matches = append(matches, match{app: app, score: score, name: candidate})
		}
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score == matches[b].score {
			return matches[a].app.AppID < matches[b].app.AppID
		}
		return matches[a].score > matches[b].score
	})
	if len(matches) == 0 {
		return Identification{}, nil
	}

	top := matches[0]
	second := 0.0
	if len(matches) > 1 {
		second = matches[1].score
	}
	if top.score < 0.92 || top.score-second < 0.05 {
		return Identification{}, nil
	}
	return Identification{
		AppID:      top.app.AppID,
		Name:       top.app.Name,
		Confidence: top.score,
		Reason:     fmt.Sprintf("matched Steam Store search name %q", top.name),
	}, nil
}

func (i *Identifier) searchApps(ctx context.Context, term string) ([]AppListEntry, error) {
	if len(i.apps) > 0 {
		return i.apps, nil
	}

	values := url.Values{}
	values.Set("term", term)
	values.Set("l", "english")
	values.Set("cc", "US")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://store.steampowered.com/api/storesearch/?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Steam Store search returned HTTP %d", resp.StatusCode)
	}
	var out struct {
		Items []struct {
			Type string `json:"type"`
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	apps := make([]AppListEntry, 0, len(out.Items))
	for _, item := range out.Items {
		if item.Type != "app" || item.ID == 0 || item.Name == "" {
			continue
		}
		apps = append(apps, AppListEntry{AppID: item.ID, Name: item.Name})
	}
	return apps, nil
}

func candidateNames(path, root string) []string {
	var names []string
	add := func(value string) {
		names = append(names, value)
	}
	add(filepath.Base(root))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		add(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	return cleanCandidateNames(names)
}

func cleanCandidateNames(names []string) []string {
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		name = cleanCandidateName(name)
		if name != "" {
			cleaned = append(cleaned, name)
		}
	}
	return dedupeStrings(cleaned)
}

func cleanCandidateName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "_files")
	name = strings.TrimSuffix(name, "-files")
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ")
	return strings.Join(strings.Fields(replacer.Replace(name)), " ")
}

func nameScore(candidate, appName string) float64 {
	c := normalizeForMatch(candidate)
	a := normalizeForMatch(appName)
	if c == "" || a == "" {
		return 0
	}
	if c == a {
		return 1
	}
	if strings.HasPrefix(a, c) || strings.HasPrefix(c, a) {
		return 0.94
	}
	return tokenJaccard(c, a)
}

func normalizeForMatch(name string) string {
	name = strings.ToLower(name)
	articles := map[string]struct{}{"the": {}, "a": {}, "an": {}}
	var words []string
	var b strings.Builder
	flush := func() {
		word := b.String()
		b.Reset()
		if word == "" {
			return
		}
		if _, skip := articles[word]; skip {
			return
		}
		words = append(words, word)
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return strings.Join(words, " ")
}

func tokenJaccard(a, b string) float64 {
	as := tokenSet(a)
	bs := tokenSet(b)
	if len(as) == 0 || len(bs) == 0 {
		return 0
	}
	intersections := 0
	for word := range as {
		if _, ok := bs[word]; ok {
			intersections++
		}
	}
	union := len(as) + len(bs) - intersections
	return float64(intersections) / float64(union)
}

func tokenSet(value string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, word := range strings.Fields(value) {
		out[word] = struct{}{}
	}
	return out
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
