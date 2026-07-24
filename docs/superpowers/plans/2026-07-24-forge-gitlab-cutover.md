# Forge Abstraction — Plan B: GitLab forge + enrichment cutover (T158–T160)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `heraut changelog` / `heraut release` enrich GitLab commits with **zero config** in GitLab CI using the built-in `CI_JOB_TOKEN` — no manually-created PAT — by implementing a native `net/http` GitLab forge (REST default, GraphQL opt-in) and wiring it into the native generator's enrichment path.

**Architecture:** `internal/forge/gitlab` implements `port.Forge` over stdlib `net/http`. REST (default) resolves MR refs per commit via the job-token-allowed `…/repository/commits/{sha}/merge_requests` endpoint and renders `by @` from the local git author name; GraphQL (`api_mode: graphql`) reuses ADR-0042's batched queries for linked `@username` handles. The app layer resolves a `port.ForgeIdentity` (Plan A's `forge.Resolve`), constructs the GitLab forge for the enrichment forge, and injects it into `native.Generator`; the generator prefers the injected forge over its legacy `glab`-based path. The `LinkContext` used for rendering links is also derived from the resolved forge, so links work zero-config too.

**Tech Stack:** Go, stdlib `net/http` + `encoding/json`, `testify`, `httptest`. **No new dependencies** (the publishing-client library question is a P3 ADR — see the design spec's Phasing).

## Scope decision (READ THIS FIRST)

Roadmap T160 originally said "remove the now-dead `changelog.remote` **and** `release.platforms`". `release.platforms` is consumed by **17 non-test files including `internal/scaffold/` (the `heraut init` wizard)**, whose rewrite is deliberately deferred to **P4, last, after the new config is battle-tested**. Removing it here would force that rewrite now.

**Therefore this plan is enrichment-first (user decision, 2026-07-24):**

| In scope (Plan B) | Deferred |
|---|---|
| GitLab forge (REST + GraphQL) | Publishing via `release.targets` → **P3** |
| Forge wired into **enrichment** (changelog + release notes) | Removing `release.platforms` → **P3** |
| `LinkContext` derived from the resolved forge | `heraut init` wizard → **P4** |
| **Remove `changelog.remote`** (forge fully replaces it) | GitHub/Azure onto `port.Forge` → **P2** |
| **Rename** `commits.remote_metadata` → `commits.enrichment_policy` + **migration error** | |

`release.targets` stays parsed-and-validated but unused until P3 — that is expected, not dead code to "clean up".

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven tests preferred.
- **No new Go dependencies.** `internal/forge` imports only `internal/{port,config}` + stdlib. `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib.
- **No real data** — synthetic placeholders only (`gitlab.example.com`, `group/subgroup/project`, `alice`, `alice@example.com`). Never a real host, org, username, or token.
- **No network in tests** — `httptest.Server` only.
- Auth header by `TokenKind`: **`port.TokenJob` → `JOB-TOKEN`**, **`port.TokenPrivate` → `PRIVATE-TOKEN`**. GitLab rejects a job token sent as `PRIVATE-TOKEN`, and rejects job tokens on GraphQL entirely — this mapping is the whole point of the epic.
- Config field changes sync three surfaces in lockstep: `internal/config/`, `schema.json`, `docs/heraut.sample.yml`.
- **Do not remove `release.platforms`** (see Scope decision). Do not touch `internal/scaffold/`.
- Errors wrapped with `%w`; sentinel errors compared with `errors.Is`.
- Never bypass git hooks (no `--no-verify`). Fix lint via `hk fix` (never `gofmt`/`yamlfmt` directly).
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — with angle brackets around the model id, and **never** a `Claude-Session:` line.

## File Structure

- `internal/forge/gitlab/gitlab.go` — **new**: `Forge` struct + constructor + `Type`/`Identity`/link methods; the `port.Forge` implementation.
- `internal/forge/gitlab/rest.go` — **new**: REST enrichment (per-commit MR lookup) + JSON payload types.
- `internal/forge/gitlab/graphql.go` — **new**: GraphQL enrichment (batched, opt-in) + the job-token guard.
- `internal/generators/native/enrich.go` — **modify**: prefer an injected `port.Forge` over the legacy dispatch.
- `internal/generators/native/generator.go` — **modify**: `Generator.forge` field + `New` option.
- `internal/app/pipeline.go` — **modify**: resolve the forge, construct the GitLab forge, inject it.
- `internal/pipeline/linkctx.go` — **modify**: derive `LinkContext` from the resolved forge; drop `remoteLinkContext`.
- `internal/config/` + `schema.json` + `docs/heraut.sample.yml` — **modify**: remove `changelog.remote`, rename the policy key, add the migration error.

Each of the four tasks below ends with an independently testable, green deliverable. **Tasks 3 and 4 together complete roadmap T160** (wiring, then the breaking config migration) — flip T160 `[ ]`→`[x]` only after Task 4.

---

### Task 1 (T158): GitLab REST forge

**Files:**
- Create: `internal/forge/gitlab/gitlab.go`
- Create: `internal/forge/gitlab/rest.go`
- Test: `internal/forge/gitlab/gitlab_test.go`

**Interfaces:**
- Consumes: `port.Forge`, `port.ForgeIdentity{Type,Host,APIURL,Project,Token,TokenKind,APIMode}`, `port.TokenKind` (`TokenNone|TokenJob|TokenPrivate`), `port.Commit{Hash,Author,Email,Date}`, `port.Enrichment{PRs map[string]PullRequest; Authors map[string]string}`, `port.PullRequest{Number,URL,Title,AuthorLogin,Labels,RefPrefix,CreatedAt,MergedAt,MergedBy,Approvers}`, `port.Author{Username}`.
- Produces (used by Tasks 2 and 3):
  - `func New(id port.ForgeIdentity, client *http.Client) *Forge` — `client` nil ⇒ a 30s-timeout default.
  - `(*Forge)` implements `port.Forge`.
  - `func (f *Forge) apiBase() string` — `id.APIURL`, else `id.Host + "/api/v4"`.
  - `func (f *Forge) setAuth(req *http.Request)` — `JOB-TOKEN` for `TokenJob`, `PRIVATE-TOKEN` for `TokenPrivate`, nothing for `TokenNone`.
  - `func gitAuthors(commits []port.Commit) map[string]string` — sha → git author name (`Author`, else email local-part).

- [ ] **Step 1: Write the failing test** — `internal/forge/gitlab/gitlab_test.go`

```go
package gitlab_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project",
	}, nil)
	assert.Equal(t, "gitlab", f.Type())
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/merge_requests/42", f.ChangeURL(42))
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/compare/v1.0.0...v1.1.0", f.CompareURL("v1.0.0", "v1.1.0"))
}

