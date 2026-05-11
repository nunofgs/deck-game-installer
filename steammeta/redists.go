package steammeta

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"deck-game-installer/vdf"
)

const commonRedistsAppID = 228980

var appBlockStart = regexp.MustCompile(`^\s*"(\d+)"\s*$`)

// SteamRedist is one Steamworks Common Redistributables depot referenced by a game.
type SteamRedist struct {
	DepotID string
	Name    string
	Verbs   []string
}

// AppInfoProvider fetches Steam appinfo metadata.
type AppInfoProvider interface {
	AppInfo(ctx context.Context, appID int) (map[string]any, error)
}

// RedistResolver resolves Steam common redists from appinfo metadata.
type RedistResolver struct {
	provider AppInfoProvider
}

// NewRedistResolver creates a resolver backed by SteamCMD appinfo.
func NewRedistResolver() *RedistResolver {
	return &RedistResolver{provider: NewSteamCMDProvider()}
}

// NewRedistResolverWithProvider creates a resolver with a test/provider override.
func NewRedistResolverWithProvider(provider AppInfoProvider) *RedistResolver {
	return &RedistResolver{provider: provider}
}

// ResolveCommonRedists resolves the common redists declared by appID.
func ResolveCommonRedists(ctx context.Context, appID int) ([]SteamRedist, error) {
	return NewRedistResolver().Resolve(ctx, appID)
}

// Resolve resolves the common redists declared by appID.
func (r *RedistResolver) Resolve(ctx context.Context, appID int) ([]SteamRedist, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	app, err := r.provider.AppInfo(ctx, appID)
	if err != nil {
		return nil, err
	}
	common, err := r.provider.AppInfo(ctx, commonRedistsAppID)
	if err != nil {
		return nil, err
	}

	redists := ExtractCommonRedists(app, common)
	for i := range redists {
		redists[i].Verbs = VerbsForRedistName(redists[i].Name)
	}
	return redists, nil
}

// SteamCMDProvider fetches appinfo by invoking Valve's steamcmd tool.
type SteamCMDProvider struct {
	path string
}

// NewSteamCMDProvider creates a SteamCMD appinfo provider.
func NewSteamCMDProvider() *SteamCMDProvider {
	return &SteamCMDProvider{path: findSteamCMD()}
}

