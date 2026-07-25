package cmd

import (
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/config"
)

// applyOfflineOverride forces the enrichment_policy to "disabled" when the persistent
// --offline flag is set, so the run makes no remote PR/MR metadata API calls regardless of
// the configured policy. A one-off override of enrichment_policy for a single run.
func applyOfflineOverride(cmd *cobra.Command, cfg *config.Config) {
	if cfg == nil {
		return
	}
	if offline, _ := cmd.Flags().GetBool("offline"); offline {
		if cfg.Commits == nil {
			cfg.Commits = &config.Commits{}
		}
		cfg.Commits.EnrichmentPolicy = "disabled"
	}
}