// TestEnrichREST_JobToken is the epic's core scenario: a CI_JOB_TOKEN must be sent as JOB-TOKEN
// (GitLab rejects it as PRIVATE-TOKEN) against the job-token-allowed per-commit MR endpoint.
func TestEnrichREST_JobToken(t *testing.T) {
	var gotPath, gotJob, gotPrivate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotJob, gotPrivate = r.URL.Path, r.Header.Get("JOB-TOKEN"), r.Header.Get("PRIVATE-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"iid": 42, "web_url": "https://gitlab.example.com/group/project/-/merge_requests/42",
			"title": "Add widget", "labels": ["feature"],
			"author": {"username": "alice"},
			"created_at": "2026-07-01T10:00:00Z", "merged_at": "2026-07-02T11:00:00Z",
			"merged_by": {"username": "bob"}
		}]`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "jobtok", TokenKind: port.TokenJob, APIMode: "rest",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	require.NoError(t, err)

	assert.Equal(t, "/api/v4/projects/group%2Fproject/repository/commits/abc123/merge_requests", gotPath)
	assert.Equal(t, "jobtok", gotJob, "a job token must be sent as JOB-TOKEN")
	assert.Empty(t, gotPrivate, "a job token must NOT be sent as PRIVATE-TOKEN")

	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, "alice", pr.AuthorLogin)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	assert.False(t, pr.MergedAt.IsZero())
	// REST commits carry no linked handle: `by @` falls back to the local git author name.
	assert.Equal(t, "Alice", en.Authors["abc123"])
}

func TestEnrichREST_PrivateTokenHeader(t *testing.T) {
	var gotJob, gotPrivate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJob, gotPrivate = r.Header.Get("JOB-TOKEN"), r.Header.Get("PRIVATE-TOKEN")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate,
	}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
	require.NoError(t, err)
	assert.Equal(t, "pat", gotPrivate, "a PAT must be sent as PRIVATE-TOKEN")
	assert.Empty(t, gotJob)
}

func TestEnrichREST_NoMR_And_ErrorStatus(t *testing.T) {
	t.Run("no MR for commit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[]`))
		}))
		defer srv.Close()
		f := gitlab.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p"}, srv.Client())
		en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
		require.NoError(t, err)
		assert.Empty(t, en.PRs, "a commit with no MR yields no PR entry")
		assert.Equal(t, "Alice", en.Authors["abc123"], "author handle still resolves offline-style")
	})

	t.Run("non-2xx is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := gitlab.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

func TestEnrich_NoCommits(t *testing.T) {
	f := gitlab.New(port.ForgeIdentity{Host: "https://gitlab.example.com", Project: "g/p"}, nil)
	en, err := f.Enrich(nil)
	require.NoError(t, err)
	assert.Empty(t, en.PRs)
	assert.Empty(t, en.Authors)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/forge/gitlab/`
Expected: compile failure — no such package / `undefined: gitlab.New`.

- [ ] **Step 3: Implement `internal/forge/gitlab/gitlab.go`**

```go
// Package gitlab implements port.Forge for GitLab over stdlib net/http. REST is the default
// transport because GitLab's GraphQL API rejects CI job tokens outright; see ADR-0043.
package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// Forge is the GitLab implementation of port.Forge.
type Forge struct {
	id     port.ForgeIdentity
	client *http.Client
}

var _ port.Forge = (*Forge)(nil)

// New constructs a GitLab forge. A nil client gets a default with a 30s timeout, matching the
// Azure enrichment client (ADR-0035).
func New(id port.ForgeIdentity, client *http.Client) *Forge {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Forge{id: id, client: client}
}

func (f *Forge) Type() string                  { return "gitlab" }
func (f *Forge) Identity() port.ForgeIdentity  { return f.id }

// webBase is the project's web root, e.g. https://gitlab.example.com/group/subgroup/project.
func (f *Forge) webBase() string {
	return strings.TrimRight(f.id.Host, "/") + "/" + f.id.Project
}

func (f *Forge) CommitURL(sha string) string { return f.webBase() + "/-/commit/" + sha }
func (f *Forge) ChangeURL(number int) string {
	return fmt.Sprintf("%s/-/merge_requests/%d", f.webBase(), number)
}
func (f *Forge) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/-/compare/%s...%s", f.webBase(), from, to)
}

// apiBase returns the REST/GraphQL API root: the explicit APIURL when set (GitLab CI provides it
// as CI_API_V4_URL), else the conventional {host}/api/v4.
func (f *Forge) apiBase() string {
	if f.id.APIURL != "" {
		return strings.TrimRight(f.id.APIURL, "/")
	}
	return strings.TrimRight(f.id.Host, "/") + "/api/v4"
}

// setAuth applies the auth header matching the token's kind. GitLab requires JOB-TOKEN for a CI
// job token and PRIVATE-TOKEN for a personal/project access token; sending a job token as
// PRIVATE-TOKEN is rejected.
func (f *Forge) setAuth(req *http.Request) {
	switch f.id.TokenKind {
	case port.TokenJob:
		req.Header.Set("JOB-TOKEN", f.id.Token)
	case port.TokenPrivate:
		req.Header.Set("PRIVATE-TOKEN", f.id.Token)
	case port.TokenNone:
	}
}

// Enrich resolves per-commit MR references and author handles. api_mode: graphql opts into the
// batched GraphQL transport (linked @usernames, requires a non-job token); the default is REST.
func (f *Forge) Enrich(commits []port.Commit) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	if f.id.APIMode == "graphql" {
		return f.enrichGraphQL(commits)
	}
	return f.enrichREST(commits)
}

