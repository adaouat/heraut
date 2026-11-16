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

func TestContent_SeparatesWhatAnAgentRunsFromWhatItDocuments(t *testing.T) {
	c := skill.Content()

	assert.True(t, strings.Contains(c, "on the commands you run"), "on the commands you run")
	assert.True(t, strings.Contains(c, "## Writing docs for people"), "## Writing docs for people")
	_, docs, _ := strings.Cut(c, "## Writing docs for people")
	docs, _, _ = strings.Cut(docs, "\n## ")
	assert.True(t, strings.Contains(docs, "Leave out `--json`"), "Leave out `--json`")
	assert.True(t, strings.Contains(docs, "interactive `heraut init`"), "interactive `heraut init`")
	assert.True(t, strings.Contains(docs, "`heraut release --dry-run`"), "`heraut release --dry-run`")
}

func TestContent_ForbidsSkippingHooksUnasked(t *testing.T) {
	c := skill.Content()

	assert.True(t, strings.Contains(c, "`--no-hooks`"), "`--no-hooks`")
	assert.True(t, strings.Contains(c, "`--skip-hook`"), "`--skip-hook`")
}
