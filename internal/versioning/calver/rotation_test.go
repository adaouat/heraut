package calver_test

import (
	"regexp"
	"testing"

	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseFormat(t *testing.T, format string) []calver.Token {
	t.Helper()
	tokens, err := calver.ParseFormat(format)
	require.NoError(t, err)
	return tokens
}

func TestValidateRotationTokens_ValidPrefix(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	assert.NoError(t, calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindYYYY}))
	// Order of the requested slice must not matter — it's a set.
	assert.NoError(t, calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindMM, calver.KindYYYY}))
}

func TestValidateRotationTokens_NotAPrefix_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	err := calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindMM})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix")
}

func TestValidateRotationTokens_TokenNotInFormat_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	err := calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindYYYY, calver.KindQQ})
	require.Error(t, err)
}

func TestValidateRotationTokens_Empty_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	err := calver.ValidateRotationTokens(tokens, nil)
	require.Error(t, err)
}

func TestValidateRotationTokens_Duplicate_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	err := calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindYYYY, calver.KindYYYY})
	require.Error(t, err)
}

func TestValidateRotationTokens_MoreThanAvailable_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.PATCH")

	err := calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindYYYY, calver.KindMM})
	require.Error(t, err)
}

// TestValidateRotationTokens_AmbiguousBoundary_Error covers a format where the rotation boundary
// token isn't followed by a literal separator (e.g. "YYYYMM.PATCH" — YYYY runs directly into MM
// with no separating character). Rendering a bucket-scoping regex from such a boundary could
// partial-match into the next token's digits, so this is rejected at validation time rather than
// risking a silently-wrong tag_pattern.
func TestValidateRotationTokens_AmbiguousBoundary_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYYMM.PATCH")

	err := calver.ValidateRotationTokens(tokens, []calver.TokenKind{calver.KindYYYY})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "separator")
}

func TestBucketKey_SameBucket_IgnoresTrailingTokens(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	k1, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2026, Month: 5, Patch: 2})
	require.NoError(t, err)
	k2, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2026, Month: 11, Patch: 0})
	require.NoError(t, err)

	assert.Equal(t, k1, k2, "same year must be the same bucket regardless of month/patch")
}

func TestBucketKey_DifferentYear_DifferentBucket(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	k1, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2026, Month: 5})
	require.NoError(t, err)
	k2, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2027, Month: 5})
	require.NoError(t, err)

	assert.NotEqual(t, k1, k2)
}

func TestBucketKey_MultiToken_OrderIsFormatOrderNotRequestedOrder(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	// Requested given in reverse order — the key must still render in format order (YYYY then MM),
	// so two callers who spell the same rotation differently agree on the bucket.
	k1, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindYYYY, calver.KindMM}, calver.Values{Year: 2026, Month: 5})
	require.NoError(t, err)
	k2, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindMM, calver.KindYYYY}, calver.Values{Year: 2026, Month: 5})
	require.NoError(t, err)

	assert.Equal(t, k1, k2)
}

func TestBucketKey_InvalidTokens_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	_, err := calver.BucketKey(tokens, []calver.TokenKind{calver.KindMM}, calver.Values{Year: 2026, Month: 5})
	require.Error(t, err)
}

func TestBucketPattern_SingleToken(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	pattern, err := calver.BucketPattern(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2026})
	require.NoError(t, err)

	re := regexp.MustCompile(pattern)
	assert.True(t, re.MatchString("2026.05.3"))
	assert.False(t, re.MatchString("2027.01.0"))
	// A malformed tag whose year digits merely start with "2026" must not partial-match — the
	// boundary requires the format's own literal separator ("." here) right after the year.
	assert.False(t, re.MatchString("20260.5.1"))
}

func TestBucketPattern_MultiToken(t *testing.T) {
	tokens := mustParseFormat(t, "YYYY.MM.PATCH")

	pattern, err := calver.BucketPattern(tokens, []calver.TokenKind{calver.KindYYYY, calver.KindMM}, calver.Values{Year: 2026, Month: 5})
	require.NoError(t, err)

	re := regexp.MustCompile(pattern)
	assert.True(t, re.MatchString("2026.05.9"))
	assert.False(t, re.MatchString("2026.11.0"))
}

func TestBucketPattern_InvalidTokens_Error(t *testing.T) {
	tokens := mustParseFormat(t, "YYYYMM.PATCH")

	_, err := calver.BucketPattern(tokens, []calver.TokenKind{calver.KindYYYY}, calver.Values{Year: 2026})
	require.Error(t, err)
}
