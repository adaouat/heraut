# GitLab enrichment via batched GraphQL — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace GitLab's O(commit) per-commit MR REST enrichment with two batched `glab api graphql` connection queries so GitLab renders `by @<commit author> in [!N]` (plus MR review-metadata) at O(pages).

**Architecture:** `enrich_gitlab.go` is rewritten to fetch (a) commit-author handles from `project.repository.commits(ref:)` and (b) merge requests from `project.mergeRequests`, inverting MR→commits (`mergeCommitSha` ∪ `squashCommitSha` ∪ `commits.nodes.sha`) into a `sha→MR` map. It returns both maps like `enrichGitHub`, flowing into the existing `overlayAuthorHandles` + PR overlay. No renderer/template change.

**Tech Stack:** Go, `glab api graphql` (CLI, via `port.Runner.RunEnv`), GitLab GraphQL API, testify, `exectest.MockRunner`.

## Global Constraints

- TDD mandatory: failing test first, RED, then implement (`.claude/rules/testing.md`).
- No new Go dependencies. `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib.
- Contract tests for every CLI invocation use `exectest.MockRunner` — assert exact args + env; no network.
- Never bypass git hooks. Lint via `hk fix` (never `gofmt`/`yamlfmt` directly).
- Enrichment stays behind `enrichForRelease`, so `remote_metadata` (required/optional/disabled) and `--force` are honored unchanged — no new config.
- SHA match is authoritative; date bounds (`committedAfter`/`mergedAfter`) only cap pagination. A commit matching no MR renders no `in [!N]`; an unlinked author renders no `by @` — neither is an error.
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` (no `Claude-Session:` line).
- **Spike-dependent constants** (confirmed in Task 1, patched into Tasks 2-4 before dispatch if they differ): the `glab api graphql` invocation (`glab api graphql -f query=<q>`); whether `commits(ref:)` accepts a commit SHA as `ref`; `iid` scalar type (String vs Int); field/arg names `committedAfter`, `mergedAfter`, `squashCommitSha`, `mergedBy`, `labels{nodes{title}}`.

---

### Task 1: Live GraphQL spike (orchestrator + user; not a subagent)

**Why not a subagent:** it requires an authenticated `glab` against a live GitLab instance, which sandboxed subagents lack. The orchestrator asks the user to run the queries and relay output, then patches the spike-dependent constants into Tasks 2-4.

**Confirm (record each answer):**

1. **glab GraphQL invocation.** Does this return data?
   ```bash
   glab api graphql -f query='{ currentUser { username } }'
   ```
   If not, find the working form (e.g. `glab api graphql --field query=...` or `glab api --method POST /graphql ...`).
2. **`commits(ref:)` accepts a SHA + `committedAfter` + `author.username`:**
   ```bash
   glab api graphql -f query='{project(fullPath:"<owner>/<repo>"){repository{commits(ref:"<a-recent-sha>",first:2){nodes{sha author{username}}pageInfo{endCursor hasNextPage}}}}}'
   ```
   Record: does `ref:"<sha>"` work (vs only branch/tag)? Is `author` ever `null`?
3. **`mergeRequests` fields + args:**
   ```bash
   glab api graphql -f query='{project(fullPath:"<owner>/<repo>"){mergeRequests(state:merged,first:2){nodes{iid webUrl title author{username}mergedAt mergedBy{username}labels{nodes{title}}mergeCommitSha squashCommitSha commits{nodes{sha}}}pageInfo{endCursor hasNextPage}}}}'
   ```
   Record: `iid` type (String or Int in JSON), presence of `squashCommitSha`, `mergedBy`, `labels{nodes{title}}`, and whether `mergedAfter:"<ISO8601>"` is accepted.

- [ ] **Step 1:** Orchestrator posts the three queries to the user; user runs them and relays output.
- [ ] **Step 2:** Record answers in the ledger. If `commits(ref:"<sha>")` does NOT accept a SHA, the ref strategy changes: Task 4 uses `currentBranch(runner)` (a new `git branch --show-current` helper) instead of `newestSHA(commits)`. If `iid` is a String, the parse struct uses `json:"iid"` on a `string` field + `strconv.Atoi`. Patch Tasks 2-4's query strings / parse structs to the confirmed names before dispatching them.
- [ ] **Step 3:** No commit (spike only).

---

