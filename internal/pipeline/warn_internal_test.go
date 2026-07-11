package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

func TestGitlabRegenWarning(t *testing.T) {
	gl := &port.LinkContext{Platform: "gitlab"}
	gh := &port.LinkContext{Platform: "github"}
	assert.Nil(t, gitlabRegenWarning(false, gl), "no warning without --regenerate")
	assert.Nil(t, gitlabRegenWarning(true, gh), "no warning on github (batched)")
	assert.Nil(t, gitlabRegenWarning(true, nil), "no warning without a remote")
	w := gitlabRegenWarning(true, gl)
	assert.Len(t, w, 1)
	assert.Contains(t, w[0], "GitLab")
}
