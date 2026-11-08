package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
)

// A first release in a repository whose version tags carry another prefix says so, naming the
// tag_prefix that would read them, instead of silently restarting at the initial version.
func TestNewResolver_WarnsWhenVersionTagsUseAnotherPrefix(t *testing.T) {
	bare, v := "", "v"
	tests := []struct {
		name     string
		tag      string
		cfg      config.Versioning
		wantWarn string
	}{
		{name: "bare tags, default v prefix", tag: "1.0.0", cfg: config.Versioning{Strategy: "semver"},
			wantWarn: `no tag matches versioning.tag_prefix "v", but 1 tag looks like a version with the prefix "" (1.0.0): set versioning.tag_prefix: "" to continue from it`},
		{name: "v tags, bare prefix", tag: "v1.0.0", cfg: config.Versioning{Strategy: "semver", TagPrefix: &bare},
			wantWarn: `no tag matches versioning.tag_prefix "", but 1 tag looks like a version with the prefix "v" (v1.0.0): set versioning.tag_prefix: "v" to continue from it`},
		{name: "calver with a v prefix", tag: "v2026.05.0", cfg: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
			wantWarn: `no tag matches versioning.tag_prefix "", but 1 tag looks like a version with the prefix "v" (v2026.05.0): set versioning.tag_prefix: "v" to continue from it`},
		{name: "matching tags", tag: "v2026.05.0", cfg: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH", TagPrefix: &v}},
		{name: "a tag that is no version", tag: "deploy-prod", cfg: config.Versioning{Strategy: "semver"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.RealGitRepo(t, tc.tag)
			cfg := &config.Config{Version: "1", Versioning: tc.cfg}

			r, err := app.NewResolver(cfg, "", false, "", "", execadapter.New(false, false))
			require.NoError(t, err)
			res, err := r.Resolve()
			require.NoError(t, err)

			if tc.wantWarn == "" {
				assert.Empty(t, res.Warnings)
				return
			}
			assert.Contains(t, res.Warnings, tc.wantWarn)
		})
	}
}

// Under CalVer, version current is the highest tag that is a CalVer version of the format, whatever
// other tags the repository holds or how git sorts them.
func TestCurrentTag_CalVerIgnoresOtherTags(t *testing.T) {
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\nlatest\n2026.10.4\n2026.11.0\n", "", nil) // git tag -l * --sort=-version:refname

	tag, err := app.CurrentTag(mr, cfg, "", false)

	require.NoError(t, err)
	assert.Equal(t, "2026.11.0", tag)
}

// With no CalVer tag at all, there is no current version: nothing to release yet.
func TestCurrentTag_CalVerWithOnlyOtherTags(t *testing.T) {
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\n", "", nil)

	_, err := app.CurrentTag(mr, cfg, "", false)

	assert.ErrorIs(t, err, versioning.ErrNothingToRelease)
}
