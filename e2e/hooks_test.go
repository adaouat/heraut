package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// hookCfg is a semver config with a changelog and the given hooks block.
func hookCfg(hooks string) string {
	return `version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
` + hooks
}

func logLines(content string) []string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

const allHooks = `hooks:
  post_bump:
    - run: "echo post_bump v={{ .Version }} t={{ .Tag }} p={{ .PreviousTag }} e={{ .Env }} >> .hooklog"
  pre_changelog:
    - run: "echo pre_changelog >> .hooklog"
  pre_tag:
    - run: "echo pre_tag >> .hooklog"
  post_tag:
    - run: "echo post_tag t={{ .Tag }} >> .hooklog"
`

func TestHooks_FireInOrderWithTemplateVariables(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, hookCfg(allHooks))
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, []string{
		"post_bump v=0.1.0 t=v0.1.0 p= e=",
		"pre_changelog",
		"pre_tag",
		"post_tag t=v0.1.0",
	}, logLines(repo.ReadFile(".hooklog")))
}

func TestHooks_PreviousTagIsFilledFromTheSecondRelease(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, hookCfg(allHooks))
	repo.Commit("feat: one")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline", "--no-hooks")
	repo.Commit("fix: two")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, []string{
		"post_bump v=0.1.1 t=v0.1.1 p=v0.1.0 e=",
		"pre_changelog",
		"pre_tag",
		"post_tag t=v0.1.1",
	}, logLines(repo.ReadFile(".hooklog")))
}

func TestHooks_EnvIsAvailableToPerEnvConfigs(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver-per-env
changelog:
  output: CHANGELOG.md
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
hooks:
  post_bump:
    - run: "echo env={{ .Env }} v={{ .Version }} t={{ .Tag }} >> .hooklog"
`)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--tag", "--env", "dev", "--offline")

	assert.Equal(t, []string{"env=dev v=0.1.0 t=dev/0.1.0"}, logLines(repo.ReadFile(".hooklog")))
}

func TestHooks_SeeTheStateOfTheirPipelinePosition(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, hookCfg(`hooks:
  pre_tag:
    - run: "git log -1 --format=%s > .pretag; git tag -l >> .pretag"
  post_tag:
    - run: "git ls-remote --tags origin > .posttag"
`))
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0\n", repo.ReadFile(".pretag"),
		"pre_tag runs after the changelog commit and before the tag exists")
	assert.Contains(t, repo.ReadFile(".posttag"), "refs/tags/v0.1.0", "post_tag runs after the tag reached the remote")
}

func TestHooks_AFailingHookAbortsWithoutRollback(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, hookCfg(`hooks:
  pre_tag:
    - run: "exit 7"
`))
	repo.Commit("feat: one")

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	out := normalize(res.Stdout + " " + res.Stderr)
	assert.Contains(t, out, `hook "exit 7"`)
	assert.Contains(t, out, "changelog failed", "the panel heading names the command, not the generation step")
	assert.NotContains(t, out, "generation failed", "the failing hook is not the changelog generation")
	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"), "the changelog commit stays")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "and it was already pushed")
	assert.Empty(t, repo.Git("tag", "-l"), "but no tag was created")
	assert.Empty(t, repo.GitRemote("tag", "-l"))
}

func TestHooks_APostBumpFailureStopsBeforeAnythingIsGenerated(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, hookCfg(`hooks:
  post_bump:
    - run: "echo one >> .hooklog"
    - run: "exit 1"
    - run: "echo three >> .hooklog"
`))
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.NotEqual(t, exitOK, res.ExitCode)
	assert.Equal(t, []string{"one"}, logLines(repo.ReadFile(".hooklog")), "the first failing step stops its list")
	assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"))
}

func TestHooks_Stage(t *testing.T) {
	bin := harness.Binary(t)
	stageCfg := hookCfg(`hooks:
  post_bump:
    - run: "echo {{ .Version }} > VERSION"
      stage:
        - VERSION
`)

	t.Run("a staged file lands in the changelog commit", func(t *testing.T) {
		repo := flowRepo(t, stageCfg)
		repo.Commit("feat: one")

		runOK(t, repo, bin, "changelog", "--commit", "--offline")

		files := strings.Fields(repo.Git("show", "--name-only", "--format=", "HEAD"))
		assert.ElementsMatch(t, []string{"CHANGELOG.md", "VERSION"}, files)
		assert.Equal(t, "0.1.0\n", repo.ReadFile("VERSION"))
	})

	t.Run("a pattern matching nothing fails the run before any commit", func(t *testing.T) {
		repo := flowRepo(t, hookCfg(`hooks:
  post_bump:
    - run: "true"
      stage:
        - nothing-here.txt
`))
		repo.Commit("feat: one")
		head := repo.Git("rev-parse", "HEAD")

		res := repo.Run(bin, nil, "changelog", "--commit", "--offline")

		require.NotEqual(t, exitOK, res.ExitCode)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "pathspec 'nothing-here.txt' did not match any files")
		assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no commit")
	})

	t.Run("without a changelog commit the file stays uncommitted", func(t *testing.T) {
		repo := flowRepo(t, stageCfg)
		repo.Commit("feat: one")
		head := repo.Git("rev-parse", "HEAD")

		runOK(t, repo, bin, "changelog", "--offline")

		assert.Equal(t, head, repo.Git("rev-parse", "HEAD"))
		assert.Contains(t, repo.Git("status", "--porcelain"), "?? VERSION")
	})
}

func TestHooks_ConfigErrors(t *testing.T) {
	bin := harness.Binary(t)
	tests := []struct {
		name  string
		hooks string
		want  string
	}{
		{"stage is only valid before the changelog commit", `hooks:
  pre_tag:
    - run: "true"
      stage:
        - x
`, "hooks.pre_tag[0].stage: not allowed here"},
		{"a bare string entry must be wrapped", `hooks:
  post_bump:
    - "echo hi"
`, `wrap it as { run: "echo hi" }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := flowRepo(t, hookCfg(tc.hooks))
			repo.Commit("feat: one")

			res := repo.Run(bin, nil, "changelog", "--offline")

			require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), normalize(tc.want))
		})
	}
}

