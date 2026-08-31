package tagfmt_test

import (
	"regexp"
	"testing"

	"github.com/adaouat/heraut/internal/versioning/tagfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveTagPattern(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		env       string
		wantEmpty bool
		matches   []string
		noMatches []string
	}{
		{
			name:      "no env token returns empty",
			template:  "v{version}",
			env:       "prod",
			wantEmpty: true,
		},
		{
			name:      "empty env returns empty",
			template:  "{env}/{version}",
			env:       "",
			wantEmpty: true,
		},
		{
			name:      "version_env suffix",
			template:  "{version}_{env}",
			env:       "prod",
			matches:   []string{"2026.3.0_prod", "1.2.3_prod"},
			noMatches: []string{"2026.3.0_test", "2026.3.0_vali", "2026.3.0_preprod"},
		},
		{
			name:      "env/version prefix",
			template:  "{env}/{version}",
			env:       "prod",
			matches:   []string{"prod/1.2.3"},
			noMatches: []string{"dev/1.2.3", "staging/1.2.3"},
		},
		{
			name:      "env/version-build",
			template:  "{env}/{version}-{build}",
			env:       "uat",
			matches:   []string{"uat/7.4.1-158404", "uat/7.4.0-155398"},
			noMatches: []string{"main/7.4.1-159001"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := tagfmt.DeriveTagPattern(tc.template, tagfmt.Tokens{Env: tc.env})
			if tc.wantEmpty {
				assert.Empty(t, pat)
				return
			}
			require.NotEmpty(t, pat)
			re, err := regexp.Compile(pat)
			require.NoError(t, err, "derived pattern must compile: %s", pat)
			for _, tag := range tc.matches {
				assert.Truef(t, re.MatchString(tag), "pattern %q should match %q", pat, tag)
			}
			for _, tag := range tc.noMatches {
				assert.Falsef(t, re.MatchString(tag), "pattern %q should NOT match %q", pat, tag)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		template string
		env      string
		version  string
		want     string
	}{
		{"version only", "{version}", "", "1.2.3", "1.2.3"},
		{"prefix slash", "dev/{version}", "dev", "1.2.3", "dev/1.2.3"},
		{"suffix slash", "{version}/dev", "dev", "1.2.3", "1.2.3/dev"},
		{"prefix underscore", "dev_{version}", "dev", "1.2.3", "dev_1.2.3"},
		{"suffix underscore", "{version}_dev", "dev", "1.2.3", "1.2.3_dev"},
		{"env token", "{env}/{version}", "staging", "2.0.0", "staging/2.0.0"},
		{"env and version", "{version}/{env}", "prod", "3.1.0", "3.1.0/prod"},
		{"release prefix", "release/{version}", "prod", "1.0.0", "release/1.0.0"},
		{"v prefix", "v{version}", "", "1.2.3", "v1.2.3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.Render(tc.template, tagfmt.Tokens{Env: tc.env, Version: tc.version})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRenderError(t *testing.T) {
	_, err := tagfmt.Render("no-version-token", tagfmt.Tokens{Env: "dev", Version: "1.0.0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{version}")
}

func TestRender_WithBuild(t *testing.T) {
	tests := []struct {
		name     string
		template string
		env      string
		version  string
		build    string
		want     string
	}{
		{"env/version-build", "{env}/{version}-{build}", "uat", "7.4.1", "158404", "uat/7.4.1-158404"},
		{"version-build no env", "{version}-{build}", "", "1.2.3", "99", "1.2.3-99"},
		{"build in front", "{build}/{env}/{version}", "prod", "2.0.0", "42", "42/prod/2.0.0"},
		{"build ignored when not in template", "{env}/{version}", "uat", "7.4.1", "158404", "uat/7.4.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.Render(tc.template, tagfmt.Tokens{Env: tc.env, Version: tc.version, Build: tc.build})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRender_BuildRequiredButEmpty(t *testing.T) {
	_, err := tagfmt.Render("{env}/{version}-{build}", tagfmt.Tokens{Env: "uat", Version: "7.4.1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{build}")
	// The error must point the user toward --build on either command that accepts it —
	// heraut release --build exists too (T222/T231), not changelog-only as this message
	// used to imply.
	assert.Contains(t, err.Error(), "--build")
	assert.Contains(t, err.Error(), "heraut changelog")
	assert.Contains(t, err.Error(), "heraut release")
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		template string
		tag      string
		want     string
	}{
		{"version only", "{version}", "1.2.3", "1.2.3"},
		{"prefix slash", "dev/{version}", "dev/1.2.3", "1.2.3"},
		{"suffix slash", "{version}/dev", "1.2.3/dev", "1.2.3"},
		{"prefix underscore", "dev_{version}", "dev_1.2.3", "1.2.3"},
		{"suffix underscore", "{version}_dev", "1.2.3_dev", "1.2.3"},
		{"env token", "{env}/{version}", "staging/2.0.0", "2.0.0"},
		{"release prefix", "release/{version}", "release/1.0.0", "1.0.0"},
		{"v prefix", "v{version}", "v1.2.3", "1.2.3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.ParseVersion(tc.template, tc.tag)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseVersionError(t *testing.T) {
	tests := []struct {
		name     string
		template string
		tag      string
	}{
		{"no version token in template", "no-version-token", "something"},
		{"tag does not match template", "dev/{version}", "prod/1.0.0"},
		{"tag too short", "dev/{version}", "dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tagfmt.ParseVersion(tc.template, tc.tag)
			require.Error(t, err)
		})
	}
}

func TestGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		template string
		env      string
		want     string
	}{
		{"version only", "{version}", "", "*"},
		{"prefix slash", "dev/{version}", "dev", "dev/*"},
		{"suffix slash", "{version}/dev", "dev", "*/dev"},
		{"prefix underscore", "dev_{version}", "dev", "dev_*"},
		{"suffix underscore", "{version}_dev", "dev", "*_dev"},
		{"env token", "{env}/{version}", "staging", "staging/*"},
		{"release prefix", "release/{version}", "prod", "release/*"},
		{"v prefix", "v{version}", "", "v*"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.GlobPattern(tc.template, tagfmt.Tokens{Env: tc.env})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGlobPatternError(t *testing.T) {
	_, err := tagfmt.GlobPattern("no-version-token", tagfmt.Tokens{Env: "dev"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{version}")
}

func TestDeriveHeadingVersionPattern(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		wantNil    bool
		noMatches  []string
		wantGroup1 map[string]string // heading → expected capture group $1
	}{
		{
			name:     "plain version returns empty (nothing to strip)",
			template: "{version}",
			wantNil:  true,
		},
		{
			name:     "v prefix returns empty (template already trims v)",
			template: "v{version}",
			wantNil:  true,
		},
		{
			name:     "no version token returns empty",
			template: "{env}/{build}",
			wantNil:  true,
		},
		{
			name:     "env suffix",
			template: "{version}_{env}",
			wantGroup1: map[string]string{
				"[2026.3.0_prod]": "2026.3.0",
				"[1.2.3_prod]":    "1.2.3",
			},
		},
		{
			name:     "env prefix",
			template: "{env}/{version}",
			wantGroup1: map[string]string{
				"[prod/1.2.3]":    "1.2.3",
				"[staging/2.0.0]": "2.0.0",
			},
		},
		{
			name:     "env prefix + build suffix",
			template: "{env}/{version}-{build}",
			wantGroup1: map[string]string{
				"[uat/7.4.1-158404]":      "7.4.1",
				"[uat/7.4.1-rc.1-158404]": "7.4.1-rc.1",
			},
		},
		{
			name:     "version-build no env",
			template: "{version}-{build}",
			wantGroup1: map[string]string{
				"[7.4.1-158404]":      "7.4.1",
				"[7.4.1-rc.1-158404]": "7.4.1-rc.1",
			},
		},
		{
			name:     "plus build separator",
			template: "{env}/{version}+{build}",
			wantGroup1: map[string]string{
				"[uat/7.4.1+158404]": "7.4.1",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := tagfmt.DeriveHeadingVersionPattern(tc.template)
			if tc.wantNil {
				assert.Empty(t, pat)
				return
			}
			require.NotEmpty(t, pat)
			re, err := regexp.Compile(pat)
			require.NoError(t, err, "derived pattern must compile: %s", pat)
			for heading, want := range tc.wantGroup1 {
				m := re.FindStringSubmatch(heading)
				require.NotNilf(t, m, "pattern should match %q", heading)
				assert.Equalf(t, want, m[1], "capture group for %q", heading)
			}
			for _, h := range tc.noMatches {
				assert.Nilf(t, re.FindStringSubmatch(h), "pattern should NOT match %q", h)
			}
		})
	}
}

// The derived pattern must not match across two headings — wildcards exclude ']'.
func TestDeriveHeadingVersionPattern_NoCrossHeadingMatch(t *testing.T) {
	pat := tagfmt.DeriveHeadingVersionPattern("{version}_{env}")
	require.NotEmpty(t, pat)
	re := regexp.MustCompile(pat)

	doc := "## [2026.3.0_prod] - x\n## [2026.2.0_prod] - y"
	got := re.ReplaceAllString(doc, "[$1]")
	assert.Equal(t, "## [2026.3.0] - x\n## [2026.2.0] - y", got)
}

func TestValidateBuildID(t *testing.T) {
	tests := []struct {
		name    string
		build   string
		wantErr bool
	}{
		{"numeric CI id", "158404", false},
		{"alphanumeric", "build42abc", false},
		{"with dots and dashes", "1.2.3-beta", false},
		{"empty", "", true},
		{"contains slash", "a/b", true},
		{"contains space", "build 42", true},
		{"contains tab", "build\t42", true},
		{"leading space", " 158404", true},
		{"trailing newline", "158404\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tagfmt.ValidateBuildID(tc.build)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVersionOverride(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"semver with v prefix", "v1.2.3", false},
		{"semver without prefix", "1.2.3", false},
		{"calver year.patch", "2024.03", false},
		{"calver full", "2024.03.15.2", false},
		{"pre-release", "1.2.3-rc.1", false},
		{"bare word", "notaversion", false},
		{"v prefix only", "v", false},
		{"empty", "", true},
		{"contains space", "1.2 .3", true},
		{"trailing space", "1.2.3 ", true},
		{"contains tab", "1.2.3\t", true},
		{"trailing newline", "1.2.3\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tagfmt.ValidateVersionOverride(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseVersion_WithBuild(t *testing.T) {
	tests := []struct {
		name     string
		template string
		tag      string
		want     string
	}{
		{"env/version-build extracts version", "{env}/{version}-{build}", "uat/7.4.1-158404", "7.4.1"},
		{"version-build extracts version", "{version}-{build}", "1.2.3-99", "1.2.3"},
		{"semver pre-release plus build", "{env}/{version}-{build}", "uat/7.4.0-rc.1-158404", "7.4.0-rc.1"},
		{"build in front", "{build}/{env}/{version}", "42/prod/2.0.0", "2.0.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.ParseVersion(tc.template, tc.tag)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGlobPattern_WithBuild(t *testing.T) {
	tests := []struct {
		name     string
		template string
		env      string
		want     string
	}{
		{"env/version-build", "{env}/{version}-{build}", "uat", "uat/*-*"},
		{"version-build no env", "{version}-{build}", "", "*-*"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagfmt.GlobPattern(tc.template, tagfmt.Tokens{Env: tc.env})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
