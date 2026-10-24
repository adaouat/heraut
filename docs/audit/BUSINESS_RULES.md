# Business rules: heraut

Extracted 2026-10-14 with the direct-subagent method (the Workflow tool was not available): three extractors (calculations, validations and eligibility, state and lifecycle), merged and de-duplicated, then spot-checked by me against the code. `legacy/heraut` points to the repo root; all paths are relative to it. Production code only.

**Verification.** I read the cited lines for about 20 of the 72 rules (SemVer arithmetic, stay_at_v0, CalVer dates, the pipeline order, git commit/tag/push, the pre-release flag, the branch guard, exit codes, the default excludes, sprint, day stats, and the promotion and splice code). Two citations were wrong and are corrected here (RULE-052 default excludes, RULE-069 changelog splice range). The remaining rules rest on the extractors' own reading; their confidence is as stated. No rule here was kept on the strength of a comment alone. No instruction-shaped text was found in the source. Exit-code numbers other than `Promotion = 4` live in the external `forge/exitcode` library and were not read.

**Dropped / folded.** The extractors overlapped heavily: promotion guards (3 reports), branch guard (2), `--offline` (2), exit codes (2), enrichment policy (2), hooks (2) and pre-release labels (2) were each folded into one rule. Nothing was refuted outright; items the extractors could not source (no clean-tree guard, no generic "tag exists" guard for plain semver) are listed as gaps at the end.

## Summary

| ID | Name | Category | Pri | Source | Conf |
|---|---|---|---|---|---|
| RULE-001 | Commit-to-bump level | Calculation | P0 | `internal/versioning/semver/bump.go:55-117` | High |
| RULE-002 | SemVer arithmetic | Calculation | P0 | `internal/versioning/semver/bump.go:119-152` | High |
| RULE-003 | Release base tag and ordering | Calculation | P0 | `internal/versioning/semver/resolver.go:140-194` | High |
| RULE-004 | Initial version | Policy | P0 | `internal/versioning/semver/resolver.go:157-208` | High |
| RULE-005 | Pre-release counter | Calculation | P0 | `internal/versioning/semver/prerelease.go:56-140` | High |
| RULE-006 | Version-branch fixed version | Calculation | P0 | `internal/versioning/semver/fixed.go:24-103` | High |
| RULE-007 | CalVer date values | Calculation | P0 | `internal/versioning/calver/resolver.go:131-150` | High |
| RULE-008 | CalVer PATCH and period reset | Calculation | P0 | `internal/versioning/calver/resolver.go:27-110` | Medium |
| RULE-009 | CalVer rendering and padding | Calculation | P0 | `internal/versioning/calver/parser.go:175-196` | High |
| RULE-010 | Tag format render and parse | Calculation | P0 | `internal/versioning/tagfmt/tagfmt.go:37-79` | High |
| RULE-011 | Per-env auto bump | Calculation | P0 | `internal/versioning/perenv/auto.go:13-63` | High |
| RULE-012 | Changelog rotation buckets | Calculation | P1 | `internal/versioning/calver/rotation.go:45-156` | High |
| RULE-013 | Commit classification into sections | Calculation | P1 | `internal/generators/native/group.go:50-162` | High |
| RULE-014 | Release-note day statistics | Calculation | P2 | `internal/generators/native/templatemodel.go:296-334` | High |
| RULE-015 | Sprint bump | Calculation | P1 | `internal/config/sprint.go:13-52` | High |
| RULE-016 | Config must validate before a run | Validation | P0 | `internal/cmd/release.go:72-82` | High |
| RULE-017 | Strict load and removed keys | Validation | P1 | `internal/config/loader.go:60-215` | High |
| RULE-018 | Resolvable publish destination | Validation | P0 | `internal/cmd/release.go:89-93` | High |
| RULE-019 | Forge entry validity and uniqueness | Validation | P1 | `internal/config/validator.go:247-321` | High |
| RULE-020 | One target per forge | Validation | P0 | `internal/config/validator.go:366-426` | High |
| RULE-021 | Enrichment forge and policy values | Validation | P1 | `internal/config/validator.go:323-345` | High |
| RULE-022 | Per-env structure and promote source | Validation | P1 | `internal/config/validator.go:895-1035` | High |
| RULE-023 | Promotion cycle detection | Validation | P1 | `internal/config/validator.go:1037-1083` | High |
| RULE-024 | Per-env tag_format and `{build}` placement | Validation | P1 | `internal/config/validator.go:845-893` | High |
| RULE-025 | CalVer, bump and template config validation | Validation | P1 | `internal/config/validator.go:567-766` | High |
| RULE-026 | Branches config validation | Validation | P1 | `internal/config/validator.go:671-718` | High |
| RULE-027 | No releasable commits | Validation | P0 | `internal/versioning/semver/resolver.go:174-182` | High |
| RULE-028 | Pre-release label and flag usage | Validation | P1 | `internal/versioning/semver/prerelease.go:34-47` | High |
| RULE-029 | Pre-release monotonicity and escalation | Validation | P0 | `internal/versioning/semver/prerelease.go:227-269` | High |
| RULE-030 | Pre-release needs a commit unless promoting | Policy | P1 | `internal/versioning/semver/prerelease.go:167-188` | High |
| RULE-031 | CalVer format parsing | Validation | P0 | `internal/versioning/calver/parser.go:47-120` | High |
| RULE-032 | Version override flag combinations | Validation | P1 | `internal/cmd/version_override.go:13-30` | High |
| RULE-033 | `--json` requires `--dry-run` | Validation | P1 | `internal/cmd/release.go:42-44` | High |
| RULE-034 | Conventional commit grammar | Validation | P1 | `internal/conventionalcommit/conventionalcommit.go:20-90` | High |
| RULE-035 | Commit type and scope allow-list | Validation | P1 | `internal/app/commit.go:49-109` | High |
| RULE-036 | Commit rules (deny, require, require_ticket) | Validation | P1 | `internal/app/commit.go:111-185` | High |
| RULE-037 | Commit rules and tickets config validation | Validation | P1 | `internal/config/validator.go:67-228` | High |
| RULE-038 | Git preflight and identity | Validation | P1 | `internal/app/check.go:22-35` | High |
| RULE-039 | Asset glob handling | Validation | P1 | `internal/platforms/globs.go:9-34` | High |
| RULE-040 | Release pre-flight gates (skipped in dry-run) | Validation | P0 | `internal/cmd/release.go:72-134` | High |
| RULE-041 | stay_at_v0 hold | Policy | P0 | `internal/versioning/semver/hold.go:18-39` | High |
| RULE-042 | Maintenance range | Policy | P0 | `internal/versioning/semver/maintenance.go:26-111` | High |
| RULE-043 | Promotion source env | Policy | P1 | `internal/versioning/perenv/promote.go:235-265` | High |
| RULE-044 | Promotion guard E001: target tag exists | Policy | P0 | `internal/versioning/perenv/promote.go:165-178` | High |
| RULE-045 | Promotion guard E002: destination ahead | Policy | P0 | `internal/versioning/perenv/promote.go:180-211` | High |
| RULE-046 | Promotion guard E003: no source tags | Policy | P0 | `internal/versioning/perenv/promote.go:140-155` | Medium |
| RULE-047 | Env branch guard | Policy | P0 | `internal/app/branch.go:23-45` | High |
| RULE-048 | Release branch allow-list | Policy | P0 | `internal/app/branchrules.go:46-143` | High |
| RULE-049 | `--env auto` resolution | Policy | P1 | `internal/app/env.go:17-62` | High |
| RULE-050 | Tag type selection | Policy | P0 | `internal/pipeline/git.go:98-108` | High |
| RULE-051 | Pre-release flag only under SemVer | Policy | P0 | `internal/pipeline/release.go:376-387` | High |
| RULE-052 | Default commit excludes | Policy | P1 | `internal/config/commits.go:198-206` | High |
| RULE-053 | `--offline` override | Policy | P1 | `internal/cmd/offline.go:12-22` | High |
| RULE-054 | Enrichment policy | Policy | P1 | `internal/generators/native/enrich.go:48-79` | Medium |
| RULE-055 | Hook validation and execution safety | Policy | P0 | `internal/pipeline/hooks.go:159-184` | Medium |
| RULE-056 | Exit-code mapping | Policy | P1 | `internal/cmd/exit.go:35-49` | High |
| RULE-057 | Release step order | Lifecycle | P0 | `internal/pipeline/release.go:190-374` | High |
| RULE-058 | No rollback on failure | Lifecycle | P0 | `internal/pipeline/release.go:219-283` | High |
| RULE-059 | Dry-run executes nothing | Lifecycle | P0 | `internal/pipeline/release.go:389-492` | High |
| RULE-060 | Changelog commit and push are idempotent | Lifecycle | P0 | `internal/pipeline/git.go:55-74` | High |
| RULE-061 | Tag pushed before post_tag and publish | Lifecycle | P0 | `internal/pipeline/release.go:257-283` | High |
| RULE-062 | Per-platform publish and hook isolation | Lifecycle | P0 | `internal/pipeline/release.go:316-370` | High |
| RULE-063 | Notes generated once or per platform | Lifecycle | P1 | `internal/pipeline/release.go:285-342` | High |
| RULE-064 | Hook points and template variables | Lifecycle | P1 | `internal/pipeline/hooks.go:12-107` | High |
| RULE-065 | Hook stage patterns feed the release commit | Lifecycle | P1 | `internal/pipeline/release.go:222-244` | High |
| RULE-066 | `--no-hooks` and `--skip-hooks` | Policy | P1 | `internal/pipeline/hooks.go:84-89` | Medium |
| RULE-067 | Changelog-only flow | Lifecycle | P0 | `internal/pipeline/changelog.go:194-315` | High |
| RULE-068 | `NoPush` semantics | Policy | P1 | `internal/pipeline/changelog.go:265-346` | High |
| RULE-069 | Incremental changelog splice | Lifecycle | P0 | `internal/generators/native/generator.go:282-490` | High |
| RULE-070 | `--json` envelope | Policy | P2 | `internal/cmd/jsonout.go:12-69` | High |
| RULE-071 | `release --dry-run --json` plan | Policy | P2 | `internal/pipeline/plan.go:30-94` | High |
| RULE-072 | Degraded-run and resolver warnings | Policy | P2 | `internal/pipeline/warn.go:14-44` | High |

