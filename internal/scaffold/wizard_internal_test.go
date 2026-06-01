package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSchemaURL(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"dev", "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"},
		{"", "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"},
		{"v1.2.3", "https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json"},
		{"v2.0.0", "https://raw.githubusercontent.com/adaouat/heraut/v2.0.0/schema.json"},
	}
	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			assert.Equal(t, tc.want, buildSchemaURL(tc.version))
		})
	}
}

func TestResolveFormatChoice_Empty(t *testing.T) {
	choice, custom := resolveFormatChoice("")
	assert.Equal(t, calverPresets[0].format, choice)
	assert.Equal(t, "", custom)
}

func TestResolveFormatChoice_KnownPreset(t *testing.T) {
	choice, custom := resolveFormatChoice("YYYY.MM.PATCH")
	assert.Equal(t, "YYYY.MM.PATCH", choice)
	assert.Equal(t, "", custom)
}

func TestResolveFormatChoice_AllPresets(t *testing.T) {
	for _, p := range calverPresets {
		if p.format == "custom" {
			continue
		}
		choice, custom := resolveFormatChoice(p.format)
		assert.Equal(t, p.format, choice, "preset %s should round-trip", p.format)
		assert.Equal(t, "", custom)
	}
}

func TestResolveFormatChoice_CustomFormat(t *testing.T) {
	choice, custom := resolveFormatChoice("YYYY.MM.DD.WW.PATCH")
	assert.Equal(t, "custom", choice)
	assert.Equal(t, "YYYY.MM.DD.WW.PATCH", custom)
}

func TestParseRemoteProject(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"ssh with .git", "git@github.com:owner/repo.git", "owner/repo"},
		{"ssh without .git", "git@github.com:owner/repo", "owner/repo"},
		{"https with .git", "https://github.com/owner/repo.git", "owner/repo"},
		{"https without .git", "https://github.com/owner/repo", "owner/repo"},
		{"gitlab ssh", "git@gitlab.com:namespace/project.git", "namespace/project"},
		{"gitlab https", "https://gitlab.com/namespace/project.git", "namespace/project"},
		{"nested gitlab groups", "git@gitlab.com:namespace/group/project.git", "namespace/group/project"},
		{"self-hosted https", "https://git.company.com/team/service.git", "team/service"},
		{"ssh scheme", "ssh://git@gitlab.com/namespace/project.git", "namespace/project"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseRemoteProject(tc.url))
		})
	}
}

func TestPlatformTokenDefault(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"github", "GH_TOKEN"},
		{"gitlab", "GITLAB_TOKEN"},
		{"", ""},
		{"other", ""},
	}
	for _, tc := range tests {
		t.Run(tc.platform, func(t *testing.T) {
			assert.Equal(t, tc.want, platformTokenDefault(tc.platform))
		})
	}
}
