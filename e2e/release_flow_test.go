package e2e_test

import (
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
    base_url: __GITHUB_URL__
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

var tokens = []string{"GH_TOKEN=ghtok", "GITLAB_TOKEN=gltok"}

// The call log lines of the GitHub and GitLab requests on the acme/widget project.
const (
	githubList    = "github [GET] [/api/v3/repos/acme/widget/releases]"
	githubRelease = "github [POST] [/api/v3/repos/acme/widget/releases]"
	githubUpload  = "github [POST] [/api/uploads/repos/acme/widget/releases/1/assets]"
	githubPublish = "github [PATCH] [/api/v3/repos/acme/widget/releases/1]"
	githubDelete  = "github [DELETE] [/api/v3/repos/acme/widget/releases/1]"
	gitlabRelease = "gitlab [POST] [/api/v4/projects/acme/widget/releases]"
)

// withStandIns points the __GITHUB_URL__ and __GITLAB_URL__ base_url placeholders of cfg at the
// repository's GitHub and GitLab stand-ins.
func withStandIns(repo *harness.Repo, cfg string) string {
	cfg = strings.ReplaceAll(cfg, "__GITHUB_URL__", repo.FakeGitHub().URL)
	return strings.ReplaceAll(cfg, "__GITLAB_URL__", repo.FakeGitLab().URL)
}

// releaseRepo is a flowRepo with the GitHub and GitLab stand-ins installed (the forges' base_url
// point at them) and one releasable commit.
func releaseRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := harness.NewRepo(t)
	repo.WriteConfig(withStandIns(repo, cfg))
	repo.Commit("chore: init")
	repo.AddRemote()
	repo.Commit("feat: one")
	return repo
}

func TestRelease_GitHubRequests(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{githubList, githubRelease}, repo.APICalls(), "the token is checked, then the release is created in one call")
	rel := repo.FakeGitHub().Release()
	assert.Equal(t, "v0.1.0", rel["tag_name"])
	assert.Equal(t, false, rel["draft"])
	assert.Equal(t, false, rel["prerelease"])
	assert.Contains(t, rel["body"], "One")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag is pushed before the release is created")
	assert.Contains(t, res.Stdout, repo.FakeGitHub().URL+"/acme/widget/releases/tag/v0.1.0")
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
		githubList,
		"gitlab [GET] [/api/v4/user]",
		githubRelease,
		gitlabRelease,
	}, repo.APICalls(), "every target is probed first, then published in declared order")
	assert.Equal(t, true, repo.FakeGitHub().Release()["draft"], "draft is GitHub-only")
}

func TestRelease_PreReleaseIsMarkedFromTheVersion(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--pre-release", "rc")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	rel := repo.FakeGitHub().Release()
	assert.Equal(t, "v0.1.0-rc.1", rel["tag_name"])
	assert.Equal(t, true, rel["prerelease"])
}

func TestRelease_GitLabIgnoresThePreReleaseFlag(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--pre-release", "rc")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	calls := repo.APICalls()
	assert.Contains(t, calls, githubRelease)
	assert.Equal(t, true, repo.FakeGitHub().Release()["prerelease"])
	assert.Contains(t, calls, gitlabRelease)
	rel := repo.FakeGitLab().Release()
	assert.Equal(t, "v0.1.0-rc.1", rel["tag_name"])
	for key := range rel {
		assert.NotContains(t, key, "prerelease", "GitLab has no pre-release concept: the release is created plain")
	}
}

