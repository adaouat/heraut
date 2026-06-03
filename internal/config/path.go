package config

import forgeconfig "github.com/adaouat/forge/config"

// PathSource identifies how a config file path was resolved. The values mirror
// forge's Resolver.Label output for app "heraut".
type PathSource string

const (
	SourceFlag      PathSource = "--config"
	SourceEnvVar    PathSource = "HERAUT_FILE"
	SourceXDGConfig PathSource = ".config/heraut.yml"
	SourceDefault   PathSource = ".heraut.yml"
)

func resolver() forgeconfig.Resolver { return forgeconfig.Resolver{App: "heraut"} }

// ResolvePathWithSource returns the config file to use and the source that determined it
// (see forge's Resolver: --config flag → HERAUT_FILE → .config/heraut.yml → .heraut.yml).
func ResolvePathWithSource(explicit string) (string, PathSource) {
	r := resolver()
	path, src := r.Resolve(explicit)
	return path, PathSource(r.Label(src))
}

// ResolvePath returns the config file to use. See ResolvePathWithSource for the order.
func ResolvePath(explicit string) string {
	path, _ := resolver().Resolve(explicit)
	return path
}

// InitDest returns the path heraut init should write to:
// .config/heraut.yml if .config/ exists, else .heraut.yml.
func InitDest() string {
	return resolver().InitDest()
}
