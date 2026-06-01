package config

import (
	"os"
	"strings"
)

// PathSource identifies how a config file path was resolved.
type PathSource string

const (
	SourceFlag      PathSource = "--config"
	SourceEnvVar    PathSource = "HERAUT_FILE"
	SourceXDGConfig PathSource = ".config/heraut.yml"
	SourceDefault   PathSource = ".heraut.yml"
)

// ResolvePathWithSource returns the config file to use and the source that determined it:
//  1. explicit (--config flag) if non-empty → SourceFlag
//  2. HERAUT_FILE env var if set → SourceEnvVar
//  3. .config/heraut.yml if it exists → SourceXDGConfig
//  4. .heraut.yml (default fallback) → SourceDefault
func ResolvePathWithSource(explicit string) (string, PathSource) {
	if explicit != "" {
		return explicit, SourceFlag
	}
	if env := strings.TrimSpace(os.Getenv("HERAUT_FILE")); env != "" {
		return env, SourceEnvVar
	}
	if _, err := os.Stat(".config/heraut.yml"); err == nil {
		return ".config/heraut.yml", SourceXDGConfig
	}
	return ".heraut.yml", SourceDefault
}

// ResolvePath returns the config file to use. See ResolvePathWithSource for the full
// resolution order.
func ResolvePath(explicit string) string {
	path, _ := ResolvePathWithSource(explicit)
	return path
}

// InitDest returns the path heraut init should write to:
// .config/heraut.yml if .config/ exists, else .heraut.yml.
func InitDest() string {
	if _, err := os.Stat(".config"); err == nil {
		return ".config/heraut.yml"
	}
	return ".heraut.yml"
}
