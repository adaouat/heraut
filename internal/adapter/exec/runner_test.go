package exec_test

import (
	"bytes"
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
