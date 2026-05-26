package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
	"github.com/adaouat/heraut/internal/port"
)

// RuntimeCheckItem records the outcome of one runtime check.
type RuntimeCheckItem struct {
	Name   string
	Value  string // resolved value (e.g. version string, identity, "clean")
	Err    error
	IsWarn bool // advisory only — shown with ! but does not fail the overall check
}

// PreflightCheck verifies git binary availability and git user name/email.
// Called automatically before heraut release and heraut changelog.
func PreflightCheck(runner port.Runner) error {
	if _, _, err := runner.Run("git", "--version"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	name, _, err := runner.Run("git", "config", "user.name")
	if err != nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("git user.name is not configured; run: git config user.name <name>")
	}
	email, _, err := runner.Run("git", "config", "user.email")
	if err != nil || strings.TrimSpace(email) == "" {
		return fmt.Errorf("git user.email is not configured; run: git config user.email <email>")
	}
	return nil
}

// RuntimeCheck verifies all runtime dependencies by calling dispatch once per check.
// dispatch receives the check name (for display before the check runs) and a run
// function that performs the check and returns the result. The caller decides how to
// present each item (spinner, plain line, etc.).
//
// Check order: git binary → working tree → generators → platforms → git user identity.
func RuntimeCheck(runner port.Runner, cfg *config.Config, dispatch func(name string, run func() RuntimeCheckItem)) {
	// git binary
	dispatch("git", func() RuntimeCheckItem {
		out, _, err := runner.Run("git", "--version")
		if err != nil {
			return RuntimeCheckItem{Name: "git", Err: err}
		}
		return RuntimeCheckItem{Name: "git", Value: strings.TrimSpace(out)}
	})

	// working tree (advisory — a dirty tree is a warning, not a hard failure)
	dispatch("working tree", func() RuntimeCheckItem {
		out, _, err := runner.Run("git", "status", "--porcelain")
		if err != nil {
			return RuntimeCheckItem{Name: "working tree", IsWarn: true,
				Err: fmt.Errorf("could not check: %w", err)}
		}
		trimmed := strings.TrimSpace(out)
		if trimmed == "" {
			return RuntimeCheckItem{Name: "working tree", Value: "clean"}
		}
		n := len(strings.Split(trimmed, "\n"))
		return RuntimeCheckItem{Name: "working tree", IsWarn: true,
			Err: fmt.Errorf("%d uncommitted change(s)", n)}
	})

	// Changelog generator
	if cfg.Changelog != nil {
		gen := cfg.Changelog.Generator
		dispatch("changelog generator", func() RuntimeCheckItem {
			g, buildErr := buildGenerator(runner, cfg.Changelog, gitcliff.ModeChangelog)
			if buildErr != nil {
				return RuntimeCheckItem{Name: "changelog generator", Err: buildErr}
			}
			return RuntimeCheckItem{
				Name: "changelog generator (" + gen + ")",
				Err:  g.Check(),
			}
		})
	}

	// Release notes generator
	if cfg.Release != nil && cfg.Release.Notes != nil {
		gen := cfg.Release.Notes.Generator
		dispatch("release-notes generator", func() RuntimeCheckItem {
			g, buildErr := buildGenerator(runner, cfg.Release.Notes, gitcliff.ModeReleaseNotes)
			if buildErr != nil {
				return RuntimeCheckItem{Name: "release-notes generator", Err: buildErr}
			}
			return RuntimeCheckItem{
				Name: "release-notes generator (" + gen + ")",
				Err:  g.Check(),
			}
		})
	}

	// Platforms
	if cfg.Release != nil {
		for i := range cfg.Release.Platforms {
			platCfg := &cfg.Release.Platforms[i]
			dispatch("platform "+platCfg.Type, func() RuntimeCheckItem {
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: "platform " + platCfg.Type, Err: buildErr}
				}
				return RuntimeCheckItem{Name: p.Name(), Err: p.Check()}
			})
		}
	}

	// git user.name
	dispatch("git user.name", func() RuntimeCheckItem {
		gitName, _, nameErr := runner.Run("git", "config", "user.name")
		if nameErr != nil || strings.TrimSpace(gitName) == "" {
			return RuntimeCheckItem{
				Name: "git user.name",
				Err:  fmt.Errorf("not configured; run: git config user.name <name>"),
			}
		}
		return RuntimeCheckItem{Name: "git user.name", Value: strings.TrimSpace(gitName)}
	})

	// git user.email
	dispatch("git user.email", func() RuntimeCheckItem {
		gitEmail, _, emailErr := runner.Run("git", "config", "user.email")
		if emailErr != nil || strings.TrimSpace(gitEmail) == "" {
			return RuntimeCheckItem{
				Name: "git user.email",
				Err:  fmt.Errorf("not configured; run: git config user.email <email>"),
			}
		}
		return RuntimeCheckItem{Name: "git user.email", Value: strings.TrimSpace(gitEmail)}
	})
}

// CheckCliff runs git-cliff --context --no-exec against the effective merged config
// for the given content driver. mode must be "changelog" or "release-notes".
// Returns nil if git-cliff accepts the config.
func CheckCliff(runner port.Runner, driver *config.ContentDriver, mode string) error {
	m := gitcliff.ModeChangelog
	if mode == "release-notes" {
		m = gitcliff.ModeReleaseNotes
	}
	return gitcliff.New(runner, driver, m).CheckCliff()
}
