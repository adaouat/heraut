# E2E forge lane: harness and scenarios B1-B4 (T345c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up Lane B of the e2e suite (ADR-0066): an opt-in `e2e_forge` build-tag harness that drives the built `heraut` binary against real private GitHub and GitLab sandbox repositories, with safety guards, a per-run branch and tag namespace, API assertions, guaranteed cleanup and a sweeper for leftovers; then scenarios B1-B4 on both forges (final release with notes, pre-release, build metadata, per-env `{env}/{version}` tag with an asset).

**Architecture:** A new package `e2e/forgeharness` (every file behind `//go:build e2e_forge`) holds the configuration, token lookup, run ids, the `Forge` interface with GitHub/GitLab implementations that shell out to the real `gh api`/`glab api`, and `Workspace` (a `harness.Repo` cloned from the sandbox onto a throwaway `e2e/<run-id>` branch, with `t.Cleanup` deleting everything the run created). Scenarios live in `e2e/forge_release_test.go` (same tag). `e2e/cmd/sweep` deletes stale `e2e-*` leftovers. No production change.

**Tech Stack:** Go 1.27, testify, real `gh` and `glab`, the T345a harness.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` ("Lane B scenarios", "Isolation and cleanup (Lane B)"); ADR-0066.

## Global Constraints

- **No real data in the repository.** Sandbox coordinates and tokens come only from environment variables or the local `gh`/`glab` login; tests, docs, plans, fixtures and the roadmap use placeholders such as `<owner>/<name>-testing`. Never print a token.
- **Opt-in only.** Every Lane B file has `//go:build e2e_forge`; `go test ./...` must not compile or run them. A missing `HERAUT_E2E_GITHUB_REPO`/`HERAUT_E2E_GITLAB_PROJECT` skips with an explicit message.
- **Safety guards (hard failures, never skips):** the sandbox repository's base name must match `HERAUT_E2E_REPO_PATTERN` (default `*testing*`) and the repository must be private; every deletion targets only names that contain the run marker `e2e-<unix>-<hex>`. Cleanup failures fail the test (`t.Errorf`), they are not swallowed.
- Local runs use the developer's logged-in `gh`/`glab` credentials (read in-process, passed to children by environment variable only); CI uses `HERAUT_E2E_GITHUB_TOKEN`/`HERAUT_E2E_GITLAB_TOKEN` secrets (T345d).
- TDD: failing test first, see it fail. Unit tests of the harness run offline under `-tags e2e_forge` using fake `gh`/`glab` on `PATH` or local bare repos. Live scenarios are verified by running them against the sandboxes.
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.
- `e2e/` imports no heraut `internal/` package. Scenario assertions pin only documented behaviour.
- Scope: B1-B4. B5-B10, the workflow, the guide and `docs/guides` are T345d.

## Review Focus

