# Hook-Declared File Staging — commit files a hook generates alongside the changelog

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-09-17
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0053 (release lifecycle hooks — the base feature this extends), 0054 (Windows
  hook execution — the per-OS shell selection this design doesn't touch), 0055 (`{{ .Env }}` hook
  template variable — precedent for extending hook template context without a new syntax)
- **New ADR required**: yes — next available number (0061 as of this writing), "Hook-declared file
  staging + object-only hook config." Two decisions in one ADR because they're coupled: the file
  staging feature is what forces the hook-entry shape to grow past a bare string.
- **Roadmap**: own dedicated file — `docs/tasks/hook-file-staging-roadmap.md`, following the
  release-hooks/changelog-rotation/forge-abstraction epics' pattern. Not filed yet; filing happens
  in the implementation-planning step after this design is reviewed.
- **Origin**: no roadmap task — direct user request, comparing heraut's ADR-0053 hooks against
  cocogitto's `pre_bump_hooks` behavior.

---

## Problem

A user asked to mimic cocogitto: edit `composer.json`'s version field in a hook before the
changelog commit, and have that file land in the same commit as `CHANGELOG.md` — the way
cocogitto's `chore(version): vX.Y.Z` commit does.

Investigation (reading cocogitto's actual source, `crates/cocogitto/src/command/bump/standard.rs`)
confirmed cocogitto achieves this via `self.repository.add_all()` — a blanket `git add -A .`
run after `pre_bump_hooks` and right before the version commit. **The user explicitly rejected
porting that behavior**: a blanket `add_all` sweeps in *any* dirty file in the working tree, not
just files the hook intentionally produced — an unrelated stray edit (a half-finished local change,
an IDE artifact) would silently ride along into a release commit. heraut's current hook
implementation (ADR-0053) is the opposite extreme: `commitChangelog` (`internal/pipeline/git.go:50`)
stages only the changelog file by exact path, so a hook-modified `composer.json` is never staged,
never committed, and never tagged — it's left dirty in the working tree with no warning (heraut's
automatic `PreflightCheck`, `internal/app/check.go:22`, doesn't check working-tree cleanliness at
all; only the separate, advisory `heraut check runtime` does, and even there it only warns).

This design sits between those two extremes: **explicit, opt-in, per-hook file declarations** —
a hook says exactly which file(s) it produces, and only those get staged.

## Goals

- A hook entry can declare the file(s)/pattern(s) it produces via a new `stage` field, staged into
  the same commit as the changelog.
- No blanket `git add -A`/`add_all` — only explicitly declared paths are ever staged beyond the
  changelog file itself.
- `stage` is meaningful only where a commit actually exists to receive it (`post_bump`,
  `pre_changelog` — both run before `commitChangelog`); declaring it anywhere else is a **config
  validation error**, not a silent no-op.
- Hook list entries become object-only (`- run: "echo {{ .Version }}"` is the minimal form) across
  all six hook points, uniformly — not just the two that support `stage` — so the config shape is
  one predictable type everywhere and future per-hook fields (already anticipated, not designed
  here) don't require another migration.
- No new glob-matching dependency: `stage` patterns pass straight to `git add`, which already does
  its own pathspec wildcard matching with no shell involved (`port.Runner` execs `git` directly).
- `stage` patterns are Go `text/template` strings, rendered through the same `hookVars` engine
  `run` already uses (so `stage: ["dist/app-{{ .Version }}.json"]` works).

## Non-goals (v1)

- **Blanket "stage everything" mode.** The one behavior explicitly rejected as the reason this
  design exists. Not offered as an option, not even opt-in.
- **Staging tied to `pre_tag`/`post_tag`/`pre_release`/`post_release`.** No commit exists at or
  after those points in the current pipeline (`pre_tag` fires *after* the changelog commit already
  landed). A future "commit hook-generated build artifacts at tag time" feature would need its own
  design (a new commit point) — not assumed or scaffolded here.
