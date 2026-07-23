package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
)

const omitNote = "remote metadata unavailable — PR authors/numbers omitted"

func TestGitlabRegenWarning(t *testing.T) {
	gl := &port.LinkContext{Platform: "gitlab"}
	gh := &port.LinkContext{Platform: "github"}
	assert.Nil(t, gitlabRegenWarning(false, gl), "no warning without --regenerate")
	assert.Nil(t, gitlabRegenWarning(true, gh), "no warning on github (batched)")
	assert.Nil(t, gitlabRegenWarning(true, nil), "no warning without a remote")
	w := gitlabRegenWarning(true, gl)
	assert.Len(t, w, 1)
	assert.Contains(t, w[0], "GitLab")
}

func TestDegradedSubs(t *testing.T) {
	assert.Nil(t, degradedSubs(&testutil.MockGenerator{DegradedVal: false}), "not degraded → no subs")

	withReason := degradedSubs(&testutil.MockGenerator{DegradedVal: true, DegradedReasonV: "boom: connection reset"})
	assert.Equal(t, []string{"boom: connection reset", omitNote}, withReason, "reason then generic note")

	noReason := degradedSubs(&testutil.MockGenerator{DegradedVal: true})
	assert.Equal(t, []string{omitNote}, noReason, "no reason exposed → generic note only")
}

func TestChangelogGenResult(t *testing.T) {
	gl := &port.LinkContext{Platform: "gitlab"}

	// Degraded: labelled "without enrichment", reason + omission note, NO rate-limit heads-up.
	detail, subs := changelogGenResult(true, gl, &testutil.MockGenerator{DegradedVal: true, DegradedReasonV: "boom"})
	assert.Equal(t, "without enrichment", detail)
	assert.Equal(t, []string{"boom", omitNote}, subs)

	// Not degraded on a GitLab regeneration: no detail, rate-limit heads-up only (kept on success).
	detail, subs = changelogGenResult(true, gl, &testutil.MockGenerator{DegradedVal: false})
	assert.Empty(t, detail)
	assert.Len(t, subs, 1)
	assert.Contains(t, subs[0], "GitLab")
}
