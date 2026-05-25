package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/stretchr/testify/assert"
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
