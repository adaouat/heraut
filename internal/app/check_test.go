package app_test

import (
	"errors"
	"strings"
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
	app.RuntimeCheck(mr, cfg,
		func(_ string) {}, // no-op for section headers
		func(_ string, run func() app.RuntimeCheckItem) {
			items = append(items, run())
		},
	)
	return items
}

// queueSuccess queues the 9 runner.Run responses for semverCfg (no generators/platforms)
// with all tools present. Call order: git, user.name, user.email, git status,
// glab, gh, git-cliff, cog, communique.
func queueSuccess(mr *testutil.MockRunner) {
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("glab 1.0.0", "", nil)         // glab --version
	mr.QueueResponse("gh 2.0.0", "", nil)           // gh --version
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // git-cliff --version
	mr.QueueResponse("cog 7.0.0", "", nil)          // cog --version
	mr.QueueResponse("communique 1.0.0", "", nil)   // communique --version
}

// ---- PreflightCheck -----------------------------------------------------------

func TestPreflightCheck_Passes(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email

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
	mr.QueueResponse("Alice", "", nil)              // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email returns empty

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user.email")
}

// ---- RuntimeCheck ------------------------------------------------------------

func TestRuntimeCheck_MinimalConfig(t *testing.T) {
	mr := testutil.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	items := collectItems(mr, cfg)
	require.Len(t, items, 9)

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
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("glab 1.0", "", nil)             // glab
	mr.QueueResponse("gh 2.0", "", nil)               // gh
	mr.QueueResponse("git-cliff 2.0", "", nil)        // git-cliff
	mr.QueueResponse("cog 7.0", "", nil)              // cog
	mr.QueueResponse("communique 1.0", "", nil)       // communique

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
	mr.QueueResponse("git version 2.40.0", "", nil)  // git --version
	mr.QueueResponse("Alice Smith\n", "", nil)       // user.name
	mr.QueueResponse("alice@example.com\n", "", nil) // user.email
	mr.QueueResponse("", "", nil)                    // git status
	mr.QueueResponse("glab 1.0", "", nil)            // glab
	mr.QueueResponse("gh 2.0", "", nil)              // gh
	mr.QueueResponse("git-cliff 2.0", "", nil)       // git-cliff
	mr.QueueResponse("cog 7.0", "", nil)             // cog
	mr.QueueResponse("communique 1.0", "", nil)      // communique

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
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)
	mr.QueueResponse("git-cliff 2.0", "", nil)
	mr.QueueResponse("cog 7.0", "", nil)
	mr.QueueResponse("communique 1.0", "", nil)

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
	mr.QueueResponse("git version 2.40.0", "", nil)                       // git --version
	mr.QueueResponse("Alice", "", nil)                                    // user.name
	mr.QueueResponse("a@b.com", "", nil)                                  // user.email
	mr.QueueResponse(" M internal/foo.go\n M internal/bar.go\n", "", nil) // git status
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)
	mr.QueueResponse("git-cliff 2.0", "", nil)
	mr.QueueResponse("cog 7.0", "", nil)
	mr.QueueResponse("communique 1.0", "", nil)

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
	queueSuccess(mr)

	cfg := semverCfg()
	var names []string
	app.RuntimeCheck(mr, cfg,
		func(_ string) {}, // ignore section headers
		func(name string, run func() app.RuntimeCheckItem) {
			names = append(names, name)
			run()
		},
	)

	assert.Equal(t, []string{
		"git", "git user.name", "git user.email", "working tree",
		"glab", "gh",
		"git-cliff", "cocogitto", "communique",
	}, names)
}

func TestRuntimeCheck_SectionHeaders(t *testing.T) {
	mr := testutil.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	var headers []string
	app.RuntimeCheck(mr, cfg,
		func(title string) { headers = append(headers, title) },
		func(_ string, run func() app.RuntimeCheckItem) { run() },
	)

	assert.Equal(t, []string{"Git", "Platforms", "Generators"}, headers)
}

func TestRuntimeCheck_WithGitcliff(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional)
	mr.QueueResponse("gh 2.0", "", nil)             // gh (optional)
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // git-cliff (required)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff"}
	items := collectItems(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "git-cliff" {
			found = true
			assert.NoError(t, it.Err)
			assert.Equal(t, "git-cliff 2.9.0", it.Value)
		}
	}
	assert.True(t, found, "expected git-cliff item")
}

func TestRuntimeCheck_WithGitHubPlatform(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")

	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional)
	mr.QueueResponse("gh 2.67.0", "", nil)          // gh (required)
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github"}},
	}
	items := collectItems(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "gh" {
			found = true
			assert.NoError(t, it.Err)
			assert.Equal(t, "gh 2.67.0", it.Value)
		}
	}
	assert.True(t, found, "expected gh item")
}

func TestRuntimeCheck_WithGitHubPlatform_MissingToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")

	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional)
	mr.QueueResponse("gh 2.67.0", "", nil)          // gh (required — binary found but token missing)
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github"}},
	}
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "gh" {
			assert.Error(t, it.Err)
			assert.Contains(t, it.Err.Error(), "GH_TOKEN")
			return
		}
	}
	t.Fatal("expected gh item")
}

func TestRuntimeCheck_UnknownChangelogGenerator(t *testing.T) {
	// "unknown-gen" is not a recognized generator; config validation would
	// normally catch this. RuntimeCheck checks only the 3 supported generators.
	// An unknown configured generator produces no runtime check item.
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional)
	mr.QueueResponse("gh 2.0", "", nil)             // gh (optional)
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional; "unknown-gen" ≠ "git-cliff")
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "unknown-gen"}
	items := collectItems(mr, cfg)

	for _, it := range items {
		assert.NotEqual(t, "unknown-gen", it.Name, "unknown generator should not appear")
	}
}

