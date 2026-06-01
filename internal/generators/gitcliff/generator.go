package gitcliff

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Mode selects which embedded TOML default to use.
type Mode int

const (
	ModeChangelog    Mode = iota
	ModeReleaseNotes Mode = iota
)

// Generator implements port.Generator for git-cliff.
type Generator struct {
	runner port.Runner
	cfg    *config.ContentDriver
	mode   Mode
}

var _ port.Generator = (*Generator)(nil)

// New constructs a Generator for git-cliff.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{runner: runner, cfg: cfg, mode: mode}
}

// Check verifies that git-cliff is available on PATH.
func (g *Generator) Check() error {
	_, _, err := g.runner.Run("git-cliff", "--version")
	if err != nil {
		return fmt.Errorf("git-cliff not found: %w", err)
	}
	return nil
}

// Validate checks that the user-specified config file exists (if any).
func (g *Generator) Validate() error {
	if g.cfg.Config == "" {
		return nil
	}
	if _, err := os.Stat(g.cfg.Config); err != nil {
		return fmt.Errorf("git-cliff config %q: %w", g.cfg.Config, err)
	}
	return nil
}

// Generate invokes git-cliff with the merged TOML config and returns stdout (release-notes
// mode) or empty string (changelog mode, where output goes to a file).
func (g *Generator) Generate(tag string) (string, error) {
	cfgPath, cleanup, err := g.prepareConfig()
	if err != nil {
		return "", err
	}
	defer cleanup()

	args := []string{"--config", cfgPath, "--tag", tag}

	// ModeChangelog: no range flag — git-cliff regenerates the full history so that
	// CHANGELOG.md always contains every release, not just the current one.
	//
	// ModeReleaseNotes: --latest — the tag is already pushed by the time release notes
	// are generated (step 6 of the release pipeline), so --unreleased would return
	// nothing. --latest returns the commits in the tag we just created.
	if g.mode == ModeReleaseNotes {
		args = append(args, "--latest")
	}

	if g.cfg.TagPattern != "" {
		args = append(args, "--tag-pattern", g.cfg.TagPattern)
	}

	if g.mode == ModeChangelog && g.cfg.Output != "" {
		args = append(args, "--output", g.cfg.Output)
	}

	stdout, _, err := g.runner.Run("git-cliff", args...)
	if err != nil {
		return "", fmt.Errorf("git-cliff: %w", err)
	}
	return stdout, nil
}

// EffectiveChangelogConfig returns the merged TOML for the changelog variant.
func (g *Generator) EffectiveChangelogConfig() (string, error) {
	return g.effectiveConfig(embeddedChangelog)
}

// EffectiveReleaseNotesConfig returns the merged TOML for the release-notes variant.
func (g *Generator) EffectiveReleaseNotesConfig() (string, error) {
	return g.effectiveConfig(embeddedReleaseNotes)
}

func (g *Generator) effectiveConfig(base string) (string, error) {
	override := ""
	if g.cfg.Config != "" {
		data, err := os.ReadFile(g.cfg.Config)
		if err != nil {
			return "", fmt.Errorf("reading git-cliff config %q: %w", g.cfg.Config, err)
		}
		override = string(data)
	}
	merged, err := MergeTOML(base, override)
	if err != nil {
		return "", err
	}
	return injectBuildPostprocessor(merged, g.cfg.BuildPostprocessorPattern)
}

// injectBuildPostprocessor prepends a postprocessor entry derived from the
// {build} tag format to the [changelog] postprocessors array in the merged TOML.
// This strips the env prefix and build ID from version headings at render time.
// When pattern is empty, merged is returned unchanged.
func injectBuildPostprocessor(merged, pattern string) (string, error) {
	if pattern == "" {
		return merged, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
		return "", fmt.Errorf("parsing merged TOML for postprocessor injection: %w", err)
	}

	changelog, _ := doc["changelog"].(map[string]any)
	if changelog == nil {
		changelog = make(map[string]any)
		doc["changelog"] = changelog
	}

	entry := map[string]any{"pattern": pattern, "replace": "[$1]"}
	switch existing := changelog["postprocessors"].(type) {
	case []any:
		changelog["postprocessors"] = append([]any{entry}, existing...)
	default:
		changelog["postprocessors"] = []any{entry}
	}

	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling TOML after postprocessor injection: %w", err)
	}
	return string(out), nil
}

// CheckCliff runs git-cliff --context --no-exec against the effective merged config.
// Called by `heraut check cliff`.
func (g *Generator) CheckCliff() error {
	cfgPath, cleanup, err := g.prepareConfig()
	if err != nil {
		return err
	}
	defer cleanup()
	_, _, err = g.runner.Run("git-cliff", "--context", "--no-exec", "--config", cfgPath)
	if err != nil {
		return fmt.Errorf("git-cliff rejected config: %w", err)
	}
	return nil
}

// prepareConfig writes the effective merged TOML to a temp file and returns its path
// plus a cleanup function. The caller must call cleanup() to remove the temp file.
func (g *Generator) prepareConfig() (string, func(), error) {
	var base string
	if g.mode == ModeChangelog {
		base = embeddedChangelog
	} else {
		base = embeddedReleaseNotes
	}

	merged, err := g.effectiveConfig(base)
	if err != nil {
		return "", func() {}, err
	}

	tmp, err := os.CreateTemp("", "heraut-cliff-*.toml")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating temp cliff config: %w", err)
	}
	if _, err := tmp.WriteString(merged); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("writing temp cliff config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("closing temp cliff config: %w", err)
	}

	path := tmp.Name()
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}
