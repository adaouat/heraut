package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResolver returns a fixed versioning.Result.
type fakeResolver struct {
	result versioning.Result
	err    error
}

func (r *fakeResolver) Resolve() (versioning.Result, error) { return r.result, r.err }

func resolvedResult(tag string) versioning.Result {
	return versioning.Result{Version: "1.2.3", Tag: tag, CurrentTag: "v1.2.2"}
}

// TestCheck_Passes calls Check and expects no error when all sub-checks pass.
func TestCheck_Passes(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.Config{
		Changelog: gen,
		Platforms: []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Check())
}

func TestCheck_GeneratorFails(t *testing.T) {
	mr := exectest.NewMockRunner()
	gen := &testutil.MockGenerator{CheckErr: errors.New("generator check failed")}

	cfg := &pipeline.Config{Changelog: gen}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generator check failed")
}

func TestCheck_PlatformFails(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{
		PlatformName: "github",
		CheckErr:     errors.New("gh not found"),
	}

	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh not found")
}

// TestRun_HappyPath_NoChangelog verifies the minimal release sequence (no changelog).
func TestRun_HappyPath_NoChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git tag + git push <tag>
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)

	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Platforms: []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	// git tag v1.2.3
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	// git push <tag>
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)

	// platform.CreateRelease was called with the resolved tag
	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "v1.2.3", platform.CreateReleaseCalls[0].Tag)
}

// TestRun_WithChangelog verifies changelog generation + commit + push before tagging.
func TestRun_WithChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git add CHANGELOG.md
	mr.QueueResponse("", "", nil)
	// git diff --cached --name-only (staged)
	mr.QueueResponse("CHANGELOG.md\n", "", nil)
	// git commit
	mr.QueueResponse("", "", nil)
	// git push
	mr.QueueResponse("", "", nil)
	// git tag
	mr.QueueResponse("", "", nil)
	// git push <tag>
	mr.QueueResponse("", "", nil)

	changelog := &testutil.MockGenerator{GenerateOut: ""}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, changelog.GenerateCalls, 1)
	assert.Equal(t, "v1.2.3", changelog.GenerateCalls[0])

	// git add → git diff --cached → git commit → git push → git tag → git push <tag>
	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[4].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[5].Args)
}

// TestRun_WithNotes verifies release notes are generated and passed to CreateRelease.
func TestRun_WithNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "## Features\n- add thing\n"}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Notes:     notes,
		Platforms: []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, notes.GenerateCalls, 1)
	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "## Features\n- add thing\n", platform.CreateReleaseCalls[0].Notes)
}

// clearAmbientCIEnv pins the ambient link-host env vars empty so single-platform context
// resolution is deterministic regardless of the CI the suite runs in (e.g. GitHub Actions
// sets GITHUB_SERVER_URL).
func clearAmbientCIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CI_PROJECT_URL", "")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")
}

// TestRun_SinglePlatform_PlatformContext verifies the single-platform path generates notes
// once with the platform's own resolved LinkContext when there is no ambient CI host
// (T75 — reverses T70b's earlier nil-context behaviour).
func TestRun_SinglePlatform_PlatformContext(t *testing.T) {
	clearAmbientCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	lc := port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	gh := &testutil.MockPlatform{PlatformName: "github", LinkContextVal: lc}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{gh}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, notes.GenerateCalls, 1)
	require.Len(t, notes.GenerateContexts, 1)
	require.NotNil(t, notes.GenerateContexts[0])
	assert.Equal(t, lc, *notes.GenerateContexts[0], "single platform passes the platform's own context")
	assert.Equal(t, "## notes\n", gh.CreateReleaseCalls[0].Notes)
}

// TestRun_SinglePlatform_AmbientHostPreferred verifies that when the ambient CI host
// describes the *same* platform, it is preferred (so a self-hosted instance, whose real
// host only lives in the CI env, is honoured) — T75.
func TestRun_SinglePlatform_AmbientHostPreferred(t *testing.T) {
	clearAmbientCIEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "n"}
	gl := &testutil.MockPlatform{
		PlatformName:   "gitlab",
		LinkContextVal: port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
	}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{gl}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.NotNil(t, notes.GenerateContexts[0])
	assert.Equal(t, "https://gitlab.example.com/grp/proj", notes.GenerateContexts[0].BaseURL,
		"ambient self-hosted host must win over the default base_url")
}

