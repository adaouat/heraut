package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "heraut.yml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestLoad_RemovedKeys(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantHint string
	}{
		{
			name: "changelog.remote",
			body: `version: "1"
versioning: {strategy: semver}
changelog:
  generator: native
  output: CHANGELOG.md
  remote:
    type: gitlab
    project: group/subgroup/project
`,
			wantHint: "forges:",
		},
		{
			name: "commits.remote_metadata",
			body: `version: "1"
versioning: {strategy: semver}
commits:
  remote_metadata: required
`,
			wantHint: "enrichment_policy",
		},
		{
			name: "environments.<env>.changelog.remote",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    changelog:
      generator: native
      output: CHANGELOG.md
      remote:
        type: gitlab
        project: group/subgroup/project
`,
			wantHint: "forges:",
		},
		{
			name: "changelog.generator",
			body: `version: "1"
versioning: {strategy: semver}
changelog:
  generator: native
  output: CHANGELOG.md
`,
			wantHint: "native is heraut's only generator",
		},
		{
			name: "changelog.config",
			body: `version: "1"
versioning: {strategy: semver}
changelog:
  config: cliff.toml
`,
			wantHint: "rendering.templates",
		},
		{
			name: "release.notes.generator",
			body: `version: "1"
versioning: {strategy: semver}
release:
  notes:
    generator: native
`,
			wantHint: "native is heraut's only generator",
		},
		{
			name: "release.notes.config",
			body: `version: "1"
versioning: {strategy: semver}
release:
  notes:
    config: comm.yaml
`,
			wantHint: "rendering.templates",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
			assert.Contains(t, err.Error(), tc.wantHint, "the error must name the replacement")
		})
	}
}

// TestLoad_RemovedKeys_PerEnvChangelogRemote checks the per-env removed-key error names the
// specific environment in its path, distinguishing it from the top-level changelog.remote case.
func TestLoad_RemovedKeys_PerEnvChangelogRemote(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    changelog:
      generator: native
      output: CHANGELOG.md
      remote:
        type: gitlab
        project: group/subgroup/project
`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
	assert.Contains(t, err.Error(), "staging", "the error must name which environment carries the removed key")
	assert.Contains(t, err.Error(), "forges:", "the error must name the replacement")
}

// TestLoad_RemovedKeys_PerEnvGenerator checks the per-env removed-key error for
// environments.<env>.changelog.generator and environments.<env>.release.notes.generator, names
// the specific environment, and carries the same hint as the top-level case.
func TestLoad_RemovedKeys_PerEnvGenerator(t *testing.T) {
	tests := []struct{ name, body string }{
		{
			name: "changelog.generator",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    changelog:
      generator: native
      output: CHANGELOG.md
`,
		},
		{
			name: "release.notes.generator",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      notes:
        generator: native
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
			assert.Contains(t, err.Error(), "staging", "the error must name which environment carries the removed key")
			assert.Contains(t, err.Error(), "native is heraut's only generator", "the hint must be present")
		})
	}
}

// TestLoad_RemovedKeys_PerEnvConfig mirrors TestLoad_RemovedKeys_PerEnvGenerator for the
// config: key (external generator config file path — meaningless without git-cliff/communique).
func TestLoad_RemovedKeys_PerEnvConfig(t *testing.T) {
	tests := []struct{ name, body string }{
		{
			name: "changelog.config",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    changelog:
      config: cliff.toml
`,
		},
		{
			name: "release.notes.config",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      notes:
        config: comm.yaml
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
			assert.Contains(t, err.Error(), "staging", "the error must name which environment carries the removed key")
			assert.Contains(t, err.Error(), "rendering.templates", "the hint must point at the native replacement")
		})
	}
}

func TestLoad_RemovedKey_ReleasePlatforms(t *testing.T) {
	tests := []struct{ name, body string }{
		{
			name: "top-level",
			body: `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`,
		},
		{
			name: "per-environment",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      platforms:
        - name: gl
          platform: gitlab
          project: group/subgroup/project
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey))
			assert.Contains(t, err.Error(), "release.targets", "the error must name the replacement")
			assert.Contains(t, err.Error(), "forges:", "and where the coordinates move to")
		})
	}
}

// The hint must name every field a forges: entry REQUIRES, or a user following it literally hits a
// second round of validation errors.
func TestLoad_RemovedKey_ReleasePlatformsHintNamesRequiredFields(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`))
	require.Error(t, err)
	require.True(t, errors.Is(err, config.ErrRemovedConfigKey))
	assert.Contains(t, err.Error(), "name", "the hint must name the required `name` field")
	assert.Contains(t, err.Error(), "`name` / `platform` (required)", "the hint must mark `name`/`platform` as required")
	assert.Contains(t, err.Error(), "release.targets")
}

// The per-env message must additionally say forges: is top-level only.
func TestLoad_RemovedKey_PerEnvHintSaysForgesIsTopLevel(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      platforms:
        - name: gl
          platform: gitlab
          project: group/subgroup/project
`))
	require.Error(t, err)
	require.True(t, errors.Is(err, config.ErrRemovedConfigKey))
	assert.Contains(t, err.Error(), "top-level")
}
