package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
)

func TestCheckFullHistory(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		err       error
		wantError bool
	}{
		{name: "shallow clone", out: "true\n", wantError: true},
		{name: "full history", out: "false\n"},
		{name: "git cannot tell", err: errors.New("not a git repository")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.out, "", tc.err)

			err := app.CheckFullHistory(mr)

			require.Len(t, mr.Calls, 1)
			assert.Equal(t, "git", mr.Calls[0].Name)
			assert.Equal(t, []string{"rev-parse", "--is-shallow-repository"}, mr.Calls[0].Args)
			if !tc.wantError {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, app.IsUnusableCheckout(err))
			assert.True(t, app.IsUnusableCheckout(fmt.Errorf("wrapped: %w", err)))
			for _, hint := range []string{"shallow clone", "git fetch --unshallow --tags", "fetch-depth: 0", `GIT_DEPTH: "0"`} {
				assert.Contains(t, err.Error(), hint)
			}
		})
	}
}

func TestIsUnusableCheckout_OtherErrors(t *testing.T) {
	assert.False(t, app.IsUnusableCheckout(nil))
	assert.False(t, app.IsUnusableCheckout(errors.New("boom")))
}

func TestShallowCloneWarning(t *testing.T) {
	tests := []struct {
		name string
		out  string
		err  error
		want bool
	}{
		{name: "shallow clone", out: "true\n", want: true},
		{name: "full history", out: "false\n"},
		{name: "git cannot tell", err: errors.New("not a git repository")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.out, "", tc.err)

			got := app.ShallowCloneWarning(mr)

			require.Len(t, mr.Calls, 1)
			assert.Equal(t, []string{"rev-parse", "--is-shallow-repository"}, mr.Calls[0].Args)
			if !tc.want {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, "shallow clone")
			assert.Contains(t, got, "git fetch --unshallow --tags")
		})
	}
}