### Task 2: `fetchGitLabAuthors` — commit-author handles via `commits(ref:)`

**Files:**
- Modify: `internal/generators/native/enrich_gitlab.go` (add query builder, parse, fetch; old code stays until Task 4)
- Test: `internal/generators/native/enrich_gitlab_internal_test.go` (add tests; old tests stay until Task 4)

**Interfaces:**
- Produces: `func fetchGitLabAuthors(runner port.Runner, lc *port.LinkContext, ref, committedAfter string, want map[string]bool) (map[string]string, error)` — `sha→author.username` for SHAs in `want`; `func projectPath(lc *port.LinkContext) string` → `lc.Owner + "/" + lc.Repo`.

- [ ] **Step 1: Write the failing test**

Add to `enrich_gitlab_internal_test.go`:

```go
func TestFetchGitLabAuthors_MapsAndOmitsNull(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[
		{"sha":"aaa","author":{"username":"alice"}},
		{"sha":"bbb","author":null},
		{"sha":"ccc","author":{"username":"carol"}}
	],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)

	got, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "2026-01-01T00:00:00Z", map[string]bool{"aaa": true, "bbb": true, "ccc": true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"aaa": "alice", "ccc": "carol"}, got)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, "graphql", mr.Calls[0].Args[1])
	assert.Contains(t, mr.Calls[0].Args[2], `commits(ref:"tagsha",committedAfter:"2026-01-01T00:00:00Z",first:100`)
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_TOKEN=tok")
}

func TestFetchGitLabAuthors_Paginates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[{"sha":"aaa","author":{"username":"alice"}}],"pageInfo":{"endCursor":"C1","hasNextPage":true}}}}}}`, "", nil)
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[{"sha":"bbb","author":{"username":"bob"}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)

	got, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "", map[string]bool{"aaa": true, "bbb": true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"aaa": "alice", "bbb": "bob"}, got)
	require.Len(t, mr.Calls, 2)
	assert.Contains(t, mr.Calls[1].Args[2], `after:"C1"`)
}

func TestFetchGitLabAuthors_ErrorsArray(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"errors":[{"message":"boom"}]}`, "", nil)
	_, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "", map[string]bool{"aaa": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run TestFetchGitLabAuthors`
Expected: compile failure — `fetchGitLabAuthors`, `projectPath` undefined.

- [ ] **Step 3: Implement**

Add to `enrich_gitlab.go` (keep the existing REST code for now):

```go
func projectPath(lc *port.LinkContext) string { return lc.Owner + "/" + lc.Repo }

func gitLabAuthorsQuery(project, ref, committedAfter, after string) string {
	var extra strings.Builder
	if committedAfter != "" {
		fmt.Fprintf(&extra, `,committedAfter:"%s"`, committedAfter)
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:"%s"`, after)
	}
	return fmt.Sprintf(`{project(fullPath:"%s"){repository{commits(ref:"%s"%s,first:100){nodes{sha author{username}}pageInfo{endCursor hasNextPage}}}}}`,
		project, ref, extra.String())
}

type gitLabCommitsResponse struct {
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
					PageInfo struct {
						EndCursor   string `json:"endCursor"`
						HasNextPage bool   `json:"hasNextPage"`
					} `json:"pageInfo"`
				} `json:"commits"`
			} `json:"repository"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func fetchGitLabAuthors(runner port.Runner, lc *port.LinkContext, ref, committedAfter string, want map[string]bool) (map[string]string, error) {
	authors := make(map[string]string)
	seen := make(map[string]bool)
	after := ""
	for {
		query := gitLabAuthorsQuery(projectPath(lc), ref, committedAfter, after)
		stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", "graphql", "-f", "query="+query)
		if err != nil {
			return nil, fmt.Errorf("glab api graphql commits: %w", err)
		}
		var resp gitLabCommitsResponse
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			return nil, fmt.Errorf("parsing glab graphql commits response: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("glab graphql commits: %s", resp.Errors[0].Message)
		}
		c := resp.Data.Project.Repository.Commits
		for _, n := range c.Nodes {
			if !want[n.SHA] {
				continue
			}
			seen[n.SHA] = true
			if n.Author != nil && n.Author.Username != "" {
				authors[n.SHA] = n.Author.Username
			}
		}
		if !c.PageInfo.HasNextPage || len(seen) == len(want) {
			return authors, nil
		}
		after = c.PageInfo.EndCursor
	}
}
```

