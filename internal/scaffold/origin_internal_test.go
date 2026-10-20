package scaffold

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
)

// gitRemoteOriginURL pre-fills the wizard's platform questions. The fake git answers only the
// exact `remote get-url origin` call, so a different command or remote name fails the lookup.
func TestGitRemoteOriginURL(t *testing.T) {
	t.Run("returns the trimmed origin URL", func(t *testing.T) {
		exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "remote get-url origin") echo "git@github.com:acme/widget.git" ;;
  *) exit 1 ;;
esac
`)
		assert.Equal(t, "git@github.com:acme/widget.git", gitRemoteOriginURL())
	})

	t.Run("no origin gives an empty string", func(t *testing.T) {
		exectest.FakeBin(t, "git", `#!/bin/sh
exit 2
`)
		assert.Equal(t, "", gitRemoteOriginURL())
	})
}
