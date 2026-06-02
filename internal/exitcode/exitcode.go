// Package exitcode maps errors to the process exit codes documented in
// Spec 01 — Exit codes. The mechanism (ExitError, Wrap, Resolve) now lives in
// forge; this package keeps heraut's code values and delegates the plumbing, so
// lower layers still classify via Wrap and cmd/heraut resolves via Resolve.
package exitcode

import forgeexit "github.com/adaouat/forge/exitcode"

// Exit codes per docs/specs/01-overview.md § Exit codes.
const (
	OK        = 0  // success
	Usage     = 1  // bad flags or arguments (also the default for unclassified errors)
	Config    = 2  // invalid YAML, missing required fields, semantic validation failure
	Runtime   = 3  // binary missing, token unset, network failure, git operation failed
	Promotion = 4  // promotion guard tripped (E001/E002/E003)
	Internal  = 70 // unexpected/internal condition
)

// Wrap annotates err with an exit code; the first/innermost classification wins.
func Wrap(code int, err error) error { return forgeexit.Wrap(code, err) }

// Resolve maps an error to an exit code: nil → OK, a coded error → that code,
// anything else → Usage (heraut's historic exit-1 default).
func Resolve(err error) int { return forgeexit.Resolve(err) }
