package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The update hint may call GitHub, so it stays quiet where heraut promises no network or machine
// output: --dry-run, --offline, --json and check config. Other runs still get it.
func TestWithUpdateHint_QuietWhereNoNetworkIsPromised(t *testing.T) {
	root := NewRootCmd("v1.0.0")
	tests := []struct {
		args  []string
		wantH bool
	}{
		{[]string{"version", "next"}, true},
		{[]string{"release", "--dry-run"}, false},
		{[]string{"changelog", "--offline"}, false},
		{[]string{"version", "next", "--json"}, false},
		{[]string{"check", "config"}, false},
		{[]string{"check", "runtime"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.args[0]+" "+tc.args[len(tc.args)-1], func(t *testing.T) {
			cmd, rest, err := root.Find(tc.args)
			require.NoError(t, err)
			require.NoError(t, cmd.ParseFlags(rest))
			calls := 0
			hint := withUpdateHint(func(*cobra.Command, []string) error { calls++; return nil })

			require.NoError(t, hint(cmd, nil))

			assert.Equal(t, tc.wantH, calls == 1)
		})
	}
}
