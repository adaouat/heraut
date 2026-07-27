# Forge P2 — GitHub + Azure onto `port.Forge` (T161, T162) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring GitHub and Azure DevOps under the same `port.Forge` abstraction GitLab already uses, so all three platforms enrich through one interface and the legacy per-platform `enrich()` switch retires.

**Architecture:** Two new packages mirroring `internal/forge/gitlab`: `internal/forge/github` (stdlib `net/http` carrying the existing, live-proven GraphQL query — so `heraut changelog` needs no external CLI for GitHub users) and `internal/forge/azure` (wrapping the existing `net/http` `pullrequestquery` logic). `port.ForgeIdentity` gains a `Repository` field, which Azure needs (org + project + repo) and which also lets Azure rejoin forge-derived link rendering.

**Tech Stack:** Go, stdlib `net/http` + `encoding/json`, `testify`, `httptest`. No new dependencies.

## Context: where this sits

P1 shipped `port.Forge`, config/resolution, and the GitLab forge (REST default + opt-in GraphQL). Today `internal/generators/native/enrich.go` has a hybrid: an injected `port.Forge` is preferred, and everything else falls through to a legacy per-platform switch calling `enrichGitHub` (via `gh api graphql`), `enrichGitLab` (via `glab`), and `enrichAzure` (native http). This plan converts the remaining two platforms and deletes that switch.

**Out of scope (later phases):** publishing still goes through `internal/platforms/{github,gitlab}` and still requires `gh`/`glab` — that is P3, together with `release.targets` and the removal of `release.platforms`. The `heraut init` wizard is P4.

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven tests preferred.
- **No new Go dependencies.** `internal/forge/*` imports only `internal/port` + stdlib. `internal/port` imports nothing from heraut. `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib. `internal/app` is the only place constructing concrete implementations.
- **No network in tests** — `httptest.Server` only. **No real data** — synthetic placeholders only (`github.example.com`, `dev.azure.example.com`, `acme/widget`, `group/subgroup/project`, `alice`).
- **No environment leakage** — any test touching resolution injects `getenv` or calls the existing `clearCIEnv(t)` helper in `internal/app`. (A real CI break was caused by exactly this: GitHub Actions' own `GITHUB_ACTIONS`/`GITHUB_REPOSITORY` hijacked forge resolution.)
- **Do not remove or alter `release.platforms`** and do not touch `internal/scaffold/` — both are later phases.
- **`httptest` performs no API-schema validation.** Any query string or endpoint path you write is unvalidated by the tests. Therefore: **port existing, live-proven request shapes verbatim** — do not redesign a query or "improve" an endpoint while moving it.
- Errors wrapped with `%w`; sentinels `errors.Is`-able.
- Never bypass git hooks. Fix lint via `hk fix` (never `gofmt`/`yamlfmt` directly). Full `go test ./...` before committing.
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — with angle brackets, and **never** a `Claude-Session:` line.
- Each task flips its roadmap entry to `[x]` in `docs/tasks/forge-abstraction-roadmap.md` with a one-paragraph completion note.

## File Structure

- `internal/port/forge.go` — **modify**: add `Repository` to `ForgeIdentity`.
- `internal/forge/github/github.go` — **new**: `Forge` struct, constructor, `Type`/`Identity`/link methods.
- `internal/forge/github/graphql.go` — **new**: the batched GraphQL enrichment (ported from `internal/generators/native/enrich_github.go`).
- `internal/forge/azure/azure.go` — **new**: `Forge` struct, constructor, `Type`/`Identity`/link methods.
- `internal/forge/azure/prquery.go` — **new**: the `pullrequestquery` enrichment (ported from `internal/generators/native/enrich_azure.go`).
- `internal/forge/{resolve,detect}.go` — **modify**: populate `Repository`; complete Azure CI identity.
- `internal/app/pipeline.go` — **modify**: construct the github/azure forges alongside gitlab.
- `internal/pipeline/linkctx.go` — **modify**: drop the `azure_devops` exclusion.
- `internal/generators/native/{enrich,enrich_github,enrich_gitlab,enrich_azure}.go` — **modify/delete**: retire the legacy switch.

Task 1 is GitHub end-to-end; Task 2 is Azure plus the switch removal. Each ends green and independently reviewable.

---

### Task 1 (T161): GitHub forge over native `net/http`

**Files:**
- Modify: `internal/port/forge.go` (add `Repository`)
- Create: `internal/forge/github/github.go`
- Create: `internal/forge/github/graphql.go`
- Modify: `internal/forge/resolve.go` (populate `Repository`)
- Modify: `internal/app/pipeline.go` (construct the github forge)
- Test: `internal/forge/github/github_test.go`

**Interfaces:**
- Consumes: `port.Forge`, `port.ForgeIdentity{Type,Host,APIURL,Project,Repository,Token,TokenKind,APIMode}`, `port.Commit{Hash,Author,Email,Date}`, `port.Enrichment{PRs map[string]PullRequest; Authors map[string]string}`, `port.PullRequest{Number,URL,Title,AuthorLogin,Labels,RefPrefix,CreatedAt,MergedAt,MergedBy,Approvers}`, `port.Author{Username}`.
- Produces (used by Task 2 and the app layer): `func New(id port.ForgeIdentity, client *http.Client) *Forge` (nil client ⇒ 30s-timeout default); `(*Forge)` implements `port.Forge`.

**Reference implementation — read before writing:** `internal/generators/native/enrich_github.go` holds the query and response types that work against the live API. Port `prFragment`, `buildGitHubQuery`, the `graphQLResponse`/`graphQLCommit`/`graphQLPR` structs, and `parseGitHubResponse`'s mapping **verbatim** — only the transport (runner → `net/http`) and the result types (native → `port`) change. `internal/forge/gitlab/gitlab.go` is the structural template for the forge type itself.

Auth: GitHub's GraphQL API takes `Authorization: bearer <token>`. `apiBase()` is `id.APIURL` when set (GitHub Actions provides `GITHUB_API_URL`), else `https://api.github.com` for `github.com`, else `{host}/api/v3` for GitHub Enterprise Server. GitHub has no job-token concept, so `TokenKind` does not select a header here.

