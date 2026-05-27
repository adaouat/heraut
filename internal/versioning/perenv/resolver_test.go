package perenv_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedNow(year int, month time.Month, day int) func() time.Time {
	return func() time.Time {
		return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	}
}

// semverCalc builds a SemVer VersionCalculator (runner not needed for BumpAuto).
func semverCalc(initialVersion string) perenv.VersionCalculator {
	cfg := &config.Config{
		Versioning: config.Versioning{InitialVersion: initialVersion},
	}
	return semver.New(nil, cfg)
}

// calverCalc builds a CalVer VersionCalculator (runner not needed for BumpFromDate).
func calverCalc(format string, now func() time.Time) perenv.VersionCalculator {
	cfg := &config.Config{
		Versioning: config.Versioning{Format: format},
	}
	return calver.New(nil, cfg, now)
}

// ---- Auto mode: SemVer backend ----

func TestResolve_Auto_Semver_NoTags_InitialVersion(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -l dev/*  → no tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", result.Version)
	assert.Equal(t, "dev/0.1.0", result.Tag)
	assert.Equal(t, "", result.CurrentTag)

	// Only the tag list call; no git log since there are no existing tags.
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"tag", "-l", "dev/*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_Auto_Semver_BumpMinor(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.2.3\ndev/1.0.0\n", "", nil)
	mr.QueueResponse("feat: add new endpoint\x00fix: typo\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.3.0", result.Version)
	assert.Equal(t, "dev/1.3.0", result.Tag)
	assert.Equal(t, "dev/1.2.3", result.CurrentTag)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"log", "dev/1.2.3..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

func TestResolve_Auto_Semver_BumpMajor_Breaking(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.0.0\n", "", nil)
	mr.QueueResponse("feat!: breaking change\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", result.Version)
	assert.Equal(t, "dev/2.0.0", result.Tag)
}

func TestResolve_Auto_Semver_NoCommitsSinceTag_Error(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.0.0\n", "", nil)
	mr.QueueResponse("", "", nil) // empty git log

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

// Custom tag format: {version}/dev
func TestResolve_Auto_Semver_CustomTagFormat(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("1.0.0/dev\n", "", nil)
	mr.QueueResponse("fix: bugfix\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "{version}/dev"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.0.1", result.Version)
	assert.Equal(t, "1.0.1/dev", result.Tag)
}

// ---- Auto mode: CalVer backend ----

func TestResolve_Auto_Calver_NoTags_FirstRelease(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -l dev/*  → no tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver-per-env",
			Format:   "YYYY.MM.PATCH",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 24)))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.0", result.Version)
	assert.Equal(t, "dev/2026.05.0", result.Tag)
	assert.Equal(t, "", result.CurrentTag)

	// Only the tag list; no git log for calver.
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"tag", "-l", "dev/*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_Auto_Calver_SamePeriod_PatchIncrement(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/2026.05.1\ndev/2026.05.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver-per-env",
			Format:   "YYYY.MM.PATCH",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 24)))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.2", result.Version)
	assert.Equal(t, "dev/2026.05.2", result.Tag)
	assert.Equal(t, "dev/2026.05.1", result.CurrentTag)
}

func TestResolve_Auto_Calver_NewPeriod_PatchReset(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/2026.04.5\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver-per-env",
			Format:   "YYYY.MM.PATCH",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 1)))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.0", result.Version)
	assert.Equal(t, "dev/2026.05.0", result.Tag)
}

// ---- Promote mode ----

// backends is used to run promote scenarios against both SemVer and CalVer.
// promote mode never calls the calculator, so the backend choice only verifies
// that perenv.Resolver accepts either VersionCalculator implementation.
var promoteBackends = []struct {
	name     string
	strategy string
	calc     perenv.VersionCalculator
	envs     map[string]config.Environment
	srcTag   string // git tag -l response for source env
	dstTag   string // candidate rendered for destination
	bareVer  string // bare version from srcTag
}{
	{
		name:     "semver backend",
		strategy: "semver-per-env",
		calc:     semverCalc("0.1.0"),
		envs: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
		srcTag:  "dev/1.2.3",
		dstTag:  "prod/1.2.3",
		bareVer: "1.2.3",
	},
	{
		name:     "calver backend",
		strategy: "calver-per-env",
		calc:     calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 24)),
		envs: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
		srcTag:  "dev/2026.05.3",
		dstTag:  "prod/2026.05.3",
		bareVer: "2026.05.3",
	},
}

func TestResolve_Promote_HappyPath(t *testing.T) {
	for _, b := range promoteBackends {
		t.Run(b.name, func(t *testing.T) {
			mr := testutil.NewMockRunner()
			mr.QueueResponse(b.srcTag+"\n", "", nil) // git tag -l dev/*
			mr.QueueResponse("", "", nil)             // git tag -l <candidateTag> → does not exist
			mr.QueueResponse("", "", nil)             // git tag -l prod/* → no dest tags yet

			cfg := &config.Config{
				Versioning: config.Versioning{
					Strategy:     b.strategy,
					Format:       "YYYY.MM.PATCH",
				},
				Environments: b.envs,
			}

			r := perenv.New(mr, cfg, "prod", false, b.calc)
			result, err := r.Resolve()
			require.NoError(t, err)

			assert.Equal(t, b.bareVer, result.Version)
			assert.Equal(t, b.dstTag, result.Tag)
			assert.Equal(t, "", result.CurrentTag) // no prior dest tag
		})
	}
}

func TestResolve_Promote_E001_NoForce(t *testing.T) {
	for _, b := range promoteBackends {
		t.Run(b.name, func(t *testing.T) {
			mr := testutil.NewMockRunner()
			mr.QueueResponse(b.srcTag+"\n", "", nil) // git tag -l dev/*
			mr.QueueResponse(b.dstTag+"\n", "", nil) // git tag -l <candidateTag> → exists!

			cfg := &config.Config{
				Versioning: config.Versioning{
					Strategy:     b.strategy,
					Format:       "YYYY.MM.PATCH",
				},
				Environments: b.envs,
			}

			r := perenv.New(mr, cfg, "prod", false, b.calc)
			_, err := r.Resolve()
			require.Error(t, err)
			assert.True(t, errors.Is(err, perenv.ErrTargetExists), "expected ErrTargetExists, got: %v", err)
		})
	}
}

func TestResolve_Promote_E001_Force_Bypasses(t *testing.T) {
	for _, b := range promoteBackends {
		t.Run(b.name, func(t *testing.T) {
			mr := testutil.NewMockRunner()
			mr.QueueResponse(b.srcTag+"\n", "", nil) // git tag -l dev/*
			mr.QueueResponse(b.dstTag+"\n", "", nil) // git tag -l <candidateTag> → exists
			mr.QueueResponse("", "", nil)             // git tag -l prod/* → some dest tags

			cfg := &config.Config{
				Versioning: config.Versioning{
					Strategy:     b.strategy,
					Format:       "YYYY.MM.PATCH",
				},
				Environments: b.envs,
			}

			r := perenv.New(mr, cfg, "prod", true, b.calc)
			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, b.dstTag, result.Tag)
		})
	}
}

func TestResolve_Promote_E002_NoForce(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		calc     perenv.VersionCalculator
		envs     map[string]config.Environment
		srcTag   string
		dstTag   string // candidate tag
		aheadTag string // destination's current tag (> candidate)
	}{
		{
			name:     "semver backend",
			strategy: "semver-per-env",
			calc:     semverCalc("0.1.0"),
			envs: map[string]config.Environment{
				"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
				"prod": {Bump: "promote", TagFormat: "prod/{version}"},
			},
			srcTag:   "dev/1.2.3",
			dstTag:   "prod/1.2.3",
			aheadTag: "prod/1.2.4",
		},
		{
			name:     "calver backend",
			strategy: "calver-per-env",
			calc:     calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 24)),
			envs: map[string]config.Environment{
				"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
				"prod": {Bump: "promote", TagFormat: "prod/{version}"},
			},
			srcTag:   "dev/2026.05.3",
			dstTag:   "prod/2026.05.3",
			aheadTag: "prod/2026.05.4",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := testutil.NewMockRunner()
			mr.QueueResponse(tc.srcTag+"\n", "", nil)  // git tag -l dev/* → src
			mr.QueueResponse("", "", nil)               // git tag -l <candidateTag> → not exist
			mr.QueueResponse(tc.aheadTag+"\n", "", nil) // git tag -l prod/* → dest is ahead

			cfg := &config.Config{
				Versioning: config.Versioning{
					Strategy:     tc.strategy,
					Format:       "YYYY.MM.PATCH",
				},
				Environments: tc.envs,
			}

			r := perenv.New(mr, cfg, "prod", false, tc.calc)
			_, err := r.Resolve()
			require.Error(t, err)
			assert.True(t, errors.Is(err, perenv.ErrDestinationAhead), "expected ErrDestinationAhead, got: %v", err)
		})
	}
}

func TestResolve_Promote_E002_Force_Bypasses(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.2.3\n", "", nil) // git tag -l dev/*
	mr.QueueResponse("", "", nil)             // git tag -l prod/1.2.3 → not exist
	mr.QueueResponse("prod/1.2.4\n", "", nil) // git tag -l prod/* → dest is ahead

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "prod", true, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "prod/1.2.3", result.Tag)
}

func TestResolve_Promote_E003_NoSourceTags(t *testing.T) {
	for _, b := range promoteBackends {
		t.Run(b.name, func(t *testing.T) {
			mr := testutil.NewMockRunner()
			mr.QueueResponse("", "", nil) // git tag -l dev/* → no source tags

			cfg := &config.Config{
				Versioning: config.Versioning{
					Strategy:     b.strategy,
					Format:       "YYYY.MM.PATCH",
				},
				Environments: b.envs,
			}

			// force=true has no effect on E003
			r := perenv.New(mr, cfg, "prod", true, b.calc)
			_, err := r.Resolve()
			require.Error(t, err)
			assert.True(t, errors.Is(err, perenv.ErrNoSourceTags), "expected ErrNoSourceTags, got: %v", err)
		})
	}
}

// ---- Rich promotion error messages (T30 / ADR-0007) ----

func TestPromotionError_E001_RichMessage(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.0.2\n", "", nil) // git tag -l dev/*
	mr.QueueResponse("prod/1.0.2\n", "", nil) // git tag -l prod/1.0.2 → already exists

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, perenv.ErrTargetExists), "sentinel preserved: %v", err)

	var pe *perenv.PromotionError
	require.True(t, errors.As(err, &pe), "expected *PromotionError, got: %T", err)

	msg := err.Error()
	assert.Contains(t, msg, "error[E001]")
	assert.Contains(t, msg, "dev/1.0.2")
	assert.Contains(t, msg, "prod/1.0.2")
	assert.Contains(t, msg, "already exists")
	assert.Contains(t, msg, "git tag -d prod/1.0.2")
	assert.Contains(t, msg, "heraut release --env prod --force")
}

func TestPromotionError_E002_RichMessage(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/1.0.2\n", "", nil)  // git tag -l dev/*
	mr.QueueResponse("", "", nil)              // git tag -l prod/1.0.2 → not exist
	mr.QueueResponse("prod/1.0.3\n", "", nil)  // git tag -l prod/* → dest is ahead

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, perenv.ErrDestinationAhead), "sentinel preserved: %v", err)

	var pe *perenv.PromotionError
	require.True(t, errors.As(err, &pe), "expected *PromotionError, got: %T", err)

	msg := err.Error()
	assert.Contains(t, msg, "error[E002]")
	assert.Contains(t, msg, "dev/1.0.2")
	assert.Contains(t, msg, "prod/1.0.3")
	assert.Contains(t, msg, "regression")
	assert.Contains(t, msg, "backwards")
	assert.Contains(t, msg, "heraut release --env prod --force")
}

func TestPromotionError_E003_RichMessage(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -l dev/* → no source tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "prod", true, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, perenv.ErrNoSourceTags), "sentinel preserved: %v", err)

	var pe *perenv.PromotionError
	require.True(t, errors.As(err, &pe), "expected *PromotionError, got: %T", err)

	msg := err.Error()
	assert.Contains(t, msg, "error[E003]")
	assert.Contains(t, msg, "prod")
	assert.Contains(t, msg, "dev/*")
	assert.Contains(t, msg, "heraut release --env dev")
	assert.Contains(t, msg, "--force cannot bypass")
}

// ---- source: field resolution ----

func TestResolve_Source_Explicit_Override(t *testing.T) {
	mr := testutil.NewMockRunner()
	// preprod is explicit source for prod; preprod has auto tags
	mr.QueueResponse("preprod/1.5.0\n", "", nil) // git tag -l preprod/*
	mr.QueueResponse("", "", nil)                 // git tag -l prod/1.5.0 → not exist
	mr.QueueResponse("", "", nil)                 // git tag -l prod/* → no dest tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":     {Bump: "auto", TagFormat: "dev/{version}"},
			"preprod": {Bump: "promote", TagFormat: "preprod/{version}", Source: "dev"},
			"prod":    {Bump: "promote", TagFormat: "prod/{version}", Source: "preprod"},
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.5.0", result.Version)
	assert.Equal(t, "prod/1.5.0", result.Tag)
	// Source was preprod, so we listed preprod/* tags
	assert.Equal(t, []string{"tag", "-l", "preprod/*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_Source_Default_SingleAutoEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("dev/2.0.0\n", "", nil) // git tag -l dev/*
	mr.QueueResponse("", "", nil)             // git tag -l prod/2.0.0 → not exist
	mr.QueueResponse("", "", nil)             // git tag -l prod/* → no dest tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
			// no source: → defaults to single auto env (dev)
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", result.Version)
	assert.Equal(t, "prod/2.0.0", result.Tag)
	assert.Equal(t, []string{"tag", "-l", "dev/*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_Source_Ambiguity_Error(t *testing.T) {
	mr := testutil.NewMockRunner()
	// No mock responses needed — error before git calls.

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev1": {Bump: "auto", TagFormat: "dev1/{version}"},
			"dev2": {Bump: "auto", TagFormat: "dev2/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
			// no source: → ambiguous (two auto envs)
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambig")
	assert.Len(t, mr.Calls, 0) // no git calls before error
}

func TestResolve_Source_SelfReference_Error(t *testing.T) {
	mr := testutil.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}", Source: "prod"}, // self-reference
		},
	}

	r := perenv.New(mr, cfg, "prod", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// ---- Error paths ----

func TestResolve_UnknownEnvironment_Error(t *testing.T) {
	mr := testutil.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "staging", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging")
}

func TestResolve_UnknownBumpMode_Error(t *testing.T) {
	mr := testutil.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "rollback", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback")
}

// TestResolve_TopLevelTagFormat verifies that versioning.tag_format is used as a
// fallback when individual environments do not define their own tag_format.
// This is the common pattern: one tag_format for all envs, per-env overrides optional.
func TestResolve_TopLevelTagFormat_Auto(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -l dev/*  → no tags

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}", // top-level; env has no override
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto"}, // no TagFormat here
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", result.Version)
	assert.Equal(t, "dev/0.1.0", result.Tag)
	assert.Equal(t, []string{"tag", "-l", "dev/*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_GitTagListError(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "fatal: not a git repo", fmt.Errorf("exit 128"))

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"dev": {Bump: "auto", TagFormat: "dev/{version}"},
		},
	}

	r := perenv.New(mr, cfg, "dev", false, semverCalc("0.1.0"))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing")
}
