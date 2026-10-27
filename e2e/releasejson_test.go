package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestReleaseDryRunJSON_PlansEveryActionAndChangesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  pre_tag:
    - run: "echo tagging {{ .Tag }} >> .hooklog"
  post_release:
    - run: "echo post {{ .Platform }} >> .hooklog"
`))
	before := repoState(t, repo)

	res := repo.Run(bin, tokens, "release", "--offline", "--dry-run", "--json")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, `{"ok":true,"data":{"tag":"v0.1.0","version":"0.1.0","pre_release":false,"steps":[`+
		`{"action":"changelog","target":"CHANGELOG.md"},`+
		`{"action":"commit_changelog","target":"CHANGELOG.md"},`+
		`{"action":"hook","point":"pre_tag","commands":["echo tagging v0.1.0 >> .hooklog"]},`+
		`{"action":"tag","target":"v0.1.0"},`+
		`{"action":"push_tag","target":"v0.1.0"},`+
		`{"action":"publish","target":"github","notes":true},`+
		`{"action":"hook","point":"post_release","target":"github","commands":["echo post github >> .hooklog"]},`+
		`{"action":"publish","target":"gitlab","notes":true},`+
		`{"action":"hook","point":"post_release","target":"gitlab","commands":["echo post gitlab >> .hooklog"]}]},"warnings":[]}`,
		strings.TrimSpace(res.Stdout))
	assert.Empty(t, repo.APICalls(), "neither the GitHub nor the GitLab API is called")
	assert.Equal(t, before, repoState(t, repo), "no tag, commit or file, and no hook ran")
}

func TestReleaseJSON_WithoutDryRunIsAUsageError(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--json")

	require.Equal(t, exitUsage, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, `"code":"usage"`)
	assert.Contains(t, res.Stdout, "--dry-run")
	assert.Empty(t, repo.GitRemote("tag", "-l"), "nothing was released")
}

func TestReleaseDryRunJSON_ErrorsAreEnveloped(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, "version: \"1\"\nversioning:\n  strategy: semvr\n")

	res := repo.Run(bin, tokens, "release", "--offline", "--dry-run", "--json")

	require.Equal(t, exitConfig, res.ExitCode)
	assert.Contains(t, res.Stdout, `"ok":false`)
	assert.Contains(t, res.Stdout, `"code":"config"`)
}
