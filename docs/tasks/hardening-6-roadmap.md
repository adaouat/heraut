# Héraut — Hardening 6 Roadmap (remaining security findings)

> Status: Not started (drafted 2026-10-24)
> Source: the Info findings NEW-003 and NEW-004 of the security re-scan (`analysis/heraut/SECURITY_FINDINGS.md`),
> scheduled by the user on 2026-10-24. NEW-002 (symlink swap between the glob check and the read) is **not**
> scheduled. Main roadmap: tracked as Phase 79 in [`roadmap.md`](roadmap.md).

## Conventions

- Task IDs **continue the global sequence** (`T421+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): ADR addendum first (both tasks refuse
  an input that is accepted today), failing test, implement, flip the marker and add a completion note.
- One task per session. Each task is sensitivity-checked and gets one e2e row.

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 79 | Remaining security findings (NEW-003, NEW-004) | T421–T422 | Not started |

### Phase 79 — Remaining security findings

#### `[ ]` T421: `--set-version` has a character allowlist (NEW-003, CWE-20; ADR-0074 addendum)

`tagfmt.ValidateVersionOverride` (`internal/versioning/tagfmt/tagfmt.go:139`) rejects only an empty value and
whitespace; shell safety of `--set-version` rests on the two per-strategy checks (`resolver.go`). Add the
allowlist `[0-9A-Za-z._+/-]` (the `/` stays: a full tag override may contain one) so a future strategy that
skips both checks cannot reach hooks unfiltered. ADR-0074 gets an addendum (this narrows what the flag accepts
for any strategy). Tests: a table of accepted and refused values (`$(id)`, `;`, backtick, quote, `*`, `{`),
the Config exit code, and one e2e row.

#### `[ ]` T422: an asset file name containing `#` is refused (NEW-004, CWE-73; ADR-0076 addendum)

`gh release create` reads `path#label` as a display-label separator, so a file whose path contains `#`
changes the asset's label or is not found (`internal/platforms/github/platform.go:186`). The GitLab driver is
unaffected. Refuse such a path with a message that names it, in the asset gate (`CheckAssetGlobs`) so it fails
before anything is committed or pushed, for the GitHub target only. ADR-0076 gets an addendum. Tests: the
gate refuses `dist/app#1.txt` for a GitHub target and accepts it for a GitLab target; one e2e row through
`release`.

**Phase 79 exit:** `go test -count=1 ./...` passes, each task confirmed to fail when its fix is reverted, and
lint is clean.
