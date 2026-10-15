package cmd

import (
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/config"
)

// applyOfflineOverride forces the enrichment_policy to "disabled" when the persistent
// --offline flag is set, so the run makes no remote PR/MR metadata API calls regardless of
// the configured policy. A one-off override of enrichment_policy for a single run.
func applyOfflineOverride(cmd *cobra.Command, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	flags := newFlagReader(cmd)
	offline := flags.Bool("offline")
	if err := flags.Err(); err != nil {
		return err
	}
	if offline {
		if cfg.Commits == nil {
			cfg.Commits = &config.Commits{}
		}
		cfg.Commits.EnrichmentPolicy = "disabled"
	}
	return nil
}
