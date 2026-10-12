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
    base_url: __GITLAB_URL__
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

// gitlabRelease is the call log line of a GitLab release creation on the acme/widget project.
const gitlabRelease = "gitlab [POST] [/api/v4/projects/acme/widget/releases]"

// releaseRepo is a flowRepo with the fake gh CLI and the GitLab stand-in installed (the forge's
// base_url points at it) and one releasable commit.
func releaseRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := harness.NewRepo(t)
	repo.FakeCLI("gh")
	repo.WriteConfig(strings.ReplaceAll(cfg, "__GITLAB_URL__", repo.FakeGitLab().URL))
	repo.Commit("chore: init")
	repo.AddRemote()
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
		"gitlab [GET] [/api/v4/user]",
		"gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget] [--draft]",
		gitlabRelease,
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

func TestRelease_GitLabIgnoresThePreReleaseFlag(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--pre-release", "rc")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	calls := normCalls(repo.CLICalls())
	assert.Contains(t, calls, "gh [release] [create] [v0.1.0-rc.1] [--notes-file] [<notes>] [--repo] [acme/widget] [--prerelease]")
	assert.Contains(t, calls, gitlabRelease)
	rel := repo.FakeGitLab().Release()
	assert.Equal(t, "v0.1.0-rc.1", rel["tag_name"])
	for key := range rel {
		assert.NotContains(t, key, "prerelease", "GitLab has no pre-release concept: the release is created plain")
	}
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

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "publish to github")
	assert.NotContains(t, repo.CLICalls(), gitlabRelease, "the second target must not publish")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag was already pushed and stays")
}

func TestRelease_GitLabAssetsAreUploadedThenLinkedOnTheRelease(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(`    - forge: gitlab
      assets:
        - "dist/*.txt"
`, ""))
	repo.WriteFile("dist/a.txt", "a\n")
	repo.WriteFile("dist/b.txt", "b\n")

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{
		"gitlab [GET] [/api/v4/user]",
		"gitlab [PUT] [/api/v4/projects/acme/widget/packages/generic/release-assets/v0.1.0/a.txt]",
		"gitlab [PUT] [/api/v4/projects/acme/widget/packages/generic/release-assets/v0.1.0/b.txt]",
		gitlabRelease,
	}, repo.CLICalls(), "assets are uploaded before the release that links them")
	links := repo.FakeGitLab().Release()["assets"].(map[string]any)["links"].([]any)
	assert.Len(t, links, 2)
}

func TestRelease_AFailingGitLabPublishExitsRuntime(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: gitlab\n", ""))
	repo.FakeGitLab().FailReleases()

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "publish to gitlab")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag was already pushed and stays")
}

func TestRelease_DryRunCallsNothingAndCreatesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	before := repoState(t, repo)

	res := repo.Run(bin, tokens, "release", "--offline", "--dry-run")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout), "[dry-run] would create release")
	assert.Empty(t, repo.CLICalls(), "neither gh nor the GitLab API is called")
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

		require.Equal(t, exitRuntime, res.ExitCode, "the run still fails when a target was skipped")
		calls := repo.CLICalls()
		for _, call := range calls {
			assert.False(t, strings.HasPrefix(call, "gh [release]"), "github must be skipped: %s", call)
		}
		assert.Contains(t, calls, gitlabRelease)
		assert.Equal(t, []string{"ok"}, logLines(repo.ReadFile(".hooklog")))
	})
}

func TestRelease_AFailingPostReleaseHookWarnsAndTheLoopContinues(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  post_release:
    - run: "{{ if eq .Platform \"github\" }}exit 1{{ else }}echo ok >> .hooklog{{ end }}"
`))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "the run still fails when a post_release hook failed")
	calls := normCalls(repo.CLICalls())
	assert.Contains(t, calls, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]", "github was published")
	assert.Contains(t, calls, gitlabRelease, "and the loop went on to gitlab")
	assert.Equal(t, []string{"ok"}, logLines(repo.ReadFile(".hooklog")))
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
