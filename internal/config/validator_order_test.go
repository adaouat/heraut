package config_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// TestValidate_environmentErrorsComeInNameOrder: the errors are printed to the user, so their
// order must not depend on Go's random map iteration.
func TestValidate_environmentErrorsComeInNameOrder(t *testing.T) {
	names := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
	var b strings.Builder
	b.WriteString("version: \"1\"\nversioning:\n  strategy: semver-per-env\nenvironments:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "  %s:\n    bump: auto\n    tag_format: \"%s/{version}\"\n    changelog:\n      tag_pattern: \"(\"\n", n, n)
	}
	cfg := mustLoad(t, b.String())

	var want []string
	for _, n := range names {
		want = append(want, "environments."+n+".changelog.tag_pattern")
	}
	for range 30 {
		var got []string
		for _, e := range config.Validate(cfg) {
			if strings.HasSuffix(e.Path, ".changelog.tag_pattern") {
				got = append(got, e.Path)
			}
		}
		if !assert.True(t, slices.Equal(want, got), "error order changed between runs: %v", got) {
			return
		}
	}
}
