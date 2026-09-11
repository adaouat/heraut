# Release Lifecycle Hooks — arbitrary shell commands at pipeline points

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-09-11
- **Author**: bchatard (with Claude)
- **Supersedes**: `docs/superpowers/specs/2026-08-28-release-hooks-exploration.md` (exploration
  notes; this doc resolves its open questions)
- **Related ADRs**: 0013 (raw-binary goreleaser — heraut ships Windows binaries, informs the
  POSIX-only v1 scope below), 0043 (forge abstraction — precedent for "new execution mode added
  deliberately, not by accident"), 0052 (versioning bump object — precedent for the nil-safe
  accessor pattern this design reuses)
- **New ADR required**: yes — ADR-0053, "Release lifecycle hooks: arbitrary shell execution via
  config." First heraut feature where a config file causes arbitrary code execution (trust model:
  same as a CI YAML `run:` step) and the first use of `sh -c` anywhere in the codebase.
- **Roadmap**: own dedicated file — `docs/tasks/release-hooks-roadmap.md`, following the
  changelog-rotation/forge-abstraction/native-generator epics' pattern. Not filed yet; filing
  happens in the implementation-planning step after this design is approved.
- **Origin**: no roadmap task — direct user request to revisit the parked exploration doc.

---

## Problem

heraut has no way to run user-defined commands at points in the release lifecycle. Common needs
this blocks: bumping a version string embedded in another file (`package.json`, `Cargo.toml`)
before the changelog commit; running a build or test gate before tagging; triggering `npm publish`
or a deploy after the tag exists; notifying Slack after a GitHub/GitLab release goes live.
cocogitto (`pre_bump_hooks`/`post_bump_hooks`, see the exploration doc) is the closest prior art,
but its two-hook model doesn't map cleanly onto heraut's own pipeline, which is richer than
cocogitto's (cocogitto has no publish step, no per-target loop, no `disable_changelog`).

This design was produced by walking the actual pipeline code
(`internal/pipeline/release.go`, `internal/pipeline/changelog.go`) rather than adapting
cocogitto's model wholesale, and settles every open question the exploration doc left unresolved.

## Goals

- Six hook points, matching the real steps in both pipelines (see Design §1).
- A single flat, global `hooks:` config block (no per-env in v1).
- Go `text/template` command strings, reusing heraut's existing templating (no new dependency,
  no second syntax to document alongside the changelog/release-notes templates).
- Hook output streams live to the terminal (the same `Interactive` runner mode built for GPG
  pinentry, T260) rather than being captured and summarized.
- `--no-hooks` flag on `release` and `changelog` for a no-config-edit escape hatch.
- Failure semantics that require **no new rollback machinery**: hooks fail exactly the way any
  other pipeline step already fails today, with one deliberate, scoped exception (per-platform
  isolation — see Design §3).

## Non-goals (v1)

- **Per-env hook overrides.** Flat/global only, matching how changelog rotation and other epics
  started flat-only before per-env was added later once real usage patterns emerged.
- **Non-POSIX shells.** `sh -c "<command>"` only. Windows is a documented limitation in v1 despite
  heraut shipping Windows binaries (ADR-0013) — hooks are a POSIX-only feature to start.
- **Configurable working directory per hook.** Always the repository root. Nothing in the
  codebase or the use cases discussed motivates anything else yet.
- **cocogitto's bump profiles, version-arithmetic DSL, and branch whitelisting.** Carried over
  unchanged from the exploration doc's non-goals list — nothing in this design reopens them.
- **Per-package/monorepo hooks.** heraut has no monorepo concept at all.
- **A rollback/stash mechanism for pre-tag hook failures.** cocogitto stashes changes on a
  pre-bump hook failure. heraut's pipeline has no rollback for *any* step failure today (a failing
  `git tag` after a successful changelog commit already leaves that commit in place) — hooks
  don't get special treatment relative to every other step.

## Design

### 1. Six hook points, mapped to exact pipeline steps

