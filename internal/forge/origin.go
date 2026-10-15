package forge

import (
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// GitOriginURL returns the origin remote URL, or "" when there is no origin (forge resolution
// then falls back to CI env or offline). Origin is the only remote heraut reads or pushes to.
func GitOriginURL(runner port.Runner) string {
	out, _, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
