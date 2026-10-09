package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

const perEnvCfg = `version: "1"
versioning:
  strategy: semver-per-env
  initial_version: "0.1.0"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`

func envArgs(cmd, env string, extra ...string) []string {
	return append([]string{"version", cmd, "--env", env}, extra...)
}

func TestPerEnv_AutoBump(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "first dev tag uses initial_version", config: perEnvCfg,
			history: []step{commit("feat: a")}, args: envArgs("next", "dev"), wantOut: "dev/0.1.0"},
		{name: "1.9.0 goes to 1.10.0 by SemVer order", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.9.0"), commit("feat: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.10.0"},
		{name: "fix bumps patch", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.4"},
		{name: "breaking commit bumps major", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("feat!: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/2.0.0"},
		{name: "another environment's tags do not leak in", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), tag("prod/9.0.0"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.4"},
		{name: "a pre-release in the namespace is not the bump base", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("feat: b"), tag("dev/1.3.0-rc.1"), commit("fix: c")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.3.0"},
		{name: "build metadata counts as the release of its core", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), tag("dev/1.2.4+77"), commit("fix: z")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.5"},
		{name: "version current reads the environment's latest tag", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("current", "dev"), wantOut: "dev/1.0.0"},
		{name: "version current with no tag in the namespace is a runtime error", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("current", "prod"), wantExit: exitRuntime, wantText: []string{"no tags found for \"prod/*\""}},
		{name: "version next without --env is a config error", config: perEnvCfg,
			history: []step{commit("feat: a")},
			args:    []string{"version", "next"}, wantExit: exitConfig, wantText: []string{`environment "" not found in config`}},
		{name: "version next with an unknown --env is a config error", config: perEnvCfg,
			history: []step{commit("feat: a")},
			args:    envArgs("next", "nope"), wantExit: exitConfig, wantText: []string{`environment "nope" not found in config`}},
		{name: "version current without --env is a config error", config: perEnvCfg,
			history: []step{commit("feat: a")},
			args:    []string{"version", "current"}, wantExit: exitConfig, wantText: []string{"--env is required for semver-per-env strategy"}},
		{name: "version current with an unknown --env is a config error", config: perEnvCfg,
			history: []step{commit("feat: a")},
			args:    envArgs("current", "nope"), wantExit: exitConfig, wantText: []string{`environment "nope" not found in config`}},
	})
}

func TestPerEnv_Promotion(t *testing.T) {
	bin := harness.Binary(t)
	chainCfg := `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  preprod:
    tag_format: "preprod/{version}"
    bump: promote
    source: dev
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: preprod
`
	runScenarios(t, bin, nil, []scenario{
		{name: "prod takes dev's latest version", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.0.2"},
		{name: "a pre-release source tag is never promoted", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2"), tag("dev/1.1.0-rc.1")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.0.2"},
		{name: "E003: no source tags", config: perEnvCfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e003", "no dev/* tags exist"}},
		{name: "E003 is not bypassed by --force", config: perEnvCfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod", "--force"),
			wantExit: exitPromotion, wantText: []string{"e003"}},
		{name: "E001: the target tag already exists", config: perEnvCfg,
			history:  []step{commit("feat: a"), tag("dev/1.0.2"), tag("prod/1.0.2")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e001", "prod/1.0.2", "already exists"}},
		{name: "E001 is bypassed by --force", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2"), tag("prod/1.0.2")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.2"},
		{name: "E002: the destination is already ahead", config: perEnvCfg,
			history:  []step{commit("feat: a"), tag("dev/1.0.2"), tag("prod/1.1.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e002", "version regression", "prod/1.1.0"}},
		{name: "E002 is bypassed by --force", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2"), tag("prod/1.1.0")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.2"},
		{name: "a chain promotes through the intermediate environment", config: chainCfg,
			history: []step{commit("feat: a"), tag("dev/1.4.0"), tag("preprod/1.3.0")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.3.0"},
		{name: "a chain's last hop has nothing to promote before the middle tag exists", config: chainCfg,
			history:  []step{commit("feat: a"), tag("dev/1.4.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e003", "no preprod/* tags exist"}},
	})
}

func TestPerEnv_TagFormats(t *testing.T) {
	bin := harness.Binary(t)
	single := func(format string) string {
		return `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "` + format + `"
    bump: auto
`
	}
	shared := `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
  prod:
    bump: promote
    source: dev
`
	runScenarios(t, bin, nil, []scenario{
		{name: "version first, env last", config: single("{version}/dev"),
			history: []step{commit("feat: a"), tag("1.2.3/dev"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "1.2.4/dev"},
		{name: "version then underscore", config: single("{version}_dev"),
			history: []step{commit("feat: a"), tag("1.2.3_dev"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "1.2.4_dev"},
		{name: "env then underscore", config: single("dev_{version}"),
			history: []step{commit("feat: a"), tag("dev_1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev_1.2.4"},
		{name: "custom prefix", config: single("release/{version}"),
			history: []step{commit("feat: a"), tag("release/1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "release/1.2.4"},
		{name: "a shared {env} format applies to every environment", config: shared,
			history: []step{commit("feat: a"), tag("dev/2.0.0"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/2.0.1"},
		{name: "a shared format also promotes", config: shared,
			history: []step{commit("feat: a"), tag("dev/2.0.0")},
			args:    envArgs("next", "prod"), wantOut: "prod/2.0.0"},
	})
}

func TestPerEnv_StayAtV0(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
  bump:
    stay_at_v0: true
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`
	runScenarios(t, bin, nil, []scenario{
		{name: "an auto environment holds the major back with a warning", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), commit("feat!: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/0.5.0",
			wantText: []string{"major bump held back by versioning.bump.stay_at_v0", "dev/1.0.0"}},
		{name: "--allow-major lifts it", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), commit("feat!: b")},
			args:    envArgs("next", "dev", "--allow-major"), wantOut: "dev/1.0.0",
			notText: []string{"held back"}},
		{name: "a promote environment is unaffected: dev at 1.0.0 promotes to 1.0.0", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), tag("prod/0.4.0"), commit("feat!: b"), tag("dev/1.0.0")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.0.0", notText: []string{"held back"}},
	})
}

func TestPerEnv_BuildMetadata(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}+{build}"
environments:
  uat:
    bump: auto
`
	runScenarios(t, bin, nil, []scenario{
		{name: "set-version with a build id fills {build}", config: cfg,
			history: []step{commit("feat: a")},
			args:    envArgs("next", "uat", "--set-version", "7.4.1", "--set-build-id", "158404"),
			wantOut: "uat/7.4.1+158404"},
		{name: "a {build} format without an id is a config error", config: cfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "uat", "--set-version", "7.4.1"),
			wantExit: exitConfig, wantText: []string{"contains {build} but no build id was provided"}},
		{name: "--set-build-id requires --set-version", config: cfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "uat", "--set-build-id", "5"),
			wantExit: exitConfig, wantText: []string{"--set-build-id requires --set-version"}},
	})
}

func TestPerEnv_EnvSelectionAndBranchGuard(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    branch: develop
    bump: auto
  prod:
    tag_format: "prod/{version}"
    branch: main
    bump: promote
    source: dev
`
	runScenarios(t, bin, nil, []scenario{
		{name: "--env auto picks the environment linked to the branch", config: cfg, branch: "main",
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("next", "auto"), wantOut: "prod/1.0.0"},
		{name: "--env auto on an unlinked branch asks for --env", config: cfg, branch: "feature/x",
			history:  []step{commit("feat: a"), tag("dev/1.0.0")},
			args:     envArgs("next", "auto"),
			wantExit: exitConfig, wantText: []string{`no env is linked to branch "feature/x"`}},
		{name: "--env auto requires a per-env strategy", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "auto"),
			wantExit: exitConfig, wantText: []string{"--env auto requires a per-env strategy"}},
		{name: "operating an environment from the wrong branch is refused", config: cfg, branch: "develop",
			history:  []step{commit("feat: a"), tag("dev/1.0.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitConfig, wantText: []string{`must be operated from branch "main"`, `current branch is "develop"`}},
		{name: "--force lifts the branch guard", config: cfg, branch: "develop",
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.0"},
	})
}

func TestPerEnv_ConfigErrors(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "an ambiguous promotion source is a config error", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  a:
    tag_format: "a/{version}"
    bump: auto
  b:
    tag_format: "b/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
`,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod"),
			wantExit: exitConfig, wantText: []string{"multiple auto environments exist; source is ambiguous"}},
		{name: "a tag_format without {version} is a config error", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/"
    bump: auto
`,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "dev"),
			wantExit: exitConfig, wantText: []string{"must contain {version}"}},
	})
}
