package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// ADR-0074 addendum: under the CalVer strategies --set-version is the canonical form of the
// configured format, because it ends up in the tag and in the hook variables.
func TestCalVer_SetVersionMustBeCanonical(t *testing.T) {
	bin := harness.Binary(t)
	setVersion := func(value string) []string { return []string{"version", "next", "--set-version", value} }
	setVersionEnv := func(env, value string) []string {
		return []string{"version", "next", "--env", env, "--set-version", value}
	}
	history := []step{commit("feat: a")}
	const formatNamed = "yyyy.mm.patch"

	runScenarios(t, bin, nil, []scenario{
		{name: "a canonical value is printed as the tag", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("2026.05.3"), wantOut: "2026.05.3"},
		{name: "a PATCH of zero is canonical", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("2026.05.0"), wantOut: "2026.05.0"},
		{name: "the configured prefix is added", config: calverFmt("YYYY.MM.PATCH", "v", ""),
			history: history, args: setVersion("2026.05.3"), wantOut: "v2026.05.3"},
		{name: "a value already carrying the prefix round-trips", config: calverFmt("YYYY.MM.PATCH", "v", ""),
			history: history, args: setVersion("v2026.05.3"), wantOut: "v2026.05.3"},
		{name: "per-env: rendered through the env's tag_format", config: calverPerEnvCfg,
			history: history, args: setVersionEnv("dev", "2026.10.1"), wantOut: "dev/2026.10.1"},
		{name: "per-env: a leading v is stripped before the template", config: calverPerEnvCfg,
			history: history, args: setVersionEnv("dev", "v2026.10.1"), wantOut: "dev/2026.10.1"},

		{name: "an unpadded month is refused with the canonical spelling", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("2026.5.3"),
			wantExit: exitConfig, wantText: []string{formatNamed, "canonical", "2026.05.3"}},
		{name: "a padded PATCH is refused", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("2026.05.03"),
			wantExit: exitConfig, wantText: []string{formatNamed, "canonical"}},
		{name: "shell syntax is refused", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("x$(id)"),
			wantExit: exitConfig, wantText: []string{"only letters, digits"}},
		{name: "a command separator is refused", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("2026.05.3;id"),
			wantExit: exitConfig, wantText: []string{"only letters, digits"}},
		{name: "a SemVer-looking value is refused", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: history, args: setVersion("1.2.3"),
			wantExit: exitConfig, wantText: []string{formatNamed}},
		{name: "per-env: shell syntax is refused", config: calverPerEnvCfg,
			history: history, args: setVersionEnv("dev", "x$(id)"),
			wantExit: exitConfig, wantText: []string{"only letters, digits"}},
		{name: "per-env: an unpadded month is refused", config: calverPerEnvCfg,
			history: history, args: setVersionEnv("dev", "2026.5.1"),
			wantExit: exitConfig, wantText: []string{formatNamed, "canonical"}},
	})
}

// A refused value must stop the run before it commits, tags or pushes anything.
func TestCalVer_RefusedSetVersionTouchesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, calverFmt("YYYY.MM.PATCH", "", "changelog:\n  output: CHANGELOG.md\n"))
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := repo.Run(bin, nil, "changelog", "--tag", "--offline", "--set-version", "2026.5.3")

	require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "canonical")
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no commit")
	assert.Empty(t, repo.Git("tag", "-l"), "no tag")
	assert.Empty(t, repo.GitRemote("tag", "-l"), "nothing pushed")
	assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
}
