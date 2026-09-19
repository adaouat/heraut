package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfig_HooksAccessors_NilSafe(t *testing.T) {
	cfg := &config.Config{}

	assert.Nil(t, cfg.PostBumpHooks())
	assert.Nil(t, cfg.PreChangelogHooks())
	assert.Nil(t, cfg.PreTagHooks())
	assert.Nil(t, cfg.PostTagHooks())
	assert.Nil(t, cfg.PreReleaseHooks())
	assert.Nil(t, cfg.PostReleaseHooks())
}

func TestConfig_HooksAccessors_ReturnsConfiguredList(t *testing.T) {
	cfg := &config.Config{
		Hooks: &config.Hooks{
			PostBump:     []config.HookStep{{Run: "echo post_bump"}},
			PreChangelog: []config.HookStep{{Run: "echo pre_changelog"}},
			PreTag:       []config.HookStep{{Run: "echo pre_tag"}},
			PostTag:      []config.HookStep{{Run: "echo post_tag"}},
			PreRelease:   []config.HookStep{{Run: "echo pre_release"}},
			PostRelease:  []config.HookStep{{Run: "echo post_release"}},
		},
	}

	assert.Equal(t, []config.HookStep{{Run: "echo post_bump"}}, cfg.PostBumpHooks())
	assert.Equal(t, []config.HookStep{{Run: "echo pre_changelog"}}, cfg.PreChangelogHooks())
	assert.Equal(t, []config.HookStep{{Run: "echo pre_tag"}}, cfg.PreTagHooks())
	assert.Equal(t, []config.HookStep{{Run: "echo post_tag"}}, cfg.PostTagHooks())
	assert.Equal(t, []config.HookStep{{Run: "echo pre_release"}}, cfg.PreReleaseHooks())
	assert.Equal(t, []config.HookStep{{Run: "echo post_release"}}, cfg.PostReleaseHooks())
}

// TestHookStep_UnmarshalYAML covers ADR-0061: hook entries must be mappings ({run: ..., stage:
// ...}); the bare-string shorthand ADR-0053 originally allowed is rejected, and an unknown field
// inside a hook entry must be rejected too — proving forge/config.Decode's KnownFields(true)
// strictness survives HookStep's own UnmarshalYAML rather than being silently bypassed by it.
func TestHookStep_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		want    config.HookStep
		wantErr string
	}{
		{
			name: "object with run only",
			doc:  `run: "echo hi"`,
			want: config.HookStep{Run: "echo hi"},
		},
		{
			name: "object with run and stage",
			doc: `
run: "echo hi"
stage:
  - CHANGELOG.md
  - VERSION
`,
			want: config.HookStep{Run: "echo hi", Stage: []string{"CHANGELOG.md", "VERSION"}},
		},
		{
			name:    "bare string rejected",
			doc:     `"echo hi"`,
			wantErr: "expected a mapping",
		},
		{
			name: "unknown field rejected",
			doc: `
run: "echo hi"
stag:
  - CHANGELOG.md
`,
			wantErr: "field stag not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var step config.HookStep
			err := yaml.Unmarshal([]byte(tc.doc), &step)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, step)
		})
	}
}
