package config

import "os"

// ResolvePath returns the config file to use:
//  1. explicit (--config flag) if non-empty
//  2. HERAUT_FILE env var if set
//  3. .config/heraut.yml if it exists
//  4. .heraut.yml (default fallback)
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("HERAUT_FILE"); env != "" {
		return env
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
