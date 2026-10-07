package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockFromEnv(t *testing.T) {
	t.Run("unset falls back to the real clock", func(t *testing.T) {
		now, err := clockFromEnv(func(string) string { return "" })
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), now(), time.Minute)
	})

	t.Run("pins the RFC 3339 instant", func(t *testing.T) {
		now, err := clockFromEnv(func(k string) string {
			if k == "HERAUT_TEST_NOW" {
				return "2026-12-31T23:59:00Z"
			}
			return ""
		})
		require.NoError(t, err)
		want := time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC)
		assert.True(t, now().Equal(want), "got %s", now())
		assert.True(t, now().Equal(want), "the pinned clock must not advance")
	})

	t.Run("rejects garbage naming the variable", func(t *testing.T) {
		_, err := clockFromEnv(func(string) string { return "yesterday" })
		require.Error(t, err)
		assert.ErrorContains(t, err, "HERAUT_TEST_NOW")
		assert.ErrorContains(t, err, "yesterday")
	})
}
