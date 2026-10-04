# Spec 04 — Versioning Strategies

Heraut supports four versioning strategies. The strategy is selected in
`.heraut.yml` (`versioning.strategy:`) and determines how the next version is computed
and what tag format is used.

| Strategy          | Picks bump from         | Tag namespace                | Use case                                  |
|-------------------|-------------------------|------------------------------|-------------------------------------------|
| `semver`          | Conventional commits    | Single (e.g. `v1.2.3`)       | Standard SemVer projects                  |
| `calver`          | Calendar + PATCH counter| Single (e.g. `2026.05.0`)    | Date-driven release cadence               |
| `semver-per-env`  | Conventional commits    | Per-env (e.g. `dev/1.2.3`)   | Multi-env promotion with SemVer arithmetic |
| `calver-per-env`  | Calendar + PATCH counter| Per-env (e.g. `dev/2026.05.0`)| Multi-env promotion with date-driven      |

The strategy selector is implemented in `internal/app/resolver.go` (`app.NewResolver`).

---

## SemVer

```yaml
versioning:
  strategy: semver
  tag_prefix: "v"                   # tag prefix, default "v"
  initial_version: "0.1.0"
  bump:
    mode: auto                      # auto | manual — defaults to auto when omitted
    overrides:                      # optional; see § Bump-level overrides below
      - type: chore
        bump: none
```

