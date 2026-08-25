package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectItems drives RuntimeCheck synchronously and returns all items.
func collectItems(mr *exectest.MockRunner, cfg *config.Config, env string) []app.RuntimeCheckItem {
	var items []app.RuntimeCheckItem
	app.RuntimeCheck(mr, cfg, env,
		func(_ string) {}, // no-op for section headers
		func(_ string, run func() app.RuntimeCheckItem) {
			items = append(items, run())
		},
	)
	return items
}

// queueSuccess queues the 7 runner.Run responses for semverCfg (no forges/targets) with all
// tools present. Call order: git, user.name, user.email, git status, git remote get-url
// origin (forge resolution — errors, no origin configured), glab, gh.
func queueSuccess(mr *exectest.MockRunner) {
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // git config user.name
	mr.QueueResponse("a@b.com", "", nil)              // git config user.email
	mr.QueueResponse("", "", nil)                     // git status --porcelain (clean)
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0.0", "", nil)           // glab --version
	mr.QueueResponse("gh 2.0.0", "", nil)             // gh --version
}

// ---- PreflightCheck -----------------------------------------------------------

func TestPreflightCheck_Passes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email

	require.NoError(t, app.PreflightCheck(mr))
}

func TestPreflightCheck_GitMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("git: command not found"))

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git not found")
}

func TestPreflightCheck_UserNameMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("", "", nil)                   // user.name returns empty

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user.name")
}

func TestPreflightCheck_UserEmailMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name OK
	mr.QueueResponse("", "", nil)                   // user.email returns empty

	err := app.PreflightCheck(mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user.email")
}

// ---- RuntimeCheck ------------------------------------------------------------

func TestRuntimeCheck_MinimalConfig(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")
	require.Len(t, items, 6)

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
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.49.0\n", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)             // glab
	mr.QueueResponse("gh 2.0", "", nil)               // gh

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

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
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice Smith\n", "", nil)        // user.name
	mr.QueueResponse("alice@example.com\n", "", nil)  // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)             // glab
	mr.QueueResponse("gh 2.0", "", nil)               // gh

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

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
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status --porcelain (clean)
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

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
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)                       // git --version
	mr.QueueResponse("Alice", "", nil)                                    // user.name
	mr.QueueResponse("a@b.com", "", nil)                                  // user.email
	mr.QueueResponse(" M internal/foo.go\n M internal/bar.go\n", "", nil) // git status
	mr.QueueResponse("", "", errors.New("no origin"))                     // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

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
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	var names []string
	app.RuntimeCheck(mr, cfg, "",
		func(_ string) {}, // ignore section headers
		func(name string, run func() app.RuntimeCheckItem) {
			names = append(names, name)
			run()
		},
	)

	assert.Equal(t, []string{
		"git", "git user.name", "git user.email", "working tree",
		"glab", "gh",
	}, names)
}

func TestRuntimeCheck_SectionHeaders(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	var headers []string
	app.RuntimeCheck(mr, cfg, "",
		func(title string) { headers = append(headers, title) },
		func(_ string, run func() app.RuntimeCheckItem) { run() },
	)

	assert.Equal(t, []string{"Git", "Platforms"}, headers)
}

func TestRuntimeCheck_WithGitHubPlatform(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "test-token")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("gh 2.67.0", "", nil)            // gh binary — inside p.Check()
	mr.QueueResponse(`[]`, "", nil)                   // gh api auth — inside p.Check().checkAPIAuth()

	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gh", Type: "github", Repository: "org/repo"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gh"}},
	}
	items := collectItems(mr, cfg, "")

	found := false
	for _, it := range items {
		if it.Name == "gh" {
			found = true
			assert.NoError(t, it.Err)
		}
	}
	assert.True(t, found, "expected gh item")
}

func TestRuntimeCheck_WithGitHubPlatform_MissingToken(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("gh 2.67.0", "", nil)            // gh binary — p.Check() runs binary before token
	// token missing → checkAPIAuth skipped → no API runner call

	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gh", Type: "github", Repository: "org/repo"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gh"}},
	}
	items := collectItems(mr, cfg, "")

	for _, it := range items {
		if it.Name == "gh" {
			assert.Error(t, it.Err)
			assert.Contains(t, it.Err.Error(), "GH_TOKEN")
			return
		}
	}
	t.Fatal("expected gh item")
}

