package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestCheckConfigJSON(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "valid", config: semverCfg(""),
			args:    []string{"check", "config", "--json"},
			wantOut: `{"ok":true,"data":{"path":".heraut.yml","source":".heraut.yml","found":true,"valid":true,"errors":[]},"warnings":[]}`},
		{name: "findings carry their path, message and hint", config: "version: \"1\"\nversioning:\n  strategy: semvr\n",
			args:     []string{"check", "config", "--json"},
			wantExit: exitConfig,
			wantText: []string{`"ok":false`, `"valid":false`, `"path":"versioning.strategy"`,
				`"message":"\"semvr\" is not a valid strategy"`, `"hint":"valid strategies: semver, calver`,
				`"code":"config"`}},
		{name: "malformed YAML is an error without findings", config: "version: [unclosed",
			args:     []string{"check", "config", "--json"},
			wantExit: exitConfig, wantText: []string{`"ok":false`, `"code":"config"`, "yaml: line 1"}, notText: []string{`"valid"`}},
	})
}

func TestCommitVerifyJSON(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "valid", config: semverCfg(""),
			args:    []string{"commit", "verify", "feat(auth)!: add login", "--json"},
			wantOut: `{"ok":true,"data":{"type":"feat","scope":"auth","breaking":true,"description":"add login"},"warnings":[]}`},
		{name: "a merge commit is skipped", config: semverCfg(""),
			args:    []string{"commit", "verify", "Merge branch 'x' into main", "--json"},
			wantOut: `{"ok":true,"data":{"skipped":true},"warnings":[]}`},
		{name: "invalid", config: semverCfg(""),
			args:     []string{"commit", "verify", "not conventional", "--json"},
			wantExit: exitUsage, wantText: []string{`"ok":false`, `"code":"usage"`}},
		{name: "a bad config keeps stdout to the envelope", config: "version: \"1\"\nversioning:\n  strategy: semvr\n",
			args:     []string{"commit", "verify", "feat: x", "--json"},
			wantExit: exitConfig, wantText: []string{`"code":"config"`}},
	})
}

// TestCheckRuntimeJSON does not pin the exit code: whether gh is installed is the machine's business.
func TestCheckRuntimeJSON(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(semverCfg(""))
	repo.Commit("feat: a")

	res := repo.Run(bin, nil, "check", "runtime", "--json")

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Checks []struct{ Section, Name, Status string } `json:"checks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &env), "stdout:\n%s", res.Stdout)
	assert.Equal(t, res.ExitCode == exitOK, env.OK)
	require.NotEmpty(t, env.Data.Checks)
	assert.Equal(t, "Git", env.Data.Checks[0].Section)
	assert.Equal(t, "git", env.Data.Checks[0].Name)
	assert.Equal(t, "ok", env.Data.Checks[0].Status)
}
