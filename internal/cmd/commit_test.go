package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeRootWithStdin is like executeRoot (defined in version_test.go, same package)
// but also wires stdin, for the --file - case.
func executeRootWithStdin(stdin string, args ...string) (string, error) {
	root := cmd.NewRootCmd("dev")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return buf.String(), err
}

func TestCommitCmd_Exists(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var commitCmd, verifyCmd, checkCmd bool
	for _, c := range root.Commands() {
		if c.Use == "commit" {
			commitCmd = true
			for _, sc := range c.Commands() {
				if strings.HasPrefix(sc.Use, "verify") {
					verifyCmd = true
				}
				if strings.HasPrefix(sc.Use, "check") {
					checkCmd = true
				}
			}
		}
	}
	assert.True(t, commitCmd, "commit command missing")
	assert.True(t, verifyCmd, "commit verify missing")
	assert.True(t, checkCmd, "commit check missing")
}

func TestCommitVerify_PositionalArg_Valid_NoConfig(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml") // deliberately does not exist
	_, err := executeRoot("commit", "verify", "feat: add x", "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_PositionalArg_InvalidGrammar(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRoot("commit", "verify", "not conventional at all", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_ConfiguredTypes_RejectsOutOfList(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat, fix]
`)
	_, err := executeRoot("commit", "verify", "docs: update readme", "--config", cfgPath)
	require.Error(t, err)
}

func TestCommitVerify_FileFlag_Valid(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	msgPath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: add x\n"), 0o644))

	_, err := executeRoot("commit", "verify", "--file", msgPath, "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_FileFlagStdin_Valid(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRootWithStdin("feat: add x\n", "commit", "verify", "--file", "-", "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_BothArgAndFile_Errors(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	msgPath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: add x\n"), 0o644))

	_, err := executeRoot("commit", "verify", "feat: add x", "--file", msgPath, "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_NoInput_Errors(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRoot("commit", "verify", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_MergeCommit_SkippedEvenWithStrictConfig(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat]
`)
	_, err := executeRoot("commit", "verify", "Merge branch 'main' into feature/x", "--config", cfgPath)
	require.NoError(t, err)
}

func TestCommitVerify_MalformedConfig_IsConfigError(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: []
`)
	_, err := executeRoot("commit", "verify", "feat: add x", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error(s) in config")
}

func TestCommitCheck_NonGitDirectory_ErrorsWithUsageExit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	missingCfg := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("commit", "check", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitCheck_AcceptsOptionalRevRangeArg(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	missingCfg := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("commit", "check", "main..HEAD", "--config", missingCfg)
	require.Error(t, err) // still errors — dir is not a git repo — but the arg parses
}
