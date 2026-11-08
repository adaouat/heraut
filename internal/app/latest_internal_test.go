package app

import (
	"os/exec"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

// A release is GitHub's "Latest" only when it is the highest final release of its line: a 1.x patch
// published after 2.0.0, or a pre-release, is not.
func TestIsLatest(t *testing.T) {
	testutil.ClearCIEnv(t)
	testutil.RealGitRepo(t, "v1.0.0")
	git := func(args ...string) { require.NoError(t, exec.Command("git", args...).Run(), "git %v", args) }
	git("commit", "--allow-empty", "-q", "-m", "feat: two")
	git("tag", "-a", "v2.0.0", "-m", "v2.0.0")
	git("tag", "-a", "v1.0.1", "-m", "v1.0.1")
	git("tag", "-a", "v2.1.0-rc.1", "-m", "v2.1.0-rc.1")
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}, Release: &config.Release{Notes: &config.ContentDriver{}}}
	runner := execadapter.New(false, false)

	pCfg, err := buildReleasePipelineConfig(runner, runner, cfg, "", "", false, false)
	require.NoError(t, err)
	require.NotNil(t, pCfg.IsLatest)

	for tag, want := range map[string]bool{"v2.0.0": true, "v1.0.1": false, "v2.1.0-rc.1": false} {
		got, err := pCfg.IsLatest(tag)
		require.NoError(t, err)
		assert.Equal(t, want, got, tag)
	}
}
