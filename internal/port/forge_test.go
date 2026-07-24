package port_test

import (
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

// fakeForge proves the Forge interface is implementable and the value types compose.
type fakeForge struct{ id port.ForgeIdentity }

func (f fakeForge) Type() string                 { return f.id.Type }
func (f fakeForge) Identity() port.ForgeIdentity { return f.id }
func (f fakeForge) CommitURL(sha string) string  { return f.id.Host + "/-/commit/" + sha }
func (f fakeForge) ChangeURL(n int) string       { return f.id.Host + "/-/merge_requests/" }
func (f fakeForge) CompareURL(from, to string) string {
	return f.id.Host + "/-/compare/" + from + "..." + to
}
func (f fakeForge) Enrich(_ []port.Commit) (port.Enrichment, error) {
	return port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 42, RefPrefix: "!", MergedBy: port.Author{Username: "bob"}}},
		Authors: map[string]string{"abc": "alice"},
	}, nil
}

func TestForge_InterfaceComposes(t *testing.T) {
	var f port.Forge = fakeForge{id: port.ForgeIdentity{
		Type: "gitlab", Host: "https://gitlab.example.com", APIURL: "https://gitlab.example.com/api/v4",
		Project: "group/subgroup/project", Token: "t", TokenKind: port.TokenJob, APIMode: "rest",
	}}
	assert.Equal(t, "gitlab", f.Type())
	assert.Equal(t, port.TokenJob, f.Identity().TokenKind)
	assert.Equal(t, "https://gitlab.example.com/-/commit/deadbeef", f.CommitURL("deadbeef"))

	en, err := f.Enrich([]port.Commit{{Hash: "abc", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	assert.NoError(t, err)
	assert.Equal(t, "alice", en.Authors["abc"])
	assert.Equal(t, "!", en.PRs["abc"].RefPrefix)
}
