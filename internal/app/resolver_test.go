package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func semverCfg() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
}

func calverCfg() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
}

func TestNewResolver_Semver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_Calver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_SemverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/${version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_CalverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "calver-per-env",
			Format:   "YYYY.MM.PATCH",
		},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/${version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_UnknownStrategy(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown-strategy"},
	}
	_, err := app.NewResolver(cfg, "", false, "", "", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-strategy")
}

func TestNewResolver_VersionOverride_Semver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "v2.0.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", result.Tag)
	assert.Equal(t, "2.0.0", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_Calver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "2026.05.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "2026.05.3", result.Tag)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_SemverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/{version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "v1.5.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "prod/1.5.0", result.Tag, "override must render through the env's tag_format, not bypass it")
	assert.Equal(t, "1.5.0", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_CalverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto", TagFormat: "{env}/{version}"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "2026.05.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/2026.05.3", result.Tag)
	assert.Equal(t, "2026.05.3", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_CustomTagPrefix_BareVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	prefix := "release-"
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix},
	}
	r, err := app.NewResolver(cfg, "", false, "1.2.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "release-1.2.3", result.Tag, "override must apply the configured tag_prefix, not the hardcoded default")
	assert.Equal(t, "1.2.3", result.Version)
}

func TestNewResolver_VersionOverride_CustomTagPrefix_FullTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	prefix := "release-"
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix},
	}
	r, err := app.NewResolver(cfg, "", false, "release-1.2.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "release-1.2.3", result.Tag, "a full tag already carrying the configured prefix must not be double-prefixed")
	assert.Equal(t, "1.2.3", result.Version)
}

func TestNewResolver_VersionOverride_EmptyTagPrefix(t *testing.T) {
	mr := exectest.NewMockRunner()
	empty := ""
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: &empty},
	}
	r, err := app.NewResolver(cfg, "", false, "1.2.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", result.Tag, "an explicit empty tag_prefix must not fall back to the semver default of \"v\"")
	assert.Equal(t, "1.2.3", result.Version)
}

func TestNewResolver_BuildID_RendersTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}+{build}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "7.4.1", "158404", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1+158404", result.Tag)
	assert.Equal(t, "7.4.1", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_BuildID_UsesEnvTagFormatOverride(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto", TagFormat: "{env}/{version}+{build}"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "7.4.1", "99", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1+99", result.Tag)
}

func TestNewResolver_BuildID_RequiresVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	_, err := app.NewResolver(semverCfg(), "", false, "", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--set-build-id requires --set-version")
}

func TestNewResolver_BuildID_NoBuildToken(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	_, err := app.NewResolver(cfg, "uat", false, "7.4.1", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{build}")
}

func TestNewResolver_BuildID_NoTagFormat(t *testing.T) {
	// Plain semver no longer errors here (ADR-0064) — CalVer still has no build-metadata
	// fallback, so its no-tag_format error path is asserted here instead.
	mr := exectest.NewMockRunner()
	_, err := app.NewResolver(calverCfg(), "", false, "7.4.1", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag_format")
}

// ADR-0064: plain semver has no tag_format, so --set-build-id appends SemVer build metadata.
func TestNewResolver_BuildID_PlainSemver_AppendsBuildMetadata(t *testing.T) {
	custom := "rel-"
	tests := []struct {
		name     string
		prefix   *string
		override string
		wantTag  string
	}{
		{"default prefix", nil, "1.4.0", "v1.4.0+158404"},
		{"prefixed override", nil, "v1.4.0", "v1.4.0+158404"},
		{"custom prefix", &custom, "rel-1.4.0", "rel-1.4.0+158404"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			cfg := semverCfg()
			cfg.Versioning.TagPrefix = tc.prefix
			r, err := app.NewResolver(cfg, "", false, tc.override, "158404", mr)
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
			assert.Equal(t, "1.4.0", result.Version)
			assert.Empty(t, mr.Calls, "static resolver must not call git")
		})
	}
}

// Plain semver's --set-build-id branch must validate that <version>+<buildID> is a real SemVer
// version with build metadata, or heraut tags something it can never read back (ADR-0064).
func TestNewResolver_BuildID_PlainSemver_ValidatesSemVer(t *testing.T) {
	tests := []struct {
		name     string
		override string
		buildID  string
		wantErr  bool
		wantTag  string
	}{
		{"non-semver build id rejected", "1.4.0", "build_1", true, ""},
		{"incomplete version rejected", "1.4", "5", true, ""},
		{"numeric build id still works", "1.4.0", "158404", false, "v1.4.0+158404"},
		{"pre-release version with build id", "1.4.0-rc.1", "158404", false, "v1.4.0-rc.1+158404"},
		{"dotted build id accepted", "1.4.0", "exp.sha.5114f85", false, "v1.4.0+exp.sha.5114f85"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			r, err := app.NewResolver(semverCfg(), "", false, tc.override, tc.buildID, mr)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--set-version")
				return
			}
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
		})
	}
}

