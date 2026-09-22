package cmd

import (
	"fmt"

	"github.com/adaouat/heraut/internal/app"
	"github.com/spf13/cobra"
)

// validateVersionOverrideFlags checks --set-version / --set-build-id the same way for every
// command that takes them. It returns plain errors; callers wrap them with exitcode.Config.
func validateVersionOverrideFlags(versionOverride, buildID string) error {
	if versionOverride != "" {
		if err := app.ValidateVersionOverride(versionOverride); err != nil {
			return err
		}
	}

	if buildID != "" {
		if versionOverride == "" {
			return fmt.Errorf("--set-build-id requires --set-version: provide the version explicitly when specifying a build ID")
		}
		if err := app.ValidateBuildID(buildID); err != nil {
			return err
		}
	}
	return nil
}

// addVersionOverrideFlags declares --set-version and --set-build-id on c.
func addVersionOverrideFlags(c *cobra.Command, versionOverride, buildID *string) {
	c.Flags().StringVar(versionOverride, "set-version", "", "override the resolved version — with or without tag prefix (e.g. 1.2.3 or v1.2.3)")
	c.Flags().StringVar(buildID, "set-build-id", "", "build ID appended to the tag via the {build} token in tag_format (requires --set-version)")
}
