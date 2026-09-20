package app

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubResolver struct {
	res versioning.Result
	err error
}

func (s stubResolver) Resolve() (versioning.Result, error) { return s.res, s.err }

func TestWarningResolver_KeepsWarningsTheInnerResolverAlreadySet(t *testing.T) {
	w := warningResolver{
		inner:    stubResolver{res: versioning.Result{Tag: "v0.69.0", Warnings: []string{"inner"}}},
		warnings: func() []string { return []string{"recorded"} },
	}

	res, err := w.Resolve()
	require.NoError(t, err)

	assert.Equal(t, []string{"inner", "recorded"}, res.Warnings)
}

func TestWarningResolver_ErrorReturnsNoWarnings(t *testing.T) {
	w := warningResolver{
		inner:    stubResolver{err: errors.New("boom")},
		warnings: func() []string { return []string{"recorded"} },
	}

	res, err := w.Resolve()
	require.Error(t, err)
	assert.Empty(t, res.Warnings, "a failed resolution must not carry warnings")
}