- [ ] **Step 1: Add `Repository` to `port.ForgeIdentity`**

In `internal/port/forge.go`, add the field to `ForgeIdentity` (after `Project`):

```go
	// Repository is the repository name when a forge separates it from the project path — Azure
	// DevOps addresses a repo as organization/project + repository. GitHub and GitLab carry the
	// full path in Project and leave this empty.
	Repository string
```

- [ ] **Step 2: Write the failing tests** — `internal/forge/github/github_test.go`

```go
package github_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/forge/github"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := github.New(port.ForgeIdentity{
		Type: "github", Host: "https://github.com", Project: "acme/widget",
	}, nil)
	assert.Equal(t, "github", f.Type())
	assert.Equal(t, "https://github.com/acme/widget/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://github.com/acme/widget/pull/42", f.ChangeURL(42))
	assert.Equal(t, "https://github.com/acme/widget/compare/v1.0.0...v1.1.0", f.CompareURL("v1.0.0", "v1.1.0"))
}

func TestEnrich_MapsPRsAndAuthors(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":{"login":"alice"}},
			"associatedPullRequests":{"nodes":[{
				"number":42,"url":"https://github.com/acme/widget/pull/42","title":"Add widget",
				"author":{"login":"alice"},
				"labels":{"nodes":[{"name":"feature"}]},
				"createdAt":"2026-07-01T10:00:00Z","mergedAt":"2026-07-02T11:00:00Z",
				"mergedBy":{"login":"bob"},
				"latestReviews":{"nodes":[{"state":"APPROVED","author":{"login":"carol"}}]}
			}]}
		}}}}`))
	}))
	defer srv.Close()

	f := github.New(port.ForgeIdentity{
		Type: "github", Host: srv.URL, APIURL: srv.URL, Project: "acme/widget",
		Token: "ghtok", TokenKind: port.TokenPrivate,
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	require.NoError(t, err)

	assert.Equal(t, "bearer ghtok", gotAuth, "GitHub GraphQL authenticates with a bearer token")
	assert.Contains(t, gotBody, "abc123", "the commit SHA is queried")

	// GitHub resolves the LINKED commit-author handle (unlike GitLab REST / Azure).
	assert.Equal(t, "alice", en.Authors["abc123"])
	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "#", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1)
	assert.Equal(t, "carol", pr.Approvers[0].Username)
}

