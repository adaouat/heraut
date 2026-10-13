package perenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLines_DropsOptionShapedNames(t *testing.T) {
	assert.Equal(t, []string{"uat/1.1.0", "uat/1.0.0"}, splitLines("uat/1.1.0\n--output=pwned\nuat/1.0.0\n"))
}
