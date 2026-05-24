package gitcliff

import _ "embed"

//go:embed cliff.changelog.toml
var embeddedChangelog string

//go:embed cliff.release-notes.toml
var embeddedReleaseNotes string
