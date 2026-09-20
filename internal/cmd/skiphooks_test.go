package cmd_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipHookFlag_Registered(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"release", cmd.NewReleaseCmd("v0.0.0-test")},
		{"changelog", cmd.NewChangelogCmd("v0.0.0-test")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.cmd.Flags().Lookup("skip-hook")
			require.NotNil(t, f, "%s has a --skip-hook flag", tc.name)
			assert.Equal(t, "stringSlice", f.Value.Type(), "repeatable and comma-separable")
		})
	}
}

func TestSkipHookFlag_CompletesHookPoints(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"release", "release", []string{"post_bump", "pre_changelog", "pre_tag", "post_tag", "pre_release", "post_release"}},
		{"changelog", "changelog", []string{"post_bump", "pre_changelog", "pre_tag", "post_tag"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeRoot("__complete", tc.cmd, "--skip-hook", "")
			require.NoError(t, err)
			for _, point := range tc.want {
				assert.Contains(t, out, point+"\n")
			}
			if tc.cmd == "changelog" {
				assert.NotContains(t, out, "pre_release", "changelog never publishes")
				assert.NotContains(t, out, "post_release", "changelog never publishes")
			}
		})
	}
}

// TestSkipHook_InvalidInputIsAConfigError covers every rejection path. Each fails before the
// config file is even read, so no repo or config fixture is needed.
func TestSkipHook_InvalidInputIsAConfigError(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		args    []string
		wantErr []string
	}{
		{
			name:    "release: unknown flag value",
			args:    []string{"release", "--skip-hook", "pre_publish"},
			wantErr: []string{`unknown hook point "pre_publish"`, "post_release"},
		},
		{
			name:    "changelog: unknown flag value",
			args:    []string{"changelog", "--skip-hook", "pre_publish"},
			wantErr: []string{`unknown hook point "pre_publish"`},
		},
		{
			name:    "changelog: release-only point via flag",
			args:    []string{"changelog", "--skip-hook", "post_release"},
			wantErr: []string{`hook point "post_release" does not apply to this command`, "--skip-hook"},
		},
		{
			name:    "changelog: pre_release via flag",
			args:    []string{"changelog", "--skip-hook", "post_tag,pre_release"},
			wantErr: []string{`hook point "pre_release" does not apply to this command`},
		},
		{
			name:    "release: --skip-hook with --no-hooks",
			args:    []string{"release", "--skip-hook", "pre_tag", "--no-hooks"},
			wantErr: []string{"--skip-hook", "--no-hooks", "cannot combine"},
		},
		{
			name:    "changelog: --skip-hook with --no-hooks",
			args:    []string{"changelog", "--skip-hook", "pre_tag", "--no-hooks"},
			wantErr: []string{"--skip-hook", "--no-hooks", "cannot combine"},
		},
		{
			name:    "release: unknown env value names the variable",
			env:     "pre_publish",
			args:    []string{"release"},
			wantErr: []string{"HERAUT_SKIP_HOOKS", `unknown hook point "pre_publish"`},
		},
		{
			name:    "changelog: release-only point via env names the variable",
			env:     "post_release",
			args:    []string{"changelog"},
			wantErr: []string{"HERAUT_SKIP_HOOKS", `hook point "post_release" does not apply to this command`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HERAUT_SKIP_HOOKS", tc.env)
			_, err := executeRoot(tc.args...)
			require.Error(t, err)
			for _, want := range tc.wantErr {
				assert.Contains(t, err.Error(), want)
			}
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
		})
	}
}

// skipHooksRepo builds a real repo with one releasable commit since v0.1.0 and a config whose
// pre_tag/post_tag hooks each append a line to hooks.log, so a test can read back exactly which
// points actually ran. GIT_CONFIG_* isolation matches the other real-git hook tests (a host
// tag.gpgSign=true would otherwise hang the run).
func skipHooksRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	testutil.RealGitRepo(t, "v0.1.0")
	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "fix: something releasable").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	require.NoError(t, os.WriteFile(".heraut.yml", []byte(`
version: "1"
versioning:
  strategy: semver

hooks:
  pre_tag:
    - run: "echo pre_tag >> hooks.log"
  post_tag:
    - run: "echo post_tag >> hooks.log"
`), 0o644))
}

func hooksLog(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("hooks.log")
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(b)
}

func TestChangelog_RealGit_SkipHookFlagSkipsOnlyThatPoint(t *testing.T) {
	t.Setenv("HERAUT_SKIP_HOOKS", "")
	skipHooksRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push", "--skip-hook", "pre_tag")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Equal(t, "post_tag\n", hooksLog(t), "pre_tag skipped, post_tag still runs")
	tagOut, err := exec.Command("git", "tag", "-l", "v0.1.1").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tagOut), "v0.1.1", "skipping a hook must not skip the tag itself")
}

func TestChangelog_RealGit_SkipHookFlagIsRepeatableAndCommaSeparable(t *testing.T) {
	for name, args := range map[string][]string{
		"repeated": {"--skip-hook", "pre_tag", "--skip-hook", "post_tag"},
		"comma":    {"--skip-hook", "pre_tag,post_tag"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HERAUT_SKIP_HOOKS", "")
			skipHooksRepo(t)

			out, err := executeRoot(append([]string{"changelog", "--tag", "--no-push"}, args...)...)
			require.NoErrorf(t, err, "output:\n%s", out)

			assert.Empty(t, hooksLog(t), "both points skipped")
		})
	}
}

func TestChangelog_RealGit_SkipHooksEnvVarSkipsListedPoints(t *testing.T) {
	t.Setenv("HERAUT_SKIP_HOOKS", " pre_tag , post_tag ")
	skipHooksRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Empty(t, hooksLog(t), "whitespace around comma-separated env entries is tolerated")
}

// TestChangelog_RealGit_SkipHookFlagOverridesEnvVar pins the precedence: an explicit flag
// replaces the env var rather than adding to it, matching --config over HERAUT_FILE. A union
// would skip both points here; replacement skips only the flag's.
func TestChangelog_RealGit_SkipHookFlagOverridesEnvVar(t *testing.T) {
	t.Setenv("HERAUT_SKIP_HOOKS", "post_tag")
	skipHooksRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push", "--skip-hook", "pre_tag")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Equal(t, "post_tag\n", hooksLog(t), "the env var's post_tag is ignored once --skip-hook is given")
}

// TestChangelog_RealGit_NoHooksWithEnvVarIsNotAnError: the env var is an ambient default, not an
// explicit request, so an explicit --no-hooks simply wins over it. Only the two *flags* together
// are contradictory.
func TestChangelog_RealGit_NoHooksWithEnvVarIsNotAnError(t *testing.T) {
	t.Setenv("HERAUT_SKIP_HOOKS", "pre_tag")
	skipHooksRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push", "--no-hooks")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Empty(t, hooksLog(t), "--no-hooks still skips every hook")
}

func TestChangelog_RealGit_EmptySkipHooksEnvVarChangesNothing(t *testing.T) {
	t.Setenv("HERAUT_SKIP_HOOKS", "")
	skipHooksRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Equal(t, "pre_tag\npost_tag\n", hooksLog(t))
}
