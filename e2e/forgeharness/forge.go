//go:build e2e_forge

package forgeharness

import (
	"bytes"
	"encoding/base64"
	"errors"
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
	ReleaseURL(tag string) string
	Check() error
	Release(tag string) (Release, bool, error)
	TagCommit(tag string) (string, error)
	Branches(prefix string) ([]string, error)
	Tags(prefix string) ([]string, error)
	Releases(prefix string) ([]string, error)
	DeleteRelease(tag string) error
	DeleteTag(tag string) error
	DeleteBranch(name string) error
	OpenAndMerge(base, head, title string) (int, error)
}

// apiError is a failed `gh api` / `glab api` call. NotFound is decided from the CLI's stderr
// alone, never from the arguments (a tag name can contain "404").
type apiError struct {
	bin      string
	args     []string
	stderr   string
	err      error
	notFound bool
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s api %s: %v: %s", e.bin, strings.Join(e.args, " "), e.err, e.stderr)
}

func (e *apiError) Unwrap() error { return e.err }

func isNotFound(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.notFound
}

// api runs `bin api args…` with the token in tokenEnv, returning stdout. The token only ever
// travels in the child's environment, and the host is pinned so a user's default host cannot
// redirect the call.
func api(bin, tokenEnv, token string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, append([]string{"api"}, args...)...)
	cmd.Env = append(os.Environ(), tokenEnv+"="+token, "GH_HOST=github.com", "GITLAB_HOST=gitlab.com")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(redact(stderr.String(), token))
		return nil, &apiError{bin: bin, args: args, stderr: msg, err: err, notFound: strings.Contains(msg, "HTTP 404")}
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
