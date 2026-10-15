package forge_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/forge"
)

func TestGitOriginURL(t *testing.T) {
	t.Run("returns trimmed origin", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.example.com/group/project.git\n", "", nil)
		assert.Equal(t, "https://gitlab.example.com/group/project.git", forge.GitOriginURL(mr))
		assert.Equal(t, []string{"remote", "get-url", "origin"}, mr.Calls[0].Args)
	})
	t.Run("no origin configured returns empty", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued → Run errors
		assert.Empty(t, forge.GitOriginURL(mr))
	})
}
