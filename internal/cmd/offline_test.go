package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestApplyOfflineOverride_ForcesDisabled(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("offline", false, "")
	require.NoError(t, cmd.Flags().Set("offline", "true"))

	cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "optional"}}
	require.NoError(t, applyOfflineOverride(cmd, cfg))
	assert.Equal(t, "disabled", cfg.EnrichmentPolicy())
}

func TestApplyOfflineOverride_UnsetLeavesPolicy(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("offline", false, "")

	cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "required"}}
	require.NoError(t, applyOfflineOverride(cmd, cfg))
	assert.Equal(t, "required", cfg.EnrichmentPolicy())
}

func TestApplyOfflineOverride_FlagNotDeclaredIsAnError(t *testing.T) {
	cfg := &config.Config{}
	assert.Error(t, applyOfflineOverride(&cobra.Command{}, cfg))
}
