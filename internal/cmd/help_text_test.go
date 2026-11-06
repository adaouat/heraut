package cmd_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/cmd"
)

// Every command explains what it is for and shows how to call it: the text is the --help output and
// the generated commands reference.
func TestEveryCommandHasLongAndExample(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		t.Run(c.CommandPath(), func(t *testing.T) {
			assert.NotEmpty(t, strings.TrimSpace(c.Long), "Long:")
			assert.NotEmpty(t, strings.TrimSpace(c.Example), "Example:")
			for _, line := range strings.Split(c.Example, "\n") {
				if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
					assert.True(t, strings.HasPrefix(line, "heraut"), "an example line runs heraut: %q", line)
				}
			}
		})
		for _, sub := range c.Commands() {
			if !sub.Hidden && sub.Name() != "help" && sub.Name() != "completion" {
				walk(sub)
			}
		}
	}
	walk(cmd.NewRootCmd("dev"))
}
