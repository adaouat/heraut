package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

func TestExcludeMatcher(t *testing.T) {
	m := config.NewExcludeMatcher(config.EffectiveExcludes([]config.Exclude{
		{Type: "wip"},
		{Regex: `^fix: typo`},
		{Regex: `([`}, // invalid: skipped, never a panic
	}))
	tests := []struct {
		name    string
		subject string
		typ     string
		want    bool
	}{
		{"default: release commit", "chore(release): 1.2.3", "chore", true},
		{"default: dependency noise", "chore(deps): bump x", "chore", true},
		{"default: deps with a suffix scope", "chore(deps-dev): bump y", "chore", true},
		{"user type exclude", "wip: half-done", "wip", true},
		{"user regex exclude on the subject", "fix: typo in readme", "fix", true},
		{"a normal commit", "feat: add login", "feat", false},
		{"same type, other scope, not excluded", "chore(ci): tweak", "chore", false},
		{"non-conventional commit", "just a plain message", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, m.Excluded(tc.subject, tc.typ))
		})
	}
}
