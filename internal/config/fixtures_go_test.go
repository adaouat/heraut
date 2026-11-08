package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// The Go loader and validator agree with the schema fixtures: every valid fixture loads and validates
// cleanly, and every invalid one is refused by one of them. The schema tests check the same fixtures
// against schema.json, so the two can't drift unnoticed.
func TestFixtures_TheGoLoaderAgreesWithTheSchema(t *testing.T) {
	valid, err := filepath.Glob("../../testdata/config/valid/*.yml")
	require.NoError(t, err)
	require.NotEmpty(t, valid)
	for _, f := range valid {
		t.Run("valid/"+filepath.Base(f), func(t *testing.T) {
			cfg, err := config.Load(f)
			require.NoError(t, err)
			assert.Empty(t, config.Validate(cfg))
		})
	}

	invalid, err := filepath.Glob("../../testdata/config/invalid/*.yml")
	require.NoError(t, err)
	require.NotEmpty(t, invalid)
	for _, f := range invalid {
		t.Run("invalid/"+filepath.Base(f), func(t *testing.T) {
			cfg, err := config.Load(f)
			if err != nil {
				return
			}
			assert.NotEmpty(t, config.Validate(cfg), "the loader and the validator both accept it")
		})
	}
}

// Every forges[].platform value has a valid fixture (testing.md).
func TestFixtures_EveryPlatformHasAValidFixture(t *testing.T) {
	valid, err := filepath.Glob("../../testdata/config/valid/*.yml")
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, f := range valid {
		cfg, err := config.Load(f)
		require.NoError(t, err)
		for _, fg := range cfg.Forges {
			seen[fg.Type] = true
		}
	}
	for _, platform := range []string{"github", "gitlab", "azure_devops"} {
		assert.True(t, seen[platform], "no valid fixture uses platform %s", platform)
	}
}

// A boolean key takes true or false only, as in the schema: yaml.v3 would read "yes", "on" and "off".
func TestLoad_BooleanKeysTakeOnlyTrueOrFalse(t *testing.T) {
	for _, v := range []string{`"yes"`, "yes", "on", "off", `"true"`} {
		_, err := config.LoadFromReader(strings.NewReader("version: \"1\"\nversioning:\n  strategy: semver\n  bump:\n    confirm_major: " + v + "\n"))
		require.Error(t, err, v)
		assert.Contains(t, err.Error(), "line 5: versioning.bump.confirm_major must be true or false", v)
	}
	_, err := config.LoadFromReader(strings.NewReader("version: \"1\"\nversioning:\n  strategy: semver\nenvironments:\n  prod:\n    bump: auto\n    disable_release: on\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environments.prod.disable_release must be true or false")
	_, err = config.LoadFromReader(strings.NewReader("version: \"1\"\nversioning:\n  strategy: semver\n  bump:\n    confirm_major: true\n"))
	assert.NoError(t, err)
}
