package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsesNative(t *testing.T) {
	tests := []struct {
		name    string
		drivers []*config.ContentDriver
		want    bool
	}{
		{name: "no drivers", drivers: nil, want: false},
		{name: "nil driver only", drivers: []*config.ContentDriver{nil}, want: false},
		{name: "git-cliff only", drivers: []*config.ContentDriver{{Generator: "git-cliff"}}, want: false},
		{name: "native changelog", drivers: []*config.ContentDriver{{Generator: "native"}, nil}, want: true},
		{name: "native notes among others", drivers: []*config.ContentDriver{{Generator: "git-cliff"}, {Generator: "native"}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, usesNative(tc.drivers...))
		})
	}
}

func TestGitOriginURL(t *testing.T) {
	t.Run("returns trimmed origin", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.example.com/group/project.git\n", "", nil)
		assert.Equal(t, "https://gitlab.example.com/group/project.git", gitOriginURL(mr))
	})
	t.Run("no origin configured returns empty", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued → Run errors
		assert.Empty(t, gitOriginURL(mr))
	})
}

func TestResolveEnrichForgeIfNeeded(t *testing.T) {
	t.Run("skips resolution when no driver is native", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued: a git call here would error
		cfg := &config.Config{}
		f, id, err := resolveEnrichForgeIfNeeded(mr, cfg, &config.ContentDriver{Generator: "git-cliff"})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.Empty(t, mr.Calls, "no git call when no driver is native")
	})

	t.Run("resolves and constructs a gitlab forge for a native driver", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)
		cfg := &config.Config{}
		f, id, err := resolveEnrichForgeIfNeeded(mr, cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "gitlab", id.Type)
		require.NotNil(t, f, "a concrete gitlab.Forge must be constructed and assigned through the interface")
		assert.Equal(t, "gitlab", f.Type())
	})

	t.Run("non-gitlab resolved forge yields no enrich forge but keeps the identity", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		cfg := &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget"}}}
		f, id, err := resolveEnrichForgeIfNeeded(mr, cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "github", id.Type)
		assert.Nil(t, f, "no port.Forge implementation exists yet for github")
	})

	t.Run("ambiguous forge propagates as an error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		t.Setenv("GITLAB_TOKEN", "glpat")
		t.Setenv("GITHUB_TOKEN", "ghp")
		cfg := &config.Config{}
		_, _, err := resolveEnrichForgeIfNeeded(mr, cfg, &config.ContentDriver{Generator: "native"})
		require.Error(t, err)
	})
}

// assertNoOriginErr simulates `git remote get-url origin` failing (no origin configured).
var assertNoOriginErr = assertError("exit status 1")

type assertError string

func (e assertError) Error() string { return string(e) }
