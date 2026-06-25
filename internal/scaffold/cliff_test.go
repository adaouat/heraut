package scaffold_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
)

func TestIsCliffGenerator(t *testing.T) {
	assert.True(t, scaffold.IsCliffGenerator("git-cliff"))
	assert.False(t, scaffold.IsCliffGenerator("communique"))
	assert.False(t, scaffold.IsCliffGenerator(""))
}
