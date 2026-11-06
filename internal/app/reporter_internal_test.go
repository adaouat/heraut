package app

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/ui"
)

// The step reporter prints a sub-result marked with ui.Warning with the warning mark, after the
// successes, and never shows the mark itself.
func TestSpinnerReporter_PrintsWarningSubsAsWarnings(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	report := spinnerReporter(&out, 1)

	err := report("Generate changelog", func() (string, []string, error) {
		return "without enrichment", []string{"notes generated", ui.Warning("remote metadata unavailable")}, nil
	})
	require.NoError(t, err)

	assert.Contains(t, out.String(), "✓ notes generated")
	assert.Contains(t, out.String(), "! remote metadata unavailable")
	assert.NotContains(t, out.String(), "✓ remote metadata unavailable")
	assert.NotContains(t, out.String(), "\x00")
}
