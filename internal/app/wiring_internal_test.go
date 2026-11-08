package app

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

// noOrigin is a runner whose git remote lookup fails, so forge resolution finds nothing.
func noOrigin() *exectest.MockRunner {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin"))
	return mr
}

// releaseReads is the read runner of a release build: no origin (forge resolution finds nothing),
// then checked out on main (git symbolic-ref), so the release commit has a branch to push to.
func releaseReads() *exectest.MockRunner {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("no origin"))
	mr.QueueResponse("main\n", "", nil)
	return mr
}

func wiringCfg() *config.Config {
	return &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"},
		Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}}
}

// versioning.tag_type decides whether the release tags are annotated, annotated by default.
func TestBuildReleasePipelineConfig_TagType(t *testing.T) {
	testutil.ClearCIEnv(t)
	for tagType, annotated := range map[string]bool{"": true, "annotated": true, "lightweight": false} {
		cfg := wiringCfg()
		cfg.Versioning.TagType = tagType
		pCfg, err := buildReleasePipelineConfig(exectest.NewMockRunner(), releaseReads(), cfg, "", "", false, false)
		require.NoError(t, err)
		assert.Equal(t, annotated, pCfg.AnnotatedTags, "tag_type %q", tagType)
	}
}

// An environment's disable_changelog and disable_release reach the release pipeline.
func TestBuildReleasePipelineConfig_PerEnvDisableFlags(t *testing.T) {
	testutil.ClearCIEnv(t)
	cfg := wiringCfg()
	cfg.Versioning.Strategy = "semver-per-env"
	cfg.Versioning.TagFormat = "{env}/{version}"
	cfg.Release = &config.Release{Notes: &config.ContentDriver{}}
	cfg.Environments = map[string]config.Environment{
		"prod": {Bump: "auto", DisableChangelog: true, DisableRelease: true},
		"dev":  {Bump: "auto"},
	}

	prod, err := buildReleasePipelineConfig(exectest.NewMockRunner(), releaseReads(), cfg, "prod", "", false, false)
	require.NoError(t, err)
	assert.True(t, prod.DisableChangelog)
	assert.True(t, prod.DisableNotes)
	assert.Empty(t, prod.Platforms)

	dev, err := buildReleasePipelineConfig(exectest.NewMockRunner(), releaseReads(), cfg, "dev", "", false, false)
	require.NoError(t, err)
	assert.False(t, dev.DisableChangelog)
	assert.False(t, dev.DisableNotes)
}

// An environment's disable_changelog and the tag type reach the changelog pipeline too.
func TestBuildChangelogPipelineConfig_Wiring(t *testing.T) {
	testutil.ClearCIEnv(t)
	cfg := wiringCfg()
	cfg.Versioning.Strategy = "semver-per-env"
	cfg.Versioning.TagFormat = "{env}/{version}"
	cfg.Environments = map[string]config.Environment{"staging": {Bump: "auto", DisableChangelog: true}, "dev": {Bump: "auto"}}

	staging, err := buildChangelogPipelineConfig(noOrigin(), noOrigin(), cfg, PipelineOpts{Out: &bytes.Buffer{}, Env: "staging"}, false)
	require.NoError(t, err)
	assert.True(t, staging.DisableChangelog)
	assert.True(t, staging.AnnotatedTags, "annotated by default")

	cfg.Versioning.TagType = "lightweight"
	dev, err := buildChangelogPipelineConfig(noOrigin(), noOrigin(), cfg, PipelineOpts{Out: &bytes.Buffer{}, Env: "dev"}, false)
	require.NoError(t, err)
	assert.False(t, dev.DisableChangelog)
	assert.False(t, dev.AnnotatedTags)
}

// A target's own assets win; release.assets applies only to a target that declares none.
func TestTargetPlatformConfig_Assets(t *testing.T) {
	cfg := wiringCfg()
	cfg.Release = &config.Release{Assets: []string{"dist/*"}, AllowMissingAssets: true}
	id := port.ForgeIdentity{Type: "github", Host: "https://github.com", Project: "acme/app"}

	own := targetPlatformConfig(cfg, config.Target{Assets: []string{"build/app.zip"}}, config.Forge{Name: "gh"}, id, cfg.Release.Assets)
	assert.Equal(t, []string{"build/app.zip"}, own.Assets, "a target's own assets are kept")
	assert.True(t, own.AllowMissingAssets)

	inherited := targetPlatformConfig(cfg, config.Target{}, config.Forge{Name: "gh"}, id, cfg.Release.Assets)
	assert.Equal(t, []string{"dist/*"}, inherited.Assets)
}