func TestHooks_SkippingAndPreviewing(t *testing.T) {
	bin := harness.Binary(t)
	release := []string{"changelog", "--commit", "--tag", "--offline"}
	newRepo := func(t *testing.T) *harness.Repo {
		repo := flowRepo(t, hookCfg(allHooks))
		repo.Commit("feat: one")
		return repo
	}

	t.Run("--no-hooks skips every hook", func(t *testing.T) {
		repo := newRepo(t)

		runOK(t, repo, bin, append(release, "--no-hooks")...)

		assert.NoFileExists(t, repo.Dir+"/.hooklog")
		assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the release itself still happens")
	})

	t.Run("--skip-hook skips only the named points", func(t *testing.T) {
		repo := newRepo(t)

		runOK(t, repo, bin, append(release, "--skip-hook", "pre_tag,post_tag")...)

		assert.Equal(t, []string{"post_bump v=0.1.0 t=v0.1.0 p= e=", "pre_changelog"}, logLines(repo.ReadFile(".hooklog")))
	})

	t.Run("--skip-hook is repeatable", func(t *testing.T) {
		repo := newRepo(t)

		runOK(t, repo, bin, append(release, "--skip-hook", "pre_tag", "--skip-hook", "post_bump")...)

		assert.Equal(t, []string{"pre_changelog", "post_tag t=v0.1.0"}, logLines(repo.ReadFile(".hooklog")))
	})

	t.Run("HERAUT_SKIP_HOOKS skips points from the environment", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, []string{"HERAUT_SKIP_HOOKS=post_bump"}, release...)

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{"pre_changelog", "pre_tag", "post_tag t=v0.1.0"}, logLines(repo.ReadFile(".hooklog")))
	})

	t.Run("an unknown hook point is a config error", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, nil, append(release, "--skip-hook", "nope")...)

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), `unknown hook point "nope"`)
	})

	t.Run("--dry-run renders the commands and never runs them", func(t *testing.T) {
		repo := newRepo(t)

		res := runOK(t, repo, bin, "changelog", "--dry-run", "--tag", "--offline")

		assert.NoFileExists(t, repo.Dir+"/.hooklog")
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "[dry-run] would run: echo pre_changelog >> .hooklog")
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "[dry-run] would run: echo post_bump v=0.1.0 t=v0.1.0 p= e= >> .hooklog",
			"the template variables are rendered into the previewed command")
	})

	t.Run("--skip-hook wins over HERAUT_SKIP_HOOKS with no merging", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, []string{"HERAUT_SKIP_HOOKS=pre_tag"}, append(release, "--skip-hook", "post_tag")...)

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{"post_bump v=0.1.0 t=v0.1.0 p= e=", "pre_changelog", "pre_tag"}, logLines(repo.ReadFile(".hooklog")),
			"only the flag's point is skipped; the variable's pre_tag still runs")
	})

	t.Run("--no-hooks wins over HERAUT_SKIP_HOOKS without an error", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, []string{"HERAUT_SKIP_HOOKS=pre_tag"}, append(release, "--no-hooks")...)

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.NoFileExists(t, repo.Dir+"/.hooklog")
	})

	t.Run("--no-hooks together with --skip-hook is a config error", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, nil, append(release, "--no-hooks", "--skip-hook", "pre_tag")...)

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "cannot combine --skip-hook with --no-hooks")
	})

	t.Run("a release-only point is rejected under changelog", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, nil, append(release, "--skip-hook", "pre_release")...)

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), `hook point "pre_release" does not apply to this command`)
	})
}
