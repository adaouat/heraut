package scaffold_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
)

func TestDefaults_Strategy(t *testing.T) {
	a := scaffold.Defaults()
	assert.Equal(t, "semver", a.Strategy)
}

func TestDefaults_Prefix(t *testing.T) {
	a := scaffold.Defaults()
	assert.Equal(t, "v", a.TagPrefix)
}

func TestDefaults_Generator(t *testing.T) {
	a := scaffold.Defaults()
	assert.Equal(t, "git-cliff", a.ChangelogGenerator)
}

func TestDefaults_Platform(t *testing.T) {
	a := scaffold.Defaults()
	assert.Len(t, a.Platforms, 1)
	assert.Equal(t, "gitlab", a.Platforms[0].Type)
}

func TestConfigToAnswers_Strategy(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "calver", a.Strategy)
}

func TestConfigToAnswers_Prefix(t *testing.T) {
	prefix := "v"
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "v", a.TagPrefix)
}

func TestConfigToAnswers_NilPrefix(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "", a.TagPrefix)
}

func TestConfigToAnswers_CalVerFormat(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "YYYY.MM.PATCH", a.Format)
}

func TestConfigToAnswers_ChangelogGenerator(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Generator: "communique", Output: "CHANGELOG.md"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "communique", a.ChangelogGenerator)
	assert.Equal(t, "CHANGELOG.md", a.ChangelogOutput)
}

func TestConfigToAnswers_NoChangelog(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "", a.ChangelogGenerator)
}

func TestConfigToAnswers_Platforms(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Type: "github", Repository: "acme/widget", TokenEnv: "GH_TOKEN"},
				{Type: "gitlab", Project: "acme/widget"},
			},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Len(t, a.Platforms, 2)
	assert.Equal(t, "github", a.Platforms[0].Type)
	assert.Equal(t, "acme/widget", a.Platforms[0].Repository)
	assert.Equal(t, "GH_TOKEN", a.Platforms[0].TokenEnv)
	assert.Equal(t, "gitlab", a.Platforms[1].Type)
}

func TestConfigToAnswers_Environments(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto"},
			"prod": {Bump: "promote", Source: "dev"},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "{env}/{version}", a.TagFormat)
	assert.Len(t, a.Environments, 2)
	names := map[string]bool{}
	for _, e := range a.Environments {
		names[e.Name] = true
	}
	assert.True(t, names["dev"])
	assert.True(t, names["prod"])
}

func TestConfigToAnswers_Sprint(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.SPRINT.PATCH", Sprint: 5},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, 5, a.Sprint)
}

func TestConfigToAnswers_SprintZeroWhenAbsent(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, 0, a.Sprint)
}

func TestConfigToAnswers_NotesGenerator(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Notes: &config.ContentDriver{Generator: "git-cliff"},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "git-cliff", a.NotesGenerator)
}

func TestValidateCalVerFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{"empty", "", "required"},
		{"missing PATCH", "YYYY.MM", "must contain PATCH"},
		{"PATCH not last", "PATCH.YYYY.MM", "PATCH must be the last"},
		{"no calendar token", "PATCH", "at least one calendar token"},
		{"unknown uppercase token", "YYYY.YYY.PATCH", `unknown token "YYY"`},
		{"unknown uppercase OOOO", "YYYY.OOOO.PATCH", `unknown token "OOOO"`},
		{"valid YYYY.MM.PATCH", "YYYY.MM.PATCH", ""},
		{"valid YYYY.WW.PATCH", "YYYY.WW.PATCH", ""},
		{"valid YYYY.SPRINT.PATCH", "YYYY.SPRINT.PATCH", ""},
		{"valid YYYY.PATCH", "YYYY.PATCH", ""},
		{"literal separator ok", "YYYY-MM.PATCH", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := scaffold.ValidateCalVerFormat(tc.format)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