// gitAuthors maps sha → the local git author name, falling back to the email local-part. REST
// commit payloads expose no linked GitLab username, so this is the only `by @` source in REST
// mode (the same trade-off Azure makes; see ADR-0043).
func gitAuthors(commits []port.Commit) map[string]string {
	authors := make(map[string]string, len(commits))
	for _, c := range commits {
		if c.Author != "" {
			authors[c.Hash] = c.Author
			continue
		}
		if local, _, ok := strings.Cut(c.Email, "@"); ok && local != "" {
			authors[c.Hash] = local
		}
	}
	return authors
}
```

- [ ] **Step 4: Implement `internal/forge/gitlab/rest.go`**

```go
package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// restMR is one merge request as returned by the REST API. Unlike GraphQL, iid is a number and
// labels is a flat string array.
type restMR struct {
	IID      int      `json:"iid"`
	WebURL   string   `json:"web_url"`
	Title    string   `json:"title"`
	Labels   []string `json:"labels"`
	Author   restUser `json:"author"`
	MergedBy restUser `json:"merged_by"`

	CreatedAt time.Time `json:"created_at"`
	MergedAt  time.Time `json:"merged_at"`
}

type restUser struct {
	Username string `json:"username"`
}

// enrichREST resolves each commit's MR via GET /projects/{id}/repository/commits/{sha}/merge_requests
// — one of the endpoints GitLab allows a CI job token to call. Author handles come from the local
// git metadata (REST exposes no linked username).
func (f *Forge) enrichREST(commits []port.Commit) (port.Enrichment, error) {
	prs := make(map[string]port.PullRequest, len(commits))
	for _, c := range commits {
		mrs, err := f.commitMRs(c.Hash)
		if err != nil {
			return port.Enrichment{}, err
		}
		if len(mrs) == 0 {
			continue
		}
		prs[c.Hash] = mrToPullRequest(mrs[0]) // first association wins, matching the other drivers
	}
	return port.Enrichment{PRs: prs, Authors: gitAuthors(commits)}, nil
}

// commitMRs fetches the merge requests that introduced one commit.
func (f *Forge) commitMRs(sha string) ([]restMR, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/repository/commits/%s/merge_requests",
		f.apiBase(), url.PathEscape(f.id.Project), url.PathEscape(sha))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	f.setAuth(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab commit merge_requests: unexpected status %s", resp.Status)
	}

	var mrs []restMR
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: decoding response: %w", err)
	}
	return mrs, nil
}

// mrToPullRequest normalizes a REST merge request into the port model. RefPrefix is "!" — GitLab
// renders merge requests as !N.
func mrToPullRequest(m restMR) port.PullRequest {
	return port.PullRequest{
		Number:      m.IID,
		URL:         m.WebURL,
		Title:       m.Title,
		AuthorLogin: m.Author.Username,
		Labels:      m.Labels,
		RefPrefix:   "!",
		CreatedAt:   m.CreatedAt,
		MergedAt:    m.MergedAt,
		MergedBy:    port.Author{Username: m.MergedBy.Username},
	}
}
```

> Note: `url.PathEscape("group/project")` yields `group%2Fproject`, which is exactly the URL-encoded project id GitLab expects — and what the test asserts.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/forge/gitlab/`
Expected: PASS (all five tests).

- [ ] **Step 6: Full suite + lint + commit**

```bash
go test ./... && hk fix
git add internal/forge/gitlab/
git commit -m "feat(forge/gitlab): REST enrichment over native net/http (T158)"
```

---

### Task 2 (T159): GitLab GraphQL mode (opt-in) + job-token guard

**Files:**
- Create: `internal/forge/gitlab/graphql.go`
- Test: `internal/forge/gitlab/graphql_test.go`

**Interfaces:**
- Consumes: `(*Forge).apiBase()`, `(*Forge).setAuth(req)`, `gitAuthors(commits)`, `mrToPullRequest` is **not** reused here (GraphQL has its own shape).
- Produces: `func (f *Forge) enrichGraphQL(commits []port.Commit) (port.Enrichment, error)` — called by `Enrich` when `APIMode == "graphql"` (already wired in Task 1); `var ErrJobTokenGraphQL = errors.New(...)`.

**Why the guard lives here:** GitLab's GraphQL API rejects CI job tokens entirely. Static config validation (T156) can't see the *resolved* token kind, so the check belongs at the transport, using `port.ForgeIdentity.TokenKind` — and it must fail **before** any network call.

