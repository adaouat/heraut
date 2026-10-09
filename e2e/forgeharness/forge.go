//go:build e2e_forge

package forgeharness

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Release is what the scenarios assert on, whichever forge produced it.
type Release struct {
	Tag        string
	Body       string
	Prerelease bool
	Draft      bool
	AssetCount int
}

// Forge is one sandbox repository on one hosting platform.
type Forge interface {
	Name() string
	Platform() string
	Coordinates() string
	CoordinatesKey() string
	TokenEnv() string
	Token() string
	CloneURL() string
	GitAuthHeader() string
	HasPreReleaseFlag() bool
	URLTag(tag string) string
	Check() error
	Release(tag string) (Release, bool, error)
	TagCommit(tag string) (string, error)
	Branches(prefix string) ([]string, error)
	Tags(prefix string) ([]string, error)
	Releases(prefix string) ([]string, error)
	DeleteRelease(tag string) error
	DeleteTag(tag string) error
	DeleteBranch(name string) error
}

// api runs `bin api args…` with the token in tokenEnv, returning stdout. The token only ever
// travels in the child's environment.
func api(bin, tokenEnv, token string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, append([]string{"api"}, args...)...)
	cmd.Env = append(os.Environ(), tokenEnv+"="+token)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s api %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(redact(stderr.String(), token)))
	}
	return stdout.Bytes(), nil
}

func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}

func basicAuth(user, token string) string {
	return "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+token))
}

func escape(s string) string { return url.PathEscape(s) }

func hasPrefix(names []string, prefix string) []string {
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}
