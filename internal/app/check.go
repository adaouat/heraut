package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
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
// runs; the run function performs the check and returns the result. ctx is the
// command's context: Ctrl-C stops a platform's API call.
//
// env selects the active environment for the Platforms section: when non-empty
// and cfg.Environments[env].Release.Targets is non-empty, it replaces the root
// release.targets list entirely (same semantics as the release pipeline's
// effective-targets resolution). An empty env, or an env with no target
// override, checks the root list.
//
// Check order:
//
//	Git:        git binary → user.name → user.email → working tree → history
//	Platforms:  one row per effective release.targets entry (resolved against
//	            forges:/CI env/git origin, same as `heraut release`), or an
//	            advisory "publish" row when nothing resolves
func RuntimeCheck(
	ctx context.Context,
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

	// history: release and changelog refuse a shallow clone, so it fails the check too
	dispatch("history", func() RuntimeCheckItem {
		if err := CheckFullHistory(runner); err != nil {
			return RuntimeCheckItem{Name: "history", Err: err}
		}
		return RuntimeCheckItem{Name: "history", Value: "full"}
	})

	// The forge is resolved once (one `git remote get-url origin`), for PR data and for publishing.
	var (
		resolved   forge.Resolved
		resolveErr error
	)
	if cfg != nil {
		resolved, resolveErr = resolveForge(runner, os.Getenv, cfg)
	}

	// ── PR data ───────────────────────────────────────────────────────────────
	if cfg != nil {
		header("PR data")
		dispatch("pr data", func() RuntimeCheckItem { return prDataCheck(cfg, resolved, resolveErr) })
	}

	// ── Platforms ─────────────────────────────────────────────────────────────
	header("Platforms")

	if cfg != nil && !PublishesReleases(cfg, env) {
		// No release: block: heraut release tags only, so no destination is resolved or checked.
		dispatch("publish", func() RuntimeCheckItem {
			return RuntimeCheckItem{Name: "publish", Value: "off (no release: block)"}
		})
		return
	}

	platCfgs, resolveErr := effectiveTargetPlatforms(cfg, env, resolved, resolveErr)

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
		// One row per effective target's resolved platform: full check (token +
		// project/repository + API auth), labeled by the resolved forge name.
		for i := range platCfgs {
			platCfg := &platCfgs[i]
			name := platCfg.Name
			dispatch(name, func() RuntimeCheckItem {
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: name, Err: buildErr}
				}
				if err := p.Check(ctx); err != nil {
					return RuntimeCheckItem{Name: name, Err: err}
				}
				return RuntimeCheckItem{Name: name}
			})
		}
	default:
		// Nothing to publish to: heraut release would refuse to run, but a changelog-only user never
		// publishes, so this warns rather than fails.
		reason := "no release target resolves to github or gitlab"
		if cfg == nil {
			reason = "no config found, destinations not checked"
		}
		dispatch("publish", func() RuntimeCheckItem {
			return RuntimeCheckItem{Name: "publish", IsWarn: true, Err: errors.New(reason)}
		})
	}
}

// effectiveTargetPlatforms resolves the config.Platform heraut would build for each effective
// release.targets entry (or the zero-config synthesized target when the list is empty), mirroring
// buildTargetPlatforms/resolveTargetForge without constructing the port.Platform driver — the
// check needs the resolved name/type before deciding whether to build it. Returns (nil, nil) when
// cfg is nil (no config file found — nothing to resolve against) or when no target resolves to a
// forge; returns a non-nil error only when resolution itself fails (e.g. an ambiguous zero-config
// environment), matching heraut release's own forge-resolution failure mode.
func effectiveTargetPlatforms(cfg *config.Config, env string, resolved forge.Resolved, resolveErr error) ([]config.Platform, error) {
	if cfg == nil {
		return nil, nil
	}
	if resolveErr != nil {
		return nil, resolveErr
	}

	targets := config.EffectiveTargets(cfg, env)

	if len(targets) == 0 {
		targets = synthesizeDefaultTarget(resolved)
	}

	platCfgs := make([]config.Platform, 0, len(targets))
	for _, t := range targets {
		f, id, err := resolveTargetForge(cfg, t, resolved)
		if err != nil {
			return nil, err
		}
		platCfg := platformConfigFromTarget(t, f, id)
		platCfg.AllowMissingAssets = cfg.Release != nil && cfg.Release.AllowMissingAssets
		platCfgs = append(platCfgs, platCfg)
	}
	return platCfgs, nil
}

// prDataCheck reports the forge changelog, release and notes ask for pull request data: they ask it
// whenever one resolves and enrichment is on, with or without a token, so a missing token is a
// warning (the request is sent and fails, and the changelog goes without PR data).
func prDataCheck(cfg *config.Config, resolved forge.Resolved, resolveErr error) RuntimeCheckItem {
	const name = "pr data"
	switch {
	case cfg.EnrichmentPolicy() == "disabled":
		return RuntimeCheckItem{Name: name, Value: "off (enrichment_policy: disabled, or --offline): no API call"}
	case resolveErr != nil:
		return RuntimeCheckItem{Name: name, IsWarn: true, Err: fmt.Errorf("no forge to ask: %w", resolveErr)}
	case len(resolved.Forges) == 0:
		return RuntimeCheckItem{Name: name, Value: "none: no forge found, commit lines only"}
	}
	id := resolved.Forges[resolved.EnrichmentIndex]
	where := fmt.Sprintf("%s %s on %s", id.Type, id.Project, id.Host)
	if id.Token == "" {
		return RuntimeCheckItem{Name: name, IsWarn: true, Err: fmt.Errorf(
			"%s, no token: changelog, release and notes still call its API and go without PR data; set its token, or enrichment_policy: disabled", where)}
	}
	return RuntimeCheckItem{Name: name, Value: where}
}
