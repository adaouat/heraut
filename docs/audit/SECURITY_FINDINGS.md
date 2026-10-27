# Security findings: heraut (2026-10-14)

## Update 2026-10-27 (after T425 to T428)

GitHub publishing no longer runs `gh`: `internal/platforms/github` calls the REST API in process through `go-github` (ADR-0077). This section supersedes the rows and the scorecard below for NEW-002, NEW-004 and SEC-004's residual. In-place update, not a re-scan.

- **NEW-002: RESOLVED for GitHub, still open for GitLab.** `platforms.OpenAssets` (`0434081`, `d062752`, T425) opens each asset once through an `os.Root` on the repository root, checks the descriptor with `os.SameFile` against the path that was checked, and refuses anything but a regular file; the GitHub driver uploads from those descriptors (`9301d41`, T427). The GitLab client still reads by path; T431 moves it onto the same helper.
- **NEW-004: RESOLVED, by removal.** The `path#label` syntax was `gh`'s. The asset name now travels as a query parameter, so a `#` in a path is accepted again; when GitHub publishes a file under a different name, heraut prints a warning (`9301d41`, T427). The ADR-0076 addendum's refusal is gone.
- **SEC-004 residual: CLOSED.** No `GH_HOST` is set any more. The API root is derived from `base_url` (`config.GitHubAPIURL`, T424), which must be https unless loopback, and the upload goes only to an origin expected for `base_url` (T426).
- **Still open:** SEC-002, NEW-002 for GitLab (T431), and the crash-leftover temp-file residual of NEW-005.

**Scorecard (current):** Critical 0 · High 0 · Medium 0 · Low 0 · Info 3 (SEC-002, NEW-002 for GitLab, NEW-005 temp-file residual). Coverage gaps from the 2026-10-22 section still apply.

## Update 2026-10-24 (after T421 and T422)

NEW-003 and NEW-004 are **resolved**; this section supersedes their rows and the scorecard in the update below. In-place update, not a re-scan.

- **NEW-003: RESOLVED** in `04767e7` (T421, ADR-0074 addendum). `tagfmt.ValidateVersionOverride` now accepts only `[0-9A-Za-z._+/-]` for every strategy, ahead of the per-strategy checks, so shell syntax and glob characters (`$(id)`, `;`, backtick, quotes, `*`, `{`) are Config errors (exit 2) before anything runs. Unit table, e2e rows, and sensitivity check (reverting fails 14 unit and 3 e2e rows).
- **NEW-004: RESOLVED** in `45ddc9a` (T422, ADR-0076 addendum). The GitHub driver's `CheckAssets` refuses a matched asset whose path contains `#` (a `#` in a directory name too), Runtime exit, before any commit or tag. GitLab is unaffected and covered by a test. Unit tests plus one `release` e2e subtest; reverting fails them.
- **Still open:** NEW-002 (Info, deliberately deferred until `gh` is replaced by an in-process client), SEC-002, and the crash-leftover temp-file residual of NEW-005.

**Scorecard (current):** Critical 0 · High 0 · Medium 0 · Low 0 · Info 3 (SEC-002, NEW-002, NEW-005 temp-file residual). Not scheduled: NEW-002 (deferred by decision). Coverage gaps from the 2026-10-22 section still apply.

## Update 2026-10-24 (after T419 and T420)

NEW-001 and NEW-005 are **resolved**; this section supersedes their rows and the scorecard below. It is an in-place update, not a re-scan.

- **NEW-001: RESOLVED** in `c8d76a7` (T419). `internal/forge/github/github.go` and `internal/forge/azure/azure.go` now wrap the client in `withTokenSafeRedirects`: `Authorization` is dropped when a redirect changes host or goes from https to http, and redirects stop after 10. A test per client (`redirect_internal_test.go`) was written first and fails without the clause.
- **NEW-005: RESOLVED** in `150d8da` (T420). `writeFileAtomic` keeps an existing file's permission bits (0644 only for a new file). Two unit tests and one e2e row. The crash-leftover temp file half of the finding is unchanged (Info; a hook that stages everything could commit it). The owner and group of an existing file are not preserved.
- **Fresh evidence (2026-10-22 to 2026-10-23):** `govulncheck ./...` reports no vulnerabilities on go1.27.3 (GO-2026-6809 is cleared); a security-auditor pass during the `assess` re-run found no hardcoded credential and no `InsecureSkipVerify`; its only open candidates were NEW-001 and NEW-005, both fixed above. That pass took SEC-001, 004, 005 and 009 from this document without re-reading `perenv/order.go`, `config/url.go` or `platforms/globs.go`.

