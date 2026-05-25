// Package exitcode maps errors to the process exit codes documented in
// Spec 01 — Exit codes. Lower layers return plain or sentinel errors; the cmd
// boundary annotates them with a code via Wrap, and cmd/heraut resolves the
// final code via Resolve. This package is a leaf — it imports nothing from heraut.
package exitcode

import "errors"

// Exit codes per docs/specs/01-overview.md § Exit codes.
const (
	OK        = 0  // success
	Usage     = 1  // bad flags or arguments (also the default for unclassified errors)
	Config    = 2  // invalid YAML, missing required fields, semantic validation failure
	Runtime   = 3  // binary missing, token unset, network failure, git operation failed
	Promotion = 4  // promotion guard tripped (E001/E002/E003)
	Internal  = 70 // unexpected/internal condition
)

// Error carries an exit code alongside an underlying error.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// Wrap annotates err with an exit code. It returns nil when err is nil. If err
// already carries an exit code (anywhere in its chain), the original code is
// preserved — the first/innermost classification wins.
func Wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	var ee *Error
	if errors.As(err, &ee) {
		return err
	}
	return &Error{Code: code, Err: err}
}

// Resolve maps an error to an exit code: nil → OK, an error carrying an exit
// code → that code, anything else → Usage (heraut's historic exit-1 default).
func Resolve(err error) int {
	if err == nil {
		return OK
	}
	var ee *Error
	if errors.As(err, &ee) {
		return ee.Code
	}
	return Usage
}
