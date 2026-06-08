package pipeline_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	flog "github.com/adaouat/forge/log"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_LogsDiagnostics asserts the release pipeline emits operator-debug
// diagnostics (step boundaries + resolved version) at Debug, and stays silent
// when the level is raised to Warn — proving the --verbose gating.
func TestRun_LogsDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		level      slog.Level
		wantLogged bool
	}{
		{"debug surfaces diagnostics", slog.LevelDebug, true},
		{"warn stays silent", slog.LevelWarn, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse("", "", nil) // git tag
			mr.QueueResponse("", "", nil) // git push --tags
			platform := &testutil.MockPlatform{PlatformName: "github"}
			cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

			var buf bytes.Buffer
			logger := flog.New(&buf, tc.level)
			p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
				WithLogger(logger)
			require.NoError(t, p.Run())

			out := buf.String()
			if tc.wantLogged {
				assert.Contains(t, out, "resolved version")
				assert.Contains(t, out, "v1.2.3")
				assert.Contains(t, out, "Resolve version") // step-boundary log
			} else {
				assert.NotContains(t, out, "resolved version")
			}
		})
	}
}
