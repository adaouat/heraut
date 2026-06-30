# ADR-0034: Native remote enrichment (Phase 2)

- **Status**: Accepted
- **Date**: 2026-06-29
- **Deciders**: bchatard

---

## Context

[ADR-0032](0032-native-content-generator.md) added the `native` generator and sequenced its
remote enrichment as **Phase 2**: "add PR-number / author / first-time-contributor /
linked-issue enrichment by calling `gh api` / `glab api` through `port.Runner` … GitHub
first; GitLab and Azure DevOps follow." Phase 1 shipped the renderer without enrichment — the
native `release_notes.tmpl` deliberately omits `by @author`, `in #N`, and the contributors
block. This ADR fixes the *design* of that enrichment before T127 code.

What enrichment must produce (the facts, not git-cliff's exact bytes — heraut's output is its
own spec per ADR-0033). git-cliff's embedded release-notes template
(`cliff.release-notes.toml`) renders, per commit:

```
- <desc> - ([abc1234](url)) by @octocat in [#42](pr-url) ([TICKET-1](href))
```

and a release-level block:

```
### New Contributors ❤️
* @octocat made their first contribution in [#42](pr-url)
```

So enrichment adds three facts: a commit's **PR/MR number** (+ link), its **author handle**,
and whether that author is a **first-time contributor**.

Two existing decisions constrain the design:

- **[ADR-0023](0023-remote-metadata-policy.md)** defines the `remote_metadata` policy
  (`required` / `optional` (default) / `disabled`) and the `Degraded()` signal. For git-cliff
  this maps onto git-cliff's own `--offline` flag. **`native` has no `--offline` flag** — it
  must implement the three branches itself.
- **The layer rule** (`coding.md`): `internal/generators/*` may import only
  `internal/{port,config,conventionalcommit}`. Native **cannot import `internal/platforms`**,
  so it cannot literally "reuse the GitHub platform driver's token/host plumbing." The bridge
  is `port.LinkContext`, which already carries `Owner`, `Repo`, `Platform`, `BaseURL`, and
  `Token` (the GitHub driver sets `Token: os.Getenv(p.tokenEnv())` in `LinkContext()`).

The invocation pattern already exists in the platform drivers:
`runner.RunEnv(["GH_TOKEN="+token], "gh", "api", "repos/{owner}/{repo}/...")`.

## Decision

### 1. Mechanism — `gh api` / `glab api` via `port.Runner`

Enrichment shells `gh api` / `glab api` through `port.Runner.RunEnv`, parsing the JSON
response. No Go SDK and no direct `net/http` — that is ADR-0032's **Phase 3**, fenced behind
its own ADR. Rationale is unchanged from ADR-0032: the CLIs already own auth, pagination, and
rate-limit handling; they are already required for `release create`; and `MockRunner` keeps
the tests network-free.

### 2. Auth flows through `LinkContext`, not a platform import

Native builds the API env (`GH_TOKEN=` / `GITLAB_TOKEN=`, plus `GH_HOST=` for a self-hosted
`BaseURL`) **from the `LinkContext`** it already receives — it does **not** import
`internal/platforms`. If the token/host construction is non-trivial enough to share, it is
lifted to a helper on `port.LinkContext` (e.g. `APIEnv()`), which both the platform drivers
and native may import without violating the layer rule. The platform-specific niceties
(`GITHUB_ACTIONS` fallback, `GH_ENTERPRISE_TOKEN`) stay in the platform driver; native uses
the token already resolved into `lc.Token`.

**Corollary — enrichment needs a token, which only some paths carry.** Release-notes are
generated *per platform* (ADR-0021), so their `LinkContext` carries `Token` → enrichment
authenticates. A changelog may be generated against an *ambient* `LinkContext` (from `remote:`
or CI) that has no token → enrichment cannot authenticate and degrades (see §5/§6).

### 3. Scope — GitHub (T127) → GitLab (T128); Azure via `az` (T129) last

