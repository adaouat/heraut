package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const branchesCfg = `version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
    - name: release/*
`

// lineHistory builds: main has v1.3.0, the line branch is cut there, main moves on to v1.4.0, and
// the repo ends up checked out on the line with extra applied.
func lineHistory(line string, extra ...step) []step {
	base := []step{
		commit("feat: a"), tag("v1.3.0"),
		checkout(line),
		switchTo("main"),
		commit("feat: b"), tag("v1.4.0"),
		switchTo(line),
	}
	return append(base, extra...)
}

func TestMaintenance_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a fix on the line releases the next patch of the line", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")), args: versionNext, wantOut: "v1.3.1"},
		{name: "version current is the line's base, not main's latest", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "current"}, wantOut: "v1.3.0"},
		{name: "a feat on a patch-only line is refused with the range", config: branchesCfg,
			history:  lineHistory("release/1.3", commit("feat: y")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"outside the maintenance range", "release/1.3 (>=1.3.0 <1.4.0)"}},
		{name: "a minor-capable line refuses a version cut on another branch", config: branchesCfg,
			history:  lineHistory("release/1.x", commit("feat: y")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.4.0 (cut on another branch)"}},
		{name: "a line without a release in range cannot resolve", config: branchesCfg,
			history:  lineHistory("release/2.0", commit("fix: x")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"no release in range 2.0.x in the history of release/2.0"}},
		{name: "version current on a line without a release in range fails too", config: branchesCfg,
			history:  lineHistory("release/2.0", commit("fix: x")),
			args:     []string{"version", "current"},
			wantExit: exitRuntime, wantText: []string{"no tags found in range 2.0.x reachable from release/2.0"}},
		{name: "main keeps resolving from its own history", config: branchesCfg,
			history: lineHistory("release/1.3", switchTo("main"), commit("fix: m")),
			args:    versionNext, wantOut: "v1.4.1"},
		{name: "an unlisted branch previews like today", config: branchesCfg,
			history: lineHistory("feature/x", commit("fix: x")), args: versionNext, wantOut: "v1.4.1"},
		{name: "a pre-release on the line stays in range", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--pre-release", "rc"}, wantOut: "v1.3.1-rc.1"},
		{name: "a build-metadata tag cut on the line itself is its release", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x"), tag("v1.3.1+7"), commit("fix: y")),
			args:    versionNext, wantOut: "v1.3.2"},
		{name: "a build-metadata tag of the same version cut elsewhere refuses", config: branchesCfg,
			history: lineHistory("release/1.3", switchTo("main"), commit("fix: m"), tag("v1.3.1+7"),
				switchTo("release/1.3"), commit("fix: x")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.3.1+7"}},
	})
}

func TestMaintenance_BranchRuleErrors(t *testing.T) {
	bin := harness.Binary(t)
	ambiguous := `version: "1"
versioning:
  strategy: semver
  branches:
    - name: release/*
    - name: release/1.3
      range: 1.3.x
`
	runScenarios(t, bin, nil, []scenario{
		{name: "two matching entries are a config error", config: ambiguous,
			history:  lineHistory("release/1.3", commit("fix: x")),
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"matches more than one versioning.branches entry"}},
		{name: "a glob match with no derivable range is a config error", config: branchesCfg,
			history:  lineHistory("release/foo", commit("fix: x")),
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"cannot derive a maintenance range from the branch name", "add range: to the entry"}},
		{name: "--set-version stays the escape hatch on such a branch", config: branchesCfg,
			history: lineHistory("release/foo", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "9.9.9"}, wantOut: "v9.9.9"},
		{name: "branches under calver is a config error", config: `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  branches:
    - name: main
`,
			history:  []step{commit("feat: a")},
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"only valid with strategy: semver"}},
	})
}

func TestMaintenance_SetVersionCollisions(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a taken version is refused", config: branchesCfg,
			history:  lineHistory("release/1.3", commit("fix: x")),
			args:     []string{"version", "next", "--set-version", "1.4.0"},
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.4.0", "pick a free version for --set-version"}},
		{name: "a free version is accepted", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "1.3.1"}, wantOut: "v1.3.1"},
		{name: "a free version with a build id is accepted", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "1.3.1", "--set-build-id", "5"}, wantOut: "v1.3.1+5"},
	})
}

func TestMaintenance_DetachedHeadUsesCIVariables(t *testing.T) {
	bin := harness.Binary(t)
	history := lineHistory("release/1.3", commit("fix: x"), detach())
	runScenarios(t, bin, nil, []scenario{
		{name: "no CI variable: the branch is unknown and previews like today", config: branchesCfg,
			history: history, args: versionNext, wantOut: "v1.4.1"},
		{name: "CI_COMMIT_BRANCH names the line", config: branchesCfg, history: history,
			args: versionNext, env: []string{"CI_COMMIT_BRANCH=release/1.3"}, wantOut: "v1.3.1"},
		{name: "BUILD_SOURCEBRANCH names the line", config: branchesCfg, history: history,
			args: versionNext, env: []string{"BUILD_SOURCEBRANCH=refs/heads/release/1.3"}, wantOut: "v1.3.1"},
		{name: "GITHUB_REF_NAME names the line when it is a branch ref", config: branchesCfg, history: history,
			args: versionNext, env: []string{"GITHUB_REF_NAME=release/1.3", "GITHUB_REF_TYPE=branch"}, wantOut: "v1.3.1"},
		{name: "GITHUB_REF_NAME is ignored for a tag ref", config: branchesCfg, history: history,
			args: versionNext, env: []string{"GITHUB_REF_NAME=release/1.3", "GITHUB_REF_TYPE=tag"}, wantOut: "v1.4.1"},
	})
}

const releaseTargetCfg = `forges:
  - name: github
    platform: github
    repository: acme/widget
release:
  targets:
    - forge: github
`

func TestMaintenance_ReleaseRefusesUnlistedBranches(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "release on an unlisted branch is refused before anything is written", config: branchesCfg + releaseTargetCfg,
			history:  lineHistory("feature/x", commit("fix: x")),
			args:     []string{"release", "--set-version", "1.2.3"},
			wantExit: exitConfig, wantText: []string{`branch matches no versioning.branches entry: branch "feature/x"`, "pass --force to release anyway"}},
	})
}

func TestMaintenance_DryRunIsNotRefused(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(branchesCfg + releaseTargetCfg)
	applySteps(repo, lineHistory("feature/x", commit("fix: x")))

	res := repo.Run(bin, nil, "release", "--set-version", "1.2.3", "--dry-run")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, "[dry-run] would tag")
	assert.NotContains(t, normalize(res.Stdout+" "+res.Stderr), "pass --force")
}
