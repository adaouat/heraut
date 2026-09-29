package semver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, s string) semver.Version {
	t.Helper()
	v, err := semver.Parse(s)
	require.NoError(t, err, "parsing %q", s)
	return v
}

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		in   string
		want semver.Version
	}{
		{"0.0.0", semver.Version{}},
		{"1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}},
		{"1.0.0-alpha", semver.Version{Major: 1, Pre: []string{"alpha"}}},
		{"1.0.0-alpha.1", semver.Version{Major: 1, Pre: []string{"alpha", "1"}}},
		{"1.0.0-0.3.7", semver.Version{Major: 1, Pre: []string{"0", "3", "7"}}},
		{"1.0.0-x-y-z.--", semver.Version{Major: 1, Pre: []string{"x-y-z", "--"}}},
		{"1.0.0-alpha+001", semver.Version{Major: 1, Pre: []string{"alpha"}, Build: []string{"001"}}},
		{"1.0.0+20130313144700", semver.Version{Major: 1, Build: []string{"20130313144700"}}},
		{"1.0.0-beta+exp.sha.5114f85", semver.Version{Major: 1, Pre: []string{"beta"}, Build: []string{"exp", "sha", "5114f85"}}},
		{"1.0.0+21AF26D3----117B344092BD", semver.Version{Major: 1, Build: []string{"21AF26D3----117B344092BD"}}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := semver.Parse(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.in, got.String(), "String must round-trip")
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, in := range []string{
		"", "1", "1.2", "1.2.3.4", "v1.2.3", " 1.2.3", "-1.2.3",
		"01.2.3", "1.02.3", "1.2.03", // leading zeros in core
		"1.2.3-", "1.2.3+", "1.2.3-rc.1+", // empty pre-release / build
		"1.2.3-01", "1.2.3-rc.01", // leading zero in numeric pre-release identifier
		"1.2.3-a..b", "1.2.3+a..b", // empty identifier
		"1.2.3-a_b", "1.2.3+a+b", "1.2.3-ä", // illegal characters
		"99999999999999999999.0.0", // overflows uint64
	} {
		t.Run(in, func(t *testing.T) {
			_, err := semver.Parse(in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, semver.ErrInvalidVersion), "want ErrInvalidVersion, got %v", err)
		})
	}
}

func TestVersion_CoreAndIsPreRelease(t *testing.T) {
	v := mustParse(t, "1.4.0-rc.2+158404")
	assert.Equal(t, "1.4.0", v.Core())
	assert.True(t, v.IsPreRelease())
	assert.False(t, mustParse(t, "1.4.0+158404").IsPreRelease(), "build metadata alone is a release")
}

// TestCompare_SpecChain is the SemVer 2.0.0 §11 example chain, plus heraut's hard-won rows.
func TestCompare_SpecChain(t *testing.T) {
	chain := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		"1.9.0", "1.10.0", "2.0.0", "2.1.0", "2.1.1",
	}
	for i := 0; i+1 < len(chain); i++ {
		lo, hi := mustParse(t, chain[i]), mustParse(t, chain[i+1])
		assert.Equal(t, -1, semver.Compare(lo, hi), "%s < %s", chain[i], chain[i+1])
		assert.Equal(t, 1, semver.Compare(hi, lo), "%s > %s", chain[i+1], chain[i])
	}
}

func TestCompare_Cases(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"build metadata ignored", "1.0.0+1", "1.0.0+2", 0},
		{"build metadata ignored on pre-release", "1.0.0-rc.1+a", "1.0.0-rc.1", 0},
		{"numeric identifier below alphanumeric", "1.0.0-1", "1.0.0-a", -1},
		{"numeric identifiers compare numerically", "1.0.0-rc.2", "1.0.0-rc.11", -1},
		{"ASCII order: dev above alpha", "1.0.0-dev.1", "1.0.0-alpha.1", 1},
		{"longer identifier list wins", "1.0.0-rc", "1.0.0-rc.1", -1},
		{"oversized numeric identifiers do not overflow", "1.0.0-rc.99999999999999999999", "1.0.0-rc.9999", 1},
		{"equal", "1.2.3", "1.2.3", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, semver.Compare(mustParse(t, tc.a), mustParse(t, tc.b)))
		})
	}
}

func stripV(tag string) (string, bool) { return strings.CutPrefix(tag, "v") }

func TestSortTags_OrdersByPrecedenceAndDropsInvalid(t *testing.T) {
	got := semver.SortTags([]string{"v1.2.3", "v1.10.0", "notatag", "v1.02.0", "v1.10.0-rc.1", "x1.0.0"}, stripV)
	tags := make([]string, len(got))
	for i, tv := range got {
		tags[i] = tv.Tag
	}
	assert.Equal(t, []string{"v1.10.0", "v1.10.0-rc.1", "v1.2.3"}, tags)
}

func TestSortTags_TiesKeepInputOrder(t *testing.T) {
	got := semver.SortTags([]string{"v1.4.0+6", "v1.4.0+5"}, stripV)
	require.Len(t, got, 2)
	assert.Equal(t, "v1.4.0+6", got[0].Tag)
	assert.Equal(t, "v1.4.0+5", got[1].Tag)
}

func TestLatest(t *testing.T) {
	sorted := semver.SortTags([]string{"v1.4.0-rc.1", "v1.3.0"}, stripV)

	final, ok := semver.Latest(sorted, false)
	require.True(t, ok)
	assert.Equal(t, "v1.3.0", final.Tag)

	highest, ok := semver.Latest(sorted, true)
	require.True(t, ok)
	assert.Equal(t, "v1.4.0-rc.1", highest.Tag)

	_, ok = semver.Latest(semver.SortTags([]string{"v1.0.0-rc.1"}, stripV), false)
	assert.False(t, ok, "only pre-releases → no final")

	_, ok = semver.Latest(nil, true)
	assert.False(t, ok)
}
