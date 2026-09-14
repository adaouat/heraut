package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
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

	r, err := buildRelease("v1.2.3", "v1.2.2", fixedDate1, time.Time{}, groups, githubLC, nil, 3, enrichment, contribs, tplHeraut{Version: "0.48.0"}, nil)
	require.NoError(t, err)

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

func TestResolveFooterLine(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		value    string
		rules    map[string]config.FooterRule
		wantLine string
		wantOK   bool
		wantErr  bool
	}{
		{
			name: "no rule uses the built-in Token: Value format", token: "Refs", value: "#1",
			wantLine: "Refs: #1", wantOK: true,
		},
		{
			name:  "hide drops the footer",
			token: "Refs", value: "#1",
			rules:  map[string]config.FooterRule{"refs": {Token: "Refs", Hide: true}},
			wantOK: false,
		},
		{
			name:  "renderer formats the footer",
			token: "Co-authored-by", value: "Jane <jane@x.com>",
			rules:    map[string]config.FooterRule{"co-authored-by": {Token: "Co-authored-by", Renderer: "**{{ .Value }}**"}},
			wantLine: "**Jane <jane@x.com>**", wantOK: true,
		},
		{
			name:  "token match is case-insensitive",
			token: "REFS", value: "#1",
			rules:  map[string]config.FooterRule{"refs": {Token: "Refs", Hide: true}},
			wantOK: false,
		},
		{
			name:  "a bad renderer surfaces an execution error",
			token: "Refs", value: "#1",
			rules:   map[string]config.FooterRule{"refs": {Token: "Refs", Renderer: "{{ .NoSuchField }}"}},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line, ok, err := resolveFooterLine(tc.token, tc.value, tc.rules)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantLine, line)
			}
		})
	}
}

func TestBuildCommit_ResolvesFooterLines(t *testing.T) {
	pc := parsedCommit{raw: rawCommit{Hash: "abc1234def", Subject: "feat: add x", Date: fixedDate1}}
	var err error
	pc.parsed, err = conventionalcommit.Parse("feat: add x\n\nCo-authored-by: Jane <jane@x.com>\nRefs: #1")
	require.NoError(t, err)

	rules := map[string]config.FooterRule{
		"co-authored-by": {Token: "Co-authored-by", Renderer: "**{{ .Value }}**"},
		"refs":           {Token: "Refs", Hide: true},
	}

	c, err := buildCommit(pc, "", nil, nil, rules)
	require.NoError(t, err)
	require.Len(t, c.Footers, 1, "the hidden Refs footer is dropped")
	assert.Equal(t, "Co-authored-by", c.Footers[0].Token)
	assert.Equal(t, "**Jane <jane@x.com>**", c.Footers[0].Line)
}

func TestBuildCommit_DefaultFooterLineUnchangedWhenNoRulesConfigured(t *testing.T) {
	pc := parsedCommit{raw: rawCommit{Hash: "abc1234def", Subject: "feat: add x", Date: fixedDate1}}
	var err error
	pc.parsed, err = conventionalcommit.Parse("feat: add x\n\nRefs: #1")
	require.NoError(t, err)

	c, err := buildCommit(pc, "", nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, c.Footers, 1)
	assert.Equal(t, "Refs: #1", c.Footers[0].Line, "no rules configured keeps today's default rendering")
}
