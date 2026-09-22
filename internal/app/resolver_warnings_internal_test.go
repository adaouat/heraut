package app

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubResolver struct {
	res versioning.Result
	err error
}

func (s stubResolver) Resolve() (versioning.Result, error) { return s.res, s.err }

func TestWarningResolver_KeepsWarningsTheInnerResolverAlreadySet(t *testing.T) {
	w := warningResolver{
		inner:           stubResolver{res: versioning.Result{Tag: "v0.69.0", Warnings: []string{"inner"}}},
		warnings:        func() []string { return []string{"recorded"} },
		wouldBeVersions: func() []string { return nil },
	}

	res, err := w.Resolve()
	require.NoError(t, err)

	assert.Equal(t, []string{"inner", "recorded"}, res.Warnings)
}

func TestWarningResolver_ErrorReturnsNoWarnings(t *testing.T) {
	w := warningResolver{
		inner:    stubResolver{err: errors.New("boom")},
		warnings: func() []string { return []string{"recorded"} },
	}

	res, err := w.Resolve()
	require.Error(t, err)
	assert.Empty(t, res.Warnings, "a failed resolution must not carry warnings")
}

func TestRewriteHeldTags(t *testing.T) {
	tests := []struct {
		name           string
		warning        string
		wouldBeVersion string
		version        string
		tag            string
		want           string
	}{
		{
			name:           "normal case: version found inside tag with a prefix",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
			wouldBeVersion: "1.0.0",
			version:        "0.69.0",
			tag:            "v0.69.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0 (re-run with --allow-major to release v1.0.0 instead)",
		},
		{
			name:           "tag_prefix empty is a no-op rewrite",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
			wouldBeVersion: "1.0.0",
			version:        "0.69.0",
			tag:            "0.69.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
		},
		{
			name:           "version not found in tag at all: warning returned unchanged, no panic",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
			wouldBeVersion: "1.0.0",
			version:        "9.9.9",
			tag:            "v0.69.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
		},
		{
			name:           "commit-subject lines below the headline are left untouched, only the headline is rewritten",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)\n  - feat!: bump vendored lib to 0.69.0",
			wouldBeVersion: "1.0.0",
			version:        "0.69.0",
			tag:            "v0.69.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0 (re-run with --allow-major to release v1.0.0 instead)\n  - feat!: bump vendored lib to 0.69.0",
		},
		{
			// wouldBeVersion is never empty at real call sites (warningResolver.Resolve filters
			// it before calling), but the function guards it directly too — defense in depth,
			// not decoration (see mutation check in the roadmap note).
			name:           "wouldBeVersion empty is defensive: warning returned unchanged, no panic, no corruption",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
			wouldBeVersion: "",
			version:        "0.69.0",
			tag:            "v0.69.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (re-run with --allow-major to release 1.0.0 instead)",
		},
		{
			// strings.Index(tag, "") returns 0, not -1, so without the explicit version == ""
			// guard the function would proceed to compute a bogus prefix/suffix instead of
			// bailing out — this guard is load-bearing (mutation-verified below).
			name:           "version empty (defensive): returns warning unchanged",
			warning:        "unrelated warning text",
			wouldBeVersion: "1.0.0",
			version:        "",
			tag:            "v0.69.0",
			want:           "unrelated warning text",
		},
		{
			// A regression guard for the strings.NewReplacer choice over two sequential
			// strings.ReplaceAll calls: here tag's static prefix ("1.0.0-", from a contrived
			// tag_format like "1.0.0-{version}") happens to contain wouldBeVersion's own text.
			// A sequential ReplaceAll(version, tag) followed by ReplaceAll(wouldBeVersion, ...)
			// would re-scan the just-inserted tag text and corrupt it into "1.0.0-1.0.0-0.2.0";
			// NewReplacer's single simultaneous pass over the original headline does not.
			name:           "overlapping substitution text does not get re-matched (NewReplacer vs sequential ReplaceAll)",
			warning:        "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.2.0 (re-run with --allow-major to release 1.0.0 instead)",
			wouldBeVersion: "1.0.0",
			version:        "0.2.0",
			tag:            "1.0.0-0.2.0",
			want:           "major bump held back by versioning.bump.stay_at_v0: 1.0.0-1.0.0 → 1.0.0-0.2.0 (re-run with --allow-major to release 1.0.0-1.0.0 instead)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteHeldTags(tc.warning, tc.wouldBeVersion, tc.version, tc.tag)
			assert.Equal(t, tc.want, got)
		})
	}
}
