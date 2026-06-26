package conventionalcommit_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    conventionalcommit.Commit
	}{
		{
			name:    "simple feat",
			message: "feat: add x",
			want:    conventionalcommit.Commit{Type: "feat", Description: "add x"},
		},
		{
			name:    "fix with scope",
			message: "fix(api): handle y",
			want:    conventionalcommit.Commit{Type: "fix", Scope: "api", Description: "handle y"},
		},
		{
			name:    "breaking bang, no scope",
			message: "feat!: breaking",
			want:    conventionalcommit.Commit{Type: "feat", Breaking: true, Description: "breaking"},
		},
		{
			name:    "breaking bang with scope",
			message: "feat(api)!: remove endpoint",
			want:    conventionalcommit.Commit{Type: "feat", Scope: "api", Breaking: true, Description: "remove endpoint"},
		},
		{
			name:    "bang in description, not type prefix, not breaking",
			message: "fix: handle the foo!: token",
			want:    conventionalcommit.Commit{Type: "fix", Description: "handle the foo!: token"},
		},
		{
			name:    "header with trailing newline (raw COMMIT_EDITMSG content)",
			message: "feat: add x\n",
			want:    conventionalcommit.Commit{Type: "feat", Description: "add x"},
		},
		{
			name:    "BREAKING CHANGE footer",
			message: "fix: y\n\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{{Token: "BREAKING CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "BREAKING-CHANGE hyphenated footer",
			message: "fix: y\n\nBREAKING-CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{{Token: "BREAKING-CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "BREAKING CHANGE mentioned mid-sentence, not a footer, not breaking",
			message: "fix: y\n\nThis is not a BREAKING CHANGE: just a mention.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "This is not a BREAKING CHANGE: just a mention.",
			},
		},
		{
			name:    "BREAKING-CHANGE mentioned mid-sentence, not a footer, not breaking",
			message: "fix: y\n\nAlso recognize the hyphenated BREAKING-CHANGE: footer as a synonym.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "Also recognize the hyphenated BREAKING-CHANGE: footer as a synonym.",
			},
		},
		{
			name:    "BREAKING CHANGE starts a wrapped body line, not its paragraph, not breaking",
			message: "fix: y\n\nDiscussing isBreaking's\nBREAKING CHANGE: footer check here.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "Discussing isBreaking's\nBREAKING CHANGE: footer check here.",
			},
		},
		{
			name:    "body with no footers",
			message: "docs: update readme\n\nExplains the new flag in detail.",
			want: conventionalcommit.Commit{
				Type: "docs", Description: "update readme",
				Body: "Explains the new flag in detail.",
			},
		},
		{
			name:    "body paragraph then separate footer block",
			message: "fix: y\n\nSome body text.\n\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Body:    "Some body text.",
				Footers: []conventionalcommit.Footer{{Token: "BREAKING CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "multiple footers, hyphenated token",
			message: "fix: y\n\nAcked-by: Alice\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{
					{Token: "Acked-by", Value: "Alice"},
					{Token: "BREAKING CHANGE", Value: "boom"},
				},
			},
		},
		{
			name:    "multi-line footer value (continuation line)",
			message: "fix: y\n\nSigned-off-by: Bob\nThis continues the previous footer.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Footers: []conventionalcommit.Footer{
					{Token: "Signed-off-by", Value: "Bob\nThis continues the previous footer."},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := conventionalcommit.Parse(tc.message)
			require.NoError(t, err)
			assert.Equal(t, &tc.want, got)
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"missing colon separator", "feat add x"},
		{"empty description", "feat:"},
		{"missing type", ": add x"},
		{"merge commit is not conventional grammar", "Merge branch 'main' into feature"},
		{"missing blank line before body", "feat: x\nBody text immediately"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conventionalcommit.Parse(tc.message)
			require.Error(t, err)
		})
	}
}

