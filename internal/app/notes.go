package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// ErrNotesFormat marks an unsupported --format value.
var ErrNotesFormat = errors.New("invalid --format")

// ErrNotesTagNotFound marks a tag (or --from tag) that does not exist.
var ErrNotesTagNotFound = errors.New("tag not found")

// ErrNotesRange marks a --from tag outside the history of the requested tag.
var ErrNotesRange = errors.New("invalid notes range")

// NotesOpts selects what GenerateNotes renders.
type NotesOpts struct {
	Tag          string
	From         string
	Env          string
	TemplateFile string
	Format       string // "markdown" (default) or "text"
	Force        bool
}

// NotesResult is the rendered text and any run notices to show next to it.
type NotesResult struct {
	Text     string
	Warnings []string
}

// GenerateNotes renders the release notes of opts.Tag (heraut notes): the groups and commit lines of
// the range from the tag's previous release, or opts.From, to the tag. It only reads git.
func GenerateNotes(runner port.Runner, cfg *config.Config, opts NotesOpts) (NotesResult, error) {
	format := opts.Format
	if format == "" {
		format = "markdown"
	}
	if format != "markdown" && format != "text" {
		return NotesResult{}, fmt.Errorf("%w %q: valid formats are markdown, text", ErrNotesFormat, opts.Format)
	}
	if (cfg.Versioning.Strategy == "semver-per-env" || cfg.Versioning.Strategy == "calver-per-env") && opts.Env == "" {
		return NotesResult{}, &perenv.EnvSelectionError{Strategy: cfg.Versioning.Strategy}
	}
	if err := requireTag(runner, opts.Tag); err != nil {
		return NotesResult{}, err
	}
	if opts.From != "" {
		if err := requireTag(runner, opts.From); err != nil {
			return NotesResult{}, err
		}
		if _, _, err := runner.Run("git", "merge-base", "--is-ancestor", opts.From, opts.Tag); err != nil {
			return NotesResult{}, fmt.Errorf("%w: %s is not in the history of %s", ErrNotesRange, opts.From, opts.Tag)
		}
	}

	driver := &config.ContentDriver{}
	if cfg.Notes != nil {
		driver.Template, driver.Rendering = cfg.Notes.Template, cfg.Notes.Rendering
	}
	if opts.TemplateFile != "" {
		driver.Template = opts.TemplateFile
	}
	enrichForge, forgeID, degraded, err := resolveEnrichForgeIfNeeded(runner, os.Getenv, cfg, opts.Force)
	if err != nil {
		return NotesResult{}, err
	}
	driver = withEnvDerivations(driver, cfg, opts.Env)
	var extra []native.Option
	if opts.From != "" {
		extra = append(extra, native.WithFromTag(opts.From))
	}
	gen := buildGenerator(runner, driver, native.ModeNotes, "", false, opts.Force, enrichForge, degraded, notesOrderFor(cfg, opts.Env, opts.Tag), extra...)
	text, err := gen.Generate(opts.Tag, pipeline.LinkContext(forgeID))
	if err != nil {
		return NotesResult{}, fmt.Errorf("generating notes: %w", err)
	}

	var res NotesResult
	if d, ok := gen.(interface {
		Degraded() bool
		DegradedReason() string
	}); ok && d.Degraded() {
		res.Warnings = append(res.Warnings, "release notes rendered without PR/MR metadata: "+d.DegradedReason())
	}
	if format == "text" {
		text = native.PlainText(text)
	}
	if text == "" {
		res.Warnings = append(res.Warnings, "no notable changes in "+notesRangeLabel(opts))
	}
	res.Text = text
	return res, nil
}

func requireTag(runner port.Runner, tag string) error {
	// A name starting with "-" would be read as an option by the git commands that take the tag as an argument.
	if strings.HasPrefix(tag, "-") {
		return fmt.Errorf("%w: %q", ErrNotesTagNotFound, tag)
	}
	if _, _, err := runner.Run("git", "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{commit}"); err != nil {
		return fmt.Errorf("%w: %q", ErrNotesTagNotFound, tag)
	}
	return nil
}

func notesRangeLabel(opts NotesOpts) string {
	if opts.From != "" {
		return opts.From + ".." + opts.Tag
	}
	return opts.Tag
}

// notesOrderFor is the tag order for a notes run. A final's predecessor skips pre-releases
// (ADR-0064), while a pre-release's is the previous tag of any kind, so the order depends on which
// of the two the requested tag is.
func notesOrderFor(cfg *config.Config, env, tag string) func([]string) []string {
	if cfg.Versioning.Strategy == "semver" || cfg.Versioning.Strategy == "semver-per-env" {
		if bare, ok := semverExtractor(cfg, env)(tag); ok {
			if v, err := semver.Parse(bare); err == nil && v.IsPreRelease() {
				return notesTagOrderFor(cfg, env)
			}
		}
	}
	return tagOrderFor(cfg, env)
}
