package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)

// Platform implements port.Platform for GitHub over the REST API (ADR-0077).
type Platform struct {
	cfg        *config.Platform
	getenv     func(string) string
	stderr     io.Writer
	httpClient *http.Client // nil: a default client
}

var _ port.Platform = (*Platform)(nil)

// New constructs a GitHub Platform.
func New(cfg *config.Platform) *Platform {
	return &Platform{cfg: cfg, getenv: os.Getenv, stderr: os.Stderr}
}

func (p *Platform) Name() string { return p.cfg.Name }

func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("%s/%s/releases/tag/%s", p.baseURL(), p.repository(), port.URLTag(tag))
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
		return base + "/releases/tag/" + port.URLTag(tag)
	}
	return base + "/" + lc.Owner + "/" + lc.Repo + "/releases/tag/" + port.URLTag(tag)
}

// LinkContext resolves this platform's link coordinates. GitHub repositories are
// owner/repo, so the path splits on the first slash. BaseURL falls back to the default
// host when unset (e.g. a config that was not run through the loader's normalize step).
func (p *Platform) LinkContext() port.LinkContext {
	owner, repo, _ := strings.Cut(p.repository(), "/")
	return port.LinkContext{
		BaseURL:  p.baseURL(),
		Owner:    owner,
		Repo:     repo,
		Platform: "github",
	}
}

// Check verifies a token is available, the repository is resolvable, and the token can read the
// repository's releases.
func (p *Platform) Check() error {
	var errs []error
	token, tokenEnv, tokenErr := p.token()
	if tokenErr != nil {
		errs = append(errs, tokenErr)
	}
	if _, _, err := splitRepository(p.repository()); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	c, err := p.client(token)
	if err != nil {
		return err
	}
	if err := c.VerifyReleasesAccess(); err != nil {
		return fmt.Errorf("github: API call failed: %w\n  hint: verify %s is valid and can read the repository's releases", err, tokenEnv)
	}
	return nil
}

// CreateRelease publishes the release through the REST API (Client.CreateRelease). Asset globs
// resolve leniently: the tag is already pushed by now, so a pattern matching nothing warns and is
// skipped rather than failing the release. Each asset is opened once and uploaded from that
// descriptor (NEW-002), and a name GitHub rewrites is reported. prerelease is derived by the
// caller from the resolved version (ADR-0064), not read from config.
func (p *Platform) CreateRelease(tag, notes string, prerelease bool) error {
	token, _, err := p.token()
	if err != nil {
		return fmt.Errorf("github release create: %w", err)
	}
	c, err := p.client(token)
	if err != nil {
		return fmt.Errorf("github release create: %w", err)
	}

	var assets []platforms.Asset
	if len(p.cfg.Assets) > 0 {
		assets, err = platforms.OpenAssets(p.cfg.Assets, func(pattern string) {
			_, _ = fmt.Fprintf(p.stderr, "warning: no files matched asset pattern %q — skipping\n", pattern)
		})
		if err != nil {
			return fmt.Errorf("github release create: resolving assets: %w", err)
		}
		defer platforms.CloseAssets(assets)
	}

	names, err := c.CreateRelease(tag, notes, p.cfg.Draft, prerelease, assets)
	if err != nil {
		return fmt.Errorf("github release create: %w", err)
	}
	for i, a := range assets {
		if names[i] != a.Name {
			_, _ = fmt.Fprintf(p.stderr, "warning: asset %q was published as %q: GitHub rewrote its name\n", a.Path, names[i])
		}
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// CheckAssets applies the asset rules (confinement to the repository, and every pattern matching a
// file unless release.allow_missing_assets is set) ahead of publishing, and refuses two files GitHub
// would publish under one name: it renames special characters and compares names without case.
func (p *Platform) CheckAssets() error {
	return platforms.CheckAssetGlobsFor(p.cfg.Assets, p.cfg.AllowMissingAssets, platforms.NameRule{
		Forge: "GitHub",
		Key:   func(name string) string { return strings.ToLower(publishedName(name)) },
	})
}

func (p *Platform) repository() string {
	if p.cfg.Repository != "" {
		return p.cfg.Repository
	}
	return p.getenv(config.EnvGitHubRepository)
}

// tokenEnvs lists the variables the token is read from, first set wins (ADR-0077): token_env alone
// when configured, else gh's chain for the host. GH_TOKEN comes last on GitHub Enterprise Server so a
// configuration that worked with the gh driver keeps working.
func (p *Platform) tokenEnvs() []string {
	if p.cfg.TokenEnv != "" {
		return []string{p.cfg.TokenEnv}
	}
	if enterpriseServer(p.baseURL()) {
		return []string{config.EnvGitHubEnterpriseCLIToken, config.EnvGitHubEnterpriseToken, config.EnvGitHubCLIToken}
	}
	return []string{config.EnvGitHubCLIToken, config.EnvGitHubToken}
}

// token returns the token and the variable it was read from.
func (p *Platform) token() (token, envName string, err error) {
	envs := p.tokenEnvs()
	for _, name := range envs {
		if t := p.getenv(name); t != "" {
			return t, name, nil
		}
	}
	if len(envs) == 1 {
		return "", "", fmt.Errorf("environment variable %s is not set", envs[0])
	}
	return "", "", fmt.Errorf("no GitHub token: none of %s is set", strings.Join(envs, ", "))
}

func (p *Platform) client(token string) (*Client, error) {
	return newClient(p.baseURL(), p.repository(), token, p.getenv, p.httpClient)
}

func (p *Platform) baseURL() string {
	if p.cfg.BaseURL != "" {
		return p.cfg.BaseURL
	}
	return config.DefaultGitHubBaseURL
}

// enterpriseServer reports whether baseURL is a GitHub Enterprise Server rather than github.com or a
// GitHub Enterprise Cloud tenant on *.ghe.com.
func enterpriseServer(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	return host != "github.com" && !strings.HasSuffix(host, ".ghe.com")
}
