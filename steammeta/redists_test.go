package steammeta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type staticAppInfoProvider map[int]map[string]any

func (p staticAppInfoProvider) AppInfo(ctx context.Context, appID int) (map[string]any, error) {
	return p[appID], nil
}

func TestParseAndExtractCommonRedists(t *testing.T) {
	raw := `
noise before
"123"
{
	"common"
	{
		"name"	"Fixture Game"
	}
	"depots"
	{
		"228989"
		{
			"depotfromapp"	"228980"
		}
		"228990"
		{
			"sharedinstall"	"1"
		}
		"500000"
		{
			"name"	"Game Content"
		}
	}
}
"228980"
{
	"depots"
	{
		"228989"
		{
			"name"	"VC++ 2022 Redist"
		}
		"228990"
		{
			"name"	"DirectX Jun 2010 Redist"
		}
	}
}
`
	parsed, err := ParseAppInfoOutput(raw)
	if err != nil {
		t.Fatal(err)
	}

	redists := ExtractCommonRedists(parsed[123], parsed[228980])
	if len(redists) != 2 {
		t.Fatalf("redists = %#v, want 2", redists)
	}
	if redists[0].DepotID != "228989" || redists[0].Name != "VC++ 2022 Redist" {
		t.Fatalf("first redist = %#v", redists[0])
	}
	if redists[1].DepotID != "228990" || redists[1].Name != "DirectX Jun 2010 Redist" {
		t.Fatalf("second redist = %#v", redists[1])
	}
}

func TestResolveCommonRedistsAddsVerbs(t *testing.T) {
	provider := staticAppInfoProvider{
		123: {
			"depots": map[string]any{
				"228989": map[string]any{"depotfromapp": "228980"},
				"229007": map[string]any{"depotfromapp": "228980"},
			},
		},
		228980: {
			"depots": map[string]any{
				"228989": map[string]any{"name": "Microsoft Visual C++ 2022 Redistributable"},
				"229007": map[string]any{"name": "Microsoft .NET Framework 4.8 Redist"},
			},
		},
	}

	redists, err := NewRedistResolverWithProvider(provider).Resolve(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	verbs := WinetricksVerbs(redists)
	want := []string{"vcrun2022", "dotnet48"}
	if !reflect.DeepEqual(verbs, want) {
		t.Fatalf("verbs = %#v, want %#v", verbs, want)
	}
}

func TestExtractCommonRedistsUsesDepotIDVerbs(t *testing.T) {
	app := map[string]any{
		"depots": map[string]any{
			"228989": map[string]any{"depotfromapp": "228980", "sharedinstall": "1"},
			"228990": map[string]any{"depotfromapp": "228980", "sharedinstall": "1"},
			"229007": map[string]any{"depotfromapp": "228980", "sharedinstall": "1"},
		},
	}
	common := map[string]any{
		"depots": map[string]any{
			"228989": map[string]any{},
			"228990": map[string]any{},
			"229007": map[string]any{},
		},
	}

	redists := ExtractCommonRedists(app, common)
	verbs := WinetricksVerbs(redists)
	want := []string{"vcrun2022", "d3dx9", "d3dcompiler_43", "xact", "dotnet48"}
	if !reflect.DeepEqual(verbs, want) {
		t.Fatalf("verbs = %#v, want %#v", verbs, want)
	}
}

func TestVerbsForRedistPrefersDepotIDMapping(t *testing.T) {
	tests := map[string]struct {
		redist SteamRedist
		want   []string
	}{
		"vc2022": {
			redist: SteamRedist{DepotID: "228989", Name: "unexpected display name"},
			want:   []string{"vcrun2022"},
		},
		"directx": {
			redist: SteamRedist{DepotID: "228990"},
			want:   []string{"d3dx9", "d3dcompiler_43", "xact"},
		},
		"dotnet35": {
			redist: SteamRedist{DepotID: "229000"},
			want:   []string{"dotnet35"},
		},
		"xna30": {
			redist: SteamRedist{DepotID: "229010"},
			want:   nil,
		},
		"fallback": {
			redist: SteamRedist{DepotID: "999999", Name: "OpenAL Redist"},
			want:   []string{"openal"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := VerbsForRedist(test.redist); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("VerbsForRedist(%#v) = %#v, want %#v", test.redist, got, test.want)
			}
		})
	}
}

func TestSteamCMDNetProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/info/2582320" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"2582320":{"appid":"2582320","depots":{"228989":{"depotfromapp":"228980","sharedinstall":"1"}}}},"status":"success"}`)) //nolint:errcheck
	}))
	defer server.Close()

	provider := &SteamCMDNetProvider{baseURL: server.URL + "/v1/info/", client: server.Client()}
	app, err := provider.AppInfo(context.Background(), 2582320)
	if err != nil {
		t.Fatal(err)
	}
	if app["appid"] != "2582320" {
		t.Fatalf("appid = %#v", app["appid"])
	}
}

func TestVerbsForRedistName(t *testing.T) {
	tests := map[string][]string{
		"VC++ 2022 Redist":                          {"vcrun2022"},
		"Microsoft Visual C++ 2022 Redistributable": {"vcrun2022"},
		"Microsoft Visual C++ 2015 Redistributable": {"vcrun2019"},
		"Microsoft Visual C++ 2013 Redistributable": {"vcrun2013"},
		"DirectX Jun 2010 Redist":                   {"d3dx9", "d3dcompiler_43", "xact"},
		"Microsoft .NET Framework 4.8":              {"dotnet48"},
		"OpenAL Redist":                             {"openal"},
		"Microsoft XNA Framework 3.0":               nil,
		"Microsoft XNA Framework 4.0":               {"xna40"},
		"NVIDIA PhysX Legacy":                       {"physx"},
		"Totally Custom Middleware":                 nil,
	}

	for name, want := range tests {
		if got := VerbsForRedistName(name); !reflect.DeepEqual(got, want) {
			t.Fatalf("VerbsForRedistName(%q) = %#v, want %#v", name, got, want)
		}
	}
}