- **Auto-detecting which files a `run` command touched** (e.g. diffing `git status` before/after
  each command). `stage` is a static, user-written declaration — same trust/explicitness model as
  `run` itself, not inferred.
- **A preflight dirty-working-tree gate.** Out of scope; unrelated to this feature. heraut's
  automatic preflight still won't check working-tree cleanliness before a run starts — a hook that
  leaves stray uncommitted changes *outside* its declared `stage` patterns is still silently
  allowed, exactly as today.
- **Per-env hook overrides.** Unchanged from ADR-0053 — still flat/global only.

## Design

### 1. Hook entries become object-only

```yaml
hooks:
  post_bump:
    - run: "php composer.phar config version {{ .Version }}"
      stage: ["composer.json"]
    - run: "echo {{ .Version }} > VERSION"
  pre_changelog:
    - run: "make lint"
  pre_tag:
    - run: "go build ./..."
```

```go
// HookStep is one command in a hook point's list (ADR-0053, extended by ADR-0061).
type HookStep struct {
    Run   string   `yaml:"run"`
    Stage []string `yaml:"stage,omitempty"`
}

// Hooks configures shell commands run at points in the release lifecycle.
type Hooks struct {
    PostBump     []HookStep `yaml:"post_bump,omitempty"`
    PreChangelog []HookStep `yaml:"pre_changelog,omitempty"`
    PreTag       []HookStep `yaml:"pre_tag,omitempty"`
    PostTag      []HookStep `yaml:"post_tag,omitempty"`
    PreRelease   []HookStep `yaml:"pre_release,omitempty"`
    PostRelease  []HookStep `yaml:"post_release,omitempty"`
}
```

All six fields change from `[]string` to `[]HookStep` uniformly, even though `stage` is only
*valid* on two of them (Design §2) — one type, one mental model, across every hook point.

