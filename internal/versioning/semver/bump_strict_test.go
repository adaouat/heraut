package semver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

func TestBumpVersion_RejectsParts_NotPlainNumbers(t *testing.T) {
	tests := []struct {
		name    string
		current string
	}{
		{"leading zero in major", "01.2.3"},
		{"leading zero in minor", "1.02.3"},
		{"leading zero in patch", "1.2.03"},
		{"plus sign", "+1.2.3"},
		{"minus sign", "1.-2.3"},
		{"extra part", "1.2.3.4"},
		{"pre-release suffix", "1.2.3-rc.1"},
		{"empty part", "1..3"},
		{"overflow", "1.2.99999999999999999999"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := semver.BumpVersion(tc.current, versioning.BumpPatch)
			require.Error(t, err)
			assert.ErrorIs(t, err, semver.ErrInvalidVersion)
		})
	}
}

func TestBumpVersion_AcceptsZeroParts(t *testing.T) {
	got, err := semver.BumpVersion("0.0.0", versioning.BumpPatch)
	require.NoError(t, err)
	assert.Equal(t, "0.0.1", got)
}