**Scorecard (current):** Critical 0 · High 0 · Medium 0 · Low 0 · Info 4 (SEC-002, NEW-002, NEW-003, NEW-004, plus the temp-file residual of NEW-005). Not scheduled: NEW-002, NEW-003, NEW-004. Coverage gaps from the 2026-10-22 section still apply (no per-class finders or refuters ran).

## Re-scan 2026-10-22 (after the hardening epics)

Scope: `71acdc5..HEAD` (46 commits). Method: the Workflow tool was not available, so this was one adversarial **security-auditor** pass (read-only, no tests run) plus `govulncheck`; I then re-read the code behind NEW-001 and NEW-005 myself and they hold. `legacy/heraut` is a one-line text file holding the repo root path, not a symlink.

**Scorecard (current):** Critical 0 · High 0 · Medium 0 · Low 1 (NEW-001) · Info 5 (SEC-002 unchanged, NEW-002 to NEW-005). Credentials: none found (the history scan of 2026-10-14 was not repeated).

**Known and out of scope.** `govulncheck ./...` reports one issue, GO-2026-6809 (Go standard library `crypto/x509`, found in go1.27.2, fixed in go1.27.3). You said the CI workflow reports it too and another agent will bump the toolchain, so it is recorded here and not remediated. Nothing else in `go.mod` is flagged (`client-go` 1.46.0, `go-retryablehttp` 0.7.8, `x/net` 0.60.0, `yaml.v3` 3.0.1, `goldmark` 1.8.6).

**Earlier findings**

| ID | Status | Evidence |
|---|---|---|
| SEC-001 (hook values from unparsed tags) | RESOLVED | `internal/versioning/perenv/order.go:44-55`: `latestTag` returns only tags that parsed. T400 also restricts `--set-build-id` to `[0-9A-Za-z._-]` (`tagfmt.go:114-131`). |
| SEC-003 (token headers on cross-host redirect) | RESOLVED | `internal/forge/gitlab/gitlab.go:28-58`, `internal/platforms/gitlab/client.go:48-62`. Residual: NEW-001. |
| SEC-004 (`http://` base URLs, scheme in `sameGitLabHost`) | RESOLVED | `internal/config/url.go:12-25` enforced in the validator, in forge resolution and in the GitLab client; `sameGitLabHost` compares scheme and host. Residual: the `gh` driver sets `GH_HOST` from the host only, so the scheme is `gh`'s choice. |
| SEC-009 (asset globs not confined) | RESOLVED | `internal/platforms/globs.go:20-52`; the pre-push gate (`steps.go:76-93`) runs after `post_bump` and before any commit or push, and a pattern matching no file is now refused (T409). Residuals: NEW-002, NEW-004. |
| SEC-005 (leading `-` in asset names) | RESOLVED | defence in depth in `globs.go:106-111`. |
| SEC-002 (`--dir`, `--output` unconfined) | UNCHANGED (Info) | operator-driven. |

**New findings**

| ID | CWE | Sev | Location | Summary |
|---|---|---|---|---|
| NEW-001 | CWE-523 / 319 | Low | `internal/forge/github/github.go:27`, `internal/forge/azure/azure.go:27` | The GitHub and Azure enrichment clients are plain `http.Client{Timeout}` with no `CheckRedirect`. Go keeps `Authorization` on a same-host redirect, so an https to http downgrade by a compromised or misconfigured forge would send the token in clear. The same downgrade was closed for GitLab in T380/T381. Needs a network or forge adversary. Fix: reuse the GitLab `withTokenSafeRedirects` pattern. |
| NEW-002 | CWE-367 | Info | `internal/platforms/globs.go:36-44`, `internal/platforms/gitlab/client.go:123-130` | The confinement check and the later open/read are by path, so a symlink swapped in between escapes confinement. Needs a local party who can already write to the repo or run hooks. |
| NEW-003 | CWE-20 | Info | `internal/app/resolver.go:407-440` | `tagfmt.ValidateVersionOverride` only rejects empty or whitespace; shell safety of `--set-version` rests on the two per-strategy checks. No bypass found today; a future strategy that skipped both would reach hooks unfiltered. Optional character allowlist. |
| NEW-004 | CWE-73 | Info | `internal/platforms/github/platform.go:186` | `gh` treats `path#label` as a display-label separator, so a file name containing `#` changes the asset label. Cosmetic. |
| NEW-005 | CWE-732 / 459 | Info | `internal/generators/native/generator.go:496-525` | `writeFileAtomic` forces the caller's mode (0644) on the replaced CHANGELOG instead of keeping an existing stricter one, and a crash can leave a `.<name>.tmp-*` file that a hook staging everything could commit. |