// Plain `semver` with a top-level tag_format carrying {build} must get the same SemVer
// build-ID/composition validation as semver-per-env (ADR-0064); otherwise it could mint a tag
// (e.g. "v1.4+build_1") the resolver can never read back.
func TestNewResolver_BuildID_PlainSemver_WithTagFormat_ValidatesSemVer(t *testing.T) {
	tests := []struct {
		name     string
		override string
		buildID  string
		wantErr  bool
		wantTag  string
	}{
		{"non-semver build id rejected", "1.4.0", "build_1", true, ""},
		{"incomplete version rejected", "1.4", "5", true, ""},
		{"numeric build id still works", "1.4.0", "158404", false, "v1.4.0+158404"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			cfg := semverCfg()
			cfg.Versioning.TagFormat = "v{version}+{build}"
			r, err := app.NewResolver(cfg, "", false, tc.override, tc.buildID, mr)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--set-build-id")
				return
			}
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
			assert.Empty(t, mr.Calls, "static resolver must not call git")
		})
	}
}

// T333: semver-per-env's build ID must be valid SemVer build metadata, since {build} always
// follows "+" in tag_format (ADR-0064) — tightening tagfmt.ValidateBuildID's lenient "/"-and-
// whitespace-only check to the full [0-9A-Za-z-] dot-separated grammar.
func TestNewResolver_BuildID_SemverPerEnv_ValidatesSemVer(t *testing.T) {
	tests := []struct {
		name     string
		override string
		buildID  string
		wantErr  bool
		wantTag  string
	}{
		{"non-semver build id rejected", "7.4.1", "build_1", true, ""},
		{"incomplete version rejected", "7.4", "5", true, ""},
		{"numeric build id still works", "7.4.1", "158404", false, "uat/7.4.1+158404"},
		{"dotted build id accepted", "7.4.1", "exp.sha.5114f85", false, "uat/7.4.1+exp.sha.5114f85"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			cfg := &config.Config{
				Version: "1",
				Versioning: config.Versioning{
					Strategy:  "semver-per-env",
					TagFormat: "{env}/{version}+{build}",
				},
				Environments: map[string]config.Environment{
					"uat": {Bump: "auto"},
				},
			}
			r, err := app.NewResolver(cfg, "uat", false, tc.override, tc.buildID, mr)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--set-build-id")
				return
			}
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
			assert.Empty(t, mr.Calls, "static resolver must not call git")
		})
	}
}

// Guard: calver-per-env keeps today's lenient tagfmt.ValidateBuildID check only (no "/" or
// whitespace) — a CalVer build ID is not SemVer build metadata, so the tightened grammar from
// T333 must not apply to it.
func TestNewResolver_BuildID_CalverPerEnv_StaysLenient(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "calver-per-env",
			Format:    "YYYY.MM.PATCH",
			TagFormat: "{env}/{version}+{build}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "2026.05.3", "build_1", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/2026.05.3+build_1", result.Tag)
}

