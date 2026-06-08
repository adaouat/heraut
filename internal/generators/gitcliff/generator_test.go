package gitcliff_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_BinaryMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "git-cliff: command not found", errors.New("exit status 127"))

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	err := gen.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git-cliff")
}

func TestCheck_BinaryPresent(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git-cliff 2.9.0", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	require.NoError(t, gen.Check())
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git-cliff", mr.Calls[0].Name)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
}

func TestValidate_NoConfig_NoError(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	require.NoError(t, gen.Validate())
	assert.Len(t, mr.Calls, 0)
}

func TestValidate_ConfigFileExists(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cliff.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[changelog]\n"), 0o600))

	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff", Config: cfgPath}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	require.NoError(t, gen.Validate())
}

func TestValidate_ConfigFileMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff", Config: "/nonexistent/cliff.toml"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	err := gen.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cliff.toml")
}

// TestGenerate_ReleaseNotes verifies the exact args passed to git-cliff in release-notes mode.
func TestGenerate_ReleaseNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("## Features\n- add thing\n", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	notes, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)
	assert.Equal(t, "## Features\n- add thing\n", notes)

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "git-cliff", call.Name)

	// --config must be present (path is a temp file — check presence, not value)
	assertHasFlag(t, call.Args, "--config")
	assertArgValue(t, call.Args, "--tag", "v1.2.3")
	// release-notes mode: --latest (tag is already pushed when notes are generated)
	assertHasFlag(t, call.Args, "--latest")
	assertNotHasFlag(t, call.Args, "--unreleased")
	// release-notes mode: no --output flag
	assertNotHasFlag(t, call.Args, "--output")
}

// TestGenerate_Changelog verifies args in changelog mode (with output file).
func TestGenerate_Changelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	_, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertHasFlag(t, call.Args, "--config")
	assertArgValue(t, call.Args, "--tag", "v1.2.3")
	assertArgValue(t, call.Args, "--output", "CHANGELOG.md")
	// changelog mode: no range flag — git-cliff generates the full history
	assertNotHasFlag(t, call.Args, "--unreleased")
	assertNotHasFlag(t, call.Args, "--latest")
	assertNotHasFlag(t, call.Args, "--tag-pattern")
}

