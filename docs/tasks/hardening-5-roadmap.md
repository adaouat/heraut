# Héraut — Hardening 5 Roadmap (security follow-ups)

> Status: Not started (drafted 2026-10-23)
> Source: the security re-scan of 2026-10-22 (`analysis/heraut/SECURITY_FINDINGS.md`, candidates NEW-001 and
> NEW-005), scheduled by the user on 2026-10-23. Main roadmap: tracked as Phase 78 in [`roadmap.md`](roadmap.md).

## Conventions

- Task IDs **continue the global sequence** (`T419+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): failing test first, implement, flip the
  marker and add a one-paragraph completion note.
- One task per session. Neither task changes documented behavior, so no ADR is needed (a note in the
  completion paragraph is enough). Both are sensitivity-checked like the Phase 77 scenarios.

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 78 | Security follow-ups from the 2026-10-22 re-scan | T419–T420 | Not started |

### Phase 78 — Security follow-ups

#### `[ ]` T419: the GitHub and Azure enrichment clients refuse a token-leaking redirect (NEW-001, CWE-523)

`internal/forge/github/github.go:27` and `internal/forge/azure/azure.go:27` build an `http.Client` with no
`CheckRedirect`, so an https request redirected to http on the same host keeps its `Authorization` header.
The GitLab clients already refuse it (`withTokenSafeRedirects`, `internal/forge/gitlab/gitlab.go`,
`internal/platforms/gitlab/client.go`). Reuse that rule: refuse a redirect whose scheme is not https
(loopback excepted, for `httptest`) or whose host changes. Test first with an `httptest.Server` that
redirects, for both clients, asserting the second request never carries the token.

#### `[ ]` T420: `writeFileAtomic` keeps the existing file's mode (NEW-005, CWE-732)

`internal/generators/native/generator.go:496` forces `0644` on the replaced changelog, so a stricter mode
(for example `0600`) is lost at the next write. `os.Stat` the target first and reuse its mode; `0644` only
for a new file. Test first: an existing `0600` changelog keeps `0600` after a write; a new file gets `0644`;
a symlinked path keeps the target's mode. Add one e2e row next to `TestChangelogWrite_IsAtomic`.

**Phase 78 exit:** `go test -count=1 ./...` passes, each task confirmed to fail when its fix is reverted, and
lint is clean.