GitHub and GitLab have bundled CLIs heraut already requires and platform drivers that resolve
owner/repo/token. **Azure DevOps (T129) is sequenced last and stays optional.** It is viable
through the **`az` CLI** (`az repos pr`, the `azure-devops` extension) on the same
`port.Runner` path — so it does *not* force the Phase-3 HTTP rewrite — at the cost of a new
*optional* dependency (`az` + extension, token `AZURE_DEVOPS_EXT_PAT`) and correlation-based
mapping (Azure exposes no direct "PRs for a commit" call). heraut has no Azure *publish* driver
today (only the link host + URL composition from
[ADR-0026](0026-azure-devops-metadata-remote.md)), so attribution there is lower-value:
recommendation is to ship GitHub + GitLab and do T129 only when Azure attribution is actually
wanted.

### 4. commit→PR mapping — batched fetch, not per-commit

Match commits to PRs/MRs with a **bounded, batched** fetch (git-cliff's model: ~16 paginated
calls for this repo, independent of commit count), not one call per commit. The batched fetch
is what makes full-changelog enrichment (§5) affordable.

- **GitHub:** a GraphQL query (`gh api graphql`) returning each commit's
  `associatedPullRequests` (number, author `login`, `authorAssociation`), paginated. GraphQL
  is the robust correlation — it resolves squash / merge / rebase without guessing from
  `merge_commit_sha`. (Paginated REST `/pulls?state=closed` + SHA correlation is the fallback
  if GraphQL proves awkward.)
- **GitLab:** paginated `glab api "projects/{id}/merge_requests?state=merged&…"`, correlated
  to commits by `merge_commit_sha` / `squash_commit_sha`.

First-time-contributor status comes from the same data: GitHub `authorAssociation ==
FIRST_TIME_CONTRIBUTOR`; GitLab from whether the author appears in any earlier release's MRs.

The exact query / pagination shape is an implementation detail of T127 / T128; the
**decision** is batched-bounded, not per-commit.

### 5. Enrichment scope: release-notes fully, changelog new section only

- **Release-notes mode** (single release): fully enriched.
- **Changelog mode** (full regeneration): only the **new / unreleased** section is enriched;
  historical sections render from git alone.

**Why not every release** (the original intent, refined during implementation + final review):
enrichment is *not* gated on a token — `enrich` short-circuits only on a nil or
unsupported-platform `LinkContext`. In CI the ambient `LinkContext` carries
`Platform: github|gitlab` (and `gh` / `glab` authenticate via the runner's ambient
credentials), so enriching every historical release would cost **one fetch per release on
every regeneration** — O(releases), exactly the cost §4's batched fetch was meant to avoid.
Scoping changelog enrichment to the new section keeps a full regeneration at **O(1)** API
calls. The newest changelog section carries the same `by @author` attribution as its
release-notes; older sections stay plain (they were plain before native too).

Full **cross-release** changelog enrichment — uniform `by @author` across all releases via the
single batched fetch (§4), matching git-cliff — remains a deferred optimization.

### 6. Policy — native implements the three `remote_metadata` branches itself

`native` has no git-cliff `--offline`, so it owns the branching:

| `remote_metadata` | native behaviour |
|-------------------|------------------|
| `disabled` (or `--offline`) | Never call the API. Bare output. This is exactly Phase-1 behaviour. |
| `required` | Call the API; **any** failure (no token, non-200, rate limit, parse error) is fatal — `Generate` returns a wrapped error. |
| `optional` (default) | Call the API; on **any** failure, drop enrichment, render bare, set `Degraded()` = true, and warn. A genuine non-enrichment error (e.g. bad git range) still surfaces. |

This mirrors ADR-0023's semantics (retry-on-failure, not predict-by-token) without git-cliff's
double-invocation: native simply skips the enrichment step and renders the model it already
has.

### 7. `Degraded()` becomes real

The Phase-1 stub returns `false`. Phase 2: `Degraded()` returns true when enrichment was
attempted under `optional` and failed. The pipeline + `heraut check` already surface
`interface{ Degraded() bool }` (ADR-0023), so no caller change is needed.

### 8. View model + render

