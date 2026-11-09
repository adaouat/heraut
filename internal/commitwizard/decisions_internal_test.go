package commitwizard

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// Every failure on the way to a staged commit stops the wizard with the reason, before any question
// about the message.
func TestResolveStaging_Failures(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name     string
		replies  []error // one per git call, in order
		prompt   error
		wantText string
	}{
		{name: "git cannot list the staged files", replies: []error{boom}, wantText: "boom"},
		{name: "git cannot read the working tree", replies: []error{nil, boom}, wantText: "checking working tree status"},
		{name: "the prompt fails", replies: []error{nil, nil}, prompt: boom, wantText: "boom"},
		{name: "git add fails", replies: []error{nil, nil, boom}, wantText: "staging all changes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			outputs := []string{"", " M x.go\n", ""} // nothing staged, a dirty tree, git add
			for i, err := range tc.replies {
				mr.QueueResponse(outputs[i], "", err)
			}

			proceed, err := resolveStaging(mr, Options{Out: &bytes.Buffer{}}, func() (bool, error) { return true, tc.prompt })

			require.Error(t, err)
			assert.False(t, proceed)
			assert.Contains(t, err.Error(), tc.wantText)
		})
	}
}

// The wizard offers the configured types and scopes; without a commits block it gets none (and falls
// back to its own list).
func TestCommitsTypesAndScopes(t *testing.T) {
	assert.Nil(t, commitsTypes(nil))
	assert.Nil(t, commitsScopes(&config.Config{}))

	cfg := &config.Config{Commits: &config.Commits{
		Types:  []config.TypeRule{{Name: "feat"}},
		Scopes: []config.ScopeRule{{Name: "api"}},
	}}
	assert.Equal(t, []config.TypeRule{{Name: "feat"}}, commitsTypes(cfg))
	assert.Equal(t, []config.ScopeRule{{Name: "api"}}, commitsScopes(cfg))
}