// A commit whose author email isn't linked to a GitHub account yields no handle, and a commit
// with no associated PR yields no PR entry — neither is an error.
func TestEnrich_UnlinkedAuthorAndNoPR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":null},"associatedPullRequests":{"nodes":[]}}}}}`))
	}))
	defer srv.Close()

	f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
	require.NoError(t, err)
	assert.Empty(t, en.Authors)
	assert.Empty(t, en.PRs)
}

func TestEnrich_GraphQLErrorAndStatus(t *testing.T) {
	t.Run("api error in body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Bad credentials"}]}`))
		}))
		defer srv.Close()
		f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Bad credentials")
	})

	t.Run("non-2xx status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

// More than one chunk's worth of SHAs must be fetched in multiple batched queries, and results
// merged — the legacy driver chunked at 50 to stay within GitHub's node limits.
func TestEnrich_ChunksLargeCommitSets(t *testing.T) {
	var queries int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":{"login":"alice"}},"associatedPullRequests":{"nodes":[]}}}}}`))
	}))
	defer srv.Close()

	commits := make([]port.Commit, 0, 51)
	for i := 0; i < 51; i++ {
		commits = append(commits, port.Commit{Hash: fmt.Sprintf("sha%02d", i), Author: "Alice"})
	}
	f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
	_, err := f.Enrich(commits)
	require.NoError(t, err)
	assert.Equal(t, 2, queries, "51 SHAs must be split into 2 chunks of at most 50")
}

func TestEnrich_NoCommits(t *testing.T) {
	f := github.New(port.ForgeIdentity{Host: "https://github.com", Project: "acme/widget"}, nil)
	en, err := f.Enrich(nil)
	require.NoError(t, err)
	assert.Empty(t, en.PRs)
	assert.Empty(t, en.Authors)
}
```

(`io` is used by the body-capture test, `fmt` by the chunking test.)

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/forge/github/`
Expected: compile failure — no such package / `undefined: github.New`.

- [ ] **Step 4: Implement `internal/forge/github/github.go`**

Mirror `internal/forge/gitlab/gitlab.go`'s structure exactly:

```go
// Package github implements port.Forge for GitHub over stdlib net/http, so changelog enrichment
// needs no `gh` CLI on PATH. See ADR-0043.
package github

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// Forge is the GitHub implementation of port.Forge.
type Forge struct {
	id     port.ForgeIdentity
	client *http.Client
}

var _ port.Forge = (*Forge)(nil)

// New constructs a GitHub forge. A nil client gets a default with a 30s timeout, matching the
// other forges.
func New(id port.ForgeIdentity, client *http.Client) *Forge {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Forge{id: id, client: client}
}

func (f *Forge) Type() string                 { return "github" }
func (f *Forge) Identity() port.ForgeIdentity { return f.id }

// webBase is the repository's web root, e.g. https://github.com/acme/widget.
func (f *Forge) webBase() string {
	return strings.TrimRight(f.id.Host, "/") + "/" + f.id.Project
}

func (f *Forge) CommitURL(sha string) string { return f.webBase() + "/commit/" + sha }
func (f *Forge) ChangeURL(number int) string {
	return fmt.Sprintf("%s/pull/%d", f.webBase(), number)
}
func (f *Forge) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/compare/%s...%s", f.webBase(), from, to)
}

// apiBase returns the GraphQL API root: the explicit APIURL when set (GitHub Actions provides
// GITHUB_API_URL), else api.github.com for github.com and {host}/api/v3 for GitHub Enterprise.
func (f *Forge) apiBase() string {
	if f.id.APIURL != "" {
		return strings.TrimRight(f.id.APIURL, "/")
	}
	host := strings.TrimRight(f.id.Host, "/")
	if host == "https://github.com" || host == "" {
		return "https://api.github.com"
	}
	return host + "/api/v3"
}

// Enrich resolves each commit's associated pull request and linked commit-author handle via
// batched GraphQL queries.
func (f *Forge) Enrich(commits []port.Commit) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	return f.enrichGraphQL(commits)
}
```

- [ ] **Step 5: Implement `internal/forge/github/graphql.go`**

