# Rule reviews: heraut

All 72 business rules in `BUSINESS_RULES.md` have a verdict: 60 confirmed, 12 wrong or not wanted, 0 needing discussion. The 2026-10-14 verdicts were implemented under the hardening roadmap (T380-T401); the later pass added the rest, and RULE-003, 013, 015, 032, 039, 052 and 065 were then discussed and settled (notes give the decision). Source paths in the cards predate the refactor (branch rules now live in `internal/versioning/branch`, the release flow in `internal/pipeline/steps.go`).

| Rule | Verdict | When | Title | Note |
|---|---|---|---|---|
| RULE-001 | Right | 2026-10-14 | Commit-to-bump level |  |
| RULE-002 | Wrong / not wanted | 2026-10-16 | SemVer arithmetic |  |
| RULE-003 | Right | 2026-10-16 | Release base tag and ordering | Build metadata is ignored in ordering (SemVer 11); a v1.4.0 / v1.4.0+5 tie is harmless. |
| RULE-004 | Right | 2026-10-16 | Initial version |  |
| RULE-005 | Right | 2026-10-16 | Pre-release counter |  |
| RULE-006 | Right | 2026-10-16 | Version-branch fixed version |  |
| RULE-007 | Wrong / not wanted | 2026-10-14 | CalVer date values | use UTC |
| RULE-008 | Wrong / not wanted | 2026-10-14 | CalVer PATCH and period reset | highest parsed value |
| RULE-009 | Right | 2026-10-16 | CalVer rendering and padding |  |
| RULE-010 | Right | 2026-10-14 | Tag format render and parse |  |
| RULE-011 | Right | 2026-10-16 | Per-env auto bump |  |
| RULE-012 | Right | 2026-10-16 | Changelog rotation buckets |  |
| RULE-013 | Wrong / not wanted | 2026-10-20 | Commit classification into sections | The built-in Security fallback (unconfigured types only) should match the whole word "security", case-insensitively, not a lowercase substring of the body. Keep the section order and document it in Spec 05. Today only the body is checked; whether the subject counts is to be settled in the task. |
| RULE-014 | Right | 2026-10-16 | Release-note day statistics |  |
| RULE-015 | Wrong / not wanted | 2026-10-20 | Sprint bump | Edit sprint only inside the top-level versioning block: replace or insert there, tolerate a trailing comment on the versioning: line, leave other sprint: keys alone. Test-first, no ADR. |
| RULE-016 | Wrong / not wanted | 2026-10-16 | Config must validate before a run |  |
| RULE-017 | Right | 2026-10-16 | Strict load and removed keys |  |
| RULE-018 | Right | 2026-10-16 | Resolvable publish destination |  |
| RULE-019 | Right | 2026-10-16 | Forge entry validity and uniqueness |  |
| RULE-020 | Right | 2026-10-16 | One target per forge |  |
| RULE-021 | Right | 2026-10-16 | Enrichment forge and policy values |  |
| RULE-022 | Right | 2026-10-16 | Per-env structure and promote source |  |
| RULE-023 | Right | 2026-10-16 | Promotion cycle detection |  |
| RULE-024 | Right | 2026-10-16 | Per-env tag_format and `{build}` placement |  |
| RULE-025 | Right | 2026-10-16 | CalVer, bump and template config validation |  |
| RULE-026 | Right | 2026-10-16 | Branches config validation |  |
| RULE-027 | Right | 2026-10-16 | No releasable commits |  |
| RULE-028 | Right | 2026-10-16 | Pre-release label and flag usage |  |
| RULE-029 | Right | 2026-10-16 | Pre-release monotonicity and escalation |  |
| RULE-030 | Right | 2026-10-16 | Pre-release needs a commit unless promoting |  |
| RULE-031 | Right | 2026-10-16 | CalVer format parsing |  |
| RULE-032 | Wrong / not wanted | 2026-10-20 | Version override flag combinations | Validate --set-version per strategy: SemVer for semver/semver-per-env (v prefix optional), the configured CalVer format for calver/calver-per-env; else a Config error. Reverses the strategy-agnostic override in tagfmt.ValidateVersionOverride, so it needs an ADR addendum (ADR-0074). |
| RULE-033 | Right | 2026-10-16 | `--json` requires `--dry-run` |  |
| RULE-034 | Right | 2026-10-16 | Conventional commit grammar |  |
| RULE-035 | Right | 2026-10-16 | Commit type and scope allow-list |  |
| RULE-036 | Right | 2026-10-16 | Commit rules (deny, require, require_ticket) |  |
| RULE-037 | Right | 2026-10-16 | Commit rules and tickets config validation |  |
| RULE-038 | Right | 2026-10-16 | Git preflight and identity |  |
| RULE-039 | Wrong / not wanted | 2026-10-16 | Asset glob handling | A glob matching nothing at publish time should fail, unless the entry is marked optional. Needs an ADR. Repo confinement (SEC-009) is already fixed. |
| RULE-040 | Right | 2026-10-16 | Release pre-flight gates (skipped in dry-run) |  |
| RULE-041 | Right | 2026-10-16 | stay_at_v0 hold |  |
| RULE-042 | Right | 2026-10-16 | Maintenance range |  |
| RULE-043 | Right | 2026-10-16 | Promotion source env |  |
| RULE-044 | Right | 2026-10-16 | Promotion guard E001: target tag exists |  |
| RULE-045 | Right | 2026-10-16 | Promotion guard E002: destination ahead |  |
| RULE-046 | Wrong / not wanted | 2026-10-14 | Promotion guard E003: no source tags | run probe for an existing tag |
| RULE-047 | Right | 2026-10-16 | Env branch guard |  |
| RULE-048 | Right | 2026-10-16 | Release branch allow-list |  |
| RULE-049 | Right | 2026-10-16 | `--env auto` resolution |  |
| RULE-050 | Right | 2026-10-16 | Tag type selection |  |
| RULE-051 | Right | 2026-10-16 | Pre-release flag only under SemVer |  |
| RULE-052 | Wrong / not wanted | 2026-10-16 | Default commit excludes | Keep excludes render-only. Warn when every releasable commit is excluded from the changelog (empty-changelog release). No bump change. |
| RULE-053 | Right | 2026-10-16 | `--offline` override |  |
| RULE-054 | Right | 2026-10-16 | Enrichment policy |  |
| RULE-055 | Right | 2026-10-14 | Hook validation and execution safety | leave quoting to the hook author and document it, also warn CI not to run hooks from an untrusted PR's config |
| RULE-056 | Right | 2026-10-16 | Exit-code mapping |  |
| RULE-057 | Right | 2026-10-16 | Release step order |  |
| RULE-058 | Right | 2026-10-14 | No rollback on failure | can we write a doc to help revert cleanly or is it too complex ("casse gueule" in french)? \| Decision: document it (failure-recovery guide), no `heraut recover` command; no automatic rollback. |
| RULE-059 | Right | 2026-10-16 | Dry-run executes nothing |  |
| RULE-060 | Right | 2026-10-16 | Changelog commit and push are idempotent |  |
| RULE-061 | Right | 2026-10-14 | Tag pushed before post_tag and publish |  |
| RULE-062 | Right | 2026-10-16 | Per-platform publish and hook isolation |  |
| RULE-063 | Right | 2026-10-16 | Notes generated once or per platform |  |
| RULE-064 | Right | 2026-10-16 | Hook points and template variables |  |
| RULE-065 | Wrong / not wanted | 2026-10-20 | Hook stage patterns feed the release commit | Declaring hook `stage` while the changelog is disabled (globally or for the env) becomes a config validation error, consistent with ADR-0061. Needs a validator rule, Spec 02 note and ADR-0061 addendum. |
| RULE-066 | Right | 2026-10-16 | `--no-hooks` and `--skip-hooks` |  |
| RULE-067 | Right | 2026-10-16 | Changelog-only flow |  |
| RULE-068 | Right | 2026-10-16 | `NoPush` semantics |  |
| RULE-069 | Wrong / not wanted | 2026-10-14 | Incremental changelog splice | the write should be atomic (temp file plus rename) |
| RULE-070 | Right | 2026-10-16 | `--json` envelope |  |
| RULE-071 | Right | 2026-10-16 | `release --dry-run --json` plan |  |
| RULE-072 | Right | 2026-10-16 | Degraded-run and resolver warnings |  |