- [ ] **Step 1: Write the failing test** — `internal/forge/gitlab/graphql_test.go`

```go
package gitlab_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A job token can never authenticate GraphQL — fail fast, with no request issued.
func TestEnrichGraphQL_JobTokenRejected(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "jobtok", TokenKind: port.TokenJob, APIMode: "graphql",
	}, srv.Client())

	_, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, gitlab.ErrJobTokenGraphQL))
	assert.Contains(t, err.Error(), "api_mode: rest", "the error must point at the fix")
	assert.False(t, called, "no network call may be made when the guard trips")
}

func TestEnrichGraphQL_LinkedUsernameAndHeader(t *testing.T) {
	var gotPrivate, gotJob string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrivate, gotJob = r.Header.Get("PRIVATE-TOKEN"), r.Header.Get("JOB-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[{"sha":"abc123","author":{"username":"alice-gl"}}]}},
			"mergeRequests":{"nodes":[{
				"iid":"42","webUrl":"https://gitlab.example.com/group/project/-/merge_requests/42",
				"title":"Add widget","author":{"username":"alice-gl"},
				"createdAt":"2026-07-01T10:00:00Z","mergedAt":"2026-07-02T11:00:00Z",
				"mergeUser":{"username":"bob-gl"},
				"labels":{"nodes":[{"title":"feature"}]},
				"mergeCommitSha":"abc123","commits":{"nodes":[]}
			}]}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com"}})
	require.NoError(t, err)

	assert.Equal(t, "pat", gotPrivate, "GraphQL uses PRIVATE-TOKEN")
	assert.Empty(t, gotJob)
	// GraphQL's whole advantage: the LINKED handle, not the local git name.
	assert.Equal(t, "alice-gl", en.Authors["abc123"])
	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob-gl", pr.MergedBy.Username)
}

func TestEnrichGraphQL_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"insufficient scope"}]}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/forge/gitlab/ -run GraphQL`
Expected: FAIL — `undefined: gitlab.ErrJobTokenGraphQL` / `enrichGraphQL` undefined.

- [ ] **Step 3: Implement `internal/forge/gitlab/graphql.go`**

The GraphQL endpoint is the API root with `/graphql` replacing `/api/v4` — derive it as `strings.TrimSuffix(f.apiBase(), "/api/v4") + "/api/graphql"`. Scalars are per ADR-0042's spike: **`iid` is a String**, the merger is **`mergeUser`** (not `mergedBy`), labels are `labels{nodes{title}}`, and there is **no `squashCommitSha`** field.

```go
package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// ErrJobTokenGraphQL reports the one combination GitLab forbids: a CI job token on GraphQL.
var ErrJobTokenGraphQL = errors.New("gitlab graphql: CI job tokens cannot authenticate GraphQL")

// gqlQuery is one batched query fetching commit-author handles and merged MRs for the project.
// mergeCommitSha and commits.nodes.sha are the SHAs by which an MR can match a target-branch
// commit (ADR-0042); GraphQL exposes no squashed-commit SHA.
const gqlQuery = `query($path:ID!,$ref:String!){project(fullPath:$path){` +
	`repository{commits(ref:$ref,first:100){nodes{sha author{username}}}}` +
	`mergeRequests(state:merged,first:100){nodes{iid webUrl title author{username}` +
	`createdAt mergedAt mergeUser{username}labels{nodes{title}}mergeCommitSha commits{nodes{sha}}}}` +
	`}}`

type gqlResponse struct {
	Data struct {
		Project struct {
			Repository struct {
				Commits struct {
					Nodes []struct {
						SHA    string `json:"sha"`
						Author *struct {
							Username string `json:"username"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"repository"`
			MergeRequests struct {
				Nodes []gqlMR `json:"nodes"`
			} `json:"mergeRequests"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// gqlMR mirrors GitLab's GraphQL scalars: iid is a String and the merger is mergeUser.
