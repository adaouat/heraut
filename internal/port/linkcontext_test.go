package port_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/port"
)

func TestLinkContext_APIEnv_GitHubDefaultHost(t *testing.T) {
	lc := &port.LinkContext{
		Platform: "github",
		BaseURL:  "https://github.com",
		Token:    "t",
	}
	assert.Equal(t, []string{"GH_TOKEN=t"}, lc.APIEnv())
}

func TestLinkContext_APIEnv_GitHubGHES(t *testing.T) {
	lc := &port.LinkContext{
		Platform: "github",
		BaseURL:  "https://github.example.com",
		Token:    "t",
	}
	env := lc.APIEnv()
	assert.Len(t, env, 2)
	assert.Contains(t, env, "GH_TOKEN=t")
	assert.Contains(t, env, "GH_HOST=github.example.com")
}

func TestLinkContext_APIEnv_EmptyToken(t *testing.T) {
	lc := &port.LinkContext{
		Platform: "github",
		BaseURL:  "https://github.com",
		Token:    "",
	}
	// No GH_TOKEN= entry when token is empty; no GH_HOST= for the default host.
	assert.Empty(t, lc.APIEnv())
}

func TestLinkContext_APIEnv_GHESEmptyToken(t *testing.T) {
	// GHES with no token: GH_HOST is still set, but no GH_TOKEN= entry.
	lc := &port.LinkContext{
		Platform: "github",
		BaseURL:  "https://ghes.example.com",
		Token:    "",
	}
	env := lc.APIEnv()
	assert.Contains(t, env, "GH_HOST=ghes.example.com")
	for _, e := range env {
		assert.False(t, strings.HasPrefix(e, "GH_TOKEN="),
			"no GH_TOKEN= entry when Token is empty, got %q", e)
	}
}

func TestLinkContext_APIEnv_Nil(t *testing.T) {
	var lc *port.LinkContext
	assert.Nil(t, lc.APIEnv())
}

func TestLinkContext_APIEnv_UnknownPlatform(t *testing.T) {
	// GitLab support is not yet implemented; the current stub returns nil.
	lc := &port.LinkContext{
		Platform: "gitlab",
		Token:    "secret",
	}
	assert.Nil(t, lc.APIEnv())
}
