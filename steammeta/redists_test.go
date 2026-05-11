package steammeta

import (
	"context"
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

func TestVerbsForRedistName(t *testing.T) {
	tests := map[string][]string{
		"VC++ 2022 Redist":                          {"vcrun2022"},
		"Microsoft Visual C++ 2022 Redistributable": {"vcrun2022"},
		"Microsoft Visual C++ 2015 Redistributable": {"vcrun2019"},
		"Microsoft Visual C++ 2013 Redistributable": {"vcrun2013"},
		"DirectX Jun 2010 Redist":                   {"d3dx9", "d3dcompiler_43", "xact"},
		"Microsoft .NET Framework 4.8":              {"dotnet48"},
		"OpenAL Redist":                             {"openal"},
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