1. A resource that is not ours must never be deleted: the run-marker check and the name/private guards, including a repo name that matches the pattern but is public.
2. Cleanup must run on failure and on skip-after-create, delete release before tag before branch, and report (not hide) failures.
3. No token may appear in logs, test output, error text, or committed files (the clone's `.git/config` lives in `t.TempDir()` only).
4. `go test ./...` and `go vet ./...` must be unaffected by the tagged files; golangci-lint must cover them (`build-tags`).
5. B3/B4 must exercise exactly what the contract tests cannot: `+` in tags and URLs, `/` in tags, GitLab asset upload.

---

### Task 1: Build tag in lint, configuration, guards, run ids, tokens

**Files:**
- Modify: `.golangci.yml`
- Create: `e2e/forgeharness/config.go`, `e2e/forgeharness/runid.go`, `e2e/forgeharness/token.go`
- Create: `e2e/forgeharness/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct{ GitHubRepo, GitLabProject, Pattern string }`; `LoadConfig() Config` (env `HERAUT_E2E_GITHUB_REPO`, `HERAUT_E2E_GITLAB_PROJECT`, `HERAUT_E2E_REPO_PATTERN`, default pattern `*testing*`).
  - `(Config) Guard(coordinates string) error`: base name must match the pattern.
  - `NewRunID(now time.Time, randHex string) string` → `e2e-<unix>-<randHex>`; `RandomHex() string` (4 hex chars); `RunIDTime(s string) (time.Time, bool)` parses the first `e2e-<digits>-<hex>` found in `s`.
  - `GitHubToken() (string, error)`, `GitLabToken() (string, error)`: env first, then the CLI login.

- [ ] **Step 1: Write the failing tests**

Create `e2e/forgeharness/config_test.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigAndGuard(t *testing.T) {
	t.Setenv("HERAUT_E2E_GITHUB_REPO", "acme/widget-testing")
	t.Setenv("HERAUT_E2E_GITLAB_PROJECT", "group/sub/widget-testing")
	t.Setenv("HERAUT_E2E_REPO_PATTERN", "")

	c := LoadConfig()

	assert.Equal(t, "acme/widget-testing", c.GitHubRepo)
	assert.Equal(t, "group/sub/widget-testing", c.GitLabProject)
	assert.Equal(t, "*testing*", c.Pattern, "the default pattern is a sandbox naming convention")
	assert.NoError(t, c.Guard("acme/widget-testing"))
	assert.NoError(t, c.Guard("group/sub/widget-testing"), "only the base name is matched")
	assert.Error(t, c.Guard("acme/widget"), "a production-looking name is refused")
	assert.Error(t, c.Guard("testing/widget"), "the owner part does not count")

	t.Setenv("HERAUT_E2E_REPO_PATTERN", "scratch-*")
	assert.NoError(t, LoadConfig().Guard("acme/scratch-one"))
	assert.Error(t, LoadConfig().Guard("acme/widget-testing"))
}

func TestRunIDs(t *testing.T) {
	id := NewRunID(time.Unix(1791548560, 0), "a1b2")

	assert.Equal(t, "e2e-1791548560-a1b2", id)
	assert.Len(t, RandomHex(), 4)

	ts, ok := RunIDTime("refs/heads/e2e/" + id + "-extra")
	require.True(t, ok)
	assert.Equal(t, int64(1791548560), ts.Unix())

	_, ok = RunIDTime("v1.2.3")
	assert.False(t, ok, "an ordinary tag carries no run id")
	_, ok = RunIDTime("e2e-notanumber-zz")
	assert.False(t, ok)
}

func writeFake(t *testing.T, dir, name, script string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755))
}

func TestTokensPreferTheEnvironmentThenTheCLILogin(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "gh", `[ "$1 $2" = "auth token" ] && echo cli-gh-token`)
	writeFake(t, dir, "glab", `[ "$1 $2 $3" = "auth status --show-token" ] && echo "  ✓ Token found: cli-gl-token"`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, k := range []string{"HERAUT_E2E_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN", "HERAUT_E2E_GITLAB_TOKEN", "GITLAB_TOKEN"} {
		t.Setenv(k, "")
	}

	gh, err := GitHubToken()
	require.NoError(t, err)
	assert.Equal(t, "cli-gh-token", gh)
	gl, err := GitLabToken()
	require.NoError(t, err)
	assert.Equal(t, "cli-gl-token", gl)

	t.Setenv("GH_TOKEN", "env-gh")
	t.Setenv("HERAUT_E2E_GITLAB_TOKEN", "env-gl")
	gh, _ = GitHubToken()
	gl, _ = GitLabToken()
	assert.Equal(t, "env-gh", gh)
	assert.Equal(t, "env-gl", gl, "the dedicated variable wins")

	t.Setenv("HERAUT_E2E_GITHUB_TOKEN", "dedicated")
	gh, _ = GitHubToken()
	assert.Equal(t, "dedicated", gh)
}
```

Run: `go test -tags e2e_forge ./e2e/forgeharness`
Expected: FAIL to compile, `undefined: LoadConfig`.

- [ ] **Step 2: Implement**

Create `e2e/forgeharness/config.go`:

```go
//go:build e2e_forge

// Package forgeharness drives the built heraut binary against real sandbox repositories (Lane B,
// ADR-0066). Every file is behind the e2e_forge build tag.
package forgeharness

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// Config holds the sandbox coordinates; it never contains a token.
type Config struct {
	GitHubRepo    string
	GitLabProject string
	Pattern       string
}

// LoadConfig reads the sandbox coordinates from the environment.
func LoadConfig() Config {
	c := Config{
		GitHubRepo:    os.Getenv("HERAUT_E2E_GITHUB_REPO"),
		GitLabProject: os.Getenv("HERAUT_E2E_GITLAB_PROJECT"),
		Pattern:       os.Getenv("HERAUT_E2E_REPO_PATTERN"),
	}
	if c.Pattern == "" {
		c.Pattern = "*testing*"
	}
	return c
}

// Guard refuses a repository whose base name does not match the sandbox pattern.
func (c Config) Guard(coordinates string) error {
	base := coordinates[strings.LastIndex(coordinates, "/")+1:]
	ok, err := path.Match(c.Pattern, base)
	if err != nil {
		return fmt.Errorf("invalid HERAUT_E2E_REPO_PATTERN %q: %w", c.Pattern, err)
	}
	if !ok {
		return fmt.Errorf("refusing %q: its name does not match the sandbox pattern %q", coordinates, c.Pattern)
	}
	return nil
}
```

Create `e2e/forgeharness/runid.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var runIDPattern = regexp.MustCompile(`e2e-([0-9]+)-[0-9a-f]{4}`)

// NewRunID builds the marker every resource of one run carries.
func NewRunID(now time.Time, randHex string) string {
	return fmt.Sprintf("e2e-%d-%s", now.Unix(), randHex)
}

// RandomHex returns 4 random hex characters.
func RandomHex() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// RunIDTime extracts the creation time of the first run id found in s.
func RunIDTime(s string) (time.Time, bool) {
	m := runIDPattern.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}
```

Create `e2e/forgeharness/token.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// GitHubToken returns a token from the environment, else from the gh login.
func GitHubToken() (string, error) {
	if v := firstEnv("HERAUT_E2E_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"); v != "" {
		return v, nil
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "", fmt.Errorf("no GitHub token: set HERAUT_E2E_GITHUB_TOKEN or run gh auth login")
	}
	return strings.TrimSpace(string(out)), nil
}

// GitLabToken returns a token from the environment, else from the glab login.
func GitLabToken() (string, error) {
	if v := firstEnv("HERAUT_E2E_GITLAB_TOKEN", "GITLAB_TOKEN"); v != "" {
		return v, nil
	}
	out, err := exec.Command("glab", "auth", "status", "--show-token").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if i := strings.Index(line, "Token"); i >= 0 {
				if j := strings.LastIndex(line, ": "); j >= 0 {
					if tok := strings.TrimSpace(line[j+2:]); tok != "" {
						return tok, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("no GitLab token: set HERAUT_E2E_GITLAB_TOKEN or run glab auth login")
}
```

In `.golangci.yml` add (top level, after `version: "2"`):

```yaml
run:
  build-tags:
    - e2e_forge
```

Run: `go test -count=1 -tags e2e_forge ./e2e/forgeharness && go vet ./... && go vet -tags e2e_forge ./...`
Expected: PASS and both vets clean. Also run `go test -count=1 ./...`: the forgeharness package must not break the default build (`go build ./...` must succeed; a package whose files are all excluded is skipped).

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add .golangci.yml e2e/forgeharness
git commit -F - <<'EOF'
test(e2e): add the forge lane's configuration, guards and tokens

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `Forge` interface, GitHub and GitLab implementations

**Files:**
- Create: `e2e/forgeharness/forge.go`, `e2e/forgeharness/github.go`, `e2e/forgeharness/gitlab.go`
- Create: `e2e/forgeharness/forge_test.go`

**Interfaces:**
- Consumes: Task 1 (`Config`, tokens).
- Produces:

```go
type Release struct {
	Tag        string
	Body       string
	Prerelease bool
	Draft      bool
	AssetCount int
}

type Forge interface {
	Name() string                       // "github" | "gitlab"
	Platform() string                   // heraut forges[].platform
	Coordinates() string                // owner/repo | group/project
	CoordinatesKey() string             // "repository" | "project"
	TokenEnv() string                   // "GH_TOKEN" | "GITLAB_TOKEN"
	Token() string
	CloneURL() string
	GitAuthHeader() string              // value of http.extraheader
	HasPreReleaseFlag() bool
	URLTag(tag string) string           // how the forge's release URL spells the tag
	Check() error                       // sandbox guard: name pattern + private
	Release(tag string) (Release, bool, error)
	TagCommit(tag string) (string, error)
	Branches(prefix string) ([]string, error)
	Tags(prefix string) ([]string, error)
	Releases(prefix string) ([]string, error)
	DeleteRelease(tag string) error
	DeleteTag(tag string) error
	DeleteBranch(name string) error
}

func NewGitHub(c Config, token string) Forge
func NewGitLab(c Config, token string) Forge
```

All calls go through the real `gh api` / `glab api` binaries with the token passed as `GH_TOKEN` / `GITLAB_TOKEN`. The tests below run offline against fake `gh`/`glab` scripts that record argv and return canned JSON.

- [ ] **Step 1: Write the failing tests**

Create `e2e/forgeharness/forge_test.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPI installs name as an executable that logs "name arg arg…" per call and prints reply
// (chosen by the first line of the joined args that contains a key of replies).
func fakeAPI(t *testing.T, name string, replies map[string]string) (calls func() []string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	var cases strings.Builder
	for key, reply := range replies {
		cases.WriteString("  *\"" + key + "\"*) cat <<'JSON'\n" + reply + "\nJSON\n;;\n")
	}
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\ncase \"$*\" in\n" + cases.String() + "  *) echo '{}' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		b, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

func TestGitHubForge(t *testing.T) {
	calls := fakeAPI(t, "gh", map[string]string{
		"repos/acme/widget-testing/releases?per_page=100": `[{"id":7,"tag_name":"e2e-1-aaaa-v0.1.0","body":"### Features","prerelease":true,"draft":false,"assets":[{"name":"a"}]}]`,
		"git/ref/tags/e2e-1-aaaa-v0.1.0":                  `{"object":{"type":"tag","sha":"TAGSHA"}}`,
		"git/tags/TAGSHA":                                 `{"object":{"sha":"COMMITSHA"}}`,
		"git/matching-refs/heads/e2e/":                    `[{"ref":"refs/heads/e2e/e2e-1-aaaa"}]`,
		"git/matching-refs/tags/e2e-":                     `[{"ref":"refs/tags/e2e-1-aaaa-v0.1.0"}]`,
		"repos/acme/widget-testing":                       `{"private":true}`,
	})
	f := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok")

	assert.Equal(t, "github", f.Name())
	assert.Equal(t, "repository", f.CoordinatesKey())
	assert.Equal(t, "GH_TOKEN", f.TokenEnv())
	assert.True(t, f.HasPreReleaseFlag())
	assert.Equal(t, "a+b", f.URLTag("a+b"), "GitHub release URLs keep the tag as is")
	assert.Equal(t, "https://github.com/acme/widget-testing.git", f.CloneURL())
	assert.NotContains(t, f.CloneURL(), "tok", "the token never goes into the URL")
	assert.True(t, strings.HasPrefix(f.GitAuthHeader(), "Authorization: Basic "))

	require.NoError(t, f.Check())
	rel, ok, err := f.Release("e2e-1-aaaa-v0.1.0")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Release{Tag: "e2e-1-aaaa-v0.1.0", Body: "### Features", Prerelease: true, AssetCount: 1}, rel)
	_, ok, err = f.Release("nope")
	require.NoError(t, err)
	assert.False(t, ok)
	sha, err := f.TagCommit("e2e-1-aaaa-v0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "COMMITSHA", sha, "an annotated tag is peeled to its commit")
	branches, _ := f.Branches("e2e/")
	tags, _ := f.Tags("e2e-")
	assert.Equal(t, []string{"e2e/e2e-1-aaaa"}, branches)
	assert.Equal(t, []string{"e2e-1-aaaa-v0.1.0"}, tags)

	require.NoError(t, f.DeleteRelease("e2e-1-aaaa-v0.1.0"))
	require.NoError(t, f.DeleteTag("e2e-1-aaaa-v0.1.0"))
	require.NoError(t, f.DeleteBranch("e2e/e2e-1-aaaa"))
	all := strings.Join(calls(), "\n")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/releases/7")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/git/refs/tags/e2e-1-aaaa-v0.1.0")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/git/refs/heads/e2e/e2e-1-aaaa")
}

func TestGitHubCheckRefusesAPublicOrMisnamedRepo(t *testing.T) {
	fakeAPI(t, "gh", map[string]string{"repos/acme/widget-testing": `{"private":false}`})

	err := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok").Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not private")

	err = NewGitHub(Config{GitHubRepo: "acme/widget", Pattern: "*testing*"}, "tok").Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox pattern")
}

func TestGitLabForge(t *testing.T) {
	calls := fakeAPI(t, "glab", map[string]string{
		"projects/group%2Fwidget-testing/releases/e2e-1-aaaa%2Fuat%2F0.1.0": `{"tag_name":"e2e-1-aaaa/uat/0.1.0","description":"### Features","assets":{"links":[{"name":"a"}]}}`,
		"repository/tags/e2e-1-aaaa%2Fuat%2F0.1.0":                         `{"commit":{"id":"COMMITSHA"}}`,
		"repository/branches?search=e2e%2F":                                 `[{"name":"e2e/e2e-1-aaaa"},{"name":"other"}]`,
		"repository/tags?search=e2e-":                                       `[{"name":"e2e-1-aaaa/uat/0.1.0"},{"name":"v1"}]`,
		"projects/group%2Fwidget-testing":                                   `{"visibility":"private"}`,
	})
	f := NewGitLab(Config{GitLabProject: "group/widget-testing", Pattern: "*testing*"}, "tok")

	assert.Equal(t, "gitlab", f.Name())
	assert.Equal(t, "project", f.CoordinatesKey())
	assert.Equal(t, "GITLAB_TOKEN", f.TokenEnv())
	assert.False(t, f.HasPreReleaseFlag())
	assert.Equal(t, "a%2Bb%2Fc", f.URLTag("a+b/c"), "GitLab release URLs escape + and /")
	assert.Equal(t, "https://gitlab.com/group/widget-testing.git", f.CloneURL())

	require.NoError(t, f.Check())
	rel, ok, err := f.Release("e2e-1-aaaa/uat/0.1.0")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Release{Tag: "e2e-1-aaaa/uat/0.1.0", Body: "### Features", AssetCount: 1}, rel)
	sha, err := f.TagCommit("e2e-1-aaaa/uat/0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "COMMITSHA", sha)
	branches, _ := f.Branches("e2e/")
	tags, _ := f.Tags("e2e-")
	assert.Equal(t, []string{"e2e/e2e-1-aaaa"}, branches, "the search is a substring match, so non-prefixed names are dropped")
	assert.Equal(t, []string{"e2e-1-aaaa/uat/0.1.0"}, tags)

	require.NoError(t, f.DeleteRelease("e2e-1-aaaa/uat/0.1.0"))
	require.NoError(t, f.DeleteTag("e2e-1-aaaa/uat/0.1.0"))
	require.NoError(t, f.DeleteBranch("e2e/e2e-1-aaaa"))
	all := strings.Join(calls(), "\n")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/releases/e2e-1-aaaa%2Fuat%2F0.1.0")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/repository/tags/e2e-1-aaaa%2Fuat%2F0.1.0")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/repository/branches/e2e%2Fe2e-1-aaaa")
}
```

Run: `go test -tags e2e_forge ./e2e/forgeharness -run 'Forge|Check'`
Expected: FAIL to compile, `undefined: NewGitHub`.

- [ ] **Step 2: Implement**

Create `e2e/forgeharness/forge.go`:

```go
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
```

Create `e2e/forgeharness/github.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type github struct {
	repo, pattern, token string
	cfg                  Config
}

// NewGitHub returns the sandbox repository configured in c.GitHubRepo.
func NewGitHub(c Config, token string) Forge {
	return &github{repo: c.GitHubRepo, pattern: c.Pattern, token: token, cfg: c}
}

func (g *github) Name() string              { return "github" }
func (g *github) Platform() string          { return "github" }
func (g *github) Coordinates() string       { return g.repo }
func (g *github) CoordinatesKey() string    { return "repository" }
func (g *github) TokenEnv() string          { return "GH_TOKEN" }
func (g *github) Token() string             { return g.token }
func (g *github) CloneURL() string          { return "https://github.com/" + g.repo + ".git" }
func (g *github) GitAuthHeader() string     { return basicAuth("x-access-token", g.token) }
func (g *github) HasPreReleaseFlag() bool   { return true }
func (g *github) URLTag(tag string) string  { return tag }
func (g *github) call(args ...string) ([]byte, error) {
	return api("gh", g.TokenEnv(), g.token, args...)
}

func (g *github) Check() error {
	if err := g.cfg.Guard(g.repo); err != nil {
		return err
	}
	out, err := g.call("repos/" + g.repo)
	if err != nil {
		return err
	}
	var r struct {
		Private bool `json:"private"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return err
	}
	if !r.Private {
		return fmt.Errorf("refusing %q: the repository is not private", g.repo)
	}
	return nil
}

