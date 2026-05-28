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