func TestIsMergeCommit(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"merge branch", "Merge branch 'main' into feature/x", true},
		{"merge pull request", "Merge pull request #42 from org/feature", true},
		{"merge remote-tracking branch", "Merge remote-tracking branch 'origin/main'", true},
		{"not a merge", "feat: add x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, conventionalcommit.IsMergeCommit(tc.message))
		})
	}
}

func TestIsFixupCommit(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"fixup", "fixup! feat: add x", true},
		{"squash", "squash! feat: add x", true},
		{"not fixup", "feat: add x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, conventionalcommit.IsFixupCommit(tc.message))
		})
	}
}

func TestCommit_Format(t *testing.T) {
	tests := []struct {
		name string
		in   conventionalcommit.Commit
		want string
	}{
		{
			name: "type + subject only",
			in:   conventionalcommit.Commit{Type: "feat", Description: "add wizard"},
			want: "feat: add wizard",
		},
		{
			name: "scope",
			in:   conventionalcommit.Commit{Type: "fix", Scope: "cmd", Description: "x"},
			want: "fix(cmd): x",
		},
		{
			name: "breaking bang",
			in:   conventionalcommit.Commit{Type: "feat", Scope: "cmd", Breaking: true, Description: "drop flag"},
			want: "feat(cmd)!: drop flag",
		},
		{
			name: "body",
			in:   conventionalcommit.Commit{Type: "feat", Description: "x", Body: "why line one\nwhy line two"},
			want: "feat: x\n\nwhy line one\nwhy line two",
		},
		{
			name: "breaking footer + user footer",
			in: conventionalcommit.Commit{
				Type: "feat", Description: "x", Breaking: true,
				Footers: []conventionalcommit.Footer{
					{Token: "BREAKING CHANGE", Value: "old removed"},
					{Token: "Closes", Value: "#42"},
				},
			},
			want: "feat!: x\n\nBREAKING CHANGE: old removed\nCloses: #42",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.in.Format())
		})
	}
}

func TestCommit_Format_RoundTripsThroughParse(t *testing.T) {
	original := conventionalcommit.Commit{
		Type: "feat", Scope: "cmd", Breaking: true, Description: "add the wizard",
		Body: "Guided prompts build the message.",
		Footers: []conventionalcommit.Footer{
			{Token: "BREAKING CHANGE", Value: "the old path is gone"},
			{Token: "Closes", Value: "#42"},
		},
	}
	reparsed, err := conventionalcommit.Parse(original.Format())
	require.NoError(t, err)
	assert.Equal(t, original.Type, reparsed.Type)
	assert.Equal(t, original.Scope, reparsed.Scope)
	assert.True(t, reparsed.Breaking)
	assert.Equal(t, original.Description, reparsed.Description)
	assert.Equal(t, original.Body, reparsed.Body)
	assert.Equal(t, original.Footers, reparsed.Footers)
}

func TestParseFooterLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   conventionalcommit.Footer
		wantOK bool
	}{
		{"colon form", "Closes: #42", conventionalcommit.Footer{Token: "Closes", Value: "#42"}, true},
		{"hash form preserves #", "Closes #42", conventionalcommit.Footer{Token: "Closes", Value: "#42"}, true},
		{"breaking change token", "BREAKING CHANGE: gone", conventionalcommit.Footer{Token: "BREAKING CHANGE", Value: "gone"}, true},
		{"not a footer", "just prose here", conventionalcommit.Footer{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := conventionalcommit.ParseFooterLine(tc.line)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func BenchmarkParse(b *testing.B) {
	inputs := map[string]string{
		"header_only": "feat: add x",
		"with_body":   "fix(api): y\n\nSome explanatory body text describing the change in more detail.",
		"with_footers": "feat(api)!: z\n\nBody text.\n\n" +
			"Acked-by: Alice\nReviewed-by: Bob\nBREAKING CHANGE: boom",
	}
	for name, in := range inputs {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = conventionalcommit.Parse(in)
			}
		})
	}
}
