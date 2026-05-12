package redist

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallRedistsUsesProtontricksAndCreatesPrefix(t *testing.T) {
	var calls []string
	installer := NewInstallerForTest(
		func(name string) (string, error) {
			if name == "protontricks" {
				return "/bin/protontricks", nil
			}
			return "", errors.New("not found")
		},
		func(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
			calls = append(calls, name+" "+joinArgs(args))
			return nil, nil
		},
	)

	err := installer.InstallRedists(t.Context(), int32(-1), filepath.Join(t.TempDir(), "pfx"), "", []string{"vcrun2022", "d3dx9"}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"/bin/protontricks 4294967295 wineboot",
		"/bin/protontricks 4294967295 vcrun2022 d3dx9",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestInstallRedistsFallsBackToWinetricksForExistingPrefix(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "pfx")
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	var gotEnv []string
	var gotArgs []string
	installer := NewInstallerForTest(
		func(name string) (string, error) {
			if name == "winetricks" {
				return "/bin/winetricks", nil
			}
			return "", errors.New("not found")
		},
		func(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
			gotArgs = args
			gotEnv = env
			return nil, nil
		},
	)

	err := installer.InstallRedists(t.Context(), 42, prefix, "", []string{"vcrun2022"}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotArgs, []string{"-q", "vcrun2022"}) {
		t.Fatalf("args = %#v", gotArgs)
	}
	if !reflect.DeepEqual(gotEnv, []string{"WINEPREFIX=" + prefix}) {
		t.Fatalf("env = %#v", gotEnv)
	}
}

func TestInstallRedistsReportsMissingTools(t *testing.T) {
	installer := NewInstallerForTest(
		func(name string) (string, error) { return "", errors.New("not found") },
		func(ctx context.Context, name string, args []string, env []string) ([]byte, error) { return nil, nil },
	)

	err := installer.InstallRedists(t.Context(), 42, filepath.Join(t.TempDir(), "pfx"), "", []string{"vcrun2022"}, func(string) {})
	if !IsMissingTool(err) {
		t.Fatalf("err = %v, want MissingToolError", err)
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}
