package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTags_DropsOptionShapedNames(t *testing.T) {
	assert.Equal(t, []string{"v1.1.0", "v1.0.0"}, parseTags("v1.1.0\n--output=pwned\nv1.0.0\n"))
}
