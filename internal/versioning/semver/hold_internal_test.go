package semver

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBumpAfterHold_AppendsAcrossCalls(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{Bump: &config.BumpConfig{StayAtV0: true}},
	}
	r := New(nil, cfg)

	r.bumpAfterHold("0.68.0", []string{"feat!: first"})
	r.bumpAfterHold("0.68.0", []string{"feat!: second"})

	require.Len(t, r.Warnings(), 2, "a second call within one resolution must not drop the first warning")
	assert.Contains(t, r.Warnings()[0], "feat!: first")
	assert.Contains(t, r.Warnings()[1], "feat!: second")
}