type gqlMR struct {
	IID       string `json:"iid"`
	WebURL    string `json:"webUrl"`
	Title     string `json:"title"`
	Author    struct{ Username string } `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	MergedAt  time.Time `json:"mergedAt"`
	MergeUser struct{ Username string } `json:"mergeUser"`
	Labels    struct {
		Nodes []struct {
			Title string `json:"title"`
		} `json:"nodes"`
	} `json:"labels"`
	MergeCommitSHA string `json:"mergeCommitSha"`
	Commits        struct {
		Nodes []struct {
			SHA string `json:"sha"`
		} `json:"nodes"`
	} `json:"commits"`
}

// enrichGraphQL resolves linked commit-author handles and MR refs in one batched query. It
// requires a personal/project access token: GitLab rejects job tokens on GraphQL outright, so the
// guard trips before any request is issued.
func (f *Forge) enrichGraphQL(commits []port.Commit) (port.Enrichment, error) {
	if f.id.TokenKind == port.TokenJob {
		return port.Enrichment{}, fmt.Errorf(
			"%w; set api_mode: rest, or supply a read_api token via token_env: %w",
			ErrJobTokenGraphQL, ErrJobTokenGraphQL)
	}

	want := make(map[string]bool, len(commits))
	for _, c := range commits {
		want[c.Hash] = true
	}

	resp, err := f.postGraphQL(newestHash(commits))
	if err != nil {
		return port.Enrichment{}, err
	}

	authors := make(map[string]string)
	for _, n := range resp.Data.Project.Repository.Commits.Nodes {
		if want[n.SHA] && n.Author != nil && n.Author.Username != "" {
			authors[n.SHA] = n.Author.Username
		}
	}

	prs := make(map[string]port.PullRequest)
	for _, n := range resp.Data.Project.MergeRequests.Nodes {
		pr := gqlMRToPullRequest(n)
		for _, sha := range landingSHAs(n) {
			if want[sha] {
				if _, seen := prs[sha]; !seen {
					prs[sha] = pr
				}
			}
		}
	}
	return port.Enrichment{PRs: prs, Authors: authors}, nil
}

// postGraphQL issues the batched query against the instance's /api/graphql endpoint.
func (f *Forge) postGraphQL(ref string) (*gqlResponse, error) {
	endpoint := strings.TrimSuffix(f.apiBase(), "/api/v4") + "/api/graphql"
	body, err := json.Marshal(map[string]any{
		"query":     gqlQuery,
		"variables": map[string]string{"path": f.id.Project, "ref": ref},
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: encoding request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	f.setAuth(req)

	httpResp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab graphql: unexpected status %s", httpResp.Status)
	}

	var out gqlResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gitlab graphql: decoding response: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("gitlab graphql: %s", out.Errors[0].Message)
	}
	return &out, nil
}

// gqlMRToPullRequest normalizes a GraphQL merge request; iid is a String scalar, so it is parsed
// (an unparsable value yields 0).
func gqlMRToPullRequest(n gqlMR) port.PullRequest {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, l := range n.Labels.Nodes {
		labels = append(labels, l.Title)
	}
	if len(labels) == 0 {
		labels = nil
	}
	num, _ := strconv.Atoi(n.IID)
	return port.PullRequest{
		Number: num, URL: n.WebURL, Title: n.Title, AuthorLogin: n.Author.Username,
		Labels: labels, RefPrefix: "!", CreatedAt: n.CreatedAt, MergedAt: n.MergedAt,
		MergedBy: port.Author{Username: n.MergeUser.Username},
	}
}

// landingSHAs are the SHAs by which an MR can match a target-branch commit: the merge commit and
// each source commit (fast-forward merges land those directly).
func landingSHAs(n gqlMR) []string {
	shas := make([]string, 0, len(n.Commits.Nodes)+1)
	if n.MergeCommitSHA != "" {
		shas = append(shas, n.MergeCommitSHA)
	}
	for _, c := range n.Commits.Nodes {
		shas = append(shas, c.SHA)
	}
	return shas
}

// newestHash returns the hash of the newest-dated commit — the commits(ref:) anchor.
func newestHash(commits []port.Commit) string {
	var newest port.Commit
	for _, c := range commits {
		if newest.Date.IsZero() || c.Date.After(newest.Date) {
			newest = c
		}
	}
	return newest.Hash
}
```

> The doubled `%w` in the guard is intentional only if both verbs wrap the same sentinel; simplify to a single `%w` plus plain text if `go vet` objects — the test asserts `errors.Is(err, ErrJobTokenGraphQL)` and that the message contains `api_mode: rest`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/forge/gitlab/`
Expected: PASS (all tests from Tasks 1 and 2).

- [ ] **Step 5: Full suite + lint + commit**

```bash
go test ./... && hk fix
git add internal/forge/gitlab/
git commit -m "feat(forge/gitlab): opt-in GraphQL mode + job-token guard (T159)"
```

---

### Task 3 (T160a): wire the forge into native enrichment

**Files:**
- Modify: `internal/generators/native/generator.go` (add the `forge` field + option)
- Modify: `internal/generators/native/enrich.go` (prefer the injected forge)
- Modify: `internal/app/pipeline.go` (resolve + construct + inject)
- Modify: `internal/pipeline/linkctx.go` (derive `LinkContext` from the resolved forge)
- Test: `internal/generators/native/enrich_forge_internal_test.go` (new)

**Interfaces:**
- Consumes: `gitlab.New(id port.ForgeIdentity, client *http.Client) *gitlab.Forge` (Task 1); `forge.Resolve(cfg *config.Config, getenv func(string) string, gitOrigin string) (forge.Resolved, error)` with `Resolved{Forges []port.ForgeIdentity; EnrichmentIndex int}` and `forge.ErrAmbiguousForge` (Plan A).
- Produces: `func WithForge(f port.Forge) Option` + `native.New(runner, cfg, mode, opts ...Option)`; `func linkContextFromIdentity(id port.ForgeIdentity) *port.LinkContext`.

**Design note:** `port.Generator.Generate(tag, lc)` stays unchanged — the forge is injected at construction, not threaded through the interface, so `gitcliff`/`communique` are untouched. The generator's existing `enrichForRelease` policy wrapper (degrade/required/`--offline`) is reused as-is: only the inner `enrich` dispatch changes.

- [ ] **Step 1: Write the failing test** — `internal/generators/native/enrich_forge_internal_test.go`

```go
package native

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubForge records the commits it was handed and returns canned enrichment.
type stubForge struct {
	got []port.Commit
	en  port.Enrichment
	err error
}

func (s *stubForge) Type() string                 { return "gitlab" }
func (s *stubForge) Identity() port.ForgeIdentity { return port.ForgeIdentity{Type: "gitlab"} }
func (s *stubForge) CommitURL(sha string) string  { return "https://gitlab.example.com/g/p/-/commit/" + sha }
func (s *stubForge) ChangeURL(int) string         { return "" }
func (s *stubForge) CompareURL(string, string) string { return "" }
func (s *stubForge) Enrich(c []port.Commit) (port.Enrichment, error) {
	s.got = c
	return s.en, s.err
}

func TestEnrich_PrefersInjectedForge(t *testing.T) {
	sf := &stubForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 42, RefPrefix: "!"}},
		Authors: map[string]string{"abc": "alice"},
	}}
	g := New(nil, testDriver(), ModeChangelog, WithForge(sf))

	er, err := g.enrich(&port.LinkContext{Platform: "gitlab"},
		[]rawCommit{{Hash: "abc", Author: "Alice", Email: "alice@example.com"}})
	require.NoError(t, err)

	require.Len(t, sf.got, 1, "the forge receives the collected commits")
	assert.Equal(t, "abc", sf.got[0].Hash)
	assert.Equal(t, "Alice", sf.got[0].Author)
	assert.Equal(t, "alice", er.authors["abc"])
	assert.Equal(t, 42, er.prs["abc"].Number)
	assert.Equal(t, "!", er.prs["abc"].RefPrefix)
}

