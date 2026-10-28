package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)

// Platform implements port.Platform for GitLab over the REST API (ADR-0069).
type Platform struct {
	cfg *config.Platform
}

var _ port.Platform = (*Platform)(nil)

// New constructs a GitLab Platform.
func New(cfg *config.Platform) *Platform {
	return &Platform{cfg: cfg}
}

func (p *Platform) Name() string { return p.cfg.Name }

func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("%s/%s/-/releases/%s", p.resolveBaseURL(), p.project(), releaseTag(tag))
}

// releaseTag escapes tag for GitLab's release route, which takes the tag as a single path
// segment: on top of port.URLTag's "+", a per-env tag's "/" must be sent as %2F.
func releaseTag(tag string) string {
	return strings.ReplaceAll(port.URLTag(tag), "/", "%2F")
}

// ReleaseURLFromContext builds the release URL from a pre-resolved link context so the
// displayed URL stays consistent with the one used to generate release notes (ADR-0022).
// Ambient contexts carry the full project URL in BaseURL (Owner/Repo empty); platform
// contexts carry the host in BaseURL with Owner and Repo set separately.
func (p *Platform) ReleaseURLFromContext(tag string, lc *port.LinkContext) string {
	if lc == nil {
		return p.ReleaseURL(tag)
	}
	base := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner == "" && lc.Repo == "" {
		return base + "/-/releases/" + releaseTag(tag)
	}
	return base + "/" + lc.Owner + "/" + lc.Repo + "/-/releases/" + releaseTag(tag)
}

// LinkContext resolves this platform's link coordinates. GitLab projects may be nested
// (group/subgroup/project), so the path splits on the last slash: everything before is
// the owner (namespace), the final segment is the repo. BaseURL falls back to the default
// host when unset (e.g. a config that was not run through the loader's normalize step).
func (p *Platform) LinkContext() port.LinkContext {
	proj := p.project()
	owner, repo := "", proj
	if i := strings.LastIndex(proj, "/"); i >= 0 {
		owner, repo = proj[:i], proj[i+1:]
	}
	return port.LinkContext{
		BaseURL:  p.resolveBaseURL(),
		Owner:    owner,
		Repo:     repo,
		Platform: "gitlab",
	}
}

