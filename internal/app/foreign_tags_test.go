package app_test

import (
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
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