// TestRun_SinglePlatform_AmbientMismatchIgnored verifies a mismatched ambient CI (e.g. a
// GitHub release built in GitLab CI) does NOT stamp the wrong host — the platform's own
// context is used instead (T75).
func TestRun_SinglePlatform_AmbientMismatchIgnored(t *testing.T) {
	clearAmbientCIEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj") // gitlab CI
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "n"}
	lc := port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	gh := &testutil.MockPlatform{PlatformName: "github", LinkContextVal: lc} // github target
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{gh}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.NotNil(t, notes.GenerateContexts[0])
	assert.Equal(t, lc, *notes.GenerateContexts[0], "mismatched ambient platform must be ignored")
}

// TestRun_MultiPlatform_NotesPerPlatform verifies notes are regenerated once per platform,
// each with that platform's distinct LinkContext (ADR-0021).
func TestRun_MultiPlatform_NotesPerPlatform(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	gh := &testutil.MockPlatform{
		PlatformName:   "github",
		LinkContextVal: port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
	}
	gl := &testutil.MockPlatform{
		PlatformName:   "gitlab",
		LinkContextVal: port.LinkContext{BaseURL: "https://gitlab.com", Owner: "acme", Repo: "widget", Platform: "gitlab"},
	}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{gh, gl}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, notes.GenerateCalls, 2)
	require.Len(t, notes.GenerateContexts, 2)
	require.NotNil(t, notes.GenerateContexts[0])
	require.NotNil(t, notes.GenerateContexts[1])
	assert.Equal(t, "github", notes.GenerateContexts[0].Platform)
	assert.Equal(t, "gitlab", notes.GenerateContexts[1].Platform)
	require.Len(t, gh.CreateReleaseCalls, 1)
	require.Len(t, gl.CreateReleaseCalls, 1)
}

// TestRun_Changelog_PlatformFallback_WhenNoAmbient verifies that when there is no ambient
// CI host and exactly one platform is configured, the changelog step receives that
// platform's link context instead of nil so commit links are rendered for local/non-CI
// runs instead of degrading to bare hashes.
func TestRun_Changelog_PlatformFallback_WhenNoAmbient(t *testing.T) {
	clearAmbientCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push HEAD
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push tag

	changelog := &testutil.MockGenerator{}
	lc := port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "grp", Repo: "proj", Platform: "gitlab"}
	gl := &testutil.MockPlatform{PlatformName: "gitlab", LinkContextVal: lc}
	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{gl},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, changelog.GenerateContexts, 1)
	require.NotNil(t, changelog.GenerateContexts[0], "changelog must receive platform context when no ambient CI env")
	assert.Equal(t, lc, *changelog.GenerateContexts[0])
}

// TestRun_Changelog_AmbientPreferredForChangelog verifies that the ambient CI host wins
// over the configured platform's base_url for the changelog, so a self-hosted instance URL
// is honoured (the GitLab "base_url not set → CI_PROJECT_URL" scenario).
func TestRun_Changelog_AmbientPreferredForChangelog(t *testing.T) {
	clearAmbientCIEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://git.adaouat.dev/bchatard/ecom-poc-release")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push HEAD
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push tag

	changelog := &testutil.MockGenerator{}
	gl := &testutil.MockPlatform{
		PlatformName:   "gitlab",
		LinkContextVal: port.LinkContext{BaseURL: "https://gitlab.com", Owner: "bchatard", Repo: "ecom-poc-release", Platform: "gitlab"},
	}
	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{gl},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, changelog.GenerateContexts, 1)
	require.NotNil(t, changelog.GenerateContexts[0])
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release", changelog.GenerateContexts[0].BaseURL,
		"ambient self-hosted URL must win over platform default for changelog")
}

