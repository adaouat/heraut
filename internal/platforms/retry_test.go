package platforms_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/platforms"
)

var (
	errTransient = errors.New("transient")
	errFinal     = errors.New("final")
)

func isTransient(err error) bool { return errors.Is(err, errTransient) }

func TestRetry(t *testing.T) {
	tests := []struct {
		name      string
		results   []error
		wantErr   error
		wantCalls int
	}{
		{"succeeds at once", []error{nil}, nil, 1},
		{"succeeds after a transient error", []error{errTransient, nil}, nil, 2},
		{"gives up after the last delay", []error{errTransient, errTransient, errTransient}, errTransient, 3},
		{"stops on an error that is not transient", []error{errTransient, errFinal}, errFinal, 2},
		{"never retries a first error that is not transient", []error{errFinal}, errFinal, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := platforms.Retry(t.Context(), []time.Duration{0, 0}, isTransient, func() error {
				err := tc.results[calls]
				calls++
				return err
			})

			assert.Equal(t, tc.wantCalls, calls)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestRetry_StopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	calls := 0
	start := time.Now()

	err := platforms.Retry(ctx, []time.Duration{time.Hour}, isTransient, func() error {
		calls++
		cancel()
		return errTransient
	})

	assert.Equal(t, 1, calls)
	assert.ErrorIs(t, err, errTransient, "the operation's own error, not the context's")
	assert.Less(t, time.Since(start), time.Minute, "the wait ends with the context")
}
