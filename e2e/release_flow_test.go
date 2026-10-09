package e2e_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const forgesCfg = `version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
commits:
  enrichment_forge: github
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
  - name: gitlab
    platform: gitlab
    project: acme/widget
    token_env: GITLAB_TOKEN
`

// releaseCfg appends a release block (targets) and any extra top-level config to forgesCfg.
func releaseCfg(targets, extra string) string {
	return forgesCfg + "release:\n  targets:\n" + targets + extra
}

const bothTargets = `    - forge: github
    - forge: gitlab
`

var notesPath = regexp.MustCompile(`\[[^\]]*heraut-notes-[0-9]+\]`)

// normCalls replaces the temp notes-file path, which differs per run, with <notes>.
func normCalls(calls []string) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = notesPath.ReplaceAllString(c, "[<notes>]")
	}
	return out
}

var tokens = []string{"GH_TOKEN=ghtok", "GITLAB_TOKEN=gltok"}

// releaseRepo is a flowRepo with both fake CLIs installed and one releasable commit.
func releaseRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := flowRepo(t, cfg)
	repo.FakeCLI("gh")
	repo.FakeCLI("glab")
	repo.Commit("feat: one")
	return repo
}

func TestRelease_GitHubArgv(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{
		"gh [--version]",
		"gh [api] [repos/acme/widget/releases?per_page=1]",
		"gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]",
	}, normCalls(repo.CLICalls()))
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag is pushed before the release is created")
	assert.Contains(t, normalize(res.Stdout), "https://github.com/acme/widget/releases/tag/v0.1.0")
}

func TestRelease_TwoTargetsPublishInDeclaredOrderWithDriverFlags(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(`    - forge: github
      draft: true
    - forge: gitlab
`, ""))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{
		"gh [--version]",
		"gh [api] [repos/acme/widget/releases?per_page=1]",
		"glab [--version]",
		"glab [api] [user]",
		"gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget] [--draft]",
		"glab [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]",
	}, normCalls(repo.CLICalls()), "every target is probed first, then published in declared order; draft is GitHub-only")
}

func TestRelease_PreReleaseIsMarkedFromTheVersion(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--pre-release", "rc")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	calls := normCalls(repo.CLICalls())
	assert.Equal(t, "gh [release] [create] [v0.1.0-rc.1] [--notes-file] [<notes>] [--repo] [acme/widget] [--prerelease]", calls[len(calls)-1])
}

func TestRelease_AssetsAreAttachedAndAZeroMatchIsLenient(t *testing.T) {
	bin := harness.Binary(t)
	cfg := releaseCfg(`    - forge: github
      assets:
        - "dist/*.txt"
`, "")

	t.Run("matching files are passed to gh", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		repo.WriteFile("dist/a.txt", "a\n")
		repo.WriteFile("dist/b.txt", "b\n")

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget] [dist/a.txt] [dist/b.txt]", calls[len(calls)-1])
	})
	t.Run("a pattern matching nothing publishes without assets", func(t *testing.T) {
		repo := releaseRepo(t, cfg)

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]", calls[len(calls)-1])
	})
}

func TestRelease_AFailingPublishAbortsTheLoop(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, ""))
	repo.FailReleases("gh")

	res := repo.Run(bin, tokens, "release", "--offline")

	require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "publish to github")
	for _, call := range repo.CLICalls() {
		assert.False(t, strings.HasPrefix(call, "glab [release]"), "the second target must not publish: %s", call)
	}
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag was already pushed and stays")
}

func TestRelease_DryRunCallsNothingAndCreatesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	before := repoState(t, repo)

	res := repo.Run(bin, tokens, "release", "--offline", "--dry-run")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout), "[dry-run] would create release")
	assert.Empty(t, repo.CLICalls(), "neither gh nor glab is invoked")
	assert.Equal(t, before, repoState(t, repo), "no tag, commit or file")
	assert.Empty(t, repo.GitRemote("tag", "-l"))
}

func TestRelease_ReleaseHooksRunPerTarget(t *testing.T) {
	bin := harness.Binary(t)

	t.Run("pre_release and post_release wrap each target in order", func(t *testing.T) {
		repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  pre_release:
    - run: "echo pre {{ .Platform }} >> .hooklog"
  post_release:
    - run: "echo post {{ .Platform }} {{ .Tag }} >> .hooklog"
`))

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{"pre github", "post github v0.1.0", "pre gitlab", "post gitlab v0.1.0"},
			logLines(repo.ReadFile(".hooklog")))
	})
	t.Run("a failing pre_release skips only its own target", func(t *testing.T) {
		repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  pre_release:
    - run: "{{ if eq .Platform \"github\" }}exit 1{{ else }}echo ok >> .hooklog{{ end }}"
`))

		res := repo.Run(bin, tokens, "release", "--offline")

		require.NotEqual(t, exitOK, res.ExitCode, "the run still fails when a target was skipped")
		calls := repo.CLICalls()
		for _, call := range calls {
			assert.False(t, strings.HasPrefix(call, "gh [release]"), "github must be skipped: %s", call)
		}
		assert.Contains(t, normCalls(calls), "glab [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]")
		assert.Equal(t, []string{"ok"}, logLines(repo.ReadFile(".hooklog")))
	})
}

func TestRelease_AMissingTokenIsRefusedInPreflight(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	before := repoState(t, repo)

	res := repo.Run(bin, nil, "release", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "environment variable gh_token is not set")
	assert.Equal(t, []string{"gh [--version]"}, repo.CLICalls(), "nothing but the binary check ran")
	assert.Equal(t, before, repoState(t, repo), "no tag, commit or file")
}

func TestRelease_ForceLiftsTheUnlistedBranchRefusal(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
changelog:
  output: CHANGELOG.md
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
release:
  targets:
    - forge: github
`
	newRepo := func(t *testing.T) *harness.Repo {
		repo := releaseRepo(t, cfg)
		repo.Checkout("feature/x")
		return repo
	}

	t.Run("without --force nothing is published", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Empty(t, repo.CLICalls())
		assert.Empty(t, repo.GitRemote("tag", "-l"))
	})
	t.Run("with --force the release goes ahead", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, tokens, "release", "--offline", "--force")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]", calls[len(calls)-1])
	})
}