// TestRun_Changelog_NilContext_MultiPlatform verifies that when there is no ambient CI host
// and multiple platforms are configured, the changelog receives nil (origin is ambiguous —
// bare hashes are safer than the wrong host).
func TestRun_Changelog_NilContext_MultiPlatform(t *testing.T) {
	clearAmbientCIEnv(t)
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push HEAD
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push tag

	changelog := &testutil.MockGenerator{}
	gh := &testutil.MockPlatform{PlatformName: "github", LinkContextVal: port.LinkContext{Platform: "github"}}
	gl := &testutil.MockPlatform{PlatformName: "gitlab", LinkContextVal: port.LinkContext{Platform: "gitlab"}}
	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{gh, gl},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, changelog.GenerateContexts, 1)
	assert.Nil(t, changelog.GenerateContexts[0], "changelog must get nil when multiple platforms and no ambient CI env")
}

// TestRun_MultiPlatform_Notes_AmbientForMatchingPlatform verifies that in multi-platform
// mode each platform's release notes prefer the ambient CI context when the platform type
// matches — the self-hosted GitLab scenario: no base_url in config, CI_PROJECT_URL carries
// the correct host, and only the matching platform should use it (ADR-0022).
func TestRun_MultiPlatform_Notes_AmbientForMatchingPlatform(t *testing.T) {
	clearAmbientCIEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://git.adaouat.dev/bchatard/ecom-poc-release")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	gl := &testutil.MockPlatform{
		PlatformName:   "gitlab",
		LinkContextVal: port.LinkContext{BaseURL: "https://gitlab.com", Owner: "bchatard", Repo: "ecom-poc-release", Platform: "gitlab"},
	}
	gh := &testutil.MockPlatform{
		PlatformName:   "github",
		LinkContextVal: port.LinkContext{BaseURL: "https://github.com", Owner: "bchatard", Repo: "ecom-poc-release", Platform: "github"},
	}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{gl, gh}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, notes.GenerateContexts, 2, "notes must be generated once per platform")
	require.NotNil(t, notes.GenerateContexts[0])
	require.NotNil(t, notes.GenerateContexts[1])
	// GitLab platform matches ambient CI_PROJECT_URL → ambient self-hosted URL wins.
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release", notes.GenerateContexts[0].BaseURL,
		"ambient self-hosted URL must win for the matching GitLab platform")
	// GitHub platform does not match GitLab ambient → its own platform context is used.
	assert.Equal(t, "https://github.com", notes.GenerateContexts[1].BaseURL,
		"non-matching platform must keep its own context")
}

// TestRun_PublishStep_UsesAmbientContextForReleaseURL verifies that the pipeline passes the
// ambient-resolved link context to ReleaseURLFromContext (not the platform's default base_url),
// so the displayed release URL in the step output and summary reflects the self-hosted host
// when CI_PROJECT_URL is present (ADR-0022). This guards against the regression where the
// publish step called plat.ReleaseURL() directly, bypassing the ambient URL resolution.
func TestRun_PublishStep_UsesAmbientContextForReleaseURL(t *testing.T) {
	clearAmbientCIEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://git.adaouat.dev/bchatard/ecom-poc-release")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	gl := &testutil.MockPlatform{
		PlatformName:   "gitlab",
		LinkContextVal: port.LinkContext{BaseURL: "https://gitlab.com", Owner: "bchatard", Repo: "ecom-poc-release", Platform: "gitlab"},
	}
	cfg := &pipeline.Config{Platforms: []port.Platform{gl}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.NotEmpty(t, gl.ReleaseURLFromContextCalls, "ReleaseURLFromContext must be called (not ReleaseURL)")
	for _, call := range gl.ReleaseURLFromContextCalls {
		require.NotNil(t, call.LC, "ReleaseURLFromContext must receive the resolved link context")
		assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release", call.LC.BaseURL,
			"ambient self-hosted URL must be passed to ReleaseURLFromContext, not the platform default")
	}
}

// TestRun_WithAssets verifies UploadAssets is called after CreateRelease.
func TestRun_WithAssets(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	platform := &testutil.MockPlatform{
		PlatformName: "github",
		HasAssetsVal: true,
	}

	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, platform.UploadAssetsCalls, 1)
	assert.Equal(t, "v1.2.3", platform.UploadAssetsCalls[0])
}

