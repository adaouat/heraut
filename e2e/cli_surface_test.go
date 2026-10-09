package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestCheckConfig(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a valid config prints its path and source", config: semverCfg(""),
			args: []string{"check", "config"}, wantOut: ".heraut.yml  (from .heraut.yml)\n✓ config: ok"},
		{name: "an invalid value names the key and the valid choices", config: "version: \"1\"\nversioning:\n  strategy: semvr\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{`versioning.strategy: "semvr" is not a valid strategy`, "valid strategies: semver, calver, semver-per-env, calver-per-env", "1 error(s)"}},
		{name: "an unknown key reports its line", config: "version: \"1\"\nversioning:\n  strategy: semver\n  bogus_key: 1\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{"line 4: field bogus_key not found in type config.versioning"}},
		{name: "malformed YAML is a config error", config: "version: [unclosed",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{"yaml: line 1"}},
	})
}

func TestConfigDiscoveryAndTildeExpansion(t *testing.T) {
	bin := harness.Binary(t)
	valid := "version: \"1\"\nversioning:\n  strategy: semver\n"
	check := func(t *testing.T, repo *harness.Repo, env []string, args ...string) (string, int) {
		t.Helper()
		res := repo.Run(bin, env, append([]string{"check", "config"}, args...)...)
		return normalize(res.Stdout + " " + res.Stderr), res.ExitCode
	}

	t.Run(".config/heraut.yml is found when .heraut.yml is absent", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteFile(".config/heraut.yml", valid)

		out, code := check(t, repo, nil)

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "(from .config/heraut.yml)")
	})
	t.Run("HERAUT_FILE beats the default file", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(valid)
		repo.WriteFile("other.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=other.yml"})

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "other.yml (from heraut_file)")
	})
	t.Run("--config beats HERAUT_FILE", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteFile("a.yml", valid)
		repo.WriteFile("b.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=b.yml"}, "--config", "a.yml")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "a.yml (from --config)")
	})
	t.Run("~ in --config expands to the home directory", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteHomeFile("cfg/x.yml", valid)

		out, code := check(t, repo, nil, "--config", "~/cfg/x.yml")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, normalize(repo.Home()+"/cfg/x.yml (from --config)"))
	})
	t.Run("~ in HERAUT_FILE expands to the home directory", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteHomeFile("cfg/x.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=~/cfg/x.yml"})

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, normalize(repo.Home()+"/cfg/x.yml (from heraut_file)"))
	})
	t.Run("a missing file after expansion names the expanded path", func(t *testing.T) {
		repo := harness.NewRepo(t)

		out, code := check(t, repo, []string{"HERAUT_FILE=~/nope.yml"})

		require.Equal(t, exitConfig, code, out)
		assert.Contains(t, out, "nope.yml: no such file or directory")
		assert.NotContains(t, out, "~/nope.yml", "the ~ was expanded before the file was opened")
	})
	t.Run("no config anywhere is a config error for check config", func(t *testing.T) {
		repo := harness.NewRepo(t)

		out, code := check(t, repo, nil)

		require.Equal(t, exitConfig, code, out)
		assert.Contains(t, out, "reading config")
	})
}

// gitOnlyPath is a PATH holding nothing but git, so the real gh/glab can never be found.
func gitOnlyPath(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	require.NoError(t, os.Symlink(gitPath, filepath.Join(dir, "git")))
	return dir
}

const githubCfg = `version: "1"
versioning:
  strategy: semver
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
release:
  targets:
    - forge: github
`

func TestCheckRuntime(t *testing.T) {
	bin := harness.Binary(t)
	newRepo := func(t *testing.T, cfg string) *harness.Repo {
		repo := harness.NewRepo(t)
		if cfg != "" {
			repo.WriteConfig(cfg)
		}
		repo.Commit("feat: one")
		return repo
	}
	run := func(t *testing.T, repo *harness.Repo, path string, env []string, args ...string) (string, int) {
		t.Helper()
		res := repo.Run(bin, append([]string{"PATH=" + path}, env...), args...)
		return normalize(res.Stdout + " " + res.Stderr), res.ExitCode
	}

	t.Run("everything present passes", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "all checks passed")
	})
	t.Run("a missing gh is its own failure", func(t *testing.T) {
		repo := newRepo(t, githubCfg)

		out, code := run(t, repo, gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "gh not found")
		assert.NotContains(t, out, "environment variable gh_token is not set")
	})
	t.Run("a missing token is its own failure", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), nil, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "environment variable gh_token is not set")
		assert.NotContains(t, out, "gh not found")
	})
	t.Run("an unset git identity fails", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")
		repo.Git("config", "--unset", "user.name")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "git user.name — not configured")
	})
	t.Run("with no config every tool is required", func(t *testing.T) {
		repo := newRepo(t, "")

		out, code := run(t, repo, gitOnlyPath(t), nil, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "no config found at .heraut.yml — all tools checked as required")
		assert.Contains(t, out, "gh: not found on path")
		assert.Contains(t, out, "glab: not found on path", "without a config every supported tool is required")
	})
}

