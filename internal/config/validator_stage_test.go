package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

const (
	stageHooks = `hooks:
  post_bump:
    - run: "echo hi"
      stage:
        - VERSION
  pre_changelog:
    - run: "echo lint"
      stage:
        - lint.txt
`
	topChangelog = "changelog:\n  output: CHANGELOG.md\n"
	devOff       = "  dev:\n    bump: auto\n    tag_format: \"dev/{version}\"\n    disable_changelog: true\n"
	prodPlain    = "  prod:\n    bump: auto\n    tag_format: \"prod/{version}\"\n"
	devOwn       = "  dev:\n    bump: auto\n    tag_format: \"dev/{version}\"\n    changelog:\n      output: CHANGELOG.md\n"
	devPlain     = "  dev:\n    bump: auto\n    tag_format: \"dev/{version}\"\n"
)

// ADR-0061 addendum: hook `stage` needs a changelog commit to join somewhere in the config.
func TestValidateHooks_StageNeedsAChangelogSomewhere(t *testing.T) {
	semver := "version: \"1\"\nversioning:\n  strategy: semver\n"
	perEnv := "version: \"1\"\nversioning:\n  strategy: semver-per-env\n"
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{"no changelog anywhere", semver + stageHooks, true},
		{"top-level changelog", semver + topChangelog + stageHooks, false},
		{"per-env: every environment disables the changelog", perEnv + "environments:\n" + devOff + prodPlain + "    disable_changelog: true\n" + stageHooks, true},
		{"per-env: every environment disables it, even with a top-level changelog", perEnv + topChangelog + "environments:\n" + devOff + stageHooks, true},
		{"per-env: one environment disables it, the other keeps the top-level changelog", perEnv + topChangelog + "environments:\n" + devOff + prodPlain + stageHooks, false},
		{"per-env: no top-level changelog but one environment configures its own", perEnv + "environments:\n" + devOwn + stageHooks, false},
		{"per-env: no changelog in any environment", perEnv + "environments:\n" + devPlain + stageHooks, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := config.Validate(mustLoad(t, tc.yaml))
			post := findErr(errs, "hooks.post_bump[0].stage")
			pre := findErr(errs, "hooks.pre_changelog[0].stage")
			if !tc.wantErr {
				assert.Nil(t, post, "%v", errs)
				assert.Nil(t, pre, "%v", errs)
				return
			}
			require.NotNil(t, post, "%v", errs)
			require.NotNil(t, pre, "%v", errs)
			assert.Contains(t, post.Message, "no changelog commit")
			assert.NotEmpty(t, post.Hint)
		})
	}
}

func TestValidateHooks_NoStageNeedsNoChangelog(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
hooks:
  post_bump:
    - run: "echo hi"
`)
	assert.Empty(t, config.Validate(cfg))
}
