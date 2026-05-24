package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
)

// EffectiveCliffConfig returns the effective merged git-cliff TOML for the given
// content driver and mode ("changelog" or "release-notes").
//
// If driver is nil or has no generator set, the embedded default TOML is returned.
// Returns an error if driver.Generator is set to something other than "git-cliff".
func EffectiveCliffConfig(driver *config.ContentDriver, mode string) (string, error) {
	if driver != nil && driver.Generator != "" && !strings.EqualFold(driver.Generator, "git-cliff") {
		return "", fmt.Errorf("generator %q is not git-cliff; heraut cliff only applies to git-cliff", driver.Generator)
	}
	if driver == nil {
		driver = &config.ContentDriver{}
	}

	m := gitcliff.ModeChangelog
	if mode == "release-notes" {
		m = gitcliff.ModeReleaseNotes
	}

	// runner is nil — Effective*Config methods only read the user config file and
	// merge with the embedded TOML; they never invoke git-cliff.
	gen := gitcliff.New(nil, driver, m)
	if m == gitcliff.ModeReleaseNotes {
		return gen.EffectiveReleaseNotesConfig()
	}
	return gen.EffectiveChangelogConfig()
}