func TestCheck_ConfigErrorsOutrankRuntimeFailures(t *testing.T) {
	bin := harness.Binary(t)
	newRepo := func(t *testing.T, cfg string) *harness.Repo {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)
		repo.Commit("feat: one")
		return repo
	}
	path := "PATH=" + gitOnlyPath(t)

	t.Run("only the runtime fails: exit 3", func(t *testing.T) {
		repo := newRepo(t, githubCfg)

		res := repo.Run(bin, []string{path}, "check")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout), "config: ok")
	})
	t.Run("config and runtime fail: the config code wins", func(t *testing.T) {
		repo := newRepo(t, "version: \"1\"\nversioning:\n  strategy: nope\nforges:\n  - name: github\n    platform: github\n    repository: acme/widget\n    token_env: GH_TOKEN\n")

		res := repo.Run(bin, []string{path}, "check")

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		out := normalize(res.Stdout + " " + res.Stderr)
		assert.Contains(t, out, `"nope" is not a valid strategy`)
		assert.Contains(t, out, "gh not found", "the runtime section still ran and reported")
	})
}

func TestCommitVerify(t *testing.T) {
	bin := harness.Binary(t)
	cfg := semverCfg("")
	runScenarios(t, bin, nil, []scenario{
		{name: "a valid message prints its parts", config: cfg,
			args:     []string{"commit", "verify", "feat(cmd): add x"},
			wantText: []string{"commit message is valid", "type: feat", "scope: cmd", "breaking: false", "description: add x"},
			wantOut:  "✓ commit message is valid\n  type:        feat\n  scope:       cmd\n  breaking:    false\n  description: add x"},
		{name: "a bang marks the commit breaking", config: cfg,
			args:     []string{"commit", "verify", "feat!: big"},
			wantText: []string{"breaking: true"}, wantOut: "✓ commit message is valid\n  type:        feat\n  breaking:    true\n  description: big"},
		{name: "a malformed header is a usage error", config: cfg,
			args:     []string{"commit", "verify", "bad message"},
			wantExit: exitUsage, wantText: []string{`invalid conventional commit header "bad message"`, `expected "type(scope)!: description"`}},
		{name: "a type outside the allow-list is a usage error", config: cfg,
			args:     []string{"commit", "verify", "wip: x"},
			wantExit: exitUsage, wantText: []string{`commit type "wip" is not allowed`}},
		{name: "a merge commit is skipped", config: cfg,
			args: []string{"commit", "verify", "Merge branch 'x' into main"}, wantOut: ""},
		{name: "a fixup commit is skipped", config: cfg,
			args: []string{"commit", "verify", "fixup! feat: x"}, wantOut: ""},
		{name: "neither a message nor --file is a usage error", config: cfg,
			args:     []string{"commit", "verify"},
			wantExit: exitUsage, wantText: []string{"provide a commit message as an argument or via --file"}},
		{name: "both a message and --file is a usage error", config: cfg,
			args:     []string{"commit", "verify", "feat: x", "--file", "msg"},
			wantExit: exitUsage, wantText: []string{"not both"}},
		{name: "a missing --file is a usage error", config: cfg,
			args:     []string{"commit", "verify", "--file", "nope.txt"},
			wantExit: exitUsage, wantText: []string{"reading commit message from nope.txt"}},
	})

	t.Run("--file reads a message from disk", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)
		repo.WriteFile("msg.txt", "fix: from file\n")

		res := runOK(t, repo, bin, "commit", "verify", "--file", "msg.txt")

		assert.Contains(t, normalize(res.Stdout), "description: from file")
	})
	t.Run("--file - reads a message from stdin", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)

		res := repo.RunStdin(bin, nil, "fix: via stdin\n", "commit", "verify", "--file", "-")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout), "description: via stdin")
	})
}

func TestCommitCheck(t *testing.T) {
	bin := harness.Binary(t)
	history := []step{
		commit("feat: good one"), commit("oops not conventional"), tag("v0.1.0"),
		commit("fix: good two"), commit("wip: bad type"),
	}
	runScenarios(t, bin, nil, []scenario{
		{name: "the full history is scanned and every invalid commit reported", config: semverCfg(""), history: history,
			args:     []string{"commit", "check"},
			wantExit: exitUsage, wantText: []string{"oops not conventional", "wip: bad type", "2 of 4 commits invalid"}},
		{name: "--from-latest-tag scans only what is after the tag", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "--from-latest-tag"},
			wantExit: exitUsage, wantText: []string{"wip: bad type", "1 of 2 commits invalid"}, notText: []string{"oops not conventional"}},
		{name: "a clean range passes", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "HEAD~2..HEAD~1"},
			wantText: []string{"all commits follow conventional commits"}, wantOut: "✓ all commits follow conventional commits!\n  1 commits analysed"},
		{name: "a range and --from-latest-tag are mutually exclusive", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "--from-latest-tag", "HEAD"},
			wantExit: exitUsage, wantText: []string{"cannot use both --from-latest-tag and a rev-range argument"}},
		{name: "an unresolvable range is reported as such", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "nonsense..range"},
			wantExit: exitUsage, wantText: []string{`listing commits in range "nonsense..range"`}},
	})
}