// AppInfo fetches and parses appinfo for one app.
func (p *SteamCMDProvider) AppInfo(ctx context.Context, appID int) (map[string]any, error) {
	if p.path == "" {
		return nil, fmt.Errorf("steamcmd not found")
	}
	cmd := exec.CommandContext(ctx, p.path)
	cmd.Stdin = strings.NewReader(
		"login anonymous\n" +
			"app_info_update 1\n" +
			"app_info_print " + strconv.Itoa(appID) + "\n" +
			"quit\n",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("steamcmd app_info_print failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	parsed, err := ParseAppInfoOutput(out.String())
	if err != nil {
		return nil, err
	}
	app := parsed[appID]
	if app == nil {
		return nil, fmt.Errorf("steamcmd output did not contain app %d", appID)
	}
	return app, nil
}

func findSteamCMD() string {
	if path, err := exec.LookPath("steamcmd"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "Steam", "steamcmd.sh"),
		filepath.Join(home, ".steam", "steamcmd", "steamcmd.sh"),
		filepath.Join(home, ".local", "share", "Steam", "steamcmd", "steamcmd.sh"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ParseAppInfoOutput parses VDF blocks printed by steamcmd app_info_print.
func ParseAppInfoOutput(raw string) (map[int]map[string]any, error) {
	lines := strings.Split(raw, "\n")
	out := map[int]map[string]any{}
	for i := 0; i < len(lines); i++ {
		matches := appBlockStart.FindStringSubmatch(lines[i])
		if len(matches) != 2 {
			continue
		}
		appID, _ := strconv.Atoi(matches[1])
		var block []string
		block = append(block, lines[i])

		balance := 0
		started := false
		for j := i + 1; j < len(lines); j++ {
			line := lines[j]
			block = append(block, line)
			delta, sawBrace := braceDelta(line)
			balance += delta
			started = started || sawBrace
			if started && balance == 0 {
				i = j
				break
			}
		}
		if !started {
			continue
		}
		parsed, err := vdf.Parse(strings.Join(block, "\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse appinfo for %d: %w", appID, err)
		}
		if app, ok := parsed[strconv.Itoa(appID)].(map[string]any); ok {
			out[appID] = app
		}
	}
	return out, nil
}

func braceDelta(line string) (int, bool) {
	delta := 0
	inQuote := false
	escaped := false
	sawBrace := false
	for _, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch r {
		case '{':
			delta++
			sawBrace = true
		case '}':
			delta--
			sawBrace = true
		}
	}
	return delta, sawBrace
}

// ExtractCommonRedists extracts depots that point at Steamworks Common Redists.
func ExtractCommonRedists(appInfo, commonInfo map[string]any) []SteamRedist {
	appDepots := mapValue(appInfo["depots"])
	commonDepots := mapValue(commonInfo["depots"])
	if appDepots == nil || commonDepots == nil {
		return nil
	}

	var redists []SteamRedist
	for depotID, raw := range appDepots {
		if _, err := strconv.Atoi(depotID); err != nil {
			continue
		}
		depot := mapValue(raw)
		if depot == nil {
			continue
		}
		if !referencesCommonRedists(depot, depotID, commonDepots) {
			continue
		}
		name := firstRecursiveString(depot, "name", "display_name", "description")
		if commonDepot := mapValue(commonDepots[depotID]); name == "" && commonDepot != nil {
			name = firstRecursiveString(commonDepot, "name", "display_name", "description")
		}
		if name == "" {
			name = "Steam common redist depot " + depotID
		}
		redists = append(redists, SteamRedist{DepotID: depotID, Name: name})
	}
	sort.Slice(redists, func(i, j int) bool { return redists[i].DepotID < redists[j].DepotID })
	return redists
}

func referencesCommonRedists(depot map[string]any, depotID string, commonDepots map[string]any) bool {
	if recursiveKeyValue(depot, "depotfromapp", strconv.Itoa(commonRedistsAppID)) ||
		recursiveKeyValue(depot, "depot_from_app", strconv.Itoa(commonRedistsAppID)) {
		return true
	}
	if _, ok := commonDepots[depotID]; ok && recursiveKeyValue(depot, "sharedinstall", "1") {
		return true
	}
	return false
}

func mapValue(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func recursiveKeyValue(m map[string]any, key, expected string) bool {
	for k, value := range m {
		if strings.EqualFold(k, key) && stringValue(value) == expected {
			return true
		}
		if child := mapValue(value); child != nil && recursiveKeyValue(child, key, expected) {
			return true
		}
	}
	return false
}

func firstRecursiveString(m map[string]any, keys ...string) string {
	want := map[string]struct{}{}
	for _, key := range keys {
		want[strings.ToLower(key)] = struct{}{}
	}
	for k, value := range m {
		if _, ok := want[strings.ToLower(k)]; ok {
			if s := stringValue(value); s != "" {
				return s
			}
		}
		if child := mapValue(value); child != nil {
			if s := firstRecursiveString(child, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}

// VerbsForRedistName maps a fetched Steam redist name to winetricks verbs.
func VerbsForRedistName(name string) []string {
	n := normalizeForMatch(name)
	switch {
	case strings.Contains(n, "visual c") ||
		strings.Contains(n, "vcredist") ||
		strings.Contains(n, "vc redist") ||
		strings.Contains(n, "vc runtime") ||
		strings.HasPrefix(n, "vc ") ||
		strings.Contains(n, " vc "):
		return vcRunVerb(n)
	case strings.Contains(n, "directx") || strings.Contains(n, "d3dx") || strings.Contains(n, "xact"):
		return []string{"d3dx9", "d3dcompiler_43", "xact"}
	case strings.Contains(n, "net framework") || strings.Contains(n, "dotnet"):
		return dotnetVerb(n)
	case strings.Contains(n, "openal"):
		return []string{"openal"}
	case strings.Contains(n, "xna"):
		if strings.Contains(n, "3 1") || strings.Contains(n, "31") {
			return []string{"xna31"}
		}
		return []string{"xna40"}
	case strings.Contains(n, "physx"):
		return []string{"physx"}
	default:
		return nil
	}
}

func vcRunVerb(name string) []string {
	versions := []string{"2022", "2019", "2017", "2015", "2013", "2012", "2010", "2008", "2005"}
	for _, version := range versions {
		if strings.Contains(name, version) {
			switch version {
			case "2022":
				return []string{"vcrun2022"}
			case "2019", "2017", "2015":
				return []string{"vcrun2019"}
			default:
				return []string{"vcrun" + version}
			}
		}
	}
	return []string{"vcrun2019"}
}

func dotnetVerb(name string) []string {
	versionMap := map[string]string{
		"4 8": "dotnet48",
		"48":  "dotnet48",
		"4 7": "dotnet472",
		"47":  "dotnet472",
		"4 6": "dotnet462",
		"46":  "dotnet462",
		"4 5": "dotnet452",
		"45":  "dotnet452",
	}
	for marker, verb := range versionMap {
		if strings.Contains(name, marker) {
			return []string{verb}
		}
	}
	return []string{"dotnet48"}
}

// WinetricksVerbs returns a deduplicated ordered list of install verbs.
func WinetricksVerbs(redists []SteamRedist) []string {
	var verbs []string
	for _, redist := range redists {
		if len(redist.Verbs) > 0 {
			verbs = append(verbs, redist.Verbs...)
			continue
		}
		verbs = append(verbs, VerbsForRedistName(redist.Name)...)
	}
	return dedupeStrings(verbs)
}
