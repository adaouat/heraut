package tagfmt_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/versioning/tagfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			got, err := tagfmt.Render(tc.template, tc.env, tc.version)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRenderError(t *testing.T) {
	_, err := tagfmt.Render("no-version-token", "dev", "1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{version}")
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
			got, err := tagfmt.GlobPattern(tc.template, tc.env)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGlobPatternError(t *testing.T) {
	_, err := tagfmt.GlobPattern("no-version-token", "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{version}")
}
