package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/ui"
	"github.com/stretchr/testify/assert"
)

// All tests use *bytes.Buffer as the writer, which is not a TTY.
// colorprofile.Detect returns NoTTY for a buffer → helpers return plain text.
// This is the CI/pipe path and the most important correctness guarantee.

func TestStatusHelpers_NoTTY_Format(t *testing.T) {
	w := &bytes.Buffer{}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Success", ui.Success(w, "config: ok"), "✓ config: ok"},
		{"Err", ui.Err(w, "git not found"), "✗ git not found"},
		{"Warn", ui.Warn(w, "cliff changelog: skip"), "! cliff changelog: skip"},
		{"Info", ui.Info(w, "hint: set GH_TOKEN"), "  hint: set GH_TOKEN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}

func TestStatusHelpers_MessageAlwaysPreserved(t *testing.T) {
	w := &bytes.Buffer{}
	msg := "git user.name: not configured; run: git config user.name <name>"
	assert.True(t, strings.HasSuffix(ui.Err(w, msg), msg))
	assert.True(t, strings.HasSuffix(ui.Success(w, msg), msg))
	assert.True(t, strings.HasSuffix(ui.Warn(w, msg), msg))
	assert.True(t, strings.HasSuffix(ui.Info(w, msg), strings.TrimLeft(msg, " ")))
}

func TestStatusHelpers_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// NO_COLOR must disable styling regardless of writer type.
	// We re-verify the plain-text contract here so the env var path is explicit.
	w := &bytes.Buffer{}
	assert.Equal(t, "✓ all good", ui.Success(w, "all good"))
	assert.Equal(t, "✗ failed", ui.Err(w, "failed"))
	assert.Equal(t, "! warning", ui.Warn(w, "warning"))
	assert.Equal(t, "  hint: x", ui.Info(w, "hint: x"))
}