## Calculation

### RULE-001: Commit-to-bump level
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/semver/bump.go:55-117`
**Plain English:** The next release bump is the highest level any commit contributes.
**Specification:**
  Given commits `fix: a`, `feat: b`, `chore: c` on top of tag 1.2.3
  When  the version resolves
  Then  the bump is minor (1.3.0); a breaking commit gives major; a non-conventional commit gives nothing
**Parameters:** defaults breaking=major, `feat`=minor, any other conventional type=patch; override rule fields `type`, `regex` (on the subject), `breaking`, `bump` (major/minor/patch/none); first matching override per commit wins; a rule matches only when every condition it sets holds.
**Edge cases handled:** a rule with `type` never matches a non-conventional commit; an unknown `bump` string becomes none; breaking = `!` or a `BREAKING CHANGE`/`BREAKING-CHANGE` footer.
**Suspected defect:** an override rule whose regex fails to compile is silently dropped (`bump.go:26-40`), and `Parse` rejects a commit whose second line is not blank, so `feat: x` followed directly by a body line contributes no bump.
**Confidence:** High — read by the extractor; the silent drop of a bad regex is also checked by config validation (RULE-025), so the first defect is only reachable if validation is bypassed. Question for a maintainer: is rejecting a commit with a non-blank second line intended?

### RULE-002: SemVer arithmetic
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/semver/bump.go:119-152`
**Plain English:** Major resets minor and patch, minor resets patch, patch adds one; numbers are integers, so 1.9.0 becomes 1.10.0.
**Specification:**
  Given a current version `MAJOR.MINOR.PATCH` and a bump level
  When  the bump is applied
  Then  major → (M+1).0.0, minor → M.(m+1).0, patch → M.m.(p+1), none → unchanged
**Parameters:** `strings.SplitN(current, ".", 3)`, `strconv.Atoi` per part.
**Edge cases handled:** fewer than three parts is an error; a suffix such as `3-rc.1` fails `Atoi`.
**Suspected defect:** `Atoi` accepts leading zeros and a sign, so `01.2.3` is not rejected here.
**Confidence:** High — verified against the code in this session.

### RULE-003: Release base tag and ordering
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/semver/resolver.go:140-194`
**Plain English:** The base for the next version is the highest final release by SemVer 2.0 precedence, not git's tag order.
**Specification:**
  Given tags `v1.4.0-rc.1`, `v1.3.0`, `v1.4.0+5`
  When  the next version resolves
  Then  the base is `v1.4.0+5` and the next version is computed from 1.4.0; commits are read from `<tag>..HEAD`
**Parameters:** tag prefix default `v` (`resolver.go:13`), overridden by `TagPrefix` (a non-nil empty string means no prefix); precedence in `semver/version.go:86-110`.
**Edge cases handled:** pre-releases are never the base; build-metadata-only tags count as the release of their core; numeric identifiers sort below alphanumeric; a longer pre-release list wins when shared identifiers are equal; tags that do not parse as strict SemVer are dropped silently.
**Confidence:** High — read by the extractor.

### RULE-004: Initial version
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/semver/resolver.go:157-208`
**Plain English:** With no earlier final tag, the first version is the configured initial version, else 0.1.0.
**Specification:**
  Given a repository with no final `v*` tag
  When  the version resolves
  Then  the result is `versioning.initial_version`, else `0.1.0`, with bump none and no commit analysis
**Parameters:** default `0.1.0`.
**Edge cases handled:** a repository with only pre-release tags takes the same path.
**Confidence:** High.

### RULE-005: Pre-release counter
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/semver/prerelease.go:56-140`
**Plain English:** A pre-release is the next core plus `-<label>.<N>`, where N is one more than the highest existing counter for that label on that core.
**Specification:**
  Given final 1.2.0, a `feat` commit, tags `v1.3.0-rc.1` and `v1.3.0-rc.2`, label `rc`
  When  releasing a pre-release
  Then  the tag is `v1.3.0-rc.3`; with no earlier rc the tag is `v1.3.0-rc.1`
**Parameters:** shape `label.<n>`, n without a leading zero.
**Edge cases handled:** only tags of exactly that shape count; the counter is global, not scoped to the maintenance range; with no final tag the core is the initial version.
**Confidence:** High.

### RULE-006: Version-branch fixed version
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/semver/fixed.go:24-103`
**Plain English:** On a version branch the core comes from the branch name and commits never change it.
**Specification:**
  Given branch version 2.1.0 and tag `v2.1.0` already exists
  When  resolving a final release
  Then  it fails ("already released… pass --set-version … --set-build-id …") unless `allowReleased` is set
**Parameters:** a pre-release gets the next counter and must sort above every tag of the core.
**Edge cases handled:** a warning is emitted when a release with a higher core is already in the branch history; `allowReleased` serves changelog runs without a tag.
**Confidence:** High.

