package gitlab

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)

const (
	defaultTokenEnv = "GITLAB_TOKEN"
	projectEnvVar   = "CI_PROJECT_PATH"
	gitlabBaseURL   = "https://gitlab.com"
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

func (p *Platform) Name() string { return p.cfg.Name }

func (p *Platform) ReleaseURL(tag string) string {
	baseURL := p.cfg.BaseURL
	if baseURL == "" {
		baseURL = gitlabBaseURL
	}
	return fmt.Sprintf("%s/%s/-/releases/%s", baseURL, p.project(), tag)
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
	baseURL := p.cfg.BaseURL
	if baseURL == "" {
		baseURL = gitlabBaseURL
	}
	return port.LinkContext{
		BaseURL:  baseURL,
		Owner:    owner,
		Repo:     repo,
		Platform: "gitlab",
	}
}

// Check verifies glab is on PATH, the token env var is set, project is resolvable,
// and the token authenticates successfully against the GitLab API.
func (p *Platform) Check() error {
	var errs []error

	_, _, binaryErr := p.runner.Run("glab", "--version")
	if binaryErr != nil {
		errs = append(errs, fmt.Errorf("glab not found: %w", binaryErr))
	}

	tokenEnv := p.tokenEnv()
	tokenMissing := os.Getenv(tokenEnv) == ""
	if tokenMissing {
		errs = append(errs, fmt.Errorf("environment variable %s is not set", tokenEnv))
	}

	if p.project() == "" {
		errs = append(errs, fmt.Errorf("project not set: configure project: in .heraut.yml or set $%s", projectEnvVar))
	}

	if binaryErr == nil {
		if err := p.checkAPIAuth(tokenEnv, tokenMissing); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// checkAPIAuth verifies API access. For the default GitLab host in GitLab CI, glab is
// already authenticated via CI autologin so no token injection is used; the project
// releases endpoint is probed via CI_PROJECT_ID. For self-hosted instances, and outside
// CI, the configured token (plus GITLAB_HOST when self-hosted) is validated via /user.
func (p *Platform) checkAPIAuth(tokenEnv string, tokenMissing bool) error {
	if !p.selfHosted() && os.Getenv("GITLAB_CI") == "true" {
		projectID := os.Getenv("CI_PROJECT_ID")
		if projectID == "" {
			return nil
		}
		endpoint := "projects/" + projectID + "/releases?per_page=1"
		_, stderr, err := p.runner.Run("glab", "api", endpoint)
		if err != nil {
			return fmt.Errorf("gitlab: API call failed (glab api %s): %s\n  hint: ensure CI_JOB_TOKEN has read access (Settings > CI/CD > Job token permissions)", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	env := append(p.tokenEnvSlice(tokenEnv), p.hostEnv()...)
	_, stderr, err := p.runner.RunEnv(env, "glab", "api", "user")
	if err != nil {
		return fmt.Errorf("gitlab: API call failed (glab api user): %s\n  hint: verify %s is valid and has the api scope", strings.TrimSpace(stderr), tokenEnv)
	}
	return nil
}

// tokenEnvSlice injects the configured token as GITLAB_TOKEN so glab always finds it.
func (p *Platform) tokenEnvSlice(envName string) []string {
	if token := os.Getenv(envName); token != "" {
		return []string{"GITLAB_TOKEN=" + token}
	}
	return nil
}

// selfHosted reports whether this platform targets a non-default GitLab host.
func (p *Platform) selfHosted() bool {
	return p.cfg.BaseURL != "" && p.cfg.BaseURL != gitlabBaseURL
}

// hostEnv returns the env vars needed to point glab at a self-hosted instance.
// Returns nil for the default host (gitlab.com), so RunEnv(p.hostEnv(), ...) is a
// no-op for the common case.
func (p *Platform) hostEnv() []string {
	if !p.selfHosted() {
		return nil
	}
	u, err := url.Parse(p.cfg.BaseURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{"GITLAB_HOST=" + u.Host}
}

// CreateRelease runs `glab release create`.
// When cfg.LenientAssets is true (release-level assets), resolved asset files are
// included as positional args for atomic create+upload (mirrors the GitHub pattern).
func (p *Platform) CreateRelease(tag, notes string) error {
	proj, err := p.requireProject()
	if err != nil {
		return err
	}

	// GitLab automatically publishes to the CI/CD Catalog when the project is a
	// registered catalog resource — no explicit publish step needed.
	args := []string{"release", "create", tag, "--notes", notes, "-R", proj}

	if p.cfg.LenientAssets && len(p.cfg.Assets) > 0 {
		files, err := platforms.ResolveGlobsLenient(p.cfg.Assets, func(pattern string) {
			_, _ = fmt.Fprintf(os.Stderr, "warning: no files matched asset pattern %q — skipping\n", pattern)
		})
		if err != nil {
			return fmt.Errorf("glab release create: resolving assets: %w", err)
		}
		args = append(args, files...)
	}

	env := append(p.tokenEnvSlice(p.tokenEnv()), p.hostEnv()...)
	if _, _, err := p.runner.RunEnv(env, "glab", args...); err != nil {
		return fmt.Errorf("glab release create: %w", err)
	}
	return nil
}

func (p *Platform) HasAssets() bool { return len(p.cfg.Assets) > 0 }

// UploadAssets resolves asset globs and uploads all matched files in one
// `glab release upload --use-package-registry` call.
// When cfg.LenientAssets is true, this is a no-op — assets were already included
// in the glab release create call atomically.
func (p *Platform) UploadAssets(tag string) error {
	if p.cfg.LenientAssets {
		return nil
	}

	proj, err := p.requireProject()
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

	args := append([]string{"release", "upload", tag, "--use-package-registry", "-R", proj}, files...)
	env := append(p.tokenEnvSlice(p.tokenEnv()), p.hostEnv()...)
	if _, _, err := p.runner.RunEnv(env, "glab", args...); err != nil {
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
