package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// strategyCfg is forgesCfg with its versioning block replaced, publishing to GitHub.
func strategyCfg(versioning, extra string) string {
	cfg := strings.Replace(forgesCfg, "versioning:\n  strategy: semver\n", versioning, 1)
	return cfg + "release:\n  targets:\n    - forge: github\n" + extra
}

const perEnvEnvs = `environments:
  dev:
    bump: auto
  prod:
    bump: promote
    source: dev
`

var nov7 = []string{"HERAUT_TEST_NOW=2026-11-07T12:00:00Z"}

// heraut release publishes under every strategy, not only plain semver.
func TestRelease_UnderEveryStrategy(t *testing.T) {
	t.Run("calver", func(t *testing.T) {
		bin := harness.Binary(t, "heraut_testclock")
		repo := releaseRepo(t, strategyCfg("versioning:\n  strategy: calver\n  format: \"YYYY.MM.PATCH\"\n", ""))

		res := repo.Run(bin, append(tokens, nov7...), "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "2026.11.0", repo.FakeGitHub().Release()["tag_name"])
		assert.Equal(t, "2026.11.0", repo.GitRemote("tag", "-l"))
	})

	t.Run("semver-per-env: auto, then a promotion", func(t *testing.T) {
		bin := harness.Binary(t)
		repo := releaseRepo(t, strategyCfg("versioning:\n  strategy: semver-per-env\n  tag_format: \"{env}/{version}\"\n", perEnvEnvs))

		res := repo.Run(bin, tokens, "release", "--env", "dev", "--offline")
		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "dev/0.1.0", repo.FakeGitHub().Release()["tag_name"])

		res = repo.Run(bin, tokens, "release", "--env", "prod", "--offline")
		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "prod/0.1.0", repo.FakeGitHub().Release()["tag_name"], "prod copies dev's version")
		assert.Equal(t, repo.Git("rev-parse", "dev/0.1.0^{commit}"), repo.Git("rev-parse", "prod/0.1.0^{commit}^"),
			"prod's release commit (its own changelog) sits right on the commit dev released")
	})

	t.Run("calver-per-env", func(t *testing.T) {
		bin := harness.Binary(t, "heraut_testclock")
		repo := releaseRepo(t, strategyCfg("versioning:\n  strategy: calver-per-env\n  format: \"YYYY.MM.PATCH\"\n  tag_format: \"{env}/{version}\"\n", perEnvEnvs))

		res := repo.Run(bin, append(tokens, nov7...), "release", "--env", "dev", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "dev/2026.11.0", repo.FakeGitHub().Release()["tag_name"])
	})
}

// An environment's disable_release tags without publishing; disable_changelog publishes without a
// changelog commit.
func TestRelease_PerEnvDisableFlags(t *testing.T) {
	bin := harness.Binary(t)
	versioning := "versioning:\n  strategy: semver-per-env\n  tag_format: \"{env}/{version}\"\n"

	t.Run("disable_release", func(t *testing.T) {
		repo := releaseRepo(t, strategyCfg(versioning, "environments:\n  dev:\n    bump: auto\n    disable_release: true\n"))

		res := repo.Run(bin, tokens, "release", "--env", "dev", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "dev/0.1.0", repo.GitRemote("tag", "-l"), "tagged and pushed")
		assert.NotContains(t, repo.APICalls(), githubRelease, "nothing published")
		assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"), "the changelog is still committed")
	})

	t.Run("disable_changelog", func(t *testing.T) {
		repo := releaseRepo(t, strategyCfg(versioning, "environments:\n  dev:\n    bump: auto\n    disable_changelog: true\n"))

		res := repo.Run(bin, tokens, "release", "--env", "dev", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, "dev/0.1.0", repo.FakeGitHub().Release()["tag_name"], "published")
		assert.Equal(t, "feat: one", repo.Git("log", "-1", "--format=%s"), "no changelog commit")
		assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
	})
}
