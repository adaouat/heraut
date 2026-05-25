package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeSelfUpdate runs `heraut self-update <args>` against a fake release server.
func executeSelfUpdate(t *testing.T, current, latestTag string, extraArgs ...string) (string, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := selfupdate.Release{TagName: latestTag}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)

	root := cmd.NewRootCmd(current, selfupdate.WithLatestURL(srv.URL+"/releases/latest"))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"self-update"}, extraArgs...))
	_, err := root.ExecuteC()
	return buf.String(), err
}

func TestSelfUpdateCmd_ExistsOnRoot(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var found bool
	for _, c := range root.Commands() {
		if c.Use == "self-update" {
			found = true
		}
	}
	assert.True(t, found, "self-update command missing from root")
}

func TestSelfUpdateCmd_Check_UpToDate(t *testing.T) {
	out, err := executeSelfUpdate(t, "v1.2.3", "v1.2.3", "--check")
	require.NoError(t, err)
	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, "up to date")
}

func TestSelfUpdateCmd_Check_UpdateAvailable(t *testing.T) {
	out, err := executeSelfUpdate(t, "v1.2.3", "v1.3.0", "--check")
	require.Error(t, err, "should exit non-zero when update is available")
	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, "v1.3.0")
}
