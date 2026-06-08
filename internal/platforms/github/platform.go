package github

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)

const (
	defaultTokenEnv = "GH_TOKEN"
	repoEnvVar      = "GITHUB_REPOSITORY"
	githubBaseURL   = "https://github.com"
)

// Platform implements port.Platform for GitHub via the gh CLI.
type Platform struct {
	runner port.Runner
	cfg    *config.Platform
}

var _ port.Platform = (*Platform)(nil)

// New constructs a GitHub Platform.
func New(runner port.Runner, cfg *config.Platform) *Platform {
	return &Platform{runner: runner, cfg: cfg}
}

func (p *Platform) Name() string { return "github" }

func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/releases/tag/%s", p.repository(), tag)
}

// LinkContext resolves this platform's link coordinates. GitHub repositories are
// owner/repo, so the path splits on the first slash. BaseURL falls back to the default
// host when unset (e.g. a config that was not run through the loader's normalize step).
func (p *Platform) LinkContext() port.LinkContext {
	owner, repo, _ := strings.Cut(p.repository(), "/")
	baseURL := p.cfg.BaseURL
	if baseURL == "" {
		baseURL = githubBaseURL
	}
	return port.LinkContext{
		BaseURL:  baseURL,
		Owner:    owner,
		Repo:     repo,
		Platform: "github",
	}
}

// Check verifies gh is on PATH, the token env var is set, repository is resolvable,
// and the token authenticates successfully against the GitHub API.
func (p *Platform) Check() error {
	var errs []error

	_, _, binaryErr := p.runner.Run("gh", "--version")
	if binaryErr != nil {
		errs = append(errs, fmt.Errorf("gh not found: %w", binaryErr))
	}

	tokenEnvName := p.tokenEnv()
	tokenMissing := os.Getenv(tokenEnvName) == ""
	if tokenMissing {
		errs = append(errs, fmt.Errorf("environment variable %s is not set", tokenEnvName))
	}

	if p.repository() == "" {
		errs = append(errs, fmt.Errorf("repository not set: configure repository: in .heraut.yml or set $%s", repoEnvVar))
	}

	if binaryErr == nil {
		if err := p.checkAPIAuth(tokenMissing); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// checkAPIAuth verifies API access. In GitHub Actions, GITHUB_TOKEN is used directly
// because gh auth status reads config files and won't see the injected env var.
// Outside Actions, the configured token is validated via a repo-scoped API call.
func (p *Platform) checkAPIAuth(tokenMissing bool) error {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		githubToken := os.Getenv("GITHUB_TOKEN")
		repo := os.Getenv("GITHUB_REPOSITORY")
		if githubToken == "" || repo == "" {
			return nil
		}
		endpoint := "repos/" + repo + "/releases?per_page=1"
		_, stderr, err := p.runner.RunEnv([]string{"GH_TOKEN=" + githubToken}, "gh", "api", endpoint)
		if err != nil {
			return fmt.Errorf("github: API call failed (gh api %s): %s\n  hint: verify GITHUB_TOKEN has read access to the repository", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	_, stderr, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", "api", "repos/{owner}/{repo}/releases?per_page=1")
	if err != nil {
		return fmt.Errorf("github: API call failed (gh api repos/{owner}/{repo}/releases): %s\n  hint: verify %s is valid and has the necessary scopes", strings.TrimSpace(stderr), p.tokenEnv())
	}
	return nil
}

// CreateRelease runs `gh release create`.
// When cfg.LenientAssets is true (release-level assets), resolved asset files are
// included as positional args so the create and upload are atomic — this avoids
// GitHub's HTTP 422 "Cannot upload assets to an immutable release" that occurs when
// uploading to an already-published release via a separate gh release upload call.
func (p *Platform) CreateRelease(tag, notes string) error {
	repo, err := p.requireRepository()
	if err != nil {
		return err
	}

	args := []string{"release", "create", tag, "--notes", notes, "--repo", repo}
	if p.cfg.Draft {
		args = append(args, "--draft")
	}
	if p.cfg.Prerelease {
		args = append(args, "--prerelease")
	}

	if p.cfg.LenientAssets && len(p.cfg.Assets) > 0 {
		files, err := platforms.ResolveGlobsLenient(p.cfg.Assets, func(pattern string) {
			_, _ = fmt.Fprintf(os.Stderr, "warning: no files matched asset pattern %q — skipping\n", pattern)
		})
		if err != nil {
			return fmt.Errorf("gh release create: resolving assets: %w", err)
		}
		args = append(args, files...)
	}

	if _, _, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", args...); err != nil {
		return fmt.Errorf("gh release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// UploadAssets resolves each asset glob and runs `gh release upload` per matched file.
// When cfg.LenientAssets is true, this is a no-op — assets were already uploaded
// atomically inside CreateRelease to avoid GitHub's HTTP 422 on separate upload.
func (p *Platform) UploadAssets(tag string) error {
	if p.cfg.LenientAssets {
		return nil
	}

	repo, err := p.requireRepository()
	if err != nil {
		return err
	}

	files, err := platforms.ResolveGlobs(p.cfg.Assets)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	for _, f := range files {
		if _, _, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", "release", "upload", tag, f, "--repo", repo); err != nil {
			return fmt.Errorf("gh release upload %s: %w", f, err)
		}
	}
	return nil
}

func (p *Platform) repository() string {
	if p.cfg.Repository != "" {
		return p.cfg.Repository
	}
	return os.Getenv(repoEnvVar)
}

func (p *Platform) requireRepository() (string, error) {
	repo := p.repository()
	if repo == "" {
		return "", fmt.Errorf("repository not set: configure repository: in .heraut.yml or set $%s", repoEnvVar)
	}
	return repo, nil
}

func (p *Platform) tokenEnv() string {
	if p.cfg.TokenEnv != "" {
		return p.cfg.TokenEnv
	}
	return defaultTokenEnv
}

// tokenEnvSlice reads the configured token and returns it as ["GH_TOKEN=<value>"]
// so gh always finds it regardless of which env var name was configured.
// Returns nil when the token is unset (auth will fail — Check() should have caught this).
func (p *Platform) tokenEnvSlice() []string {
	if token := os.Getenv(p.tokenEnv()); token != "" {
		return []string{"GH_TOKEN=" + token}
	}
	return nil
}
