package versioning_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/versioning"
)

func TestBumpType_String(t *testing.T) {
	tests := []struct {
		bump versioning.BumpType
		want string
	}{
		{versioning.BumpNone, "none"},
		{versioning.BumpPatch, "patch"},
		{versioning.BumpMinor, "minor"},
		{versioning.BumpMajor, "major"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.bump.String())
	}
}
