// Package commitwizard implements `heraut commit create` — an interactive wizard that
// builds a Conventional Commits message and runs git commit. The interactive form lives
// in form.go (no unit tests, same as internal/scaffold/wizard.go); everything else is
// unit- or contract-tested.
package commitwizard

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
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
// footer ahead of the user's footers. The subject and breaking description are trimmed so
// stray whitespace never reaches the commit header or footer.
func Assemble(a Answers) *conventionalcommit.Commit {
	breakingDesc := strings.TrimSpace(a.BreakingDesc)
	c := &conventionalcommit.Commit{
		Type:        a.Type,
		Scope:       a.Scope,
		Breaking:    a.Breaking,
		Description: strings.TrimSpace(a.Subject),
		Body:        a.Body,
	}
	if a.Breaking && breakingDesc != "" {
		c.Footers = append(c.Footers, conventionalcommit.Footer{
			Token: "BREAKING CHANGE",
			Value: breakingDesc,
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

// Run drives the interactive wizard: TTY check → staging (unless --all or --dry-run) →
// collect answers → finalize. Returns nil for clean no-ops (cancel/decline/dry-run).
func Run(r port.Runner, cfg *config.Config, opts Options) error {
	if !ui.IsTTY(opts.Out) {
		return errors.New("commit create requires an interactive terminal")
	}

	if !opts.DryRun && !opts.All {
		proceed, err := resolveStaging(r, opts, confirmStageAll)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	a, err := collectAnswers(cfg)
	if err != nil {
		return err
	}
	return finalize(r, cfg, a, opts, confirmCommit)
}

// resolveStaging ensures something is staged before the wizard collects a message, or
// decides to stop. It returns proceed=false (after printing the reason) when the working
// tree is clean (nothing to commit) or the user declines to stage. confirmStage is injected
// so the decision is testable without a terminal; Run passes the interactive confirmStageAll.
func resolveStaging(r port.Runner, opts Options, confirmStage func() (bool, error)) (bool, error) {
	staged, err := hasStaged(r)
	if err != nil {
		return false, err
	}
	if staged {
		return true, nil
	}

	dirty, err := hasWorkingTreeChanges(r)
	if err != nil {
		return false, err
	}
	if !dirty {
		_, _ = fmt.Fprintln(opts.Out, ui.Info(opts.Out, "nothing to commit — working tree clean"))
		return false, nil
	}

	stage, err := confirmStage()
	if err != nil {
		return false, err
	}
	if !stage {
		_, _ = fmt.Fprintln(opts.Out, ui.Info(opts.Out, "nothing staged — cancelled"))
		return false, nil
	}
	if err := stageAll(r); err != nil {
		return false, err
	}
	return true, nil
}

// commitTypeDescriptions are the one-line hints shown beside the 10 built-in types.
var commitTypeDescriptions = map[string]string{
	"feat":     "A new feature",
	"fix":      "A bug fix",
	"docs":     "Documentation only",
	"chore":    "Tooling / housekeeping",
	"refactor": "Code change, no behaviour change",
	"test":     "Adding or fixing tests",
	"style":    "Formatting / whitespace",
	"perf":     "Performance improvement",
	"ci":       "CI / release tooling",
	"build":    "Build system / dependencies",
}

// typeOptionLabel renders the select-menu label for a commit type: "<type>  <description>"
// for the built-in types, or the bare type for custom configured ones.
func typeOptionLabel(t string) string {
	if d, ok := commitTypeDescriptions[t]; ok {
		return fmt.Sprintf("%-6s  %s", t, d)
	}
	return t
}

// finalize assembles, verifies, and (unless dry-run) commits. confirm is injected so the
// pipeline is testable without a terminal; Run passes the interactive confirmCommit form.
func finalize(r port.Runner, cfg *config.Config, a Answers, opts Options, confirm func(out io.Writer, msg string) (bool, error)) error {
	msg := Assemble(a).Format()

	if err := app.VerifyCommit(cfg, msg); err != nil {
		return fmt.Errorf("assembled message failed validation: %w", err)
	}

	if opts.DryRun {
		_, _ = fmt.Fprintln(opts.Out, msg)
		_, _ = fmt.Fprintln(opts.Out, ui.Info(opts.Out, "[dry-run] would run: git commit"))
		return nil
	}

	ok, err := confirm(opts.Out, msg)
	if err != nil {
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(opts.Out, msg)
		_, _ = fmt.Fprintln(opts.Out, ui.Info(opts.Out, "commit cancelled"))
		return nil
	}

	return commit(r, msg, opts.All)
}
