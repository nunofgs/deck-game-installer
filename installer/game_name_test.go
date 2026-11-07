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

			if got := gameNameFromPath("/games/"+test.name+".iso", ""); got != test.want {
				t.Errorf("ISO name = %q, want %q", got, test.want)
			}

			if test.want == "" {
				return
			}

			if got := DeriveGameName("/games/" + test.name + "/setup.exe"); got != titleCase(test.want) {
				t.Errorf("directory name = %q, want %q", got, test.want)
			}

			if got := DeriveGameName("/downloads/" + test.name + ".exe"); got != titleCase(test.want) {
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
