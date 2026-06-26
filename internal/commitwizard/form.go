package commitwizard

import (
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/ui"
)

const (
	scopeCustom = "\x00custom"
	scopeNone   = "\x00none"
)

func themedForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(ui.HuhTheme())
}

// collectAnswers runs the field prompts (type, scope, subject, breaking, body, footers)
// and returns the assembled Answers.
func collectAnswers(cfg *config.Config) (Answers, error) {
	var a Answers
	var scopeChoice, customScope, footerText string

	typeOpts := make([]huh.Option[string], 0)
	for _, t := range app.AllowedCommitTypes(cfg) {
		typeOpts = append(typeOpts, huh.NewOption(typeOptionLabel(t), t))
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().Title("Type").Options(typeOpts...).Value(&a.Type),
		),
	}

	scopes := configuredScopes(cfg)
	if len(scopes) > 0 {
		scopeOpts := make([]huh.Option[string], 0, len(scopes)+2)
		for _, s := range scopes {
			scopeOpts = append(scopeOpts, huh.NewOption(s, s))
		}
		scopeOpts = append(scopeOpts, huh.NewOption("(custom…)", scopeCustom), huh.NewOption("(none)", scopeNone))
		groups = append(groups,
			huh.NewGroup(
				huh.NewSelect[string]().Title("Scope").Options(scopeOpts...).Value(&scopeChoice),
			),
			huh.NewGroup(
				huh.NewInput().Title("Custom scope").Value(&customScope),
			).WithHideFunc(func() bool { return scopeChoice != scopeCustom }),
		)
	} else {
		groups = append(groups,
			huh.NewGroup(
				huh.NewInput().Title("Scope").Description("optional — leave empty for none").Value(&a.Scope),
			),
		)
	}

	groups = append(groups,
		huh.NewGroup(
			huh.NewInput().Title("Subject").
				Description("short imperative summary").
				Value(&a.Subject).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("subject is required")
					}
					if strings.ContainsRune(s, '\n') {
						return fmt.Errorf("subject must be a single line")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Breaking change?").Value(&a.Breaking),
		),
		huh.NewGroup(
			huh.NewInput().Title("Describe the breaking change").Value(&a.BreakingDesc),
		).WithHideFunc(func() bool { return !a.Breaking }),
		huh.NewGroup(
			huh.NewText().Title("Body").Description("optional — the why").Value(&a.Body),
		),
		huh.NewGroup(
			huh.NewText().Title("Footers").
				Description(`optional — one "Token: value" per line, e.g. Closes: #42`).
				Value(&footerText).
				Validate(func(s string) error {
					_, err := parseFooterLines(s)
					return err
				}),
		),
	)

	if err := themedForm(groups...).Run(); err != nil {
		return Answers{}, fmt.Errorf("collecting commit details: %w", err)
	}

	switch scopeChoice {
	case scopeCustom:
		a.Scope = strings.TrimSpace(customScope)
	case scopeNone, "":
		// a.Scope already set by the free-text path, or intentionally empty
	default:
		a.Scope = scopeChoice
	}

	footers, err := parseFooterLines(footerText)
	if err != nil {
		return Answers{}, err
	}
	a.Footers = footers
	return a, nil
}

func configuredScopes(cfg *config.Config) []string {
	if cfg != nil && cfg.CommitLint != nil {
		return cfg.CommitLint.Scopes
	}
	return nil
}

// confirmStageAll prompts when nothing is staged. Returns true to `git add -A`, false to cancel.
func confirmStageAll() (bool, error) {
	var stage bool
	err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Nothing staged").
				Description("Stage all changes (git add -A) before committing?").
				Affirmative("Stage all").
				Negative("Cancel").
				Value(&stage),
		),
	).Run()
	if err != nil {
		return false, fmt.Errorf("confirming staging: %w", err)
	}
	return stage, nil
}

// confirmCommit shows the assembled message and asks for final confirmation.
func confirmCommit(_ io.Writer, msg string) (bool, error) {
	var ok bool
	err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Commit this message?").
				Description(msg).
				Value(&ok),
		),
	).Run()
	if err != nil {
		return false, fmt.Errorf("confirming commit: %w", err)
	}
	return ok, nil
}
