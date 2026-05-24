package communique_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/communique"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_BinaryMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "communique: command not found", errors.New("exit status 127"))

	cfg := &config.ContentDriver{Generator: "communique"}
	gen := communique.New(mr, cfg)

	err := gen.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "communique")
}

func TestCheck_BinaryPresent(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("communique 1.0.0", "", nil)

	cfg := &config.ContentDriver{Generator: "communique"}
	gen := communique.New(mr, cfg)

	require.NoError(t, gen.Check())
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "communique", mr.Calls[0].Name)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
}

func TestValidate_NoConfig_Error(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "communique"}
	gen := communique.New(mr, cfg)

	err := gen.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestValidate_ConfigFileMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "communique", Config: "/nonexistent/communique.yml"}
	gen := communique.New(mr, cfg)

	err := gen.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "communique.yml")
}

func TestValidate_ConfigFileExists(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "communique.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("version: 1\n"), 0o600))

	mr := testutil.NewMockRunner()
	cfg := &config.ContentDriver{Generator: "communique", Config: cfgPath}
	gen := communique.New(mr, cfg)

	require.NoError(t, gen.Validate())
	assert.Len(t, mr.Calls, 0)
}

func TestGenerate_ExactArgs(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "communique.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("version: 1\n"), 0o600))

	mr := testutil.NewMockRunner()
	mr.QueueResponse("## Changelog\n- feat: add thing\n", "", nil)

	cfg := &config.ContentDriver{Generator: "communique", Config: cfgPath}
	gen := communique.New(mr, cfg)

	notes, err := gen.Generate("v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "## Changelog\n- feat: add thing\n", notes)

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "communique", call.Name)
	assert.Equal(t, []string{"generate", "--config", cfgPath, "v1.2.3"}, call.Args)
}

func TestGenerate_RunnerError(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "communique.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("version: 1\n"), 0o600))

	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "error: bad config", errors.New("exit status 1"))

	cfg := &config.ContentDriver{Generator: "communique", Config: cfgPath}
	gen := communique.New(mr, cfg)

	_, err := gen.Generate("v1.2.3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "communique")
}
