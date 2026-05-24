package gitlab

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

const (
	defaultTokenEnv  = "GITLAB_TOKEN"
	projectEnvVar    = "CI_PROJECT_PATH"
	gitlabBaseURL    = "https://gitlab.com"
)

// Platform implements port.Platform for GitLab via the glab CLI.
type Platform struct {
	runner port.Runner
	cfg    *config.Platform
}

var _ port.Platform = (*Platform)(nil)

// New constructs a GitLab Platform.
func New(runner port.Runner, cfg *config.Platform) *Platform {
	return &Platform{runner: runner, cfg: cfg}
}

func (p *Platform) Name() string { return "gitlab" }

func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("%s/%s/-/releases/%s", gitlabBaseURL, p.project(), tag)
}

// Check verifies glab is on PATH, the token env var is set, and project is resolvable.
func (p *Platform) Check() error {
	if _, _, err := p.runner.Run("glab", "--version"); err != nil {
		return fmt.Errorf("glab not found: %w", err)
	}
	tokenEnv := p.tokenEnv()
	if os.Getenv(tokenEnv) == "" {
		return fmt.Errorf("environment variable %s is not set", tokenEnv)
	}
	if p.project() == "" {
		return fmt.Errorf("project not set: configure project: in .heraut.yml or set $%s", projectEnvVar)
	}
	return nil
}

// CreateRelease runs `glab release create`.
func (p *Platform) CreateRelease(tag, notes string) error {
	proj, err := p.requireProject()
	if err != nil {
		return err
	}

	args := []string{"release", "create", tag, "--notes", notes}
	if p.cfg.Catalog {
		args = append(args, "--publish-to-catalog")
	}
	args = append(args, "-R", proj)

	if _, _, err := p.runner.Run("glab", args...); err != nil {
		return fmt.Errorf("glab release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// UploadAssets resolves each asset glob and runs `glab release upload-asset` per matched file.
func (p *Platform) UploadAssets(tag string) error {
	proj, err := p.requireProject()
	if err != nil {
		return err
	}

	files, err := resolveGlobs(p.cfg.Assets)
	if err != nil {
		return err
	}

	for _, f := range files {
		if _, _, err := p.runner.Run("glab", "release", "upload-asset", tag, f, "-R", proj); err != nil {
			return fmt.Errorf("glab release upload-asset %s: %w", f, err)
		}
	}
	return nil
}

func (p *Platform) project() string {
	if p.cfg.Project != "" {
		return p.cfg.Project
	}
	return os.Getenv(projectEnvVar)
}

func (p *Platform) requireProject() (string, error) {
	proj := p.project()
	if proj == "" {
		return "", fmt.Errorf("project not set: configure project: in .heraut.yml or set $%s", projectEnvVar)
	}
	return proj, nil
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