```
post_bump      — after "Resolve version" (release.go:108 / changelog.go:97)
pre_changelog  — before "Generate changelog" (release.go:129 / changelog.go:129)
pre_tag        — before "Create tag" (release.go:157 / changelog.go:161) — tag not yet local-created
post_tag       — after "Push tag" (release.go:167 / changelog.go:171) — tag now pushed to origin
pre_release    — before "Publish to <platform>"'s CreateRelease, per platform (release.go:199)
post_release   — after that platform's CreateRelease + UploadAssets, per platform (release.go:221)
```

`post_bump`/`pre_changelog`/`pre_tag`/`post_tag` fire in **both** pipelines — `heraut release` and
`heraut changelog --tag` run the identical resolve → changelog → commit → tag sequence, so no
separate on/off switch is needed per command. `pre_release`/`post_release` only fire in
`heraut release`, since `changelog.go` never publishes; they simply never trigger in the
changelog-only pipeline.

`post_bump` fires unconditionally on every resolve, regardless of what runs downstream —
including `DisableChangelog: true` with no `--tag`, and plain `heraut changelog` runs with no
`--tag`/`--commit` flags at all. It is purely "a version was determined," independent of whether
anything else happens this run.

No hook exists before version resolution (nothing meaningful to act on yet — resolution is the
pipeline's first action) or between changelog-write and commit (the commit step is mechanical
persistence of what `pre_changelog` already anticipated; a `post_changelog` point was considered
and explicitly deferred, see Non-goals).

### 2. Config schema

```yaml
hooks:
  post_bump:
    - "echo {{ .Version }} > VERSION"
  pre_changelog:
    - "make lint"
  pre_tag:
    - "go build ./..."
  post_tag:
    - "npm publish"
  pre_release:
    - "echo 'about to publish to {{ .Platform }}'"
  post_release:
    - "curl -X POST $SLACK_WEBHOOK -d 'Released {{ .Tag }} to {{ .Platform }}'"
```

```go
// Hooks configures shell commands run at points in the release lifecycle (ADR-0053).
type Hooks struct {
    PostBump     []string `yaml:"post_bump,omitempty"`
    PreChangelog []string `yaml:"pre_changelog,omitempty"`
    PreTag       []string `yaml:"pre_tag,omitempty"`
    PostTag      []string `yaml:"post_tag,omitempty"`
    PreRelease   []string `yaml:"pre_release,omitempty"`
    PostRelease  []string `yaml:"post_release,omitempty"`
}
```

Added as `Hooks *Hooks `yaml:"hooks,omitempty"`` on the top-level `Config` struct
(`internal/config/config.go`), alongside `Changelog`/`Release`/`Rendering`/`Forges`. Nil-safe
accessors (e.g. `Config.PostBumpHooks() []string`) mirror ADR-0052's pattern: omitting the whole
`hooks:` block, or any individual key, is equivalent to an empty list — zero behavior change for
every existing config.

Each list runs its commands **in order**; the first failing command in a list stops the rest of
*that same list* (see Design §3 for what happens to the surrounding pipeline step).

### 3. Failure semantics

- **`post_bump` / `pre_changelog` / `pre_tag` / `post_tag`**: a failing hook aborts the whole run
  and returns a wrapped error, identically to any other `runStep` failure in the pipeline today.
  No new rollback: if `pre_tag` fails after the changelog was already committed and pushed, that
  commit stays — exactly as if `git tag` itself had failed at that point today.
- **`pre_release` / `post_release`**: isolated per platform, a deliberate asymmetry from the
  pipeline's existing all-or-nothing platform loop (today, an actual `CreateRelease` failure on
  platform A still aborts before platform B is attempted — that behavior is unchanged). A
  `pre_release` hook failure skips `CreateRelease`/`UploadAssets` *and* `post_release` for that
  platform entirely (nothing was published, so there is nothing for `post_release` to react to)
  and the loop proceeds to the next platform. A `post_release` hook failure just warns (the release
  already happened; nothing to undo) and also proceeds. If any platform was skipped or warned due
  to a hook failure, the overall command still exits non-zero after attempting every remaining
  platform.

