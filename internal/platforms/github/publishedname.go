package github

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// letters GitHub transliterates that have no canonical decomposition, so norm.NFD cannot strip them.
var transliterated = map[rune]string{
	'ß': "ss",
	'ø': "o", 'Ø': "O",
	'æ': "ae", 'Æ': "AE",
	'ł': "l", 'Ł': "L",
}

// publishedName is the name GitHub publishes an uploaded asset under, as observed on github.com
// (T438): accents are dropped, the letters in transliterated are spelled out, every character other
// than an ASCII letter, a digit or one of "._-+@" becomes ".", runs of "." collapse into one, a
// trailing "." is dropped and a leading "." gets "default" in front. GitHub documents only that it
// renames such names, not how, so this is a model of what it did; it is used to refuse two assets
// that would collide, not to predict the published name for the user.
func publishedName(name string) string {
	var b strings.Builder
	lastDot := false
	write := func(s string) {
		for _, r := range s {
			if r == '.' {
				if lastDot {
					continue
				}
				lastDot = true
			} else {
				lastDot = false
			}
			b.WriteRune(r)
		}
	}
	for _, r := range norm.NFD.String(name) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-+@", r)):
			write(string(r))
		case transliterated[r] != "":
			write(transliterated[r])
		default:
			write(".")
		}
	}
	out := strings.TrimSuffix(b.String(), ".")
	if out == "" || strings.HasPrefix(out, ".") {
		out = "default" + out
	}
	return out
}
