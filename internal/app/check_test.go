package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- PreflightCheck -----------------------------------------------------------

func TestPreflightCheck_Passes(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // git config user.name
	mr.QueueResponse("a@b.com", "", nil)             // git config user.email

	require.NoError(t, app.PreflightCheck(mr))
}

func TestPreflightCheck_GitMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", errors.New("git: command not found"))

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git not found")
}

func TestPreflightCheck_UserNameMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // user.name returns empty

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user.name")
}

func TestPreflightCheck_UserEmailMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email returns empty

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user.email")
}

// ---- RuntimeCheck ------------------------------------------------------------

func TestRuntimeCheck_MinimalConfig(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	items := app.RuntimeCheck(mr, cfg)
	require.NotEmpty(t, items)

	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	assert.Contains(t, names, "git")
	assert.Contains(t, names, "git user.name")
	assert.Contains(t, names, "git user.email")
}

func TestRuntimeCheck_WithGitcliff(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // generator Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff"}
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "changelog generator (git-cliff)" {
			found = true
			assert.NoError(t, it.Err)
		}
	}
	assert.True(t, found, "expected changelog generator item")
}

func TestRuntimeCheck_WithGitHubPlatform(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("gh version 2.0.0", "", nil)   // platform Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github"}},
	}
	items := app.RuntimeCheck(mr, cfg)
	require.NotEmpty(t, items)
}

func TestRuntimeCheck_UnknownChangelogGenerator(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "unknown-gen"}
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "changelog generator" {
			found = true
			assert.Error(t, it.Err)
		}
	}
	assert.True(t, found, "expected changelog generator error item")
}

func TestRuntimeCheck_WithReleaseNotes(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // notes generator Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{Generator: "git-cliff"},
	}
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "release-notes generator (git-cliff)" {
			found = true
			assert.NoError(t, it.Err)
		}
	}
	assert.True(t, found, "expected release-notes generator item")
}

func TestRuntimeCheck_UnknownPlatform(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "unknown-plat"}},
	}
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "platform unknown-plat" {
			found = true
			assert.Error(t, it.Err)
		}
	}
	assert.True(t, found, "expected platform error item")
}

func TestRuntimeCheck_UserNameMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // user.name empty
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "git user.name" && it.Err != nil {
			found = true
		}
	}
	assert.True(t, found, "expected git user.name error item")
}

func TestRuntimeCheck_UserEmailMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)               // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email empty

	cfg := semverCfg()
	items := app.RuntimeCheck(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "git user.email" && it.Err != nil {
			found = true
		}
	}
	assert.True(t, found, "expected git user.email error item")
}

// ---- CheckCliff ---------------------------------------------------------------

func TestAppCheckCliff_Passes(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	driver := &config.ContentDriver{Generator: "git-cliff"}
	require.NoError(t, app.CheckCliff(mr, driver, "changelog"))
}

func TestAppCheckCliff_Fails(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "invalid TOML", errors.New("exit status 1"))

	driver := &config.ContentDriver{Generator: "git-cliff"}
	err := app.CheckCliff(mr, driver, "changelog")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git-cliff rejected config")
}

func TestAppCheckCliff_ReleaseNotesMode(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	driver := &config.ContentDriver{Generator: "git-cliff"}
	require.NoError(t, app.CheckCliff(mr, driver, "release-notes"))
	require.Len(t, mr.Calls, 1)
	// Verify --context --no-exec were passed
	args := mr.Calls[0].Args
	hasContext, hasNoExec := false, false
	for _, a := range args {
		if a == "--context" {
			hasContext = true
		}
		if a == "--no-exec" {
			hasNoExec = true
		}
	}
	assert.True(t, hasContext, "expected --context in args")
	assert.True(t, hasNoExec, "expected --no-exec in args")
}
