package app

import (
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func standaloneNotesCfg() *config.Config {
	return &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"},
		Commits: &config.Commits{EnrichmentPolicy: "disabled"}}
}

func standaloneNotesRepo(t *testing.T) (commit func(string), tag func(string), git func(...string)) {
	t.Helper()
	git = historyRepo(t)
	commit = func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag = func(name string) { git("tag", "-a", "-m", name, name) }
	return commit, tag, git
}

func runNotes(t *testing.T, cfg *config.Config, opts NotesOpts) (NotesResult, error) {
	t.Helper()
	return GenerateNotes(execadapter.New(false, false), cfg, opts)
}

func TestGenerateNotes_DefaultsToThePreviousRelease(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat(auth): add login")
	tag("v1.0.0")
	commit("fix: stop crash")
	tag("v1.1.0")

	res, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0"})

	require.NoError(t, err)
	assert.Contains(t, res.Text, "### ")
	assert.Contains(t, res.Text, "- Stop crash")
	assert.NotContains(t, res.Text, "Add login")
	assert.Empty(t, res.Warnings)
}

func TestGenerateNotes_AFinalSpansItsReleaseCandidates(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: first")
	tag("v1.0.0")
	commit("fix: in rc one")
	tag("v1.1.0-rc.1")
	commit("fix: in rc two")
	tag("v1.1.0")

	res, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0"})

	require.NoError(t, err)
	assert.Contains(t, res.Text, "In rc one")
	assert.Contains(t, res.Text, "In rc two")
	assert.NotContains(t, res.Text, "- First")
}

func TestGenerateNotes_AReleaseCandidateStopsAtThePreviousOne(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: first")
	tag("v1.0.0")
	commit("fix: in rc one")
	tag("v1.1.0-rc.1")
	commit("fix: in rc two")
	tag("v1.1.0-rc.2")

	res, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0-rc.2"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "In rc two")
	assert.NotContains(t, res.Text, "In rc one")

	res, err = runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0-rc.2", From: "v1.0.0"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "In rc one", "--from widens the range across the candidates")
	assert.Contains(t, res.Text, "In rc two")
}

func TestGenerateNotes_ATagFromAnotherLineIsBoundedByItsOwnHistory(t *testing.T) {
	commit, tag, git := standaloneNotesRepo(t)
	commit("chore: base")
	tag("v0.9.0")
	commit("feat: released in one zero")
	tag("v1.0.0")
	commit("feat: released in one one")
	tag("v1.1.0")
	git("checkout", "-b", "release/0.9", "v0.9.0")
	commit("fix: patch on the old line")
	tag("v1.0.5")
	git("checkout", "main")

	res, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0"})

	require.NoError(t, err)
	assert.Contains(t, res.Text, "Released in one one")
	assert.NotContains(t, res.Text, "Released in one zero", "v1.0.5 is another line, so it is not v1.1.0's predecessor")
}

func TestGenerateNotes_TextFormat(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat(auth): add login")
	tag("v1.0.0")
	commit("fix(ui): stop crash")
	tag("v1.1.0")

	res, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0", Format: "text"})

	require.NoError(t, err)
	assert.Contains(t, res.Text, "- (ui) Stop crash")
	assert.NotContains(t, res.Text, "#")
	assert.NotContains(t, res.Text, "*")
}

func TestGenerateNotes_Errors(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: a")
	tag("v1.0.0")
	commit("fix: b")
	tag("v1.1.0")

	t.Run("unknown tag", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v9.9.9"})
		require.ErrorIs(t, err, ErrNotesTagNotFound)
		assert.Contains(t, err.Error(), "v9.9.9")
	})
	t.Run("unknown from", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0", From: "v0.0.1"})
		require.ErrorIs(t, err, ErrNotesTagNotFound)
		assert.Contains(t, err.Error(), "v0.0.1")
	})
	t.Run("from newer than the tag", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.0.0", From: "v1.1.0"})
		require.ErrorIs(t, err, ErrNotesRange)
	})
	t.Run("unknown format", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0", Format: "html"})
		require.ErrorIs(t, err, ErrNotesFormat)
	})
	t.Run("per-env strategy needs --env", func(t *testing.T) {
		cfg := standaloneNotesCfg()
		cfg.Versioning.Strategy = "semver-per-env"
		_, err := runNotes(t, cfg, NotesOpts{Tag: "v1.1.0"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--env is required")
		assert.True(t, IsEnvSelection(err), "a missing --env is a configuration error")
	})
	t.Run("a tag name that looks like an option", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "--output=pwned"})
		require.ErrorIs(t, err, ErrNotesTagNotFound)
		_, err = runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0", From: "--output=pwned"})
		require.ErrorIs(t, err, ErrNotesTagNotFound)
	})
}

