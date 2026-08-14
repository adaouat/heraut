package app_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
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
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_ChangelogBuildsNative(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithGitHubPlatform(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gh"}},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_WithGitLabPlatform(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gl", Type: "gitlab", Project: "group/subgroup/project"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gl"}},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_UnknownPlatform(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "bad", Type: "unknown-plat"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "bad"}},
	}
	_, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-plat")
}

func TestBuildPipeline_WithNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_AnnotatedTagsDefault(t *testing.T) {
	// Empty TagType → annotated is the default (AnnotatedTags: true).
	// We verify by building successfully — behavior is in pipeline tests.
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_LightweightTagType(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Versioning.TagType = "lightweight"
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_PerEnvDisableFlags(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Environments = map[string]config.Environment{
		"prod": {DisableChangelog: true, DisableNotes: true},
	}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Env: "prod"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_WithCommitAndTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Commit: true, Tag: true}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildChangelogPipeline_PerEnvDisable(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Environments = map[string]config.Environment{
		"staging": {DisableChangelog: true},
	}
	opts := app.PipelineOpts{Out: &bytes.Buffer{}, Env: "staging"}
	p, err := app.BuildChangelogPipeline(mr, cfg, defaultResolver, opts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestBuildPipeline_ReleaseAssets_PropagatesToPlatforms(t *testing.T) {
	// release.assets at the top level should build successfully — the platform
	// contract tests verify the actual upload behavior with LenientAssets=true.
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gh", Type: "github", Repository: "org/repo"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gh"}},
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
	mr := exectest.NewMockRunner()
	mr.QueueResponse("true\n", "", nil)
	assert.True(t, app.ReadGPGSign(mr))
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"config", "--get", "tag.gpgSign"}, mr.Calls[0].Args)
}

func TestReadGPGSign_False(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("false\n", "", nil)
	assert.False(t, app.ReadGPGSign(mr))
}

func TestReadGPGSign_NotSet(t *testing.T) {
	// git config --get exits 1 when the key is not set — treat as false.
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("exit status 1"))
	assert.False(t, app.ReadGPGSign(mr))
}
