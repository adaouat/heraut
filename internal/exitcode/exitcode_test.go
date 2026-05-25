package exitcode_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Nil_OK(t *testing.T) {
	assert.Equal(t, exitcode.OK, exitcode.Resolve(nil))
}

func TestResolve_PlainError_Usage(t *testing.T) {
	// An unclassified error defaults to the usage/generic code (historic exit 1).
	assert.Equal(t, exitcode.Usage, exitcode.Resolve(errors.New("boom")))
}

func TestWrap_Nil_ReturnsNil(t *testing.T) {
	assert.NoError(t, exitcode.Wrap(exitcode.Config, nil))
}

func TestWrap_PreservesMessageAndUnwrap(t *testing.T) {
	base := errors.New("bad config")
	wrapped := exitcode.Wrap(exitcode.Config, base)

	require.Error(t, wrapped)
	assert.Equal(t, "bad config", wrapped.Error())
	assert.Equal(t, base, errors.Unwrap(wrapped))
	assert.Equal(t, exitcode.Config, exitcode.Resolve(wrapped))
}

func TestResolve_FindsCodeThroughFmtErrorf(t *testing.T) {
	base := exitcode.Wrap(exitcode.Promotion, errors.New("E001"))
	chained := fmt.Errorf("running pipeline: %w", base)
	assert.Equal(t, exitcode.Promotion, exitcode.Resolve(chained))
}

func TestWrap_AlreadyClassified_FirstCodeWins(t *testing.T) {
	// Re-wrapping an already-coded error must not override the original code.
	inner := exitcode.Wrap(exitcode.Promotion, errors.New("guard"))
	outer := exitcode.Wrap(exitcode.Runtime, inner)
	assert.Equal(t, exitcode.Promotion, exitcode.Resolve(outer))
}

func TestCodes_MatchSpec(t *testing.T) {
	// Spec 01 — Exit codes.
	assert.Equal(t, 0, exitcode.OK)
	assert.Equal(t, 1, exitcode.Usage)
	assert.Equal(t, 2, exitcode.Config)
	assert.Equal(t, 3, exitcode.Runtime)
	assert.Equal(t, 4, exitcode.Promotion)
	assert.Equal(t, 70, exitcode.Internal)
}
