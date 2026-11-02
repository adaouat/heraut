package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
)

func unconfirmedMajor() versioning.Result {
	res := resolvedResult("v2.0.0")
	res.UnconfirmedMajor = []string{"feat!: break the api"}
	return res
}

// ADR-0078: a major still to confirm stops the run at "Resolve version", before any hook, write,
// commit or tag — in a dry run too, so it shows what the real run would do.
func TestRun_RefusesAnUnconfirmedMajor(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		mr := exectest.NewMockRunner()
		gen := &testutil.MockGenerator{}
		plat := &testutil.MockPlatform{PlatformName: "github"}
		cfg := &pipeline.Config{
			Changelog: gen, ChangelogFile: "CHANGELOG.md", Platforms: []port.Platform{plat},
			PostBumpHooks: []pipeline.HookStep{{Run: "make dist"}},
		}
		p := pipeline.New(mr, &fakeResolver{result: unconfirmedMajor()}, cfg, &bytes.Buffer{}, dryRun)

		err := p.Run(t.Context())

		require.Error(t, err, "dry run %v", dryRun)
		assert.True(t, errors.Is(err, versioning.ErrMajorNotConfirmed), "dry run %v: %v", dryRun, err)
		assert.Empty(t, mr.Calls, "no hook and no git command")
		assert.Empty(t, gen.GenerateCalls)
		assert.Empty(t, plat.CreateReleaseCalls)
	}
}

func TestPlan_RefusesAnUnconfirmedMajor(t *testing.T) {
	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: unconfirmedMajor()}, &pipeline.Config{}, &bytes.Buffer{}, true)

	_, err := p.Plan()

	assert.True(t, errors.Is(err, versioning.ErrMajorNotConfirmed), "%v", err)
}

func TestChangelogRun_RefusesAnUnconfirmedMajor(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		mr := exectest.NewMockRunner()
		gen := &testutil.MockGenerator{}
		cfg := &pipeline.ChangelogConfig{Changelog: gen, ChangelogFile: "CHANGELOG.md", Tag: true}
		p := pipeline.NewChangelog(mr, &fakeResolver{result: unconfirmedMajor()}, cfg, &bytes.Buffer{}, dryRun)

		err := p.Run(t.Context())

		assert.True(t, errors.Is(err, versioning.ErrMajorNotConfirmed), "dry run %v: %v", dryRun, err)
		assert.Empty(t, mr.Calls)
		assert.Empty(t, gen.GenerateCalls)
	}
}