func TestEnrich_ForgeErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	g := New(nil, testDriver(), ModeChangelog, WithForge(&stubForge{err: sentinel}))
	_, err := g.enrich(&port.LinkContext{Platform: "gitlab"}, []rawCommit{{Hash: "abc"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel))
}

// Without an injected forge, the legacy per-platform dispatch is unchanged.
func TestEnrich_NoForgeFallsBackToLegacy(t *testing.T) {
	g := New(nil, testDriver(), ModeChangelog)
	er, err := g.enrich(nil, []rawCommit{{Hash: "abc"}})
	require.NoError(t, err)
	assert.Empty(t, er.prs)
}
```

> `testDriver()` is a helper for a minimal `*config.ContentDriver`. If the package's existing internal tests already define one, reuse it; otherwise add `func testDriver() *config.ContentDriver { return &config.ContentDriver{Generator: "native"} }` to this new file.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'TestEnrich_(PrefersInjectedForge|ForgeErrorPropagates|NoForgeFallsBackToLegacy)'`
Expected: FAIL — `undefined: WithForge`.

- [ ] **Step 3: Add the forge field + option** — `internal/generators/native/generator.go`

Add to the `Generator` struct (after `httpClient`):

```go
	forge          port.Forge
```

Add after the `New` function:

```go
// Option customizes a Generator at construction.
type Option func(*Generator)

// WithForge injects the resolved enrichment forge (ADR-0043). When set, it supersedes the legacy
// per-platform CLI dispatch for fetching PR/MR metadata.
func WithForge(f port.Forge) Option {
	return func(g *Generator) { g.forge = f }
}
```

Change `New`'s signature to accept options (the variadic keeps every existing call site compiling):

```go
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode, opts ...Option) *Generator {
	g := &Generator{
		runner:     runner,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
		mode:       mode,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}
```

- [ ] **Step 4: Prefer the forge in `enrich`** — `internal/generators/native/enrich.go`

Insert at the top of `enrich`, **before** the `if lc == nil` guard (an injected forge carries its own identity and does not need a LinkContext):

```go
	if g.forge != nil {
		pc := make([]port.Commit, 0, len(commits))
		for _, c := range commits {
			pc = append(pc, port.Commit{Hash: c.Hash, Author: c.Author, Email: c.Email, Date: c.Date})
		}
		en, err := g.forge.Enrich(pc)
		if err != nil {
			return enrichResult{}, err
		}
		return enrichResult{prs: fromPortPRs(en.PRs), authors: en.Authors}, nil
	}
```

Add the boundary converter at the end of the file. **It must preserve every field the native model carries** — the Plan A review flagged that `port.PullRequest` has no `Platforms` and `port.Author` has no `Name`/`Email`, so those stay zero here by design:

```go
// fromPortPRs converts port.PullRequest values into the native render model. Platforms stays nil
// (the port model carries no per-platform bag) and Author.Name/Email stay empty — native's
// contributors tier fills those from local git, not from remote enrichment.
func fromPortPRs(in map[string]port.PullRequest) map[string]PullRequest {
	out := make(map[string]PullRequest, len(in))
	for sha, p := range in {
		approvers := make([]Author, 0, len(p.Approvers))
		for _, a := range p.Approvers {
			approvers = append(approvers, Author{Username: a.Username})
		}
		if len(approvers) == 0 {
			approvers = nil
		}
		out[sha] = PullRequest{
			Number: p.Number, URL: p.URL, AuthorLogin: p.AuthorLogin, RefPrefix: p.RefPrefix,
			Title: p.Title, Labels: p.Labels, CreatedAt: p.CreatedAt, MergedAt: p.MergedAt,
			MergedBy:  Author{Username: p.MergedBy.Username},
			Approvers: approvers,
		}
	}
	return out
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/generators/native/`
Expected: PASS.

- [ ] **Step 6: Derive the LinkContext from the resolved forge** — `internal/pipeline/linkctx.go`

Add (keep `ambientLinkContext` and the platform fallbacks as they are):

```go
// linkContextFromIdentity converts a resolved forge identity into the link context used to render
// commit/MR links, so links resolve from the same source as enrichment (ADR-0043).
func linkContextFromIdentity(id port.ForgeIdentity) *port.LinkContext {
	if id.Type == "" || id.Host == "" {
		return nil
	}
	owner, repo := splitProjectPath(id.Project)
	return &port.LinkContext{
		BaseURL:  strings.TrimRight(id.Host, "/"),
		Owner:    owner,
		Repo:     repo,
		Platform: id.Type,
		Token:    id.Token,
	}
}
```

