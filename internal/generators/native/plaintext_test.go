package native_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/generators/native"
)

func TestPlainText(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"heading is a plain line", "### Features\n\n- a", "Features\n\n- a"},
		{"scope emphasis", "- *(auth)* Add login", "- (auth) Add login"},
		{"breaking marker keeps the brackets", "- [**breaking**] Drop v1", "- [breaking] Drop v1"},
		{"link keeps its text only", "- Fix ([PROJ-12](https://example.com/PROJ-12))", "- Fix (PROJ-12)"},
		{"code span loses its backticks", "- Add `--force` flag", "- Add --force flag"},
		{"intraword underscore is untouched", "- Handle snake_case keys", "- Handle snake_case keys"},
		{"escaped punctuation is unescaped", `- Rename a\_b`, "- Rename a_b"},
		{"emphasis written as stars is formatting", "- Add *stars* support", "- Add stars support"},
		{"nested list keeps its depth", "- a\n  - b", "- a\n  - b"},
		{"groups are separated by one blank line", "### A\n\n- x\n\n### B\n\n- y", "A\n\n- x\n\nB\n\n- y"},
		{"loose list collapses to one item per line", "### A\n\n- x\n\n- y\n\n\n### B\n\n- z", "A\n\n- x\n- y\n\nB\n\n- z"},
		{"bare url survives", "- See <https://example.com>", "- See https://example.com"},
		{"empty stays empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, native.PlainText(tc.in))
		})
	}
}

func TestPlainText_SampleRange(t *testing.T) {
	in := "### Features\n\n- *(auth)* Add biometric login\n- [**breaking**] Drop the v1 sync API ([PROJ-12](https://example.com/PROJ-12))\n\n### Bug Fixes\n\n- *(ui)* Fix crash when rotating the settings screen"
	want := "Features\n\n- (auth) Add biometric login\n- [breaking] Drop the v1 sync API (PROJ-12)\n\nBug Fixes\n\n- (ui) Fix crash when rotating the settings screen"
	assert.Equal(t, want, native.PlainText(in))
}
