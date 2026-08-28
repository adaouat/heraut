package semver_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMajorMinor_BareVersion(t *testing.T) {
	major, minor, err := semver.MajorMinor("1.4.2")
	require.NoError(t, err)
	assert.Equal(t, 1, major)
	assert.Equal(t, 4, minor)
}

func TestMajorMinor_IgnoresPreReleaseAndBuildMetadata(t *testing.T) {
	// A manual --version override can carry a pre-release/build suffix (heraut is
	// strategy-agnostic about override shape — tagfmt.ValidateVersionOverride only checks for
	// whitespace). MAJOR/MINOR must still extract cleanly since the suffix always attaches to
	// PATCH, never to MAJOR or MINOR.
	tests := []struct {
		name    string
		version string
		major   int
		minor   int
	}{
		{"pre-release", "2.7.0-rc.1", 2, 7},
		{"build metadata", "2.7.0+build.5", 2, 7},
		{"pre-release and build metadata", "2.7.0-rc.1+build.5", 2, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			major, minor, err := semver.MajorMinor(tc.version)
			require.NoError(t, err)
			assert.Equal(t, tc.major, major)
			assert.Equal(t, tc.minor, minor)
		})
	}
}

func TestMajorMinor_ZeroValues(t *testing.T) {
	major, minor, err := semver.MajorMinor("0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 0, major)
	assert.Equal(t, 0, minor)
}

func TestMajorMinor_InvalidVersion_Error(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty", ""},
		{"too few components", "1.4"},
		{"non-numeric major", "a.4.2"},
		{"non-numeric minor", "1.b.2"},
		{"not a version at all", "not-a-version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := semver.MajorMinor(tc.version)
			require.Error(t, err)
		})
	}
}
