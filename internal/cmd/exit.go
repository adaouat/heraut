package cmd

import (
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/exitcode"
)

// ExitCode maps the error returned from command execution to a process exit
// code (see Spec 01 — Exit codes). cmd/heraut passes the error from
// fang.Execute here to decide os.Exit's argument.
func ExitCode(err error) int {
	return exitcode.Resolve(err)
}

// wrapRunErr classifies an error from resolving a version or running a pipeline:
// promotion guards (E001/E002/E003) map to exit code 4, everything else to the
// runtime code. Returns nil when err is nil.
func wrapRunErr(err error) error {
	if err == nil {
		return nil
	}
	if app.IsPromotionGuard(err) {
		return exitcode.Wrap(exitcode.Promotion, err)
	}
	return exitcode.Wrap(exitcode.Runtime, err)
}
