package gitlab

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
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

	// GitLab automatically publishes to the CI/CD Catalog when the project is a
	// registered catalog resource — no explicit publish step needed.
	args := []string{"release", "create", tag, "--notes", notes, "-R", proj}

	if _, _, err := p.runner.Run("glab", args...); err != nil {
		return fmt.Errorf("glab release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// UploadAssets resolves asset globs and uploads all matched files in one
// `glab release upload --use-package-registry` call.
func (p *Platform) UploadAssets(tag string) error {
	proj, err := p.requireProject()
	if err != nil {
		return err
	}

	files, err := platforms.ResolveGlobs(p.cfg.Assets)
	if err != nil {
		return err
	}

	args := append([]string{"release", "upload", tag, "--use-package-registry", "-R", proj}, files...)
	if _, _, err := p.runner.Run("glab", args...); err != nil {
		return fmt.Errorf("glab release upload: %w", err)
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
