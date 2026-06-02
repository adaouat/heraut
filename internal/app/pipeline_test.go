package app_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	result versioning.Result
	err    error
}

func (r *fakeResolver) Resolve() (versioning.Result, error) { return r.result, r.err }

var defaultResolver = &fakeResolver{
	result: versioning.Result{Version: "1.0.0", Tag: "v1.0.0"},
}

var defaultOpts = app.PipelineOpts{Out: &bytes.Buffer{}}

func TestBuildPipeline_Minimal(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithGitcliff(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithCommunique(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "communique", Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithCocogitto(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "cocogitto", Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_UnknownGenerator(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "unknown-gen"}
	_, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-gen")
}

func TestBuildPipeline_WithGitHubPlatform(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github"}},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithGitLabPlatform(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "gitlab"}},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_UnknownPlatform(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "unknown-plat"}},
	}
	_, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-plat")
}

func TestBuildPipeline_WithNotes(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{Generator: "git-cliff"},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_UnknownNotesGenerator(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{Generator: "unknown-notes"},
	}
	_, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-notes")
}

func TestBuildPipeline_AnnotatedTagsDefault(t *testing.T) {
	// Empty TagType → annotated is the default (AnnotatedTags: true).
	// We verify by building successfully — behavior is in pipeline tests.
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_LightweightTagType(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Versioning.TagType = "lightweight"
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_PerEnvDisableFlags(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Environments = map[string]config.Environment{
		"prod": {DisableChangelog: true, DisableNotes: true},
	}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Env: "prod"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_WithGitcliff(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_UnknownGenerator(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "unknown-gen"}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}}
	_, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-gen")
}

func TestBuildChangelogPipeline_WithCommitAndTag(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Commit: true, Tag: true}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_PerEnvDisable(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Environments = map[string]config.Environment{
		"staging": {DisableChangelog: true},
	}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Env: "staging"}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_PerEnvDerivesTagPattern(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git-cliff generate

	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "calver-per-env",
			Format:    "YYYY.SPRINT.PATCH",
			TagFormat: "{version}_{env}",
		},
		Changelog: &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto"},
		},
	}
	res := &fakeResolver{result: versioning.Result{Version: "2026.3.1", Tag: "2026.3.1_prod"}}
	opts := app.PipelineOpts{Env: "prod", Out: &bytes.Buffer{}}

	p, err := app.BuildChangelogPipeline(mr, cfg, res, opts)
	require.NoError(t, err)
	require.NoError(t, p.Run())

	var cliffArgs []string
	for _, c := range mr.Calls {
		if c.Name == "git-cliff" {
			cliffArgs = c.Args
		}
	}
	require.NotNil(t, cliffArgs, "expected a git-cliff call")
	// --tag-pattern scoped to the prod env must be present.
	var got string
	for i, a := range cliffArgs {
		if a == "--tag-pattern" && i+1 < len(cliffArgs) {
			got = cliffArgs[i+1]
		}
	}
	assert.Equal(t, "^.+_prod$", got)
}

func TestBuildChangelogPipeline_ExplicitTagPatternWins(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{version}_{env}",
		},
		Changelog: &config.ContentDriver{
			Generator:  "git-cliff",
			Output:     "CHANGELOG.md",
			TagPattern: "custom-pattern",
		},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto"},
		},
	}
	res := &fakeResolver{result: versioning.Result{Version: "1.2.3", Tag: "1.2.3_prod"}}
	opts := app.PipelineOpts{Env: "prod", Out: &bytes.Buffer{}}

	p, err := app.BuildChangelogPipeline(mr, cfg, res, opts)
	require.NoError(t, err)
	require.NoError(t, p.Run())

	var got string
	for _, c := range mr.Calls {
		if c.Name == "git-cliff" {
			for i, a := range c.Args {
				if a == "--tag-pattern" && i+1 < len(c.Args) {
					got = c.Args[i+1]
				}
			}
		}
	}
	assert.Equal(t, "custom-pattern", got, "explicit user tag_pattern must win over derivation")
}

func TestBuildChangelogPipeline_PerEnvPartialOverrideMerges(t *testing.T) {
	// ADR-0019: per-env changelog with only `config` inherits the top-level
	// generator + output. The pipeline must build (no "generator required" error)
	// and use the inherited output file.
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Changelog: &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"},
		Environments: map[string]config.Environment{
			"prod": {
				Bump:      "auto",
				Changelog: &config.ContentDriver{Config: "cliff.prod.toml"},
			},
		},
	}
	opts := app.PipelineOpts{Env: "prod", Out: &bytes.Buffer{}}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err, "partial per-env override must inherit the generator")
	assert.NotNil(t, p)
}

func TestBuildPipeline_ReleaseAssets_PropagatesToPlatforms(t *testing.T) {
	// release.assets at the top level should build successfully — the platform
	// contract tests verify the actual upload behavior with LenientAssets=true.
	mr := testutil.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github", Repository: "org/repo"}},
		Assets: []string{
			"dist/heraut_*_linux_amd64",
			"dist/checksums.txt",
		},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestReadGPGSign_True(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("true\n", "", nil)
	assert.True(t, app.ReadGPGSign(mr))
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"config", "--get", "tag.gpgSign"}, mr.Calls[0].Args)
}

func TestReadGPGSign_False(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("false\n", "", nil)
	assert.False(t, app.ReadGPGSign(mr))
}

func TestReadGPGSign_NotSet(t *testing.T) {
	// git config --get exits 1 when the key is not set — treat as false.
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", errors.New("exit status 1"))
	assert.False(t, app.ReadGPGSign(mr))
}