// T336/ADR-0064 (Phase 1.5): --set-version must itself be a valid SemVer v2 version under semver
// and semver-per-env — build metadata is rejected with a hint toward --set-build-id, since build
// metadata has exactly one entry point.
func TestNewResolver_VersionOverride_SemVerValidation(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		override string
		wantErr  bool
		wantTag  string
	}{
		{"bare version accepted", "semver", "1.4.0", false, "v1.4.0"},
		{"prefixed version accepted", "semver", "v1.4.0", false, "v1.4.0"},
		{"pre-release accepted", "semver", "1.4.0-rc.1", false, "v1.4.0-rc.1"},
		{"missing patch rejected", "semver", "1.4", true, ""},
		{"leading zero rejected", "semver", "01.4.0", true, ""},
		{"build metadata rejected", "semver", "1.4.0+abc", true, ""},
		{"semver-per-env bare version accepted", "semver-per-env", "1.4.0", false, "uat/1.4.0"},
		{"semver-per-env pre-release accepted", "semver-per-env", "1.4.0-rc.1", false, "uat/1.4.0-rc.1"},
		{"semver-per-env missing patch rejected", "semver-per-env", "1.4", true, ""},
		{"semver-per-env leading zero rejected", "semver-per-env", "01.4.0", true, ""},
		{"semver-per-env build metadata rejected", "semver-per-env", "1.4.0+abc", true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			env := ""
			cfg := semverCfg()
			if tc.strategy == "semver-per-env" {
				env = "uat"
				cfg = &config.Config{
					Version:    "1",
					Versioning: config.Versioning{Strategy: "semver-per-env"},
					Environments: map[string]config.Environment{
						"uat": {Bump: "auto", TagFormat: "uat/{version}"},
					},
				}
			}

			r, err := app.NewResolver(cfg, env, false, tc.override, "", mr)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--set-version")
				return
			}
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
		})
	}
}

