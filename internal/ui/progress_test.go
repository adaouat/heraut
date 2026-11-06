package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/ui"
)

// A step's sub-result lines are successes unless marked with ui.Warning; SplitSubs separates them,
// in order, without the mark.
func TestSplitSubs(t *testing.T) {
	ok, warnings := ui.SplitSubs([]string{"notes generated", ui.Warning("enrichment unavailable"), "assets uploaded", ui.Warning("authors omitted")})
	assert.Equal(t, []string{"notes generated", "assets uploaded"}, ok)
	assert.Equal(t, []string{"enrichment unavailable", "authors omitted"}, warnings)

	ok, warnings = ui.SplitSubs(nil)
	assert.Empty(t, ok)
	assert.Empty(t, warnings)
}
