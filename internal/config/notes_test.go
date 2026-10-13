package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadNotesDoc(t *testing.T, doc string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o644))
	return Load(path)
}

func TestLoad_NotesBlock(t *testing.T) {
	cfg, err := loadNotesDoc(t, `version: "1"
versioning:
  strategy: semver
notes:
  template: .config/heraut/notes.tmpl
  rendering:
    templates:
      release:
        section: "## {{ .Version }}"
`)
	require.NoError(t, err)

	require.NotNil(t, cfg.Notes)
	assert.Equal(t, ".config/heraut/notes.tmpl", cfg.Notes.Template)
	assert.Equal(t, "## {{ .Version }}", cfg.Notes.Rendering.Templates["release.section"])
}

func TestLoad_NotesRejectsOutputAndTagPattern(t *testing.T) {
	for _, key := range []string{"output: NOTES.md", "tag_pattern: ^v"} {
		_, err := loadNotesDoc(t, "version: \"1\"\nversioning:\n  strategy: semver\nnotes:\n  "+key+"\n")
		require.Error(t, err, key)
		assert.Contains(t, err.Error(), "not found in type config.Notes", key)
	}
}

func TestValidate_NotesTemplates(t *testing.T) {
	cfg, err := loadNotesDoc(t, `version: "1"
versioning:
  strategy: semver
notes:
  template: missing.tmpl
  rendering:
    templates:
      commit:
        message: "{{ .Description"
`)
	require.NoError(t, err)

	errs := Validate(cfg)

	var paths []string
	for _, e := range errs {
		paths = append(paths, e.Path)
	}
	assert.Contains(t, paths, "notes.template")
	assert.Contains(t, paths, "notes.rendering.templates.commit.message")
}
