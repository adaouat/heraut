package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
)

const omitNote = "remote metadata unavailable — PR authors/numbers omitted"

func TestDegradedSubs(t *testing.T) {
	assert.Nil(t, degradedSubs(&testutil.MockGenerator{DegradedVal: false}), "not degraded → no subs")

	withReason := degradedSubs(&testutil.MockGenerator{DegradedVal: true, DegradedReasonV: "boom: connection reset"})
	assert.Equal(t, []string{"boom: connection reset", omitNote}, withReason, "reason then generic note")

	noReason := degradedSubs(&testutil.MockGenerator{DegradedVal: true})
	assert.Equal(t, []string{omitNote}, noReason, "no reason exposed → generic note only")
}

func TestChangelogGenResult(t *testing.T) {
	// Degraded: "without enrichment" + reason + omission note.
	detail, subs := changelogGenResult(&testutil.MockGenerator{DegradedVal: true, DegradedReasonV: "boom"})
	assert.Equal(t, "without enrichment", detail)
	assert.Equal(t, []string{"boom", omitNote}, subs)

	// Not degraded: no detail, no sub-results (GitLab is now batched — no rate-limit heads-up).
	detail, subs = changelogGenResult(&testutil.MockGenerator{DegradedVal: false})
	assert.Empty(t, detail)
	assert.Empty(t, subs)
}
