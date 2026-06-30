package scaffold

import (
	"fmt"
	"sort"

	"github.com/adaouat/heraut/internal/config"
)

// DroppedEnvFields returns the YAML paths of per-environment settings that could not be
// matched after the wizard ran. An env is "lost" when the original config contained an
// environment that no rebuilt entry matched by name — i.e. the user renamed or removed
// an environment during the wizard.
//
// Call this after RunWizard (post-wizard) to emit a targeted warning only when carry-
// through actually failed, rather than a pre-wizard "these might be dropped" notice.
func DroppedEnvFields(cfg *config.Config, rebuilt []EnvAnswer) []string {
	if cfg == nil || len(cfg.Environments) == 0 {
		return nil
	}
	rebuiltNames := make(map[string]bool, len(rebuilt))
	for _, e := range rebuilt {
		rebuiltNames[e.Name] = true
	}
	envNames := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)
	var dropped []string
	for _, name := range envNames {
		if rebuiltNames[name] {
			continue
		}
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

// DroppedFields returns the YAML paths of settings present in cfg that ConfigToAnswers
// and GenerateYAML do not preserve. The "Update it?" flow in `heraut init` uses this to
// warn the user before regenerating the config, since these settings would otherwise be
// silently dropped from the rewritten file.
func DroppedFields(cfg *config.Config) []string {
	return nil
}

// DroppedPlatformFields returns the YAML paths of per-platform settings that could not
// be carried through to the rebuilt platform list after the wizard ran. A field is
// "dropped" only when the original config had more entries of a given platform type than
// the rebuilt list — i.e. the user removed or changed the type of a platform during the
// wizard, leaving a tail of original entries with no positional match.
//
// Call this after RunWizard (post-wizard) to emit a targeted warning only when carry-
// through actually failed, rather than the pre-wizard "these might be dropped" notice
// that T99 originally placed.
func DroppedPlatformFields(cfg *config.Config, rebuilt []PlatformAnswer) []string {
	if cfg == nil || cfg.Release == nil {
		return nil
	}
	rebuiltCount := make(map[string]int)
	for _, p := range rebuilt {
		rebuiltCount[p.Type]++
	}
	typeOrdinal := make(map[string]int)
	var dropped []string
	for _, p := range cfg.Release.Platforms {
		typeOrdinal[p.Type]++
		if typeOrdinal[p.Type] > rebuiltCount[p.Type] {
			path := "release.platforms." + p.Name
			if p.BaseURL != "" && p.BaseURL != config.DefaultBaseURL(p.Type) {
				dropped = append(dropped, path+".base_url")
			}
			if p.Draft {
				dropped = append(dropped, path+".draft")
			}
			if p.Prerelease {
				dropped = append(dropped, path+".prerelease")
			}
		}
	}
	return dropped
}