### RULE-007: CalVer date values
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/calver/resolver.go:131-150`
**Plain English:** Quarter, semester and ISO week come from the run date; the year is the ISO year when the format has a week token.
**Specification:**
  Given the date 2025-12-29 and format `YYYY.WW.PATCH`
  When  values are extracted
  Then  year 2026 and week 01 (calendar year would reuse 2025.01.0)
**Parameters:** quarter `(month-1)/3+1`, semester `(month-1)/6+1`, sprint from `versioning.sprint`.
**Edge cases handled:** ISO year only when `WW` is in the format.
**Confidence:** High — verified in this session. Question for a maintainer: which time zone does the `now` clock use (no normalisation in this file)?

### RULE-008: CalVer PATCH and period reset
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/calver/resolver.go:27-110`
**Plain English:** PATCH restarts at 0 in a new period and otherwise is the latest tag's PATCH plus one.
**Specification:**
  Given latest tag `2026.05.3` and format `YYYY.MM.PATCH`
  When  run in May 2026 → `2026.05.4`; run in June 2026 → `2026.06.0`
  Then  PATCH is previous+1 only when every non-PATCH token renders the same value
**Parameters:** period key built from rendered non-PATCH tokens.
**Edge cases handled:** a PATCH-only format has an empty period key and increments forever.
**Suspected defect:** the latest tag is the first parsable tag in git `--sort=-version:refname` order (lines 36-52), not the highest by calendar. For formats whose tokens do not sort lexically as versions (for example SPRINT first, or zero-padded fields) the wrong base could be chosen and an existing tag re-issued.
**Confidence:** Medium — the sort behaviour of git for such tags was not tested. Question for a maintainer: is the tag order guaranteed correct for all supported CalVer formats?

### RULE-009: CalVer rendering and padding
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/calver/parser.go:175-196`
**Plain English:** Year pads to four digits, month/day/week to two, the rest are not padded.
**Specification:**
  Given 2026-03-04 and format `YYYY.MM.DD.PATCH`
  When  the version renders
  Then  `2026.03.04.0`
**Parameters:** YYYY 4, MM/DD/WW 2; QQ, SS, SPRINT, PATCH unpadded.
**Confidence:** High.

### RULE-010: Tag format render and parse
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/tagfmt/tagfmt.go:37-79`
**Plain English:** A tag is built from a template with `{version}`, `{env}` and `{build}`, and can be parsed back.
**Specification:**
  Given template `{env}/{version}+{build}`, env `uat`, version `7.4.1`, build `158404`
  When  rendered
  Then  `uat/7.4.1+158404`
**Parameters:** the template must contain `{version}`; parsing matches `{version}` as `.+` and `{env}`/`{build}` as `[^/]+`, anchored; glob form replaces `{version}` and `{build}` with `*` (`tagfmt.go:157-165`).
**Edge cases handled:** `{build}` with an empty build ID is `ErrBuildIDRequired`; parsing is ambiguous when `{version}` is adjacent to `{build}`.
**Confidence:** High. Question for a maintainer: is an env containing `/` intentionally unsupported?

### RULE-011: Per-env auto bump
**Category:** Calculation
**Priority:** P0
**Source:** `internal/versioning/perenv/auto.go:13-63`
**Plain English:** An auto environment computes its next version from its own latest release tag.
**Specification:**
  Given env `dev`, tag format `{version}-dev`, latest tag `1.2.3-dev` and a `feat` since then
  When  the env resolves
  Then  `1.3.0`, tag `1.3.0-dev`
**Parameters:** semver-per-env orders by SemVer §11 (`perenv/order.go:16-25`); calver-per-env keeps git order and a lenient dotted-integer check (`order.go:27-34`).
**Edge cases handled:** with no env tags the calculator returns the initial version; pre-releases are never the base.
**Confidence:** High.

### RULE-012: Changelog rotation buckets
**Category:** Calculation
**Priority:** P1
**Source:** `internal/versioning/calver/rotation.go:45-156`
**Plain English:** Changelog files can rotate per period; the bucket pattern comes from a prefix of the version format.
**Specification:**
  Given format `YYYY.MM.PATCH`, rotation `[YYYY]`, version `2026.05.1`
  When  the bucket is computed
  Then  pattern `^2026\.`; rotation `[MM]` alone is rejected as "not a prefix"
**Parameters:** SemVer counterpart (`semver/rotation.go:37-57`): MAJOR or MAJOR+MINOR (patterns `^<M>\.` and `^<M>\.<m>\.`); MINOR alone is rejected.
**Edge cases handled:** order of requested tokens is irrelevant; duplicates rejected; a separator literal must follow.
**Confidence:** High.

### RULE-013: Commit classification into sections
**Category:** Calculation
**Priority:** P1
**Source:** `internal/generators/native/group.go:50-162`
**Plain English:** Each commit goes into the changelog section of its type.
**Specification:**
  Given `feat(api): x`, `fix: y`, `chore(ci): z`, `build: w`
  When  grouped
  Then  sections Features, Bug Fixes, Miscellaneous Tasks, Other in that order
**Parameters:** defaults in `internal/config/commits.go:146-160`: feat 0, fix 1, refactor 2, docs 3, perf 4, style 5, test 6, chore and ci 7; unordered types 100; Security 101; Revert 102; Other 103. Scoped commits first, by scope ascending, then unscoped in input order.
**Edge cases handled:** merge, `fixup!`, `squash!` and exclude-matching commits dropped; a user type with the same name replaces the default; `remove:true` drops it; unconfigured type with the word "security" in the body → Security (case-sensitive substring).
**Suspected defect:** the Security test is a plain substring ("insecurity" matches); it applies only to unconfigured types.
**Confidence:** High.

### RULE-014: Release-note day statistics
**Category:** Calculation
**Priority:** P2
**Source:** `internal/generators/native/templatemodel.go:296-334`
**Plain English:** The commit timespan and days since the previous release are whole days, truncated.
**Specification:**
  Given commits 36 hours apart
  When  stats are computed
  Then  timespan is 1 day (`int(Hours()/24)`)
**Edge cases handled:** days-since-previous is shown only when a previous release date exists.
**Confidence:** High — verified in this session.

### RULE-015: Sprint bump
**Category:** Calculation
**Priority:** P1
**Source:** `internal/config/sprint.go:13-52`
**Plain English:** `version sprint bump` adds one to the sprint counter in the config file, in place.
**Specification:**
  Given `sprint: 4` (or no key)
  When  `heraut version sprint bump` runs
  Then  the file holds 5 (or 1); `--dry-run` prints "would bump sprint N -> N+1" and writes nothing
**Parameters:** replace regex `(?m)^(\s+sprint:\s*)\d+`; otherwise insert after a bare `versioning:` line (`^versioning:\s*$`), copying the next non-blank line's indent (default 2); file mode 0644.
**Edge cases handled:** formatting is preserved; failures give the Config exit code.
**Suspected defect:** the replace regex matches every indented `sprint:` line, so an unrelated `sprint:` key elsewhere (for example a per-env block) would be bumped too; the insert fails if `versioning:` has a trailing comment.
**Confidence:** High.

## Validation

### RULE-016: Config must validate before a run
**Category:** Validation
**Priority:** P0
**Source:** `internal/cmd/release.go:72-82`
**Plain English:** A release or changelog run stops with a config error if the config fails any semantic check; all errors are reported together.
**Specification:**
  Given a config with `versioning.strategy: foo`
  When  `heraut release` runs
  Then  all errors are printed, exit code is Config, nothing is tagged
**Parameters:** strategies semver, calver, semver-per-env, calver-per-env; `version` must be "1"; `tag_type` annotated or lightweight; `bump.mode` auto or manual; platforms github, gitlab, azure_devops; `api_mode` rest, graphql or empty.
**Edge cases handled:** validation runs after the `--offline` override (RULE-053).
**Suspected defect:** `validateEnums` iterates the environments map unsorted, so error order is non-deterministic.
**Confidence:** High.

