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
