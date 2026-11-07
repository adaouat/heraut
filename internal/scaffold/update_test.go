package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
)

// Updating a config through the wizard, answers unchanged, rewrites the same configuration: every key
// the wizard doesn't ask about is kept, whatever it is. Each valid fixture is a config a user might
// update.
func TestGenerateUpdatedYAML_UnchangedAnswersKeepTheConfig(t *testing.T) {
	fixtures, err := filepath.Glob("../../testdata/config/valid/*.yml")
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			orig, err := config.Load(fixture)
			require.NoError(t, err)

			content, err := scaffold.GenerateUpdatedYAML(orig, scaffold.ConfigToAnswers(orig), "dev")
			require.NoError(t, err)

			out := filepath.Join(t.TempDir(), ".heraut.yml")
			require.NoError(t, os.WriteFile(out, []byte(content), 0o600))
			got, err := config.Load(out)
			require.NoError(t, err, "the rewritten config loads:\n%s", content)
			want, err := config.Load(fixture)
			require.NoError(t, err)
			assert.Equal(t, want, normalizePrefix(got, want), "rewritten:\n%s", content)
		})
	}
}

// normalizePrefix accepts an explicit tag_prefix equal to the strategy default where the original
// left it unset: the wizard asks for the prefix, so it writes the value it shows.
func normalizePrefix(got, want *config.Config) *config.Config {
	if want.Versioning.TagPrefix == nil && got.Versioning.TagPrefix != nil {
		def := "v"
		if want.Versioning.Strategy == "calver" || want.Versioning.Strategy == "calver-per-env" {
			def = ""
		}
		if *got.Versioning.TagPrefix == def {
			got.Versioning.TagPrefix = nil
		}
	}
	return got
}

// A config without tag_prefix is offered its strategy's default, so accepting the wizard as is keeps
// the tags heraut reads.
func TestConfigToAnswers_UnsetPrefixIsTheStrategyDefault(t *testing.T) {
	for strategy, want := range map[string]string{"semver": "v", "semver-per-env": "v", "calver": "", "calver-per-env": ""} {
		a := scaffold.ConfigToAnswers(&config.Config{Version: "1", Versioning: config.Versioning{Strategy: strategy}})
		assert.Equal(t, want, a.TagPrefix, strategy)
	}
}

// Changed answers replace what the wizard asks about, and only that.
func TestGenerateUpdatedYAML_ChangedAnswersKeepTheRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`version: "1"
versioning:
  strategy: semver
  bump:
    stay_at_v0: true
hooks:
  post_tag:
    - run: echo {{ .Tag }}
git:
  push_options:
    commit: [ci.skip]
changelog:
  output: CHANGELOG.md
forges:
  - name: github
    platform: github
release:
  allow_missing_assets: true
  targets:
    - assets: ["dist/*"]
`), 0o600))
	orig, err := config.Load(path)
	require.NoError(t, err)
	a := scaffold.ConfigToAnswers(orig)
	a.TagPrefix = ""
	a.ChangelogOutput = "HISTORY.md"

	content, err := scaffold.GenerateUpdatedYAML(orig, a, "dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	got, err := config.Load(path)
	require.NoError(t, err, content)

	require.NotNil(t, got.Versioning.TagPrefix)
	assert.Empty(t, *got.Versioning.TagPrefix, "the new prefix")
	assert.Equal(t, "HISTORY.md", got.Changelog.Output, "the new output")
	assert.Equal(t, orig.Versioning.Bump, got.Versioning.Bump, "stay_at_v0 kept")
	assert.Equal(t, orig.Hooks, got.Hooks, "hooks kept")
	assert.Equal(t, orig.Git, got.Git, "push options kept")
	assert.True(t, got.Release.AllowMissingAssets, "allow_missing_assets kept")
	assert.Equal(t, []string{"dist/*"}, got.Release.Targets[0].Assets, "the target's assets kept")
}

// A release turned off in the wizard drops the release block and its targets.
func TestGenerateUpdatedYAML_ReleaseTurnedOff(t *testing.T) {
	orig := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{AllowMissingAssets: true, Targets: []config.Target{{Assets: []string{"dist/*"}}}}}
	a := scaffold.ConfigToAnswers(orig)
	a.PublishReleases = false

	content, err := scaffold.GenerateUpdatedYAML(orig, a, "dev")

	require.NoError(t, err)
	assert.NotContains(t, content, "release:")
}
