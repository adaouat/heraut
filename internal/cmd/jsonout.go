package cmd

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/exitcode"
)

// jsonEnvelope is the one shape every --json command prints (Spec 03 § Machine-readable output).
type jsonEnvelope struct {
	OK       bool       `json:"ok"`
	Data     any        `json:"data,omitempty"`
	Warnings []string   `json:"warnings"`
	Error    *jsonError `json:"error,omitempty"`
}

type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// writeJSON prints the envelope for a finished command as a single line and returns err unchanged,
// so the caller returns it and the process keeps the exit code the error carries.
func writeJSON(w io.Writer, data any, warnings []string, err error) error {
	if warnings == nil {
		warnings = []string{}
	}
	env := jsonEnvelope{OK: err == nil, Warnings: warnings}
	if err == nil {
		env.Data = data
	} else {
		env.Error = &jsonError{Code: errorCode(err), Message: err.Error(), Hint: errorHint(err)}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if encErr := enc.Encode(env); encErr != nil && err == nil {
		return exitcode.Wrap(exitcode.Runtime, encErr)
	}
	return err
}

// errorCode is the stable error.code: a promotion guard's E-number, else the name of the exit-code
// class.
func errorCode(err error) string {
	if id := app.PromotionGuardID(err); id != "" {
		return id
	}
	switch exitcode.Resolve(err) {
	case exitcode.Config:
		return "config"
	case exitcode.Runtime:
		return "runtime"
	case exitcode.Promotion:
		return "promotion"
	case exitcode.Internal:
		return "internal"
	}
	return "usage"
}

func errorHint(err error) string {
	var h interface{ Hint() string }
	if errors.As(err, &h) {
		return h.Hint()
	}
	return ""
}
