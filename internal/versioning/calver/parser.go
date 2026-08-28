package calver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TokenKind identifies a CalVer format token.
type TokenKind int

const (
	KindLiteral TokenKind = iota
	KindYYYY
	KindMM
	KindDD
	KindWW
	KindQQ
	KindSS
	KindSPRINT
	KindPATCH
)

// String returns the format-token spelling for k (e.g. "YYYY"), for use in error messages.
func (k TokenKind) String() string {
	for _, kt := range knownTokens {
		if kt.kind == k {
			return kt.text
		}
	}
	return "LITERAL"
}

// Token is one component of a parsed CalVer format string.
type Token struct {
	Kind    TokenKind
	Literal string // non-empty only when Kind == KindLiteral
}

// Values holds the numeric components extracted from a CalVer version string.
type Values struct {
	Year, Month, Day, Week, Quarter, Semester, Sprint, Patch int
}

// knownTokens ordered longest-first to avoid prefix conflicts (SPRINT before SS).
var knownTokens = []struct {
	text string
	kind TokenKind
}{
	{"SPRINT", KindSPRINT},
	{"PATCH", KindPATCH},
	{"YYYY", KindYYYY},
	{"MM", KindMM},
	{"DD", KindDD},
	{"WW", KindWW},
	{"QQ", KindQQ},
	{"SS", KindSS},
}

// ParseFormat parses a CalVer format string (e.g. "YYYY.MM.PATCH") into tokens.
// Returns an error if PATCH is absent or not the last non-literal token.
func ParseFormat(format string) ([]Token, error) {
	if format == "" {
		return nil, fmt.Errorf("calver format must not be empty")
	}

	var tokens []Token
	rem := format
	for rem != "" {
		matched := false
		for _, kt := range knownTokens {
			if strings.HasPrefix(rem, kt.text) {
				tokens = append(tokens, Token{Kind: kt.kind})
				rem = rem[len(kt.text):]
				matched = true
				break
			}
		}
		if !matched {
			// Consume characters until the next known token or end.
			end := 1
			for end < len(rem) {
				isKnown := false
				for _, kt := range knownTokens {
					if strings.HasPrefix(rem[end:], kt.text) {
						isKnown = true
						break
					}
				}
				if isKnown {
					break
				}
				end++
			}
			tokens = append(tokens, Token{Kind: KindLiteral, Literal: rem[:end]})
			rem = rem[end:]
		}
	}

	// Validate: PATCH must be present and must be the last non-literal token.
	patchFound := false
	lastNonLiteral := KindLiteral
	for _, t := range tokens {
		if t.Kind != KindLiteral {
			lastNonLiteral = t.Kind
			if t.Kind == KindPATCH {
				patchFound = true
			}
		}
	}
	if !patchFound {
		return nil, fmt.Errorf("calver format %q must contain PATCH token", format)
	}
	if lastNonLiteral != KindPATCH {
		return nil, fmt.Errorf("PATCH must be the last token in calver format %q", format)
	}

	return tokens, nil
}

// ParseVersion extracts values from a version string using the given token sequence.
func ParseVersion(tokens []Token, version string) (Values, error) {
	re, err := buildParseRegex(tokens)
	if err != nil {
		return Values{}, fmt.Errorf("building parse regex: %w", err)
	}

	match := re.FindStringSubmatch(version)
	if match == nil {
		return Values{}, fmt.Errorf("version %q does not match calver format", version)
	}

	var v Values
	for name, idx := range namedGroups(re) {
		n, _ := strconv.Atoi(match[idx])
		switch name {
		case "YYYY":
			v.Year = n
		case "MM":
			v.Month = n
		case "DD":
			v.Day = n
		case "WW":
			v.Week = n
		case "QQ":
			v.Quarter = n
		case "SS":
			v.Semester = n
		case "SPRINT":
			v.Sprint = n
		case "PATCH":
			v.Patch = n
		}
	}
	return v, nil
}

// RenderVersion formats a version string from tokens and values.
func RenderVersion(tokens []Token, v Values) string {
	var sb strings.Builder
	for _, t := range tokens {
		if t.Kind == KindLiteral {
			sb.WriteString(t.Literal)
			continue
		}
		sb.WriteString(renderToken(t.Kind, v))
	}
	return sb.String()
}

// renderToken formats the value v holds for a single non-literal token kind. Shared by
// RenderVersion (whole-version rendering) and the rotation helpers in rotation.go (partial,
// bucket-prefix rendering) so the two never drift on padding (e.g. "%04d" for YYYY).
func renderToken(kind TokenKind, v Values) string {
	switch kind {
	case KindYYYY:
		return fmt.Sprintf("%04d", v.Year)
	case KindMM:
		return fmt.Sprintf("%02d", v.Month)
	case KindDD:
		return fmt.Sprintf("%02d", v.Day)
	case KindWW:
		return fmt.Sprintf("%02d", v.Week)
	case KindQQ:
		return fmt.Sprintf("%d", v.Quarter)
	case KindSS:
		return fmt.Sprintf("%d", v.Semester)
	case KindSPRINT:
		return fmt.Sprintf("%d", v.Sprint)
	case KindPATCH:
		return fmt.Sprintf("%d", v.Patch)
	default:
		return ""
	}
}

func buildParseRegex(tokens []Token) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for _, t := range tokens {
		switch t.Kind {
		case KindLiteral:
			sb.WriteString(regexp.QuoteMeta(t.Literal))
		case KindYYYY:
			sb.WriteString(`(?P<YYYY>\d{4})`)
		case KindMM:
			sb.WriteString(`(?P<MM>\d{1,2})`)
		case KindDD:
			sb.WriteString(`(?P<DD>\d{1,2})`)
		case KindWW:
			sb.WriteString(`(?P<WW>\d{1,2})`)
		case KindQQ:
			sb.WriteString(`(?P<QQ>\d)`)
		case KindSS:
			sb.WriteString(`(?P<SS>\d)`)
		case KindSPRINT:
			sb.WriteString(`(?P<SPRINT>\d+)`)
		case KindPATCH:
			sb.WriteString(`(?P<PATCH>\d+)`)
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

func namedGroups(re *regexp.Regexp) map[string]int {
	result := make(map[string]int)
	for i, name := range re.SubexpNames() {
		if name != "" {
			result[name] = i
		}
	}
	return result
}
