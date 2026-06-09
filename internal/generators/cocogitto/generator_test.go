package cocogitto_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/cocogitto"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_BinaryMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "cog: command not found", errors.New("exit status 127"))

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	err := gen.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cog")
}

func TestCheck_BinaryPresent(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("cog 7.0.0", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	require.NoError(t, gen.Check())
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "cog", mr.Calls[0].Name)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
}

func TestValidate_NoConfigNoTemplate(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	require.NoError(t, gen.Validate())
}

func TestValidate_ConfigFileMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "cocogitto", Config: "/nonexistent/cog.toml"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	err := gen.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cog.toml")
}

func TestValidate_TemplateFileMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "cocogitto", Template: "/nonexistent/tmpl.tera"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	err := gen.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tmpl.tera")
}

func TestValidate_BothFilesExist(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cog.toml")
	tmplPath := filepath.Join(tmp, "release.tera")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[changelog]\n"), 0o600))
	require.NoError(t, os.WriteFile(tmplPath, []byte("template\n"), 0o600))

	mr := exectest.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "cocogitto", Config: cfgPath, Template: tmplPath}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	require.NoError(t, gen.Validate())
}

// TestGenerate_NoneNone_ReleaseNotes: embedded config + embedded template → no -t flag.
func TestGenerate_NoneNone_ReleaseNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("## Features\n- add thing\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	notes, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)
	assert.Equal(t, "## Features\n- add thing\n", notes)

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "cog", call.Name)

	// --config (embedded cog.toml with template path injected) must precede subcommand.
	assertArgsBefore(t, call.Args, "--config", "changelog")
	// Template is embedded in the cog.toml — no -t flag.
	assertNotHasFlag(t, call.Args, "-t")
	// release-notes mode → --at <tag>
	assertArgValue(t, call.Args, "--at", "v1.2.3")
	assertContains(t, call.Args, "changelog")
}

// TestGenerate_NoneNone_Changelog: no --at in changelog mode, no -t.
func TestGenerate_NoneNone_Changelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("full changelog\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeChangelog)

	_, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertHasFlag(t, call.Args, "--config")
	assertNotHasFlag(t, call.Args, "-t")
	assertContains(t, call.Args, "changelog")
	assertNotHasFlag(t, call.Args, "--at")
}

// TestGenerate_NoneTemplate: user template path injected into embedded cog.toml — no -t.
func TestGenerate_NoneTemplate(t *testing.T) {
	tmp := t.TempDir()
	tmplPath := filepath.Join(tmp, "release.tera")
	require.NoError(t, os.WriteFile(tmplPath, []byte("template\n"), 0o600))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto", Template: tmplPath}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	_, err := gen.Generate("v1.0.0", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	// Template is configured in the embedded cog.toml — no -t flag.
	assertHasFlag(t, call.Args, "--config")
	assertNotHasFlag(t, call.Args, "-t")
	assertArgValue(t, call.Args, "--at", "v1.0.0")
}

// TestGenerate_ConfigNone: user config, no -t.
func TestGenerate_ConfigNone(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cog.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[changelog]\n"), 0o600))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto", Config: cfgPath}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	_, err := gen.Generate("v2.0.0", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertArgValue(t, call.Args, "--config", cfgPath)
	assertNotHasFlag(t, call.Args, "-t")
	assertArgValue(t, call.Args, "--at", "v2.0.0")
}

// TestGenerate_ConfigTemplate: user config + user template → -t used to override.
func TestGenerate_ConfigTemplate(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cog.toml")
	tmplPath := filepath.Join(tmp, "release.tera")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[changelog]\n"), 0o600))
	require.NoError(t, os.WriteFile(tmplPath, []byte("template\n"), 0o600))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto", Config: cfgPath, Template: tmplPath}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	_, err := gen.Generate("v3.0.0", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertArgValue(t, call.Args, "--config", cfgPath)
	assertArgValue(t, call.Args, "-t", tmplPath)
	assertArgValue(t, call.Args, "--at", "v3.0.0")
}

