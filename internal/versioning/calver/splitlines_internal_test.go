package calver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLines_DropsOptionShapedNames(t *testing.T) {
	assert.Equal(t, []string{"2026.1.0", "2025.12.0"}, splitLines("2026.1.0\n--output=pwned\n2025.12.0\n"))
}