func TestRelease_AssetsAreAttachedAndAZeroMatchIsRefusedUnlessAllowed(t *testing.T) {
	bin := harness.Binary(t)
	const target = `    - forge: github
      assets:
        - "dist/*.txt"
`
	cfg := releaseCfg(target, "")

	t.Run("matching files are uploaded to a draft that is then published", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		repo.WriteFile("dist/a.txt", "a\n")
		repo.WriteFile("dist/b.txt", "b\n")

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{githubList, githubRelease, githubUpload, githubUpload, githubPublish}, repo.APICalls())
		assert.Equal(t, true, repo.FakeGitHub().Release()["draft"])
		assert.Equal(t, []string{"a.txt", "b.txt"}, repo.FakeGitHub().Uploads())
	})
	t.Run("a pattern matching nothing is refused before anything is committed or pushed", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		before := repoState(t, repo)

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		out := normalize(res.Stdout + " " + res.Stderr)
		assert.Contains(t, out, `"dist/*.txt" matches no file`)
		assert.Contains(t, out, "allow_missing_assets")
		assert.Equal(t, before, repoState(t, repo), "no changelog commit, no tag")
		assert.Empty(t, repo.GitRemote("tag", "-l"))
		assert.NotContains(t, repo.APICalls(), githubRelease, "only the read-only preflight may have reached GitHub")
	})
	t.Run("a file name with a # is published under the name GitHub gives it, with a warning", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		repo.WriteFile("dist/app#1.txt", "a\n")

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{"app#1.txt"}, repo.FakeGitHub().Uploads(), "the name is sent as the file's basename")
		assert.Contains(t, res.Stderr, `asset "dist/app#1.txt" was published as "app.1.txt"`)
	})
	t.Run("a refused upload deletes the draft and exits runtime", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		repo.WriteFile("dist/a.txt", "a\n")
		repo.FakeGitHub().FailUploads()

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{githubList, githubRelease, githubUpload, githubDelete}, repo.APICalls(), "no draft is left behind")
		out := normalize(res.Stdout + " " + res.Stderr)
		assert.Contains(t, out, "publish to github")
		assert.Contains(t, out, "uploading asset a.txt: 422 validation failed")
		assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag was already pushed and stays")
	})
	t.Run("a dry run only warns", func(t *testing.T) {
		repo := releaseRepo(t, cfg)

		res := repo.Run(bin, tokens, "release", "--offline", "--dry-run")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), `"dist/*.txt" matches no file`)
	})
	t.Run("allow_missing_assets publishes without assets", func(t *testing.T) {
		repo := releaseRepo(t, releaseCfg(target, "  allow_missing_assets: true\n"))

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{githubList, githubRelease}, repo.APICalls(), "nothing to upload")
	})
}

// Two assets with one basename would be published under one name: the run refuses them before the
// changelog commit, for either forge, and a dry run refuses them too.
func TestRelease_DuplicateAssetNamesAreRefusedBeforeAnythingIsCommitted(t *testing.T) {
	bin := harness.Binary(t)
	for _, forge := range []string{"github", "gitlab"} {
		cfg := releaseCfg("    - forge: "+forge+"\n      assets:\n        - \"dist/*/app.txt\"\n", "")
		for _, args := range [][]string{{"release", "--offline"}, {"release", "--offline", "--dry-run"}} {
			t.Run(forge+" "+strings.Join(args[1:], " "), func(t *testing.T) {
				repo := releaseRepo(t, cfg)
				repo.WriteFile("dist/linux/app.txt", "linux\n")
				repo.WriteFile("dist/darwin/app.txt", "darwin\n")
				before := repoState(t, repo)

				res := repo.Run(bin, tokens, args...)

				require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
				assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), `would both be published as "app.txt"`)
				assert.Equal(t, before, repoState(t, repo), "no changelog commit, no tag")
				assert.Empty(t, repo.GitRemote("tag", "-l"))
				for _, call := range repo.APICalls() {
					assert.Contains(t, call, "[GET]", "only the read-only preflight may have reached the forge")
				}
			})
		}
	}
}

func TestRelease_AFailingPublishAbortsTheLoop(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, ""))
	repo.FakeGitHub().FailReleases()

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "publish to github")
	assert.NotContains(t, repo.APICalls(), gitlabRelease, "the second target must not publish")
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
	}, repo.APICalls(), "assets are uploaded before the release that links them")
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
	assert.Empty(t, repo.APICalls(), "neither the GitHub nor the GitLab API is called")
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
		calls := repo.APICalls()
		assert.NotContains(t, calls, githubRelease, "github must be skipped")
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
	calls := repo.APICalls()
	assert.Contains(t, calls, githubRelease, "github was published")
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
	assert.Empty(t, repo.APICalls(), "the token is checked before any request")
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
    base_url: __GITHUB_URL__
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
		assert.Empty(t, repo.APICalls())
		assert.Empty(t, repo.GitRemote("tag", "-l"))
	})
	t.Run("with --force the release goes ahead", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, tokens, "release", "--offline", "--force")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{githubList, githubRelease}, repo.APICalls())
	})
}
