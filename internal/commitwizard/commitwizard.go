// Package commitwizard implements `heraut commit create` — an interactive wizard that
// builds a Conventional Commits message and runs git commit. The interactive form lives
// in form.go (no unit tests, same as internal/scaffold/wizard.go); everything else is
// unit- or contract-tested.
package commitwizard

import (
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// Answers is the data the interactive form collects.
type Answers struct {
	Type         string
	Scope        string
	Subject      string
	Body         string
	Breaking     bool
	BreakingDesc string
	Footers      []conventionalcommit.Footer
}

// Options controls a wizard run.
type Options struct {
	All    bool      // pass -a to git commit (stage tracked modifications)
	DryRun bool      // print the assembled message, do not stage or commit
	Out    io.Writer // command stdout (also used for the TTY check)
}

// Assemble maps collected Answers to a conventional-commit Commit. A breaking change adds
// "!" to the header; a non-empty breaking description is prepended as a BREAKING CHANGE
// footer ahead of the user's footers.
func Assemble(a Answers) *conventionalcommit.Commit {
	c := &conventionalcommit.Commit{
		Type:        a.Type,
		Scope:       a.Scope,
		Breaking:    a.Breaking,
		Description: a.Subject,
		Body:        a.Body,
	}
	if a.Breaking && a.BreakingDesc != "" {
		c.Footers = append(c.Footers, conventionalcommit.Footer{
			Token: "BREAKING CHANGE",
			Value: a.BreakingDesc,
		})
	}
	c.Footers = append(c.Footers, a.Footers...)
	return c
}

// parseFooterLines converts a multi-line footer block into structured footers, skipping
// blank lines and erroring on any line that is not a valid trailer.
func parseFooterLines(text string) ([]conventionalcommit.Footer, error) {
	var footers []conventionalcommit.Footer
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f, ok := conventionalcommit.ParseFooterLine(line)
		if !ok {
			return nil, fmt.Errorf("invalid footer line %q: expected \"Token: value\"", line)
		}
		footers = append(footers, f)
	}
	return footers, nil
}
