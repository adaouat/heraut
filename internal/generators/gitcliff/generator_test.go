package gitcliff_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"os/exec"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
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

// TestEffectiveConfig_ThinTemplates verifies both embedded TOMLs are branch-free
// interpolation (T75 / ADR-0022): they reference the heraut-injected URL-prefix vars and
// carry no remote_url macro, no platform if/else (no HERAUT_PLATFORM read), and no ambient
// CI fallback chain (relocated to Go — see TestLinkEnv / TestAmbientLinkContext). The
// per-platform path shapes (/pull/ vs /-/merge_requests/, /commit/ vs /-/commit/) and the
// old /pulls/ vs /pull/ concern (T74) are now asserted in Go by TestLinkEnv.
func TestEffectiveConfig_ThinTemplates(t *testing.T) {
	mr := exectest.NewMockRunner()
	gen := gitcliff.New(mr, &config.ContentDriver{Generator: "git-cliff"}, gitcliff.ModeReleaseNotes)

	rn, err := gen.EffectiveReleaseNotesConfig()
	require.NoError(t, err)
	cl, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)

	for name, toml := range map[string]string{"release-notes": rn, "changelog": cl} {
		// Interpolates the heraut-injected prefixes.
		assert.Contains(t, toml, "HERAUT_COMMIT_URL", "%s: commit link must use the injected prefix", name)
		assert.Contains(t, toml, "HERAUT_PR_URL", "%s: PR/MR link must use the injected prefix", name)
		assert.Contains(t, toml, "HERAUT_PR_LABEL", "%s: PR/MR label must use the injected glyph", name)
		// Branch-free: macro, platform discriminator and ambient chain are all gone.
		assert.NotContains(t, toml, "remote_url", "%s: remote_url macro must be removed", name)
		assert.NotContains(t, toml, "HERAUT_PLATFORM", "%s: no platform if/else in the template", name)
		assert.NotContains(t, toml, "CI_PROJECT_URL", "%s: ambient fallback is relocated to Go", name)
		assert.NotContains(t, toml, "/pulls/", "%s", name)
	}
	// Compare links exist only in the changelog template.
	assert.Contains(t, cl, "HERAUT_COMPARE_URL", "changelog compare link must use the injected prefix")
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

// ── remote_metadata policy (T78) ──────────────────────────────────────────────

func TestGenerate_RemoteDisabled_PassesOffline(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "disabled"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	_, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assertHasFlag(t, mr.Calls[0].Args, "--offline")
	assert.False(t, gen.Degraded())
}

func TestGenerate_RemoteRequired_NoOfflineFailsHard(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "boom", errors.New("exit status 101"))

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "required"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	_, err := gen.Generate("v1.2.3", nil)
	require.Error(t, err)
	require.Len(t, mr.Calls, 1) // no retry under 'required'
	assertNotHasFlag(t, mr.Calls[0].Args, "--offline")
}

// TestGenerate_RemoteRequired_NoForgeResolvedSucceedsSilently pins a documented divergence from
// native (T174, docs/specs/05-generators-and-platforms.md "Auto-detection and self-hosted
// hosts"): required only suppresses the --offline retry, it does not assert that a forge exists.
// With lc nil (no forge resolved), injectRemote adds no [remote.*] section, so git-cliff has
// nothing to fetch and exits cleanly — required and optional are indistinguishable here, unlike
// native's enrichForRelease, which hard-errors under required when no forge is resolvable. This
// is intentional, accepted divergence (not a bug to fix): git-cliff removal is deferred with no
// ETA, so the behavior stays as documented rather than growing new plumbing for code slated for
// eventual removal.
func TestGenerate_RemoteRequired_NoForgeResolvedSucceedsSilently(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "required"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	out, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err, "required does not assert that a forge exists — see docs/specs/05-generators-and-platforms.md")
	assert.Equal(t, "notes", out)
	require.Len(t, mr.Calls, 1) // no retry under 'required'
	assertNotHasFlag(t, mr.Calls[0].Args, "--offline")
	assert.False(t, gen.Degraded())
}

func TestGenerate_RemoteOptional_SuccessNoRetry(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("notes", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff"} // empty → optional
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	out, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)
	assert.Equal(t, "notes", out)
	require.Len(t, mr.Calls, 1)
	assertNotHasFlag(t, mr.Calls[0].Args, "--offline")
	assert.False(t, gen.Degraded())
}

