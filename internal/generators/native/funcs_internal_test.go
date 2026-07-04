package native

import (
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderWith(t *testing.T, tmpl string, data any) string {
	t.Helper()
	tt, err := template.New("t").Funcs(templateFuncs()).Parse(tmpl)
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, tt.Execute(&sb, data))
	return sb.String()
}

func TestTemplateFuncs(t *testing.T) {
	assert.Equal(t, "Hello", renderWith(t, `{{ upperFirst "hello" }}`, nil))
	assert.Equal(t, "a,b", renderWith(t, `{{ join "," (list "a" "b") }}`, nil))
	assert.Equal(t, "  x", renderWith(t, `{{ indent 2 "x" }}`, nil))
	assert.Equal(t, "x", renderWith(t, `{{ trim "  x  " }}`, nil))
	d := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-07-04", renderWith(t, `{{ date "2006-01-02" .D }}`, map[string]any{"D": d}))
}
