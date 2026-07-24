# Azure DevOps commit-author `by @` from local git — Design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-24
- **Author**: bchatard (with Claude)
- **Related**: ADR-0035 (Azure native `net/http` enrichment), ADR-0036 (unified enrichment model), ADR-0039 (commit-author attribution), ADR-0042 (GitLab batched GraphQL), roadmap T151 (this)

---

## Problem

Native renders `by @<commit author>` for GitHub (ADR-0039) and GitLab (ADR-0042) but **not**
Azure DevOps. `enrich()`'s azure branch returns `enrichResult{prs: prs}` — no `authors` map — so
`overlayAuthorHandles` has nothing to stamp, and Azure commit lines carry only the MR reference
(`in [!N]`), no `by @`.

## Spike findings (Azure has no resolvable identity)

A live spike against a real Azure DevOps instance established that Azure cannot map a commit to an
Azure identity/handle:

1. **Commits API** (`_apis/git/repositories/{repo}/commits`) — each commit's `author`/`committer`
   carry only git `name`/`email`/`date`; **no identity id or linked account**.
2. **Identities API** (`vssps …/_apis/identities?searchFilter=General&filterValue=<git-email>`) —
   returns **no match** for the git author email.
3. **Graph subject query** (`vssps …/_apis/graph/subjectquery`) by the git email — **no match**.

The corporate git email is not the Azure account UPN, and Azure exposes no lookup from one to the
other. Unlike GitHub (`author.user.login`) and GitLab (`author.username`), **there is no API path
to an Azure handle** — batchable or otherwise. The only commit-author signal available is the
**local git author email**, which native already collects (`rawCommit.Email`, `%ae`).

## Goal

Give Azure DevOps the same `by @<author>` attribution GitHub/GitLab have, sourced from the **local
git author email's local-part** — the exact rendering `enrich_azure.go`'s `azureAuthorLogin`
already applies to Azure PR authors — with **no new API call**.

## Non-goals

- **Resolving a "real" Azure identity / clickable @mention** — impossible (spike). `by @<localpart>`
  is a text attribution, not an Azure mention. This is inherent to Azure.
- **Any new HTTP request** — the handle is purely local. The existing `pullrequestquery` POST
  (for MR refs / review metadata) is unchanged.
- **GitHub / GitLab behavior** — unchanged.

## Design

### 1. `azureCommitAuthors` — a pure local helper

Add to `internal/generators/native/enrich_azure.go`:

```go
// azureCommitAuthors maps each commit SHA to its author handle — the git author email's
// local-part, via the existing azureAuthorLogin. Azure exposes no identity resolvable from a git
// email (T151 spike), so this local render is the only source; it makes no API call. Commits whose
// author yields no handle are omitted.
func azureCommitAuthors(commits []rawCommit) map[string]string {
	authors := make(map[string]string, len(commits))
	for _, c := range commits {
		if h := azureAuthorLogin(azureIdentityRef{DisplayName: c.Author, UniqueName: c.Email}); h != "" {
			authors[c.Hash] = h
		}
	}
	return authors
}
```

`azureAuthorLogin` (unchanged) returns the local-part of `UniqueName` (the email) when present,
else the full `DisplayName` (the git name). `rawCommit` always carries `%ae`, so in practice this
is the email local-part — identical to how Azure PR authors already render. It credits the commit
**author** (`%ae`), per ADR-0039.

### 2. Wire the `enrich()` dispatch

`internal/generators/native/enrich.go`, azure branch:

```go
case "azure_devops":
    prs, err := enrichAzure(g.httpClient, lc, shas)
    return enrichResult{prs: prs, authors: azureCommitAuthors(commits)}, err
```

`enrich()` already receives `commits []rawCommit`, so no signature change. The `authors` map flows
into the existing `overlayAuthorHandles(groups, er.authors)` → `buildCommit` → template
`by @{{ .Author.Username }}`. No renderer/template change.

### 3. Policy gating (falls out of the existing path — the resolved sub-decision)

Because it rides `enrich()` → `enrichForRelease`, Azure `by @` is gated by `remote_metadata`
exactly like GitHub/GitLab, with no special-casing:

- `disabled` / `--offline`: `enrichForRelease` returns before calling `enrich()` → no `authors` →
  no Azure `by @`.
- `optional` + `pullrequestquery` fails: `enrichForRelease` degrades and discards the whole
  `enrichResult` (including the local `authors`) → no Azure `by @` (+ the degrade sub-result).
- `optional`/`required` + query succeeds: `authors` is used → Azure renders `by @<localpart> in [!N]`.

This keeps `by @` uniformly "part of enrichment" across all three platforms. Trade-off (accepted):
a transient `pullrequestquery` failure also drops the *local* attribution — acceptable for the
lowest-value platform and simplest (no restructuring).

## Error handling

`azureCommitAuthors` is pure and cannot fail. `enrichAzure` keeps its existing error wrapping →
`enrichForRelease` applies the `remote_metadata` policy. No change.

## Testing

Contract / unit (`internal/generators/native`, synthetic data only):

- `azureCommitAuthors`: `sha → email local-part`; a commit with only a name (no email) falls back
  to the git name; empty author omitted.
- `enrich()` azure dispatch returns a populated `authors` map (verified via the end-to-end test).
- End-to-end `TestGenerate_Enrich_Azure` (`enrich_azure_internal_test.go`): flip the current
  `assert.NotContains(t, out, "by @", …)` to `assert.Contains(t, out, "by @a in [!42](…)")` — the
  fixture commit author is `a@example.com` → `a`. Keep the `in [!42]` and `Degraded()==false`
  assertions.

**Confidentiality:** the spike used real, confidential Azure data (host, emails, names). None of it
appears in any test, fixture, spec, or doc — synthetic placeholders only (`a@example.com`, `a`).

## Documentation & records

- **Roadmap (`docs/tasks/native-generator-roadmap.md`):** mark **T151 `[x]` done**, with a
  completion note: the spike proved no API resolves an Azure identity from the git email, so the
  commit-author handle is rendered from the local git email local-part (no new API call); gated by
  `remote_metadata` like GitHub/GitLab. Update the Phase 2.10 glance row (Azure now covered).
- **Spec 05 (`docs/specs/05-generators-and-platforms.md`):** update the "Azure DevOps only, this
  cut — does not yet resolve the commit-author handle" caveat: Azure now renders `by @<author>`
  from the local git email local-part (no Azure identity is resolvable from a git email); note it
  is not a clickable Azure mention.
- **No new ADR** — a small extension of ADR-0039 (attribution) and ADR-0035 (Azure enrichment),
  captured in the spec + roadmap.

## Migration

Additive and non-breaking: Azure commit lines gain `by @<author>` where they previously had none;
default templates render it automatically. GitHub, GitLab, and `disabled`/offline behavior are
unchanged.
