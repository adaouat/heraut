package pipeline

import "github.com/adaouat/heraut/internal/port"

// Config holds the runtime options for a release pipeline run.
type Config struct {
	// Changelog is the optional changelog generator (generates CHANGELOG.md).
	Changelog port.Generator
	// ChangelogFile is the output path for the changelog (from cfg.Changelog.Output).
	ChangelogFile string
	// Notes is the optional release-notes generator (generates release page text).
	Notes port.Generator
	// Platforms is the list of platforms to publish to.
	Platforms []port.Platform
	// CommitMessage is the git commit message template for the changelog commit.
	// Defaults to "chore(release): ${version}". ${version} is substituted.
	CommitMessage string
	// DisableChangelog skips changelog generation and commit for the active env.
	DisableChangelog bool
	// DisableNotes skips release notes generation for the active env.
	DisableNotes bool
	// AnnotatedTags creates annotated git tags (-a -m <commit_message>).
	// When false, lightweight tags are created. Defaults to false (set by app layer).
	AnnotatedTags bool
}
