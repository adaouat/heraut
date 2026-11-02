package native

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

type runKey struct{}

// The command's context reaches the forge in every mode, so Ctrl-C stops a pull-request lookup.
func TestGenerate_PassesItsContextToTheForge(t *testing.T) {
	newCommit := record("aaa1111111", "A", "a@example.com", "2026-02-01T00:00:00Z", "feat: new", "")
	tests := []struct {
		name       string
		mode       Mode
		regenerate bool
		calls      int
		output     func(t *testing.T) string
		queue      func(mr *exectest.MockRunner)
	}{
		{"release notes", ModeReleaseNotes, false, 1, nil, queueReleaseNotesGit},
		{"notes", ModeNotes, false, 1, nil, func(mr *exectest.MockRunner) {
			mr.QueueResponse("v1.1.0\nv1.0.0\n", "", nil) // git tag -l --merged v1.1.0
			mr.QueueResponse(newCommit, "", nil)          // git log v1.0.0..v1.1.0
		}},
		{"changelog, first section", ModeChangelog, false, 1, nil, func(mr *exectest.MockRunner) {
			mr.QueueResponse("", "", nil)             // listTags: none
			mr.QueueResponse(newCommit, "", nil)      // new release commits
			mr.QueueResponse("aaa1111111\n", "", nil) // resolveEnrichRef("HEAD")
		}},
		{"changelog, spliced section", ModeChangelog, false, 1, func(t *testing.T) string {
			out := filepath.Join(t.TempDir(), "CHANGELOG.md")
			require.NoError(t, os.WriteFile(out, []byte("# Changelog\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"), 0o644))
			return out
		}, func(mr *exectest.MockRunner) {
			mr.QueueResponse("v1.0.0\n", "", nil)     // listTags
			mr.QueueResponse(newCommit, "", nil)      // v1.0.0..HEAD
			mr.QueueResponse("aaa1111111\n", "", nil) // resolveEnrichRef("HEAD")
		}},
		// --regenerate enriches every section, the last one a past release's.
		{"changelog, regenerated", ModeChangelog, true, 2, nil, func(mr *exectest.MockRunner) {
			mr.QueueResponse("v1.0.0\n", "", nil)     // listTags
			mr.QueueResponse(newCommit, "", nil)      // v1.0.0..HEAD
			mr.QueueResponse("aaa1111111\n", "", nil) // resolveEnrichRef("HEAD")
			mr.QueueResponse(record("bbb2222222", "B", "b@example.com", "2026-01-01T00:00:00Z", "fix: old", ""), "", nil)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			tc.queue(mr)
			mr.QueueResponse("", "", nil) // anything after enrichment (authorsBefore)
			driver := &config.ContentDriver{RegenerateChangelog: tc.regenerate}
			if tc.output != nil {
				driver.Output = tc.output(t)
			}
			forge := &countingForge{}
			g := New(mr, driver, tc.mode, WithForge(forge))

			ctx := context.WithValue(t.Context(), runKey{}, "this run")
			_, err := g.Generate(ctx, "v1.1.0", ghLC())
			require.NoError(t, err)

			require.Equal(t, tc.calls, forge.calls)
			assert.Equal(t, "this run", forge.ctx.Value(runKey{}))
		})
	}
}

// Under the default optional policy, an unavailable forge only drops the PR links. An interrupted
// run is not an unavailable forge: it stops, instead of going on to write a changelog without them.
func TestEnrichForRelease_AnInterruptFailsEvenWhenOptional(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	forge := &countingForge{err: fmt.Errorf("github graphql: %w", context.Canceled)}
	g := New(nil, &config.ContentDriver{RemoteMetadata: "optional"}, ModeChangelog, WithForge(forge))

	_, err := g.enrichForRelease(ctx, []rawCommit{{Hash: "abc"}}, "v1.0.0")

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, g.Degraded())
}

// A forge that fails on its own while the run goes on still degrades, as before.
func TestEnrichForRelease_AFailureWithTheRunStillGoingDegrades(t *testing.T) {
	forge := &countingForge{err: errors.New("API rate limit exceeded")}
	g := New(nil, &config.ContentDriver{RemoteMetadata: "optional"}, ModeChangelog, WithForge(forge))

	_, err := g.enrichForRelease(t.Context(), []rawCommit{{Hash: "abc"}}, "v1.0.0")

	require.NoError(t, err)
	assert.True(t, g.Degraded())
}