Port from `internal/generators/native/enrich_github.go` **verbatim** where possible — the `prFragment` constant, `buildGitHubQuery` (aliased `s0…sN` objects), the `graphQLResponse`/`graphQLCommit`/`graphQLPR` structs, and `parseGitHubResponse`'s field mapping. Change only:
- transport: build a `POST {apiBase}/graphql` request with body `{"query": "..."}`, set `Content-Type: application/json`, `Accept: application/json`, and `Authorization: bearer <token>` when the token is non-empty;
- result types: build `port.PullRequest` / `port.Author` instead of the native ones, with `RefPrefix: "#"`;
- chunking: keep the `ghChunkSize = 50` constant and the chunk loop, merging each chunk's maps into the result.

Non-2xx → `fmt.Errorf("github graphql: unexpected status %s", resp.Status)`. A `data.errors[0].message` in the body → `fmt.Errorf("github graphql: %s", …)`. Decode failures wrapped with `%w`. The owner/name for the query come from splitting `f.id.Project` on the first `/`.

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./internal/forge/github/`
Expected: PASS (all six tests).

- [ ] **Step 7: Populate `Repository` during resolution**

In `internal/forge/resolve.go`'s `resolveExplicit`, set `Repository` on the built identity from `config.Forge.Repository` for the azure type (github/gitlab keep it empty — they carry the full path in `Project`). Add a focused test row in `internal/forge/resolve_test.go` asserting an azure forge config with `project: myorg/myproject` and `repository: myrepo` yields `Project == "myorg/myproject"` and `Repository == "myrepo"`.

- [ ] **Step 8: Construct the GitHub forge in the app layer**

In `internal/app/pipeline.go`'s `resolveEnrichForgeIfNeeded`, extend the type switch so `github` builds `githubforge.New(id, nil)` alongside the existing `gitlab` branch. Import as `githubforge "github.com/adaouat/heraut/internal/forge/github"` to avoid colliding with `internal/platforms/github`. **Guard the same way the gitlab branch does** — only assign to the `port.Forge` interface variable inside the branch, never a typed nil.

Update `internal/app/forge_internal_test.go`'s existing subtest "non-gitlab resolved forge yields no enrich forge but keeps the identity" — with GitHub now implemented, that expectation changes: assert a forge IS constructed and `f.Type() == "github"`. Keep using the hermetic `fakeEnv` helper.

- [ ] **Step 9: Full suite + lint + roadmap + commit**

Flip **T161** to `[x]` with a completion note.

```bash
go test ./... && hk fix
git add internal/port/forge.go internal/forge/ internal/app/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "feat(forge/github): implement port.Forge over native net/http (T161)"
```

---

### Task 2 (T162): Azure forge + retire the legacy `enrich()` switch

**Files:**
- Create: `internal/forge/azure/azure.go`
- Create: `internal/forge/azure/prquery.go`
- Modify: `internal/forge/detect.go` (complete Azure CI identity)
- Modify: `internal/forge/resolve.go` (azure `Repository`, if not already done in Task 1)
- Modify: `internal/app/pipeline.go` (construct the azure forge)
- Modify: `internal/pipeline/linkctx.go` (drop the `azure_devops` exclusion)
- Modify: `internal/generators/native/enrich.go` (delete the legacy switch)
- Delete: `internal/generators/native/enrich_github.go`, `enrich_gitlab.go`, `enrich_azure.go` and their tests
- Test: `internal/forge/azure/azure_test.go`

**Interfaces:**
- Consumes: `port.ForgeIdentity` incl. the `Repository` field added in Task 1; `port.Forge`; `port.Commit`; `port.Enrichment`.
- Produces: `func New(id port.ForgeIdentity, client *http.Client) *Forge` implementing `port.Forge`.

**Reference implementation — read before writing:** `internal/generators/native/enrich_azure.go` holds the working `pullrequestquery` POST, the `azurePRQuery`/`azurePRQueryResult`/`azurePR`/`azureIdentityRef` types, `azureAuthorLogin`, and `azureCommitAuthors`. Port them **verbatim**, changing only the source of org/project/repo (now `id.Project` + `id.Repository` instead of `lc.Owner`/`lc.Repo`) and the result types (native → `port`).

Azure specifics to preserve: the API version constant (`7.1`), HTTP Basic auth with an empty username and the token as password, `RefPrefix: "!"`, approvers from reviewers with `vote >= 10`, and commit-author handles rendered locally from the git author email's local-part (Azure exposes no linked handle — ADR-0043/T151).

- [ ] **Step 1: Write the failing tests** — `internal/forge/azure/azure_test.go`

```go
package azure_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/forge/azure"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := azure.New(port.ForgeIdentity{
		Type: "azure_devops", Host: "https://dev.azure.example.com",
		Project: "myorg/myproject", Repository: "myrepo",
	}, nil)
	assert.Equal(t, "azure_devops", f.Type())
	assert.Equal(t, "https://dev.azure.example.com/myorg/myproject/_git/myrepo/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://dev.azure.example.com/myorg/myproject/_git/myrepo/pullrequest/42", f.ChangeURL(42))
}

