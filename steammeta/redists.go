package steammeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type commonRedistMapping struct {
	name  string
	verbs []string
}

var commonRedistDepotMappings = map[string]commonRedistMapping{
	"228982": {name: "VC 2008 Redist", verbs: []string{"vcrun2008"}},
	"228983": {name: "VC 2010 Redist", verbs: []string{"vcrun2010"}},
	"228984": {name: "VC 2012 Redist", verbs: []string{"vcrun2012"}},
	"228985": {name: "VC 2013 Redist", verbs: []string{"vcrun2013"}},
	"228986": {name: "VC 2015 Redist", verbs: []string{"vcrun2019"}},
	"228987": {name: "VC 2017 Redist", verbs: []string{"vcrun2019"}},
	"228988": {name: "VC 2019 Redist", verbs: []string{"vcrun2019"}},
	"228989": {name: "VC 2022 Redist", verbs: []string{"vcrun2022"}},
	"228990": {name: "DirectX Jun 2010 Redist", verbs: []string{"d3dx9", "d3dcompiler_43", "xact"}},
	"229000": {name: ".NET 3.5 Redist", verbs: []string{"dotnet35"}},
	"229001": {name: ".NET 3.5 Client Profile Redist", verbs: []string{"dotnet35"}},
	"229002": {name: ".NET 4.0 Redist", verbs: []string{"dotnet40"}},
	"229003": {name: ".NET 4.0 Client Profile Redist", verbs: []string{"dotnet40"}},
	"229004": {name: ".NET 4.5.2 Redist", verbs: []string{"dotnet452"}},
	"229005": {name: ".NET 4.6 Redist", verbs: []string{"dotnet462"}},
	"229006": {name: ".NET 4.7 Redist", verbs: []string{"dotnet472"}},
	"229007": {name: ".NET 4.8 Redist", verbs: []string{"dotnet48"}},
	"229010": {name: "XNA 3.0 Redist"},
	"229011": {name: "XNA 3.1 Redist", verbs: []string{"xna31"}},
	"229012": {name: "XNA 4.0 Redist", verbs: []string{"xna40"}},
	"229020": {name: "OpenAL 2.0.7.0 Redist", verbs: []string{"openal"}},
	"229030": {name: "PhysX System Software 8.09.04", verbs: []string{"physx"}},
	"229031": {name: "PhysX System Software 9.12.1031", verbs: []string{"physx"}},
	"229032": {name: "PhysX System Software 9.13.1220", verbs: []string{"physx"}},
	"229033": {name: "PhysX System Software 9.14.0702", verbs: []string{"physx"}},
}

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

// NewRedistResolver creates a resolver backed by steamcmd.net appinfo,
// falling back to local SteamCMD when available.
func NewRedistResolver() *RedistResolver {
	return &RedistResolver{provider: NewFallbackProvider(
		NewSteamCMDNetProvider(),
		NewSteamCMDProvider(),
	)}
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
		redists[i].Verbs = VerbsForRedist(redists[i])
	}
	return redists, nil
}

// FallbackProvider tries several providers in order.
type FallbackProvider struct {
	providers []AppInfoProvider
}

// NewFallbackProvider creates an appinfo provider chain.
func NewFallbackProvider(providers ...AppInfoProvider) *FallbackProvider {
	return &FallbackProvider{providers: providers}
}

