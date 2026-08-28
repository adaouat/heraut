package calver

import (
	"fmt"
	"regexp"
	"strings"
)

// nonPatchOrder returns the non-literal, non-PATCH token kinds of tokens, in format order. This is
// the full set of dimensions a CalVer format tracks and that a rotating changelog can group by.
func nonPatchOrder(tokens []Token) []TokenKind {
	var order []TokenKind
	for _, t := range tokens {
		if t.Kind != KindLiteral && t.Kind != KindPATCH {
			order = append(order, t.Kind)
		}
	}
	return order
}

// boundaryIndex returns the index in tokens immediately after the n-th non-literal, non-PATCH
// token (in format order). Used to find what comes right after a rotation boundary.
func boundaryIndex(tokens []Token, n int) int {
	count := 0
	for i, t := range tokens {
		if t.Kind != KindLiteral && t.Kind != KindPATCH {
			count++
			if count == n {
				return i + 1
			}
		}
	}
	return len(tokens)
}

// bucketKey renders a period/version-bucket key from an explicit, ordered token list.
func bucketKey(order []TokenKind, v Values) string {
	parts := make([]string, len(order))
	for i, k := range order {
		parts[i] = renderToken(k, v)
	}
	return strings.Join(parts, "|")
}

// ValidateRotationTokens checks that requested — the set of format tokens a rotating changelog
// output or tag-scope pattern groups by — forms a contiguous, non-empty prefix of tokens' own
// non-PATCH, non-literal token order, immediately followed by a literal separator in the format.
// A changelog can only rotate by dimensions the version format itself already tracks, in the order
// it tracks them, with an unambiguous boundary — so tag-scoping stays derivable from the version
// string alone, with no risk of a partial-digit false match (see
// docs/superpowers/specs/2026-08-28-changelog-rotation-design.md §1, §4).
func ValidateRotationTokens(tokens []Token, requested []TokenKind) error {
	if len(requested) == 0 {
		return fmt.Errorf("rotation token set must not be empty")
	}

	order := nonPatchOrder(tokens)
	if len(requested) > len(order) {
		return fmt.Errorf("rotation tokens %v: format has only %d rotatable token(s) (%v)", tokenNames(requested), len(order), tokenNames(order))
	}

	seen := make(map[TokenKind]bool, len(requested))
	for _, k := range requested {
		if seen[k] {
			return fmt.Errorf("rotation token %s requested more than once", k)
		}
		seen[k] = true
	}

	prefix := order[:len(requested)]
	for _, k := range prefix {
		if !seen[k] {
			return fmt.Errorf("rotation tokens %v are not a prefix of format tokens %v (expected %v)", tokenNames(requested), tokenNames(order), tokenNames(prefix))
		}
	}

	afterIdx := boundaryIndex(tokens, len(requested))
	if afterIdx >= len(tokens) || tokens[afterIdx].Kind != KindLiteral {
		return fmt.Errorf("rotation boundary token %s must be followed by a literal separator in the format", prefix[len(prefix)-1])
	}

	return nil
}

// TokenKindFromName returns the TokenKind for a format-token spelling (e.g. "YYYY"), or false if
// name isn't a recognized token — the inverse of TokenKind.String(). Used by callers (e.g.
// internal/app's changelog-rotation decorator) that parse {TOKEN} placeholders out of a
// user-authored string and need the typed value BucketKey/BucketPattern/ValidateRotationTokens
// expect.
func TokenKindFromName(name string) (TokenKind, bool) {
	for _, kt := range knownTokens {
		if kt.text == name {
			return kt.kind, true
		}
	}
	return 0, false
}

// RenderToken formats the value v holds for a single non-literal token kind — the same
// per-token-kind formatting RenderVersion and the bucket helpers use internally, exported so a
// caller substituting individual {TOKEN} placeholders (rather than rendering a whole version
// string) doesn't need to duplicate the padding rules (e.g. "%04d" for YYYY).
func RenderToken(kind TokenKind, v Values) string {
	return renderToken(kind, v)
}

func tokenNames(kinds []TokenKind) []string {
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = k.String()
	}
	return names
}

// BucketKey returns a key that uniquely identifies which rotation bucket v belongs to, given the
// tokens a rotating changelog output/tag pattern groups by. requested must be a valid, non-empty
// prefix of tokens' own token order (see ValidateRotationTokens, which BucketKey calls internally).
// The key always renders in the format's own token order, regardless of the order requested was
// given in, so two callers who spell the same rotation differently still agree on the bucket.
func BucketKey(tokens []Token, requested []TokenKind, v Values) (string, error) {
	if err := ValidateRotationTokens(tokens, requested); err != nil {
		return "", err
	}
	return bucketKey(nonPatchOrder(tokens)[:len(requested)], v), nil
}

// BucketPattern returns a regular expression matching any bare version string in the same
// rotation bucket as v, given the tokens a rotating changelog output/tag pattern groups by. The
// pattern is anchored at the start (^) only — callers compose their own tag_prefix quoting and
// decide whether to anchor the end. requested must be a valid, non-empty prefix of tokens' own
// token order (see ValidateRotationTokens, which BucketPattern calls internally — this also
// guarantees the token immediately after the rotation boundary is a literal separator, so the
// match can never partial-match into the next token's digits).
func BucketPattern(tokens []Token, requested []TokenKind, v Values) (string, error) {
	if err := ValidateRotationTokens(tokens, requested); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("^")
	remaining := len(requested)
	i := 0
	for ; remaining > 0; i++ {
		t := tokens[i]
		if t.Kind == KindLiteral {
			sb.WriteString(regexp.QuoteMeta(t.Literal))
			continue
		}
		sb.WriteString(renderToken(t.Kind, v))
		remaining--
	}
	// ValidateRotationTokens guarantees tokens[i] is a literal separator here.
	sb.WriteString(regexp.QuoteMeta(tokens[i].Literal))

	return sb.String(), nil
}