### 4. Template variables

Reuses Go's `text/template` (already used for changelog/release-notes rendering — no new
dependency, one syntax across the whole config file), fed from the pipeline's already-resolved
`versioning.Result`:

| Variable          | Source                          | Available at                          |
|-------------------|----------------------------------|----------------------------------------|
| `{{ .Version }}`  | `result.Version`                | all six points                        |
| `{{ .Tag }}`      | `result.Tag`                    | all six points                        |
| `{{ .PreviousTag }}` | `result.CurrentTag`           | all six points (empty if no prior tag) |
| `{{ .Platform }}` | `plat.Name()`                   | `pre_release`/`post_release` only (empty elsewhere) |

Because these are real Go templates, a single `pre_release`/`post_release` hook can already branch
per platform with `{{ if eq .Platform "github" }}...{{ end }}` — no per-platform config keys
needed, unlike cocogitto's flat-substitution model.

### 5. Execution model

New unexported helper (proposed: `internal/pipeline/hooks.go`, a sibling to `git.go`) wraps
`port.Runner`:

```go
func runHook(runner port.Runner, cmd string) error {
    return runner.RunDir("", nil, "sh", "-c", cmd) // dir="" = repo root; env unmodified
}
```

`runner` here is always the same pre-configured *interactive* runner instance
`Pipeline.WithInteractiveRunner`/`ChangelogPipeline.WithInteractiveRunner` already inject for
GPG-signing (T260) — not a flag passed per call. `gitHelper.interactiveRunner` is git-specific
today; this design generalizes that one field up to `Pipeline`/`ChangelogPipeline` itself (or
duplicates the same optional-field-with-fallback pattern) so hook execution can reach it without
going through `gitHelper`. Every hook command runs through that instance — the exact mode
`forge/exec.CmdRunner` already implements for GPG pinentry: stdin/stdout/stderr connect
directly to the real terminal, streamed live, not captured. This is a deliberate trade-off: hook
output (a build log, `npm publish`'s progress, an OTP prompt) is the visible progress for that
step — it cannot also be echoed as a one-line step-reporter sub-result the way `git tag` or
`gh release create` output is today. The existing GPG-pinentry precedent already establishes that
the reporter/spinner UI accommodates an interactive sub-command; hooks reuse that same
accommodation rather than inventing a second one.

Pipeline wiring: each of the six points becomes a `p.runStep(...)` call (or, for
`pre_release`/`post_release`, a call inside the existing per-platform loop) that iterates the
configured command list and calls `runHook` for each, short-circuiting the list on first failure
per Design §3.

`--no-hooks` is declared locally on `release` and `changelog` (not root, per the existing
per-command flag convention — T266) and, when set, skips all six points for that invocation
without touching config.

**Dry-run**: shows the *rendered* command string (real version/tag substituted in), matching how
dry-run output already shows real resolved tag names rather than placeholders elsewhere in the
pipeline — e.g. `[dry-run] would run pre_tag hook: go build ./...`.

## New ADR-0053 outline

**Title**: Release lifecycle hooks: arbitrary shell execution via config.

**Decision**: `hooks:` config entries are shell command strings executed via `sh -c` through
`port.Runner` in `Interactive` mode. This is heraut's first capability where the config file
directly causes arbitrary code execution — the trust model is identical to a CI YAML `run:` step
(anyone who can edit `.heraut.yml` can run anything the invoking user/CI credentials can run) and
should be stated explicitly rather than left implicit.

**Why this needs its own ADR** (vs. a roadmap note): every other `port.Runner` call in the
codebase today is a structured invocation of a known binary (`git`, `gh`, `glab`) with discrete,
heraut-controlled args — this is the first free-form user string, and the first use of `sh -c`
anywhere. A future contributor reasoning about heraut's execution model needs this documented as a
deliberate boundary crossing, not something that crept in.

**Also document**: the per-platform hook-failure isolation asymmetry (Design §3) — a rule a future
contributor could plausibly "fix" into consistency with the rest of the pipeline's all-or-nothing
loop behavior without realizing it was intentional.

## Roadmap placement

New `docs/tasks/release-hooks-roadmap.md`, following the changelog-rotation/forge-abstraction epics'
structure (own file, pointer from `docs/tasks/roadmap.md`'s Phase list). Not filed yet — filing and
task breakdown is the next step after this design is reviewed. Expected shape (sizing TBD when
filed):

1. `internal/config`: `Hooks` struct + nil-safe accessors + `schema.json` + sample config.
2. `internal/pipeline`: `runHook`/`runHooks` helper (contract tests: `MockRunner` asserts exact
   `sh -c "<rendered>"` call).
3. Wire the four shared points into both `release.go` and `changelog.go`.
4. Wire `pre_release`/`post_release` into `release.go`'s per-platform loop, including the
   isolation/continue behavior.
5. `--no-hooks` flag on both commands.
6. Dry-run rendering for all six points.
7. ADR-0053.
8. Docs: `docs/specs/` (wherever pipeline behavior is documented today), README if it covers
   config surface.
9. Integration test: one real-git-repo happy path proving a `pre_tag`/`post_tag` hook actually
   executed and templated correctly.

## Testing plan

- **Unit**: template rendering (`Result` → rendered command string) as a pure function, including
  the empty-`PreviousTag`/empty-`Platform` cases.
- **Contract** (`exectest.MockRunner`): for each of the six points, in both pipelines where
  applicable, assert the exact `sh`, `-c`, `"<rendered command>"` args reach the runner, and that
  `Interactive` mode is set.
- **Unit**: per-platform failure isolation — a failing `pre_release` hook for platform A skips A's
  `CreateRelease` but platform B is still attempted; overall `Run()` returns a non-nil error;
  a failing `post_release` hook similarly doesn't stop platform B.
- **Unit**: `post_bump`/`pre_changelog`/`pre_tag`/`post_tag` failures abort the run immediately —
  no further steps execute, error is wrapped and propagated (`errors.Is`-compatible).
- **Integration** (`internal/testutil.RealGitRepo`): one happy path with an echo-to-file hook at
  `pre_tag` and `post_tag`, proving real execution order and template substitution against a real
  git repo.
- **Schema**: `testdata/config/valid/hooks.yml` fixture covering all six keys; one invalid fixture
  if any schema-level validation is added (e.g. rejecting an empty string in a command list).

## Resolved questions

(Carried over from the exploration doc's Open Questions section, each resolved during this design
session.)

- **Hook points**: six, not cocogitto's two — `post_bump`, `pre_changelog`, `pre_tag`, `post_tag`,
  `pre_release`, `post_release`. Driven by reading heraut's actual pipeline rather than adapting
  cocogitto 1:1; `post_bump`/`pre_changelog` are separable in heraut (unlike cocogitto) because of
  `disable_changelog`, and `pre_tag`/`post_tag` are separable because heraut has an explicit local
  tag → push split cocogitto's doc doesn't surface.
- **Applies to `heraut changelog` too?**: yes, automatically, for the four shared points — falls
  out of both pipelines running the same steps, not a separate config toggle.
- **Template syntax**: Go `text/template`, not cocogitto's Tera-style syntax or plain env vars.
- **Failure semantics**: no rollback/stash mechanism (unlike cocogitto) — matches the pipeline's
  existing no-rollback-anywhere behavior, except the new deliberate per-platform isolation for
  `pre_release`/`post_release`.
- **Platform**: POSIX (`sh -c`) only for v1; Windows is a documented gap.
- **Per-env overrides**: out of scope for v1, flat/global `hooks:` only.
- **`--no-hooks` escape hatch**: in scope for v1.
- **Dry-run rendering**: shows the rendered command (real values substituted), not the raw
  template.
- **Working directory**: always repo root; no per-hook override in v1.
- **Hook output handling**: `Interactive` passthrough (streamed live), not captured/buffered —
  reuses the T260 GPG-pinentry runner mode rather than inventing a new execution mode.
