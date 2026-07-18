package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRelease_MapsTree(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat(api): add thing")
	pc.raw.Email = "jane@x"
	pc.raw.Author = "Jane"
	pc.raw.Date = fixedDate1
	pc.raw.AuthorHandle = "jane" // commit author, resolved via enrichment overlay
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{pc}}}
	enrichment := map[string]PullRequest{
		"abc1234def": {Number: 42, URL: "https://github.com/o/r/pull/42", AuthorLogin: "jane", RefPrefix: "#", Title: "PR title"},
	}
	contribs := []Contributor{{Author: Author{Name: "Jane", Email: "jane@x", Username: "jane"}, IsFirstTime: true}}

	r := buildRelease("v1.2.3", "v1.2.2", fixedDate1, time.Time{}, groups, githubLC, nil, 3, enrichment, contribs, tplHeraut{Version: "0.48.0"})

	assert.Equal(t, "1.2.3", r.Version)
	assert.Equal(t, "v1.2.3", r.Tag)
	require.Len(t, r.Groups, 1)
	c := r.Groups[0].Commits[0]
	assert.Equal(t, "feat", c.Type)
	assert.Equal(t, "api", c.Scope)
	assert.Equal(t, "Add thing", c.Description) // upper-first, conventional-commit description
	assert.Equal(t, "abc1234", c.ShortHash)
	assert.Equal(t, "jane", c.Author.Username)
	require.NotNil(t, c.PR)
	assert.Equal(t, "#42", c.PR.Ref)
	assert.Equal(t, "PR title", c.PR.Title)
	require.Len(t, r.Contributors, 1)
	assert.Equal(t, 1, r.Stats.CommitCount)
}

func TestTemplateModel_FieldsPresent(t *testing.T) {
	r := tplRelease{
		Version: "1.2.3", Tag: "v1.2.3",
		Groups: []tplGroup{{Name: "Features", Commits: []tplCommit{{
			Description: "x", ShortHash: "abc1234",
			Author:  Author{Username: "jane"},
			PR:      &tplPR{Number: 42, Ref: "#42", Author: Author{Username: "jane"}},
			Footers: []tplFooter{{Token: "Refs", Value: "#1"}},
		}}}},
		Contributors: []tplContributor{{Author: Author{Username: "jane"}}},
		Stats:        tplStats{CommitCount: 1},
		Heraut:       tplHeraut{Version: "0.48.0"},
	}
	assert.Equal(t, "#42", r.Groups[0].Commits[0].PR.Ref)
	assert.Equal(t, "Refs", r.Groups[0].Commits[0].Footers[0].Token)
	assert.Equal(t, 1, r.Stats.CommitCount)
	assert.Equal(t, "0.48.0", r.Heraut.Version)
}
