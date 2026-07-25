package native

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDriver() *config.ContentDriver { return &config.ContentDriver{Generator: "native"} }

// stubForge records the commits it was handed and returns canned enrichment.
type stubForge struct {
	got []port.Commit
	en  port.Enrichment
	err error
}

func (s *stubForge) Type() string                 { return "gitlab" }
func (s *stubForge) Identity() port.ForgeIdentity { return port.ForgeIdentity{Type: "gitlab"} }
func (s *stubForge) CommitURL(sha string) string {
	return "https://gitlab.example.com/g/p/-/commit/" + sha
}
func (s *stubForge) ChangeURL(int) string             { return "" }
func (s *stubForge) CompareURL(string, string) string { return "" }
func (s *stubForge) Enrich(c []port.Commit) (port.Enrichment, error) {
	s.got = c
	return s.en, s.err
}

func TestEnrich_PrefersInjectedForge(t *testing.T) {
	sf := &stubForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 42, RefPrefix: "!"}},
		Authors: map[string]string{"abc": "alice"},
	}}
	g := New(nil, testDriver(), ModeChangelog, WithForge(sf))

	er, err := g.enrich(&port.LinkContext{Platform: "gitlab"},
		[]rawCommit{{Hash: "abc", Author: "Alice", Email: "alice@example.com"}})
	require.NoError(t, err)

	require.Len(t, sf.got, 1, "the forge receives the collected commits")
	assert.Equal(t, "abc", sf.got[0].Hash)
	assert.Equal(t, "Alice", sf.got[0].Author)
	assert.Equal(t, "alice", er.authors["abc"])
	assert.Equal(t, 42, er.prs["abc"].Number)
	assert.Equal(t, "!", er.prs["abc"].RefPrefix)
}

func TestEnrich_ForgeErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	g := New(nil, testDriver(), ModeChangelog, WithForge(&stubForge{err: sentinel}))
	_, err := g.enrich(&port.LinkContext{Platform: "gitlab"}, []rawCommit{{Hash: "abc"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel))
}

// Without an injected forge, the legacy per-platform dispatch is unchanged.
func TestEnrich_NoForgeFallsBackToLegacy(t *testing.T) {
	g := New(nil, testDriver(), ModeChangelog)
	er, err := g.enrich(nil, []rawCommit{{Hash: "abc"}})
	require.NoError(t, err)
	assert.Empty(t, er.prs)
}
