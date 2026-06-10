package gitcliff

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Remote-metadata policy values (T78). Empty resolves to remoteOptional.
const (
	remoteOptional = "optional"
	remoteRequired = "required"
	remoteDisabled = "disabled"
)

// Mode selects which embedded TOML default to use.
type Mode int

const (
	ModeChangelog    Mode = iota
	ModeReleaseNotes Mode = iota
)

// Generator implements port.Generator for git-cliff.
type Generator struct {
	runner   port.Runner
	cfg      *config.ContentDriver
	mode     Mode
	degraded bool
}

var _ port.Generator = (*Generator)(nil)

// New constructs a Generator for git-cliff.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{runner: runner, cfg: cfg, mode: mode}
}

// Degraded reports whether the most recent Generate/CheckCliff fell back to --offline
// because the remote metadata fetch failed under the "optional" policy (T78). Callers
// (pipeline / check) use it to warn that PR author / number were omitted.
func (g *Generator) Degraded() bool { return g.degraded }

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
//
// When lc is non-nil, heraut-owned env vars (HERAUT_REMOTE_URL, HERAUT_PLATFORM) are
// injected into the git-cliff process so the template resolves links against that
// platform; when nil, git-cliff runs with the ambient environment and the template falls
// through to its CI-var detection (ADR-0021 / T68).
func (g *Generator) Generate(tag string, lc *port.LinkContext) (string, error) {
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

	stdout, err := g.runCliff(args, lc)
	if err != nil {
		return "", fmt.Errorf("git-cliff: %w", err)
	}
	return stdout, nil
}

// runCliff runs git-cliff applying the remote-metadata policy (T78):
//   - disabled: always pass --offline (never reach the platform API).
//   - required: never pass --offline; a remote-fetch failure is fatal.
//   - optional (default): try online; on any failure retry with --offline. If the offline
//     retry succeeds the run is marked degraded; if it also fails the original (online)
//     error is returned so a genuine config error still surfaces.
func (g *Generator) runCliff(args []string, lc *port.LinkContext) (string, error) {
	switch g.cfg.RemoteMetadata {
	case remoteDisabled:
		return g.exec(appendOffline(args), lc)
	case remoteRequired:
		return g.exec(args, lc)
	default: // remoteOptional
		stdout, err := g.exec(args, lc)
		if err == nil {
			return stdout, nil
		}
		offlineOut, offlineErr := g.exec(appendOffline(args), lc)
		if offlineErr != nil {
			return "", err
		}
		g.degraded = true
		return offlineOut, nil
	}
}

// appendOffline returns args with --offline appended, without mutating the input.
func appendOffline(args []string) []string {
	return append(slices.Clone(args), "--offline")
}

func (g *Generator) exec(args []string, lc *port.LinkContext) (string, error) {
	if lc != nil {
		stdout, _, err := g.runner.RunEnv(linkEnv(lc), "git-cliff", args...)
		return stdout, err
	}
	stdout, _, err := g.runner.Run("git-cliff", args...)
	return stdout, err
}

// linkEnv translates a LinkContext into the heraut-owned env vars the embedded git-cliff
// template reads via get_env: HERAUT_REMOTE_URL (the repository root URL) and
// HERAUT_PLATFORM (github|gitlab, for the PR vs MR link shape). T71 updates the template
// to prefer these over the ambient CI-var chain.
func linkEnv(lc *port.LinkContext) []string {
	remote := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner != "" {
		remote += "/" + lc.Owner
	}
	if lc.Repo != "" {
		remote += "/" + lc.Repo
	}

	// GitLab routes everything below the project under /-/ (e.g. /-/merge_requests/);
	// GitHub does not and uses /pull/. The label glyph differs too (! vs #). This is the
	// only per-platform path knowledge — kept here in Go (table-tested) so the templates
	// stay branch-free (T75 / ADR-0022).
	infix, prPath, label := "", "pull", "#"
	if lc.Platform == "gitlab" {
		infix, prPath, label = "/-", "merge_requests", "!"
	}

	return []string{
		"HERAUT_REMOTE_URL=" + remote,
		"HERAUT_PLATFORM=" + lc.Platform,
		"HERAUT_COMMIT_URL=" + remote + infix + "/commit/",
		"HERAUT_PR_URL=" + remote + infix + "/" + prPath + "/",
		"HERAUT_PR_LABEL=" + label,
		"HERAUT_COMPARE_URL=" + remote + infix + "/compare/",
	}
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
	return injectHeadingPostprocessor(merged, g.cfg.HeadingVersionPattern)
}

// injectHeadingPostprocessor prepends a postprocessor entry (derived from the effective
// tag_format) to the [changelog] postprocessors array in the merged TOML. This strips the
// env prefix/suffix and build ID from version headings at render time. When pattern is
// empty, merged is returned unchanged.
func injectHeadingPostprocessor(merged, pattern string) (string, error) {
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
	args := []string{"--context", "--no-exec", "--config", cfgPath}
	if _, err := g.runCliff(args, nil); err != nil {
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
