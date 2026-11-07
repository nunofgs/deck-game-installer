package installer

import "testing"

func TestGameNamesOmitBracketedText(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{"Hollow Knight [test]", "Hollow Knight"},
		{"[test] Hollow Knight", "Hollow Knight"},
		{"Hollow [test] Knight [build 2]", "Hollow Knight"},
		{"Hollow Knight []", "Hollow Knight"},
		{"Hollow Knight [test [build 2]]", "Hollow Knight"},
		{"Hollow Knight", "Hollow Knight"},
		{"Hollow Knight [test", "Hollow Knight [test"},
		{"[test]", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := cleanGameName(test.name); got != test.want {
				t.Errorf("manual name = %q, want %q", got, test.want)
			}

			if got := gameNameFromPath("/games/" + test.name + ".iso"); got != test.want {
				t.Errorf("ISO name = %q, want %q", got, test.want)
			}

			if test.want == "" {
				return
			}

			if got := DeriveGameName("/games/" + test.name + "/setup.exe"); got != test.want {
				t.Errorf("directory name = %q, want %q", got, test.want)
			}

			if got := DeriveGameName("/downloads/" + test.name + ".exe"); got != test.want {
				t.Errorf("filename = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDeriveGameNameSkipsBracketedGenericDirectory(t *testing.T) {
	if got := DeriveGameName("/downloads [test]/Hollow Knight [test].exe"); got != "Hollow Knight" {
		t.Errorf("name = %q, want Hollow Knight", got)
	}
}

func TestCleanGameName(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{"hOLLOW_kNIGHT_files [test]", "Hollow Knight"},
		{"Hollow-Knight-files [test]", "Hollow Knight"},
		{"Hollow.Knight_FILES", "Hollow Knight"},
		{"  Hollow  [test]  Knight  ", "Hollow Knight"},
		{"Hollow Knight Files", "Hollow Knight Files"},
		{"Hollow Knight_files_extra", "Hollow Knight Files Extra"},
		{"_-.[test]", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := cleanGameName(test.name); got != test.want {
				t.Errorf("name = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDeriveGameNameFallbacks(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"/downloads [test]/Hollow_Knight_files [test].exe", "Hollow Knight"},
		{"/Hollow-Knight-files [test]/setup.exe", "Hollow Knight"},
		{"/[test]/Hollow Knight.exe", "Hollow Knight"},
		{"/downloads/setup [test].exe", ""},
		{"/_-.[test]/setup.exe", ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			if got := DeriveGameName(test.path); got != test.want {
				t.Errorf("name = %q, want %q", got, test.want)
			}
		})
	}
}
