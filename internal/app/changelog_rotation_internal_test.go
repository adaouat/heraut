package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/port"
)

// stubGenerator is a minimal port.Generator double used to prove wrapWithRotation's passthrough
// behavior and Check/Validate delegation without constructing a real native.Generator.
type stubGenerator struct {
	checkErr, validateErr error
}

func (s *stubGenerator) Check() error    { return s.checkErr }
func (s *stubGenerator) Validate() error { return s.validateErr }
func (s *stubGenerator) Generate(tag string, lc *port.LinkContext) (string, error) {
	return "stub:" + tag, nil
}

func TestWrapWithRotation_NoTokens_Passthrough(t *testing.T) {
	gen := &stubGenerator{}
	driver := &config.ContentDriver{Output: "CHANGELOG.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}

	got := wrapWithRotation(gen, nil, cfg, driver, "", false, false, nil, "")

	assert.Same(t, gen, got, "no rotation tokens must return the original generator unchanged")
}

func TestWrapWithRotation_WithTokens_DelegatesCheckAndValidate(t *testing.T) {
	wantErr := assert.AnError
	gen := &stubGenerator{checkErr: wantErr, validateErr: wantErr}
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}

	got := wrapWithRotation(gen, nil, cfg, driver, "", false, false, nil, "")

	assert.NotSame(t, gen, got, "tokens present must wrap, not pass through")
	assert.ErrorIs(t, got.Check(), wantErr)
	assert.ErrorIs(t, got.Validate(), wantErr)
}

func TestRotatingGenerator_ResolveDriver_CalverSingleToken(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY"}}

	got, err := rg.resolveDriver("2026.05.3")
	require.NoError(t, err)
	assert.Equal(t, "CHANGELOG_2026.md", got.Output)
	assert.Equal(t, `^2026\.`, got.TagPattern)
}

func TestRotatingGenerator_ResolveDriver_CalverMultiToken(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}_{MM}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY", "MM"}}

	got, err := rg.resolveDriver("2026.05.3")
	require.NoError(t, err)
	assert.Equal(t, "CHANGELOG_2026_05.md", got.Output)
	assert.Equal(t, `^2026\.05\.`, got.TagPattern)
}

func TestRotatingGenerator_ResolveDriver_CalverStripsTagPrefix(t *testing.T) {
	prefix := "v"
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH", TagPrefix: &prefix}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY"}}

	got, err := rg.resolveDriver("v2026.05.3")
	require.NoError(t, err)
	assert.Equal(t, "CHANGELOG_2026.md", got.Output)
}

func TestRotatingGenerator_ResolveDriver_SemverMajorOnly(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{MAJOR}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"MAJOR"}}

	got, err := rg.resolveDriver("v1.4.2")
	require.NoError(t, err)
	assert.Equal(t, "CHANGELOG_1.md", got.Output)
	assert.Equal(t, `^1\.`, got.TagPattern)
}

func TestRotatingGenerator_ResolveDriver_SemverMajorMinor(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{MAJOR}_{MINOR}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"MAJOR", "MINOR"}}

	got, err := rg.resolveDriver("v1.4.2")
	require.NoError(t, err)
	assert.Equal(t, "CHANGELOG_1_4.md", got.Output)
	assert.Equal(t, `^1\.4\.`, got.TagPattern)
}

func TestRotatingGenerator_ResolveDriver_ExplicitTagPatternWins(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md", TagPattern: "custom-pattern"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY"}}

	got, err := rg.resolveDriver("2026.05.3")
	require.NoError(t, err)
	assert.Equal(t, "custom-pattern", got.TagPattern, "explicit user tag_pattern must win over derivation")
}

func TestRotatingGenerator_ResolveDriver_InvalidVersion_Error(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY"}}

	// A manual --version override that doesn't match the configured calver format.
	_, err := rg.resolveDriver("not-a-calver-version")
	require.Error(t, err)
}

func TestRotatingGenerator_ResolveDriver_UnsupportedStrategy_Error(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG_{YYYY}.md"}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"}}
	rg := &rotatingGenerator{cfg: cfg, driver: driver, tokens: []string{"YYYY"}}

	_, err := rg.resolveDriver("2026.05.3")
	require.Error(t, err)
}

// record builds one git-log record in the shape git emits for --format=logFormat: six
// \x01-delimited fields terminated by a NUL. Mirrors
// internal/generators/native/commits_internal_test.go's own helper — not importable across
// packages since it's unexported there, and duplicating six lines here beats reaching into
// native's test internals.
func record(hash, author, email, date, subject, body string) string {
	return strings.Join([]string{hash, author, email, date, subject, body}, "\x01") + "\x00"
}

// TestRotatingGenerator_Generate_WritesConcreteFile is the one full round-trip test: proves the
// decorator, wired through buildGenerator exactly like the config-build-time call, actually
// produces a file at the resolved bucket name — not the literal "{YYYY}" pattern — and that
// LastOutputPath reports it. Broader rotation scenarios (period boundaries, real git tags) are
// T248's job; this confirms the wiring itself is correct.
func TestRotatingGenerator_Generate_WritesConcreteFile(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // listTags: no tags yet (first release)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-05-01T00:00:00Z", "feat: initial", ""), "", nil) // new release: full history

	dir := t.TempDir()
	driver := &config.ContentDriver{Output: filepath.Join(dir, "CHANGELOG_{YYYY}.md")}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}

	gen := buildGenerator(mr, driver, native.ModeChangelog, "", false, false, nil, "")
	wrapped := wrapWithRotation(gen, mr, cfg, driver, "", false, false, nil, "")

	body, err := wrapped.Generate("2026.05.0", nil)
	require.NoError(t, err)
	assert.Contains(t, body, "## [2026.05.0]")

	wantPath := filepath.Join(dir, "CHANGELOG_2026.md")
	written, err := os.ReadFile(wantPath)
	require.NoError(t, err, "expected the concrete bucket file to exist, not the literal {YYYY} pattern")
	assert.Equal(t, body, string(written))

	reporter, ok := wrapped.(interface{ LastOutputPath() string })
	require.True(t, ok)
	assert.Equal(t, wantPath, reporter.LastOutputPath())

	_, statErr := os.Stat(filepath.Join(dir, "CHANGELOG_{YYYY}.md"))
	assert.True(t, os.IsNotExist(statErr), "the literal, unsubstituted pattern must never be written to disk")
}
