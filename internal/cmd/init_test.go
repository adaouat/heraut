package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_ExistsOnRoot(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var found bool
	for _, c := range root.Commands() {
		if c.Use == "init" {
			found = true
		}
	}
	assert.True(t, found, "init command missing from root")
}

func TestInitCmd_DefaultsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "yaml-language-server")
	assert.Contains(t, string(data), "semver")
}

func TestInitCmd_DefaultsProducesValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
}

func TestInitCmd_DefaultsForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing content"), 0o644))

	_, err := executeRoot("init", "--defaults", "--force", "--config", cfgPath)
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
	assert.NotContains(t, string(data), "existing content")
}

func TestInitCmd_AutoDestDotHeraut(t *testing.T) {
	// No .config/ dir → writes to .heraut.yml in CWD.
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")

	_, err := executeRoot("init", "--defaults")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".heraut.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
}

func TestInitCmd_AutoDestDotConfig(t *testing.T) {
	// .config/ dir exists → writes to .config/heraut.yml.
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".config"), 0o755))
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")

	_, err := executeRoot("init", "--defaults")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".config", "heraut.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
	_, err = os.Stat(filepath.Join(dir, ".heraut.yml"))
	assert.True(t, os.IsNotExist(err), ".heraut.yml should not be created when .config/ exists")
}

func TestInitCmd_ExplicitConfigFlagOverridesAutoDest(t *testing.T) {
	// --config always wins over auto-dest, even when .config/ exists.
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".config"), 0o755))
	t.Chdir(dir)
	explicit := filepath.Join(dir, "custom.yml")

	_, err := executeRoot("init", "--defaults", "--config", explicit)
	require.NoError(t, err)

	_, err = os.Stat(explicit)
	require.NoError(t, err, "custom.yml should be written")
	_, err = os.Stat(filepath.Join(dir, ".config", "heraut.yml"))
	assert.True(t, os.IsNotExist(err), ".config/heraut.yml should not be created when --config is explicit")
}

func TestInitCmd_EnvVarDeterminesWriteDest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	dest := filepath.Join(dir, "custom-from-env.yml")
	t.Setenv("HERAUT_FILE", dest)

	_, err := executeRoot("init", "--defaults")
	require.NoError(t, err)

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
	_, err = os.Stat(filepath.Join(dir, ".heraut.yml"))
	assert.True(t, os.IsNotExist(err), ".heraut.yml should not be created when HERAUT_FILE is set")
}

func TestInitCmd_ExplicitFlagWinsOverEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", filepath.Join(dir, "env.yml"))
	explicit := filepath.Join(dir, "flag.yml")

	_, err := executeRoot("init", "--defaults", "--config", explicit)
	require.NoError(t, err)

	_, err = os.Stat(explicit)
	require.NoError(t, err, "flag.yml should be written")
	_, err = os.Stat(filepath.Join(dir, "env.yml"))
	assert.True(t, os.IsNotExist(err), "env.yml should not be created when --config flag is set")
}

func TestInitCmd_DefaultsWithExistingNoForceErrors(t *testing.T) {
	// T227: --defaults must not silently overwrite an existing config — require --force,
	// consistent with the rest of the CLI's destructive-action posture (e.g. promotion
	// guards). Previously this overwrote without prompt or --force; deliberately changed.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing content"), 0o644))

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(data), "the existing file must be left untouched")
}
