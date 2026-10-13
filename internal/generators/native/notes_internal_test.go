package native

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestGenerator_Notes_BoundComesFromTheTagsOwnHistory(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.0\nv1.1.0\nv1.0.0\n", "", nil) // git tag -l --merged v1.2.0
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil)

	g := New(mr, &config.ContentDriver{}, ModeNotes)
	out, err := g.Generate("v1.2.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "- Add the thing")
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-l", "--merged", "v1.2.0", "--sort=-version:refname"}, mr.Calls[0].Args,
		"tags are listed from the requested tag, never from HEAD")
	assert.Equal(t, []string{"log", "v1.1.0..v1.2.0", "--reverse", "--format=" + logFormat}, mr.Calls[1].Args)
}
