package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPromotionGuard(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("boom"), false},
		{"E001 target exists", perenv.ErrTargetExists, true},
		{"E002 destination ahead", perenv.ErrDestinationAhead, true},
		{"E003 no source tags", perenv.ErrNoSourceTags, true},
		{"wrapped E001", fmt.Errorf("resolving version: %w", perenv.ErrTargetExists), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, app.IsPromotionGuard(tc.err))
		})
	}
}

func TestIsBuildIDRequired(t *testing.T) {
	// A real tagfmt.Render call, not a hand-built copy of the sentinel — closes the gap where a
	// future change to Render could stop returning ErrBuildIDRequired without this table noticing
	// (the other rows all use the sentinel directly or a wrapped copy of it).
	_, renderErr := tagfmt.Render("{env}/{version}-{build}", tagfmt.Tokens{Env: "uat", Version: "1.0.0"})
	require.Error(t, renderErr)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sentinel", tagfmt.ErrBuildIDRequired, true},
		{
			"sentinel wrapped twice",
			fmt.Errorf("resolving version: %w", fmt.Errorf("rendering tag: %w", tagfmt.ErrBuildIDRequired)),
			true,
		},
		{"unrelated", errors.New("boom"), false},
		{"promotion guard", fmt.Errorf("resolving version: %w", perenv.ErrTargetExists), false},
		{"error from a real tagfmt.Render call, not a hand-built sentinel", renderErr, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, app.IsBuildIDRequired(tc.err))
		})
	}
}