func TestRuntimeCheck_EnvTargetOverrideReplacesRoot(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("gh 2.67.0", "", nil)            // github-prod binary — only entry checked

	cfg := semverCfg()
	cfg.Forges = []config.Forge{
		{Name: "github-root", Type: "github", Repository: "org/root"},
		{Name: "github-prod", Type: "github", Repository: "org/prod"},
	}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "github-root"}},
	}
	cfg.Environments = map[string]config.Environment{
		"prod": {
			Bump: "auto",
			Release: &config.EnvRelease{
				Targets: []config.Target{{Forge: "github-prod"}},
			},
		},
	}
	items := collectItems(mr, cfg, "prod")

	var names []string
	for _, it := range items {
		if it.Name == "github-root" || it.Name == "github-prod" {
			names = append(names, it.Name)
		}
	}
	assert.Equal(t, []string{"github-prod"}, names, "env release.targets should replace root targets entirely")
}

func TestRuntimeCheck_EnvWithoutTargetOverrideInheritsRoot(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("gh 2.67.0", "", nil)            // github-root binary — only entry checked

	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "github-root", Type: "github", Repository: "org/root"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "github-root"}},
	}
	cfg.Environments = map[string]config.Environment{
		"staging": {Bump: "auto"}, // no release override → inherits root targets
	}
	items := collectItems(mr, cfg, "staging")

	var names []string
	for _, it := range items {
		if it.Name == "github-root" {
			names = append(names, it.Name)
		}
	}
	assert.Equal(t, []string{"github-root"}, names, "env without release.targets should inherit root")
}

func TestRuntimeCheck_AmbiguousForgeIsWarnWithoutPublishConfig(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GITHUB_TOKEN", "gh")
	t.Setenv("GITLAB_TOKEN", "gl")

	t.Run("no publish config -> advisory warning", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
		mr.QueueResponse("Alice", "", nil)                // user.name
		mr.QueueResponse("a@b.com", "", nil)              // user.email
		mr.QueueResponse("", "", nil)                     // git status
		mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)

		cfg := semverCfg()
		items := collectItems(mr, cfg, "")

		found := false
		for _, it := range items {
			if it.Name == "forge" {
				found = true
				assert.True(t, it.IsWarn, "a user who configured no publishing must not fail the check")
				require.Error(t, it.Err, "the row must still explain what failed")
			}
		}
		assert.True(t, found, "expected forge item")
	})

	// No forges: block (so resolution still runs zero-config auto-detection and hits the same
	// ambiguity), but a release.targets entry means the user explicitly asked for a publish
	// destination — resolveTargetForge never reaches the point of using a forges[] entry, so the
	// ambiguity in resolveForge itself is what must surface as a hard error here.
	t.Run("release.targets configured -> hard failure", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
		mr.QueueResponse("Alice", "", nil)                // user.name
		mr.QueueResponse("a@b.com", "", nil)              // user.email
		mr.QueueResponse("", "", nil)                     // git status
		mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)

		cfg := semverCfg()
		cfg.Release = &config.Release{Targets: []config.Target{{}}}
		items := collectItems(mr, cfg, "")

		found := false
		for _, it := range items {
			if it.Name == "forge" {
				found = true
				assert.False(t, it.IsWarn, "explicit publish config means a resolution failure is real")
				require.Error(t, it.Err)
			}
		}
		assert.True(t, found, "expected forge item")
	})

	// resolveExplicit never errors, but it is not the only error source funneled into resolveErr:
	// effectiveTargetPlatforms also calls resolveTargetForge per target, which does error with a
	// non-empty forges: block. config.Load would reject both configs below via validateTargetForges,
	// but a *config.Config built as a struct literal bypasses that — which is exactly how these
	// reach the resolveErr branch with len(cfg.Forges) > 0.
	t.Run("forges configured, target names an unknown forge -> hard failure", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
		mr.QueueResponse("Alice", "", nil)                // user.name
		mr.QueueResponse("a@b.com", "", nil)              // user.email
		mr.QueueResponse("", "", nil)                     // git status
		mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)

		cfg := semverCfg()
		cfg.Forges = []config.Forge{{Name: "A", Type: "gitlab", Project: "group/subgroup/project"}}
		cfg.Release = &config.Release{Targets: []config.Target{{Forge: "Z"}}}
		items := collectItems(mr, cfg, "")

		found := false
		for _, it := range items {
			if it.Name == "forge" {
				found = true
				assert.False(t, it.IsWarn, "a configured forges: block means a resolution failure is real")
				require.Error(t, it.Err, "the row must still explain what failed")
				assert.Contains(t, it.Err.Error(), "Z", "the message must name the unresolvable forge")
			}
		}
		assert.True(t, found, "expected forge item")
	})

	// The only shape that isolates wantsForge's first disjunct: forges: is non-empty while the
	// effective release.targets list is empty, so len(config.EffectiveTargets(cfg, env)) > 0 cannot
	// carry the result. Drop `len(cfg.Forges) > 0` from wantsForge and this row flips to a warning.
	t.Run("two forges, no targets -> hard failure via forges: alone", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
		mr.QueueResponse("Alice", "", nil)                // user.name
		mr.QueueResponse("a@b.com", "", nil)              // user.email
		mr.QueueResponse("", "", nil)                     // git status
		mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)

		cfg := semverCfg()
		cfg.Forges = []config.Forge{
			{Name: "A", Type: "gitlab", Project: "group/subgroup/project"},
			{Name: "B", Type: "github", Repository: "acme/widget"},
		}
		require.Empty(t, config.EffectiveTargets(cfg, ""), "fixture must leave release.targets empty")
		items := collectItems(mr, cfg, "")

		found := false
		for _, it := range items {
			if it.Name == "forge" {
				found = true
				assert.False(t, it.IsWarn, "forges: alone must make a resolution failure a hard error")
				require.Error(t, it.Err, "the row must still explain what failed")
			}
		}
		assert.True(t, found, "expected forge item")
	})
}