// Check verifies a token is available, the project is resolvable, and the token authenticates
// against the GitLab API.
func (p *Platform) Check() error {
	var errs []error

	token, jobToken, tokenErr := p.credentials()
	if tokenErr != nil {
		errs = append(errs, tokenErr)
	}

	if p.project() == "" {
		errs = append(errs, fmt.Errorf("project not set: configure project: in .heraut.yml or set $%s", config.EnvGitLabProjectPath))
	}

	if tokenErr == nil {
		if err := p.checkAPIAuth(token, jobToken); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// credentials picks the token and how it is sent. In CI autologin (the job's own instance) the
// job token (CI_JOB_TOKEN, sent as JOB-TOKEN) is used; a missing job token, or any other run,
// uses the configured token (sent as PRIVATE-TOKEN).
func (p *Platform) credentials() (token string, jobToken bool, err error) {
	if p.inCIAutologin() {
		if t := os.Getenv(config.EnvGitLabJobToken); t != "" {
			return t, true, nil
		}
	}
	envName := p.tokenEnv()
	if t := os.Getenv(envName); t != "" {
		return t, false, nil
	}
	return "", false, fmt.Errorf("environment variable %s is not set", envName)
}

func (p *Platform) client() (*Client, error) {
	token, jobToken, err := p.credentials()
	if err != nil {
		return nil, err
	}
	return NewClient(p.resolveBaseURL(), token, jobToken)
}

// checkAPIAuth verifies API access with the call each token kind can make: a job token cannot
// reach /user, so it reads the project's releases (by CI_PROJECT_ID when set), while a personal
// token is validated through /user.
func (p *Platform) checkAPIAuth(token string, jobToken bool) error {
	c, err := NewClient(p.resolveBaseURL(), token, jobToken)
	if err != nil {
		return err
	}
	if jobToken {
		project := os.Getenv(config.EnvGitLabProjectID)
		if project == "" {
			project = p.project()
		}
		if err := c.VerifyReleasesAccess(project); err != nil {
			return fmt.Errorf("gitlab: API call failed (list releases of %s): %w\n  hint: ensure CI_JOB_TOKEN has read access (Settings > CI/CD > Job token permissions)", project, err)
		}
		return nil
	}
	if err := c.VerifyUser(); err != nil {
		return fmt.Errorf("gitlab: API call failed (GET /user): %w\n  hint: verify %s is valid and has the api scope", err, p.tokenEnv())
	}
	return nil
}

// selfHosted reports whether this platform targets a non-default GitLab host.
func (p *Platform) selfHosted() bool {
	return p.cfg.BaseURL != "" && p.cfg.BaseURL != config.DefaultGitLabBaseURL
}

// inCIAutologin reports whether this run authenticates with the CI job token (CI_JOB_TOKEN is
// not a valid PRIVATE-TOKEN, so the two are never mixed).
//
// It applies to the default gitlab.com host, and — as of T215 — to a self-hosted base_url naming
// the exact instance the CI job itself is running on (CI_SERVER_URL), so that isn't the
// "separately configured" self-hosted target ADR-0025 §4 guards against. A self-hosted base_url
// naming a genuinely different instance still requires the configured token — the job token
// cannot reach a host other than its own.
func (p *Platform) inCIAutologin() bool {
	if os.Getenv(config.EnvGitLabCI) != "true" {
		return false
	}
	if !p.selfHosted() {
		return true
	}
	return sameGitLabHost(p.cfg.BaseURL, os.Getenv(config.EnvGitLabServerURL))
}

// sameGitLabHost reports whether a and b name the same GitLab instance, comparing scheme and host
// (a trailing slash or letter case is ignored). Either side missing or unparsable is "no". The
// scheme counts: trusting the job token for an http URL when the runner is https would send it in
// the clear.
func sameGitLabHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil || ua.Host == "" || ub.Host == "" {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host)
}

// CreateRelease opens the asset files once (platforms.OpenAssets), uploads them and creates the
// release (Client.CreateRelease). As on GitHub, a pattern matching nothing warns and is skipped.
// prerelease is accepted to satisfy the shared port.Platform contract and ignored — GitLab
// releases have no pre-release concept (ADR-0064).
func (p *Platform) CreateRelease(ctx context.Context, tag, notes string, prerelease bool) error {
	proj, err := p.requireProject()
	if err != nil {
		return err
	}

	var assets []platforms.Asset
	if len(p.cfg.Assets) > 0 {
		assets, err = platforms.OpenAssets(p.cfg.Assets, func(pattern string) {
			_, _ = fmt.Fprintf(os.Stderr, "warning: no files matched asset pattern %q — skipping\n", pattern)
		})
		if err != nil {
			return fmt.Errorf("gitlab release create: resolving assets: %w", err)
		}
		defer platforms.CloseAssets(assets)
	}

	c, err := p.client()
	if err != nil {
		return fmt.Errorf("gitlab release create: %w", err)
	}
	if err := c.CreateRelease(ctx, proj, tag, notes, assets); err != nil {
		return fmt.Errorf("gitlab release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// CheckAssets applies the asset rules (confinement to the repository, and every pattern matching a
// file unless release.allow_missing_assets is set) ahead of publishing.
func (p *Platform) CheckAssets() error {
	return platforms.CheckAssetGlobs(p.cfg.Assets, p.cfg.AllowMissingAssets)
}

func (p *Platform) project() string {
	if p.cfg.Project != "" {
		return p.cfg.Project
	}
	return os.Getenv(config.EnvGitLabProjectPath)
}

func (p *Platform) requireProject() (string, error) {
	proj := p.project()
	if proj == "" {
		return "", fmt.Errorf("project not set: configure project: in .heraut.yml or set $%s", config.EnvGitLabProjectPath)
	}
	return proj, nil
}

func (p *Platform) tokenEnv() string {
	if p.cfg.TokenEnv != "" {
		return p.cfg.TokenEnv
	}
	return config.EnvGitLabToken
}

// resolveBaseURL returns the effective base URL for this GitLab instance. Resolution order:
//  1. cfg.BaseURL (explicit config — always wins)
//  2. CI_SERVER_URL when GITLAB_CI=true (modern GitLab CI, guaranteed to be the instance URL)
//  3. scheme+host extracted from CI_PROJECT_URL (no GITLAB_CI gate — mirrors how
//     ambientLinkContext() uses CI_PROJECT_URL unconditionally; covers custom CI setups and
//     containers that expose CI_PROJECT_URL without setting GITLAB_CI)
//  4. gitlab.com (public fallback for local / non-CI runs without explicit config)
func (p *Platform) resolveBaseURL() string {
	if p.cfg.BaseURL != "" {
		return p.cfg.BaseURL
	}
	if os.Getenv(config.EnvGitLabCI) == "true" {
		if u := os.Getenv(config.EnvGitLabServerURL); u != "" {
			return u
		}
	}
	if u := os.Getenv(config.EnvGitLabProjectURL); u != "" {
		if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	return config.DefaultGitLabBaseURL
}
