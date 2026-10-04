package app

import (
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

func resolvePreRelease(t *testing.T, label string, allowMajor bool) (versioning.Result, error) {
	t.Helper()
	testutil.ClearCIEnv(t)
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}
	r, err := NewResolver(cfg, "", false, "", "", execadapter.New(false, false),
		WithPreRelease(label), WithAllowMajor(allowMajor))
	require.NoError(t, err)
	return r.Resolve()
}

// TestPreReleaseLifecycle_RealRepo (T342, ADR-0064): spec § 7 scenarios against a real git repo,
// with tags created annotated the way the pipeline creates them.
func TestPreReleaseLifecycle_RealRepo(t *testing.T) {
	t.Run("beta to rc to final", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")

		commit("feat: add widget")
		res, err := resolvePreRelease(t, "beta", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-beta.1", res.Tag)
		git("tag", "-a", "-m", res.Tag, res.Tag)

		commit("fix: widget crash")
		res, err = resolvePreRelease(t, "beta", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-beta.2", res.Tag)
		git("tag", "-a", "-m", res.Tag, res.Tag)

		res, err = resolvePreRelease(t, "rc", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-rc.1", res.Tag)
		git("tag", "-a", "-m", res.Tag, res.Tag)

		// scenario 7: version current, before the final
		runner := execadapter.New(false, false)
		cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}
		cur, err := CurrentTag(runner, cfg, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.0", cur)
		cur, err = CurrentTag(runner, cfg, "", true)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-rc.1", cur)

		res, err = resolvePreRelease(t, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0", res.Tag)
	})

	t.Run("patch to minor escalation", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
		commit("fix: small bug")
		res, err := resolvePreRelease(t, "rc", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.1-rc.1", res.Tag)
		git("tag", "-a", "-m", res.Tag, res.Tag)

		commit("feat: add widget")
		res, err = resolvePreRelease(t, "rc", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-rc.1", res.Tag)
		require.NotEmpty(t, res.Warnings)
		joined := strings.Join(res.Warnings, "\n")
		assert.Contains(t, joined, "pre-release core escalated")
		assert.Contains(t, joined, "feat: add widget")
	})

	t.Run("blocked major and allow-major", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
		commit("feat: add widget")
		res, err := resolvePreRelease(t, "rc", false)
		require.NoError(t, err)
		git("tag", "-a", "-m", res.Tag, res.Tag)

		commit("feat!: drop legacy api")
		_, err = resolvePreRelease(t, "rc", false)
		require.ErrorIs(t, err, semver.ErrMajorEscalation)

		res, err = resolvePreRelease(t, "rc", true)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0-rc.1", res.Tag)
	})

	t.Run("next blocked after rc", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
		commit("feat: add widget")
		git("tag", "-a", "-m", "rc1", "v1.4.0-rc.1")
		commit("fix: more")
		_, err := resolvePreRelease(t, "next", false)
		require.ErrorIs(t, err, semver.ErrPreReleaseRegression)
	})

	t.Run("re-cut with no commit", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
		commit("feat: add widget")
		git("tag", "-a", "-m", "rc1", "v1.4.0-rc.1")
		_, err := resolvePreRelease(t, "rc", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no commits since v1.4.0-rc.1")
	})

	t.Run("side-branch tag ignored by commit requirement", func(t *testing.T) {
		git, commit := notesRepo(t)
		commit("feat: initial")
		git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
		git("checkout", "-b", "side")
		commit("feat: side work")
		git("tag", "-a", "-m", "rc1", "v1.4.0-rc.1")
		git("checkout", "main")
		commit("feat: main work")
		res, err := resolvePreRelease(t, "rc", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.4.0-rc.2", res.Tag)
		assert.Equal(t, "v1.3.0", res.CurrentTag)
	})
}
