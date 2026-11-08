package app_test

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
)

// exitOne is the error git returns for "no": a process that exited 1.
func exitOne(t *testing.T) error {
	t.Helper()
	err := exec.Command("false").Run()
	require.Error(t, err)
	return err
}

// heraut notes checks both tags exist and that --from is in the tag's history, with these exact git
// calls; only exit 1 means "not an ancestor", any other failure is reported as a failure.
func TestGenerateNotes_RangeChecksContract(t *testing.T) {
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}
	opts := app.NotesOpts{Tag: "v1.1.0", From: "v1.0.0"}
	tests := []struct {
		name     string
		mergeErr func(*testing.T) error
		want     error
		wantText string
	}{
		{"not an ancestor (exit 1)", exitOne, app.ErrNotesRange, "v1.0.0 is not in the history of v1.1.0"},
		{"git fails otherwise", func(*testing.T) error { return errors.New("fatal: bad object") }, nil, "checking that v1.0.0 is in the history of v1.1.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse("", "", nil)            // show-ref v1.1.0
			mr.QueueResponse("", "", nil)            // show-ref v1.0.0
			mr.QueueResponse("", "", tc.mergeErr(t)) // merge-base --is-ancestor

			_, err := app.GenerateNotes(t.Context(), mr, cfg, opts)

			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			} else {
				assert.NotErrorIs(t, err, app.ErrNotesRange)
			}
			assert.Contains(t, err.Error(), tc.wantText)
			require.Len(t, mr.Calls, 3)
			assert.Equal(t, []string{"show-ref", "--verify", "--quiet", "refs/tags/v1.1.0"}, mr.Calls[0].Args)
			assert.Equal(t, []string{"show-ref", "--verify", "--quiet", "refs/tags/v1.0.0"}, mr.Calls[1].Args)
			assert.Equal(t, []string{"merge-base", "--is-ancestor", "v1.0.0", "v1.1.0"}, mr.Calls[2].Args)
		})
	}
}

// A tag that doesn't exist (show-ref exits 1) is reported as missing, not as a git failure.
func TestGenerateNotes_MissingTagContract(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", exitOne(t))

	_, err := app.GenerateNotes(t.Context(), mr, &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}, app.NotesOpts{Tag: "v9.9.9"})

	assert.ErrorIs(t, err, app.ErrNotesTagNotFound)
	assert.Equal(t, []string{"show-ref", "--verify", "--quiet", "refs/tags/v9.9.9"}, mr.Calls[0].Args)
}
