package config

import "strings"

// ValidationError describes a single semantic validation failure.
type ValidationError struct {
	Path    string
	Message string
	Hint    string
}

func (e ValidationError) Error() string {
	s := e.Path + ": " + e.Message
	if e.Hint != "" {
		s += "\n  hint: " + e.Hint
	}
	return s
}

// ValidationErrors is a list of ValidationError that implements error.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "\n")
}
