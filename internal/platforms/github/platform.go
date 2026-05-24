package github

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

const (
	defaultTokenEnv = "GH_TOKEN"
	repoEnvVar      = "GITHUB_REPOSITORY"
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

// Check verifies gh is on PATH, the token env var is set, and repository is resolvable.
func (p *Platform) Check() error {
	if _, _, err := p.runner.Run("gh", "--version"); err != nil {
		return fmt.Errorf("gh not found: %w", err)
	}
	tokenEnv := p.tokenEnv()
	if os.Getenv(tokenEnv) == "" {
		return fmt.Errorf("environment variable %s is not set", tokenEnv)
	}
	if p.repository() == "" {
		return fmt.Errorf("repository not set: configure repository: in .heraut.yml or set $%s", repoEnvVar)
	}
	return nil
}

// CreateRelease runs `gh release create`.
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

	if _, _, err := p.runner.Run("gh", args...); err != nil {
		return fmt.Errorf("gh release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// UploadAssets resolves each asset glob and runs `gh release upload` per matched file.
func (p *Platform) UploadAssets(tag string) error {
	repo, err := p.requireRepository()
	if err != nil {
		return err
	}

	files, err := resolveGlobs(p.cfg.Assets)
	if err != nil {
		return err
	}

	for _, f := range files {
		if _, _, err := p.runner.Run("gh", "release", "upload", tag, f, "--repo", repo); err != nil {
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

// resolveGlobs expands each glob pattern and returns all matched paths.
func resolveGlobs(patterns []string) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files matched asset pattern %q", pattern)
		}
		files = append(files, matches...)
	}
	return files, nil
}
