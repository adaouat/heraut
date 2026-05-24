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
	Name string
	Err  error
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

// RuntimeCheck verifies all runtime dependencies: git binary, configured generator
// and platform CLIs (including token env vars), and git user name/email.
func RuntimeCheck(runner port.Runner, cfg *config.Config) []RuntimeCheckItem {
	var items []RuntimeCheckItem

	// git binary
	_, _, err := runner.Run("git", "--version")
	items = append(items, RuntimeCheckItem{Name: "git", Err: err})

	// Changelog generator
	if cfg.Changelog != nil {
		gen, buildErr := buildGenerator(runner, cfg.Changelog, gitcliff.ModeChangelog)
		if buildErr != nil {
			items = append(items, RuntimeCheckItem{Name: "changelog generator", Err: buildErr})
		} else {
			items = append(items, RuntimeCheckItem{
				Name: "changelog generator (" + cfg.Changelog.Generator + ")",
				Err:  gen.Check(),
			})
		}
	}

	// Release notes generator
	if cfg.Release != nil && cfg.Release.Notes != nil {
		gen, buildErr := buildGenerator(runner, cfg.Release.Notes, gitcliff.ModeReleaseNotes)
		if buildErr != nil {
			items = append(items, RuntimeCheckItem{Name: "release-notes generator", Err: buildErr})
		} else {
			items = append(items, RuntimeCheckItem{
				Name: "release-notes generator (" + cfg.Release.Notes.Generator + ")",
				Err:  gen.Check(),
			})
		}
	}

	// Platforms
	if cfg.Release != nil {
		for i := range cfg.Release.Platforms {
			p, buildErr := buildPlatform(runner, &cfg.Release.Platforms[i])
			if buildErr != nil {
				items = append(items, RuntimeCheckItem{
					Name: "platform " + cfg.Release.Platforms[i].Type,
					Err:  buildErr,
				})
			} else {
				items = append(items, RuntimeCheckItem{Name: p.Name(), Err: p.Check()})
			}
		}
	}

	// git user.name
	gitName, _, nameErr := runner.Run("git", "config", "user.name")
	if nameErr != nil || strings.TrimSpace(gitName) == "" {
		items = append(items, RuntimeCheckItem{
			Name: "git user.name",
			Err:  fmt.Errorf("not configured; run: git config user.name <name>"),
		})
	} else {
		items = append(items, RuntimeCheckItem{Name: "git user.name"})
	}

	// git user.email
	gitEmail, _, emailErr := runner.Run("git", "config", "user.email")
	if emailErr != nil || strings.TrimSpace(gitEmail) == "" {
		items = append(items, RuntimeCheckItem{
			Name: "git user.email",
			Err:  fmt.Errorf("not configured; run: git config user.email <email>"),
		})
	} else {
		items = append(items, RuntimeCheckItem{Name: "git user.email"})
	}

	return items
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