func TestEnrich_MapsPRsAndLocalAuthors(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"abc123":[{
			"pullRequestId":42,"title":"Add widget",
			"createdBy":{"displayName":"Alice","uniqueName":"alice@example.com"},
			"labels":[{"name":"feature"}],
			"creationDate":"2026-07-01T10:00:00Z","closedDate":"2026-07-02T11:00:00Z",
			"closedBy":{"displayName":"Bob","uniqueName":"bob@example.com"},
			"reviewers":[{"displayName":"Carol","uniqueName":"carol@example.com","vote":10}]
		}]}]}`))
	}))
	defer srv.Close()

	f := azure.New(port.ForgeIdentity{
		Type: "azure_devops", Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo",
		Token: "pat", TokenKind: port.TokenPrivate,
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	require.NoError(t, err)

	assert.Contains(t, gotPath, "/myorg/myproject/_apis/git/repositories/myrepo/pullrequestquery")
	assert.True(t, strings.HasPrefix(gotAuth, "Basic "), "Azure authenticates with HTTP Basic")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":pat"))
	assert.Equal(t, want, gotAuth, "the PAT is the password with an empty username")

	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, "alice", pr.AuthorLogin, "email local-part is the handle")
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1)
	assert.Equal(t, "carol", pr.Approvers[0].Username, "only vote >= 10 counts as approval")

	// Azure exposes no linked handle: `by @` comes from the local git author.
	assert.Equal(t, "alice", en.Authors["abc123"])
}

func TestEnrich_ErrorsAndEmpty(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := azure.New(port.ForgeIdentity{Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("project without organization is an error", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.example.com", Project: "myproject", Repository: "myrepo"}, nil)
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err, "Project must be organization/project")
	})

	t.Run("no commits", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.example.com", Project: "myorg/myproject", Repository: "myrepo"}, nil)
		en, err := f.Enrich(nil)
		require.NoError(t, err)
		assert.Empty(t, en.PRs)
	})
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/forge/azure/`
Expected: compile failure — no such package / `undefined: azure.New`.

- [ ] **Step 3: Implement `internal/forge/azure/{azure,prquery}.go`**

`azure.go` mirrors the GitLab/GitHub forge shape: `Forge{id, client}`, `New`, `Type() == "azure_devops"`, `Identity()`, and links built from `{host}/{org}/{project}/_git/{repo}` — `CommitURL` appends `/commit/{sha}`, `ChangeURL` appends `/pullrequest/{n}`, `CompareURL` appends `/branchCompare?baseVersion=GT{from}&targetVersion=GT{to}`. `Enrich` returns an empty `port.Enrichment` for zero commits, else calls the pullrequestquery path.

`prquery.go` ports the request/response logic verbatim from the reference file: endpoint
`{host}/{org}/{project}/_apis/git/repositories/{repo}/pullrequestquery?api-version=7.1`
(each path segment `url.PathEscape`d), POST body `{"queries":[{"type":"lastMergeCommit","items":[…shas]}]}`, `Authorization: Basic base64(":"+token)` when the token is non-empty, and `results[0][sha] → PR` correlation. Keep `azureAuthorLogin` (email local-part, else displayName) and the local commit-author map. Splitting `id.Project` into org/project must error when it has no `/`.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/forge/azure/`
Expected: PASS.

- [ ] **Step 5: Complete Azure CI identity resolution**

`internal/forge/detect.go`'s Azure branch currently returns only `SYSTEM_TEAMPROJECT` as the project and no repository, which cannot address the API. Azure Pipelines also provides the organization inside `SYSTEM_COLLECTIONURI` (e.g. `https://dev.azure.com/myorg/`) and the repository as `BUILD_REPOSITORY_NAME`. Update the branch to return `Project` as `"{org}/{teamProject}"` (org parsed from the collection URI's first path segment) and the repository from `BUILD_REPOSITORY_NAME`, threading a repository value out of `detectCIForge` into the identity.

