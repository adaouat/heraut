//go:build e2e_forge

package forgeharness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweepDeletesOnlyOldRunResources(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	old := NewRunID(now.Add(-48*time.Hour), "aaaa")
	fresh := NewRunID(now.Add(-1*time.Hour), "bbbb")
	f := &localForge{
		tags:     []string{old + "-v0.1.0", fresh + "-v0.1.0", "v1.0.0", old + "/uat/0.1.0"},
		releases: []string{old + "-v0.1.0", fresh + "-v0.1.0"},
		branches: []string{"e2e/" + old, "e2e/" + fresh, "main"},
	}

	deleted, err := Sweep(f, 24*time.Hour, now, false)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{old + "-v0.1.0", old + "/uat/0.1.0"}, f.deletedReleases,
		"a release is deleted before its tag, for every old tag")
	assert.ElementsMatch(t, []string{old + "-v0.1.0", old + "/uat/0.1.0"}, f.deletedTags)
	assert.Equal(t, []string{"e2e/" + old}, f.deletedBranches)
	assert.Len(t, deleted, 5, "two releases, two tags, one branch")

	f2 := &localForge{tags: []string{old + "-v0.1.0"}}
	deleted, err = Sweep(f2, 24*time.Hour, now, true)
	require.NoError(t, err)
	assert.Empty(t, f2.deletedTags, "a dry run deletes nothing")
	assert.Empty(t, f2.deletedReleases)
	assert.Equal(t, []string{"release " + old + "-v0.1.0", "tag " + old + "-v0.1.0"}, deleted, "but reports what it would delete")
}

func TestSweepKeepsGoingAfterAFailureAndReportsIt(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	a := NewRunID(now.Add(-48*time.Hour), "aaaa")
	b := NewRunID(now.Add(-72*time.Hour), "bbbb")
	f := &localForge{
		tags:        []string{a + "-v1", b + "-v1"},
		failRelease: map[string]bool{a + "-v1": true},
	}

	deleted, err := Sweep(f, 24*time.Hour, now, false)

	require.Error(t, err, "the failure is reported")
	assert.Contains(t, err.Error(), a+"-v1")
	assert.Equal(t, []string{b + "-v1"}, f.deletedReleases, "the other run's release was still deleted")
	assert.ElementsMatch(t, []string{a + "-v1", b + "-v1"}, f.deletedTags, "and both tags")
	assert.Len(t, deleted, 4)
}