// Build metadata specifically must hint at --set-build-id, not just report a parse failure
// (ADR-0064 Phase 1.5).
func TestNewResolver_VersionOverride_BuildMetadataHintsSetBuildID(t *testing.T) {
	mr := exectest.NewMockRunner()
	_, err := app.NewResolver(semverCfg(), "", false, "1.4.0+abc", "", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--set-build-id")
}

// When the plain-semver path (no tag_format) rejects a --set-version value
// and a non-default tag_prefix is configured, the error must name the expected prefix — otherwise
// a user with `tag_prefix: "rel-"` who passes an unprefixed or wrongly-prefixed value has no hint
// that "rel-" is what heraut actually expects.
func TestNewResolver_VersionOverride_SemVerValidation_NamesConfiguredPrefix(t *testing.T) {
	mr := exectest.NewMockRunner()
	custom := "rel-"
	cfg := semverCfg()
	cfg.Versioning.TagPrefix = &custom

	_, err := app.NewResolver(cfg, "", false, "v1.2.3", "", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expected an optional "rel-" prefix`)
}

// Guard: CalVer strategies keep today's lenient --set-version check. 2026.05.0 has a leading zero
// in its month segment and would fail strict SemVer parsing — it must stay accepted, since the
// new check (T336) only applies to semver/semver-per-env.
func TestNewResolver_VersionOverride_Calver_SemVerCheckDoesNotApply(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "2026.05.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "2026.05.0", result.Tag)
}

func TestNewResolver_VersionOverride_CalverPerEnv_SemVerCheckDoesNotApply(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto", TagFormat: "{env}/{version}"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "2026.05.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/2026.05.0", result.Tag)
}

func TestValidateBuildID(t *testing.T) {
	require.NoError(t, app.ValidateBuildID("158404"))
	require.Error(t, app.ValidateBuildID("bad/value"))
	require.Error(t, app.ValidateBuildID(""))
}

func TestValidateVersionOverride(t *testing.T) {
	require.NoError(t, app.ValidateVersionOverride("v1.2.3"))
	require.NoError(t, app.ValidateVersionOverride("2024.03.15.2"))
	require.Error(t, app.ValidateVersionOverride(""))
	require.Error(t, app.ValidateVersionOverride("1.2.3 "))
}

func TestNewResolver_PreRelease_UsageErrors(t *testing.T) {
	manual := semverCfg()
	manual.Versioning.Bump = &config.BumpConfig{Mode: "manual"}

	tests := []struct {
		name     string
		cfg      *config.Config
		override string
		label    string
		wantSub  string
	}{
		{"with set-version", semverCfg(), "1.4.0-rc.1", "rc",
			"--pre-release cannot be combined with --set-version: --set-version already chooses the version (pass a pre-release value such as 1.4.0-rc.1 to it instead)"},
		{"calver", calverCfg(), "", "rc",
			`--pre-release requires versioning.strategy: semver (got "calver"): pre-releases are minted for plain semver only (ADR-0064)`},
		{"semver-per-env", &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver-per-env"}}, "", "rc",
			`--pre-release requires versioning.strategy: semver (got "semver-per-env")`},
		{"calver-per-env", &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"}}, "", "rc",
			`--pre-release requires versioning.strategy: semver (got "calver-per-env")`},
		{"manual bump mode", manual, "", "rc",
			"--pre-release requires versioning.bump.mode: auto — manual mode has no computed version to build a pre-release on"},
		{"invalid label", semverCfg(), "", "RC!",
			`--pre-release "RC!": `},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			_, err := app.NewResolver(tc.cfg, "", false, tc.override, "", mr, app.WithPreRelease(tc.label))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
			assert.Empty(t, mr.Calls, "usage errors must be raised before any git call")
		})
	}
}

func TestNewResolver_PreRelease_ReachesResolver(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.3.0\n", "", nil)
	mr.QueueResponse("feat: add x\x00", "", nil)
	mr.QueueResponse("v1.3.0\n", "", nil)

	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr, app.WithPreRelease("rc"))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0-rc.1", res.Tag)
}

func TestNewResolver_PreReleaseEmpty_LeavesFinalPathUntouched(t *testing.T) {
	r, err := app.NewResolver(calverCfg(), "", false, "", "", exectest.NewMockRunner(), app.WithPreRelease(""))
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func maintenanceCfg(rules ...config.BranchRule) *config.Config {
	cfg := semverCfg()
	cfg.Versioning.Branches = rules
	return cfg
}

func gitArgs(mr *exectest.MockRunner) [][]string {
	out := make([][]string, len(mr.Calls))
	for i, c := range mr.Calls {
		out[i] = c.Args
	}
	return out
}

func TestNewResolver_Maintenance(t *testing.T) {
	revParse := []string{"rev-parse", "--abbrev-ref", "HEAD"}
	globalList := []string{"tag", "-l", "v*", "--sort=-version:refname"}
	mergedList := []string{"tag", "-l", "v*", "--merged", "HEAD", "--sort=-version:refname"}
	mainAndRelease := []config.BranchRule{{Name: "main"}, {Name: "release/*"}}

	tests := []struct {
		name       string
		cfg        *config.Config
		override   string
		preRelease string
		responses  []string
		wantTag    string
		wantErr    error
		wantCalls  [][]string
	}{
		{
			name:      "no branches block: no rev-parse call",
			cfg:       semverCfg(),
			responses: []string{"v2.0.0\nv1.3.1\n", "fix: x\x00"},
			wantTag:   "v2.0.1",
			wantCalls: [][]string{globalList, {"log", "v2.0.0..HEAD", "--format=%B%x00"}},
		},
		{
			name:      "maintenance branch: resolver gets the range",
			cfg:       maintenanceCfg(mainAndRelease...),
			responses: []string{"release/1.3\n", "v1.3.1\nv1.3.0\n", "fix: x\x00", ""},
			wantTag:   "v1.3.2",
			// ADR-0065: the collision probe also matches build-metadata releases of the version.
			wantCalls: [][]string{revParse, mergedList, {"log", "v1.3.1..HEAD", "--format=%B%x00"}, {"tag", "-l", "v1.3.2", "v1.3.2+*"}},
		},
		{
			name:      "release branch: global listing",
			cfg:       maintenanceCfg(mainAndRelease...),
			responses: []string{"main\n", "v2.0.0\nv1.3.1\n", "fix: x\x00"},
			wantTag:   "v2.0.1",
			wantCalls: [][]string{revParse, globalList, {"log", "v2.0.0..HEAD", "--format=%B%x00"}},
		},
		{
			name:      "unlisted branch: global listing (previews unchanged)",
			cfg:       maintenanceCfg(mainAndRelease...),
			responses: []string{"feature/foo\n", "v2.0.0\n", "fix: x\x00"},
			wantTag:   "v2.0.1",
			wantCalls: [][]string{revParse, globalList, {"log", "v2.0.0..HEAD", "--format=%B%x00"}},
		},
		{
			name:      "ambiguous match: error",
			cfg:       maintenanceCfg(config.BranchRule{Name: "release/*"}, config.BranchRule{Name: "release/1.3", Range: "1.3.x"}),
			responses: []string{"release/1.3\n"},
			wantErr:   app.ErrAmbiguousBranch,
			wantCalls: [][]string{revParse},
		},
		{
			name:      "underivable range under auto resolution: error",
			cfg:       maintenanceCfg(mainAndRelease...),
			responses: []string{"release/7.8.0\n"},
			wantErr:   app.ErrUnderivableRange,
			wantCalls: [][]string{revParse},
		},
		{
			name:       "pre-release on maintenance branch: base and range from the line",
			cfg:        maintenanceCfg(mainAndRelease...),
			preRelease: "rc",
			responses:  []string{"release/1.3\n", "v2.0.0\nv1.3.1\n", "v1.3.1\nv1.3.0\n", "fix: x\x00"},
			wantTag:    "v1.3.2-rc.1",
			wantCalls:  [][]string{revParse, globalList, mergedList, {"log", "v1.3.1..HEAD", "--format=%B%x00"}},
		},
		{
			// Pre-release is auto resolution, so it needs the range; only --set-version is exempt.
			name:       "pre-release on underivable glob match: error",
			cfg:        maintenanceCfg(mainAndRelease...),
			preRelease: "rc",
			responses:  []string{"release/legacy\n"},
			wantErr:    app.ErrUnderivableRange,
			wantCalls:  [][]string{revParse},
		},
		{
			// ADR-0065: --set-version gets the collision guard, so the only git call is that probe —
			// still no branch detection, so an underivable glob match is no error here.
			name:      "--set-version on release/7.8.0 with release/*: collision probe only",
			cfg:       maintenanceCfg(mainAndRelease...),
			override:  "7.8.0",
			responses: []string{""},
			wantTag:   "v7.8.0",
			wantCalls: [][]string{{"tag", "-l", "v7.8.0", "v7.8.0+*"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearBranchEnv(t)
			mr := exectest.NewMockRunner()
			for _, out := range tc.responses {
				mr.QueueResponse(out, "", nil)
			}
			r, err := app.NewResolver(tc.cfg, "", false, tc.override, "", mr, app.WithPreRelease(tc.preRelease))
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, tc.wantCalls, gitArgs(mr))
				return
			}
			require.NoError(t, err)
			res, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantCalls, gitArgs(mr))
		})
	}
}

func TestNewResolver_Maintenance_ManualModeSkipsBranchDetection(t *testing.T) {
	clearBranchEnv(t)
	cfg := maintenanceCfg(config.BranchRule{Name: "main"}, config.BranchRule{Name: "release/*"})
	cfg.Versioning.Bump = &config.BumpConfig{Mode: "manual"}
	mr := exectest.NewMockRunner()

	r, err := app.NewResolver(cfg, "", false, "", "", mr)
	require.NoError(t, err)
	_, err = r.Resolve()
	require.ErrorContains(t, err, "manual bump mode requires --set-version")
	assert.Empty(t, mr.Calls, "manual mode must not run branch detection")
}

// The spec's collision guard applies to --set-version when versioning.branches is set (ADR-0065):
// a tag of that version, or a build-metadata release of it, fails at Resolve time so the error
// exits Runtime like the auto path's.
func TestNewResolver_SetVersion_CollisionGuard(t *testing.T) {
	branches := []config.BranchRule{{Name: "main"}, {Name: "release/*"}}
	tests := []struct {
		name      string
		cfg       *config.Config
		override  string
		buildID   string
		responses []string
		wantTag   string
		wantErr   []string
		wantCalls [][]string
	}{
		{
			name: "free version", cfg: maintenanceCfg(branches...), override: "1.3.2",
			responses: []string{""},
			wantTag:   "v1.3.2",
			wantCalls: [][]string{{"tag", "-l", "v1.3.2", "v1.3.2+*"}},
		},
		{
			name: "tag exists", cfg: maintenanceCfg(branches...), override: "1.3.2",
			responses: []string{"v1.3.2\n"},
			wantErr:   []string{"v1.3.2", "already exists"},
			wantCalls: [][]string{{"tag", "-l", "v1.3.2", "v1.3.2+*"}},
		},
		{
			name: "build-metadata release exists", cfg: maintenanceCfg(branches...), override: "1.3.2",
			responses: []string{"v1.3.2+7\n"},
			wantErr:   []string{"v1.3.2+7", "already exists"},
			wantCalls: [][]string{{"tag", "-l", "v1.3.2", "v1.3.2+*"}},
		},
		{
			name: "with a build ID: exact tag only", cfg: maintenanceCfg(branches...), override: "1.3.2", buildID: "9",
			responses: []string{""},
			wantTag:   "v1.3.2+9",
			wantCalls: [][]string{{"tag", "-l", "v1.3.2+9"}},
		},
		{
			name: "no branches block: no git call", cfg: semverCfg(), override: "1.3.2",
			wantTag:   "v1.3.2",
			wantCalls: [][]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearBranchEnv(t)
			mr := exectest.NewMockRunner()
			for _, out := range tc.responses {
				mr.QueueResponse(out, "", nil)
			}
			r, err := app.NewResolver(tc.cfg, "", false, tc.override, tc.buildID, mr)
			require.NoError(t, err)
			res, err := r.Resolve()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, semver.ErrTagExists)
				for _, want := range tc.wantErr {
					assert.ErrorContains(t, err, want)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantTag, res.Tag)
			}
			assert.Equal(t, tc.wantCalls, gitArgs(mr))
		})
	}
}

func TestNewResolver_SetVersion_CollisionGuard_GitError(t *testing.T) {
	clearBranchEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("boom"))
	r, err := app.NewResolver(maintenanceCfg(config.BranchRule{Name: "main"}), "", false, "1.3.2", "", mr)
	require.NoError(t, err)
	_, err = r.Resolve()
	require.ErrorContains(t, err, "checking for existing tag v1.3.2")
	require.ErrorContains(t, err, "boom")
}

func TestNewResolver_SetVersion_WithoutCollisionGuard(t *testing.T) {
	clearBranchEnv(t)
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(maintenanceCfg(config.BranchRule{Name: "main"}), "", false, "1.3.2", "", mr, app.WithoutCollisionGuard())
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v1.3.2", res.Tag)
	assert.Empty(t, gitArgs(mr), "a run that never tags must not probe for an existing tag")
}