// TestRun_DryRun verifies no git mutations or platform calls are made.
func TestRun_DryRun(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	out := &bytes.Buffer{}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true)
	require.NoError(t, p.Run())

	// No real runner calls in dry-run
	assert.Len(t, mr.Calls, 0)
	// No platform calls
	assert.Len(t, platform.CreateReleaseCalls, 0)
	// Dry-run output contains the resolved tag
	assert.Contains(t, out.String(), "v1.2.3")
}

// TestRun_DisableChangelog verifies changelog step is skipped when flag is set.
func TestRun_DisableChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:        changelog,
		ChangelogFile:    "CHANGELOG.md",
		Platforms:        []port.Platform{platform},
		DisableChangelog: true,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	// Generator.Generate should not have been called
	assert.Len(t, changelog.GenerateCalls, 0)
	// Only git tag + git push <tag> (no add/commit/push for changelog)
	assert.Len(t, mr.Calls, 2)
}

// TestRun_CommitMessage verifies the default commit message format.
func TestRun_CommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push
	mr.QueueResponse("", "", nil)               // git tag
	mr.QueueResponse("", "", nil)               // git push <tag>

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	commitCall := mr.Calls[2]
	assert.Equal(t, "commit", commitCall.Args[0])
	assert.Equal(t, "-m", commitCall.Args[1])
	assert.Contains(t, commitCall.Args[2], "1.2.3")
}

// TestRun_CustomCommitMessage verifies commit message template substitution.
func TestRun_CustomCommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push
	mr.QueueResponse("", "", nil)               // git tag
	mr.QueueResponse("", "", nil)               // git push <tag>

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
		CommitMessage: "release: v${version}",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	commitCall := mr.Calls[2]
	assert.Equal(t, "release: v1.2.3", commitCall.Args[2])
}

// TestRun_AnnotatedTag verifies git tag -a is used when AnnotatedTags is true.
func TestRun_AnnotatedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		AnnotatedTags: true,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
}

// TestRun_SignedTag verifies that SignTags:true produces "git tag -s <tag> -m <msg>"
// regardless of AnnotatedTags — -s implies annotated, and signing always takes precedence.
func TestRun_SignedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -s
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		SignTags:      true,
		AnnotatedTags: true, // should be ignored when signing
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-s", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	// -a must NOT appear alongside -s
	assert.NotContains(t, mr.Calls[0].Args, "-a")
}

// TestRun_SignedTag_LightweightBase verifies that SignTags:true overrides a lightweight
// base config — the tag is still signed (annotated) because the user opted into signing.
func TestRun_SignedTag_LightweightBase(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -s
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		SignTags:      true,
		AnnotatedTags: false, // lightweight base, signing wins
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-s", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
}

// TestRun_AnnotatedTagCustomMessage verifies the annotation reuses the commit_message template.
func TestRun_AnnotatedTagCustomMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		AnnotatedTags: true,
		CommitMessage: "release: v${version}",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "release: v1.2.3"}, mr.Calls[0].Args)
}

// TestRun_ResolverError propagates resolver failures.
func TestRun_ResolverError(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &pipeline.Config{}
	p := pipeline.New(mr, &fakeResolver{err: errors.New("no commits since last tag")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

// TestRun_DisableNotes verifies release notes generation is skipped when DisableNotes is set.
func TestRun_DisableNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateOut: "## Notes\n"}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Notes:        notes,
		Platforms:    []port.Platform{platform},
		DisableNotes: true,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	// Notes generator must not have been called
	assert.Len(t, notes.GenerateCalls, 0)
	// Platform must still be called, but with empty notes
	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "", platform.CreateReleaseCalls[0].Notes)
}

// TestRun_MultiplePlatforms verifies all platforms receive a CreateRelease call.
func TestRun_MultiplePlatforms(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	p1 := &testutil.MockPlatform{PlatformName: "github"}
	p2 := &testutil.MockPlatform{PlatformName: "gitlab"}

	cfg := &pipeline.Config{Platforms: []port.Platform{p1, p2}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Len(t, p1.CreateReleaseCalls, 1)
	assert.Len(t, p2.CreateReleaseCalls, 1)
}

// TestRun_GitAddError propagates git add failures.
func TestRun_GitAddError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo")) // git add fails

	changelog := &testutil.MockGenerator{}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}

