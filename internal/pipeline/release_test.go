package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

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
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{CheckErr: errors.New("git-cliff not found")}

	cfg := &pipeline.Config{Changelog: gen}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git-cliff not found")
}

func TestCheck_PlatformFails(t *testing.T) {
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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
	assert.Equal(t, []string{"push", "--tags"}, mr.Calls[1].Args)

	// platform.CreateRelease was called with the resolved tag
	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "v1.2.3", platform.CreateReleaseCalls[0].Tag)
}

// TestRun_WithChangelog verifies changelog generation + commit + push before tagging.
func TestRun_WithChangelog(t *testing.T) {
	mr := testutil.NewMockRunner()
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
	assert.Equal(t, []string{"push"}, mr.Calls[2].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "--tags"}, mr.Calls[4].Args)
}

// TestRun_WithNotes verifies release notes are generated and passed to CreateRelease.
func TestRun_WithNotes(t *testing.T) {
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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
	mr := testutil.NewMockRunner()
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

// TestRun_ResolverError propagates resolver failures.
func TestRun_ResolverError(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &pipeline.Config{}
	p := pipeline.New(mr, &fakeResolver{err: errors.New("no commits since last tag")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

// TestRun_MultiplePlatforms verifies all platforms receive a CreateRelease call.
func TestRun_MultiplePlatforms(t *testing.T) {
	mr := testutil.NewMockRunner()
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
