package app

import (
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// TestMaintenanceCurrentAndPreRelease_RealRepo replays ADR-0065 worked examples #9 and #10:
// main carries v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0, release/1.3 forks at v1.3.1 with one fix.
func TestMaintenanceCurrentAndPreRelease_RealRepo(t *testing.T) {
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v1.3.0")
	commit("fix: first")
	tag("v1.3.1")
	git("branch", "release/1.3")
	commit("feat: second")
	tag("v1.4.0")
	commit("feat!: third")
	tag("v2.0.0")
	git("checkout", "release/1.3")
	commit("fix: x")

	cfg := &config.Config{Version: "1", Versioning: config.Versioning{
		Strategy: "semver",
		Branches: []config.BranchRule{{Name: "main"}, {Name: "release/*"}},
	}}
	runner := execadapter.New(false, false)

	t.Run("#9 version current", func(t *testing.T) {
		cur, err := CurrentTag(runner, cfg, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.1", cur)
	})

	t.Run("#10 --pre-release rc", func(t *testing.T) {
		r, err := NewResolver(cfg, "", false, "", "", runner, WithPreRelease("rc"))
		require.NoError(t, err)
		res, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2-rc.1", res.Tag)
		assert.Equal(t, "v1.3.1", res.CurrentTag)
		tag(res.Tag)

		cur, err := CurrentTag(runner, cfg, "", true)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2-rc.1", cur)
	})

	t.Run("main unchanged", func(t *testing.T) {
		git("checkout", "main")
		cur, err := CurrentTag(runner, cfg, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0", cur)
	})
}
