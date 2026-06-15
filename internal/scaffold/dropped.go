package scaffold

import (
	"fmt"
	"sort"

	"github.com/adaouat/heraut/internal/config"
)

// DroppedFields returns the YAML paths of settings present in cfg that ConfigToAnswers
// and GenerateYAML do not preserve. The "Update it?" flow in `heraut init` uses this to
// warn the user before regenerating the config, since these settings would otherwise be
// silently dropped from the rewritten file.
func DroppedFields(cfg *config.Config) []string {
	var dropped []string

	if cfg.Release != nil {
		for _, p := range cfg.Release.Platforms {
			dropped = append(dropped, platformDroppedFields(p, "release.platforms."+p.Name)...)
		}
	}

	envNames := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)
	for _, name := range envNames {
		env := cfg.Environments[name]
		if env.Changelog != nil {
			dropped = append(dropped, fmt.Sprintf("environments.%s.changelog", name))
		}
		if env.Release != nil {
			dropped = append(dropped, fmt.Sprintf("environments.%s.release", name))
		}
	}

	return dropped
}

// platformDroppedFields reports the fields of a single platform entry that are not
// preserved by PlatformAnswer: a base_url overriding the type's default, draft, and
// prerelease.
func platformDroppedFields(p config.Platform, path string) []string {
	var dropped []string
	if p.BaseURL != "" && p.BaseURL != config.DefaultBaseURL(p.Type) {
		dropped = append(dropped, path+".base_url")
	}
	if p.Draft {
		dropped = append(dropped, path+".draft")
	}
	if p.Prerelease {
		dropped = append(dropped, path+".prerelease")
	}
	return dropped
}
