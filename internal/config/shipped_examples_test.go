package config_test

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShippedExamples_LoadAndValidate is a regression net for the whole config surface: it loads
// (and validates) docs/heraut.sample.yml exactly as shipped, plus every fenced ```yaml block in
// README.md that looks like a full config (has a top-level version: key). Nothing previously
// parsed either — that gap is exactly why a versioning.prefix typo survived in README (fixed
// separately) and why docs/heraut.sample.yml's environments: reference block drifted to a
// 6-space indent nested under release.targets[0] (I6): the block only "worked" because every line
// in it was a YAML comment, so nothing ever caught it as invalid.
func TestShippedExamples_LoadAndValidate(t *testing.T) {
	t.Run("docs/heraut.sample.yml", func(t *testing.T) {
		cfg, err := config.Load("../../docs/heraut.sample.yml")
		require.NoError(t, err, "docs/heraut.sample.yml must parse as shipped")
		errs := config.Validate(cfg)
		assert.Empty(t, errs, "docs/heraut.sample.yml must validate cleanly: %v", errs)
	})

	blocks := readmeYAMLConfigBlocks(t, "../../README.md")
	require.NotEmpty(t, blocks, "expected at least one full-config yaml block in README.md")
	for i, block := range blocks {
		t.Run("README.md block", func(t *testing.T) {
			cfg, err := config.LoadFromReader(strings.NewReader(block))
			require.NoErrorf(t, err, "README.md yaml block %d must parse:\n%s", i, block)
			errs := config.Validate(cfg)
			assert.Emptyf(t, errs, "README.md yaml block %d must validate cleanly: %v\n%s", i, errs, block)
		})
	}
}

// versionKeyPattern matches a top-level (unindented) "version:" key — the marker distinguishing a
// full, loadable config block from a fragment (a snippet showing only one section).
var versionKeyPattern = regexp.MustCompile(`(?m)^version:`)

// readmeYAMLConfigBlocks extracts every fenced ```yaml ... ``` block from path that contains a
// top-level version: key, skipping fragments that are clearly partial (no version: key, e.g. a
// snippet showing only one config section).
func readmeYAMLConfigBlocks(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var blocks []string
	var current strings.Builder
	inYAMLBlock := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case !inYAMLBlock && trimmed == "```yaml":
			inYAMLBlock = true
			current.Reset()
		case inYAMLBlock && trimmed == "```":
			inYAMLBlock = false
			if versionKeyPattern.MatchString(current.String()) {
				blocks = append(blocks, current.String())
			}
		case inYAMLBlock:
			current.WriteString(line)
			current.WriteString("\n")
		}
	}
	require.NoError(t, scanner.Err())
	return blocks
}
