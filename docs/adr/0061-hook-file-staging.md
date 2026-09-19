# ADR-0061: Hook-declared file staging + object-only hook config

- **Status**: Accepted
- **Date**: 2026-09-19
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-09-17-hook-file-staging-design.md`](../superpowers/specs/2026-09-17-hook-file-staging-design.md)
- **Related ADRs**: [0053](0053-release-lifecycle-hooks.md) (release lifecycle hooks — the
  base feature this extends), [0054](0054-windows-hook-execution.md) (per-OS shell
  selection, unchanged by this ADR), [0055](0055-env-hook-template-variable.md) (`{{ .Env }}`
  hook template variable — precedent for extending hook template context without a new
  syntax; `stage` reuses the same rendering engine rather than inventing one)

---

## Context

A user asked to mimic cocogitto: edit `composer.json`'s version field in a hook before the
changelog commit, and have that file land in the same commit as `CHANGELOG.md` — the way
cocogitto's `chore(version): vX.Y.Z` commit does.

Investigation (reading cocogitto's actual source,
`crates/cocogitto/src/command/bump/standard.rs`) confirmed cocogitto achieves this via
`self.repository.add_all()` — a call that resolves to `git2::Index::add_all(["."], ...)`, i.e.
a blanket `git add -A .` run after `pre_bump_hooks` and right before the version commit.

The user explicitly rejected porting that behavior. A blanket `add_all` sweeps in *any* dirty
file in the working tree, not just files the hook intentionally produced — an unrelated stray
edit (a half-finished local change, an IDE artifact, a `.env` accidentally left unignored)
would silently ride along into a release commit. There is no way to distinguish "the hook
meant to change this" from "this happened to be dirty when the hook ran."

heraut's existing hook implementation ([ADR-0053](0053-release-lifecycle-hooks.md)) sat at
the opposite extreme: `commitChangelog` (`internal/pipeline/git.go`) staged only the
changelog file by exact path. A hook that modified `composer.json` was never staged, never
committed, and never tagged — it was left dirty in the working tree with no warning. heraut's
automatic `PreflightCheck` (`internal/app/check.go`) doesn't check working-tree cleanliness
at all; only the separate, advisory `heraut check runtime` does, and even there it only
warns. So the pre-existing behavior wasn't neutral — it silently dropped a hook's output on
the floor, which is its own kind of surprising.

This ADR records the design chosen to sit between those two extremes: **explicit, opt-in,
per-hook file declarations** — a hook says exactly which file(s) it produces, and only those
get staged.

## Decision

Two coupled decisions, recorded in one ADR because the second forces the first: adding a
second field to a hook entry is what pushes the config shape past a bare string.

### 1. `hooks:` list entries become object-only

Every hook point's list entries move from a bare shell-command string (ADR-0053's only
supported shape) to a mapping:

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
```

`run` is required on every `HookStep`, at all six hook points; `stage` is optional and,
per Decision 2 below, valid only on two of them. All six `Hooks` fields
(`PostBump`, …, `PostRelease`) change from `[]string` to `[]HookStep` uniformly — even
though `stage` only makes sense on two of them — so the config shape is one predictable
type everywhere, and a future per-hook field (already anticipated, not designed here)
doesn't force another migration through the same six list types.

**This is a deliberate, stated breaking change.** A bare string under any hook point
(`post_bump: ["echo hi"]`) is now a config error:

```
✗ hooks.post_bump[0]: expected a mapping, got a plain string "echo hi"
  hint: wrap it as { run: "echo hi" }
```

The blast radius is small — the hooks feature had been out for about a week (v0.64.0 →
v0.67.0 at the time this shipped) and heraut's own `.heraut.yml` didn't use `hooks:` yet —
but "small" is not "zero" for any external adopter who had already written the bare-string
form, and that cost is named explicitly here rather than left implicit.

**Why object-only, not a scalar-or-mapping shorthand** (accept both `- "echo hi"` and
`- run: "echo hi"`): a backward-compatible shorthand was considered and rejected. Supporting
both shapes indefinitely means every consumer of `Hooks` — the loader, the validator, the
renderer, the documentation, every fixture — carries a permanent two-shape branch for a
feature that's a week old with no real-world configs depending on the old shape yet. Object-
only is a one-time, bounded cost; a permanent shorthand is an unbounded one paid on every
future change to `HookStep`.

### 2. `stage` is opt-in and scoped to `post_bump`/`pre_changelog`

A `HookStep` may declare `stage: [...]` — path(s) or pattern(s) the `run` command is
expected to produce. Every declared `stage` entry, from every hook step across **both**
`post_bump` and `pre_changelog` for the current run, is staged into the same commit as
`CHANGELOG.md`, in the order those hooks executed. No blanket `git add -A`; only what a
hook explicitly named.

`stage` is valid only under `post_bump` and `pre_changelog`:

| Hook point      | `stage` allowed? | Why |
|------------------|:---:|-----|
| `post_bump`      | Yes | Runs before the changelog commit; a file it stages lands there. |
| `pre_changelog`  | Yes | Same — still before the changelog commit. |
| `pre_tag`        | No  | Runs *after* the changelog commit already landed — no commit left to receive a staged file. |
| `post_tag`       | No  | Same — after the changelog commit, after the tag too. |
| `pre_release`    | No  | No commit exists at or after publish time in the current pipeline. |
| `post_release`   | No  | Same. |

A `stage` entry outside `post_bump`/`pre_changelog` is a **config validation error**, not a
silent no-op:

```
✗ hooks.pre_tag[0].stage: not allowed here
  hint: stage is only valid under post_bump or pre_changelog
  (files staged there have no commit left to land in)
```