type ghRelease struct {
	ID         int    `json:"id"`
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

func (g *github) releases() ([]ghRelease, error) {
	out, err := g.call("repos/" + g.repo + "/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var rs []ghRelease
	if err := json.Unmarshal(out, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

func (g *github) Release(tag string) (Release, bool, error) {
	rs, err := g.releases()
	if err != nil {
		return Release{}, false, err
	}
	for _, r := range rs {
		if r.TagName == tag {
			return Release{Tag: r.TagName, Body: r.Body, Prerelease: r.Prerelease, Draft: r.Draft, AssetCount: len(r.Assets)}, true, nil
		}
	}
	return Release{}, false, nil
}

func (g *github) TagCommit(tag string) (string, error) {
	out, err := g.call("repos/" + g.repo + "/git/ref/tags/" + tag)
	if err != nil {
		return "", err
	}
	var ref struct {
		Object struct{ Type, SHA string } `json:"object"`
	}
	if err := json.Unmarshal(out, &ref); err != nil {
		return "", err
	}
	if ref.Object.Type != "tag" {
		return ref.Object.SHA, nil
	}
	out, err = g.call("repos/" + g.repo + "/git/tags/" + ref.Object.SHA)
	if err != nil {
		return "", err
	}
	var t struct {
		Object struct{ SHA string } `json:"object"`
	}
	if err := json.Unmarshal(out, &t); err != nil {
		return "", err
	}
	return t.Object.SHA, nil
}

func (g *github) refs(kind, prefix string) ([]string, error) {
	out, err := g.call("repos/" + g.repo + "/git/matching-refs/" + kind + "/" + prefix)
	if err != nil {
		return nil, err
	}
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(out, &refs); err != nil {
		return nil, err
	}
	var names []string
	for _, r := range refs {
		names = append(names, strings.TrimPrefix(r.Ref, "refs/"+kind+"/"))
	}
	return names, nil
}

func (g *github) Branches(prefix string) ([]string, error) { return g.refs("heads", prefix) }
func (g *github) Tags(prefix string) ([]string, error)     { return g.refs("tags", prefix) }

func (g *github) Releases(prefix string) ([]string, error) {
	rs, err := g.releases()
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, r := range rs {
		tags = append(tags, r.TagName)
	}
	return hasPrefix(tags, prefix), nil
}

func (g *github) DeleteRelease(tag string) error {
	rs, err := g.releases()
	if err != nil {
		return err
	}
	for _, r := range rs {
		if r.TagName == tag {
			_, err := g.call("-X", "DELETE", "repos/"+g.repo+"/releases/"+strconv.Itoa(r.ID))
			return err
		}
	}
	return nil
}

func (g *github) DeleteTag(tag string) error {
	_, err := g.call("-X", "DELETE", "repos/"+g.repo+"/git/refs/tags/"+tag)
	return err
}

func (g *github) DeleteBranch(name string) error {
	_, err := g.call("-X", "DELETE", "repos/"+g.repo+"/git/refs/heads/"+name)
	return err
}
```

Note: `api -X DELETE …` — the fake logs `api -X DELETE repos/…`; keep the argument order `-X DELETE <path>`.

Create `e2e/forgeharness/gitlab.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type gitlab struct {
	project, token string
	cfg            Config
}

// NewGitLab returns the sandbox project configured in c.GitLabProject.
func NewGitLab(c Config, token string) Forge {
	return &gitlab{project: c.GitLabProject, token: token, cfg: c}
}

func (g *gitlab) Name() string            { return "gitlab" }
func (g *gitlab) Platform() string        { return "gitlab" }
func (g *gitlab) Coordinates() string     { return g.project }
func (g *gitlab) CoordinatesKey() string  { return "project" }
func (g *gitlab) TokenEnv() string        { return "GITLAB_TOKEN" }
func (g *gitlab) Token() string           { return g.token }
func (g *gitlab) CloneURL() string        { return "https://gitlab.com/" + g.project + ".git" }
func (g *gitlab) GitAuthHeader() string   { return basicAuth("oauth2", g.token) }
func (g *gitlab) HasPreReleaseFlag() bool { return false }

func (g *gitlab) URLTag(tag string) string {
	return strings.NewReplacer("+", "%2B", "/", "%2F").Replace(tag)
}

func (g *gitlab) base() string { return "projects/" + escape(g.project) }

func (g *gitlab) call(args ...string) ([]byte, error) {
	return api("glab", g.TokenEnv(), g.token, args...)
}

func (g *gitlab) Check() error {
	if err := g.cfg.Guard(g.project); err != nil {
		return err
	}
	out, err := g.call(g.base())
	if err != nil {
		return err
	}
	var p struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		return err
	}
	if p.Visibility != "private" {
		return fmt.Errorf("refusing %q: the project is not private (visibility %q)", g.project, p.Visibility)
	}
	return nil
}

func (g *gitlab) Release(tag string) (Release, bool, error) {
	out, err := g.call(g.base() + "/releases/" + escape(tag))
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return Release{}, false, nil
		}
		return Release{}, false, err
	}
	var r struct {
		TagName     string `json:"tag_name"`
		Description string `json:"description"`
		Assets      struct {
			Links []struct{ Name string } `json:"links"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return Release{}, false, err
	}
	return Release{Tag: r.TagName, Body: r.Description, AssetCount: len(r.Assets.Links)}, true, nil
}

func (g *gitlab) TagCommit(tag string) (string, error) {
	out, err := g.call(g.base() + "/repository/tags/" + escape(tag))
	if err != nil {
		return "", err
	}
	var t struct {
		Commit struct{ ID string } `json:"commit"`
	}
	if err := json.Unmarshal(out, &t); err != nil {
		return "", err
	}
	return t.Commit.ID, nil
}

func (g *gitlab) names(path, prefix string) ([]string, error) {
	out, err := g.call(path + "?search=" + escape(prefix) + "&per_page=100")
	if err != nil {
		return nil, err
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	var names []string
	for _, i := range items {
		names = append(names, i.Name)
	}
	return hasPrefix(names, prefix), nil
}

func (g *gitlab) Branches(prefix string) ([]string, error) {
	return g.names(g.base()+"/repository/branches", prefix)
}

func (g *gitlab) Tags(prefix string) ([]string, error) {
	return g.names(g.base()+"/repository/tags", prefix)
}

func (g *gitlab) Releases(prefix string) ([]string, error) {
	out, err := g.call(g.base() + "/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var rs []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(out, &rs); err != nil {
		return nil, err
	}
	var tags []string
	for _, r := range rs {
		tags = append(tags, r.TagName)
	}
	return hasPrefix(tags, prefix), nil
}

func (g *gitlab) DeleteRelease(tag string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/releases/"+escape(tag))
	return err
}

func (g *gitlab) DeleteTag(tag string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/repository/tags/"+escape(tag))
	return err
}

func (g *gitlab) DeleteBranch(name string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/repository/branches/"+escape(name))
	return err
}
```

Check details while running the tests: `url.PathEscape("e2e/")` is `e2e%2F` (the fake expects `search=e2e%2F`); a GitLab 404 from `glab api` surfaces as an error containing "404"; the GitHub fake keys match substrings of the joined args (`repos/acme/widget-testing/releases?per_page=100` etc.), and the more specific `repos/acme/widget-testing` key also matches every other call, so list keys in the map with the most specific first is not guaranteed by Go's map order: make `fakeAPI` take an ordered `[][2]string` instead of a map if a test shows the wrong reply being chosen, and keep the assertions unchanged.

Run: `go test -count=1 -tags e2e_forge ./e2e/forgeharness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/forgeharness
git commit -F - <<'EOF'
test(e2e): add GitHub and GitLab sandbox clients for the forge lane

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `Workspace`

**Files:**
- Create: `e2e/forgeharness/workspace.go`
- Create: `e2e/forgeharness/workspace_test.go`

**Interfaces:**
- Consumes: `Forge`, `NewRunID`, `RandomHex`, `harness.NewRepo/Git/WriteConfig/WriteFile/Commit` (T345a-b).
- Produces:

```go
type Workspace struct {
	Repo      *harness.Repo
	Forge     Forge
	RunID     string
	Branch    string // "e2e/<RunID>"
	TagPrefix string // "<RunID>-v"
}

func Require(t *testing.T, name string) Forge          // skips if unset; fails on a guard error
func NewWorkspace(t *testing.T, f Forge) *Workspace    // clone, branch, cleanup registered
func (w *Workspace) Config(strategyBlock, releaseExtra string) string
func (w *Workspace) Env() []string                     // the forge's token variable only
```

`Config` returns a valid `.heraut.yml`: `version: "1"`, the given `versioning:` block, `changelog.output`, a `forges:` entry for the forge and a `release.targets:` entry; `releaseExtra` is spliced inside the single target (for assets).

- [ ] **Step 1: Write the failing test**

Create `e2e/forgeharness/workspace_test.go` (uses a local bare repository as the "sandbox" via a fake `Forge`):

```go
//go:build e2e_forge

package forgeharness

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localForge is a Forge whose clone URL is a local bare repository; it records deletions.
type localForge struct {
	Forge
	url                         string
	deletedReleases, deletedTags, deletedBranches []string
	tags, releases, branches    []string
}

func (l *localForge) CloneURL() string                      { return l.url }
func (l *localForge) GitAuthHeader() string                 { return "X-Test: 1" }
func (l *localForge) Name() string                          { return "github" }
func (l *localForge) Platform() string                      { return "github" }
func (l *localForge) Coordinates() string                   { return "acme/widget-testing" }
func (l *localForge) CoordinatesKey() string                { return "repository" }
func (l *localForge) TokenEnv() string                      { return "GH_TOKEN" }
func (l *localForge) Token() string                         { return "tok" }
func (l *localForge) Tags(prefix string) ([]string, error)  { return hasPrefix(l.tags, prefix), nil }
func (l *localForge) Releases(p string) ([]string, error)   { return hasPrefix(l.releases, p), nil }
func (l *localForge) Branches(p string) ([]string, error)   { return hasPrefix(l.branches, p), nil }
func (l *localForge) DeleteRelease(tag string) error        { l.deletedReleases = append(l.deletedReleases, tag); return nil }
func (l *localForge) DeleteTag(tag string) error            { l.deletedTags = append(l.deletedTags, tag); return nil }
func (l *localForge) DeleteBranch(name string) error        { l.deletedBranches = append(l.deletedBranches, name); return nil }

func newBare(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--bare", "-b", "main", dir},
	} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	seed := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "s@example.com"},
		{"config", "user.name", "s"},
		{"commit", "-q", "--allow-empty", "-m", "chore: seed"},
		{"push", "-q", dir, "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = seed
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return dir
}

func TestWorkspaceClonesBranchesAndCleansUpWhatTheRunCreated(t *testing.T) {
	f := &localForge{url: newBare(t)}
	var runID string

	t.Run("run", func(t *testing.T) {
		w := NewWorkspace(t, f)
		runID = w.RunID

		assert.Regexp(t, `^e2e-[0-9]+-[0-9a-f]{4}$`, w.RunID)
		assert.Equal(t, "e2e/"+w.RunID, w.Branch)
		assert.Equal(t, w.RunID+"-v", w.TagPrefix)
		assert.Equal(t, w.Branch, w.Repo.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "chore: seed", w.Repo.Git("log", "-1", "--format=%s"), "the branch starts at the sandbox's main")
		assert.Equal(t, []string{"GH_TOKEN=tok"}, w.Env())
		assert.Contains(t, w.Config("versioning:\n  strategy: semver\n", ""), "repository: acme/widget-testing")
		assert.Contains(t, w.Config("versioning:\n  strategy: semver\n", ""), "token_env: GH_TOKEN")

		// the run "created" these; another run's resources must survive
		f.tags = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9", "v1.0.0"}
		f.releases = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9"}
		f.branches = []string{w.Branch, "e2e/e2e-1-ffff", "main"}
	})

	assert.Equal(t, []string{runID + "-v0.1.0"}, f.deletedReleases)
	assert.Equal(t, []string{runID + "-v0.1.0"}, f.deletedTags)
	assert.Equal(t, []string{"e2e/" + runID}, f.deletedBranches)
	for _, deleted := range append(append(f.deletedReleases, f.deletedTags...), f.deletedBranches...) {
		assert.True(t, strings.Contains(deleted, runID), "only this run's resources are deleted: %s", deleted)
	}
}
```

(Fix the unused `localForge` embedding of `Forge` interface: the embedded nil `Forge` only satisfies the methods not overridden; do not call them.)

Run: `go test -tags e2e_forge ./e2e/forgeharness -run Workspace`
Expected: FAIL to compile, `undefined: NewWorkspace`.

- [ ] **Step 2: Implement**

Create `e2e/forgeharness/workspace.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adaouat/heraut/e2e/harness"
)

// Workspace is a clone of a sandbox repository on a throwaway branch; everything the run creates
// carries RunID and is deleted by the cleanup registered in NewWorkspace.
type Workspace struct {
	Repo      *harness.Repo
	Forge     Forge
	RunID     string
	Branch    string
	TagPrefix string
}

// Require returns the named sandbox forge, skipping when it is not configured and failing when a
// guard refuses it.
func Require(t *testing.T, name string) Forge {
	t.Helper()
	c := LoadConfig()
	var f Forge
	switch name {
	case "github":
		if c.GitHubRepo == "" {
			t.Skip("HERAUT_E2E_GITHUB_REPO is not set: skipping the GitHub sandbox scenarios")
		}
		tok, err := GitHubToken()
		if err != nil {
			t.Skip(err.Error())
		}
		f = NewGitHub(c, tok)
	case "gitlab":
		if c.GitLabProject == "" {
			t.Skip("HERAUT_E2E_GITLAB_PROJECT is not set: skipping the GitLab sandbox scenarios")
		}
		tok, err := GitLabToken()
		if err != nil {
			t.Skip(err.Error())
		}
		f = NewGitLab(c, tok)
	default:
		t.Fatalf("unknown forge %q", name)
	}
	if err := f.Check(); err != nil {
		t.Fatalf("sandbox guard: %v", err)
	}
	return f
}

// NewWorkspace clones f's default branch into a temporary repository, creates e2e/<run-id> and
// registers the cleanup.
func NewWorkspace(t *testing.T, f Forge) *Workspace {
	t.Helper()
	id := NewRunID(time.Now(), RandomHex())
	w := &Workspace{Forge: f, RunID: id, Branch: "e2e/" + id, TagPrefix: id + "-v"}

	repo := harness.NewRepo(t)
	repo.Git("remote", "add", "origin", f.CloneURL())
	repo.Git("config", "credential.helper", "")
	repo.Git("config", "http.extraheader", f.GitAuthHeader())
	repo.Git("fetch", "-q", "origin", "main")
	repo.Git("checkout", "-q", "-b", w.Branch, "origin/main")
	w.Repo = repo

	t.Cleanup(func() { w.cleanup(t) })
	return w
}

// Env is the environment a heraut run needs for this forge: its token variable and nothing else.
func (w *Workspace) Env() []string { return []string{w.Forge.TokenEnv() + "=" + w.Forge.Token()} }

// Config renders a .heraut.yml for this forge: versioningBlock is the full `versioning:` section,
// releaseExtra is spliced into the single release target (for example assets).
func (w *Workspace) Config(versioningBlock, releaseExtra string) string {
	return fmt.Sprintf(`version: "1"
%schangelog:
  output: CHANGELOG.md
forges:
  - name: %s
    platform: %s
    %s: %s
    token_env: %s
release:
  targets:
    - forge: %s
%s`, versioningBlock, w.Forge.Name(), w.Forge.Platform(), w.Forge.CoordinatesKey(), w.Forge.Coordinates(),
		w.Forge.TokenEnv(), w.Forge.Name(), releaseExtra)
}

// cleanup deletes this run's releases, tags and branch, in that order. Failures fail the test:
// a leaked resource must be visible.
func (w *Workspace) cleanup(t *testing.T) {
	t.Helper()
	f := w.Forge
	if releases, err := f.Releases(w.RunID); err != nil {
		t.Errorf("cleanup: listing releases: %v", err)
	} else {
		for _, tag := range releases {
			w.delete(t, "release", tag, f.DeleteRelease)
		}
	}
	for _, prefix := range []string{w.RunID, strings.TrimSuffix(w.TagPrefix, "-v")} {
		tags, err := f.Tags(prefix)
		if err != nil {
			t.Errorf("cleanup: listing tags: %v", err)
			continue
		}
		for _, tag := range tags {
			w.delete(t, "tag", tag, f.DeleteTag)
		}
	}
	branches, err := f.Branches(w.Branch)
	if err != nil {
		t.Errorf("cleanup: listing branches: %v", err)
		return
	}
	for _, b := range branches {
		w.delete(t, "branch", b, f.DeleteBranch)
	}
}

func (w *Workspace) delete(t *testing.T, kind, name string, del func(string) error) {
	t.Helper()
	if !strings.Contains(name, w.RunID) {
		t.Errorf("cleanup: refusing to delete %s %q: it does not carry the run id %s", kind, name, w.RunID)
		return
	}
	if err := del(name); err != nil {
		t.Errorf("cleanup: deleting %s %q: %v", kind, name, err)
	}
}
```

Run: `go test -count=1 -tags e2e_forge ./e2e/forgeharness`
Expected: PASS. (The per-env tag `"<RunID>/uat/0.1.0"` starts with the run id, so `Tags(w.RunID)` covers it; the second prefix in the loop only duplicates work and can be removed if the duplicate deletion fails: make the loop a single `f.Tags(w.RunID)` call if so.)

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/forgeharness
git commit -F - <<'EOF'
test(e2e): add the forge lane's workspace with guarded cleanup

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Scenarios B1-B4 and the `test:e2e` task

**Files:**
- Create: `e2e/forge_release_test.go`
- Modify: `.config/mise/conf.d/project.toml`

**Interfaces:**
- Consumes: Tasks 1-3; `harness.Binary`; `runOK`-free direct `Run`.
- Produces: tests `TestForge_B1_FinalRelease`, `TestForge_B2_PreRelease`, `TestForge_B3_BuildMetadata`, `TestForge_B4_PerEnvTagWithAsset`, each running once per forge.

- [ ] **Step 1: Write the scenarios**

Create `e2e/forge_release_test.go`:

```go
//go:build e2e_forge

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/forgeharness"
	"github.com/adaouat/heraut/e2e/harness"
)

var forgeNames = []string{"github", "gitlab"}

func eachForge(t *testing.T, fn func(t *testing.T, ws *forgeharness.Workspace)) {
	t.Helper()
	for _, name := range forgeNames {
		t.Run(name, func(t *testing.T) {
			ws := forgeharness.NewWorkspace(t, forgeharness.Require(t, name))
			fn(t, ws)
		})
	}
}

func commitAndPushNothing(ws *forgeharness.Workspace, cfg string, subjects ...string) {
	ws.Repo.WriteConfig(cfg)
	ws.Repo.Git("add", ".heraut.yml")
	ws.Repo.Git("commit", "-q", "-m", "chore: init e2e")
	for _, s := range subjects {
		ws.Repo.Commit(s)
	}
}

func release(t *testing.T, ws *forgeharness.Workspace, bin string, args ...string) harness.Result {
	t.Helper()
	res := ws.Repo.Run(bin, ws.Env(), append([]string{"release", "--offline"}, args...)...)
	require.Equal(t, exitOK, res.ExitCode, "heraut release %v\nstdout:\n%s\nstderr:\n%s", args, res.Stdout, res.Stderr)
	return res
}

func semverBlock(ws *forgeharness.Workspace) string {
	return "versioning:\n  strategy: semver\n  tag_prefix: \"" + ws.TagPrefix + "\"\n"
}

func TestForge_B1_FinalRelease(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitAndPushNothing(ws, ws.Config(semverBlock(ws), ""), "feat: one", "fix: two")

		release(t, ws, bin)

		tag := ws.TagPrefix + "0.1.0"
		rel, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "the release exists on the forge")
		assert.Contains(t, rel.Body, "One", "the release body carries the generated notes")
		assert.Contains(t, rel.Body, "Two")
		assert.False(t, rel.Prerelease)
		assert.False(t, rel.Draft)
		sha, err := ws.Forge.TagCommit(tag)
		require.NoError(t, err)
		assert.Equal(t, ws.Repo.Git("rev-parse", "HEAD"), sha, "the tag points at the changelog commit")
	})
}