Ensure `enrich_gitlab.go` imports `strings` (add it).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run TestFetchGitLabAuthors`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_gitlab.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(generators/native): fetch GitLab commit-author handles via GraphQL"
```

---

### Task 3: `fetchGitLabMRs` — merge requests + inversion to `sha→MR`

**Files:**
- Modify: `internal/generators/native/enrich_gitlab.go` (add MR query, parse, fetch, inversion)
- Test: `internal/generators/native/enrich_gitlab_internal_test.go`

**Interfaces:**
- Consumes: `projectPath` (Task 2).
- Produces: `func fetchGitLabMRs(runner port.Runner, lc *port.LinkContext, mergedAfter string, want map[string]bool) (map[string]PullRequest, error)` — `sha→PullRequest` indexed by `mergeCommitSha` ∪ `squashCommitSha` ∪ `commits.nodes.sha`, first MR per SHA wins.

- [ ] **Step 1: Write the failing test**

```go
func TestFetchGitLabMRs_InvertsAllShaSources(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[
		{"iid":7,"webUrl":"https://gitlab.com/g/p/-/merge_requests/7","title":"Add x",
		 "author":{"username":"alice"},"mergedAt":"2026-01-02T00:00:00Z","mergedBy":{"username":"maint"},
		 "labels":{"nodes":[{"title":"enhancement"}]},
		 "mergeCommitSha":"merge1","squashCommitSha":"squash1","commits":{"nodes":[{"sha":"src1"},{"sha":"src2"}]}}
	],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)

	got, err := fetchGitLabMRs(mr, gitlabLC(), "2026-01-01T00:00:00Z", map[string]bool{"merge1": true, "squash1": true, "src1": true, "nope": true})
	require.NoError(t, err)

	want := PullRequest{Number: 7, URL: "https://gitlab.com/g/p/-/merge_requests/7", Title: "Add x",
		AuthorLogin: "alice", Labels: []string{"enhancement"}, RefPrefix: "!",
		MergedAt: got["merge1"].MergedAt, MergedBy: Author{Username: "maint"}}
	assert.Equal(t, want, got["merge1"])
	assert.Equal(t, 7, got["squash1"].Number, "squashCommitSha maps to the MR")
	assert.Equal(t, 7, got["src1"].Number, "source commit sha maps to the MR")
	assert.NotContains(t, got, "src2", "src2 not in want")
	assert.NotContains(t, got, "nope", "no MR → absent")
	assert.Equal(t, "2026-01-02T00:00:00Z", got["merge1"].MergedAt.UTC().Format(time.RFC3339))

	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Args[2], `mergeRequests(state:merged,mergedAfter:"2026-01-01T00:00:00Z",first:100`)
}

func TestFetchGitLabMRs_Paginates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[{"iid":1,"webUrl":"u1","author":{"username":"a"},"mergeCommitSha":"m1","commits":{"nodes":[]}}],"pageInfo":{"endCursor":"C1","hasNextPage":true}}}}}`, "", nil)
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[{"iid":2,"webUrl":"u2","author":{"username":"b"},"mergeCommitSha":"m2","commits":{"nodes":[]}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)

	got, err := fetchGitLabMRs(mr, gitlabLC(), "", map[string]bool{"m1": true, "m2": true})
	require.NoError(t, err)
	assert.Equal(t, 1, got["m1"].Number)
	assert.Equal(t, 2, got["m2"].Number)
	require.Len(t, mr.Calls, 2)
	assert.Contains(t, mr.Calls[1].Args[2], `after:"C1"`)
}

func TestFetchGitLabMRs_ErrorsArray(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"errors":[{"message":"bad mr query"}]}`, "", nil)
	_, err := fetchGitLabMRs(mr, gitlabLC(), "", map[string]bool{"m1": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad mr query")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run TestFetchGitLabMRs`
Expected: compile failure — `fetchGitLabMRs` undefined.

- [ ] **Step 3: Implement**

Add to `enrich_gitlab.go`:

