# ADR-0035: Azure DevOps enrichment via a native `net/http` client (not the `az` CLI)

- **Status**: Accepted
- **Date**: 2026-07-03
- **Deciders**: bchatard
- **Supersedes**: [ADR-0034](0034-native-remote-enrichment.md) §3 (Azure transport only —
  the `az`-CLI choice). ADR-0034's GitHub (`gh api`) and GitLab (`glab api`) decisions stand.

---

## Context

T129 is the last Phase-2 native-enrichment task: PR / author attribution for Azure DevOps,
bringing the native generator to ADR-0026 parity. [ADR-0034](0034-native-remote-enrichment.md)
§3 chose the **`az` CLI** (`az repos pr` / `az devops invoke`, `azure-devops` extension) on the
same `port.Runner` path as `gh`/`glab`, explicitly to avoid pulling ADR-0032's Phase-3 HTTP
rewrite forward.

Two facts, surfaced while sequencing T129, change that call:

1. **`az` is Python.** heraut ships a batteries-included Docker image ([ADR-0016](0016-bundled-docker-image.md))
   that bundles every external CLI. Bundling `az` drags a full Python runtime + `pip` + a large,
   frequently-churning dependency tree into the image — a real, ongoing maintenance surface.
2. **heraut does not already require `az`.** `gh` and `glab` are CLIs heraut *already* depends on
   for its GitHub/GitLab **publish** drivers, so reusing them for enrichment is free. heraut has
   **no Azure publish driver** (only link-host + URL composition from
   [ADR-0026](0026-azure-devops-metadata-remote.md)). `az` would be **net-new** weight for the
   **lowest-value** enrichment platform — an asymmetry that justifies a different transport for
   Azure specifically.

heraut needs exactly **one** Azure endpoint: the batched commit→PR lookup
(`POST …/pullrequestquery`). The Microsoft Go SDK (`github.com/microsoft/azure-devops-go-api`)
was considered — it is light (one dependency, `google/uuid`) and maintained (`v7.1.0`, 2026-06) —
but for a single stable endpoint it carries unnecessary `Connection`/client ceremony and a large
generated source surface.

## Decision

Implement Azure DevOps enrichment as a **thin, native `net/http` client** in
`internal/generators/native/enrich_azure.go`, superseding ADR-0034 §3's `az`-CLI choice.

### Request

One **batched** request per release (better than GitLab's per-commit; matches ADR-0034 §4's
bounded-fetch intent):

```http
POST {BaseURL}/{organization}/{project}/_apis/git/repositories/{repository}/pullrequestquery?api-version=7.1
Content-Type: application/json
Authorization: Basic base64(":" + <PAT>)

{"queries":[{"type":"lastMergeCommit","items":["<sha>", …]}]}
```

- `type: lastMergeCommit` — "pull requests that created the supplied merge commits" (the
  commit→PR direction we want; the `commit` type is broader and unneeded).
- `organization`/`project` are split from `LinkContext.Owner` (the `organization/project`
  string, per ADR-0026); `repository` is `LinkContext.Repo`; base is `LinkContext.BaseURL`
  (`https://dev.azure.com`, or an on-prem Azure DevOps Server host via `changelog.remote.api_url`).

### Auth

PAT via `Authorization: Basic base64(":" + LinkContext.Token)` (empty username — the Azure DevOps
PAT convention). The token is **`LinkContext.Token`**, already resolved from
`changelog.remote.token_env` (default `AZURE_DEVOPS_TOKEN`, ADR-0026) — the *same* field
`gh`/`glab` enrichment reads. **No new env var** (`az`'s `AZURE_DEVOPS_EXT_PAT` is not used), and
`port.LinkContext.APIEnv()` stays `nil` for `azure_devops` (it feeds CLI env auth, which native
Azure does not use).

### Response → `prInfo`

`results[0]` is a `commitId → []pullRequest` dictionary. For each release SHA, take the first PR:

| `prInfo` field | Source |
|----------------|--------|
| `Number`       | `pullRequestId` |
| `URL`          | composed `azureRepoRoot(lc) + "/pullrequest/" + id` (the REST body carries no web URL) |
| `AuthorLogin`  | `createdBy.uniqueName` local-part (before `@`), falling back to `createdBy.displayName` |
| `RefPrefix`    | `"!"` |
| `FirstTimer`   | always `false` — see deferral below |

### Policy, dispatch, testing

- Routed through the existing `enrich` / `enrichForRelease` seam (add the `azure_devops` case), so
  the `remote_metadata` policy (`disabled` / `required` / `optional`) and `Degraded()` behave
  identically to GitHub/GitLab (ADR-0034 §6/§7). A non-2xx status or transport error returns a
  wrapped error; the policy decides fatal-vs-degrade.
- The `native` generator gains an `*http.Client` (sane default timeout) used only by the Azure
  path. Contract-tested with **`httptest.Server`** — the HTTP analog of `MockRunner`: assert the
  method, path, `api-version`, `Authorization` header, and request body; return canned JSON. The
  project already sanctions `httptest.Server` (self-update tests); no real network.

### Scope boundary

This is a **narrow Phase-3 slice for Azure only.** `gh`/`glab` stay CLI-via-runner (justified by
being already-required publish deps). The full Phase 3 — dropping `gh`/`glab` for HTTP clients —
remains deferred behind ADR-0032.

## Consequences

- heraut owns the Azure wire (auth header + JSON models) for one endpoint — small, isolated to
  `enrich_azure.go`. **No `az`, no Python, no Docker bloat, zero new Go dependencies** (stdlib only).
- Azure enrichment diverges from the runner/`MockRunner` test model, using `httptest.Server`
  instead. Accepted: it is isolated, deterministic, and network-free.
- `heraut check runtime` gains **no** Azure CLI check — native Azure enrichment needs only the
  token + network, not a binary on `PATH`.
- **First-timer detection is deferred** (no "New Contributors" block for Azure), mirroring GitLab's
  initial T128 scope: Azure PRs carry no `authorAssociation`, and the "earliest merged PR" trick
  (as done for GitLab in T137) is a follow-up, not part of this ADR.

## Alternatives considered

- **`az` CLI (ADR-0034 §3).** Rejected now: Python runtime + churny deps in the bundled image, a
  net-new dependency for the lowest-value platform. `az devops invoke` can reach `pullrequestquery`,
  but the transport cost outweighs the consistency benefit here.
- **`azure-devops-go-api` SDK.** Rejected: for one endpoint the `Connection`/client ceremony and
  large generated surface aren't worth it, even though its dependency footprint is light.
- **Full Phase 3 (drop `gh`/`glab` too).** Out of scope; remains deferred behind ADR-0032.