func TestForge_B2_PreRelease(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitAndPushNothing(ws, ws.Config(semverBlock(ws), ""), "feat: one")

		release(t, ws, bin, "--pre-release", "rc")

		rel, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0-rc.1")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, ws.Forge.HasPreReleaseFlag(), rel.Prerelease,
			"GitHub marks it a pre-release; GitLab has no such flag and creates a plain release")
	})
}

func TestForge_B3_BuildMetadata(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitAndPushNothing(ws, ws.Config(semverBlock(ws), ""), "feat: one")

		res := release(t, ws, bin, "--set-version", "0.1.0", "--set-build-id", "158404")

		tag := ws.TagPrefix + "0.1.0+158404"
		_, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "a tag carrying + build metadata is accepted")
		assert.Contains(t, res.Stdout, ws.Forge.URLTag(tag), "the printed release URL spells the tag the way the forge does")
		assert.True(t, strings.Contains(res.Stdout, "/releases/") || strings.Contains(res.Stdout, "/-/releases/"))
	})
}

func TestForge_B4_PerEnvTagWithAsset(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		block := "versioning:\n  strategy: semver-per-env\n  tag_format: \"" + ws.RunID + "/{env}/{version}\"\nenvironments:\n  uat:\n    bump: auto\n"
		cfg := ws.Config(block, "      assets:\n        - \"dist/app.txt\"\n")
		ws.Repo.WriteFile("dist/app.txt", "asset\n")
		ws.Repo.Git("add", "dist/app.txt")
		commitAndPushNothing(ws, cfg, "feat: one")

		release(t, ws, bin, "--env", "uat")

		tag := ws.RunID + "/uat/0.1.0"
		rel, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "a tag with / is accepted (the T335 regression on GitLab)")
		assert.Equal(t, 1, rel.AssetCount, "the asset was attached")
	})
}
```

The first run of each test is the red phase: with the sandbox variables unset every `t.Run` skips (a green skip, which proves the gating), so the real red/green is done live in Step 2.

- [ ] **Step 2: Run live against the sandboxes**

Export the sandbox coordinates in your shell (never commit them), then:

```bash
HERAUT_E2E_GITHUB_REPO=<owner>/<name>-testing HERAUT_E2E_GITLAB_PROJECT=<group>/<name>-testing \
  go test -count=1 -tags e2e_forge -run 'TestForge_' ./e2e/ -v
