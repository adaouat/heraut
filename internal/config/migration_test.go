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

// release.platforms is deliberately NOT removed in this cut — it must still load.
func TestLoad_PlatformsStillSupported(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`))
	require.NoError(t, err)
}
