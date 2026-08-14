package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
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

// RuntimeCheck verifies all runtime dependencies, grouped into two sections:
// Git and Platforms. header is called once per section before its items.
// dispatch is called once per item with the label shown while the check
// runs; the run function performs the check and returns the result.
//
// env selects the active environment for the Platforms section: when non-empty
// and cfg.Environments[env].Release.Targets is non-empty, it replaces the root
// release.targets list entirely (same semantics as the release pipeline's
// effective-targets resolution). An empty env, or an env with no target
// override, checks the root list.
//
// Check order:
//
//	Git:        git binary → user.name → user.email → working tree
//	Platforms:  one row per effective release.targets entry (resolved against
//	            forges:/CI env/git origin, same as `heraut release`), or
//	            glab (GitLab) → gh (GitHub) as a binary-only fallback when
//	            nothing resolves
//
// Configured tools are hard errors when missing; unconfigured-but-supported
// tools warn when absent and succeed silently when present.
func RuntimeCheck(
	runner port.Runner,
	cfg *config.Config,
	env string,
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

	platCfgs, resolveErr := effectiveTargetPlatforms(runner, cfg, env)

	switch {
	case resolveErr != nil:
		// Forge resolution failed (e.g. an ambiguous multi-token machine with no CI/origin to
		// disambiguate). Whether that is a hard failure depends on whether the user actually asked
		// for a publish destination: explicit forges: or a non-empty effective release.targets
		// means `heraut release` will hit this same error, so report it as a hard failure. With
		// neither configured, heraut was only attempting zero-config detection for a user who may
		// be changelog-only and never publish — report it as an advisory warning instead, so
		// `heraut check` (commonly a CI gate) doesn't fail on a destination nobody asked for.
		wantsForge := len(cfg.Forges) > 0 || len(config.EffectiveTargets(cfg, env)) > 0
		dispatch("forge", func() RuntimeCheckItem {
			return RuntimeCheckItem{Name: "forge", Err: resolveErr, IsWarn: !wantsForge}
		})
	case len(platCfgs) > 0:
		// One row per effective target's resolved platform: full check (binary + token +
		// project/repository + API auth), labeled by the resolved forge name.
		for i := range platCfgs {
			platCfg := &platCfgs[i]
			name := platCfg.Name
			dispatch(name, func() RuntimeCheckItem {
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: name, Err: buildErr}
				}
				if err := p.Check(); err != nil {
					return RuntimeCheckItem{Name: name, Err: err}
				}
				return RuntimeCheckItem{Name: name}
			})
		}
	default:
		// No target resolves to a publishable platform: fall back to a binary-only probe of both
		// supported CLIs. Required (hard error) when cfg is nil (no config file found, so both
		// could plausibly be needed); advisory otherwise.
		required := cfg == nil
		for _, bin := range []string{"glab", "gh"} {
			dispatch(bin, func() RuntimeCheckItem {
				out, _, err := runner.Run(bin, "--version")
				if err != nil {
					if required {
						return RuntimeCheckItem{Name: bin,
							Err: fmt.Errorf("%s: not found on PATH", bin)}
					}
					return RuntimeCheckItem{Name: bin, IsWarn: true,
						Err: fmt.Errorf("not found (not required by this config)")}
				}
				return RuntimeCheckItem{Name: bin, Value: strings.TrimSpace(out)}
			})
		}
	}
}

// effectiveTargetPlatforms resolves the config.Platform heraut would build for each effective
// release.targets entry (or the zero-config synthesized target when the list is empty), mirroring
// buildTargetPlatforms/resolveTargetForge without constructing the port.Platform driver — the
// check needs the resolved name/type before deciding whether to build it. Returns (nil, nil) when
// cfg is nil (no config file found — nothing to resolve against) or when no target resolves to a
// forge; returns a non-nil error only when resolution itself fails (e.g. an ambiguous zero-config
// environment), matching heraut release's own forge-resolution failure mode.
func effectiveTargetPlatforms(runner port.Runner, cfg *config.Config, env string) ([]config.Platform, error) {
	if cfg == nil {
		return nil, nil
	}

	targets := config.EffectiveTargets(cfg, env)
	resolved, err := resolveForge(runner, os.Getenv, cfg)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		if len(resolved.Forges) == 0 {
			return nil, nil
		}
		targets = []config.Target{{}}
	}

	platCfgs := make([]config.Platform, 0, len(targets))
	for _, t := range targets {
		f, id, err := resolveTargetForge(cfg, t, resolved)
		if err != nil {
			return nil, err
		}
		platCfgs = append(platCfgs, platformConfigFromTarget(t, f, id))
	}
	return platCfgs, nil
}
