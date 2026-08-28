package cmd

import (
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/exitcode"
)

// ExitCode maps the error returned from command execution to a process exit
// code (see Spec 01 — Exit codes). cmd/heraut passes the error from
// forge/cli.Run here to decide os.Exit's argument.
func ExitCode(err error) int {
	return exitcode.Resolve(err)
}

// wrapRunErr classifies an error from resolving a version or running a pipeline:
// promotion guards (E001/E002/E003) map to exit code 4, everything else to the
// runtime code. Returns nil when err is nil.
//
// summary is optional. When given, the returned error displays summary instead of err's own
// message (exitcode.WrapSummary) — used after a ui.Spinner-reported pipeline run has already
// shown the detailed error once, so fang's top-level error panel doesn't repeat it verbatim.
// version.go's bare resolver error (no step reporter involved) omits it, so its only display
// of the error is the one fang shows.
func wrapRunErr(err error, summary ...string) error {
	if err == nil {
		return nil
	}
	code := exitcode.Runtime
	if app.IsPromotionGuard(err) {
		code = exitcode.Promotion
	}
	if len(summary) > 0 {
		return exitcode.WrapSummary(code, err, summary[0])
	}
	return exitcode.Wrap(code, err)
}