func TestRuntimeCheck_UnknownPlatform(t *testing.T) {
	// Unknown platforms are caught by config validation, not RuntimeCheck.
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional; "unknown-plat" ≠ "gitlab")
	mr.QueueResponse("gh 2.0", "", nil)             // gh (optional)
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "unknown-plat"}},
	}
	items := collectItems(mr, cfg)

	for _, it := range items {
		assert.NotEqual(t, "unknown-plat", it.Name, "unknown platform should not appear")
	}
}

func TestRuntimeCheck_UserNameMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // user.name empty
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)
	mr.QueueResponse("git-cliff 2.0", "", nil)
	mr.QueueResponse("cog 7.0", "", nil)
	mr.QueueResponse("communique 1.0", "", nil)

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
	mr.QueueResponse("Alice", "", nil)              // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email empty
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)
	mr.QueueResponse("git-cliff 2.0", "", nil)
	mr.QueueResponse("cog 7.0", "", nil)
	mr.QueueResponse("communique 1.0", "", nil)

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

func TestRuntimeCheck_WithReleaseNotes(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // glab (optional)
	mr.QueueResponse("gh 2.0", "", nil)             // gh (optional)
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // git-cliff (required for notes)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{Generator: "git-cliff"},
	}
	items := collectItems(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "git-cliff" {
			found = true
			assert.NoError(t, it.Err)
		}
	}
	assert.True(t, found, "expected git-cliff item")
}

// ---- Optional tool checks -------------------------------------------------------

func TestRuntimeCheck_OptionalGeneratorsWarnWhenMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)               // git --version
	mr.QueueResponse("Alice", "", nil)                            // user.name
	mr.QueueResponse("a@b.com", "", nil)                          // user.email
	mr.QueueResponse("", "", nil)                                 // git status
	mr.QueueResponse("glab 1.0.0", "", nil)                       // glab (optional, found)
	mr.QueueResponse("gh 2.0.0", "", nil)                         // gh (optional, found)
	mr.QueueResponse("", "", errors.New("git-cliff: not found"))  // git-cliff (optional, missing)
	mr.QueueResponse("", "", errors.New("cog: not found"))        // cog (optional, missing)
	mr.QueueResponse("", "", errors.New("communique: not found")) // communique (optional, missing)

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	warnNames := make(map[string]bool)
	for _, it := range items {
		if it.IsWarn && it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			warnNames[it.Name] = true
		}
	}
	assert.True(t, warnNames["git-cliff"], "expected optional warn for git-cliff")
	assert.True(t, warnNames["cocogitto"], "expected optional warn for cocogitto")
	assert.True(t, warnNames["communique"], "expected optional warn for communique")
	assert.False(t, warnNames["glab"], "glab was found, no optional warn expected")
	assert.False(t, warnNames["gh"], "gh was found, no optional warn expected")
}

func TestRuntimeCheck_OptionalPlatformsWarnWhenMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)         // git --version
	mr.QueueResponse("Alice", "", nil)                      // user.name
	mr.QueueResponse("a@b.com", "", nil)                    // user.email
	mr.QueueResponse("", "", nil)                           // git status
	mr.QueueResponse("", "", errors.New("glab: not found")) // glab (optional, missing)
	mr.QueueResponse("", "", errors.New("gh: not found"))   // gh (optional, missing)
	mr.QueueResponse("git-cliff 2.0.0", "", nil)            // git-cliff (optional, found)
	mr.QueueResponse("cog 7.0.0", "", nil)                  // cog (optional, found)
	mr.QueueResponse("communique 1.0.0", "", nil)           // communique (optional, found)

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	warnNames := make(map[string]bool)
	for _, it := range items {
		if it.IsWarn && it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			warnNames[it.Name] = true
		}
	}
	assert.True(t, warnNames["glab"], "expected optional warn for glab")
	assert.True(t, warnNames["gh"], "expected optional warn for gh")
	assert.False(t, warnNames["git-cliff"], "git-cliff was found, no optional warn expected")
}

func TestRuntimeCheck_OptionalToolsSilentWhenPresent(t *testing.T) {
	mr := testutil.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			t.Errorf("unexpected optional warn item: %s — %v", it.Name, it.Err)
		}
	}
}

func TestRuntimeCheck_ConfiguredGeneratorExcludedFromOptional(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)               // git --version
	mr.QueueResponse("Alice", "", nil)                            // user.name
	mr.QueueResponse("a@b.com", "", nil)                          // user.email
	mr.QueueResponse("", "", nil)                                 // git status
	mr.QueueResponse("glab 1.0", "", nil)                         // glab (optional)
	mr.QueueResponse("gh 2.0", "", nil)                           // gh (optional)
	mr.QueueResponse("git-cliff 2.9.0", "", nil)                  // git-cliff (required — IS configured)
	mr.QueueResponse("", "", errors.New("cog: not found"))        // cog (optional, missing)
	mr.QueueResponse("", "", errors.New("communique: not found")) // communique (optional, missing)

	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "git-cliff"}
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "git-cliff" && it.IsWarn {
			t.Error("git-cliff is configured (required), must not appear as optional warn")
		}
	}
	warnNames := make(map[string]bool)
	for _, it := range items {
		if it.IsWarn && it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			warnNames[it.Name] = true
		}
	}
	assert.True(t, warnNames["cocogitto"])
	assert.True(t, warnNames["communique"])
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
