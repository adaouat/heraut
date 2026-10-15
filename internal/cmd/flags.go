package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/exitcode"
)

// flagReader reads a command's flags and remembers the first failure, so a command reads its
// flags in a row and checks Err once. A name that is not declared on the command, or a flag of
// another type, would otherwise read as the zero value without a word.
type flagReader struct {
	cmd *cobra.Command
	err error
}

func newFlagReader(cmd *cobra.Command) *flagReader { return &flagReader{cmd: cmd} }

func (r *flagReader) String(name string) string {
	v, err := r.cmd.Flags().GetString(name)
	r.note(name, err)
	return v
}

func (r *flagReader) Bool(name string) bool {
	v, err := r.cmd.Flags().GetBool(name)
	r.note(name, err)
	return v
}

func (r *flagReader) note(name string, err error) {
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("reading --%s: %w", name, err)
	}
}

// Err returns the first failure as an internal error (Runtime): the flag names are heraut's own, so
// a failure is a bug in the command, not something the user typed.
func (r *flagReader) Err() error {
	if r.err == nil {
		return nil
	}
	return exitcode.Wrap(exitcode.Runtime, r.err)
}