### RULE-017: Strict load and removed keys
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/loader.go:60-215`
**Plain English:** Unknown keys and removed keys fail the load, with a migration hint for removed ones.
**Specification:**
  Given a config containing `changelog.generator`
  When  it loads
  Then  it fails with `ErrRemovedConfigKey` and the hint
**Parameters:** removed keys include `changelog.remote`, `commits.remote_metadata` (renamed `enrichment_policy`), `release.platforms`, `changelog.generator/config`, `release.notes.generator/config`, top-level `rendering.trailers`, `release.targets[i].prerelease`, `environments.<e>.disable_notes`, and per-env variants.
**Edge cases handled:** `rendering.trailers` under changelog or notes gets only the generic strict-decode error.
**Confidence:** High.

### RULE-018: Resolvable publish destination
**Category:** Validation
**Priority:** P0
**Source:** `internal/cmd/release.go:89-93`
**Plain English:** `release` needs at least one place it can really publish to.
**Specification:**
  Given no `release.targets` and an azure_devops-only forge
  When  `heraut release` runs
  Then  it fails with the Config code before any write
**Parameters:** publish drivers exist only for `github` and `gitlab` (`internal/app/platforms.go:70-77`); auto-detection from CI env or git origin (`internal/app/pipeline.go:228-240`).
**Edge cases handled:** a forge resolution error counts as "not resolvable"; `heraut changelog` is the tag-only alternative.
**Confidence:** High.

### RULE-019: Forge entry validity and uniqueness
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:247-321`
**Plain English:** Each forge needs a unique name, a valid platform and absolute http(s) URLs, and no two forges may point at the same destination.
**Specification:**
  Given two `platform: github` forges with no repository
  When  validated
  Then  the second reports "resolves to the same destination as forges[0]"
**Parameters:** destination key (platform, base_url, project, repository).
**Edge cases handled:** an entry with an invalid platform is skipped in the duplicate scan.
**Confidence:** High.

### RULE-020: One target per forge
**Category:** Validation
**Priority:** P0
**Source:** `internal/config/validator.go:366-426`
**Plain English:** A release target must name a known forge, and two targets may not resolve to the same forge.
**Specification:**
  Given `targets: [{}, {}]` and no forges
  When  validated
  Then  "more than one target resolves to the same forge"
**Parameters:** applies to `release.targets` and each `environments.<e>.release.targets`; with no forges every bare target maps to the sentinel `\x00auto-detected`; `forge` is required when more than one forge exists.
**Why it matters:** the second `release create` would fail after the tag is already pushed (RULE-061).
**Edge cases handled:** a per-env target list replaces the root list, so the check is per list.
**Confidence:** High.

### RULE-021: Enrichment forge and policy values
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:323-345`
**Plain English:** The enrichment policy and forge names must be valid, and the forge is mandatory when several are configured.
**Specification:**
  Given two forges and no `enrichment_forge`
  When  validated
  Then  "required when more than one forge is configured"
**Parameters:** `commits.enrichment_policy` ∈ {"", disabled, optional, required}.
**Confidence:** High.

### RULE-022: Per-env structure and promote source
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:895-1035`
**Plain English:** Environments exist only for the per-env strategies, and each needs a valid bump mode and source.
**Specification:**
  Given strategy `semver` and an `environments` block
  When  validated
  Then  "only valid with semver-per-env or calver-per-env"
**Parameters:** `bump` ∈ {auto, promote}; `source` is invalid on auto envs; a promote env's `source` must exist and not be itself; with no `source` exactly one auto env must exist; `disable_changelog` with a `changelog` override and `disable_release` with a `release` override are rejected (`validator.go:1013-1035`).
**Confidence:** High.

### RULE-023: Promotion cycle detection
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:1037-1083`
**Plain English:** Promotion source links must not form a cycle.
**Specification:**
  Given prod.source=staging and staging.source=prod
  When  validated
  Then  one error "cycle detected (prod → staging → prod)"
**Edge cases handled:** one error per cycle; the resolver also guards self-reference at runtime (`perenv/promote.go:233-247`).
**Confidence:** High.

### RULE-024: Per-env tag_format and `{build}` placement
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:845-893`
**Plain English:** A tag format must contain `{version}`, and `{build}` may only follow a single `+`.
**Specification:**
  Given `tag_format: "{version}-{build}"`
  When  validated
  Then  `{build}` must directly follow "+"
**Edge cases handled:** `++{build}` is rejected; each env needs its own or a common format; the init wizard reuses these functions.
**Confidence:** High.

### RULE-025: CalVer, bump and template config validation
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:567-766`
**Plain English:** CalVer needs a format, SPRINT needs a sprint, bump overrides must be well-formed, and templates must parse.
**Specification:**
  Given format `YYYY.SPRINT.PATCH` with sprint unset
  When  validated
  Then  "format contains the SPRINT token but sprint is unset"
**Parameters:** `sprint >= 1` when SPRINT is used; an override sets at least one of type/regex/breaking, never both type and regex; `bump` ∈ {major, minor, patch, none}; regexes and `tag_pattern` must compile; template files must be readable and parse as Go `text/template`.
**Confidence:** High.

### RULE-026: Branches config validation
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:671-718`
**Plain English:** `versioning.branches` works only with strategy semver and each entry must be well-formed.
**Specification:**
  Given `branches` with strategy semver-per-env
  When  validated
  Then  error with a hint to use `environments.<env>.branch`
**Parameters:** each entry needs a valid glob name; an optional `range` is `N.x` or `N.N.x` and unique across entries.
**Confidence:** High.

### RULE-027: No releasable commits
**Category:** Validation
**Priority:** P0
**Source:** `internal/versioning/semver/resolver.go:174-182`
**Plain English:** A release cannot be cut with no commits, or only commits that do not bump.
**Specification:**
  Given base `v1.2.3` and one `docs:` commit with override `{type: docs, bump: none}`
  When  the version resolves
  Then  "no releasable commits since v1.2.3: 1 commit(s)…"; zero commits gives "no commits since v1.2.3"
**Edge cases handled:** `parseCommits` splits on NUL and drops whitespace-only chunks.
**Confidence:** High.

### RULE-028: Pre-release label and flag usage
**Category:** Validation
**Priority:** P1
**Source:** `internal/versioning/semver/prerelease.go:34-47`
**Plain English:** A pre-release label must be a valid identifier, and `--pre-release` only works in some modes.
**Specification:**
  Given `--pre-release 123` or `r.c`
  When  the run starts
  Then  rejected ("must not be purely numeric" / no dots)
**Parameters:** label matches `[0-9A-Za-z-]+`, not purely numeric; `--pre-release` is rejected with `--set-version`, with any strategy other than plain semver, and with bump mode manual (`internal/app/resolver.go:309-325`).
**Confidence:** High.

### RULE-029: Pre-release monotonicity and escalation
**Category:** Validation
**Priority:** P0
**Source:** `internal/versioning/semver/prerelease.go:227-269`
**Plain English:** A new pre-release must sort above every existing tag of its core, and escalating the core to a new major needs `--allow-major`.
**Specification:**
  Given an open series `v1.3.0-rc.1` and a `feat!` that makes the core 2.0.0
  When  releasing `rc`
  Then  `ErrMajorEscalation`; with `--allow-major` a warning and `v2.0.0-rc.1`
**Parameters:** label `alpha` after `rc.1` fails as a regression; a new minor or patch core only warns ("pre-release core escalated A → B").
**Edge cases handled:** on a maintenance line only in-range series are considered.
**Confidence:** High.

### RULE-030: Pre-release needs a commit unless promoting
**Category:** Policy
**Priority:** P1
**Source:** `internal/versioning/semver/prerelease.go:167-188`
**Plain English:** Re-cutting a label or opening a series needs a new commit; switching label on the same core is a promotion and does not.
**Specification:**
  Given HEAD at `v1.3.0-beta.2` with no new commits
  When  releasing `rc` → `v1.3.0-rc.1`; releasing `beta` again → "no commits since v1.3.0-beta.2"
