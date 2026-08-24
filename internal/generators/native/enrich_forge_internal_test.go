package native

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDriver() *config.ContentDriver { return &config.ContentDriver{} }

// stubForge records the commits it was handed and returns canned enrichment.
type stubForge struct {
	got    []port.Commit
	gotRef string
	en     port.Enrichment
	err    error
}

func (s *stubForge) Type() string                 { return "gitlab" }
func (s *stubForge) Identity() port.ForgeIdentity { return port.ForgeIdentity{Type: "gitlab"} }
func (s *stubForge) CommitURL(sha string) string {
	return "https://gitlab.example.com/g/p/-/commit/" + sha
}
func (s *stubForge) ChangeURL(int) string             { return "" }
func (s *stubForge) CompareURL(string, string) string { return "" }
func (s *stubForge) Enrich(c []port.Commit, ref string) (port.Enrichment, error) {
	s.got = c
	s.gotRef = ref
	return s.en, s.err
}

func TestEnrich_PrefersInjectedForge(t *testing.T) {
	sf := &stubForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 42, RefPrefix: "!"}},
		Authors: map[string]string{"abc": "alice"},
	}}
	g := New(nil, testDriver(), ModeChangelog, WithForge(sf))

	er, err := g.enrich([]rawCommit{{Hash: "abc", Author: "Alice", Email: "alice@example.com"}}, "v1.0.0")
	require.NoError(t, err)

	require.Len(t, sf.got, 1, "the forge receives the collected commits")
	assert.Equal(t, "abc", sf.got[0].Hash)
	assert.Equal(t, "Alice", sf.got[0].Author)
	assert.Equal(t, "v1.0.0", sf.gotRef, "a non-HEAD ref reaches the forge unchanged")
	assert.Equal(t, "alice", er.authors["abc"])
	assert.Equal(t, 42, er.prs["abc"].Number)
	assert.Equal(t, "!", er.prs["abc"].RefPrefix)
}

func TestEnrich_ForgeErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	g := New(nil, testDriver(), ModeChangelog, WithForge(&stubForge{err: sentinel}))
	_, err := g.enrich([]rawCommit{{Hash: "abc"}}, "v1.0.0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel))
}

// Without an injected forge, enrichment yields nothing (no transport left to fall back to).
func TestEnrich_NoForgeYieldsNoEnrichment(t *testing.T) {
	g := New(nil, testDriver(), ModeChangelog)
	er, err := g.enrich([]rawCommit{{Hash: "abc"}}, "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, er.prs)
}

// An injected forge satisfies remote_metadata: required on its own — it carries its own identity,
// so nothing else needs to be configured for the "nothing configured" case to not apply.
func TestEnrichForRelease_RequiredSatisfiedByInjectedForge(t *testing.T) {
	sf := &stubForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 7, RefPrefix: "!"}},
		Authors: map[string]string{"abc": "alice"},
	}}
	driver := testDriver()
	driver.RemoteMetadata = "required"
	g := New(nil, driver, ModeChangelog, WithForge(sf))

	er, err := g.enrichForRelease([]rawCommit{{Hash: "abc", Author: "Alice"}}, "v1.0.0")
	require.NoError(t, err, "an injected forge must satisfy the required policy")
	assert.Equal(t, 7, er.prs["abc"].Number)
	assert.Equal(t, "alice", er.authors["abc"])
	assert.False(t, g.Degraded())
}

// The required-policy error still fires when there is no forge configured.
func TestEnrichForRelease_RequiredStillErrorsWithoutForge(t *testing.T) {
	driver := testDriver()
	driver.RemoteMetadata = "required"
	g := New(nil, driver, ModeChangelog)

	_, err := g.enrichForRelease([]rawCommit{{Hash: "abc"}}, "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote enrichment (required)")
}

// WithDegraded seeds a degraded state at construction — used by the pipeline (T175) when it could
// not resolve an enrichment forge under a non-required policy, so the run is already marked
// degraded before any commit is generated.
func TestWithDegraded_SeedsDegradedState(t *testing.T) {
	g := New(nil, testDriver(), ModeChangelog, WithDegraded("resolving forge: ambiguous forge"))
	assert.True(t, g.Degraded())
	assert.Equal(t, "resolving forge: ambiguous forge", g.DegradedReason())
}
