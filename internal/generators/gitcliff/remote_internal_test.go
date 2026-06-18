package gitcliff

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalTOML is a minimal valid git-cliff TOML to use as the base in injectRemote tests.
const minimalTOML = `
[changelog]
header = ""
body = ""
`

func TestInjectRemote_GitHub(t *testing.T) {
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	got, err := injectRemote(minimalTOML, lc)
	require.NoError(t, err)
	assert.Contains(t, got, `[remote.github]`)
	assert.Contains(t, got, `owner = 'acme'`)
	assert.Contains(t, got, `repo = 'widget'`)
	assert.NotContains(t, got, `[remote.gitlab]`)
}

func TestInjectRemote_GitLab_NestedGroup(t *testing.T) {
	lc := &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"}
	got, err := injectRemote(minimalTOML, lc)
	require.NoError(t, err)
	assert.Contains(t, got, `[remote.gitlab]`)
	assert.Contains(t, got, `owner = 'group/sub'`)
	assert.Contains(t, got, `repo = 'proj'`)
	assert.NotContains(t, got, `[remote.github]`)
}

func TestInjectRemote_NilContext(t *testing.T) {
	got, err := injectRemote(minimalTOML, nil)
	require.NoError(t, err)
	assert.Equal(t, minimalTOML, got, "nil lc must leave TOML unchanged")
}

func TestInjectRemote_EmptyOwner(t *testing.T) {
	lc := &port.LinkContext{BaseURL: "https://github.com/acme/widget", Repo: "widget", Platform: "github"}
	got, err := injectRemote(minimalTOML, lc)
	require.NoError(t, err)
	assert.Equal(t, minimalTOML, got, "empty Owner must leave TOML unchanged")
}

func TestInjectRemote_EmptyRepo(t *testing.T) {
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Platform: "github"}
	got, err := injectRemote(minimalTOML, lc)
	require.NoError(t, err)
	assert.Equal(t, minimalTOML, got, "empty Repo must leave TOML unchanged")
}

func TestInjectRemote_UserAlreadyDeclaredSection_NotOverridden(t *testing.T) {
	// User's override config already has [remote.github] with custom values.
	base := minimalTOML + `
[remote.github]
owner = "custom-org"
repo = "custom-repo"
`
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	got, err := injectRemote(base, lc)
	require.NoError(t, err)
	// heraut must not overwrite the user's values
	assert.True(t, strings.Contains(got, "custom-org"), "user-declared owner must be preserved")
	assert.True(t, strings.Contains(got, "custom-repo"), "user-declared repo must be preserved")
	assert.False(t, strings.Contains(got, "acme"), "heraut must not inject its owner when user declared the section")
}

func TestInjectRemote_UserDeclaredOtherPlatform_InjectsOwn(t *testing.T) {
	// User has [remote.gitlab] but we're running the GitHub platform — heraut injects github.
	base := minimalTOML + `
[remote.gitlab]
owner = "gl-group"
repo = "gl-repo"
`
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	got, err := injectRemote(base, lc)
	require.NoError(t, err)
	assert.Contains(t, got, `[remote.github]`)
	assert.Contains(t, got, "acme")
	assert.Contains(t, got, "[remote.gitlab]", "existing gitlab section must be preserved")
}

func TestInjectRemote_PreservesExistingTOML(t *testing.T) {
	base := minimalTOML + `
[git]
commit_parsers = [{message = "^feat", group = "Features"}]
`
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	got, err := injectRemote(base, lc)
	require.NoError(t, err)
	assert.Contains(t, got, "commit_parsers", "existing [git] section must be preserved")
	assert.Contains(t, got, "[remote.github]")
}