// TestRun_GitCommitError propagates git commit failures.
func TestRun_GitCommitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                             // git add OK
	mr.QueueResponse("CHANGELOG.md\n", "", nil)               // git diff --cached (staged)
	mr.QueueResponse("", "", errors.New("nothing to commit")) // git commit fails

	changelog := &testutil.MockGenerator{}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git commit")
}

// TestRun_GitPushError propagates git push failures.
func TestRun_GitPushError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                         // git add OK
	mr.QueueResponse("CHANGELOG.md\n", "", nil)           // git diff --cached (staged)
	mr.QueueResponse("", "", nil)                         // git commit OK
	mr.QueueResponse("", "", errors.New("push rejected")) // git push fails

	changelog := &testutil.MockGenerator{}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git push")
}

// TestRun_DefaultChangelogFile verifies "CHANGELOG.md" is used when ChangelogFile is empty.
func TestRun_DefaultChangelogFile(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push
	mr.QueueResponse("", "", nil)               // git tag
	mr.QueueResponse("", "", nil)               // git push <tag>

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog: changelog,
		// ChangelogFile intentionally empty — should default to CHANGELOG.md
		Platforms: []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
}

// TestRun_ChangelogGenerateError propagates changelog generation failures.
func TestRun_ChangelogGenerateError(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{GenerateErr: errors.New("cliff failed")}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generating changelog")
}

// TestRun_GitTagError propagates git tag failures.
func TestRun_GitTagError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("tag already exists"))

	cfg := &pipeline.Config{}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git tag")
}

// TestRun_GitPushTagError propagates git push <tag> failures.
func TestRun_GitPushTagError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                         // git tag OK
	mr.QueueResponse("", "", errors.New("push rejected")) // git push <tag> fails

	cfg := &pipeline.Config{}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git push v1.2.3")
}

// TestRun_ReleaseNotesError propagates release notes generation failures.
func TestRun_ReleaseNotesError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	notes := &testutil.MockGenerator{GenerateErr: errors.New("cliff notes failed")}

	cfg := &pipeline.Config{
		Notes: notes,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generating release notes")
}

// TestRun_CreateReleaseError propagates platform create release failures.
func TestRun_CreateReleaseError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	platform := &testutil.MockPlatform{
		PlatformName:     "github",
		CreateReleaseErr: errors.New("API error"),
	}

	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create release")
}

// TestRun_UploadAssetsError propagates platform upload assets failures.
func TestRun_UploadAssetsError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	platform := &testutil.MockPlatform{
		PlatformName:    "github",
		HasAssetsVal:    true,
		UploadAssetsErr: errors.New("upload failed"),
	}

	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload assets")
}

// TestCheck_ChangelogGeneratorError propagates changelog generator check failures.
func TestCheck_ChangelogGeneratorError(t *testing.T) {
	changelog := &testutil.MockGenerator{CheckErr: errors.New("generator check failed")}
	cfg := &pipeline.Config{Changelog: changelog}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changelog generator")
}

// TestRun_ChangelogNothingToCommit verifies that when the regenerated changelog is
// byte-identical to the last commit (git add stages nothing), the pipeline skips the
// commit with a warning naming the file and still tags + publishes.
func TestRun_ChangelogNothingToCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git diff --cached --name-only (empty: nothing staged)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	out := &bytes.Buffer{}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false)
	require.NoError(t, p.Run())

	// No commit and no HEAD push happened.
	for _, c := range mr.Calls {
		require.NotEqual(t, "commit", c.Args[0], "git commit must be skipped when nothing is staged")
	}
	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[2].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[3].Args)

	// Still published.
	require.Len(t, platform.CreateReleaseCalls, 1)

	// Warning names the file.
	assert.Contains(t, out.String(), "CHANGELOG.md unchanged")
}

// TestCheck_NotesGeneratorError propagates release-notes generator check failures.
func TestCheck_NotesGeneratorError(t *testing.T) {
	notes := &testutil.MockGenerator{CheckErr: errors.New("cog not found")}
	cfg := &pipeline.Config{Notes: notes}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release-notes generator")
}

// TestCheck_PlatformError propagates platform check failures.
func TestCheck_PlatformError(t *testing.T) {
	platform := &testutil.MockPlatform{PlatformName: "github", CheckErr: errors.New("GH_TOKEN not set")}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform github")
}
