package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// heraut's own config refuses breaking-change commits, so a 2.0 is only ever cut on purpose
// through the Release workflow's version input.
func TestVerifyCommit_OwnConfigRefusesBreakingChanges(t *testing.T) {
	cfg, err := config.Load("../../.config/heraut.yml")
	require.NoError(t, err)
	require.Empty(t, config.Validate(cfg))

	tests := []struct {
		name    string
		message string
		refused bool
	}{
		{"plain feat", "feat(cmd): add a flag", false},
		{"plain fix without scope", "fix: handle an empty repo", false},
		{"body mentions breaking in prose", "fix: x\n\nThis is not a breaking change.", false},
		{"footer token other than BREAKING CHANGE", "fix: x\n\nRefs: #12", false},
		{"bang without scope", "feat!: drop a flag", true},
		{"bang with scope", "feat(cmd)!: drop a flag", true},
		{"BREAKING CHANGE footer", "feat: x\n\nBREAKING CHANGE: the flag is gone", true},
		{"BREAKING-CHANGE footer", "fix: x\n\nBREAKING-CHANGE: the flag is gone", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.VerifyCommit(cfg, tc.message)
			if !tc.refused {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "a major release is cut with the Release workflow")
		})
	}
}
