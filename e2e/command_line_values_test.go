package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const calverPerEnvBuildCfg = `version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
environments:
  uat:
    tag_format: "uat/{version}+{build}"
    bump: auto
`

// ADR-0074: the values that come from the command line and end up in a tag, and from there in the
// hook variables, are restricted.
func TestCommandLineValues_BuildID(t *testing.T) {
	bin := harness.Binary(t)
	history := []step{commit("feat: a")}
	semverBuild := func(id string) []string {
		return []string{"version", "next", "--set-version", "1.2.3", "--set-build-id", id}
	}
	perEnvBuild := func(id string) []string {
		return []string{"version", "next", "--env", "uat", "--set-version", "2026.05.3", "--set-build-id", id}
	}

	runScenarios(t, bin, nil, []scenario{
		{name: "semver: a run number", config: semverCfg(""), history: history,
			args: semverBuild("158404"), wantOut: "v1.2.3+158404"},
		{name: "semver: dot-separated identifiers", config: semverCfg(""), history: history,
			args: semverBuild("run.12-a"), wantOut: "v1.2.3+run.12-a"},
		{name: "semver: shell syntax is refused", config: semverCfg(""), history: history,
			args: semverBuild("x$(id)"), wantExit: exitConfig, wantText: []string{"build id", "only letters, digits"}},
		{name: "semver: a slash is refused", config: semverCfg(""), history: history,
			args: semverBuild("a/b"), wantExit: exitConfig, wantText: []string{"build id", "must not contain '/'"}},
		{name: "semver: whitespace is refused", config: semverCfg(""), history: history,
			args: semverBuild("a b"), wantExit: exitConfig, wantText: []string{"build id", "whitespace"}},
		{name: "semver: an underscore passes the shared rule but is not SemVer build metadata", config: semverCfg(""), history: history,
			args: semverBuild("a_b"), wantExit: exitConfig, wantText: []string{"--set-build-id"}},
		{name: "calver-per-env: a run number", config: calverPerEnvBuildCfg, history: history,
			args: perEnvBuild("158404"), wantOut: "uat/2026.05.3+158404"},
		{name: "calver-per-env: an underscore is a tag-safe character", config: calverPerEnvBuildCfg, history: history,
			args: perEnvBuild("build_1"), wantOut: "uat/2026.05.3+build_1"},
		{name: "calver-per-env: shell syntax is refused", config: calverPerEnvBuildCfg, history: history,
			args: perEnvBuild("x$(id)"), wantExit: exitConfig, wantText: []string{"build id", "only letters, digits"}},
		{name: "calver-per-env: a slash is refused", config: calverPerEnvBuildCfg, history: history,
			args: perEnvBuild("a/b"), wantExit: exitConfig, wantText: []string{"build id", "must not contain '/'"}},
		{name: "calver-per-env: a semicolon is refused", config: calverPerEnvBuildCfg, history: history,
			args: perEnvBuild("a;id"), wantExit: exitConfig, wantText: []string{"build id", "only letters, digits"}},
	})
}

// --env names an environment, which only the per-env strategies have. Elsewhere the value would
// reach the hook variables unchecked.
func TestCommandLineValues_EnvNeedsAPerEnvStrategy(t *testing.T) {
	bin := harness.Binary(t)
	history := []step{commit("feat: a"), tag("v1.0.0"), commit("feat: b")}
	const need = "requires a per-env strategy"

	runScenarios(t, bin, nil, []scenario{
		{name: "semver: version next --env", config: semverCfg(""), history: history,
			args: []string{"version", "next", "--env", "prod"}, wantExit: exitConfig, wantText: []string{need}},
		{name: "semver: version current --env", config: semverCfg(""), history: history,
			args: []string{"version", "current", "--env", "prod"}, wantExit: exitConfig, wantText: []string{need}},
		{name: "semver: shell syntax in --env", config: semverCfg(""), history: history,
			args: []string{"version", "next", "--env", "x$(id)"}, wantExit: exitConfig, wantText: []string{need}},
		{name: "calver: version next --env", config: calverFmt("YYYY.MM.PATCH", "", ""), history: history,
			args: []string{"version", "next", "--env", "prod"}, wantExit: exitConfig, wantText: []string{need}},
		{name: "semver: without --env nothing changes", config: semverCfg(""), history: history,
			args: []string{"version", "next"}, wantOut: "v1.1.0"},
	})
}

// A refused value must never get as far as a hook.
func TestCommandLineValues_ARefusedValueNeverReachesAHook(t *testing.T) {
	bin := harness.Binary(t)
	hooks := `hooks:
  post_bump:
    - run: "echo {{ .Env }}:{{ .Tag }} >> .hooklog"
`
	tests := []struct {
		name string
		args []string
	}{
		{"a build ID with shell syntax", []string{"changelog", "--tag", "--offline", "--set-version", "1.2.3", "--set-build-id", "x$(id)"}},
		{"--env under semver", []string{"changelog", "--tag", "--offline", "--env", "x$(id)"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := flowRepo(t, hookCfg(hooks))
			repo.Commit("feat: one")
			head := repo.Git("rev-parse", "HEAD")

			res := repo.Run(bin, nil, tc.args...)

			require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.NoFileExists(t, repo.Dir+"/.hooklog", "the hook must not have run")
			assert.Equal(t, head, repo.Git("rev-parse", "HEAD"))
			assert.Empty(t, repo.Git("tag", "-l"))
		})
	}
}