```

Expected: 8 passing subtests (4 scenarios x 2 forges), no `cleanup:` errors, and afterwards the sandboxes hold no `e2e-*` branch, tag or release (verify with `gh api repos/<owner>/<name>-testing/tags` and `glab api projects/<escaped>/repository/tags`). If a scenario fails, read the failure and the forge state, fix the cause, and repeat; do not loosen an assertion. Also confirm gating: `go test -count=1 -tags e2e_forge -run TestForge_ ./e2e/` with the variables unset reports skips, and `go test -count=1 ./e2e/...` (no tag) does not list `TestForge_`.

- [ ] **Step 3: The mise task, lint, commit**

Append to `.config/mise/conf.d/project.toml`:

```toml
[tasks."test:e2e"]
description = "Run the forge-sandbox e2e scenarios (needs HERAUT_E2E_GITHUB_REPO / HERAUT_E2E_GITLAB_PROJECT)"
run = "go test -count=1 -tags e2e_forge -run 'TestForge_' ./e2e/ -v"
```

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e .config/mise/conf.d/project.toml
git commit -F - <<'EOF'
test(e2e): cover final, pre-release, build-metadata and per-env releases live

Scenarios B1-B4 run once per forge against the private sandboxes: notes
and tag target, GitHub's prerelease flag vs GitLab's plain release, a +
build-metadata tag and its URL spelling, and a / tag with an uploaded
asset (the T335 regression). Opt-in behind the e2e_forge build tag.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Sweeper

**Files:**
- Create: `e2e/cmd/sweep/main.go`
- Create: `e2e/forgeharness/sweep.go`, `e2e/forgeharness/sweep_test.go`

**Interfaces:**
- Consumes: `Forge`, `RunIDTime`.
- Produces: `Sweep(f Forge, olderThan time.Duration, now time.Time, dryRun bool) (deleted []string, err error)` and the command `go run -tags e2e_forge ./e2e/cmd/sweep [-older-than 24h] [-dry-run]`.

- [ ] **Step 1: Write the failing test**

Create `e2e/forgeharness/sweep_test.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweepDeletesOnlyOldRunResources(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	old := NewRunID(now.Add(-48*time.Hour), "aaaa")
	fresh := NewRunID(now.Add(-1*time.Hour), "bbbb")
	f := &localForge{
		tags:     []string{old + "-v0.1.0", fresh + "-v0.1.0", "v1.0.0", old + "/uat/0.1.0"},
		releases: []string{old + "-v0.1.0", fresh + "-v0.1.0"},
		branches: []string{"e2e/" + old, "e2e/" + fresh, "main"},
	}

	deleted, err := Sweep(f, 24*time.Hour, now, false)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{old + "-v0.1.0", old + "/uat/0.1.0"}, f.deletedTags)
	assert.Equal(t, []string{old + "-v0.1.0"}, f.deletedReleases)
	assert.Equal(t, []string{"e2e/" + old}, f.deletedBranches)
	assert.Len(t, deleted, 4)

	f2 := &localForge{tags: []string{old + "-v0.1.0"}}
	deleted, err = Sweep(f2, 24*time.Hour, now, true)
	require.NoError(t, err)
	assert.Empty(t, f2.deletedTags, "a dry run deletes nothing")
	assert.Equal(t, []string{old + "-v0.1.0"}, deleted, "but reports what it would delete")
}
```

Run: `go test -tags e2e_forge ./e2e/forgeharness -run Sweep`
Expected: FAIL to compile, `undefined: Sweep`.

- [ ] **Step 2: Implement**

Create `e2e/forgeharness/sweep.go`:

```go
//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"time"
)

