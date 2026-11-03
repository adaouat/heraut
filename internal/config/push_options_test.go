package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestPushOptions_LoadsThroughStrictLoader(t *testing.T) {
	cfg, err := config.Load("../../testdata/config/valid/push-options.yml")
	require.NoError(t, err)

	require.NotNil(t, cfg.Git)
	require.NotNil(t, cfg.Git.PushOptions)
	assert.Equal(t, []string{"ci.skip"}, cfg.Git.PushOptions.Commit)
	assert.Equal(t, []string{"ci.variable=DEPLOY=true"}, cfg.Git.PushOptions.Tag)

	staging := cfg.Environments["staging"].Git
	require.NotNil(t, staging)
	require.NotNil(t, staging.PushOptions)
	assert.Nil(t, staging.PushOptions.Commit, "an absent list stays nil")
	assert.NotNil(t, staging.PushOptions.Tag, "an explicit [] must stay distinguishable from absent")
	assert.Empty(t, staging.PushOptions.Tag)
}

func TestPushOptions_NotAListIsAParseError(t *testing.T) {
	_, err := config.Load("../../testdata/config/invalid/push_options_not_a_list.yml")
	require.Error(t, err)
}

func TestConfig_PushOptions(t *testing.T) {
	root := &config.Git{PushOptions: &config.PushOptions{
		Commit: []string{"ci.skip"},
		Tag:    []string{"ci.variable=DEPLOY=true"},
	}}
	tests := []struct {
		name       string
		root       *config.Git
		envs       map[string]config.Environment
		env        string
		wantCommit []string
		wantTag    []string
	}{
		{name: "no git block anywhere", env: "", wantCommit: nil, wantTag: nil},
		{name: "no git block, unknown environment", env: "prod", wantCommit: nil, wantTag: nil},
		{name: "git block without push_options", root: &config.Git{}, env: "", wantCommit: nil, wantTag: nil},
		{name: "root only, no environment", root: root, env: "", wantCommit: []string{"ci.skip"}, wantTag: []string{"ci.variable=DEPLOY=true"}},
		{
			name: "environment without a git block inherits both lists",
			root: root, envs: map[string]config.Environment{"prod": {}},
			env: "prod", wantCommit: []string{"ci.skip"}, wantTag: []string{"ci.variable=DEPLOY=true"},
		},
		{
			name: "environment overriding commit only, tag inherits",
			root: root,
			envs: map[string]config.Environment{"prod": {Git: &config.Git{PushOptions: &config.PushOptions{Commit: []string{"ci.variable=ENV=prod"}}}}},
			env:  "prod", wantCommit: []string{"ci.variable=ENV=prod"}, wantTag: []string{"ci.variable=DEPLOY=true"},
		},
		{
			name: "environment [] clears the root list",
			root: root,
			envs: map[string]config.Environment{"prod": {Git: &config.Git{PushOptions: &config.PushOptions{Commit: []string{}}}}},
			env:  "prod", wantCommit: nil, wantTag: []string{"ci.variable=DEPLOY=true"},
		},
		{
			name: "environment tag [] clears the root list, commit inherits",
			root: root,
			envs: map[string]config.Environment{"prod": {Git: &config.Git{PushOptions: &config.PushOptions{Tag: []string{}}}}},
			env:  "prod", wantCommit: []string{"ci.skip"}, wantTag: nil,
		},
		{
			name: "environment sets options the root lacks",
			envs: map[string]config.Environment{"dev": {Git: &config.Git{PushOptions: &config.PushOptions{Tag: []string{"ci.skip"}}}}},
			env:  "dev", wantCommit: nil, wantTag: []string{"ci.skip"},
		},
		{
			name: "another environment's override does not leak",
			root: root,
			envs: map[string]config.Environment{"dev": {Git: &config.Git{PushOptions: &config.PushOptions{Commit: []string{}}}}},
			env:  "prod", wantCommit: []string{"ci.skip"}, wantTag: []string{"ci.variable=DEPLOY=true"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Git: tc.root, Environments: tc.envs}
			got := cfg.PushOptions(tc.env)
			assert.Equal(t, tc.wantCommit, nilIfEmpty(got.Commit), "commit")
			assert.Equal(t, tc.wantTag, nilIfEmpty(got.Tag), "tag")
		})
	}
}

func TestConfig_PushOptions_NilConfig(t *testing.T) {
	var cfg *config.Config
	got := cfg.PushOptions("prod")
	assert.Empty(t, got.Commit)
	assert.Empty(t, got.Tag)
}

func TestValidate_PushOptions(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantPath string
		wantMsg  string
	}{
		{
			name:     "empty value at the root",
			yaml:     "git:\n  push_options:\n    commit: [\"ci.skip\", \"\"]\n",
			wantPath: "git.push_options.commit[1]",
			wantMsg:  "empty",
		},
		{
			name:     "newline at the root",
			yaml:     "git:\n  push_options:\n    tag: [\"ci.skip\\nx\"]\n",
			wantPath: "git.push_options.tag[0]",
			wantMsg:  "line break",
		},
		{
			name:     "carriage return",
			yaml:     "git:\n  push_options:\n    tag: [\"ci.skip\\r\"]\n",
			wantPath: "git.push_options.tag[0]",
			wantMsg:  "line break",
		},
		{
			name:     "NUL byte",
			yaml:     "git:\n  push_options:\n    commit: [\"ci\\0skip\"]\n",
			wantPath: "git.push_options.commit[0]",
			wantMsg:  "NUL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, "version: \"1\"\nversioning:\n  strategy: semver\n"+tc.yaml)
			errs := config.Validate(cfg)
			e := findErr(errs, tc.wantPath)
			require.NotNil(t, e, "expected an error on %q, got %v", tc.wantPath, errs)
			assert.Contains(t, e.Message, tc.wantMsg)
			assert.NotEmpty(t, e.Hint)
		})
	}
}

func TestValidate_PushOptions_PerEnvironment(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
    git:
      push_options:
        commit: ["ok", ""]
`)
	e := findErr(config.Validate(cfg), "environments.dev.git.push_options.commit[1]")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "empty")
}

func TestValidate_PushOptions_AcceptsAnyOtherValue(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
git:
  push_options:
    commit: ["ci.skip", "-starts-with-a-dash", "ci.variable=A=b c", "merge_request.create"]
    tag: []
`)
	assert.Empty(t, config.Validate(cfg))
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
