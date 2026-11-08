package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

const ignoredKeysBase = "version: \"1\"\nversioning:\n  strategy: semver\n"

// A key heraut would ignore is a config error, not a silent no-op: a typo inside a nested list
// fails to load, and a key that doesn't apply to the platform fails validation, naming the right one.
func TestConfig_KeysHerautWouldIgnoreAreErrors(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		loadErr  string // the loader refuses it
		path     string // else validation reports this path…
		contains string // …with this text
	}{
		{name: "unknown key in rendering.excludes",
			yaml: "rendering:\n  excludes:\n    - type: chore\n      typo_key: oops\n", loadErr: "typo_key"},
		{name: "unknown key in a per-driver exclude",
			yaml: "changelog:\n  rendering:\n    excludes:\n      - regx: '^x'\n", loadErr: "regx"},
		{name: "unknown key in a commit trailer",
			yaml: "rendering:\n  templates:\n    commit:\n      trailers:\n        - token: Signed-off-by\n          rendrer: bogus\n", loadErr: "rendrer"},
		{name: "project on a GitHub forge",
			yaml: "forges:\n  - name: gh\n    platform: github\n    project: acme/app\n",
			path: "forges[0].project", contains: "GitHub uses repository"},
		{name: "repository on a GitLab forge",
			yaml: "forges:\n  - name: gl\n    platform: gitlab\n    repository: acme/app\n",
			path: "forges[0].repository", contains: "GitLab uses project"},
		{name: "draft on a GitLab target",
			yaml: "forges:\n  - name: gl\n    platform: gitlab\n    project: acme/app\nrelease:\n  targets:\n    - forge: gl\n      draft: true\n",
			path: "release.targets[0].draft", contains: "GitHub only"},
		{name: "draft on an environment's GitLab target",
			yaml: "forges:\n  - name: gl\n    platform: gitlab\n    project: acme/app\nrelease: {}\nenvironments:\n  prod:\n    bump: auto\n    release:\n      targets:\n        - forge: gl\n          draft: true\n",
			path: "environments.prod.release.targets[0].draft", contains: "GitHub only"},
		{name: "output on the release notes",
			yaml: "release:\n  notes:\n    output: NOTES.md\n",
			path: "release.notes.output", contains: "not written to a file"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := ignoredKeysBase + tc.yaml
			if strings.Contains(tc.yaml, "environments:") {
				yaml = strings.Replace(yaml, "strategy: semver\n", "strategy: semver-per-env\n  tag_format: \"{env}/{version}\"\n", 1)
			}
			cfg, err := config.LoadFromReader(strings.NewReader(yaml))
			if tc.loadErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.loadErr)
				return
			}
			require.NoError(t, err)
			var found *config.ValidationError
			for _, ve := range config.Validate(cfg) {
				if ve.Path == tc.path {
					found = &ve
				}
			}
			require.NotNil(t, found, "expected a validation error at %s, got %v", tc.path, config.Validate(cfg))
			assert.Contains(t, found.Message+" "+found.Hint, tc.contains)
		})
	}
}

// The keys that do apply stay valid: repository on GitHub, project on GitLab, both on Azure DevOps,
// draft on GitHub.
func TestConfig_KeysThatApplyStayValid(t *testing.T) {
	yaml := ignoredKeysBase + `forges:
  - name: gh
    platform: github
    repository: acme/app
  - name: gl
    platform: gitlab
    project: acme/app
  - name: az
    platform: azure_devops
    project: acme/app
    repository: app
commits:
  enrichment_forge: gh
release:
  targets:
    - forge: gh
      draft: true
    - forge: gl
`
	cfg, err := config.LoadFromReader(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Empty(t, config.Validate(cfg))
}

// A hook template that would fail when its hook runs fails validation instead, before anything is
// tagged or published: an unknown field, or a template that doesn't parse.
func TestConfig_HookTemplatesAreChecked(t *testing.T) {
	tests := []struct {
		name, hooks, path, contains string
	}{
		{"unknown field in run", "  post_tag:\n    - run: \"echo {{ .Tga }}\"\n", "hooks.post_tag[0].run", "Tga"},
		{"unparsable run", "  pre_release:\n    - run: \"echo {{ .Tag \"\n", "hooks.pre_release[0].run", "template"},
		{"unknown field in stage", "  post_bump:\n    - run: \"make\"\n      stage: [\"dist/{{ .Release }}\"]\n", "hooks.post_bump[0].stage[0]", "Release"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.LoadFromReader(strings.NewReader(ignoredKeysBase + "changelog:\n  output: CHANGELOG.md\nhooks:\n" + tc.hooks))
			require.NoError(t, err)
			var found *config.ValidationError
			for _, ve := range config.Validate(cfg) {
				if ve.Path == tc.path {
					found = &ve
				}
			}
			require.NotNil(t, found, "expected a validation error at %s, got %v", tc.path, config.Validate(cfg))
			assert.Contains(t, found.Message, tc.contains)
		})
	}

	valid, err := config.LoadFromReader(strings.NewReader(ignoredKeysBase + "changelog:\n  output: CHANGELOG.md\nhooks:\n  post_bump:\n    - run: \"echo {{ .Version }} {{ .Tag }} {{ .PreviousTag }} {{ .Env }}\"\n      stage: [\"dist/{{ .Version }}\"]\n  post_release:\n    - run: \"notify {{ .Platform }}\"\n"))
	require.NoError(t, err)
	assert.Empty(t, config.Validate(valid), "every documented field renders")
}