**Confidence:** High.

### RULE-031: CalVer format parsing
**Category:** Validation
**Priority:** P0
**Source:** `internal/versioning/calver/parser.go:47-120`
**Plain English:** A CalVer format is made of known tokens, must contain PATCH, and PATCH must come last.
**Specification:**
  Given `YYYY.MM` → error "must contain PATCH"; `PATCH.YYYY` → "PATCH must be the last token"; `YYYY.MM.PATCH` valid
**Parameters:** tokens SPRINT, PATCH, YYYY, MM, DD, WW, QQ, SS plus literals; SPRINT tested before SS; parsing back: YYYY 4 digits, MM/DD/WW 1-2, QQ/SS 1, SPRINT/PATCH 1+.
**Confidence:** High.

### RULE-032: Version override flag combinations
**Category:** Validation
**Priority:** P1
**Source:** `internal/cmd/version_override.go:13-30`
**Plain English:** `--set-version` and `--set-build-id` must be used consistently.
**Specification:**
  Given `--set-build-id 42` without `--set-version`
  When  `release` runs
  Then  Config error
**Parameters:** `--set-version` non-empty, no whitespace, "/" allowed (`tagfmt.go:110-139`); build ID non-empty, no "/" and no whitespace; `--set-version` must not contain "+" when a build ID is given. The SemVer shape is checked later (`internal/app/resolver.go:361-378`).
**Suspected defect:** only whitespace is forbidden in `--set-version`; shell metacharacters pass, which matters because the value reaches hook commands unquoted (RULE-055).
**Confidence:** High.

### RULE-033: `--json` requires `--dry-run`
**Category:** Validation
**Priority:** P1
**Source:** `internal/cmd/release.go:42-44`
**Plain English:** The JSON output describes a plan, so it is only allowed on a run that changes nothing.
**Specification:**
  Given `heraut release --json`
  When  run
  Then  Usage error "--json describes the plan of a run that changes nothing: pass --dry-run as well"
**Confidence:** High.

### RULE-034: Conventional commit grammar
**Category:** Validation
**Priority:** P1
**Source:** `internal/conventionalcommit/conventionalcommit.go:20-90`
**Plain English:** A commit message must follow Conventional Commits.
**Specification:**
  Given "feat add x"
  When  verified
  Then  rejected (no `: `)
**Parameters:** header regex `^(\w+)(\(([^)]*)\))?(!)?: (.+)$`; anything after the header needs a blank line; only the last paragraph can be a footer block.
**Edge cases handled:** merge commits and `fixup!`/`squash!` are skipped (`internal/app/commit.go:57-59`, `conventionalcommit.go:93-102`); a description that is only whitespace after `: ` passes `(.+)`.
**Confidence:** High.

### RULE-035: Commit type and scope allow-list
**Category:** Validation
**Priority:** P1
**Source:** `internal/app/commit.go:49-109`
**Plain English:** The commit type must be known, and the scope must be allowed when scopes are restricted.
**Specification:**
  Given `scopes_restricted` with allowed `[api]` and `feat(ui): x`
  When  verified
  Then  "commit scope "ui" is not allowed"
**Edge cases handled:** a commit without a scope is always allowed; effective types = built-in defaults merged with `commits.types`.
**Confidence:** High.

### RULE-036: Commit rules (deny, require, require_ticket)
**Category:** Validation
**Priority:** P1
**Source:** `internal/app/commit.go:111-185`
**Plain English:** User-defined rules check the header, body, footer or whole message, and all violations are reported at once.
**Specification:**
  Given rule `{deny: "(?i)\bwip\b", target: header}` and header `feat: wip stuff`
  When  verified
  Then  an error with the rule's message
**Parameters:** deny violated on match; require violated on no match; require_ticket violated when no `commits.tickets` pattern matches; optional `types`/`scopes` scoping.
**Edge cases handled:** a regex compile error returns an error rather than skipping; `CheckCommitRange` evaluates every commit and skips merges and fixups (`internal/app/commit_check.go:62-92`).
**Confidence:** High.

### RULE-037: Commit rules and tickets config validation
**Category:** Validation
**Priority:** P1
**Source:** `internal/config/validator.go:67-228`
**Plain English:** Commit types, scopes, rules and ticket patterns must be well-formed.
**Specification:**
  Given a rule that sets both `deny` and `require`
  When  validated
  Then  "set only one of deny, require, or require_ticket"
**Parameters:** type names `^\w+$` and unique; ticket URL absolute http(s) containing `{ticket}`; `scopes_restricted` needs a non-empty scope list; rule target ∈ {header, body, footer, message, ""}; `require_ticket` needs non-empty `commits.tickets`.
**Confidence:** High.

### RULE-038: Git preflight and identity
**Category:** Validation
**Priority:** P1
**Source:** `internal/app/check.go:22-35`
**Plain English:** A real release or changelog run needs `git` on PATH and a configured committer.
**Specification:**
  Given an unset `git config user.email`
  When  `release` runs (not dry-run)
  Then  "preflight check failed: git user.email is not configured", Runtime exit code
**Confidence:** High.

### RULE-039: Asset glob handling
**Category:** Validation
**Priority:** P1
**Source:** `internal/platforms/globs.go:9-34`
**Plain English:** Release asset patterns are expanded; a pattern matching nothing only warns.
**Specification:**
  Given `dist/*.zip` matching nothing
  When  publishing to github
  Then  a warning and a release without assets
**Parameters:** `filepath.Glob`, directories dropped; invalid glob syntax errors; for GitLab a missing file or two assets with the same basename abort before any request (`internal/platforms/gitlab/client.go:112-125`); the GitHub release is created before assets upload (`internal/platforms/github/platform.go:181-198`).
**Suspected defect:** a typo in a glob silently publishes a release without its assets; and globs are not confined to the repository (SEC-009).
**Confidence:** High on mechanics. Question for a maintainer: is warn-only deliberate on every platform?

### RULE-040: Release pre-flight gates (skipped in dry-run)
**Category:** Validation
**Priority:** P0
**Source:** `internal/cmd/release.go:72-134`
**Plain English:** Before any write, a real release loads and validates the config, requires a destination, and checks branch and tools; a dry run skips the branch and tool checks.
**Specification:**
  Given a non-dry-run release with `git` missing
  When  the run starts
  Then  Runtime exit code before any mutation
**Parameters:** order: load, `--offline` override, validate, resolve env, require destination, build resolver and pipeline; then (non-dry-run only) `CheckBranch`, `CheckReleaseBranch`, `PreflightCheck`, `pipe.Check()`.
**Suspected defect:** a dry run can show a plan the real run would refuse (branch rules are not evaluated).
**Confidence:** High.

## Policy and eligibility

### RULE-041: stay_at_v0 hold
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/semver/hold.go:18-39`
**Plain English:** While the major version is 0 and `stay_at_v0` is on, a major bump becomes a minor bump with a warning.
**Specification:**
  Given current 0.4.2, `stay_at_v0` on, a `feat!:` commit
  When  the version resolves
  Then  0.5.0 with warning "major bump held back … 1.0.0 → 0.5.0"; with `--allow-major` the result is 1.0.0
**Parameters:** warning lists up to 5 subjects then "… and N more"; applies only when the bump is exactly major and major is 0; also per-env, in maintenance and pre-release modes.
**Confidence:** High — verified in this session.

### RULE-042: Maintenance range
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/semver/maintenance.go:26-111`
**Plain English:** A maintenance branch may only release versions inside its range.
**Specification:**
  Given branch range `1.3.x`, base `v1.3.4`, a `feat` commit
  When  the version resolves
  Then  next 1.4.0 is outside [1.3.0, 1.4.0) → `ErrOutOfRange`; a `fix` gives `v1.3.5`