// Sweep deletes releases, tags and branches that carry a run id older than olderThan. With dryRun
// it only reports. Resources without a run id are never touched.
func Sweep(f Forge, olderThan time.Duration, now time.Time, dryRun bool) ([]string, error) {
	var deleted []string
	old := func(name string) bool {
		ts, ok := RunIDTime(name)
		return ok && now.Sub(ts) > olderThan
	}
	step := func(kind string, list func(string) ([]string, error), prefix string, del func(string) error) error {
		names, err := list(prefix)
		if err != nil {
			return fmt.Errorf("listing %ss: %w", kind, err)
		}
		for _, n := range names {
			if !old(n) {
				continue
			}
			deleted = append(deleted, n)
			if dryRun {
				continue
			}
			if err := del(n); err != nil {
				return fmt.Errorf("deleting %s %q: %w", kind, n, err)
			}
		}
		return nil
	}
	if err := step("release", f.Releases, "e2e-", f.DeleteRelease); err != nil {
		return deleted, err
	}
	if err := step("tag", f.Tags, "e2e-", f.DeleteTag); err != nil {
		return deleted, err
	}
	if err := step("branch", f.Branches, "e2e/", f.DeleteBranch); err != nil {
		return deleted, err
	}
	return deleted, nil
}
```

Create `e2e/cmd/sweep/main.go`:

```go
//go:build e2e_forge