// TestGenerate_TagPattern verifies --tag-pattern is passed when configured.
func TestGenerate_TagPattern(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff", TagPattern: "dev/*"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	_, err := gen.Generate("dev/1.2.3", nil)
	require.NoError(t, err)

	assertArgValue(t, mr.Calls[0].Args, "--tag-pattern", "dev/*")
}

// TestGenerate_LinkContext_InjectsEnv verifies a non-nil LinkContext is translated into
// heraut-owned env vars on the git-cliff invocation (ADR-0021 / T68).
func TestGenerate_LinkContext_InjectsEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	lc := &port.LinkContext{
		BaseURL:  "https://gitlab.example.com",
		Owner:    "acme",
		Repo:     "widget",
		Platform: "gitlab",
	}
	_, err := gen.Generate("v1.2.3", lc)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "git-cliff", call.Name)
	assert.Contains(t, call.Env, "HERAUT_REMOTE_URL=https://gitlab.example.com/acme/widget")
	assert.Contains(t, call.Env, "HERAUT_PLATFORM=gitlab")
}

// TestGenerate_NilLinkContext_NoEnv verifies the single-platform path injects nothing,
// so git-cliff falls through to ambient CI detection exactly as before.
func TestGenerate_NilLinkContext_NoEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	_, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Nil(t, mr.Calls[0].Env, "nil LinkContext must not inject env vars")
}

func TestEffectiveChangelogConfig(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	toml, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	assert.Contains(t, toml, "[changelog]")
	assert.Contains(t, toml, "[git]")
}

func TestEffectiveReleaseNotesConfig(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	toml, err := gen.EffectiveReleaseNotesConfig()
	require.NoError(t, err)
	assert.Contains(t, toml, "[changelog]")
}

// TestEffectiveConfig_PrefersHerautContext verifies both embedded TOMLs prefer the
// heraut-injected vars for link resolution while preserving the ambient CI fallback
// (T71a / ADR-0021). Byte assertion — actual Tera rendering is verified manually with the
// real git-cliff (the suite has no real-binary tests).
func TestEffectiveConfig_PrefersHerautContext(t *testing.T) {
	mr := exectest.NewMockRunner()
	gen := gitcliff.New(mr, &config.ContentDriver{Generator: "git-cliff"}, gitcliff.ModeReleaseNotes)

	rn, err := gen.EffectiveReleaseNotesConfig()
	require.NoError(t, err)
	cl, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)

	for name, toml := range map[string]string{"release-notes": rn, "changelog": cl} {
		assert.Contains(t, toml, "HERAUT_REMOTE_URL", "%s: remote_url must prefer the heraut-injected URL", name)
		assert.Contains(t, toml, "HERAUT_PLATFORM", "%s: pr_link must discriminate via HERAUT_PLATFORM", name)
		assert.Contains(t, toml, "CI_PROJECT_URL", "%s: ambient CI fallback must be preserved", name)
	}
}

func TestEffectiveConfig_WithUserOverride(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cliff.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[changelog]\nheader = \"# My App\"\n"), 0o600))

	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "git-cliff", Config: cfgPath}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	toml, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	assert.Contains(t, toml, "# My App")
	// base git section still present
	assert.Contains(t, toml, "[git]")
}

func TestEffectiveConfig_HeadingPostprocessorInjected(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{
		Generator:             "git-cliff",
		HeadingVersionPattern: `\[(?:[^/\]]+/)?([0-9]+\.[0-9]+\.[0-9]+)-[0-9]+\]`,
	}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	toml, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	assert.Contains(t, toml, `[0-9]+\.[0-9]+\.[0-9]+`)
	assert.Contains(t, toml, `[$1]`)
	assert.Contains(t, toml, "postprocessors")
}

func TestEffectiveConfig_HeadingPostprocessorPrependsToExisting(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cliff.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"[changelog]\npostprocessors = [{pattern = 'foo', replace = 'bar'}]\n",
	), 0o600))

	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{
		Generator:             "git-cliff",
		Config:                cfgPath,
		HeadingVersionPattern: `\[([0-9]+)\]`,
	}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	toml, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	// Both the derived pattern and the user's pattern must be present.
	assert.Contains(t, toml, `[0-9]+`)
	assert.Contains(t, toml, "foo")
}

// TestCheckCliff_Passes verifies exact args passed to git-cliff for config validation.
func TestCheckCliff_Passes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	require.NoError(t, gen.CheckCliff())
	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "git-cliff", call.Name)
	assertHasFlag(t, call.Args, "--context")
	assertHasFlag(t, call.Args, "--no-exec")
	assertHasFlag(t, call.Args, "--config")
}

// TestCheckCliff_Fails verifies error wrapping when git-cliff rejects the config.
func TestCheckCliff_Fails(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "invalid TOML", errors.New("exit status 1"))

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	err := gen.CheckCliff()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git-cliff rejected config")
}

// TestCheckCliff_ReleaseNotesMode verifies ModeReleaseNotes uses the release-notes base config.
func TestCheckCliff_ReleaseNotesMode(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	require.NoError(t, gen.CheckCliff())
	require.Len(t, mr.Calls, 1)
	assertHasFlag(t, mr.Calls[0].Args, "--context")
	assertHasFlag(t, mr.Calls[0].Args, "--no-exec")
}

// assertHasFlag checks that args contains a specific flag (without requiring its value).
func assertHasFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	if !slices.Contains(args, flag) {
		t.Errorf("expected flag %q in args %v", flag, args)
	}
}

func assertNotHasFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	if slices.Contains(args, flag) {
		t.Errorf("unexpected flag %q in args %v", flag, args)
	}
}

// assertArgValue checks that args contains flag followed immediately by value.
func assertArgValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("expected %q %q in args %v", flag, value, args)
}