**Parameters:** `N.x` owns [N.0.0, N+1.0.0), `N.M.x` owns [N.M.0, N.M+1.0); base is the highest in-range final reachable from HEAD (`--merged HEAD`); upper bound exclusive.
**Edge cases handled:** `ErrTagExists` when the next tag exists (a `+build` variant counts); `ErrNoInRangeRelease`; pre-release and build metadata ignored for containment.
**Confidence:** High.

### RULE-043: Promotion source env
**Category:** Policy
**Priority:** P1
**Source:** `internal/versioning/perenv/promote.go:235-265`
**Plain English:** A promote env takes from its declared source, or from the single auto env.
**Specification:**
  Given exactly one auto env and `prod` set to promote without `source`
  When  promoting
  Then  the source is that auto env; two auto envs → "ambiguous source"
**Edge cases handled:** only direct self-reference is checked here; longer cycles are RULE-023.
**Confidence:** High.

### RULE-044: Promotion guard E001: target tag exists
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/perenv/promote.go:165-178`
**Plain English:** Promotion refuses to recreate a tag that already exists.
**Specification:**
  Given latest staging `v1.4.0` and `prod/v1.4.0` already present
  When  `release --env prod`
  Then  E001, exit code Promotion (4); `--force` skips the guard and git's own duplicate-tag error is the fail-safe
**Confidence:** High.

### RULE-045: Promotion guard E002: destination ahead
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/perenv/promote.go:180-211`
**Plain English:** Promotion refuses to move a destination environment backwards.
**Specification:**
  Given prod latest 1.5.0 and staging latest 1.4.0
  When  promoting to prod
  Then  E002 with a suggested source tag, unless `--force`
