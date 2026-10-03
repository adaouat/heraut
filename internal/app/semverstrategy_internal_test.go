package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildReleasePipelineConfig_SemVerStrategy covers the FIX-1 wiring seam: the app layer, not
// the pipeline, knows which versioning.strategy is active, so it is the one that must tell the
// pipeline whether the active strategy is SemVer-based (pipeline.Config.SemVerStrategy) —
// isPreRelease alone cannot be trusted, since a CalVer version can itself parse as a SemVer
// pre-release (e.g. a `format: YYYY.MM.SS-PATCH` resolving "2026.10.2-0").
func TestBuildReleasePipelineConfig_SemVerStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		format   string
		want     bool
	}{
		{"semver", "", true},
		{"semver-per-env", "", true},
		{"calver", "YYYY.MM.PATCH", false},
		{"calver-per-env", "YYYY.MM.PATCH", false},
	}
	for _, tc := range tests {
		t.Run(tc.strategy, func(t *testing.T) {
			testutil.ClearCIEnv(t)
			runner := exectest.NewMockRunner()
			readRunner := exectest.NewMockRunner()
			readRunner.QueueResponse("", "", assertNoOriginErr)

			cfg := &config.Config{
				Version:    "1",
				Versioning: config.Versioning{Strategy: tc.strategy, Format: tc.format},
			}

			pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
			require.NoError(t, err)
			assert.Equal(t, tc.want, pCfg.SemVerStrategy)
		})
	}
}
