package skill_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/skill"
)

func TestContent_HasSkillFrontmatter(t *testing.T) {
	c := skill.Content()

	assert.True(t, strings.HasPrefix(c, "---\nname: heraut\n"), "a skill file opens with its frontmatter")
	head, _, found := strings.Cut(strings.TrimPrefix(c, "---\n"), "\n---\n")
	assert.True(t, found, "the frontmatter is closed")
	assert.Contains(t, head, "description: ")
}
