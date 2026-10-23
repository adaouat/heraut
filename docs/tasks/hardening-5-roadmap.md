# Héraut — Hardening 5 Roadmap (security follow-ups)

> Status: Complete (drafted 2026-10-23)
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
| 78 | Security follow-ups from the 2026-10-22 re-scan | T419–T420 | Complete |

### Phase 78 — Security follow-ups

#### `[x]` T419: the GitHub and Azure enrichment clients refuse a token-leaking redirect (NEW-001, CWE-523)

`internal/forge/github/github.go:27` and `internal/forge/azure/azure.go:27` build an `http.Client` with no
`CheckRedirect`, so an https request redirected to http on the same host keeps its `Authorization` header.
The GitLab clients already refuse it (`withTokenSafeRedirects`, `internal/forge/gitlab/gitlab.go`,
`internal/platforms/gitlab/client.go`). Reuse that rule: refuse a redirect whose scheme is not https
(loopback excepted, for `httptest`) or whose host changes. Test first with an `httptest.Server` that
redirects, for both clients, asserting the second request never carries the token.

**Completion note.** `withTokenSafeRedirects` was added to `internal/forge/github/github.go` and
`internal/forge/azure/azure.go`; `New` wraps the client (a caller-set `CheckRedirect` is kept). It deletes
`Authorization` when the redirect changes host or drops https to http, and stops after 10 redirects, the same
rule as the GitLab client. Deviation: the task said "refuse"; the GitLab client drops the header instead of
failing the request, and I kept that so the three clients behave alike. The function is copied, not shared:
the layer table lets `forge/*` import only `port` and `config`, and a shared helper would need a new package
for 15 lines. Tests are `redirect_internal_test.go` in both packages (four URL rows plus the redirect limit),
written first and red (a nil `CheckRedirect` panicked). Sensitivity: removing the https-to-http clause fails
the downgrade row; restored.

#### `[x]` T420: `writeFileAtomic` keeps the existing file's mode (NEW-005, CWE-732)

`internal/generators/native/generator.go:496` forces `0644` on the replaced changelog, so a stricter mode
(for example `0600`) is lost at the next write. `os.Stat` the target first and reuse its mode; `0644` only
for a new file. Test first: an existing `0600` changelog keeps `0600` after a write; a new file gets `0644`;
a symlinked path keeps the target's mode. Add one e2e row next to `TestChangelogWrite_IsAtomic`.

**Completion note.** `writeFileAtomic` stats the resolved path first and reuses its permission bits; the
`0644` argument now applies only to a new file. Unit tests `TestWriteChangelog_KeepsTheModeOfAnExistingFile`
and `..._OfASymlinkTarget` were written first and red; the new-file `0644` test stays. One e2e subtest in
`TestChangelogWrite_IsAtomic` chmods the changelog to `0600` between two runs of the built binary. Sensitivity:
stashing the production change fails all three; restored. Deferred: the group and owner of an existing file
are not preserved (a rename creates the file as the current user), which the finding did not ask for.

**Phase 78 exit:** `go test -count=1 ./...` passes, each task confirmed to fail when its fix is reverted, and
lint is clean.
