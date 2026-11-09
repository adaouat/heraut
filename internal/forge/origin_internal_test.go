package forge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitOrigin(t *testing.T) {
	tests := []struct {
		name, origin                   string
		typ, host, project, repository string
		ok                             bool
	}{
		// the forms already read
		{name: "scp-like ssh", origin: "git@github.com:acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "https", origin: "https://github.com/acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "https with credentials", origin: "https://ci:secret@gitlab.com/group/sub/app.git", typ: "gitlab", host: "https://gitlab.com", project: "group/sub/app", ok: true},
		{name: "azure ssh", origin: "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo", typ: "azure_devops", host: "https://dev.azure.com", project: "myorg/myproject", repository: "myrepo", ok: true},
		// the forms that went offline
		{name: "ssh URL", origin: "ssh://git@github.com/acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "ssh URL with a port", origin: "ssh://git@gitlab.com:22/group/app.git", typ: "gitlab", host: "https://gitlab.com", project: "group/app", ok: true},
		{name: "GitHub's ssh over 443", origin: "ssh://git@ssh.github.com:443/acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "GitLab's ssh over 443", origin: "ssh://git@altssh.gitlab.com:443/group/app.git", typ: "gitlab", host: "https://gitlab.com", project: "group/app", ok: true},
		{name: "https with the default port", origin: "https://github.com:443/acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "mixed-case host", origin: "git@GitHub.com:acme/app.git", typ: "github", host: "https://github.com", project: "acme/app", ok: true},
		{name: "mixed-case https", origin: "HTTPS://GitLab.com/group/app", typ: "gitlab", host: "https://gitlab.com", project: "group/app", ok: true},
		{name: "azure ssh URL", origin: "ssh://git@ssh.dev.azure.com/v3/myorg/myproject/myrepo", typ: "azure_devops", host: "https://dev.azure.com", project: "myorg/myproject", repository: "myrepo", ok: true},
		// not a known public host, or not an origin at all
		{name: "self-hosted", origin: "ssh://git@git.example.com:2222/acme/app.git"},
		{name: "a local path", origin: "/srv/git/app.git"},
		{name: "file URL", origin: "file:///srv/git/app.git"},
		{name: "no path", origin: "https://github.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typ, host, project, repository, ok := parseGitOrigin(tc.origin)

			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.typ, typ)
			assert.Equal(t, tc.host, host)
			assert.Equal(t, tc.project, project)
			assert.Equal(t, tc.repository, repository)
		})
	}
}
