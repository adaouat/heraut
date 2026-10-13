package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const notesCfg = "version: \"1\"\nversioning:\n  strategy: semver\n"

func notesHistory() []step {
	return []step{commit("feat(auth): add login"), tag("v1.0.0"), commit("fix(ui): stop the crash"), tag("v1.1.0")}
}

func TestNotes(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "markdown by default", config: notesCfg, history: notesHistory(),
			args: []string{"notes", "v1.1.0", "--offline"}, wantOut: "### 🐛 Bug Fixes\n\n- *(ui)* Stop the crash"},
		{name: "text format has no markdown characters", config: notesCfg, history: notesHistory(),
			args: []string{"notes", "v1.1.0", "--offline", "--format", "text"}, wantOut: "🐛 Bug Fixes\n\n- (ui) Stop the crash"},
		{name: "an unknown --from names the tag", config: notesCfg, history: notesHistory(),
			args:     []string{"notes", "v1.1.0", "--offline", "--format", "text", "--from", "v0.0.0"},
			wantExit: exitRuntime, wantText: []string{"v0.0.0"}},
		{name: "a notes block can bring the heading back", history: notesHistory(),
			config: notesCfg + "notes:\n  rendering:\n    templates:\n      release:\n        section: \"## {{ .Version }}\"\n",
			args:   []string{"notes", "v1.1.0", "--offline"}, wantOut: "## 1.1.0\n\n### 🐛 Bug Fixes\n\n- *(ui)* Stop the crash"},
		{name: "unknown tag", config: notesCfg, history: notesHistory(),
			args: []string{"notes", "v9.9.9", "--offline"}, wantExit: exitRuntime, wantText: []string{"v9.9.9"}},
		{name: "bad format is a usage error", config: notesCfg, history: notesHistory(),
			args: []string{"notes", "v1.1.0", "--offline", "--format", "html"}, wantExit: exitUsage, wantText: []string{"markdown", "text"}},
	})
}

func TestNotes_OutputWritesTheFileAndKeepsStdoutEmpty(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(notesCfg)
	applySteps(repo, notesHistory())
	file := filepath.Join(t.TempDir(), "store.txt")

	res := repo.Run(bin, nil, "notes", "v1.1.0", "--offline", "--format", "text", "--output", file)

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Empty(t, strings.TrimSpace(res.Stdout))
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "🐛 Bug Fixes\n\n- (ui) Stop the crash\n", string(b))
}

func TestNotes_Bounds(t *testing.T) {
	bin := harness.Binary(t)
	rcHistory := []step{
		commit("feat: add alpha"), tag("v1.0.0"),
		commit("feat: add beta"), tag("v1.1.0-rc.1"),
		commit("fix: repair gamma"), tag("v1.1.0-rc.2"),
		commit("fix: repair delta"), tag("v1.1.0"),
	}
	// maintenance forks from v1.0.0; main carries v1.1.0 on its own line
	lines := []step{
		commit("feat: add alpha"), tag("v1.0.0"), checkout("maint"), switchTo("main"),
		commit("feat: add main-only"), tag("v1.1.0"),
		switchTo("maint"), commit("fix: patch maint-only"), tag("v1.0.1"),
	}
	runScenarios(t, bin, nil, []scenario{
		{name: "a final spans its release candidates", config: notesCfg, history: rcHistory,
			args: []string{"notes", "v1.1.0", "--offline"}, wantOut: "### 🚀 Features\n\n- Add beta\n\n\n### 🐛 Bug Fixes\n\n- Repair gamma\n\n- Repair delta"},
		{name: "a pre-release stops at the previous tag", config: notesCfg, history: rcHistory,
			args: []string{"notes", "v1.1.0-rc.2", "--offline"}, wantOut: "### 🐛 Bug Fixes\n\n- Repair gamma"},
		{name: "--from spans several builds", config: notesCfg, history: rcHistory,
			args: []string{"notes", "v1.1.0-rc.2", "--offline", "--from", "v1.0.0"}, wantOut: "### 🚀 Features\n\n- Add beta\n\n\n### 🐛 Bug Fixes\n\n- Repair gamma"},
		{name: "a maintenance tag stays on its line", config: notesCfg, history: lines,
			args: []string{"notes", "v1.0.1", "--offline"}, wantOut: "### 🐛 Bug Fixes\n\n- Patch maint-only"},
		{name: "the main line does not see the maintenance fix", config: notesCfg, history: lines,
			args: []string{"notes", "v1.1.0", "--offline"}, wantOut: "### 🚀 Features\n\n- Add main-only"},
		{name: "--from outside the tag's history is refused", config: notesCfg, history: lines,
			args: []string{"notes", "v1.0.1", "--offline", "--from", "v1.1.0"}, wantExit: exitRuntime,
			wantText: []string{"v1.1.0"}},
	})
}

const notesPerEnvCfg = `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  uat:
    tag_format: "uat/{version}"
    bump: auto
`

func TestNotes_PerEnv(t *testing.T) {
	bin := harness.Binary(t)
	history := []step{commit("feat: add alpha"), tag("dev/0.1.0"), commit("fix: repair beta"), tag("dev/0.1.1")}
	runScenarios(t, bin, nil, []scenario{
		{name: "the env's previous tag bounds the range", config: notesPerEnvCfg, history: history,
			args: []string{"notes", "dev/0.1.1", "--env", "dev", "--offline"}, wantOut: "### 🐛 Bug Fixes\n\n- Repair beta"},
		{name: "--env is required", config: notesPerEnvCfg, history: history,
			args: []string{"notes", "dev/0.1.1", "--offline"}, wantExit: exitConfig, wantText: []string{"--env"}},
		{name: "another env's tag is refused", config: notesPerEnvCfg, history: history,
			args: []string{"notes", "dev/0.1.1", "--env", "uat", "--offline"}, wantExit: exitRuntime,
			wantText: []string{"dev/0.1.1"}},
	})
}