func TestRuntimeCheck_UnknownPlatform(t *testing.T) {
	// An unrecognized forge platform type is normally caught by config validation before
	// RuntimeCheck runs, but RuntimeCheck still reports a hard error for the resolved entry
	// (labeled by its configured name) rather than silently skipping it.
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)

	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "unknown-plat", Type: "unknown-plat"}}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "unknown-plat"}},
	}
	items := collectItems(mr, cfg, "")

	for _, it := range items {
		if it.Name == "unknown-plat" {
			assert.Error(t, it.Err)
			assert.Contains(t, it.Err.Error(), "unsupported platform")
			return
		}
	}
	t.Fatal("expected unknown-plat item")
}

func TestRuntimeCheck_MultipleSameTypePlatforms(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)             // gitlab-com p.Check() binary
	// token missing → checkAPIAuth skipped for gitlab-com
	mr.QueueResponse("glab 1.0", "", nil) // gitlab-internal p.Check() binary
	// token missing → checkAPIAuth skipped for gitlab-internal

	cfg := semverCfg()
	cfg.Forges = []config.Forge{
		{Name: "gitlab-com", Type: "gitlab", Project: "acme/widget"},
		{Name: "gitlab-internal", Type: "gitlab", Project: "acme/widget", BaseURL: "https://gitlab.example.com"},
	}
	cfg.Commits = &config.Commits{EnrichmentForge: "gitlab-com"}
	cfg.Release = &config.Release{
		Targets: []config.Target{{Forge: "gitlab-com"}, {Forge: "gitlab-internal"}},
	}
	items := collectItems(mr, cfg, "")

	var names []string
	for _, it := range items {
		if it.Name == "gitlab-com" || it.Name == "gitlab-internal" {
			names = append(names, it.Name)
			assert.Error(t, it.Err, "%s: missing GITLAB_TOKEN should be a hard error", it.Name)
			assert.Contains(t, it.Err.Error(), "GITLAB_TOKEN")
		}
	}
	assert.Equal(t, []string{"gitlab-com", "gitlab-internal"}, names)
}

func TestRuntimeCheck_UserNameMissing(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("", "", nil)                     // user.name empty
	mr.QueueResponse("a@b.com", "", nil)              // user.email
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

	found := false
	for _, it := range items {
		if it.Name == "git user.name" && it.Err != nil {
			found = true
		}
	}
	assert.True(t, found, "expected git user.name error item")
}

func TestRuntimeCheck_UserEmailMissing(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)   // git --version
	mr.QueueResponse("Alice", "", nil)                // user.name OK
	mr.QueueResponse("", "", nil)                     // user.email empty
	mr.QueueResponse("", "", nil)                     // git status
	mr.QueueResponse("", "", errors.New("no origin")) // git remote get-url origin (forge resolution)
	mr.QueueResponse("glab 1.0", "", nil)
	mr.QueueResponse("gh 2.0", "", nil)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

	found := false
	for _, it := range items {
		if it.Name == "git user.email" && it.Err != nil {
			found = true
		}
	}
	assert.True(t, found, "expected git user.email error item")
}

// ---- Optional tool checks -------------------------------------------------------

