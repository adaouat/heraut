# Héraut — Hardening 6 Roadmap (remaining security findings)

> Status: Complete (drafted 2026-10-24; T421–T422 done, NEW-002 later resolved by T425–T431)
> Source: the Info findings NEW-003 and NEW-004 of the security re-scan (`analysis/heraut/SECURITY_FINDINGS.md`),
> scheduled by the user on 2026-10-24. NEW-002 (symlink swap between the glob check and the read) is **deferred
> by decision** (2026-10-24) until the `gh` dependency is removed for GitHub publishing, as `glab` was for GitLab
> (ADR-0069): with a client in-process the file can be opened once and read from the descriptor, which `gh`'s
> path argument does not allow. Main roadmap: tracked as Phase 79 in [`roadmap.md`](roadmap.md).

## Conventions

- Task IDs **continue the global sequence** (`T421+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): ADR addendum first (both tasks refuse
  an input that is accepted today), failing test, implement, flip the marker and add a completion note.
- One task per session. Each task is sensitivity-checked and gets one e2e row.

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 79 | Remaining security findings (NEW-003, NEW-004) | T421–T422 | Complete |

### Phase 79 — Remaining security findings

#### `[x]` T421: `--set-version` has a character allowlist (NEW-003, CWE-20; ADR-0074 addendum)

`tagfmt.ValidateVersionOverride` (`internal/versioning/tagfmt/tagfmt.go:139`) rejects only an empty value and
whitespace; shell safety of `--set-version` rests on the two per-strategy checks (`resolver.go`). Add the
allowlist `[0-9A-Za-z._+/-]` (the `/` stays: a full tag override may contain one) so a future strategy that
skips both checks cannot reach hooks unfiltered. ADR-0074 gets an addendum (this narrows what the flag accepts
for any strategy). Tests: a table of accepted and refused values (`$(id)`, `;`, backtick, quote, `*`, `{`),
the Config exit code, and one e2e row.

**Done.** `ValidateVersionOverride` now refuses anything outside `[0-9A-Za-z._+/-]`; it still runs first, in `cmd`, ahead of the per-strategy checks, so a shell-syntax value now gets the allowlist message ("only letters, digits…") instead of the CalVer format message. The three CalVer e2e rows that used `x$(id)` / `;id` keep their exit-2 assertion and only changed the expected text; a new `e2e/set_version_charset_test.go` covers the semver path. The `+` stays in the set for the full-tag/CalVer paths, but the SemVer strategies still reject build metadata (hint to use `--set-build-id`). Sensitivity: reverting the production change fails 3 e2e rows and 14 unit rows. Spec 03 updated. No ordering check was added (out of scope).

#### `[x]` T422: an asset file name containing `#` is refused (NEW-004, CWE-73; ADR-0076 addendum)

`gh release create` reads `path#label` as a display-label separator, so a file whose path contains `#`
changes the asset's label or is not found (`internal/platforms/github/platform.go:186`). The GitLab driver is
unaffected. Refuse such a path with a message that names it, in the asset gate (`CheckAssetGlobs`) so it fails
before anything is committed or pushed, for the GitHub target only. ADR-0076 gets an addendum. Tests: the
gate refuses `dist/app#1.txt` for a GitHub target and accepts it for a GitLab target; one e2e row through
`release`.

**Done.** ADR-0076 addendum written first. The rule lives in the GitHub driver's `CheckAssets` (after the shared `CheckAssetGlobs`), not in the shared helper, so GitLab is untouched; it checks the matched paths, so a `#` in a directory name is caught too. Failure is Runtime, before any commit or tag. Tests: GitHub unit (refused/accepted, `gh` never invoked), GitLab unit (accepted), one `release` e2e subtest; reverting the production change fails the e2e subtest and the unit test. Spec 05 updated. Publish-time resolution is unchanged (the gate always runs first).

**Phase 79 exit:** `go test -count=1 ./...` passes, each task confirmed to fail when its fix is reverted, and
lint is clean.
