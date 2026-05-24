package calver_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFormat_Tokens(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		wantKinds   []calver.TokenKind
		wantLiterals []string
	}{
		{
			name:   "YYYY.MM.PATCH",
			format: "YYYY.MM.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindMM, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", ""},
		},
		{
			name:   "YYYY.MM.DD.PATCH",
			format: "YYYY.MM.DD.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindMM, calver.KindLiteral, calver.KindDD, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", "", ".", ""},
		},
		{
			name:   "YYYY.WW.PATCH",
			format: "YYYY.WW.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindWW, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", ""},
		},
		{
			name:   "YYYY.QQ.PATCH",
			format: "YYYY.QQ.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindQQ, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", ""},
		},
		{
			name:   "YYYY.SS.PATCH",
			format: "YYYY.SS.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindSS, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", ""},
		},
		{
			name:   "YYYY.SPRINT.PATCH",
			format: "YYYY.SPRINT.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindSPRINT, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", "", ".", ""},
		},
		{
			name:   "YYYY.PATCH yearly",
			format: "YYYY.PATCH",
			wantKinds:    []calver.TokenKind{calver.KindYYYY, calver.KindLiteral, calver.KindPATCH},
			wantLiterals: []string{"", ".", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := calver.ParseFormat(tc.format)
			require.NoError(t, err)
			require.Len(t, tokens, len(tc.wantKinds))
			for i, tok := range tokens {
				assert.Equal(t, tc.wantKinds[i], tok.Kind, "token[%d] kind", i)
				if tc.wantLiterals[i] != "" {
					assert.Equal(t, tc.wantLiterals[i], tok.Literal, "token[%d] literal", i)
				}
			}
		})
	}
}

func TestParseFormat_NoPATCH_Error(t *testing.T) {
	_, err := calver.ParseFormat("YYYY.MM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PATCH")
}

func TestParseFormat_PATCHNotLast_Error(t *testing.T) {
	_, err := calver.ParseFormat("PATCH.YYYY.MM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "last")
}

func TestParseFormat_Empty_Error(t *testing.T) {
	_, err := calver.ParseFormat("")
	require.Error(t, err)
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		version string
		want    calver.Values
	}{
		{
			name:    "YYYY.MM.PATCH",
			format:  "YYYY.MM.PATCH",
			version: "2026.05.3",
			want:    calver.Values{Year: 2026, Month: 5, Patch: 3},
		},
		{
			name:    "YYYY.MM.DD.PATCH",
			format:  "YYYY.MM.DD.PATCH",
			version: "2026.05.07.0",
			want:    calver.Values{Year: 2026, Month: 5, Day: 7, Patch: 0},
		},
		{
			name:    "YYYY.WW.PATCH",
			format:  "YYYY.WW.PATCH",
			version: "2026.19.2",
			want:    calver.Values{Year: 2026, Week: 19, Patch: 2},
		},
		{
			name:    "YYYY.QQ.PATCH",
			format:  "YYYY.QQ.PATCH",
			version: "2026.2.0",
			want:    calver.Values{Year: 2026, Quarter: 2, Patch: 0},
		},
		{
			name:    "YYYY.SS.PATCH",
			format:  "YYYY.SS.PATCH",
			version: "2026.1.0",
			want:    calver.Values{Year: 2026, Semester: 1, Patch: 0},
		},
		{
			name:    "YYYY.SPRINT.PATCH",
			format:  "YYYY.SPRINT.PATCH",
			version: "2026.5.1",
			want:    calver.Values{Year: 2026, Sprint: 5, Patch: 1},
		},
		{
			name:    "YYYY.PATCH",
			format:  "YYYY.PATCH",
			version: "2026.7",
			want:    calver.Values{Year: 2026, Patch: 7},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := calver.ParseFormat(tc.format)
			require.NoError(t, err)
			got, err := calver.ParseVersion(tokens, tc.version)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseVersion_Mismatch_Error(t *testing.T) {
	tokens, err := calver.ParseFormat("YYYY.MM.PATCH")
	require.NoError(t, err)

	_, err = calver.ParseVersion(tokens, "notaversion")
	require.Error(t, err)
}

func TestRenderVersion(t *testing.T) {
	tests := []struct {
		name   string
		format string
		values calver.Values
		want   string
	}{
		{
			name:   "YYYY.MM.PATCH",
			format: "YYYY.MM.PATCH",
			values: calver.Values{Year: 2026, Month: 5, Patch: 3},
			want:   "2026.05.3",
		},
		{
			name:   "YYYY.MM.DD.PATCH zero-pad",
			format: "YYYY.MM.DD.PATCH",
			values: calver.Values{Year: 2026, Month: 5, Day: 7, Patch: 0},
			want:   "2026.05.07.0",
		},
		{
			name:   "YYYY.WW.PATCH zero-pad week",
			format: "YYYY.WW.PATCH",
			values: calver.Values{Year: 2026, Week: 19, Patch: 2},
			want:   "2026.19.2",
		},
		{
			name:   "YYYY.QQ.PATCH no padding",
			format: "YYYY.QQ.PATCH",
			values: calver.Values{Year: 2026, Quarter: 2, Patch: 0},
			want:   "2026.2.0",
		},
		{
			name:   "YYYY.SS.PATCH no padding",
			format: "YYYY.SS.PATCH",
			values: calver.Values{Year: 2026, Semester: 1, Patch: 0},
			want:   "2026.1.0",
		},
		{
			name:   "YYYY.SPRINT.PATCH no padding",
			format: "YYYY.SPRINT.PATCH",
			values: calver.Values{Year: 2026, Sprint: 5, Patch: 1},
			want:   "2026.5.1",
		},
		{
			name:   "YYYY.PATCH",
			format: "YYYY.PATCH",
			values: calver.Values{Year: 2026, Patch: 7},
			want:   "2026.7",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := calver.ParseFormat(tc.format)
			require.NoError(t, err)
			got := calver.RenderVersion(tokens, tc.values)
			assert.Equal(t, tc.want, got)
		})
	}
}
