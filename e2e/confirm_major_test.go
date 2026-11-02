package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// confirmMajorRepo is a repository at v1.4.0 with a breaking commit since, and a remote.
func confirmMajorRepo(t *testing.T) *harness.Repo {
	t.Helper()
	repo := harness.NewRepo(t)
	cfg := strings.Replace(withStandIns(repo, githubCfg), "  strategy: semver\n", "  strategy: semver\n  bump:\n    confirm_major: true\n", 1)
	repo.WriteConfig(cfg + "changelog:\n  output: CHANGELOG.md\n")
	repo.Commit("chore: init")
	repo.AddRemote()
	repo.Tag("v1.4.0")
	repo.Commit("feat!: drop the v1 API")
	return repo
}

// ADR-0078: with versioning.bump.confirm_major, an automatic major stops heraut release and heraut
// changelog with the Config code, before anything is written or tagged, a dry run included.
func TestConfirmMajor_RefusesAnAutomaticMajor(t *testing.T) {
	bin := harness.Binary(t)
	for _, args := range [][]string{
		{"release", "--offline"},
		{"release", "--offline", "--dry-run"},
		{"changelog", "--tag"},
		{"changelog", "--dry-run"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			repo := confirmMajorRepo(t)

			res := repo.Run(bin, tokens, args...)

			assert.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			out := res.Stdout + res.Stderr
			assert.Contains(t, out, "v2.0.0 is a major release")
			assert.Contains(t, out, "--allow-major")
			assert.Contains(t, out, "feat!: drop the v1 API")
			assert.Equal(t, "v1.4.0", strings.TrimSpace(repo.Git("tag", "--list")), "nothing is tagged")
			assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
			for _, call := range repo.APICalls() {
				assert.NotContains(t, call, "[POST]", "nothing is published")
			}
		})
	}
}

// --allow-major confirms the major for the run.
func TestConfirmMajor_AllowMajorReleasesTheMajor(t *testing.T) {
	bin := harness.Binary(t)
	repo := confirmMajorRepo(t)

	res := repo.Run(bin, tokens, "release", "--offline", "--allow-major")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, repo.Git("tag", "--list"), "v2.0.0")
}

// heraut version next previews the major: the tag on stdout, exit 0, the warning on stderr.
func TestConfirmMajor_VersionNextPreviews(t *testing.T) {
	bin := harness.Binary(t)
	repo := confirmMajorRepo(t)

	res := repo.Run(bin, nil, "version", "next")

	require.Equal(t, exitOK, res.ExitCode, "stderr:\n%s", res.Stderr)
	assert.Equal(t, "v2.0.0\n", res.Stdout)
	assert.Contains(t, res.Stderr, "refuse it without --allow-major")
}