func TestRuntimeCheck_OptionalPlatformsWarnWhenMissing(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)         // git --version
	mr.QueueResponse("Alice", "", nil)                      // user.name
	mr.QueueResponse("a@b.com", "", nil)                    // user.email
	mr.QueueResponse("", "", nil)                           // git status
	mr.QueueResponse("", "", errors.New("no origin"))       // git remote get-url origin (forge resolution)
	mr.QueueResponse("", "", errors.New("glab: not found")) // glab (optional, missing)
	mr.QueueResponse("", "", errors.New("gh: not found"))   // gh (optional, missing)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

	warnNames := make(map[string]bool)
	for _, it := range items {
		if it.IsWarn && it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			warnNames[it.Name] = true
		}
	}
	assert.True(t, warnNames["glab"], "expected optional warn for glab")
	assert.True(t, warnNames["gh"], "expected optional warn for gh")
}

// TestRuntimeCheck_NotesOnlyNoTargets_SkipsPublishCheck covers T214: release.notes set with no
// release.targets means "notes only, no publish" (docs/specs/02-configuration.md) — even though a
// forge resolves (a configured forges: entry here), heraut check must not run a full
// publish-credential Check() against it, only the advisory binary-only fallback.
func TestRuntimeCheck_NotesOnlyNoTargets_SkipsPublishCheck(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)         // git --version
	mr.QueueResponse("Alice", "", nil)                      // user.name
	mr.QueueResponse("a@b.com", "", nil)                    // user.email
	mr.QueueResponse("", "", nil)                           // git status
	mr.QueueResponse("", "", errors.New("no origin"))       // git remote get-url origin (forge resolution)
	mr.QueueResponse("", "", errors.New("glab: not found")) // glab (binary-only fallback probe)
	mr.QueueResponse("", "", errors.New("gh: not found"))   // gh (binary-only fallback probe)

	cfg := semverCfg()
	cfg.Forges = []config.Forge{{Name: "gl", Type: "gitlab", Project: "group/repo"}}
	cfg.Release = &config.Release{Notes: &config.ContentDriver{}}
	items := collectItems(mr, cfg, "")

	for _, it := range items {
		assert.NotEqual(t, "gl", it.Name, "notes-only must not run a per-target publish check")
	}
	warnNames := make(map[string]bool)
	for _, it := range items {
		if it.IsWarn {
			warnNames[it.Name] = true
		}
	}
	assert.True(t, warnNames["glab"], "falls back to the advisory binary-only probe")
	assert.True(t, warnNames["gh"], "falls back to the advisory binary-only probe")
}

func TestRuntimeCheck_OptionalToolsSilentWhenPresent(t *testing.T) {
	testutil.ClearCIEnv(t)
	mr := exectest.NewMockRunner()
	queueSuccess(mr)

	cfg := semverCfg()
	items := collectItems(mr, cfg, "")

	for _, it := range items {
		if it.Err != nil && strings.Contains(it.Err.Error(), "not required") {
			t.Errorf("unexpected optional warn item: %s — %v", it.Name, it.Err)
		}
	}
}

// ---- RuntimeCheck with nil config -------------------------------------------

// queueSuccessNilConfig queues the 6 runner.Run responses for the nil-config path: unlike a
// non-nil config, effectiveTargetPlatforms short-circuits before ever resolving a forge (nothing
// to resolve against), so no git remote get-url origin call happens. Call order: git, user.name,
// user.email, git status, glab, gh.
func queueSuccessNilConfig(mr *exectest.MockRunner) {
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("glab 1.0.0", "", nil)         // glab --version
	mr.QueueResponse("gh 2.0.0", "", nil)           // gh --version
}

func TestRuntimeCheck_NilConfig_AllToolsPassWhenPresent(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueSuccessNilConfig(mr)

	items := collectItems(mr, nil, "")
	require.Len(t, items, 6)
	for _, it := range items {
		if it.Name == "working tree" {
			continue // always advisory regardless of config
		}
		assert.NoError(t, it.Err, "nil config: %s should pass when binary present", it.Name)
		assert.False(t, it.IsWarn, "nil config: %s must not be optional", it.Name)
	}
}

func TestRuntimeCheck_NilConfig_MissingBinaryIsHardError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil)         // git
	mr.QueueResponse("Alice", "", nil)                      // user.name
	mr.QueueResponse("a@b.com", "", nil)                    // user.email
	mr.QueueResponse("", "", nil)                           // git status
	mr.QueueResponse("", "", errors.New("glab: not found")) // glab missing
	mr.QueueResponse("gh 2.0.0", "", nil)                   // gh

	items := collectItems(mr, nil, "")

	for _, it := range items {
		if it.Name == "glab" {
			assert.Error(t, it.Err, "missing glab should be a hard error with nil config")
			assert.False(t, it.IsWarn, "missing glab must not be a warning with nil config")
			return
		}
	}
	t.Fatal("expected glab item")
}
