// Package skill embeds the agent skill (SKILL.md) that teaches an AI coding agent how to drive the
// heraut binary. `heraut skill install` and `heraut skill print` ship it, so it is versioned with
// the CLI it describes.
package skill

import _ "embed"

//go:embed SKILL.md
var content string

// Content returns the SKILL.md text.
func Content() string { return content }