Add table-driven tests in `internal/forge/resolve_test.go` (using the injected `getenv`, never real env): `TF_BUILD=true`, `SYSTEM_COLLECTIONURI=https://dev.azure.example.com/myorg/`, `SYSTEM_TEAMPROJECT=myproject`, `BUILD_REPOSITORY_NAME=myrepo`, `SYSTEM_ACCESSTOKEN=tok` → identity with `Type: "azure_devops"`, `Project: "myorg/myproject"`, `Repository: "myrepo"`, `TokenKind: port.TokenPrivate`.

- [ ] **Step 6: Construct the azure forge + restore its links**

In `internal/app/pipeline.go`, add the `azure_devops` branch constructing `azureforge.New(id, nil)` (import alias `azureforge`), with the same non-nil guard.

In `internal/pipeline/linkctx.go`, remove `id.Type == "azure_devops"` from `linkContextFromIdentity`'s guard and the comment explaining the exclusion — `ForgeIdentity` now carries `Repository`, so map azure explicitly: `Owner` = `id.Project` (the full `organization/project`, which is what the Azure link/enrichment code expects) and `Repo` = `id.Repository`. Keep the empty-`Project` guard. Update `TestLinkContextFromIdentity`: the azure row now expects a non-nil context with `Owner == "myorg/myproject"` and `Repo == "myrepo"`, and add a row proving an azure identity with an empty `Repository` still returns nil.

- [ ] **Step 7: Retire the legacy switch**

In `internal/generators/native/enrich.go`, delete the entire `lc`-based `switch lc.Platform` block and the now-unreachable `if lc == nil` guard, so `enrich` is just the forge path (returning an empty `enrichResult` when `g.forge == nil`). Delete `enrich_github.go`, `enrich_gitlab.go`, `enrich_azure.go` and their test files, plus any helper left unused (`oldestCommitDate`, `newestSHA`, `enrichable`, `gqlString`) — **follow the compiler**, and check `enrichForRelease` still compiles: its `required && g.forge == nil && !enrichable(lc)` condition must be simplified to drop `enrichable` if that helper goes.

**Preserve behaviour, not code:** `enrichForRelease`'s `remote_metadata`/`enrichment_policy` semantics (disabled / optional-degrade / required-fatal, `--force` downgrade) must be unchanged, and its existing tests must still pass. If deleting a test file would delete an assertion about *policy* behaviour rather than about a deleted transport, move that test rather than dropping it.

- [ ] **Step 8: Full suite + lint**

Run: `go test ./... && hk fix`
Expected: all PASS. Then re-run under a simulated CI environment, because forge resolution reads CI markers:
`GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./...`
Expected: also all PASS.

- [ ] **Step 9: Roadmap + commit**

Flip **T162** to `[x]` with a completion note, and update the "Progress at a glance" P2 row to Complete.

```bash
git add internal/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "feat(forge/azure): implement port.Forge and retire the enrich switch (T162)"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] Full suite green **under a simulated CI environment** (see Task 2 Step 8).
- [ ] `hk check` → clean.
- [ ] `git grep -n "enrichGitHub\|enrichGitLab\|enrichAzure"` → no hits outside history/docs.
- [ ] `git grep -n "release.platforms\|EffectivePlatforms"` → **still present and working** (removal is P3).
- [ ] `internal/generators/native` imports no forge package (the generator consumes `port.Forge` only).
- [ ] No real data in any changed file — synthetic placeholders only.

## Handoff to P3

With all three platforms behind `port.Forge`, P3 folds publishing in: `release.targets` starts driving publication, `release.platforms` and the `internal/platforms/*` drivers retire, `gh`/`glab` leave the runtime entirely, and the publishing HTTP client (stdlib vs official SDK) is decided in its own ADR.