Then, in `(*Pipeline).changelogLinkContext`, prefer a forge-derived context ahead of the ambient one. The pipeline gains a `ForgeIdentity *port.ForgeIdentity` field on its config (`internal/pipeline/config.go` and `internal/pipeline/changelog.go`, mirroring how `ChangelogRemote` is threaded today), set by the app layer in Step 7:

```go
	if p.cfg.ForgeIdentity != nil {
		if lc := linkContextFromIdentity(*p.cfg.ForgeIdentity); lc != nil {
			return lc
		}
	}
```

- [ ] **Step 7: Resolve + construct + inject in the app layer** — `internal/app/pipeline.go`

In both `BuildPipeline` and `BuildChangelogPipeline`, before constructing the generator:

```go
	resolved, err := forge.Resolve(cfg, os.Getenv, gitOriginURL(runner))
	if err != nil {
		return nil, fmt.Errorf("resolving forge: %w", err)
	}
	var enrichForge port.Forge
	var forgeID *port.ForgeIdentity
	if len(resolved.Forges) > 0 {
		id := resolved.Forges[resolved.EnrichmentIndex]
		forgeID = &id
		if id.Type == "gitlab" {
			enrichForge = gitlabforge.New(id, nil)
		}
	}
```

Import the forge package as `gitlabforge "github.com/adaouat/heraut/internal/forge/gitlab"` to avoid colliding with the existing `internal/platforms/gitlab` import. Pass `native.WithForge(enrichForge)` when non-nil (guard it — a typed-nil interface would defeat the `g.forge != nil` check), and set the pipeline config's `ForgeIdentity: forgeID`.

Add the git-origin helper (`internal/app/pipeline.go`), tolerating repos with no origin:

```go
// gitOriginURL returns the origin remote URL, or "" when there is no origin (forge resolution
// then falls back to CI env or offline).
func gitOriginURL(runner port.Runner) string {
	out, _, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
```

- [ ] **Step 8: Run the full suite + lint**

Run: `go test ./... && hk fix`
Expected: all PASS; lint clean. Existing GitLab-enrichment tests that assert the legacy `glab` path must still pass — they construct the generator without `WithForge`, so they take the fallback branch.

- [ ] **Step 9: Commit**

```bash
git add internal/generators/native/ internal/app/pipeline.go internal/pipeline/
git commit -m "feat(app): wire the resolved forge into native enrichment (T160)"
```

---

### Task 4 (T160b): remove `changelog.remote`, rename the policy key, add the migration error

**Files:**
- Modify: `internal/config/config.go` (delete `Remote` struct + `ContentDriver.Remote`; delete `Commits.RemoteMetadata`)
- Modify: `internal/config/commits.go` (delete `RemoteMetadata`)
- Modify: `internal/config/loader.go` (migration detection)
- Modify: `internal/config/validator.go` (drop the old-remote rules)
- Modify: `internal/pipeline/{config.go,changelog.go,linkctx.go}` (drop `ChangelogRemote` + `remoteLinkContext`)
- Modify: `internal/generators/native/enrich.go` + any `RemoteMetadata` reader → `EnrichmentPolicy`
- Modify: `schema.json`, `docs/heraut.sample.yml`
- Test: `internal/config/migration_test.go` (new)

**Interfaces:**
- Consumes: `config.Commits.EnrichmentPolicy` (Plan A, T155).
- Produces: `var ErrRemovedConfigKey = errors.New("removed config key")` in `internal/config`, returned by `Load` when a removed key is present.

**Removed keys and their replacements** (the error must state the mapping):

| Removed | Replacement |
|---|---|
| `changelog.remote` | top-level `forges:` + `commits.enrichment_forge` |
| `commits.remote_metadata` | `commits.enrichment_policy` |

`release.platforms` is **NOT** removed (see Scope decision).

- [ ] **Step 1: Write the failing test** — `internal/config/migration_test.go`

```go
package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "heraut.yml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestLoad_RemovedKeys(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantHint string
	}{
		{
			name: "changelog.remote",
			body: `version: "1"
versioning: {strategy: semver}
changelog:
  generator: native
  output: CHANGELOG.md
  remote:
    type: gitlab
    project: group/subgroup/project
`,
			wantHint: "forges:",
		},
		{
			name: "commits.remote_metadata",
			body: `version: "1"
versioning: {strategy: semver}
commits:
  remote_metadata: required
`,
			wantHint: "enrichment_policy",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
			assert.Contains(t, err.Error(), tc.wantHint, "the error must name the replacement")
		})
	}
}

// release.platforms is deliberately NOT removed in this cut — it must still load.
func TestLoad_PlatformsStillSupported(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`))
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run 'TestLoad_(RemovedKeys|PlatformsStillSupported)'`
Expected: FAIL — `undefined: config.ErrRemovedConfigKey`, and the old keys still parse.

- [ ] **Step 3: Add the migration error** — `internal/config/loader.go`

Because the loader is strict, deleting the struct fields alone makes removed keys produce a generic "unknown key" error. Detect them first and return a mapped, actionable error.

**Loader shape (verified):** `Load(path)` currently does `forgeconfig.Load(path, &cfg)` and `LoadFromReader(r)` does `forgeconfig.Decode(r, &cfg)` — neither holds the raw bytes, and `loader.go` does **not** import a YAML package today. So you must read the bytes yourself and add the `gopkg.in/yaml.v3` import (already a project dependency — no new module):

- `Load`: `raw, err := os.ReadFile(path)` → `checkRemovedKeys(raw)` → then `forgeconfig.Decode(bytes.NewReader(raw), &cfg)` (equivalent to the old `forgeconfig.Load`, and keeps a single read).
- `LoadFromReader`: `raw, err := io.ReadAll(r)` → `checkRemovedKeys(raw)` → `forgeconfig.Decode(bytes.NewReader(raw), &cfg)`.

Both paths must run the check, so a config loaded from a reader gets the same migration error. Add:

```go
// ErrRemovedConfigKey reports a config key removed by the forge migration (ADR-0043).
var ErrRemovedConfigKey = errors.New("removed config key")