The commit view gains optional `{AuthorLogin, PRNumber, PRURL}`; the release view gains a
`Contributors []contributor{Login, PRNumber, PRURL}` (first-timers only). Templates stay dumb
(ADR-0022): the Go view-model assembly appends ` by @login in [#N](url)` to the commit line
and emits the `### New Contributors` block. `rendering` / `commits` config is unaffected — no
new config keys.

### 9. Testing

Contract tests (`MockRunner`) queue canned API JSON, assert the exact `gh api` / `glab api`
argv (endpoint + env), and cover all three policy branches incl. the degraded fallback and a
malformed-JSON failure. No real network; no real-CLI smoke test (it would need credentials and
a network).

## Token requirements

Native enrichment reuses the **same token the publish step already uses** — it adds no new
secret for GitHub or GitLab, and it *removes* one:

| Tool | Used for | Token env |
|------|----------|-----------|
| `gh` | GitHub publish **+ native enrich** (`gh api`) | `GH_TOKEN` (cfg `token_env`); GHES adds `GH_ENTERPRISE_TOKEN` |
| `glab` | GitLab publish **+ native enrich** (`glab api`) | `GITLAB_TOKEN` |
| `git-cliff` | the enrichment `native` replaces | `GITHUB_TOKEN` / `GITLAB_TOKEN` / `AZURE_DEVOPS_TOKEN` |
| `az` (optional, Azure) | native Azure enrich | `AZURE_DEVOPS_EXT_PAT` (or `az login`) |

Today a GitHub project needs **both** `GH_TOKEN` (for `gh` publish) **and** `GITHUB_TOKEN`
(for git-cliff enrichment) — the mismatch behind the ADR-0023 incident. `native` reads the
token through `LinkContext.Token` (= the publish token), so GitHub collapses to **one** token;
GitLab was already single (`GITLAB_TOKEN`, shared with git-cliff). Only Azure-via-`az`
introduces a new token, and only if adopted.

## Alternatives considered

- **Go SDKs (`go-github`, `go-gitlab`) or raw `net/http`.** Rejected for Phase 2: it is
  ADR-0032's Phase 3, adds heavy dependencies + supply-chain surface, and shifts auth /
  pagination / rate-limit maintenance onto heraut. Requires its own ADR.
- **Per-commit mapping** (`commits/{sha}/pulls` per commit). Rejected: O(commits) calls make
  full-changelog enrichment (§5) unaffordable. The batched fetch (§4) is bounded *and* enables
  uniform full enrichment, at the cost of correlation logic.
- **Predict-by-token skip** (skip when no token present). Already rejected by ADR-0023 — it
  misses the present-but-rate-limited case; retry/degrade-on-failure is the faithful policy.
- **A shared enrichment helper in `internal/platforms`.** Rejected — native cannot import it
  (layer rule); the shared surface, if any, lives on `port.LinkContext`.

## Consequences

- Native reaches git-cliff-equivalent **attribution** (`by @author in #N`, New Contributors)
  for GitHub and GitLab release-notes — the last functional gap vs git-cliff for the common
  case.
- heraut takes on the **platform API surface** for enrichment (the ADR-0032 trade), isolated
  to `enrich_github.go` / `enrich_gitlab.go`.
- `gh` / `glab` remain required (already are); **no new token** for GitHub/GitLab (reuses the
  publish token) and one *fewer* than the git-cliff path (drops the separate `GITHUB_TOKEN`).
  Azure attribution is available through the optional `az` CLI (new dep + `AZURE_DEVOPS_EXT_PAT`).
- Enrichment is a bounded batched fetch (~16 calls), independent of commit count, enabling
  uniform full-changelog enrichment that re-renders cleanly on every regeneration.
- A new architectural seam: enrichment auth rides `LinkContext`, reinforcing it as the single
  carrier of platform identity into the generator layer.

## Tasks

Implements roadmap Phase 2: **T127** (GitHub, batched), **T128** (GitLab, batched), **T129**
(Azure DevOps via `az` — optional, last). The deferred Phase-1 "days between releases" stat
(needs the previous tag's date) folds into T127's view-model wiring or ships as a small
standalone follow-up.
