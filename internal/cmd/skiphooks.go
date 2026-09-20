package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
)

const skipHooksEnv = "HERAUT_SKIP_HOOKS"

// addSkipHookFlag declares --skip-hook on c, storing into target, and completes it with points —
// the hook points the command actually runs.
func addSkipHookFlag(c *cobra.Command, target *[]string, points []string) {
	c.Flags().StringSliceVar(target, "skip-hook", nil,
		"skip individual hook points for this run ("+strings.Join(points, "/")+"); repeatable or comma-separated, "+
			"also read from "+skipHooksEnv+" when the flag is absent; cannot be combined with --no-hooks")
	_ = c.RegisterFlagCompletionFunc("skip-hook", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return points, cobra.ShellCompDirectiveNoFileComp
	})
}

// resolveSkipHooks turns --skip-hook and HERAUT_SKIP_HOOKS into the validated list of hook points
// to skip. allowed is the set of points the invoking command runs.
//
// Errors lead with a plain word rather than the flag/variable name: fang capitalises the first
// letter of an error, which mangles "--skip-hook" into "--Skip-Hook".
//
// The flag and --no-hooks are contradictory when both are given explicitly, so that is an error.
// The env var is an ambient default rather than a request: an explicit --skip-hook replaces it
// (never merges with it, like --config over HERAUT_FILE), and an explicit --no-hooks simply wins
// over it, so a CI-wide HERAUT_SKIP_HOOKS never makes a one-off --no-hooks fail.
func resolveSkipHooks(c *cobra.Command, flagValues []string, noHooks bool, allowed []string) ([]string, error) {
	flagGiven := c.Flags().Changed("skip-hook")
	if flagGiven && noHooks {
		return nil, fmt.Errorf("cannot combine --skip-hook with --no-hooks: --no-hooks already skips every hook, so drop --skip-hook (or drop --no-hooks to skip only the points you name)")
	}
	if noHooks {
		return nil, nil
	}

	source, values := "--skip-hook", flagValues
	if !flagGiven {
		env := os.Getenv(skipHooksEnv)
		if strings.TrimSpace(env) == "" {
			return nil, nil
		}
		source, values = skipHooksEnv, strings.Split(env, ",")
	}

	points, err := app.ValidateSkipHooks(values, allowed)
	if err != nil {
		return nil, fmt.Errorf("invalid value in %s: %w", source, err)
	}
	return points, nil
}
