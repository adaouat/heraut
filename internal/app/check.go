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

// RuntimeCheck verifies all runtime dependencies, grouped into three sections:
// Git, Platforms, and Generators. header is called once per section before its
// items. dispatch is called once per item with the label shown while the check
// runs; the run function performs the check and returns the result.
//
// Check order:
//
//	Git:        git binary → user.name → user.email → working tree
//	Platforms:  glab (GitLab) → gh (GitHub)
//	Generators: git-cliff → cocogitto → communique
//
// Configured tools are hard errors when missing; unconfigured-but-supported
// tools warn when absent and succeed silently when present.
func RuntimeCheck(
	runner port.Runner,
	cfg *config.Config,
	header func(title string),
	dispatch func(name string, run func() RuntimeCheckItem),
) {
	// ── Git ──────────────────────────────────────────────────────────────────
	header("Git")

	dispatch("git", func() RuntimeCheckItem {
		out, _, err := runner.Run("git", "--version")
		if err != nil {
			return RuntimeCheckItem{Name: "git", Err: err}
		}
		return RuntimeCheckItem{Name: "git", Value: strings.TrimSpace(out)}
	})

	dispatch("git user.name", func() RuntimeCheckItem {
		name, _, err := runner.Run("git", "config", "user.name")
		if err != nil || strings.TrimSpace(name) == "" {
			return RuntimeCheckItem{
				Name: "git user.name",
				Err:  fmt.Errorf("not configured; run: git config user.name <name>"),
			}
		}
		return RuntimeCheckItem{Name: "git user.name", Value: strings.TrimSpace(name)}
	})

	dispatch("git user.email", func() RuntimeCheckItem {
		email, _, err := runner.Run("git", "config", "user.email")
		if err != nil || strings.TrimSpace(email) == "" {
			return RuntimeCheckItem{
				Name: "git user.email",
				Err:  fmt.Errorf("not configured; run: git config user.email <email>"),
			}
		}
		return RuntimeCheckItem{Name: "git user.email", Value: strings.TrimSpace(email)}
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

	// ── Platforms ─────────────────────────────────────────────────────────────
	header("Platforms")

	usedPlats := configuredPlatforms(cfg)
	for _, op := range []struct{ typ, binary, display string }{
		{"gitlab", "glab", "glab"},
		{"github", "gh", "gh"},
	} {
		op := op
		required := usedPlats[op.typ]
		dispatch(op.display, func() RuntimeCheckItem {
			if required && cfg != nil {
				// Full check: binary + token + project + API auth.
				platCfg := findPlatformCfg(cfg, op.typ)
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: op.display, Err: buildErr}
				}
				if err := p.Check(); err != nil {
					return RuntimeCheckItem{Name: op.display, Err: err}
				}
				return RuntimeCheckItem{Name: op.display}
			}
			// Binary-only check (no config available for token/project resolution).
			// Missing binary is a hard error when required, advisory otherwise.
			out, _, err := runner.Run(op.binary, "--version")
			if err != nil {
				if required {
					return RuntimeCheckItem{Name: op.display,
						Err: fmt.Errorf("%s: not found on PATH", op.binary)}
				}
				return RuntimeCheckItem{Name: op.display, IsWarn: true,
					Err: fmt.Errorf("not found (not required by this config)")}
			}
			return RuntimeCheckItem{Name: op.display, Value: strings.TrimSpace(out)}
		})
	}

	// ── Generators ────────────────────────────────────────────────────────────
	header("Generators")

	usedGens := configuredGenerators(cfg)
	for _, og := range []struct{ name, binary, display string }{
		{"git-cliff", "git-cliff", "git-cliff"},
		{"cocogitto", "cog", "cocogitto"},
		{"communique", "communique", "communique"},
	} {
		og := og
		required := usedGens[og.name]
		dispatch(og.display, func() RuntimeCheckItem {
			out, _, err := runner.Run(og.binary, "--version")
			if err != nil {
				if required {
					return RuntimeCheckItem{Name: og.display, Err: fmt.Errorf("%s: not found on PATH", og.binary)}
				}
				return RuntimeCheckItem{Name: og.display, IsWarn: true,
					Err: fmt.Errorf("not found (not required by this config)")}
			}
			return RuntimeCheckItem{Name: og.display, Value: strings.TrimSpace(out)}
		})
	}
}

// configuredGenerators returns the set of generator names active in cfg.
// When cfg is nil (no config file found) all supported generators are required.
func configuredGenerators(cfg *config.Config) map[string]bool {
	if cfg == nil {
		return map[string]bool{"git-cliff": true, "cocogitto": true, "communique": true}
	}
	m := make(map[string]bool)
	if cfg.Changelog != nil {
		m[cfg.Changelog.Generator] = true
	}
	if cfg.Release != nil && cfg.Release.Notes != nil {
		m[cfg.Release.Notes.Generator] = true
	}
	return m
}

// configuredPlatforms returns the set of platform types active in cfg.
// When cfg is nil (no config file found) all supported platforms are required.
func configuredPlatforms(cfg *config.Config) map[string]bool {
	if cfg == nil {
		return map[string]bool{"github": true, "gitlab": true}
	}
	m := make(map[string]bool)
	if cfg.Release != nil {
		for _, p := range cfg.Release.Platforms {
			m[p.Type] = true
		}
	}
	return m
}

// findPlatformCfg returns the config for the platform of the given type, or nil.
func findPlatformCfg(cfg *config.Config, typ string) *config.Platform {
	if cfg.Release == nil {
		return nil
	}
	for i := range cfg.Release.Platforms {
		if cfg.Release.Platforms[i].Type == typ {
			return &cfg.Release.Platforms[i]
		}
	}
	return nil
}

// CheckCliff runs git-cliff --context --no-exec against the effective merged config
// for the given content driver, applying the remote_metadata policy. mode must be
// "changelog" or "release-notes". Returns whether the check fell back to --offline
// (degraded, optional policy) and an error if git-cliff rejected the config. The
// caller's driver is never mutated — the policy is applied to a copy.
func CheckCliff(runner port.Runner, driver *config.ContentDriver, mode, policy string) (bool, error) {
	m := gitcliff.ModeChangelog
	if mode == "release-notes" {
		m = gitcliff.ModeReleaseNotes
	}
	d := *driver
	d.RemoteMetadata = policy
	gen := gitcliff.New(runner, &d, m)
	if err := gen.CheckCliff(); err != nil {
		return false, err
	}
	return gen.Degraded(), nil
}