```go
func gitLabMRsQuery(project, mergedAfter, after string) string {
	var extra strings.Builder
	if mergedAfter != "" {
		fmt.Fprintf(&extra, `,mergedAfter:"%s"`, mergedAfter)
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:"%s"`, after)
	}
	return fmt.Sprintf(`{project(fullPath:"%s"){mergeRequests(state:merged%s,first:100){nodes{iid webUrl title author{username}mergedAt mergedBy{username}labels{nodes{title}}mergeCommitSha squashCommitSha commits{nodes{sha}}}pageInfo{endCursor hasNextPage}}}}`,
		project, extra.String())
}

type gitLabMRNode struct {
	IID    int    `json:"iid"`
	WebURL string `json:"webUrl"`
	Title  string `json:"title"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	MergedAt time.Time `json:"mergedAt"`
	MergedBy struct {
		Username string `json:"username"`
	} `json:"mergedBy"`
	Labels struct {
		Nodes []struct {
			Title string `json:"title"`
		} `json:"nodes"`
	} `json:"labels"`
	MergeCommitSHA  string `json:"mergeCommitSha"`
	SquashCommitSHA string `json:"squashCommitSha"`
	Commits         struct {
		Nodes []struct {
			SHA string `json:"sha"`
		} `json:"nodes"`
	} `json:"commits"`
}

type gitLabMRsResponse struct {
	Data struct {
		Project struct {
			MergeRequests struct {
				Nodes    []gitLabMRNode `json:"nodes"`
				PageInfo struct {
					EndCursor   string `json:"endCursor"`
					HasNextPage bool   `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"mergeRequests"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func mrNodeToPR(n gitLabMRNode) PullRequest {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, l := range n.Labels.Nodes {
		labels = append(labels, l.Title)
	}
	if len(labels) == 0 {
		labels = nil
	}
	return PullRequest{
		Number: n.IID, URL: n.WebURL, Title: n.Title, AuthorLogin: n.Author.Username,
		Labels: labels, RefPrefix: "!", MergedAt: n.MergedAt,
		MergedBy: Author{Username: n.MergedBy.Username},
	}
}

// landingSHAs are every SHA by which an MR can match a target-branch commit: the merge commit,
// the squash commit, and each source commit (fast-forward merges land those directly).
func landingSHAs(n gitLabMRNode) []string {
	shas := make([]string, 0, len(n.Commits.Nodes)+2)
	if n.MergeCommitSHA != "" {
		shas = append(shas, n.MergeCommitSHA)
	}
	if n.SquashCommitSHA != "" {
		shas = append(shas, n.SquashCommitSHA)
	}
	for _, c := range n.Commits.Nodes {
		shas = append(shas, c.SHA)
	}
	return shas
}

func fetchGitLabMRs(runner port.Runner, lc *port.LinkContext, mergedAfter string, want map[string]bool) (map[string]PullRequest, error) {
	prs := make(map[string]PullRequest)
	after := ""
	for {
		query := gitLabMRsQuery(projectPath(lc), mergedAfter, after)
		stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", "graphql", "-f", "query="+query)
		if err != nil {
			return nil, fmt.Errorf("glab api graphql merge_requests: %w", err)
		}
		var resp gitLabMRsResponse
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			return nil, fmt.Errorf("parsing glab graphql merge_requests response: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("glab graphql merge_requests: %s", resp.Errors[0].Message)
		}
		mrs := resp.Data.Project.MergeRequests
		for _, n := range mrs.Nodes {
			pr := mrNodeToPR(n)
			for _, sha := range landingSHAs(n) {
				if want[sha] {
					if _, ok := prs[sha]; !ok {
						prs[sha] = pr
					}
				}
			}
		}
		if !mrs.PageInfo.HasNextPage || len(prs) == len(want) {
			return prs, nil
		}
		after = mrs.PageInfo.EndCursor
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run TestFetchGitLabMRs`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_gitlab.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(generators/native): fetch GitLab MRs via GraphQL, invert to sha map"
```

---

### Task 4: Cutover — `enrichGitLab` orchestration + `enrich()` dispatch; remove REST

**Files:**
- Modify: `internal/generators/native/enrich_gitlab.go` (replace `enrichGitLab` + remove old REST `gitLabMR`/`parseGitLabMRs`)
- Modify: `internal/generators/native/enrich.go` (gitlab dispatch + `oldestCommitDate` + `newestSHA`)
- Modify: `internal/generators/native/enrich_gitlab_internal_test.go` (remove the 8 REST unit tests; rewrite `TestGenerate_Enrich_GitLab`; keep/adapt `TestEnrichGitLab_SelfHostedHostInAPIEnv` as GraphQL)

**Interfaces:**
- Consumes: `fetchGitLabAuthors` (Task 2), `fetchGitLabMRs` (Task 3), `rawCommit.Date`/`.Hash`.
- Produces: `func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string, since time.Time, ref string) (map[string]PullRequest, map[string]string, error)`; helpers `oldestCommitDate([]rawCommit) time.Time`, `newestSHA([]rawCommit) string`.

- [ ] **Step 1: Write the failing end-to-end test**

Replace the old `TestGenerate_Enrich_GitLab` in `enrich_gitlab_internal_test.go` with:

```go
// End-to-end: GitLab enrichment now renders "by @<commit author> in [!N]" from two batched
// GraphQL queries (commits for authors, mergeRequests for the ref).
func TestGenerate_Enrich_GitLab(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	// authors query (commits connection):
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[{"sha":"abc1234567","author":{"username":"alice"}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)
	// mergeRequests query (merge commit == the release commit):
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[{"iid":7,"webUrl":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"bob"},"mergeCommitSha":"abc1234567","commits":{"nodes":[]}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", gitlabLC())
	require.NoError(t, err)
	assert.Contains(t, out, "by @alice in [!7](https://gitlab.com/g/p/-/merge_requests/7)",
		"commit author @alice (not MR author @bob), MR ref !7")
	assert.False(t, g.Degraded())
}
```

Also replace `TestEnrichGitLab_SelfHostedHostInAPIEnv` (it referenced the REST endpoint) with:

```go
func TestEnrichGitLab_SelfHostedHostInAPIEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)
	lc := &port.LinkContext{Platform: "gitlab", BaseURL: "https://git.example.com", Owner: "g", Repo: "p", Token: "tok"}

	_, _, err := enrichGitLab(mr, lc, []string{"abc123"}, time.Time{}, "abc123")
	require.NoError(t, err)
	require.Len(t, mr.Calls, 2)
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_HOST=git.example.com")
}
```

Delete the other seven REST tests: `TestEnrichGitLab_ReviewFields`, `_MapsMR`, `_TitleAndLabels`, `_NoMR_Absent`, `_ErrorWrapped`, `_MalformedJSON`, `_Subgroup`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'TestGenerate_Enrich_GitLab|TestEnrichGitLab_SelfHosted'`
Expected: compile failure — `enrichGitLab` old signature / old REST tests reference removed symbols.

- [ ] **Step 3: Replace `enrichGitLab` + remove REST code**

In `enrich_gitlab.go`, delete the old REST `enrichGitLab`, the `gitLabMR` struct, and `parseGitLabMRs`. Add:

```go
// enrichGitLab returns sha→MR and sha→commit-author-handle for the given SHAs via two batched
// glab api graphql connection queries. since bounds pagination to the release window; ref is the
// commits(ref:) anchor (the range tip). Returns empty maps for no SHAs.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string, since time.Time, ref string) (map[string]PullRequest, map[string]string, error) {
	if len(shas) == 0 {
		return map[string]PullRequest{}, map[string]string{}, nil
	}
	want := make(map[string]bool, len(shas))
	for _, s := range shas {
		want[s] = true
	}
	sinceStr := ""
	if !since.IsZero() {
		// Subtract a small buffer so a boundary commit/MR at exactly `since` is not excluded by an
		// exclusive committedAfter/mergedAfter. SHA match is authoritative; this only bounds pagination.
		sinceStr = since.Add(-time.Minute).UTC().Format(time.RFC3339)
	}
	authors, err := fetchGitLabAuthors(runner, lc, ref, sinceStr, want)
	if err != nil {
		return nil, nil, err
	}
	prs, err := fetchGitLabMRs(runner, lc, sinceStr, want)
	if err != nil {
		return nil, nil, err
	}
	return prs, authors, nil
}
```

Remove the now-unused `net/url` import if present.

- [ ] **Step 4: Wire `enrich()` dispatch + helpers**

In `enrich.go`, change the gitlab case and add the helpers:

```go
	case "gitlab":
		prs, authors, err := enrichGitLab(g.runner, lc, shas, oldestCommitDate(commits), newestSHA(commits))
		return enrichResult{prs: prs, authors: authors}, err
```

```go
// oldestCommitDate returns the minimum committed date over commits (bounds the GitLab fetch), or
// the zero time when empty.
func oldestCommitDate(commits []rawCommit) time.Time {
	var oldest time.Time
	for _, c := range commits {
		if oldest.IsZero() || c.Date.Before(oldest) {
			oldest = c.Date
		}
	}
	return oldest
}

// newestSHA returns the hash of the newest-dated commit (the range tip; the commits(ref:) anchor),
// or "" when empty.
func newestSHA(commits []rawCommit) string {
	var newest rawCommit
	for _, c := range commits {
		if newest.Date.IsZero() || c.Date.After(newest.Date) {
			newest = c
		}
	}
	return newest.Hash
}
```

Add `"time"` to `enrich.go` imports.

> Spike contingency (Task 1): if `commits(ref:)` rejects a SHA, add `func currentBranch(runner port.Runner) (string, error)` (`git branch --show-current`) and pass its result as `ref` instead of `newestSHA(commits)`; the end-to-end test then queues one extra `git branch --show-current` response.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/generators/native/`
Expected: PASS (all native tests, incl. the rewritten GitLab e2e).

- [ ] **Step 6: Commit**

```bash
git add internal/generators/native/enrich_gitlab.go internal/generators/native/enrich.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(generators/native): GitLab enrichment via batched GraphQL (authors + MR refs)"
```

---

### Task 5: Remove the obsolete GitLab regeneration rate-limit warning

**Files:**
- Modify: `internal/pipeline/warn.go` (remove `gitlabRegenWarning`, simplify `changelogGenResult`)
- Modify: `internal/pipeline/warn_internal_test.go` (remove `TestGitlabRegenWarning`, update `TestChangelogGenResult`)
- Modify: `internal/pipeline/changelog.go`, `internal/pipeline/release.go` (drop the now-unused `regenerate`/`lc` args at the `changelogGenResult` call)

**Interfaces:**
- Consumes: `degradedSubs` (unchanged).
- Produces: `func changelogGenResult(gen port.Generator) (detail string, subs []string)` — degrade branch unchanged; success branch returns `("", nil)`.

- [ ] **Step 1: Update the failing test**

In `warn_internal_test.go`, delete `TestGitlabRegenWarning`, and replace `TestChangelogGenResult` with:

```go
func TestChangelogGenResult(t *testing.T) {
	// Degraded: "without enrichment" + reason + omission note.
	detail, subs := changelogGenResult(&testutil.MockGenerator{DegradedVal: true, DegradedReasonV: "boom"})
	assert.Equal(t, "without enrichment", detail)
	assert.Equal(t, []string{"boom", omitNote}, subs)

	// Not degraded: no detail, no sub-results (GitLab is now batched — no rate-limit heads-up).
	detail, subs = changelogGenResult(&testutil.MockGenerator{DegradedVal: false})
	assert.Empty(t, detail)
	assert.Empty(t, subs)
}
```

Remove the now-unused `port` import from `warn_internal_test.go` if nothing else uses it.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/pipeline/ -run TestChangelogGenResult`
Expected: compile failure — `changelogGenResult` still takes `(regenerate, lc, gen)`.

- [ ] **Step 3: Implement**

In `warn.go`, delete `gitlabRegenWarning` entirely and change `changelogGenResult`:

```go
// changelogGenResult builds the completed-step detail and sub-results for a changelog generation
// step. On degrade the step is labelled "without enrichment" and lists the failure reason plus the
// omission note; otherwise there is nothing to add.
func changelogGenResult(gen port.Generator) (detail string, subs []string) {
	if subs = degradedSubs(gen); subs != nil {
		return "without enrichment", subs
	}
	return "", nil
}
```

In `changelog.go` and `release.go`, change the call from
`detail, subs := changelogGenResult(p.cfg.RegenerateChangelog, changelogCtx, p.cfg.Changelog)`
to
`detail, subs := changelogGenResult(p.cfg.Changelog)`.

- [ ] **Step 4: Run to verify it passes**

Run: `go build ./... && go test ./internal/pipeline/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/warn.go internal/pipeline/warn_internal_test.go internal/pipeline/changelog.go internal/pipeline/release.go
git commit -m "refactor(pipeline): drop GitLab regen rate-limit warning (now batched)"
```

---

### Task 6: Docs — ADR-0042 + spec 05 + roadmap

**Files:**
- Create: `docs/adr/0042-gitlab-graphql-enrichment.md`
- Modify: `docs/adr/README.md` (index row, per repo convention)
- Modify: `docs/specs/05-generators-and-platforms.md`
- Modify: `docs/tasks/native-generator-roadmap.md` (T150 → `[x]`, glance table)

- [ ] **Step 1: Write ADR-0042**

Create `docs/adr/0042-gitlab-graphql-enrichment.md` (match the header style of `0041-...md`):

```markdown
# ADR-0042: GitLab enrichment via batched GraphQL

- **Status**: Accepted
- **Date**: 2026-07-23
- **Deciders**: bchatard

---

## Context

Native rendered `by @<handle> in [#N]` for GitHub but not GitLab. GitLab's enrichment made a
per-commit REST call to `/repository/commits/:sha/merge_requests` — O(commit), slow and
rate-limit-prone on large repos — and never resolved the commit author's GitLab handle, so GitLab
carried no `by @`. GitLab GraphQL exposes no commit→MR field, but a spike found two batched paths:
`repository.commits(ref:).nodes.author.username` (commit-author handles) and `mergeRequests.nodes`
with `mergeCommitSha` / `squashCommitSha` / `commits.nodes.sha` (invertible to a `commitSha → MR`
map).

## Decision

GitLab enrichment uses two batched `glab api graphql` connection queries:

1. **Commit authors** — `commits(ref: <range tip>, committedAfter: <oldest range date>)`, paginated,
   filtered to the range SHAs → `by @<commit author>`.
2. **MR references** — `mergeRequests(state: merged, mergedAfter: <oldest range date>)`, paginated,
   inverted by indexing each MR under its `mergeCommitSha`, `squashCommitSha`, and each source
   `commits.nodes.sha` (covering merge / squash / fast-forward strategies) → `in [!N]` plus MR
   review-metadata.

SHA match is authoritative; the date bounds only cap pagination. A commit with no MR renders no
`in [!N]`; an unlinked author renders no `by @`. The per-commit MR REST enrichment is removed, so
GitLab is now O(pages), and the `--regenerate` GitLab rate-limit warning is dropped.

## Consequences

- GitLab reaches parity with GitHub — `by @<commit author> in [!N]` with MR review-metadata — at a
  fraction of the API cost.
- Enrichment stays on the `glab` CLI transport; Phase 3 (raw-HTTP clients) would port it later.
- `by @` credits the *commit* author (ADR-0039), not the MR author.
```

- [ ] **Step 2: Update the ADR index**

In `docs/adr/README.md`, add after the 0041 row:
`| [0042](0042-gitlab-graphql-enrichment.md) | GitLab enrichment via batched GraphQL | Accepted |`

- [ ] **Step 3: Update spec 05**

In `docs/specs/05-generators-and-platforms.md`, in the GitLab enrichment description, state that GitLab enrichment now uses batched `glab api graphql` (commit authors via `commits`, MR refs via inverted `mergeRequests`), and note the graceful omission: a commit attributable to no MR renders no `in [!N]`. (Locate the current GitLab enrichment paragraph and edit it faithfully.)

- [ ] **Step 4: Update the roadmap**

In `docs/tasks/native-generator-roadmap.md`: flip `#### \`[ ]\` T150` → `#### \`[x]\` T150`, retitle to note MR refs are included, add a completion note (batched GraphQL, authors + inverted MR refs, per-commit REST dropped), and update the "Progress at a glance" Phase 2.10 row to drop T150 from the follow-ups.

- [ ] **Step 5: Lint + commit**

```bash
hk fix
git add docs/adr/0042-gitlab-graphql-enrichment.md docs/adr/README.md docs/specs/05-generators-and-platforms.md docs/tasks/native-generator-roadmap.md
git commit -m "docs(adr): 0042 GitLab enrichment via batched GraphQL"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] `hk check` (or `mise run lint:check`) → clean.
- [ ] Manual integration check (orchestrator + user, live GitLab): on `/tmp/release-notes` run
  `./heraut changelog --regenerate --verbose` and confirm the trace shows the two `glab api graphql`
  calls (commits + mergeRequests), and the changelog renders `by @<author> in [!N]` where an MR
  exists. Do not commit/push in that repo.
```
