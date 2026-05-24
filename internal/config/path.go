package config

import "os"

// ResolvePath returns the config file to use:
//  1. explicit if non-empty
//  2. .config/heraut.yml if it exists
//  3. .heraut.yml (default fallback)
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if _, err := os.Stat(".config/heraut.yml"); err == nil {
		return ".config/heraut.yml"
	}
	return ".heraut.yml"
}

// InitDest returns the path heraut init should write to:
// .config/heraut.yml if .config/ exists, else .heraut.yml.
func InitDest() string {
	if _, err := os.Stat(".config"); err == nil {
		return ".config/heraut.yml"
	}
	return ".heraut.yml"
}