// Command sweep deletes stale e2e-* branches, tags and releases left in the forge sandboxes by
// crashed runs. Usage: go run -tags e2e_forge ./e2e/cmd/sweep [-older-than 24h] [-dry-run]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/adaouat/heraut/e2e/forgeharness"
)

func main() {
	olderThan := flag.Duration("older-than", 24*time.Hour, "delete run resources older than this")
	dryRun := flag.Bool("dry-run", false, "only report what would be deleted")
	flag.Parse()

	c := forgeharness.LoadConfig()
	var forges []forgeharness.Forge
	if c.GitHubRepo != "" {
		tok, err := forgeharness.GitHubToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitHub(c, tok))
	}
	if c.GitLabProject != "" {
		tok, err := forgeharness.GitLabToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitLab(c, tok))
	}
	if len(forges) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to sweep: set HERAUT_E2E_GITHUB_REPO and/or HERAUT_E2E_GITLAB_PROJECT")
		return
	}
	for _, f := range forges {
		exitOn(f.Check())
		deleted, err := forgeharness.Sweep(f, *olderThan, time.Now(), *dryRun)
		for _, d := range deleted {
			fmt.Printf("%s: %s\n", f.Name(), d)
		}
		exitOn(err)
	}
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "sweep:", err)
		os.Exit(1)
	}
}
```

Run: `go test -count=1 -tags e2e_forge ./e2e/forgeharness && go vet -tags e2e_forge ./e2e/...`
Expected: PASS and clean. Then run the sweeper once live in dry-run mode to confirm it lists nothing on clean sandboxes: `HERAUT_E2E_GITHUB_REPO=… HERAUT_E2E_GITLAB_PROJECT=… go run -tags e2e_forge ./e2e/cmd/sweep -dry-run`.

- [ ] **Step 3: Lint and commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): add a sweeper for stale forge sandbox resources

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Close T345c

**Files:**
- Modify: `docs/tasks/roadmap.md`

- [ ] **Step 1: Flip and annotate**

Change the `#### \`[ ]\` T345c:` heading to `[x]` and add a completion note: the packages and files added, the guard list, that scenarios ran live against both private sandboxes (8 passing subtests) and left them clean, the lint `build-tags` change, the `mise run test:e2e` task, and any deviation (for example the GitLab 404-detection or an ordered fake). Use placeholders for coordinates (`<owner>/<name>-testing`). Update the Phase 60 status row. `grep -n 'T345c' docs/tasks/roadmap.md` must show the `[x]` heading and no placeholder text; `grep -rn` for the real sandbox names in `docs/` and `e2e/` must print nothing.

- [ ] **Step 2: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): close T345c

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane B: harness, configuration variables, safety guards, per-run branch and tag namespace, cleanup plus sweeper, B1-B4 on both forges): Tasks 1-5; B5-B10, the workflow and the guide are T345d.

**Type consistency:** `Forge`, `Release`, `Workspace`, `Config`, `Sweep` are defined in Tasks 1-3/5 and used unchanged in 4-5; `localForge` (Task 3 test file) is reused by Task 5's test, which is why both live in package `forgeharness`.

**Placeholders:** the coordinates are `<owner>/<name>-testing` by design; the Task 6 note is written from the live run.