Reviewed with no finding: `internal/config/forgeenv.go` (constants), GitHub token handling (env slices, no token in error text in the diff), `forge.GitOriginURL` (fixed argv), `IsSecureURL` (`localhost.evil.com`, userinfo, IPv6 loopback), `ValidateBuildID` on every path that takes the flags. No instruction-shaped text aimed at AI tools was seen.

**Coverage gaps.** (1) No per-class finders and no refuter or second judge ran: NEW-001 to NEW-005 are **not judged** beyond my re-read of NEW-001 and NEW-005. (2) The large pure refactors (the validator split, the wizard split, the pipeline file split) were skimmed, not checked for behavior change. (3) Not covered: the test suite, client-go's retry layer at runtime (`CheckRedirect` is set on `api.HTTPClient()` but not traced into it), Windows path semantics, the native template set beyond the earlier func-map finding, and the hook-quoting changes beyond confirming the inputs are validated. I can re-run just these.

**Remediation log (re-scan).** No Critical or High finding, so by this command's rule no patch was drafted and no patch review ran; `security_remediation.patch` still does not exist. Candidate roadmap items, not implemented and not on any roadmap until you say so: NEW-001 (Low, a small fix with a test per forge), NEW-005 (keep the existing file mode), and optionally NEW-003.

---

`legacy/heraut` points to the repository root. The Workflow tool was not available, so this used the fallback: two security-auditor passes (the second adversarially re-checked the first), plus my own reading of the cited code for the hook sink, `latestTag` and the GitLab client. The scan is static; nothing was executed against a live forge. `govulncheck ./...` reported no vulnerabilities.

## Scorecard
| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 0 (SEC-001 was claimed Medium and re-graded Low-Medium) |
| Low | 5 |
| Info | 3 |

Top CWEs: CWE-78 (hook command), CWE-522/200 (token headers), CWE-319/918 (base URL), CWE-22 (asset globs), CWE-88 (refuted).
Credentials: none found in the working tree. Git history scanned 2026-10-14 (see Coverage gaps): no secrets found. Nothing was refuted by tool, but one claim (SEC-005) was refuted by tracing and SEC-001 was downgraded; the first pass over-stated it.

## Coverage gaps
No Workflow run, so no per-class finders. Git history was scanned afterwards with a pattern scan of the added lines of all 1,248 commits reachable from any ref (183,951 added lines; private keys, AWS, GitHub, GitLab and Slack tokens, JWTs, URL credentials, and `password`/`secret`/`api_key`/`token` assignments of 16+ characters). The only matches were two documentation/workflow lines that reference environment variables (`${E2E_GITHUB_TOKEN}`, `${E2E_GITLAB_TOKEN}`, `${HOMEBREW_TAP_TOKEN}`), not values. The scan is regex-based (no entropy check), skips binary files and does not cover unreachable objects, the reflog or stashes. Not covered: runtime behaviour of client-go's retryable HTTP redirect handling (not verified), the full native template set beyond its func map, Windows paths. All findings below were judged by a second reading, not by a separate refuter per finding.