func TestGenerate_RemoteOptional_DegradesOnFailure(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "Could not get github metadata", errors.New("exit status 101")) // online
	mr.QueueResponse("offline notes", "", nil)                                           // --offline retry

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "optional"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	out, err := gen.Generate("v1.2.3", nil)
	require.NoError(t, err)
	assert.Equal(t, "offline notes", out)
	require.Len(t, mr.Calls, 2)
	assertNotHasFlag(t, mr.Calls[0].Args, "--offline")
	assertHasFlag(t, mr.Calls[1].Args, "--offline")
	assert.True(t, gen.Degraded())
}

func TestGenerate_RemoteOptional_BothFail_ReturnsOriginalError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "online boom", errors.New("online error"))
	mr.QueueResponse("", "offline boom", errors.New("offline error"))

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "optional"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeReleaseNotes)

	_, err := gen.Generate("v1.2.3", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "online error") // original online error, not the offline one
	require.Len(t, mr.Calls, 2)
	assert.False(t, gen.Degraded())
}

func TestCheckCliff_RemoteOptional_DegradesOnFailure(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "Could not get github metadata", errors.New("exit status 101"))
	mr.QueueResponse("", "", nil) // --offline retry succeeds

	cfg := &config.ContentDriver{Generator: "git-cliff"} // optional
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	require.NoError(t, gen.CheckCliff())
	require.Len(t, mr.Calls, 2)
	assertNotHasFlag(t, mr.Calls[0].Args, "--offline")
	assertHasFlag(t, mr.Calls[1].Args, "--offline")
	assert.True(t, gen.Degraded())
}

func TestCheckCliff_RemoteDisabled_PassesOffline(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.ContentDriver{Generator: "git-cliff", RemoteMetadata: "disabled"}
	gen := gitcliff.New(mr, cfg, gitcliff.ModeChangelog)

	require.NoError(t, gen.CheckCliff())
	require.Len(t, mr.Calls, 1)
	assertHasFlag(t, mr.Calls[0].Args, "--offline")
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

// TestEmbeddedConfig_RealGitCliff runs the *real* git-cliff against heraut's embedded
// default config in both modes, to catch an embedded TOML the tool would reject — the gap
// that let the cocogitto config bug (T76) ship past the MockRunner contract tests. Skips
// when git-cliff is not on PATH; runs in CI where mise installs it. See T77.
func TestEmbeddedConfig_RealGitCliff(t *testing.T) {
	if _, err := exec.LookPath("git-cliff"); err != nil {
		t.Skip("git-cliff not on PATH")
	}
	testutil.RealGitRepo(t, "v0.1.0")
	runner := execadapter.New(false, false)
	for name, mode := range map[string]gitcliff.Mode{
		"changelog":     gitcliff.ModeChangelog,
		"release-notes": gitcliff.ModeReleaseNotes,
	} {
		t.Run(name, func(t *testing.T) {
			gen := gitcliff.New(runner, &config.ContentDriver{Generator: "git-cliff"}, mode)
			_, err := gen.Generate("v0.1.0", nil)
			require.NoError(t, err, "real git-cliff must accept the embedded %s config", name)
		})
	}
}

func TestEffectiveConfig_InjectsTicketLinkParsers(t *testing.T) {
	cfg := &config.ContentDriver{Generator: "git-cliff", Tickets: []config.Ticket{
		{Pattern: "[A-Z]+-[0-9]+", URL: "https://acme.atlassian.net/browse/{ticket}"}, // no group → wrapped
		{Pattern: "GH-([0-9]+)", URL: "https://github.com/acme/app/issues/{ticket}"},  // group → as-is
	}}
	gen := gitcliff.New(nil, cfg, gitcliff.ModeReleaseNotes)

	out, err := gen.EffectiveReleaseNotesConfig()
	require.NoError(t, err)
	assert.Contains(t, out, "link_parsers")
	assert.Contains(t, out, "([A-Z]+-[0-9]+)") // no-group pattern wrapped in a capture group
	assert.Contains(t, out, "browse/$1")       // {ticket} → $1 in href
	assert.Contains(t, out, "GH-([0-9]+)")     // already-grouped pattern left as-is
	assert.Contains(t, out, "issues/$1")
}

func TestEffectiveConfig_NoTickets_NoLinkParsers(t *testing.T) {
	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(nil, cfg, gitcliff.ModeChangelog)
	out, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	assert.NotContains(t, out, "atlassian")
}