**Deliberate breaking change.** A bare string under any hook point (`post_bump: ["echo hi"]`, the
only form ADR-0053 ever supported) becomes a config error. Blast radius is small — heraut's own
`.heraut.yml` doesn't use `hooks:` yet, and the feature has been out for about a week (v0.64.0 →
current v0.67.0) — but it's still a real break for any external adopter, and must be stated as
deliberate (the ADR's job) rather than incidental. The loader (`internal/config/loader.go`) gives a
specific, actionable error rather than surfacing yaml.v3's raw type-mismatch text:

```
✗ hooks.post_bump[0]: expected a mapping, got a plain string
  hint: wrap it as { run: "echo {{ .Version }} > VERSION" }
```

### 2. `stage` scope validation

`internal/config/validator.go` gains a rule: `Stage` must be empty on every `HookStep` under
`PreTag`, `PostTag`, `PreRelease`, `PostRelease`. A non-empty `Stage` there is a hard validation
error:

```
✗ hooks.pre_tag[0].stage: not allowed here
  hint: stage is only valid under post_bump or pre_changelog
  (files staged there have no commit left to land in)
```

`post_bump` and `pre_changelog` are the only points that run *before* `commitChangelog` — see the
pipeline order in Design §3.

Separately, `Run` is required and non-empty on every `HookStep`, at all six points — `stage` is
optional, `run` is not. A `HookStep` with only `stage` and no `run` (e.g. `- stage: ["x"]`) is a
config error: `stage` describes a side effect of running a command, not a standalone action, and
there is no such thing as a "stage only" step.

### 3. Staging mechanism — extends `commitChangelog`, no new glob code

Today (`internal/pipeline/git.go:50`):

```go
func (g *gitHelper) commitChangelog(file, msg string, push bool) (bool, error) {
    if err := g.run("git", "add", file); err != nil { ... }
    ...
}
```

Widens to accept every path/pattern to stage, not just the changelog file:

```go
func (g *gitHelper) commitChangelog(files []string, msg string, push bool) (bool, error) {
    if err := g.run("git", "add", files...); err != nil { ... }
    ...
}
```

Callers (`release.go:229`, `changelog.go:241`) build `files` as
`[changelogFile, ...renderedStagePatterns]`, where `renderedStagePatterns` is every `Stage` entry
from every `HookStep` across **both** `post_bump` and `pre_changelog` for this run, in the order
those hooks executed, template-rendered through the existing `hookVars`/`renderHookCmd` machinery
(Design §4 of the ADR-0053 design doc) exactly like `Run` strings are today.

No new glob-matching code is needed: `git add <pattern>` already does its own pathspec wildcard
matching without shell involvement (`port.Runner` execs `git` directly — no shell expands the
pattern first), so a rendered `stage` entry like `dist/*.json` passes straight through unchanged.

The existing `hasStagedChanges()` gate (`git.go:74`, `git diff --cached --name-only`) is already
generic — not scoped to the changelog path — so it composes for free: if `composer.json` changed
but the changelog is byte-identical to the last commit, the commit still happens and contains just
`composer.json`; if nothing staged changed at all, the existing "nothing to commit" warn-and-
continue path is unchanged.

### 4. Error handling — zero-match patterns already fail correctly

A `stage` pattern matching zero files needs no new detection code: plain `git add <pathspec>`
already exits non-zero when a pathspec matches nothing, and that error already propagates
(`fmt.Errorf("git add: %w", err)`). A typo'd or stale `stage` pattern aborts the run loudly,
consistent with "a failing hook step aborts the run" (ADR-0053 Design §3) — it's treated the same
as any other pipeline step failure, not silently ignored.

### 5. Dry-run

Extends the existing `[dry-run] would run: <cmd>` line (`internal/pipeline/hooks.go:74`,
`dryRunHookLines`) with one additional line per non-empty `Stage`:

```
[dry-run] would run: php composer.phar config version 1.4.0
[dry-run] would stage: composer.json
```

Matches ADR-0053's existing promise that dry-run renders every hook node without executing
anything.

### 6. `--no-hooks`

No new logic: `--no-hooks` already skips hook execution entirely at each of the six points, and
`stage` only ever takes effect as a consequence of its owning hook step running — skipping the step
skips its staging too, automatically.

## New ADR-0061 outline

**Title**: Hook-declared file staging + object-only hook config.

**Decision**: (1) `hooks:` list entries require the `{run: "...", stage: [...]}` mapping form —
bare strings, ADR-0053's only supported shape, become a config error. (2) `stage` lets a hook
declare file(s)/pattern(s) it produces, staged into the changelog commit alongside `CHANGELOG.md`,
valid only under `post_bump`/`pre_changelog`.

**Why not cocogitto's `add_all`**: record the explicit rejection — a blanket `git add -A` risks
sweeping unrelated dirty files into a release commit; explicit declaration keeps hooks' blast
radius to exactly what the user named, consistent with ADR-0053's trust model (hooks can do
anything, but heraut itself never silently broadens what a hook's *declared* effect is).

**Why object-only, not scalar-or-mapping shorthand**: a deliberate, stated breaking change over a
backward-compatible shorthand, traded for one predictable shape everywhere and easier future
extension — explicitly chosen with the compatibility cost known, not overlooked.

**Why `stage` is scope-validated, not silently inert elsewhere**: matches heraut's existing
"config error, not silent no-op" philosophy (e.g. an unresolvable `release.targets` today) rather
than letting a misplaced `stage` produce a mysteriously dirty index with no explanation.

## Roadmap placement

New `docs/tasks/hook-file-staging-roadmap.md`, pointed to from `docs/tasks/roadmap.md`'s Phase
list. Not filed yet. Expected task breakdown (sizing TBD when filed):

1. `internal/config`: `HookStep` struct, all six `Hooks` fields `[]string` → `[]HookStep`, loader
   error for a bare string with the migration hint, `schema.json`, `docs/heraut.sample.yml`.
2. `internal/config/validator.go`: reject `stage` outside `post_bump`/`pre_changelog`.
3. `internal/pipeline/hooks.go`: update `renderHookCmds`/`runHookPoint`/`dryRunHookLines` (and
   whatever iterates `[]string` today) to operate on `[]HookStep`, rendering `.Run` and each
   `.Stage` entry independently.
4. `internal/pipeline/git.go`: `commitChangelog(file, ...)` → `commitChangelog(files []string, ...)`.
5. `internal/pipeline/release.go` + `changelog.go`: collect rendered `Stage` patterns from
   `post_bump`+`pre_changelog` results, pass into the widened `commitChangelog` call.
6. Dry-run: `[dry-run] would stage: <pattern>` lines.
7. ADR-0061.
8. Docs: Spec 02 § `hooks` (object-only shape + `stage` subsection + scope-validation table),
   `docs/guides/release-pipeline-and-hooks.md`, `docs/heraut.sample.yml`.
9. Tests (Testing plan below).

## Testing plan

- **Unit**: loader rejects a bare string under any hook point with the specific hint text.
- **Unit**: validator rejects non-empty `stage` under `pre_tag`/`post_tag`/`pre_release`/
  `post_release`; accepts it under `post_bump`/`pre_changelog`.
- **Unit**: `stage` pattern template rendering (`{{ .Version }}` etc.) as a pure function, same
  table-driven shape as existing `Run`-rendering tests.
- **Contract** (`exectest.MockRunner`): `commitChangelog` issues one `git add` call with the
  changelog path *and* every rendered `stage` pattern from both `post_bump` and `pre_changelog`, in
  order; a `git add` failure (simulating a zero-match pathspec) propagates as the commit step's
  error.
- **Unit**: `hasStagedChanges()` composition — changelog byte-identical but a staged file changed
  still produces a commit; nothing staged at all still hits the existing "nothing to commit" path
  unchanged.
- **Integration** (`internal/testutil.RealGitRepo`): a `post_bump` hook writes `composer.json`,
  declares `stage: ["composer.json"]`; the resulting changelog commit's tree contains both
  `CHANGELOG.md` and `composer.json`. A second case: a hook declares a `stage` pattern that matches
  nothing — the run aborts with the `git add` error, no commit is created.
- **Schema**: `testdata/config/valid/hooks.yml` updated to the object form (with and without
  `stage`); `testdata/config/invalid/hooks_bare_string.yml` and
  `testdata/config/invalid/hooks_stage_wrong_point.yml` fixtures for the two new error paths.

## Resolved questions

(Captured from the design conversation, in order.)

- **Blanket `add_all` vs. explicit declaration**: explicit, opt-in `stage` — cocogitto's `add_all`
  was investigated (confirmed via source: `git2::Index::add_all(["."], ...)`, run right after
  `pre_bump_hooks`) and deliberately rejected as too broad.
- **Staging model**: per-command declarations (`{run, stage}` on each hook entry), not a separate
  flat `hooks.stage: [...]` list — chosen for co-location between a command and the file(s) it
  produces, and because the user wants the object shape available for future per-command
  extensibility regardless of `stage` specifically.
- **Hook config shape**: object-only (`- run: "..."` is the minimum), not scalar-or-mapping
  shorthand — a deliberate breaking change, accepted with the compatibility cost stated explicitly
  (small blast radius: feature is ~1 week old, heraut's own config doesn't use it yet).
- **`stage` scope**: valid only under `post_bump`/`pre_changelog`; a config validation error
  (not silent) anywhere else — chosen over "parse everywhere, only consumed at the two live
  points" specifically to avoid a silently-inert typo.
- **Glob matching**: no new dependency — `git add <pattern>` already does its own pathspec
  wildcard matching with no shell involved.
- **Zero-match `stage` pattern**: already an error for free, via `git add`'s own exit code —
  no new detection code needed.
- **Preflight dirty-tree check**: confirmed out of scope during investigation — `PreflightCheck`
  (`internal/app/check.go:22`) never checks working-tree cleanliness today (only the separate,
  advisory `heraut check runtime` does); this design doesn't change that.