## Findings
| ID | CWE | Sev | Location | Verdict | Summary |
|---|---|---|---|---|---|
| SEC-001 | CWE-78 | Low-Medium (fixed 2026-10-15, T383, b28ce7b) | `internal/pipeline/hooks.go:36-46,167-173`; `internal/versioning/perenv/order.go:41-52`; `perenv/promote.go:213-229` | PARTIAL | Hook templates insert `{{.Tag}}`, `{{.PreviousTag}}`, `{{.Env}}` unquoted into `sh -c`. `Tag` is built by heraut from digit/alnum-only parts, `Env` must be a configured environment, and plain semver/calver paths only accept parsed tags. The exposed path is promote runs: `latestTag` returns `rawTags[0]` even when it does not parse (`order.go:51`), so `calver-per-env`, or `semver-per-env` with no parseable destination tag, can pass an attacker-pushed tag like `<prefix>9999$(cmd)-<env>` into `{{.PreviousTag}}`. Needs a hook using `PreviousTag` and a promote run. |
| SEC-003 | CWE-522 | Low | `internal/forge/gitlab/gitlab.go:26,62-64` | CONFIRMED mechanism | `http.Client` has no `CheckRedirect`; Go forwards custom headers (`PRIVATE-TOKEN`, `JOB-TOKEN`) on cross-host redirects. Needs a redirecting GitLab server. |
| SEC-004 | CWE-319 / 918 | Low | `internal/platforms/gitlab/platform.go:155-170,232-255`; `internal/forge/detect.go:57`; `internal/config/validator.go:775-781` | CONFIRMED | `base_url` / `CI_SERVER_URL` may be `http://` or any host; `sameGitLabHost` ignores the scheme. A PR that edits `base_url` can aim the token at another host (same trust boundary as hooks). |
| SEC-009 | CWE-22 | Low-Medium | `internal/platforms/globs.go`; `internal/platforms/gitlab/client.go:46-125` | CONFIRMED (new) | `release.assets` globs are not confined to the repo, so absolute paths or `..` could publish runner files as release assets. Config author trust only. |
| SEC-002 | CWE-22 / 73 | Info | `internal/cmd/skill.go`, `notes.go:74`, `init.go:123`, `config/sprint.go` | Info | `--dir`/`--output` unconfined; operator-driven. |
| SEC-005 | CWE-88 | Info | `app/notes.go:121`, `generators/native/commits.go:112`, `port/refs.go:11` | REFUTED as claimed | Tags starting with `-` are rejected or filtered upstream. Defence-in-depth only: an asset path starting with `-` passed to `gh` could read as an option. |
| SEC-006 | CWE-1336 | Info | `generators/native/funcs.go:11-20` | No finding | Func map is only `upperFirst, date, join, list, indent, trim`. |
| SEC-007/008 | CWE-377 / 732 | Info | temp files, file modes | No finding | `CreateTemp` 0600, removed afterwards. |

Design note (not a heraut bug): hooks are arbitrary shell by design (ADR-0053), so anyone who can change `.heraut.yml` already has code execution. Running heraut on an untrusted PR's config in CI with secrets is the real risk and is not documented.

## Remediation Log
No Critical or High finding, so by this command's rule no patch was drafted: `security_remediation.patch` and `.local.patch` do not exist, and no Patch Review ran. The Low and Low-Medium items are better fixed through the project's own TDD flow. Suggested minimal fixes, in priority order:
1. SEC-001: make `latestTag` return a tag only when it parsed; add a `shq` quoting template func (or pass values as `HERAUT_TAG` etc. env vars); document that CI must not run hooks from untrusted config.
2. SEC-004 / SEC-003: require `https` unless loopback; compare scheme and host in `sameGitLabHost`; add `CheckRedirect` that drops the token headers on a host change.
3. SEC-009: require assets to resolve under the repo root after `EvalSymlinks`; prefix `./` before passing paths to `gh`.

## Patch Review
Not applicable (no patch).

## Status updates
- 2026-10-14: SEC-003 fixed in T380 (commits `f826273`, `ba4b684`). Both the enrichment client and the client-go publish client were affected; GitHub and Azure enrichment were not (Authorization only, which net/http strips cross-domain). The client-go redirect question is closed. Still open: a same-host https to http redirect keeps the token (T381).
- 2026-10-15: SEC-004 fixed in T381 (commit 1868679). https is mandatory for forge `base_url`, `api_url`, `CI_SERVER_URL` and `CI_API_V4_URL`, with a loopback exception for local servers and tests; `sameGitLabHost` compares scheme and host. The same-host https to http redirect gap from T380 is closed. Not verified: where `gh` picks the scheme for GitHub Enterprise (`GH_HOST`).
- 2026-10-15: SEC-009 fixed in T382 (commit 5651dd1). Asset matches are resolved (absolute path, then symlinks) and refused unless under the working directory. Residual: the refusal happens at publish time, after the tag is pushed; an asset-glob preflight would close that.
- 2026-10-15: Phase 1 of the brief (SEC-003, SEC-004, SEC-009) is complete. SEC-001 (latestTag) is fixed too (T383, b28ce7b), with the rest of Phase 2.
- 2026-10-15: T388 documents the unquoted hook-variable contract and the untrusted-PR-config warning (Spec 02 § Quoting and trust, hooks guide, ADR-0053 clarification). Found while checking it: `calver-per-env` `--set-build-id` accepts shell syntax (only `/` and whitespace are rejected), so it reaches `Tag` in hooks; logged as T400. Low, operator-supplied input.
