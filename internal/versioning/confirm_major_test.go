package versioning_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/versioning"
)

// ADR-0078: the refusal heraut release and heraut changelog return for a major still to confirm.
func TestResult_MajorConfirmationError(t *testing.T) {
	assert.NoError(t, versioning.Result{Tag: "v2.0.0"}.MajorConfirmationError(), "nothing to confirm")

	err := versioning.Result{Tag: "v2.0.0", UnconfirmedMajor: []string{"feat!: break the api"}}.MajorConfirmationError()
	require.Error(t, err)
	assert.True(t, errors.Is(err, versioning.ErrMajorNotConfirmed))
	assert.Equal(t, "v2.0.0 is a major release, and versioning.bump.confirm_major asks to confirm it: "+
		"pass --allow-major on this run to release it\n  - feat!: break the api", err.Error())
}

// The preview heraut version next prints instead of refusing.
func TestResult_MajorConfirmationWarning(t *testing.T) {
	assert.Empty(t, versioning.Result{Tag: "v2.0.0"}.MajorConfirmationWarning())

	w := versioning.Result{Tag: "dev/2.0.0", UnconfirmedMajor: []string{"feat!: break the api"}}.MajorConfirmationWarning()
	assert.Equal(t, "dev/2.0.0 is a major release: heraut release and heraut changelog refuse it without --allow-major "+
		"(versioning.bump.confirm_major)\n  - feat!: break the api", w)
}

// At most five commits are listed, then a count of the others.
func TestResult_MajorConfirmationListsFiveCommits(t *testing.T) {
	res := versioning.Result{Tag: "v2.0.0", UnconfirmedMajor: []string{"feat!: a", "feat!: b", "feat!: c", "feat!: d", "feat!: e", "feat!: f", "feat!: g"}}
	msg := res.MajorConfirmationError().Error()
	assert.Contains(t, msg, "  - feat!: e\n  … and 2 more")
	assert.NotContains(t, msg, "feat!: f")
	assert.Contains(t, res.MajorConfirmationWarning(), "  - feat!: e\n  … and 2 more")
}