func TestGenerateNotes_EmptyRangeWarns(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: a")
	tag("v1.0.0")
	commit("feat: b")
	tag("v1.1.0")
	cfg := standaloneNotesCfg()
	cfg.Rendering = &config.Rendering{Excludes: []config.Exclude{{Type: "feat"}}}

	res, err := runNotes(t, cfg, NotesOpts{Tag: "v1.1.0"})

	require.NoError(t, err)
	assert.Empty(t, res.Text)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "no notable changes")
}

func TestGenerateNotes_TemplatePrecedence(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: a")
	tag("v1.0.0")
	commit("fix: b")
	tag("v1.1.0")

	global := standaloneNotesCfg()
	global.Rendering = &config.Rendering{Templates: config.TemplateOverrides{"commit.message": "* {{ .Description }}"}}
	res, err := runNotes(t, global, NotesOpts{Tag: "v1.1.0"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "* B", "a global override reaches the notes")

	both := standaloneNotesCfg()
	both.Rendering = global.Rendering
	both.Notes = &config.Notes{Rendering: &config.Rendering{Templates: config.TemplateOverrides{"commit.message": "> {{ .Description }}"}}}
	res, err = runNotes(t, both, NotesOpts{Tag: "v1.1.0"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "> B", "the notes block wins over the global one")
	assert.NotContains(t, res.Text, "* B")

	heading := standaloneNotesCfg()
	heading.Notes = &config.Notes{Rendering: &config.Rendering{Templates: config.TemplateOverrides{"release.section": "## {{ .Version }}"}}}
	res, err = runNotes(t, heading, NotesOpts{Tag: "v1.1.0"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "## 1.1.0", "a heading is one override away")
}

func TestGenerateNotes_OnlyReleaseTagsOfTheStrategy(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: a")
	tag("v1.0.0")
	commit("fix: b")
	tag("build-7")
	tag("v1.1.0")
	tag("v1.1.0-nightly")

	t.Run("a semver repo refuses a tag outside the tag prefix", func(t *testing.T) {
		_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "build-7"})
		require.ErrorIs(t, err, ErrNotesNotReleaseTag)
		assert.Contains(t, err.Error(), "build-7")
	})
	t.Run("a calver repo refuses a tag that is not a dotted version", func(t *testing.T) {
		cfg := standaloneNotesCfg()
		cfg.Versioning.Strategy = "calver"
		_, err := runNotes(t, cfg, NotesOpts{Tag: "build-7"})
		require.ErrorIs(t, err, ErrNotesNotReleaseTag)
	})
	t.Run("a per-env repo refuses another environment's tag", func(t *testing.T) {
		cfg := standaloneNotesCfg()
		cfg.Versioning.Strategy = "semver-per-env"
		cfg.Environments = map[string]config.Environment{
			"uat":  {Bump: "auto", TagFormat: "uat/{version}"},
			"prod": {Bump: "auto", TagFormat: "prod/{version}"},
		}
		_, err := runNotes(t, cfg, NotesOpts{Tag: "v1.1.0", Env: "uat"})
		require.ErrorIs(t, err, ErrNotesNotReleaseTag)
	})
}

func TestGenerateNotes_ATagIsNotARevisionExpression(t *testing.T) {
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: a")
	tag("v1.0.0")
	commit("fix: b")
	tag("v1.1.0")

	_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0^"})
	require.ErrorIs(t, err, ErrNotesTagNotFound)
	_, err = runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.1.0", From: "v1.1.0~1"})
	require.ErrorIs(t, err, ErrNotesTagNotFound)
}

func TestGenerateNotes_GitFailuresKeepTheirCause(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := runNotes(t, standaloneNotesCfg(), NotesOpts{Tag: "v1.0.0"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotesTagNotFound, "outside a repository the tag is not the problem")
	assert.Contains(t, err.Error(), "not a git repository")
}
