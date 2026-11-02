package cmd_test

import (
	"encoding/json"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
)

const confirmMajorConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump:
    confirm_major: true
changelog:
  output: CHANGELOG.md
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
`

func fakeGitBreakingSinceV140(t *testing.T) {
	t.Helper()
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.4.0" ;;
  "log v1.4.0..HEAD --format=%B"*) printf "feat!: break the api\x00" ;;
  *) exit 1 ;;
esac
`)
}

// ADR-0078: release and changelog refuse a major still to confirm with the Config code, a dry run
// included.
func TestConfirmMajor_ReleaseAndChangelogRefuseWithTheConfigCode(t *testing.T) {
	for _, args := range [][]string{
		{"release", "--dry-run"},
		{"release", "--dry-run", "--json"},
		{"changelog", "--dry-run"},
	} {
		t.Run(args[0]+" "+args[len(args)-1], func(t *testing.T) {
			cfgPath := writeConfig(t, confirmMajorConfig)
			fakeGitBreakingSinceV140(t)

			out, err := executeRoot(append(args, "--config", cfgPath)...)

			require.Error(t, err, out)
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err), out)
			assert.Contains(t, out+err.Error(), "v2.0.0 is a major release")
			assert.Contains(t, out+err.Error(), "--allow-major")
		})
	}
}

func TestConfirmMajor_AllowMajorReleases(t *testing.T) {
	cfgPath := writeConfig(t, confirmMajorConfig)
	fakeGitBreakingSinceV140(t)

	out, err := executeRoot("release", "--config", cfgPath, "--dry-run", "--allow-major")
	require.NoError(t, err, out)

	assert.Contains(t, out, "v2.0.0")
	assert.NotContains(t, out, "is a major release")
}

// version next previews: the tag on stdout, exit 0, the warning on stderr.
func TestConfirmMajor_VersionNextPreviewsWithAWarning(t *testing.T) {
	cfgPath := writeConfig(t, confirmMajorConfig)
	fakeGitBreakingSinceV140(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "v2.0.0\n", stdout, "stdout must stay exactly the tag")
	assert.Contains(t, stderr, "! v2.0.0 is a major release: heraut release and heraut changelog refuse it without --allow-major")
	assert.Contains(t, stderr, "  - feat!: break the api")
}

func TestConfirmMajor_VersionNextJSONCarriesTheWarning(t *testing.T) {
	cfgPath := writeConfig(t, confirmMajorConfig)
	fakeGitBreakingSinceV140(t)

	stdout, _, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--json")
	require.NoError(t, err)

	var env struct {
		OK       bool              `json:"ok"`
		Data     map[string]string `json:"data"`
		Warnings []string          `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &env))
	assert.True(t, env.OK)
	assert.Equal(t, "v2.0.0", env.Data["tag"])
	require.Len(t, env.Warnings, 1)
	assert.Contains(t, env.Warnings[0], "refuse it without --allow-major")
}
