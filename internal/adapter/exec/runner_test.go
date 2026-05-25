package exec_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	exec_pkg "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/testutil"
)

func TestRunner_Run_success(t *testing.T) {
	testutil.FakeBin(t, "heraut_greet", "#!/bin/sh\necho hello")
	r := exec_pkg.New(false, false)
	stdout, stderr, err := r.Run("heraut_greet")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", stdout)
	assert.Empty(t, stderr)
}

func TestRunner_Run_dryRun(t *testing.T) {
	// A nonexistent binary would fail if actually executed.
	r := exec_pkg.New(true, false)
	stdout, stderr, err := r.Run("nonexistent_xyzzy_heraut_abc")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunner_Run_failure(t *testing.T) {
	testutil.FakeBin(t, "heraut_fail", "#!/bin/sh\necho oops >&2\nexit 1")
	r := exec_pkg.New(false, false)
	stdout, stderr, err := r.Run("heraut_fail")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "oops\n", stderr)
}

func TestRunner_RunEnv_propagatesEnv(t *testing.T) {
	testutil.FakeBin(t, "heraut_printvar", "#!/bin/sh\necho $MY_TEST_VAR")
	r := exec_pkg.New(false, false)
	stdout, _, err := r.RunEnv([]string{"MY_TEST_VAR=testvalue"}, "heraut_printvar")
	require.NoError(t, err)
	assert.Equal(t, "testvalue\n", stdout)
}

func TestRunner_RunEnv_dryRun(t *testing.T) {
	r := exec_pkg.New(true, false)
	stdout, stderr, err := r.RunEnv([]string{"KEY=val"}, "nonexistent_xyzzy_heraut_abc")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunner_Run_verbose(t *testing.T) {
	testutil.FakeBin(t, "heraut_verbose", "#!/bin/sh\necho done")
	var buf bytes.Buffer
	r := exec_pkg.New(false, true)
	r.Out = &buf
	stdout, _, err := r.Run("heraut_verbose")
	require.NoError(t, err)
	assert.Equal(t, "done\n", stdout)
	assert.Contains(t, buf.String(), "heraut_verbose")
}

func TestRunner_Run_verbose_echoesOutput(t *testing.T) {
	testutil.FakeBin(t, "heraut_echo", "#!/bin/sh\necho out-line\necho err-line >&2")
	var buf bytes.Buffer
	r := exec_pkg.New(false, true)
	r.Out = &buf
	_, _, err := r.Run("heraut_echo")
	require.NoError(t, err)

	logged := buf.String()
	assert.Contains(t, logged, "[exec] heraut_echo")
	assert.Contains(t, logged, "out-line", "verbose should echo captured stdout")
	assert.Contains(t, logged, "err-line", "verbose should echo captured stderr")
	// Echoed output lines are indented under the [exec] line.
	assert.Contains(t, logged, "  out-line")
	assert.Contains(t, logged, "  err-line")
}

func TestRunner_Run_failure_includesStderr(t *testing.T) {
	testutil.FakeBin(t, "heraut_failmsg", "#!/bin/sh\necho 'boom detail' >&2\nexit 1")
	r := exec_pkg.New(false, false)
	_, _, err := r.Run("heraut_failmsg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "heraut_failmsg", "error should name the failing command")
	assert.Contains(t, err.Error(), "boom detail", "error should carry the command's stderr")
}

func TestRunner_Run_failure_noStderr_cleanMessage(t *testing.T) {
	testutil.FakeBin(t, "heraut_failquiet", "#!/bin/sh\nexit 1")
	r := exec_pkg.New(false, false)
	_, _, err := r.Run("heraut_failquiet")
	require.Error(t, err)
	// No stderr → no trailing ": " artifact appended after the exit status.
	assert.False(t, strings.HasSuffix(err.Error(), ": "), "error message should not end with a dangling colon")
	assert.Contains(t, err.Error(), "heraut_failquiet")
}