// AppInfo fetches appinfo from the first provider that succeeds.
func (p *FallbackProvider) AppInfo(ctx context.Context, appID int) (map[string]any, error) {
	var lastErr error
	for _, provider := range p.providers {
		app, err := provider.AppInfo(ctx, appID)
		if err == nil {
			return app, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no appinfo providers configured")
}

// SteamCMDNetProvider fetches appinfo through https://api.steamcmd.net.
type SteamCMDNetProvider struct {
	baseURL string
	client  *http.Client
}

// NewSteamCMDNetProvider creates a steamcmd.net appinfo provider.
func NewSteamCMDNetProvider() *SteamCMDNetProvider {
	return &SteamCMDNetProvider{
		baseURL: "https://api.steamcmd.net/v1/info/",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// AppInfo fetches and parses appinfo for one app.
func (p *SteamCMDNetProvider) AppInfo(ctx context.Context, appID int) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+strconv.Itoa(appID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steamcmd.net returned HTTP %d", resp.StatusCode)
	}

	var out struct {
		Status string                    `json:"status"`
		Data   map[string]map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("steamcmd.net returned status %q", out.Status)
	}
	app := out.Data[strconv.Itoa(appID)]
	if app == nil {
		return nil, fmt.Errorf("steamcmd.net output did not contain app %d", appID)
	}
	return app, nil
}

// SteamCMDProvider fetches appinfo by invoking Valve's steamcmd tool.
type SteamCMDProvider struct {
	path string
}

// NewSteamCMDProvider creates a SteamCMD appinfo provider.
func NewSteamCMDProvider() *SteamCMDProvider {
	return &SteamCMDProvider{path: findSteamCMD()}
}

// AppInfo fetches and parses appinfo for one app, using a per-app cache to
// avoid repeated full-database steamcmd updates.
func (p *SteamCMDProvider) AppInfo(ctx context.Context, appID int) (map[string]any, error) {
	if p.path == "" {
		return nil, fmt.Errorf("steamcmd not found")
	}

	if cached, ok := loadAppInfoCache(appID); ok {
		return cached, nil
	}

	app, err := p.fetchAppInfo(ctx, appID)
	if err != nil {
		return nil, err
	}
	saveAppInfoCache(appID, app)
	return app, nil
}

// fetchAppInfo runs steamcmd to fetch appinfo for a single app.
// It tries a fast path (no update) first; if the app is missing from the local
// cache it falls back to a full app_info_update and retries.
func (p *SteamCMDProvider) fetchAppInfo(ctx context.Context, appID int) (map[string]any, error) {
	// Fast path: use whatever steamcmd already has cached locally.
	if app, err := p.runAppInfoPrint(ctx, appID, false); err == nil {
		return app, nil
	}

	// Slow path: force a full update so steamcmd fetches this app's depot data.
	// This can take several minutes on first run — it downloads the entire Steam
	// app database. The result is cached above so this only happens once per app.
	return p.runAppInfoPrint(ctx, appID, true)
}

func (p *SteamCMDProvider) runAppInfoPrint(ctx context.Context, appID int, forceUpdate bool) (map[string]any, error) {
	var script string
	if forceUpdate {
		script = "login anonymous\napp_info_update 1\napp_info_print " + strconv.Itoa(appID) + "\nquit\n"
	} else {
		script = "login anonymous\napp_info_print " + strconv.Itoa(appID) + "\nquit\n"
	}
	cmd := exec.CommandContext(ctx, p.path)
	cmd.Stdin = strings.NewReader(script)
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

func appInfoCachePath(appID int) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "deck-game-installer", "appinfo", strconv.Itoa(appID)+".json")
}

func loadAppInfoCache(appID int) (map[string]any, bool) {
	path := appInfoCachePath(appID)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}
	return result, len(result) > 0
}

func saveAppInfoCache(appID int, info map[string]any) {
	path := appInfoCachePath(appID)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
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
			name = commonRedistDepotDisplayName(depotID)
		}
		if name == "" {
			name = "Steam common redist depot " + depotID
		}
		redists = append(redists, SteamRedist{DepotID: depotID, Name: name})
	}
	sort.Slice(redists, func(i, j int) bool { return redists[i].DepotID < redists[j].DepotID })
	return redists
}

func commonRedistDepotDisplayName(depotID string) string {
	if mapping, ok := commonRedistDepotMappings[depotID]; ok {
		return mapping.name
	}
	return ""
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

// VerbsForRedist maps a Steam common redist to winetricks verbs. Known
// Steamworks Common Redistributables depots use depot IDs first; fetched names
// are only a fallback for unknown future depots or unusual appinfo.
func VerbsForRedist(redist SteamRedist) []string {
	if mapping, ok := commonRedistDepotMappings[redist.DepotID]; ok {
		return append([]string(nil), mapping.verbs...)
	}
	return VerbsForRedistName(redist.Name)
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
	case strings.Contains(n, "net framework") || strings.Contains(n, "dotnet") || strings.HasPrefix(n, "net "):
		return dotnetVerb(n)
	case strings.Contains(n, "openal"):
		return []string{"openal"}
	case strings.Contains(n, "xna"):
		if strings.Contains(n, "3 1") || strings.Contains(n, "31") {
			return []string{"xna31"}
		}
		if strings.Contains(n, "4 0") || strings.Contains(n, "40") {
			return []string{"xna40"}
		}
		return nil
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
	versionMap := []struct {
		marker string
		verb   string
	}{
		{marker: "4 8", verb: "dotnet48"},
		{marker: "48", verb: "dotnet48"},
		{marker: "4 7", verb: "dotnet472"},
		{marker: "47", verb: "dotnet472"},
		{marker: "4 6", verb: "dotnet462"},
		{marker: "46", verb: "dotnet462"},
		{marker: "4 5", verb: "dotnet452"},
		{marker: "45", verb: "dotnet452"},
		{marker: "4 0", verb: "dotnet40"},
		{marker: "40", verb: "dotnet40"},
		{marker: "3 5", verb: "dotnet35"},
		{marker: "35", verb: "dotnet35"},
	}
	for _, version := range versionMap {
		if strings.Contains(name, version.marker) {
			return []string{version.verb}
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
		verbs = append(verbs, VerbsForRedist(redist)...)
	}
	return dedupeStrings(verbs)
}
