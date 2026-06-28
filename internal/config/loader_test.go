package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestLoadFromReader_semver(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump: auto
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	assert.Equal(t, "1", cfg.Version)
	assert.Equal(t, "semver", cfg.Versioning.Strategy)
	require.NotNil(t, cfg.Versioning.TagPrefix)
	assert.Equal(t, "v", *cfg.Versioning.TagPrefix)
	assert.Equal(t, "0.1.0", cfg.Versioning.InitialVersion)
	assert.Equal(t, "auto", cfg.Versioning.Bump)
}

func TestLoadFromReader_calver(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	assert.Equal(t, "calver", cfg.Versioning.Strategy)
	assert.Equal(t, "YYYY.MM.PATCH", cfg.Versioning.Format)
	// Explicit empty prefix is distinguishable from unset.
	require.NotNil(t, cfg.Versioning.TagPrefix)
	assert.Equal(t, "", *cfg.Versioning.TagPrefix)
}

func TestLoadFromReader_prefixUnset(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	// Unset prefix is nil (distinct from explicit "").
	assert.Nil(t, cfg.Versioning.TagPrefix)
}

func TestLoadFromReader_semverPerEnv(t *testing.T) {
	src := `
version: "1"
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
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	assert.Equal(t, "semver-per-env", cfg.Versioning.Strategy)
	assert.Equal(t, "{env}/{version}", cfg.Versioning.TagFormat)
	require.Contains(t, cfg.Environments, "dev")
	require.Contains(t, cfg.Environments, "prod")
	assert.Equal(t, "auto", cfg.Environments["dev"].Bump)
	assert.Equal(t, "promote", cfg.Environments["prod"].Bump)
	assert.Equal(t, "dev", cfg.Environments["prod"].Source)
}

func TestLoadFromReader_calverPerEnv(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	assert.Equal(t, "calver-per-env", cfg.Versioning.Strategy)
	assert.Equal(t, "dev/{version}", cfg.Environments["dev"].TagFormat)
	assert.Equal(t, "prod/{version}", cfg.Environments["prod"].TagFormat)
}

func TestLoadFromReader_withChangelog(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  output: CHANGELOG.md
  tag_pattern: "dev/*"
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	require.NotNil(t, cfg.Changelog)
	assert.Equal(t, "git-cliff", cfg.Changelog.Generator)
	assert.Equal(t, "CHANGELOG.md", cfg.Changelog.Output)
	assert.Equal(t, "dev/*", cfg.Changelog.TagPattern)
}

func TestLoadFromReader_withRelease(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
release:
  notes:
    generator: git-cliff
  platforms:
    - platform: github
      repository: acme/widget
      token_env: GH_TOKEN
      draft: true
    - platform: gitlab
      project: acme/widget
      token_env: GITLAB_TOKEN
      assets:
        - dist/myapp_*
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	require.NotNil(t, cfg.Release)
	require.NotNil(t, cfg.Release.Notes)
	assert.Equal(t, "git-cliff", cfg.Release.Notes.Generator)
	require.Len(t, cfg.Release.Platforms, 2)

	gh := cfg.Release.Platforms[0]
	assert.Equal(t, "github", gh.Type)
	assert.Equal(t, "acme/widget", gh.Repository)
	assert.Equal(t, "GH_TOKEN", gh.TokenEnv)
	assert.True(t, gh.Draft)

	gl := cfg.Release.Platforms[1]
	assert.Equal(t, "gitlab", gl.Type)
	assert.Equal(t, "acme/widget", gl.Project)
	assert.Equal(t, []string{"dist/myapp_*"}, gl.Assets)
}

func TestLoadFromReader_withEnvOverrides(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
    release:
      platforms:
        - platform: gitlab
  prod:
    bump: promote
    release:
      platforms:
        - platform: gitlab
        - platform: github
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	require.Contains(t, cfg.Environments, "dev")
	require.Contains(t, cfg.Environments, "prod")
	require.NotNil(t, cfg.Environments["dev"].Release)
	require.Len(t, cfg.Environments["dev"].Release.Platforms, 1)
	assert.Equal(t, "gitlab", cfg.Environments["dev"].Release.Platforms[0].Type)
	require.Len(t, cfg.Environments["prod"].Release.Platforms, 2)
}

func TestLoadFromReader_withEnvVersioningDisableFlags(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    disable_changelog: true
    disable_notes: true
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	dev := cfg.Environments["dev"]
	assert.True(t, dev.DisableChangelog)
	assert.True(t, dev.DisableNotes)
}

func TestLoadFromReader_rejectsUnknownTopLevelKey(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
unknown_field: true
`
	_, err := config.LoadFromReader(strings.NewReader(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown_field")
}

func TestLoadFromReader_rejectsUnknownNestedKey(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
  typo_key: true
`
	_, err := config.LoadFromReader(strings.NewReader(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "typo_key")
}

func TestLoadFromReader_rejectsUnknownPlatformKey(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      completely_unknown: true
`
	_, err := config.LoadFromReader(strings.NewReader(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completely_unknown")
}

func TestLoad_fileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/heraut.yml")
	require.Error(t, err)
}

func TestLoad_fromFixtures(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		strategy string
	}{
		{"semver", "../../testdata/config/valid/semver.yml", "semver"},
		{"calver", "../../testdata/config/valid/calver.yml", "calver"},
		{"semver-per-env", "../../testdata/config/valid/semver-per-env.yml", "semver-per-env"},
		{"calver-per-env", "../../testdata/config/valid/calver-per-env.yml", "calver-per-env"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Load(tc.path)
			require.NoError(t, err)
			assert.Equal(t, tc.strategy, cfg.Versioning.Strategy)
		})
	}
}

func TestLoad_invalidFixture(t *testing.T) {
	_, err := config.Load("../../testdata/config/invalid/unknown_key.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown_field")
}

func TestLoadFromReader_TypeMismatch(t *testing.T) {
	// A YAML map into an integer field triggers a yaml.TypeError
	src := `
version: "1"
versioning:
  strategy: semver
  sprint:
    nested: value
`
	_, err := config.LoadFromReader(strings.NewReader(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config:")
}

func TestLoadFromReader_MalformedYAML(t *testing.T) {
	// Completely malformed YAML triggers a non-TypeError error (EOF or parse error)
	_, err := config.LoadFromReader(strings.NewReader("{{{{invalid yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config:")
}

func TestLoadFromReader_DefaultsChangelogOutput(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "empty string",
			src: `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  output: ""
`,
		},
		{
			name: "omitted field",
			src: `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.LoadFromReader(strings.NewReader(tc.src))
			require.NoError(t, err)
			require.NotNil(t, cfg.Changelog)
			assert.Equal(t, "CHANGELOG.md", cfg.Changelog.Output)
		})
	}
}

func TestLoadFromReader_ReleaseAssets(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms:
    - platform: github
      repository: owner/repo
  assets:
    - "dist/heraut_*_linux_amd64"
    - "dist/checksums.txt"
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	require.NotNil(t, cfg.Release)
	assert.Equal(t, []string{
		"dist/heraut_*_linux_amd64",
		"dist/checksums.txt",
	}, cfg.Release.Assets)
}

func TestLoadFromReader_ReleaseAssets_Empty(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms:
    - platform: github
      repository: owner/repo
`
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err)
	require.NotNil(t, cfg.Release)
	assert.Empty(t, cfg.Release.Assets)
}

func TestLoad_Tickets(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://acme.atlassian.net/browse/{ticket}'
`)
	require.Len(t, cfg.Tickets(), 1)
	assert.Equal(t, "[A-Z]+-[0-9]+", cfg.Tickets()[0].Pattern)
	assert.Equal(t, "https://acme.atlassian.net/browse/{ticket}", cfg.Tickets()[0].URL)
}

func TestLoad_CommitsScopes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
versioning:
  strategy: semver
commits:
  types:
    - name: feat
    - name: fix
  scopes: [cmd, config, versioning]
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Commits)
	assert.Equal(t, []string{"cmd", "config", "versioning"}, cfg.Commits.Scopes)
}