**Parameters:** compares SemVer precedence (semver-per-env, so a destination pre-release above the candidate also blocks) or dotted integers (calver-per-env); `git tag -l <glob> --sort=-version:refname`.
**Edge cases handled:** `CurrentTag` (the hook's `PreviousTag`) is the highest destination release tag, never a pre-release (`promote.go:219-224`); if the source format needs `{build}` the hint falls back to a placeholder.
**Confidence:** High.

### RULE-046: Promotion guard E003: no source tags
**Category:** Policy
**Priority:** P0
**Source:** `internal/versioning/perenv/promote.go:140-155`
**Plain English:** You cannot promote from an environment that has no release; this guard cannot be forced.
**Specification:**
  Given an empty source env
  When  promoting, even with `--force`
  Then  E003
**Suspected defect:** a gap rather than a defect: `--set-version` with plain semver and no `versioning.branches` appears to have no tag-existence probe (`internal/app/resolver.go:220-258` only guards maintenance/branches cases), so a duplicate would fail at `git tag` after the changelog commit is already pushed.
**Confidence:** Medium — I confirmed `ExistingRelease` is only called from `app/resolver.go:244` and `maintenance.go:96`, but did not trace whether `guardSetVersion` is applied to every config. Question for a maintainer: is a collision probe outside the branches/per-env cases intentionally absent?

### RULE-047: Env branch guard
**Category:** Policy
**Priority:** P0
**Source:** `internal/app/branch.go:23-45`
**Plain English:** If an environment declares a branch, a real release must run from it.
**Specification:**
  Given env `prod` with branch `main`, currently on `develop`
  When  `release --env prod`
  Then  refused ("use --force to override"), Config exit code
**Edge cases handled:** no-op if env is empty or has no branch, or with `--force`; a detached HEAD yields `HEAD` and mismatches; failure to read the branch gives Runtime; not run in dry-run; for `changelog` it also runs without `--tag`.
**Confidence:** High — verified in this session.

### RULE-048: Release branch allow-list
**Category:** Policy
**Priority:** P0
**Source:** `internal/app/branchrules.go:46-143`
**Plain English:** With `versioning.branches`, publishing is allowed only from a branch matching exactly one entry.
**Specification:**
  Given branches `[main, release/*]` and branch `hotfix/x`
  When  `release` runs
  Then  `ErrUnlistedBranch`, unless `--force`
**Parameters:** branch from git; detached-HEAD fallback to `CI_COMMIT_BRANCH`, `GITHUB_REF_NAME` (only when `GITHUB_REF_TYPE=branch`) or `BUILD_SOURCEBRANCH` stripped of `refs/heads/`; `path.Match` for globs, equality otherwise.
**Edge cases handled:** a glob match with no derivable `N.x`/`N.N.x`/`N.N`/`N.N.N` needs an explicit `range` (`ErrUnderivableRange`); an undeterminable branch is refused.
**Confidence:** High.

### RULE-049: `--env auto` resolution
**Category:** Policy
**Priority:** P1
**Source:** `internal/app/env.go:17-62`
**Plain English:** `--env auto` picks the one environment bound to the current branch.
**Specification:**
  Given two envs both bound to `main`
  When  `--env auto` on `main`
  Then  error "pass --env explicitly"
**Edge cases handled:** needs a per-env strategy; errors on a detached HEAD, no match, or more than one match.
**Confidence:** High.

### RULE-050: Tag type selection
**Category:** Policy
**Priority:** P0
**Source:** `internal/pipeline/git.go:98-108`
**Plain English:** Signed beats annotated beats lightweight.
**Specification:**
  Given `SignTags` true and `AnnotatedTags` true
  When  the tag is created
  Then  `git tag -s <tag> -m <msg>` (interactive); annotated → `-a -m`; otherwise `git tag <tag>`
**Parameters:** tag message = commit message template with the version substituted; `SignTags` comes from git config `tag.gpgSign`.
**Confidence:** High — verified in this session.

### RULE-051: Pre-release flag only under SemVer
**Category:** Policy
**Priority:** P0
**Source:** `internal/pipeline/release.go:376-387`
**Plain English:** A release is flagged "pre-release" on the platform only for a SemVer strategy and a SemVer pre-release version.
**Specification:**
  Given semver and `2.0.0-rc.1` → prerelease=true; calver `YYYY.MM.SS-PATCH` and `2026.10.2-0` → false
**Edge cases handled:** an unparsable version yields false; the SemVer-shaped CalVer version is why the strategy gate exists.
**Confidence:** High — verified in this session.

### RULE-052: Default commit excludes
**Category:** Policy
**Priority:** P1
**Source:** `internal/config/commits.go:198-206`
**Plain English:** Release commits and dependency/PR noise never appear in changelogs.
**Specification:**
  Given a commit `chore(release): 0.80.0`
  When  the changelog is generated
  Then  it is omitted
**Parameters:** `^chore\(release\):`, `^chore\(deps.*\)`, `^chore\(pr\)`, `^chore\(pull\)`; user excludes are added to these, not replacing them; subject only.
**Suspected defect:** excludes do not affect bumping, so a `chore(deps):` commit still counts as a patch bump and can yield a release with an empty changelog.
**Confidence:** High — location corrected and read in this session (the extractor's cited lines were wrong).

### RULE-053: `--offline` override
**Category:** Policy
**Priority:** P1
**Source:** `internal/cmd/offline.go:12-22`
**Plain English:** `--offline` turns off PR/MR enrichment for the run, whatever the config says.
**Specification:**
  Given `enrichment_policy: required` and `--offline`
  When  `release` runs
  Then  policy is forced to `disabled`; no enrichment, no failure
**Edge cases handled:** creates `cfg.Commits` if nil; applied after load and before validation; declared locally on `release` and `changelog`.
**Confidence:** High.

### RULE-054: Enrichment policy
**Category:** Policy
**Priority:** P1
**Source:** `internal/generators/native/enrich.go:48-79`
**Plain English:** Enrichment can be off, best-effort or mandatory.
**Specification:**
  Given `optional` and a 500 from the forge
  When  the changelog renders
  Then  it continues without PR numbers/authors and is flagged degraded; `required` with no forge aborts ("remote enrichment (required): no forge resolved…")
**Parameters:** `disabled | optional | required`; "" behaves as optional; the config key `commits.remote_metadata` was renamed.
**Edge cases handled:** only the first failure reason is kept; `--force` downgrades `required` to optional (`internal/app/pipeline.go:665-672`).
**Confidence:** Medium — Question for a maintainer: is the `--force` downgrade intended for release notes as well as the changelog?

### RULE-055: Hook validation and execution safety
**Category:** Policy
**Priority:** P0
**Source:** `internal/pipeline/hooks.go:159-184`
**Plain English:** Hook commands run through a shell with the version and tag substituted in, unquoted.
**Specification:**
  Given a hook `deploy {{.Env}} {{.Tag}}`, env `prod`, tag `v1.4.0`
  When  the hook point runs
  Then  `sh -c "deploy prod v1.4.0"` (`cmd /D /C` on Windows), from the repo root, inheriting the environment
**Parameters:** config validation (`internal/config/validator.go:1094-1133`): each entry needs `run`; `stage` allowed only under post_bump and pre_changelog; hooks never run in dry-run or with `--no-hooks`.
**Suspected defect:** template values are not shell-quoted. See SEC-001 (a pushed tag reaching `PreviousTag` in promote runs) and RULE-032 (`--set-version` allows shell metacharacters). Hooks are arbitrary shell by design, so the real boundary is who may edit the config.
**Confidence:** Medium — a maintainer should decide whether quoting is the hook author's job.

### RULE-056: Exit-code mapping
**Category:** Policy
**Priority:** P1
**Source:** `internal/cmd/exit.go:35-49`
**Plain English:** Error classes map to fixed exit codes.
**Specification:**
  Given a promotion guard error (E001/E002/E003)
  When  the command fails
  Then  exit code 4 (`Promotion`), and the summary reads "release failed"
**Parameters:** missing `{build}` ID or bad `--env` → Config; branch mismatch → Config; branch read failure → Runtime; any other pipeline error → Runtime; unclassified → Usage; innermost classification wins. Numeric values of OK/Usage/Config/Runtime/Internal are defined in the external `forge/exitcode` library and were not read.
**Confidence:** High on the mapping; the numbers are an open question for a maintainer.

## Lifecycle

### RULE-057: Release step order
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/release.go:190-374`
**Plain English:** A release runs resolve, hooks, changelog, commit, tag, push tag, notes, then publish, stopping at the first error.
**Specification:**
  Given tag v1.4.0 resolved, a changelog configured, one platform
  When  `release` runs for real
  Then  resolve → post_bump hooks → [pre_changelog hooks, generate changelog, commit and push] → pre_tag hooks → create tag → push tag → post_tag hooks → [notes if one platform] → per platform publish → summary
**Edge cases handled:** with `disable_changelog` the changelog block (including pre_changelog) is skipped but tag and publish still run.
**Confidence:** High — order verified in this session.

### RULE-058: No rollback on failure
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/release.go:219-283`
**Plain English:** Nothing is undone when a step fails; recovery is to re-run.
**Specification:**
  Given the changelog commit is pushed and `git tag` then fails
  When  `Run` returns the error
  Then  the commit stays on the remote and there is no tag; if publish fails after the tag push, the tag stays and the release does not exist
**Suspected defect:** partial states (pushed commit without tag, pushed tag without release). The only recovery is `commitChangelog` re-pushing HEAD when ahead of upstream (RULE-060) plus a re-run, which may hit "tag exists" (E001, or git's own error for plain semver).
**Confidence:** High on behaviour. Question for a maintainer: is no-rollback intended and documented as "re-run to recover"?

### RULE-059: Dry-run executes nothing
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/release.go:389-492`
**Plain English:** A dry run resolves the version and prints what it would do, without changing anything.
**Specification:**
  Given `--dry-run`, tag v1.4.0 and hook `echo {{.Tag}}`
  When  `release` runs
  Then  `[dry-run] would run: echo v1.4.0`, `would tag v1.4.0 and push`, and no git mutation
**Edge cases handled:** the resolver uses a read-only runner; a hook template render error aborts the dry run; `NoHooks` suppresses hook lines; the plain-text path omits the "would generate notes" line the reporter path prints.
**Confidence:** High.

### RULE-060: Changelog commit and push are idempotent
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/git.go:55-74`
**Plain English:** The changelog is committed only if it changed, and pushed if there is something new to push.
**Specification:**
  Given a changelog byte-identical to HEAD and HEAD one commit ahead of upstream
  When  the changelog step runs
  Then  no commit, HEAD is pushed, and a "nothing to commit" warning is printed
**Parameters:** `git add <changelog + hook-staged files>`, `git diff --cached --name-only`, `git commit -m` (interactive, for GPG pinentry), `git push origin HEAD` when staged or `rev-list --count @{u}..HEAD > 0`; default message `chore(release): ${version}`.
**Edge cases handled:** with a detached HEAD or no upstream nothing is pushed; the release pipeline always pushes.
**Confidence:** High — verified in this session.

### RULE-061: Tag pushed before post_tag and publish
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/release.go:257-283`
**Plain English:** The tag becomes public before the post_tag hooks and before any release is created.
**Specification:**
  Given a post_tag hook that exits 1
  When  the release runs
  Then  the tag is already pushed, publish is never attempted, and the run fails
**Suspected defect:** a failing post_tag hook leaves a public tag with no release. In the changelog pipeline post_tag runs even with `NoPush` (`changelog.go:306-310`).
**Confidence:** High on behaviour. Question for a maintainer: is the unpublished-tag outcome acceptable?

### RULE-062: Per-platform publish and hook isolation
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/release.go:316-370`
**Plain English:** Each platform is published in turn; a hook failure skips only that platform, a publish failure stops the run.
**Specification:**
  Given platforms `[github, gitlab]` and github's pre_release hook exits 1
  When  publishing
  Then  github is skipped, gitlab publishes, the run errors "hook failed for platform(s): github" (Runtime)
**Parameters:** per platform: pre_release hooks, optional notes regeneration, `CreateRelease(tag, notes, prerelease)`, asset upload, post_release hooks.
**Edge cases handled:** a post_release failure after `CreateRelease` still reports the platform as failed though the release exists; a `CreateRelease` failure on the first platform leaves the next unpublished.
**Confidence:** High.

### RULE-063: Notes generated once or per platform
**Category:** Lifecycle
**Priority:** P1
**Source:** `internal/pipeline/release.go:285-342`
**Plain English:** With one platform the notes are generated once; with several, once per platform with its own links.
**Specification:**
  Given two platforms
  When  releasing
  Then  notes are generated twice, each with that platform's link context
**Parameters:** enabled when a notes generator exists and `DisableNotes` is false.
**Confidence:** High.

### RULE-064: Hook points and template variables
**Category:** Lifecycle
**Priority:** P1
**Source:** `internal/pipeline/hooks.go:12-107`
**Plain English:** Six hook points exist and each command can use five template variables.
**Specification:**
  Given a hook with `{{.Env}}` and env `prod`
  When  the point runs
  Then  the command is rendered and run; an unknown field is an error
**Parameters:** points post_bump, pre_changelog, pre_tag, post_tag, pre_release, post_release; variables `Version`, `Tag`, `PreviousTag` (= `result.CurrentTag`), `Platform` (release points only), `Env`; steps run in order and stop at the first failure.
**Confidence:** High.

### RULE-065: Hook stage patterns feed the release commit
**Category:** Lifecycle
**Priority:** P1
**Source:** `internal/pipeline/release.go:222-244`
**Plain English:** Files a hook declares with `stage` are added to the changelog commit.
**Specification:**
  Given a post_bump hook with `stage: [package.json]` and the changelog enabled
  When  the commit step runs
  Then  `git add CHANGELOG.md package.json`
**Edge cases handled:** a pattern that matches nothing makes `git add` fail and aborts the run.
**Suspected defect:** with the changelog disabled, the stage patterns are dropped and the hook's edits stay uncommitted (`postBumpStage` is only used inside the changelog block; verified).
**Confidence:** High. Question for a maintainer: is that intended?

### RULE-066: `--no-hooks` and `--skip-hooks`
**Category:** Policy
**Priority:** P1
**Source:** `internal/pipeline/hooks.go:84-89`
**Plain English:** Hooks are skipped on a dry run, with `--no-hooks`, or when a point has no steps; `--skip-hooks` filters individual points.
**Specification:**
  Given `--no-hooks`
  When  any hook point is reached
  Then  it is skipped
**Suspected defect:** the plan (RULE-071) consults only `NoHooks`, so `--skip-hooks` may not be reflected in the plan.
**Confidence:** Medium — `resolveSkipHooks` (`internal/cmd/release.go:35`) was not read. Question for a maintainer: where is `--skip-hooks` applied and does the plan reflect it?

### RULE-067: Changelog-only flow
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/pipeline/changelog.go:194-315`
**Plain English:** `heraut changelog` generates the file and optionally commits and tags.
**Specification:**
  Given `--tag` with `disable_changelog: true`
  When  `heraut changelog` runs
  Then  "changelog disabled for TAG" notice, no changelog, the tag is created and pushed
**Parameters:** order: resolve, post_bump, [pre_changelog, generate], [commit unless NoPush], [pre_tag, tag, push, post_tag]; `Tag` implies commit; neither Commit nor Tag means only the file is written.
**Edge cases handled:** a pre-release prints "pre-release: CHANGELOG.md not updated"; the summary distinguishes "unchanged, HEAD pushed" from "committed and pushed".
**Confidence:** High.

### RULE-068: `NoPush` semantics
**Category:** Policy
**Priority:** P1
**Source:** `internal/pipeline/changelog.go:265-346`
**Plain English:** `NoPush` creates the commit and tag locally and pushes nothing.
**Specification:**
  Given `NoPush` and `Tag`
  When  dry-run
  Then  `would tag v1.4.0 (no push)`
**Edge cases handled:** post_tag hooks still run; `release` has no equivalent.
**Confidence:** High.

### RULE-069: Incremental changelog splice
**Category:** Lifecycle
**Priority:** P0
**Source:** `internal/generators/native/generator.go:282-490`
**Plain English:** An existing changelog gets only the new release section spliced in; a missing or blank one is built from all history.
**Specification:**
  Given a `CHANGELOG.md` with anchors and 3 `feat` commits since v1.3.0
  When  generating v1.4.0
  Then  a v1.4.0 section is inserted and older sections are unchanged
**Parameters:** `--regenerate` (or `--regenerate-changelog`) forces a full rebuild; a non-empty file with no heraut-release anchors fails with an error naming the flags; preamble and postamble are always re-rendered; an empty new section leaves the file untouched; written with `os.WriteFile(…, 0o644)` (line 486).
**Edge cases handled:** bootstrap enriches only the newest section unless regenerating.
**Suspected defect:** a direct, non-atomic write can leave a truncated file if interrupted; there is no rotation or backup in this path.
**Confidence:** High — range corrected from the extractor's 418-469 after reading the file.

### RULE-070: `--json` envelope
**Category:** Policy
**Priority:** P2
**Source:** `internal/cmd/jsonout.go:12-69`
**Plain English:** With `--json` the output is a single JSON line with a stable shape.
**Specification:**
  Given a promotion E001
  When  a `--json` command fails
  Then  `{"ok":false,"warnings":[],"error":{"code":"E001",…}}` and the process exit code is unchanged
**Parameters:** `{ok, data?, warnings[], error?{code, message, hint?}}`; `warnings` is `[]`, never null; `error.code` is the guard ID if present, else `config|runtime|promotion|internal|usage`; HTML escaping off; `hint` only if the error implements `Hint()`.
**Edge cases handled:** an encoding failure on an otherwise OK run gives a Runtime exit code.
**Confidence:** High.

### RULE-071: `release --dry-run --json` plan
**Category:** Policy
**Priority:** P2
**Source:** `internal/pipeline/plan.go:30-94`
**Plain English:** The plan lists the steps of a real run in order, without running anything.
**Specification:**
  Given two platforms and the changelog enabled
  When  `release --dry-run --json`
  Then  no `notes` step (publish steps carry `notes:true`); hooks are rendered, never run; normal pipeline output is discarded
**Edge cases handled:** `--no-hooks` omits hook steps; `--skip-hooks` is not reflected (see RULE-066).
**Confidence:** High.

### RULE-072: Degraded-run and resolver warnings
**Category:** Policy
**Priority:** P2
**Source:** `internal/pipeline/warn.go:14-44`
**Plain English:** Non-fatal problems are shown as warnings after the step that produced them.
**Specification:**
  Given `optional` enrichment fails
  When  the changelog step finishes
  Then  the step is labelled "without enrichment" with sub-lines giving the reason
**Edge cases handled:** resolver warnings print after "Resolve version"; the "nothing to commit" warning notes the run continues; with `--json` warnings go into the `warnings` array.
**Confidence:** High.

## Rules requiring domain-expert confirmation

Medium or Low confidence, and P0 rules with a suspected defect.

| Rule | Question |
|---|---|
| RULE-001 (P0, High, defect) | Is rejecting a commit whose second line is not blank intended, given `feat: x` plus an immediate body line then adds no bump? |
| RULE-008 (P0, Medium) | Is git's `version:refname` order guaranteed to put the latest CalVer tag first for every supported format (SPRINT first, zero-padded fields)? |
| RULE-046 (P0, Medium) | Is the absence of a `--set-version` tag-collision probe outside the branches and per-env cases intentional? |
| RULE-054 (P1, Medium) | Should `--force` downgrade `required` enrichment for release notes, or only for the changelog? |
| RULE-055 (P0, Medium) | Is shell quoting of hook template variables the hook author's job? Should `--set-version` reject shell metacharacters? |
| RULE-058 (P0, defect) | Is "no rollback, re-run to recover" intended and documented? |
| RULE-061 (P0, defect) | Is a public tag with no release (failed post_tag hook) acceptable? |
| RULE-066 (P1, Medium) | Where is `--skip-hooks` applied, and should the dry-run plan reflect it? |
| RULE-069 (P0, defect) | Should the changelog write be atomic (temp file + rename)? |
| RULE-007 (P0, High) | Which time zone does the injected `now` use for CalVer dates? |
| RULE-056 (P1, High) | What are forge's numeric Usage/Config/Runtime/Internal exit codes? |
| RULE-065, RULE-039 (P1, defect) | Is dropping hook `stage` patterns with the changelog disabled, and warn-only unmatched asset globs, intended? |

## Instruction-shaped content in the source

None found by any of the three extractors or in the lines I read.

## Coverage gaps

- Not read: `whatsnew`, `heraut notes`, `resolveSkipHooks`, the internals of the init wizard and commit wizard, the full native template set, and the external `forge/*` library behaviour (exit codes, update check).
- Gaps the extractors reported: there is no working-tree-clean guard in the release or changelog pipeline (`git status --porcelain` appears only in `internal/app/check.go` and the commit wizard); there is no generic "tag already exists" guard for plain semver auto resolution (only per-env promotion, maintenance and the `--set-version` branches case); no `.heraut.yml` size or depth limit.
- About 50 of the 72 rules were not re-read by me and rely on the extractors' reading.
