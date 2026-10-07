package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestParseBranchRange(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr string
	}{
		{"1.x", "1.x", ""},
		{"1.3.x", "1.3.x", ""},
		{"0.x", "0.x", ""},
		{"10.20.x", "10.20.x", ""},
		{"1.3", "", "not a valid range"},
		{"1.3.0", "", "not a valid range"},
		{"v1.x", "", "not a valid range"},
		{"01.x", "", "not a valid range"},
		{"1.03.x", "", "not a valid range"},
		{"x", "", "not a valid range"},
		{"", "", "not a valid range"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := config.ParseBranchRange(tc.in)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestParseBranchRange_fields(t *testing.T) {
	r, err := config.ParseBranchRange("2.x")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), r.Major)
	assert.Nil(t, r.Minor)

	r, err = config.ParseBranchRange("2.5.x")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), r.Major)
	require.NotNil(t, r.Minor)
	assert.Equal(t, uint64(5), *r.Minor)
}

func TestDeriveBranchRange(t *testing.T) {
	tests := []struct {
		branch string
		want   string
		ok     bool
	}{
		{"release/1.3", "1.3.x", true},
		{"release/1.3.x", "1.3.x", true},
		{"release/1.x", "1.x", true},
		{"release/v1.3", "1.3.x", true},
		{"release/V1.3", "1.3.x", true},
		{"1.3.x", "1.3.x", true},
		{"support/2.x", "2.x", true},
		{"release/legacy", "", false},
		{"release/7.8.0", "", false},
		{"release/1.3/hotfix", "", false},
		{"main", "", false},
		{"release/vv1.3", "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.branch, func(t *testing.T) {
			got, ok := config.DeriveBranchRange(tc.branch)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.want, got.String())
			}
		})
	}
}

func TestBranchRule_IsGlob(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"release/*", true},
		{"release/1.?", true},
		{"release/[12].x", true},
		{"main", false},
		{"release/1.3", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.BranchRule{Name: tc.name}.IsGlob())
		})
	}
}

func TestValidate_branches(t *testing.T) {
	const head = "version: \"1\"\nversioning:\n  strategy: semver\n  branches:\n"
	tests := []struct {
		name     string
		yaml     string
		wantPath string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "non-semver strategy",
			yaml:     "version: \"1\"\nversioning:\n  strategy: calver\n  format: \"YYYY.MM.PATCH\"\n  branches:\n    - name: main\n",
			wantPath: "versioning.branches",
			wantMsg:  "only valid with strategy: semver (current strategy: calver)",
			wantHint: "environments.<env>.branch",
		},
		{
			name:     "empty name",
			yaml:     head + "    - name: \"\"\n",
			wantPath: "versioning.branches[0].name",
			wantMsg:  "required",
		},
		{
			name:     "bad glob",
			yaml:     head + "    - name: main\n    - name: \"release/[\"\n",
			wantPath: "versioning.branches[1].name",
			wantMsg:  "invalid glob",
		},
		{
			name:     "bad range",
			yaml:     head + "    - name: release/1.3\n      range: \"1.3\"\n",
			wantPath: "versioning.branches[0].range",
			wantMsg:  `"1.3" is not a valid range`,
			wantHint: "use N.x (patch and minor) or N.N.x (patch only), e.g. 1.x or 1.3.x",
		},
		{
			name:     "duplicate range",
			yaml:     head + "    - name: release/a\n      range: 1.3.x\n    - name: release/b\n      range: 1.3.x\n",
			wantPath: "versioning.branches[1].range",
			wantMsg:  "duplicates versioning.branches[0].range (1.3.x)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := config.Validate(mustLoad(t, tc.yaml))
			e := findErr(errs, tc.wantPath)
			require.NotNil(t, e, "expected error on %q, got %v", tc.wantPath, errs)
			assert.Contains(t, e.Message, tc.wantMsg)
			assert.Contains(t, e.Hint, tc.wantHint)
		})
	}
}

func TestValidate_branchesValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
    - name: release/1.3
      range: 1.3.x
    - name: "release/*"
    - name: support/2
      range: 2.x
`)
	assert.Empty(t, config.Validate(cfg))
}
