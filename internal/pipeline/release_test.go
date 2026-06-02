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
	gen := &testutil.MockGenerator{CheckErr: errors.New("git-cliff not found")}

	cfg := &pipeline.Config{Changelog: gen}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git-cliff not found")
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
	// git tag + git push --tags
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
	// git push --tags
	assert.Equal(t, []string{"push", "origin", "--tags"}, mr.Calls[1].Args)

	// platform.CreateRelease was called with the resolved tag
	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "v1.2.3", platform.CreateReleaseCalls[0].Tag)
}

// TestRun_WithChangelog verifies changelog generation + commit + push before tagging.
func TestRun_WithChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git add CHANGELOG.md
	mr.QueueResponse("", "", nil)
	// git commit
	mr.QueueResponse("", "", nil)
	// git push
	mr.QueueResponse("", "", nil)
	// git tag
	mr.QueueResponse("", "", nil)
	// git push --tags
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

	// git add → git commit → git push → git tag → git push --tags
	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, "commit", mr.Calls[1].Args[0])
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[2].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "origin", "--tags"}, mr.Calls[4].Args)
}

// TestRun_WithNotes verifies release notes are generated and passed to CreateRelease.
func TestRun_WithNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

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

// TestRun_WithAssets verifies UploadAssets is called after CreateRelease.
func TestRun_WithAssets(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	// Only git tag + git push --tags (no add/commit/push for changelog)
	assert.Len(t, mr.Calls, 2)
}

// TestRun_CommitMessage verifies the default commit message format.
func TestRun_CommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}

	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	commitCall := mr.Calls[1]
	assert.Equal(t, "commit", commitCall.Args[0])
	assert.Equal(t, "-m", commitCall.Args[1])
	assert.Contains(t, commitCall.Args[2], "1.2.3")
}

// TestRun_CustomCommitMessage verifies commit message template substitution.
func TestRun_CustomCommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

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

	commitCall := mr.Calls[1]
	assert.Equal(t, "release: v1.2.3", commitCall.Args[2])
}

// TestRun_AnnotatedTag verifies git tag -a is used when AnnotatedTags is true.
func TestRun_AnnotatedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	cfg := &pipeline.Config{
		AnnotatedTags: true,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "--tags"}, mr.Calls[1].Args)
}

// TestRun_SignedTag verifies that SignTags:true produces "git tag -s <tag> -m <msg>"
// regardless of AnnotatedTags — -s implies annotated, and signing always takes precedence.
func TestRun_SignedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -s
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

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

// TestRun_GitPushTagsError propagates git push --tags failures.
func TestRun_GitPushTagsError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                         // git tag OK
	mr.QueueResponse("", "", errors.New("push rejected")) // git push --tags fails

	cfg := &pipeline.Config{}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git push --tags")
}

// TestRun_ReleaseNotesError propagates release notes generation failures.
func TestRun_ReleaseNotesError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	mr.QueueResponse("", "", nil) // git push --tags

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
	changelog := &testutil.MockGenerator{CheckErr: errors.New("git-cliff not found")}
	cfg := &pipeline.Config{Changelog: changelog}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changelog generator")
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
