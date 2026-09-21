package pipeline

import (
	"bytes"
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

func TestPrintResolveWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings []string
		want     string
	}{
		{"nil prints nothing", nil, ""},
		{"empty slice prints nothing", []string{}, ""},
		{"one entry", []string{"first"}, "! first\n"},
		{"two entries each get their own headline in order", []string{"first", "second"}, "! first\n! second\n"},
		{"detail lines follow the headline verbatim", []string{"held back\n  - feat!: a\n  - feat!: b"}, "! held back\n  - feat!: a\n  - feat!: b\n"},
		{
			"an entry with detail lines is followed by the next entry's headline",
			[]string{"held back\n  - feat!: a", "other"},
			"! held back\n  - feat!: a\n! other\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			printResolveWarnings(&out, tc.warnings)
			assert.Equal(t, tc.want, out.String())
		})
	}
}