// removedKeys maps a removed config path to its replacement guidance.
var removedKeys = []struct{ path, hint string }{
	{"changelog.remote", "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it"},
	{"commits.remote_metadata", "rename to `commits.enrichment_policy` (same values: disabled | optional | required)"},
}

// checkRemovedKeys reports the first removed key present in the raw YAML, with migration guidance.
func checkRemovedKeys(raw []byte) error {
	var probe struct {
		Changelog struct {
			Remote any `yaml:"remote"`
		} `yaml:"changelog"`
		Commits struct {
			RemoteMetadata any `yaml:"remote_metadata"`
		} `yaml:"commits"`
	}
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return nil // malformed YAML surfaces from the strict parse with better context
	}
	present := map[string]bool{
		"changelog.remote":        probe.Changelog.Remote != nil,
		"commits.remote_metadata": probe.Commits.RemoteMetadata != nil,
	}
	for _, k := range removedKeys {
		if present[k.path] {
			return fmt.Errorf("%w: `%s` — %s", ErrRemovedConfigKey, k.path, k.hint)
		}
	}
	return nil
}
```

Call `checkRemovedKeys(raw)` **before** the strict decode in both `Load` and `LoadFromReader`, so the mapped error wins over the generic "unknown key" error.

- [ ] **Step 4: Delete the old fields and their consumers**

1. `internal/config/config.go` — delete the `Remote` struct and `ContentDriver.Remote`.
2. `internal/config/commits.go` — delete `RemoteMetadata`; keep `EnrichmentPolicy`.
3. `internal/config/config.go` — update the `RemoteMetadata()` accessor to read `EnrichmentPolicy` and rename it `EnrichmentPolicy()`; update its callers.
4. `internal/config/validator.go` — delete the `changelog.remote` validation block (around the `d.Remote == nil` guard, ~line 525).
5. `internal/config/merge.go` — delete the per-env `override.Remote` merge branch (~line 45).
6. `internal/pipeline/config.go` + `changelog.go` — delete the `ChangelogRemote` fields; `internal/app/pipeline.go` assigns them in two places (`pCfg.ChangelogRemote = driver.Remote`, `cCfg.ChangelogRemote = driver.Remote`) — delete both.
7. `internal/pipeline/linkctx.go` — delete `remoteLinkContext`, `remoteBaseURL`, `tokenEnvOrDefault` and their now-unused constants if nothing else references them; delete the `remoteLinkContext` branch in `changelogLinkContext`.
8. `internal/generators/native/enrich.go` — `g.cfg.RemoteMetadata` → the `EnrichmentPolicy`-sourced field (the `ContentDriver.RemoteMetadata` propagated field can keep its Go name; only the YAML-facing key changed — rename it too if it is not load-bearing elsewhere).
9. `internal/app/` — update wherever the policy is propagated onto the ContentDriver.

Follow the compiler: `go build ./...` until clean.

- [ ] **Step 5: Sync `schema.json` and `docs/heraut.sample.yml`**

Delete the `changelog.remote` schema block and the `commits.remote_metadata` property; ensure `commits.enrichment_policy` and `commits.enrichment_forge` are present (added in T155). Remove the `changelog.remote` / `remote_metadata` sections from the sample and make sure the `forges:` block is documented as the replacement. Delete or update any fixture in `testdata/config/` that uses a removed key — **do not delete a fixture that covers a still-supported feature**; migrate it to the new keys instead.

- [ ] **Step 6: Run the full suite + lint**

Run: `go test ./... && hk fix`
Expected: all PASS. Tests that exercised `changelog.remote` should now assert the migration error (update them; do not delete the assertion's intent).

- [ ] **Step 7: Update the roadmap + commit**

Flip **T158, T159, T160** to `[x]` in `docs/tasks/forge-abstraction-roadmap.md`, each with a one-paragraph completion note, and update the "Progress at a glance" P1 row to Complete. Note in T160's entry that `release.platforms` removal and `release.targets` publishing moved to **P3** per the enrichment-first scope decision.

```bash
git add internal/ schema.json docs/heraut.sample.yml testdata/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "feat(config)!: remove changelog.remote, rename remote_metadata (T160)"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] `hk check` → clean.
- [ ] `git grep -n "changelog.remote\|remote_metadata"` → only migration-error strings, tests asserting them, ADRs, and roadmap history.
- [ ] `git grep -n "release.platforms\|EffectivePlatforms"` → **still present and working** (removal is P3).
- [ ] No real data in any changed file — synthetic placeholders only.
- [ ] Manual sanity (optional, no network): `heraut check config` against a `forges:`-only config resolves without error.

## Handoff to P2 / P3

- **P2** migrates GitHub + Azure onto `port.Forge` and retires the legacy `enrich()` dispatch entirely; the `fromPortPRs` converter added in Task 3 becomes the single boundary (watch `Platforms` / `Author.Name`/`Email`, which the port model does not carry).
- **P3** folds publishing into the forge: `release.targets` starts driving publication, `release.platforms` is removed, and the publishing HTTP client (stdlib vs official SDK) is decided in its own ADR.
- **P4** updates the `heraut init` wizard last, once the config has been battle-tested.
