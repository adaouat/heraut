// Package heraut holds build-time assets embedded into the heraut binary.
package heraut

import _ "embed"

// Changelog is the project changelog, embedded so `whatsnew` can fall back to it offline.
//
//go:embed CHANGELOG.md
var Changelog string
