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

// collectItems drives RuntimeCheck synchronously and returns all items.
func collectItems(mr *testutil.MockRunner, cfg *config.Config) []app.RuntimeCheckItem {
	var items []app.RuntimeCheckItem
	app.RuntimeCheck(mr, cfg, func(_ string, run func() app.RuntimeCheckItem) {
		items = append(items, run())
	})
	return items
}

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)
	require.NotEmpty(t, items)

	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	assert.Contains(t, names, "git")
	assert.Contains(t, names, "working tree")
	assert.Contains(t, names, "git user.name")
	assert.Contains(t, names, "git user.email")
}

func TestRuntimeCheck_GitValue(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.49.0\n", "", nil) // git --version
	mr.QueueResponse("", "", nil)                      // git status --porcelain
	mr.QueueResponse("Alice", "", nil)                 // user.name
	mr.QueueResponse("a@b.com", "", nil)               // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "git" {
			assert.NoError(t, it.Err)
			assert.Equal(t, "git version 2.49.0", it.Value)
			return
		}
	}
	t.Fatal("expected git item")
}

func TestRuntimeCheck_UserNameValue(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("", "", nil)                      // git status --porcelain
	mr.QueueResponse("Alice Smith\n", "", nil)         // user.name
	mr.QueueResponse("alice@example.com\n", "", nil)   // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "git user.name" {
			assert.NoError(t, it.Err)
			assert.Equal(t, "Alice Smith", it.Value)
			return
		}
	}
	t.Fatal("expected git user.name item")
}

func TestRuntimeCheck_WorkingTreeClean(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "working tree" {
			assert.NoError(t, it.Err)
			assert.False(t, it.IsWarn)
			assert.Equal(t, "clean", it.Value)
			return
		}
	}
	t.Fatal("expected working tree item")
}

func TestRuntimeCheck_WorkingTreeDirty(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)                             // git --version
	mr.QueueResponse(" M internal/foo.go\n M internal/bar.go\n", "", nil) // git status --porcelain
	mr.QueueResponse("Alice", "", nil)                                           // user.name
	mr.QueueResponse("a@b.com", "", nil)                                         // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "working tree" {
			assert.Error(t, it.Err)
			assert.True(t, it.IsWarn, "dirty working tree should be a warning, not a hard error")
			assert.Contains(t, it.Err.Error(), "2")
			return
		}
	}
	t.Fatal("expected working tree item")
}

func TestRuntimeCheck_DispatchNames(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	var names []string
	app.RuntimeCheck(mr, cfg, func(name string, run func() app.RuntimeCheckItem) {
		names = append(names, name)
		run()
	})

	assert.Equal(t, []string{"git", "working tree", "git user.name", "git user.email"}, names)
}

func TestRuntimeCheck_WithGitcliff(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // generator Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff"}
	items := collectItems(mr, cfg)

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("gh version 2.0.0", "", nil)   // platform Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github"}},
	}
	items := collectItems(mr, cfg)
	require.NotEmpty(t, items)
}

func TestRuntimeCheck_UnknownChangelogGenerator(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "unknown-gen"}
	items := collectItems(mr, cfg)

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // notes generator Check()
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{Generator: "git-cliff"},
	}
	items := collectItems(mr, cfg)

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("Alice", "", nil)               // user.name
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "unknown-plat"}},
	}
	items := collectItems(mr, cfg)

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("", "", nil)                   // user.name empty
	mr.QueueResponse("a@b.com", "", nil)             // user.email

	cfg := semverCfg()
	items := collectItems(mr, cfg)

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
	mr.QueueResponse("", "", nil)                   // git status --porcelain
	mr.QueueResponse("Alice", "", nil)               // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email empty

	cfg := semverCfg()
	items := collectItems(mr, cfg)

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