**Why an error instead of silently ignoring `stage` at the other four points**: this matches
heraut's existing "config error, not silent no-op" philosophy — the same posture an
unresolvable `release.targets` already gets. A `stage` entry that heraut quietly discarded
at `pre_tag` would leave a user with a `.heraut.yml` that reads as if it does something it
doesn't, and no signal anywhere that it isn't happening. A run that fails loudly at
`heraut check config` (or the next `release`/`changelog` invocation) is strictly better than
a mysteriously-never-staged file discovered only by reading the source.

### Mechanism: extends `commitChangelog`, no new dependency

`commitChangelog` (`internal/pipeline/git.go`) widened from staging one fixed changelog
path to staging a `files []string` — the changelog path plus every rendered `stage` pattern
collected from `post_bump` and `pre_changelog` for the run — in a single `git add` call.
`stage` entries are Go `text/template` strings, rendered through the same `hookVars`
engine `run` already uses (ADR-0053), so `stage: ["dist/app-{{ .Version }}.json"]` works
exactly like a templated `run` command does.

No new glob-matching dependency was introduced: a rendered `stage` pattern passes straight
to `git add`, and `git add` already does its own pathspec wildcard matching with no shell
involved (`port.Runner` execs `git` directly — no shell expands the pattern first). A
pattern like `dist/*.json` is git's problem to match, not heraut's.

A `stage` pattern that matches nothing needed no new detection code either: `git add
<pathspec>` already exits non-zero when a pathspec matches zero files, and that failure
already propagates as an ordinary pipeline-step error — the same as any other `git add`
failure. A typo'd or stale `stage` pattern aborts the run loudly, consistent with "a failing
hook step aborts the run" (ADR-0053).

Dry-run extends the existing `[dry-run] would run: <cmd>` line with one additional
`[dry-run] would stage: <pattern>` line per non-empty `stage` entry, rendered (not raw),
matching ADR-0053's existing promise that dry-run shows every hook node without executing
anything.

## Consequences

- **Deliberate breaking change to `hooks:` config**, stated explicitly rather than left for
  users to discover via a strict-decode error with no context. A bare string under any hook
  point now fails to load, with a migration hint pointing at the object form.
- **No blanket staging, ever.** A hook's blast radius on the git index is exactly the paths
  it names in its own `stage` list — never "everything dirty in the tree." An unrelated
  stray local change stays untouched by a release run, unlike cocogitto's `add_all`.
- **`stage` is dead weight if misused, in two different ways.** Declaring it at the wrong
  hook point (`pre_tag`/`post_tag`/`pre_release`/`post_release`) is a hard config error at
  validation time, before any hook runs. Declaring it at a valid point (`post_bump`/
  `pre_changelog`) in a run where no changelog commit happens this run — no `changelog:`
  block configured, `disable_changelog: true`, or `heraut changelog` invoked without
  `--commit`/`--tag` — is *not* an error: the hook's `run` command still executes, but there
  is no commit for `stage` to join, so its files stay uncommitted in the working tree.
- **One predictable `HookStep` shape everywhere**, even though `stage` is only meaningful on
  two of the six points — a future hook-entry field (already anticipated) extends the same
  type instead of requiring a second migration.
- **Still no rollback machinery.** A `stage` pattern that matches nothing aborts the run via
  `git add`'s own exit code, same as any other step failure — no new undo path for whatever
  ran before it, matching ADR-0053's existing "no rollback for any step" posture.
- **No preflight dirty-working-tree gate was added.** Out of scope for this ADR — a hook
  that leaves stray uncommitted changes *outside* its declared `stage` patterns is still
  silently allowed, exactly as before. `stage` narrows what a hook's output *can* land in a
  commit; it does not audit what a hook leaves behind.

## Alternatives considered

- **cocogitto's `add_all` (blanket `git add -A` after hooks, before the commit).** Rejected
  as the whole reason this design exists — see Context above. Simpler to implement (zero new
  config surface) but trades away the ability to say "the release commit contains exactly
  what I intended," which the user explicitly wanted to keep.
- **A separate flat `hooks.stage: [...]` list, independent of any single hook step.**
  Rejected in favor of per-step `{run, stage}` co-location: a flat list decouples a staged
  path from the command that produced it, which reads fine for one hook but becomes
  ambiguous the moment more than one `post_bump`/`pre_changelog` step exists — the object
  form also leaves room for other future per-step fields without inventing a second
  parallel list per new field.
- **Parse `stage` everywhere, only consume it at `post_bump`/`pre_changelog`.** Rejected —
  silently ignoring a misplaced `stage` produces a mysteriously dirty-or-clean index with no
  explanation, exactly the failure mode "config error, not silent no-op" exists to prevent
  elsewhere in this codebase (Decision 2 above).
- **Auto-detecting which files a `run` command touched** (diffing `git status` before/after
  each hook command) instead of an explicit `stage` declaration. Rejected: `stage` keeps the
  same trust/explicitness model `run` itself already has — a user-written declaration, not
  something heraut infers — and diffing working-tree state around arbitrary shell execution
  is meaningfully more machinery (and more surprising when a hook's side effects extend
  beyond the file it meant to change) for a v1 that didn't need it.
- **A backward-compatible scalar-or-mapping shorthand** for hook entries. Rejected — see
  Decision 1 above; a permanent two-shape branch outlives the one-week-old feature it would
  be preserving compatibility for.
- **Extending `stage` to `pre_tag`/`post_tag`/`pre_release`/`post_release`.** Rejected: no
  commit exists at or after those points in the current pipeline (`pre_tag` fires *after*
  the changelog commit already landed). A future "commit hook-generated build artifacts at
  tag time" feature would need its own design — a new commit point, not assumed or
  scaffolded here.
