package ui_test

import (
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/ui"
)

func TestHelpLong(t *testing.T) {
	long := ui.HelpLong()

	assert.NotEmpty(t, long)
	assert.True(t, strings.Contains(long, "██"), "ASCII block art expected")
	assert.Contains(t, long, ui.CatchPhrase)
}

func TestVersionTemplate(t *testing.T) {
	tmpl := ui.VersionTemplate()
	require.NotEmpty(t, tmpl)

	assert.True(t, strings.Contains(tmpl, "██"), "ASCII block art expected")
	assert.Contains(t, tmpl, ui.CatchPhrase)

	_, err := template.New("version").Parse(tmpl)
	require.NoError(t, err, "version template must be valid text/template")
}
