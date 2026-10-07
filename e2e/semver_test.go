package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

func semverCfg(bumpExtra string) string {
	return `version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump:
    mode: auto
` + bumpExtra
}

const stayAtV0 = "    stay_at_v0: true\n"

const manualCfg = `version: "1"
versioning:
  strategy: semver
  bump:
    mode: manual
`

var versionNext = []string{"version", "next"}

func TestSemVer_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "untagged repo uses initial_version", config: semverCfg(""),
			args: versionNext, wantOut: "v0.1.0"},
		{name: "feat bumps minor and 1.9.0 goes to 1.10.0", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("feat: b")},
			args:    versionNext, wantOut: "v1.10.0"},
		{name: "fix bumps patch", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("fix: b")},
			args:    versionNext, wantOut: "v1.9.1"},
		{name: "chore is a patch per the default table", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("chore: b")},
			args:    versionNext, wantOut: "v1.9.1"},
		{name: "bang prefix bumps major", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v2.0.0"},
		{name: "BREAKING CHANGE footer bumps major", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("fix: b\n\nBREAKING CHANGE: x")},
			args:    versionNext, wantOut: "v2.0.0"},
		{name: "non-conventional commits give no bump", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.0.0"), commit("update stuff")},
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"no releasable commits since v1.0.0"}},
		{name: "custom tag prefix round-trips", config: `version: "1"
versioning:
  strategy: semver
  tag_prefix: "rel-"
  initial_version: "0.1.0"
  bump:
    mode: auto
`,
			history: []step{commit("feat: a"), tag("rel-1.0.0"), commit("fix: b")},
			args:    versionNext, wantOut: "rel-1.0.1"},
		{name: "pre-release tag is not the bump base", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1")},
			args:    versionNext, wantOut: "v1.4.0"},
		{name: "build-metadata tag counts as its core release", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.4.0"), commit("fix: b"), tag("v1.4.1+158404"), commit("fix: c")},
			args:    versionNext, wantOut: "v1.4.2"},
		{name: "manual mode refuses to compute", config: manualCfg,
			history:  []step{commit("feat: a")},
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"manual bump mode requires --set-version"}},
		{name: "manual mode accepts --set-version", config: manualCfg,
			history: []step{commit("feat: a")},
			args:    []string{"version", "next", "--set-version", "2.0.0"}, wantOut: "v2.0.0"},
		{name: "empty tag prefix round-trips", config: `version: "1"
versioning:
  strategy: semver
  tag_prefix: ""
  initial_version: "0.1.0"
  bump:
    mode: auto
`,
			history: []step{commit("feat: a"), tag("1.0.0"), commit("fix: b")},
			args:    versionNext, wantOut: "1.0.1"},
	})
}

func TestSemVer_StayAtV0(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "without the setting a breaking commit reaches 1.0.0", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v1.0.0"},
		{name: "setting holds the major back to a minor with a warning", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v0.5.0",
			wantText: []string{"major bump held back by versioning.bump.stay_at_v0", "feat!: b"}},
		{name: "--allow-major lifts it for the run", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--allow-major"}, wantOut: "v1.0.0",
			notText: []string{"held back"}},
		{name: "no effect once the major is 1 or higher", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v1.2.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v2.0.0", notText: []string{"held back"}},
		{name: "--set-version ignores the setting", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--set-version", "1.0.0"}, wantOut: "v1.0.0",
			notText: []string{"held back"}},
		{name: "a pre-release is held back too, naming the candidates", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--pre-release", "rc"}, wantOut: "v0.5.0-rc.1",
			wantText: []string{"major bump held back", "v1.0.0-rc.1"}},
		{name: "a pre-release with --allow-major opens the major", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--pre-release", "rc", "--allow-major"}, wantOut: "v1.0.0-rc.1"},
	})
}

func TestSemVer_PreReleaseLifecycle(t *testing.T) {
	bin := harness.Binary(t)
	rc := []string{"version", "next", "--pre-release", "rc"}
	runScenarios(t, bin, nil, []scenario{
		{name: "first rc of the next minor", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b")},
			args:    rc, wantOut: "v1.4.0-rc.1"},
		{name: "second rc increments the counter", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1"), commit("fix: c")},
			args:    rc, wantOut: "v1.4.0-rc.2"},
		{name: "re-cutting a label needs a new commit", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1")},
			args:     rc,
			wantExit: exitRuntime, wantText: []string{"no commits since v1.4.0-rc.1"}},
		{name: "a higher label needs no new commit", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-beta.2")},
			args:    rc, wantOut: "v1.4.0-rc.1"},
		{name: "a lower label than an existing one is a regression", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("fix: c")},
			args:     []string{"version", "next", "--pre-release", "beta"},
			wantExit: exitRuntime, wantText: []string{"would sort below existing v1.4.0-rc.2"}},
		{name: "a breaking commit escalating the series is refused", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("feat!: d")},
			args:     rc,
			wantExit: exitRuntime, wantText: []string{"would escalate to a new major 2.0.0"}},
		{name: "--allow-major opens the new series with a warning", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("feat!: d")},
			args:    []string{"version", "next", "--pre-release", "rc", "--allow-major"}, wantOut: "v2.0.0-rc.1",
			wantText: []string{"pre-release core escalated 1.4.0 → 2.0.0"}},
		{name: "--pre-release cannot be combined with --set-version", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     []string{"version", "next", "--pre-release", "rc", "--set-version", "1.0.0"},
			wantExit: exitConfig, wantText: []string{"cannot be combined with --set-version"}},
		{name: "a dotted label is rejected", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     []string{"version", "next", "--pre-release", "bad.label"},
			wantExit: exitConfig, wantText: []string{"contains '.'"}},
		{name: "manual bump mode cannot mint pre-releases", config: manualCfg,
			history:  []step{commit("feat: a")},
			args:     rc,
			wantExit: exitConfig, wantText: []string{"requires versioning.bump.mode: auto"}},
		{name: "calver cannot mint pre-releases", config: `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""
`,
			history:  []step{commit("feat: a")},
			args:     rc,
			wantExit: exitConfig, wantText: []string{"requires versioning.strategy: semver"}},
	})
}

func TestSemVer_Overrides(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "set-version renders through the prefix", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "1.2.3"}, wantOut: "v1.2.3"},
		{name: "an already prefixed value round-trips", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "v1.2.3"}, wantOut: "v1.2.3"},
		{name: "set-build-id appends build metadata", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "1.2.3", "--set-build-id", "99"}, wantOut: "v1.2.3+99"},
		{name: "build metadata inside the version is rejected", config: semverCfg(""),
			args:     []string{"version", "next", "--set-version", "1.2.3+5"},
			wantExit: exitConfig, wantText: []string{"must not carry build metadata"}},
		{name: "a non-SemVer value is rejected", config: semverCfg(""),
			args:     []string{"version", "next", "--set-version", "nope"},
			wantExit: exitConfig, wantText: []string{"is not a valid semver version"}},
	})
}
