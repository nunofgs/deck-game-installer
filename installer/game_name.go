package installer

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	bracketedText  = regexp.MustCompile(`\[[^\[\]]*\]`)
	filesSuffix    = regexp.MustCompile(`(?i)[_-]files$`)
	nameSeparators = strings.NewReplacer("_", " ", ".", " ", "-", " ")
)

func cleanGameName(name string) string {
	for bracketedText.MatchString(name) {
		name = bracketedText.ReplaceAllString(name, "")
	}

	name = filesSuffix.ReplaceAllString(strings.TrimSpace(name), "")
	words := strings.Fields(strings.ToLower(nameSeparators.Replace(name)))
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}

	return strings.Join(words, " ")
}

// DeriveGameName tries the parent directory name before the executable filename.
func DeriveGameName(path string) string {
	genericNames := map[string]bool{
		"disc1": true, "disc2": true, "disk1": true, "disk2": true,
		"cd1": true, "cd2": true, "dvd1": true, "dvd2": true,
		"setup": true, "install": true, "installer": true,
		"": true, ".": true, "/": true, "home": true, "downloads": true,
	}

	for _, name := range []string{
		cleanGameName(filepath.Base(filepath.Dir(path))),
		gameNameFromPath(path),
	} {
		if genericNames[strings.ToLower(name)] {
			continue
		}

		return name
	}

	return ""
}

func gameNameFromPath(path string) string {
	return cleanGameName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}