Version is inferred from [Conventional Commits](https://www.conventionalcommits.org/)
since the last tag.

### Bump determination

Each commit resolves to a level — `major`, `minor`, `patch`, or `none` — by checking, in
order:

1. **User overrides** (`versioning.bump.overrides`, see below) — the first matching rule
   wins.
2. **Built-in defaults**, when no override matches:

   | Commit pattern                                    | Bump level |
   |---------------------------------------------------|------------|
   | `type!:` / `type(scope)!:` prefix (e.g. `feat!:`, `fix(api)!:`) or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer | major |
   | Any `feat:` commit (and no breaking change)       | minor      |
   | Any other conventional commit (`fix:`, chore/docs/refactor/style/test/ci/build/…) | patch |
   | Not a conventional commit, and no override matched it | none (ignored) |

The release's bump is the **highest** level contributed by any commit. If nothing
contributes — every commit resolved to `none`, or none were conventional commits an
override matched — the release has no bump, which is an error (see below), not a silent
patch. The `!` must sit immediately before the colon in the subject's type/scope prefix —
a bare `!:` inside the description does not trigger a major bump. `BREAKING-CHANGE:` is
treated as a synonym of `BREAKING CHANGE:`, per Conventional Commits 1.0.0. Either form
must begin its own paragraph to count as a footer — either the message's first line, or
a line immediately following a blank line. A wrapped body line that merely starts with
the token (a continuation of the previous line's sentence) does not trigger a major
bump.

### Bump-level overrides

`versioning.bump.overrides` is a list of rules, evaluated in order — the first match
wins, including over the "breaking commits are major" and "feat commits are minor"
built-in defaults. Each rule sets exactly one of `type` (the conventional-commit type) or
`regex` (matched against the commit subject), optionally combined with `breaking` (`true`
or `false`) to further scope the match, plus the `bump` level to assign: `major`,
`minor`, `patch`, or `none`.

```yaml
versioning:
  bump:
    overrides:
      - type: chore
        bump: none                  # chore commits no longer contribute to the bump
      - regex: '^chore\(deps.*\)'
        bump: none
      - type: fix
        bump: minor                 # promote fixes to a minor bump
      - breaking: true
        bump: minor                 # see § Staying at v0 for a self-retiring alternative that warns
```

With a `{breaking: true, bump: minor}` override in place nothing resolves to `major`, so
`stay_at_v0` never fires and no warning is printed.

A commit whose only matching rule resolves to `none` behaves like a non-conventional
commit: it does not raise the release's bump level. If every commit since the last tag
ends up this way, `heraut release` fails with an error listing the excluded commits
(subject lines only) instead of silently cutting an empty-content release:

```
no releasable commits since v0.62.0: 3 commit(s) since then are excluded from the version bump
  - chore: bump deps
  - docs: fix typo
  - chore(ci): update workflow
```

### Staying at v0 (`stay_at_v0`)

A project that is deliberately pre-1.0 can hold breaking changes back to a minor bump
([ADR-0063](../adr/0063-hold-major-at-v0.md)):

```yaml
versioning:
  bump:
    mode: auto
    stay_at_v0: true                # optional, default false
```

While the **current major version is 0**, a release whose bump resolves to `major` — after
`bump.overrides`, so an override that yields `major` is held back too — becomes a `minor` bump
instead (`v0.68.0` → `v0.69.0`, not `v1.0.0`). heraut prints a warning naming the commits that
forced the major, the version it held back, and how to lift it:

```
! major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0 (pass --allow-major on this run to get v1.0.0 instead)
  - feat(cmd)!: scope CLI flags to commands that use them, not root
```

Pass `--allow-major` (`heraut release`, `heraut changelog`, `heraut version next`) to release the
major for one run — `heraut version next --allow-major` previews it. The warning shows in
`--dry-run` too; `heraut version next` prints it to stderr so stdout stays exactly the tag.

`--allow-major` only affects the invocation it's passed to. It cannot undo a minor release a
previous run already tagged — the auto-resolver looks at commits since the *latest* tag, and by
then the commit that forced the major is already inside it. Decide before running `release`/
`changelog` for real: preview with `heraut version next` or `--dry-run` (both side-effect-free),
then add `--allow-major` to the real run if you want it. To get the major after a minor release
has already happened, use `--set-version` instead.

The setting is ignored with `--set-version` and under `bump.mode: manual` (a `semver` setting —
`bump.mode` has no effect on `semver-per-env` environments, where `stay_at_v0` still applies),
does nothing once the current major is 1 or higher (so it can stay in the config after 1.0), and
applies to `semver` and to the `bump: auto` environments of `semver-per-env` — not to CalVer or
`promote` environments.
It is v0-only on purpose: SemVer permits breaking changes in `0.y.z` (§4) but requires a major
bump for them from 1.0.0 (§8), so holding one back above 1.0 would make the version number lie.
The changelog and release notes still mark the commits as breaking.

### Prefix handling

`tag_prefix` (default `"v"`) is stripped before SemVer comparison and re-applied on output.
Tags are sorted by SemVer order, not lexicographically — `v1.10.0` is newer than
`v1.9.0`, and bumping `v1.9.0` produces `v1.10.0` (never `v1.100.0`).

### Pre-release tags

heraut parses tags strictly per SemVer 2.0.0 and orders them by SemVer §11 precedence in Go — it
does not depend on git's `version:refname` sort or on `versionsort.suffix` (ADR-0064) for version
resolution, the E002 regression check, `version current`, changelog section bounds, release-notes
previous-tag resolution, changelog rotation, or compare links (T334). When resolving the next
version, the current tag is the highest-precedence **release**: pre-release tags (`v1.3.0-rc.1`)
are never the bump base, and a tag carrying only build metadata (`v1.4.0+158404`) counts as the
release of its core (`1.4.0`). Tags that are not valid SemVer (`v1.02.0`, `v1.2.3.4`) are ignored.
If no tag qualifies, heraut behaves as if no tags exist and returns `initial_version`. Tags of
equal §11 precedence (e.g. `v1.4.0` and `v1.4.0+5`, which differ only in build metadata) keep
git's listing order relative to each other, since `Compare` treats them as equal and the sort is
stable.

For `semver`/`semver-per-env`, this extends to the native generator: pre-release tags are never a
range boundary and get no `CHANGELOG.md` section of their own in a *regenerated* changelog — their
commits fold into the next release's section — and a changelog/release-notes/rotation previous-tag
lookup resolves the previous *release*, never a pre-release `git describe` topology would otherwise
pick. `calver` and `calver-per-env` are unaffected: no order is injected for them, so they keep
walking git's own `version:refname` order exactly as before.

A pre-release run never writes `CHANGELOG.md` at cut time either: `heraut release` (including
`--pre-release`) and `heraut changelog --set-version X-pre` skip changelog generation and its
commit, while the tag, push and publish still happen (ADR-0064). The next final's incremental
run therefore lists those commits in its own section.

The same rules apply to `semver-per-env` (source and destination selection, and the E002
comparison). `calver-per-env` keeps its dotted-integer handling — zero-padded CalVer versions are
not SemVer.

heraut mints pre-release tags itself with `heraut release --pre-release <label>` (plain `semver` only; see [Pre-release lifecycle](#pre-release-lifecycle) below). Pre-release tags can also be created with `--set-version`.

### Pre-release lifecycle

Definitions: the **last final** is the highest-precedence tag with no pre-release identifiers; the
**core** of a version is its `MAJOR.MINOR.PATCH`; a **series** is the set of pre-release tags
sharing a core. A final release is computed as above and never uses a pre-release tag as its bump
base, so promoting `v1.4.0-rc.2` to `v1.4.0` needs no new commit.

A pre-release run (`semver` resolver, label set) resolves in this order (ADR-0064):

1. **Core** — the bump over all commits since the last final (`stay_at_v0` applied), exactly as a
   final would compute it. With no final yet, the core is `initial_version` and no commits are
   read. When `stay_at_v0` holds a major back, the warning names pre-release candidates on both
   sides (`v1.0.0-rc.1 → v0.4.0-rc.1 (… to get v1.0.0-rc.1 instead)`), the would-be counter being the
   one `--allow-major` would produce.
2. **Escalation** — *S* is the highest pre-release whose core is above the last final. If the new
   core is higher than *S*'s core, the bump level rose. A new **major** is an error
   (`ErrMajorEscalation`) unless `--allow-major`; a minor or patch rise (or a major with
   `--allow-major`) is allowed with the warning `pre-release core escalated <S.core> → <core>`
   followed by the subjects of the commits at the new bump level (max 5, then `… and N more`).
3. **Candidate** — `<core>-<label>.<N>`, where *N* is one more than the highest existing counter of
   that label on that core (`1` when none). Only tags shaped exactly `<label>.<n>` count.
4. **Monotonicity** — the candidate must sort strictly above **every** existing tag of the same
   core, finals included (`ErrPreReleaseRegression`); the highest offending tag is named.
5. **Commit requirement** — at least one commit since the **previous tag**: the highest tag, of any
   kind, that sorts below the candidate and is reachable from `HEAD` (`git tag --merged HEAD`).
   Exemption: when the previous tag is a pre-release of the **same core** and the candidate uses a
   different, higher label (`beta.2` → `rc.1`), no new commit is needed, as with promotion to the
   final; re-cutting the same label, or a previous tag of a different core, still requires one.
   The exemption chains on a single commit: `beta.1` → `rc.1` → `rcx.1` each need no new commit.
   Tags on other branches never count. The result's `CurrentTag` is that previous tag (else the
   last final), which is the range the release notes span.

**Label grammar**: one SemVer identifier — `[0-9A-Za-z-]+`, not purely numeric, no dots. heraut
always appends `.N`. Ordering between labels is §11's ASCII ordering (`alpha < beta < rc`; `RC`
sorts below `rc`).

Worked examples (last final `v1.3.0`):

| Commits since `v1.3.0` | Existing tags   | Label  | Result                                              |
|------------------------|-----------------|--------|-----------------------------------------------------|
| `feat: X`              | —               | `beta` | `v1.4.0-beta.1`                                     |
| + `fix: bug`           | `beta.1`        | `beta` | `v1.4.0-beta.2`                                     |
| (same)                 | `beta.2`        | `rc`   | `v1.4.0-rc.1`                                       |
| (same)                 | `rc.1`          | `beta` | error: `v1.4.0-beta.3` would sort below `v1.4.0-rc.1` |
| (none new)             | `rc.1`          | `rc`   | error: no commits since `v1.4.0-rc.1`               |
| `fix: A`               | —               | `rc`   | `v1.3.1-rc.1`                                       |
| + `feat: B`            | `v1.3.1-rc.1`   | `rc`   | `v1.4.0-rc.1` + escalation warning                  |
| + `feat!: C`           | `v1.4.0-rc.1`   | `rc`   | error: major escalation                             |
| (same)                 | `v1.4.0-rc.1`   | `rc` + `--allow-major` | `v2.0.0-rc.1` + escalation warning |

Error messages:

- `pre-release series v1.4.0-rc.1 would escalate to a new major 2.0.0: breaking change(s) since
  v1.3.0 — pass --allow-major to open the 2.0.0 series, or ship 1.4.0 first`, followed by the
  breaking commit subjects.
- `v1.4.0-beta.3 would sort below existing v1.4.0-rc.1 — ship 1.4.0 or use a label that sorts
  higher`.
- `no commits since v1.4.0-rc.1 — create at least one commit before running heraut release`.

### Initial version

When no tags matching the prefix exist, the resolver returns `initial_version` (default
`0.1.0`) — the first release does not bump.

### Manual mode

`bump.mode: manual` requires `--set-version X.Y.Z` to be passed to `heraut release` or
`heraut changelog`. If omitted, the command fails immediately with a runtime error
(exit code 3 — see [Spec 01 § Exit codes](01-overview.md#exit-codes)) before any git
operations. `heraut version next` also accepts `--set-version` and then prints the tag without
resolving a version from git history; without it, it fails with that same error. Since manual
mode only exists for `semver` (§ Staying at v0 above), the `X.Y.Z` passed here is validated as
SemVer v2 like any other `semver` `--set-version` (ADR-0064): a non-SemVer value such as
`2024.03` is a config error naming `--set-version`, not an opaque tag string.

`--set-version` is not exclusive to manual mode — passed to *any* strategy, it short-circuits
bump resolution entirely and bypasses the git calls that version resolution would make, exactly
as described here (see also [Spec 03 § `heraut release`](03-commands.md#heraut-release)). `calver`
and `calver-per-env` have no manual mode and keep today's lenient check (non-empty, no
whitespace) regardless of shape.

---

## CalVer

```yaml
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"       # CalVer format string
  tag_prefix: ""
```

The version is derived from the current date plus a `PATCH` counter that resets when
the calendar period changes.

### CalVer format tokens

| Token    | Description                        | Example value |
|----------|------------------------------------|---------------|
| `YYYY`   | 4-digit calendar year              | `2026`        |
| `MM`     | 2-digit month (zero-padded)        | `05`          |
| `DD`     | 2-digit day of month (zero-padded) | `07`          |
| `WW`     | 2-digit ISO week number (01–53)    | `19`          |
| `QQ`     | Quarter (1–4)                      | `2`           |
| `SS`     | Semester (1–2)                     | `1`           |
| `SPRINT` | Manually-managed sprint counter    | `5`           |
| `PATCH`  | Auto-incrementing patch (required) | `0`, `3`      |

`PATCH` is mandatory and must be the last *non-literal* token — a trailing literal
suffix after it (e.g. `YYYY.MM.PATCH-rc`) is allowed. It resets to `0` whenever the
period defined by the other tokens changes (e.g. new month for `YYYY.MM.PATCH`), and
increments by one for each release within the same period.

`SPRINT` is the only token not derived from the clock. Set it in
`versioning.sprint` and advance it manually with `heraut version sprint bump` at the
start of each sprint. When sprint changes, `PATCH` resets to `0` on the next release.

`heraut init` prompts for the initial `sprint` value when the selected CalVer format
contains `SPRINT`; it writes the entered value to `versioning.sprint` in the generated
`.heraut.yml`.

### Common format examples

| Format              | Example tags                          | Period    |
|---------------------|---------------------------------------|-----------|
| `YYYY.MM.PATCH`     | `2026.05.0`, `2026.05.1`              | Monthly   |
| `YYYY.MM.DD.PATCH`  | `2026.05.07.0`                        | Daily     |
| `YYYY.WW.PATCH`     | `2026.19.0`                           | Weekly    |
| `YYYY.QQ.PATCH`     | `2026.2.0`, `2026.2.1`                | Quarterly |
| `YYYY.SS.PATCH`     | `2026.1.0`, `2026.2.0`                | Bi-annual |
| `YYYY.SPRINT.PATCH` | `2026.5.0`, `2026.5.1`                | Sprint    |
| `YYYY.PATCH`        | `2026.0`, `2026.1`                    | Yearly    |

### Sprint example

```yaml
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
  sprint: 5   # bumped by the PO at the start of each sprint
```

`heraut version sprint bump` increments `sprint` from `5` to `6` and writes back. The
next release after the bump produces `2026.6.0`.

### Period-change resolution

The resolver compares the period of the current date against the period of the latest
tag's date. If they differ, `PATCH` resets to `0`. If they match, `PATCH` becomes the
latest tag's `PATCH` + 1.

The "period" depends on which tokens appear in `format`:

| Format contains | Period bucket                |
|-----------------|------------------------------|
| `YYYY.MM`       | Calendar month               |
| `YYYY.WW`       | ISO week                     |
| `YYYY.MM.DD`    | Calendar day                 |
| `YYYY.QQ`       | Calendar quarter             |
| `YYYY.SS`       | Calendar semester            |
| `YYYY.SPRINT`   | The `sprint` config value    |
| `YYYY` only     | Calendar year                |

---

## SemVer per environment

```yaml
versioning:
  strategy: semver-per-env

environments:
  dev:
    tag_format: "dev/{version}"   # tags: dev/1.0.1, dev/1.0.2
    branch: develop
    bump: auto
  prod:
    tag_format: "prod/{version}"  # tags: prod/1.0.0, prod/1.0.1
    branch: main
    bump: promote                 # promote = take version from latest dev tag
```

Each environment has its own tag namespace. Typically one environment uses `bump: auto`
to drive version computation; others use `bump: promote` to receive copies.

### Tag format

Each environment specifies its tag structure via `tag_format`, a string containing the
mandatory `{version}` token. The token can appear anywhere, enabling any ordering. The
`{env}` token is also available (substituted with the environment name).

| `tag_format`          | Example tags    | Pattern        |
|-----------------------|-----------------|----------------|
| `"dev/{version}"`     | `dev/1.0.2`     | ENV/SEMVER     |
| `"{version}/dev"`     | `1.0.2/dev`     | SEMVER/ENV     |
| `"{version}_dev"`     | `1.0.2_dev`     | SEMVER_ENV     |
| `"dev_{version}"`     | `dev_1.0.2`     | ENV_SEMVER     |
| `"release/{version}"` | `release/1.0.2` | custom prefix  |
| `"{env}/{version}"`   | `dev/1.0.2`, `prod/1.0.2` | shared format using `{env}` |

The `{version}` / `{env}` substitution is implemented in `internal/versioning/tagfmt/`,
shared between both per-env strategies ([ADR-0009](../adr/0009-generic-perenv-resolver.md)).

Instead of repeating an identical `tag_format` on every environment, set it once at the
top-level `versioning.tag_format` using `{env}`; a per-environment `tag_format` always
overrides the common one when both are set. See
[Spec 02 § Common `tag_format`](02-configuration.md#common-tag_format).

A third token, `{build}`, is available for CI build IDs (e.g. `"{env}/{version}+{build}"`
→ `uat/7.4.1+158404`) — populated by `--set-build-id <id>` on `heraut changelog`/`heraut release`/`heraut version next`,
requires `--set-version` to also be passed. See
[Spec 02 § `{build}` token](02-configuration.md#build-token--ci-build-ids) for the full
reference.

### Bump modes

**`bump: auto`** — resolves the latest tag in **this environment's own** namespace by
SemVer (not lexicographically, so `dev/1.10.0` beats `dev/1.9.0`), reads conventional
commits since that tag, and
increments the patch/minor/major component accordingly. The same pre-release-tag skip
policy as plain SemVer applies (§ Pre-release tags): a tag like `dev/1.3.0-rc.1` is
skipped in favor of `dev/1.2.3` when selecting the current tag.

**`bump: promote`** — resolves the latest tag of the source environment, strips the
source format to extract the bare version, and renders it under the destination format.
Example: `dev/1.0.2` → `prod/1.0.2`. Applies the same pre-release-tag skip policy as
`bump: auto` (§ Pre-release tags above) — a pre-release tag sorting first in the source
environment is never selected as the promotion candidate.

The source environment is determined by the optional `source:` field (see
[ADR-0008](../adr/0008-promote-source-env.md)):

- **`source` omitted** — backward-compatible default: the single `bump: auto`
  environment is used. Validation error if zero or more than one `auto` environment
  exists.
- **`source: <env>`** — promotes from the named environment regardless of its `bump`
  mode. Enables disambiguation when multiple `auto` environments exist, and chaining
  (e.g. `prod` promotes from `preprod`, which itself promotes from `dev`).

### Promotion guards (E001/E002/E003)

Three hard-fail conditions are checked before any tag is created
([ADR-0007](../adr/0007-version-promotion-error-handling.md)):

| Code | Condition                                                                | Bypassed by `--force`?   |
|------|--------------------------------------------------------------------------|--------------------------|
| E001 | The candidate target tag already exists                                  | Yes                      |
| E002 | The destination environment is already at a version strictly greater than the candidate | Yes           |
| E003 | The source environment has no tags yet (nothing to promote)              | No — `--force` has no effect |

Each error message includes the source env, destination env, candidate version, and
the latest tag(s) involved. A destination already at *exactly* the candidate version
doesn't reach E002's strict-greater-than check — E001 already caught it, since the
candidate tag would already exist.

### Version resolution logic

For `bump: promote` (the path the E001/E002/E003 guards apply to):

1. List all tags matching the **source** environment's glob (derived from its
   `tag_format` via `tagfmt.GlobPattern`)
2. Sort by version order (not lexicographically), skipping any pre-release tag
   (§ Pre-release tags) — **E003** fires here if no qualifying source tag remains
3. Extract the bare version from the latest qualifying source tag
4. Render the candidate tag under the destination environment's `tag_format`
5. Check **E001** (candidate tag already exists)
6. Check **E002** (destination already ahead of the candidate)
7. Pass the resolved tag to downstream generators / platforms

`bump: auto` follows the same list → sort → skip-pre-release shape (§ Bump modes above)
but computes the next version from commits or the clock instead of promoting one, and
has no E001/E002/E003 guards — there's nothing to promote, so nothing to guard against.

heraut resolves the tag itself and passes it to the changelog/release-notes generator and
publish drivers directly — it does not rely on any of them to independently re-derive a
tag pattern from `tag_prefix`/`tag_format`. `changelog.tag_pattern` (native's own tag-walk
scope) is a separate, generator-level concern — see
[Spec 02 § Content generation](02-configuration.md#content-generation).

---

## CalVer per environment

```yaml
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"       # CalVer format for the version component

environments:
  dev:
    tag_format: "dev/{version}"   # dev/2026.05.0
    bump: auto
  prod:
    tag_format: "prod/{version}"  # prod/2026.05.0
    bump: promote
```

Combines CalVer versioning (§ CalVer format tokens) with the multi-environment tag
model (§ SemVer per environment: tag format, bump modes, source field, E001/E002/E003).

**`bump: auto`** — finds the latest tag for the environment (by CalVer order, not
lexicographic), then computes the next version from the clock:

- If the calendar period has changed: `PATCH` resets to `0`.
- Within the same period: `PATCH` increments by one.

Unlike `semver-per-env`, **no conventional commit parsing** is performed — the CalVer
version advances purely from the date.

**`bump: promote`** — same source resolution, `source:` field semantics, and
E001/E002/E003 checks as `semver-per-env`, including E002's regression check — one
dot-separated-integer comparison implements it for both strategies, since both SemVer
(`1.2.3`) and CalVer (`2026.05.3`) versions compare correctly component-by-component this
way. There is no separate CalVer-specific ordering.

---

## Generic per-env resolver internals

The two per-env strategies share a single implementation
([ADR-0009](../adr/0009-generic-perenv-resolver.md)):

```
internal/versioning/perenv/
   resolver.go    — VersionCalculator interface + New(runner, cfg, env, force, calc)
   auto.go        — auto mode (list tags → sort → bump from commits or date)
   promote.go     — promote mode (resolve source → strip → re-render → E001/E002/E003)
```

The `VersionCalculator` interface has two methods:

- `BumpAuto(tags []string, commits []string) (string, error)` — implemented by
  `internal/versioning/semver/`
- `BumpFromDate(tags []string) (string, error)` — implemented by
  `internal/versioning/calver/`

`app.NewResolver` selects which `VersionCalculator` to wire when constructing a
`perenv.Resolver`. SemVer and CalVer per-env modes share the same package; only the
calculator differs.