// TestGenerate_LinkContext_PassesRemoteFlags verifies a non-nil LinkContext is translated
// into cog's --remote/--owner/--repository flags, with the scheme stripped from the host
// (ADR-0021 / T68).
func TestGenerate_LinkContext_PassesRemoteFlags(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	lc := &port.LinkContext{
		BaseURL:  "https://gitlab.example.com",
		Owner:    "acme",
		Repo:     "widget",
		Platform: "gitlab",
	}
	_, err := gen.Generate("v1.2.3", lc)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertArgValue(t, call.Args, "--remote", "gitlab.example.com")
	assertArgValue(t, call.Args, "--owner", "acme")
	assertArgValue(t, call.Args, "--repository", "widget")
}

// TestGenerate_NilLinkContext_NoRemoteFlags verifies the single-platform path adds no
// remote flags — cog behaves exactly as before.
func TestGenerate_NilLinkContext_NoRemoteFlags(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	_, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)

	call := mr.Calls[0]
	assertNotHasFlag(t, call.Args, "--remote")
	assertNotHasFlag(t, call.Args, "--owner")
	assertNotHasFlag(t, call.Args, "--repository")
}

// TestGenerate_OutputWritten verifies heraut writes stdout to output file in changelog mode.
func TestGenerate_OutputWritten(t *testing.T) {
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "CHANGELOG.md")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("## Generated changelog\n", "", nil)

	cfg := &config.ContentDriver{Generator: "cocogitto", Output: outputPath}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeChangelog)

	result, err := gen.Generate("v1.0.0", nil)
	require.NoError(t, err)
	assert.Empty(t, result)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "## Generated changelog\n", string(data))
}

func TestGenerate_RunnerError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal error", errors.New("exit status 1"))

	cfg := &config.ContentDriver{Generator: "cocogitto"}
	gen := cocogitto.New(mr, cfg, cocogitto.ModeReleaseNotes)

	_, err := gen.Generate("v1.0.0", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cog")
}

func assertHasFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Errorf("expected flag %q in args %v", flag, args)
}

func assertNotHasFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			t.Errorf("unexpected flag %q in args %v", flag, args)
			return
		}
	}
}

func assertArgValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("expected %q %q in args %v", flag, value, args)
}

func assertContains(t *testing.T, args []string, elem string) {
	t.Helper()
	for _, a := range args {
		if a == elem {
			return
		}
	}
	t.Errorf("expected %q in args %v", elem, args)
}

func assertArgsBefore(t *testing.T, args []string, flag1, flag2 string) {
	t.Helper()
	idx1, idx2 := -1, -1
	for i, a := range args {
		if a == flag1 && idx1 == -1 {
			idx1 = i
		}
		if a == flag2 && idx2 == -1 {
			idx2 = i
		}
	}
	if idx1 == -1 {
		t.Errorf("flag %q not found in args %v", flag1, args)
		return
	}
	if idx2 == -1 {
		t.Errorf("flag %q not found in args %v", flag2, args)
		return
	}
	if idx1 >= idx2 {
		t.Errorf("expected %q (idx %d) before %q (idx %d) in args %v", flag1, idx1, flag2, idx2, args)
	}
}

// TestEmbeddedConfig_RealCog runs the *real* cog against heraut's embedded default config
// in both modes. This is the guard that would have caught the T76 bug (the embedded
// cog.toml's commit_parsers block, which cog 7.0.0 rejects) — the MockRunner contract tests
// never run cog, so an invalid embedded config sailed through. Skips when cog is not on
// PATH; runs in CI where mise installs it. See T77.
func TestEmbeddedConfig_RealCog(t *testing.T) {
	if _, err := exec.LookPath("cog"); err != nil {
		t.Skip("cog not on PATH")
	}
	testutil.RealGitRepo(t, "v0.1.0")
	runner := execadapter.New(false, false)
	for name, mode := range map[string]cocogitto.Mode{
		"changelog":     cocogitto.ModeChangelog,
		"release-notes": cocogitto.ModeReleaseNotes,
	} {
		t.Run(name, func(t *testing.T) {
			gen := cocogitto.New(runner, &config.ContentDriver{Generator: "cocogitto"}, mode)
			_, err := gen.Generate("v0.1.0", nil)
			require.NoError(t, err, "real cog must accept the embedded %s config", name)
		})
	}
}
