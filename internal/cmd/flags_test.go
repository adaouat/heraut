package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/exitcode"
)

func flagTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("env", "", "")
	cmd.Flags().Bool("force", false, "")
	cmd.Flags().StringSlice("skip", nil, "")
	return cmd
}

func TestFlagReader_ReadsDeclaredFlags(t *testing.T) {
	cmd := flagTestCmd()
	require.NoError(t, cmd.Flags().Set("env", "prod"))
	require.NoError(t, cmd.Flags().Set("force", "true"))

	f := newFlagReader(cmd)
	assert.Equal(t, "prod", f.String("env"))
	assert.True(t, f.Bool("force"))
	assert.NoError(t, f.Err())
}

func TestFlagReader_UndeclaredFlagIsAnError(t *testing.T) {
	f := newFlagReader(flagTestCmd())

	assert.False(t, f.Bool("xq-force"), "a mistyped name reads as the zero value")
	assert.Equal(t, "", f.String("xq-env"))

	err := f.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--xq-force", "the first bad name is reported")
	assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err), "a missing flag is a heraut bug, not a usage error")
}

func TestFlagReader_WrongTypeIsAnError(t *testing.T) {
	f := newFlagReader(flagTestCmd())
	_ = f.Bool("env")
	assert.Error(t, f.Err())
}
