package app_test

import (
	"errors"
	"testing"

	exectest "github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
)

func TestCheckCliff_PropagatesDisabledPolicy(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	driver := &config.ContentDriver{Generator: "git-cliff"}
	degraded, err := app.CheckCliff(mr, driver, "changelog", "disabled")
	require.NoError(t, err)
	assert.False(t, degraded)
	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Args, "--offline")
}

func TestCheckCliff_OptionalDegradesAndReports(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "metadata 403", errors.New("exit status 101"))
	mr.QueueResponse("", "", nil) // --offline retry succeeds

	driver := &config.ContentDriver{Generator: "git-cliff"}
	degraded, err := app.CheckCliff(mr, driver, "changelog", "optional")
	require.NoError(t, err)
	assert.True(t, degraded)
	require.Len(t, mr.Calls, 2)
}

func TestCheckCliff_DoesNotMutateDriver(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	driver := &config.ContentDriver{Generator: "git-cliff"}
	_, _ = app.CheckCliff(mr, driver, "changelog", "disabled")
	assert.Empty(t, driver.RemoteMetadata) // the caller's driver is left untouched
}
