//go:build e2e_forge

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/forgeharness"
	"github.com/adaouat/heraut/e2e/harness"
)

var forgeNames = []string{"github", "gitlab"}

func eachForge(t *testing.T, fn func(t *testing.T, ws *forgeharness.Workspace)) {
	t.Helper()
	for _, name := range forgeNames {
		t.Run(name, func(t *testing.T) {
			ws := forgeharness.NewWorkspace(t, forgeharness.Require(t, name))
			fn(t, ws)
		})
	}
}

func commitConfigAndSubjects(ws *forgeharness.Workspace, cfg string, subjects ...string) {
	ws.Repo.WriteConfig(cfg)
	ws.Repo.Git("add", ".heraut.yml")
	ws.Repo.Git("commit", "-q", "-m", "chore: init e2e")
	for _, s := range subjects {
		ws.Repo.Commit(s)
	}
}

func release(t *testing.T, ws *forgeharness.Workspace, bin string, args ...string) harness.Result {
	t.Helper()
	res := ws.Repo.Run(bin, ws.Env(), append([]string{"release", "--offline"}, args...)...)
	require.Equal(t, exitOK, res.ExitCode, "heraut release %v\nstdout:\n%s\nstderr:\n%s", args, res.Stdout, res.Stderr)
	return res
}

func semverBlock(ws *forgeharness.Workspace) string {
	return "versioning:\n  strategy: semver\n  tag_prefix: \"" + ws.TagPrefix + "\"\n"
}

func TestForge_B1_FinalRelease(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), ""), "feat: one", "fix: two")

		release(t, ws, bin)

		tag := ws.TagPrefix + "0.1.0"
		rel, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "the release exists on the forge")
		assert.Contains(t, rel.Body, "One", "the release body carries the generated notes")
		assert.Contains(t, rel.Body, "Two")
		assert.False(t, rel.Prerelease)
		assert.False(t, rel.Draft)
		sha, err := ws.Forge.TagCommit(tag)
		require.NoError(t, err)
		assert.Equal(t, ws.Repo.Git("rev-parse", "HEAD"), sha, "the tag points at the changelog commit")
	})
}

func TestForge_B2_PreRelease(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), ""), "feat: one")

		release(t, ws, bin, "--pre-release", "rc")

		rel, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0-rc.1")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, ws.Forge.HasPreReleaseFlag(), rel.Prerelease,
			"GitHub marks it a pre-release; GitLab has no such flag and creates a plain release")
	})
}

func TestForge_B3_BuildMetadata(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), ""), "feat: one")

		res := release(t, ws, bin, "--set-version", "0.1.0", "--set-build-id", "158404")

		tag := ws.TagPrefix + "0.1.0+158404"
		_, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "a tag carrying + build metadata is accepted")
		assert.Contains(t, res.Stdout, ws.Forge.ReleaseURL(tag), "the printed release URL spells the tag the way the forge does")
	})
}

func TestForge_B4_PerEnvTagWithAsset(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		block := "versioning:\n  strategy: semver-per-env\n  tag_format: \"" + ws.RunID + "/{env}/{version}\"\nenvironments:\n  uat:\n    bump: auto\n"
		cfg := ws.Config(block, "      assets:\n        - \"dist/app.txt\"\n")
		ws.Repo.WriteFile("dist/app.txt", "asset\n")
		ws.Repo.Git("add", "dist/app.txt")
		commitConfigAndSubjects(ws, cfg, "feat: one")

		release(t, ws, bin, "--env", "uat")

		tag := ws.RunID + "/uat/0.1.0"
		rel, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "a tag with / is accepted (the T335 regression on GitLab)")
		assert.Equal(t, 1, rel.AssetCount, "the asset was attached")
	})
}

func trimmed(s string) string { return strings.TrimSpace(s) }

func replaceOnce(s, old, new string) string { return strings.Replace(s, old, new, 1) }
