package cocogitto

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

//go:embed cog.toml changelog.tera release-notes.tera
var embedded embed.FS

// templatePlaceholder is the literal TOML value in the embedded cog.toml that gets
// replaced at runtime with the absolute path of the resolved template file.
const templatePlaceholder = `"<PATH_TEMPLATE.TERA>"`

// Mode selects changelog (full history) or release-notes (single release, adds --at).
type Mode int

const (
	ModeChangelog    Mode = iota
	ModeReleaseNotes Mode = iota
)

// Generator implements port.Generator for cocogitto.
type Generator struct {
	runner port.Runner
	cfg    *config.ContentDriver
	mode   Mode
}

var _ port.Generator = (*Generator)(nil)

// New constructs a Generator for cocogitto.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{runner: runner, cfg: cfg, mode: mode}
}

// Check verifies that cog is available on PATH.
func (g *Generator) Check() error {
	_, _, err := g.runner.Run("cog", "--version")
	if err != nil {
		return fmt.Errorf("cog not found: %w", err)
	}
	return nil
}

// Validate checks that user-specified config and template files exist (if provided).
func (g *Generator) Validate() error {
	if g.cfg.Config != "" {
		if _, err := os.Stat(g.cfg.Config); err != nil {
			return fmt.Errorf("cocogitto config %q: %w", g.cfg.Config, err)
		}
	}
	if g.cfg.Template != "" {
		if _, err := os.Stat(g.cfg.Template); err != nil {
			return fmt.Errorf("cocogitto template %q: %w", g.cfg.Template, err)
		}
	}
	return nil
}

// Generate invokes cog changelog with the resolved config and returns stdout
// (or empty string when output is written to a file).
//
// When lc is non-nil, the per-platform context is passed via cog's native
// --remote/--owner/--repository flags (the host scheme is stripped — cog prepends
// https://); when nil, no remote flags are added (ADR-0021 / T68).
func (g *Generator) Generate(tag string, lc *port.LinkContext) (string, error) {
	cfgPath, cfgCleanup, err := g.resolveCogConfig()
	if err != nil {
		return "", err
	}
	defer cfgCleanup()

	// --config is a global cog flag — must precede the subcommand.
	args := []string{"--config", cfgPath, "changelog"}

	// For config/template: user owns the cog.toml but wants a specific template;
	// use -t to override whatever template the user's cog.toml specifies.
	if g.cfg.Config != "" && g.cfg.Template != "" {
		args = append(args, "-t", g.cfg.Template)
	}

	if g.mode == ModeReleaseNotes {
		args = append(args, "--at", tag)
	}

	if lc != nil {
		args = append(args,
			"--remote", schemeless(lc.BaseURL),
			"--owner", lc.Owner,
			"--repository", lc.Repo,
		)
	}

	stdout, _, err := g.runner.Run("cog", args...)
	if err != nil {
		return "", fmt.Errorf("cog: %w", err)
	}

	if g.cfg.Output != "" {
		if err := os.WriteFile(g.cfg.Output, []byte(stdout), 0o644); err != nil {
			return "", fmt.Errorf("writing cog output to %q: %w", g.cfg.Output, err)
		}
		return "", nil
	}

	return stdout, nil
}

// resolveCogConfig returns the path to the cog.toml to use.
//
// When the user provides their own config it is returned as-is. Otherwise the
// embedded cog.toml is written to a temp file with the template path injected
// at the templatePlaceholder position. The template path itself comes from
// the user-specified template (if any) or the embedded changelog.tera.
func (g *Generator) resolveCogConfig() (string, func(), error) {
	if g.cfg.Config != "" {
		// User-owned config: return it directly; template handling via -t in Generate.
		return g.cfg.Config, func() {}, nil
	}

	// Resolve the template path (user's or embedded default).
	tmplPath, tmplCleanup, err := g.resolveTemplatePath()
	if err != nil {
		return "", func() {}, err
	}

	// Read the embedded cog.toml and inject the resolved template path.
	tomlData, err := embedded.ReadFile("cog.toml")
	if err != nil {
		tmplCleanup()
		return "", func() {}, fmt.Errorf("reading embedded cog.toml: %w", err)
	}
	modifiedTOML := strings.Replace(
		string(tomlData),
		templatePlaceholder,
		fmt.Sprintf("%q", tmplPath),
		1,
	)

	tmp, err := os.CreateTemp("", "heraut-cog-*.toml")
	if err != nil {
		tmplCleanup()
		return "", func() {}, fmt.Errorf("creating temp cog.toml: %w", err)
	}
	if _, err := tmp.WriteString(modifiedTOML); err != nil {
		_ = os.Remove(tmp.Name())
		tmplCleanup()
		return "", func() {}, fmt.Errorf("writing temp cog.toml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		tmplCleanup()
		return "", func() {}, fmt.Errorf("closing temp cog.toml: %w", err)
	}

	tomlPath := tmp.Name()
	return tomlPath, func() {
		_ = os.Remove(tomlPath)
		tmplCleanup()
	}, nil
}

// resolveTemplatePath returns the path to the Tera template.
// When the user specified a template it is returned as-is. Otherwise the
// embedded template matching the mode is written to a temp file.
func (g *Generator) resolveTemplatePath() (string, func(), error) {
	if g.cfg.Template != "" {
		return g.cfg.Template, func() {}, nil
	}
	asset := "changelog.tera"
	if g.mode == ModeReleaseNotes {
		asset = "release-notes.tera"
	}
	return writeTempEmbed("heraut-cog-*.tera", asset)
}

// schemeless strips the http(s):// scheme and any trailing slash from a base URL, since
// cog's --remote expects a bare host (it prepends https:// itself).
func schemeless(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimRight(u, "/")
}

func writeTempEmbed(pattern, asset string) (string, func(), error) {
	data, err := embedded.ReadFile(asset)
	if err != nil {
		return "", func() {}, fmt.Errorf("reading embedded asset %q: %w", asset, err)
	}
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("closing temp file: %w", err)
	}
	path := tmp.Name()
	return path, func() { _ = os.Remove(path) }, nil
}
