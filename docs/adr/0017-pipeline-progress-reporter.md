# ADR-0017: Pipeline Progress Reporter Pattern

- **Status**: Accepted
- **Date**: 2026-05-27
- **Deciders**: bchatard

---

## Context

The `heraut release` and `heraut changelog` commands run multi-step sequential pipelines
but emit no progress feedback while those steps execute. Today, the user sees nothing
until the final `released v1.2.3` line — or an error. For operations that call external
tools (`git-cliff`, `gh`, `glab`) or perform git operations, this silence is a poor
experience: the user cannot tell whether the command is working or hung.

The `heraut check` command (T36) already solved the same problem at the check layer via
a streaming dispatch pattern: `app.RuntimeCheck(runner, cfg, func(name string, run func()
RuntimeCheckItem))` calls the provided callback for each check item, and
`internal/cmd/check.go` wraps each call with a `ui.StartStep` spinner. That solution is local to the check layer
and does not generalise to the release/changelog pipelines.

The question: **how should the pipeline layer communicate per-step progress to the UI
layer**, and **how should sub-steps** (e.g. asset upload is logically part of "publish to
platform", not a separate numbered step) **be represented**?

This builds on ADR-0015 (rejected: charm/log not adopted) and the `internal/ui` Option 4
path it deferred: grow `internal/ui` with typed helpers without a logging framework.

### Step inventory

**`heraut release`** — between 2 and N steps depending on config:

| Step | Always? |
|------|---------|
| Resolve version | always |
| Generate changelog | `cfg.Changelog != nil && !cfg.DisableChangelog` |
| Commit changelog | same as above |
| Create tag | always |
| Push tag | always |
| Generate release notes | `cfg.Notes != nil && !cfg.DisableNotes` |
| Publish to {platform} + optional asset upload | one per platform |

**`heraut changelog`** — 1 to 5 steps:

| Step | Always? |
|------|---------|
| Resolve version | always |
| Generate changelog | `cfg.Changelog != nil` |
| Commit changelog | `(cfg.Commit \|\| cfg.Tag) && cfg.Changelog != nil` |
| Create tag | `cfg.Tag` |
| Push tags | `cfg.Tag` |

---

## Options considered

### Option 1 — Embed `ui.Step` calls directly in the pipeline

The pipeline imports `internal/ui` (allowed by the layer rules) and calls `ui.StartStep`
at each step boundary.

**Pros:** Simple. No new abstraction.

**Cons:** The pipeline is coupled to lipgloss styles and the spinner. Pipeline tests would
capture spinner output, requiring ANSI stripping or assertions that tolerate it.
Fundamentally mixes the orchestration logic with the presentation layer in the same file.

### Option 2 — `StepFn` callback injected into the pipeline (chosen)

Define a func type `StepFn` in `internal/ui`. Both `Pipeline` and `ChangelogPipeline`
gain an optional `reporter StepFn` field. When `nil`, each step runs directly without any
UI wrapper (existing behaviour, tests unaffected). When set, each step's work is wrapped
by the reporter.

The step function returns `(result string, subResults []string, error)`:
- `result` is shown inline on the success line: `✓ [N/M] step name — result`
- `subResults` are printed as indented lines below the success line, with a leading `✓`

**Pros:** Mirrors the `app.RuntimeCheck` dispatch pattern already in use. Pipeline tests
stay clean — they pass a `nil` reporter. The `ui.Progress` type provides the production
implementation. The signature extension for sub-results is a minor addition to the step
function signature.

**Cons:** The step function signature `func() (string, []string, error)` is slightly more
verbose than a plain `func() error`. Callers that don't use sub-results return `nil` for
the slice.

### Option 3 — Progress interface

Define a `port.Progress` or `ui.ProgressReporter` interface that the pipeline calls.

**Pros:** Swappable implementations.

**Cons:** A named interface adds complexity without adding testability over a func type.
The pipeline has exactly one caller (the `cmd` layer) and one production implementation.
A `nil`-able func type is simpler and expresses the same contract. Rejected per Go
convention ("the bigger the interface, the weaker the abstraction" — but here the issue
is the opposite: an interface for one method adds noise).

---

## Decision

**Option 2 — `StepFn` callback, `nil` = silent, sub-results as `[]string`.**

### `StepFn` type

```go
// StepFn is the type of a function that wraps one named pipeline step.
// name is shown while the step is running and on the completion line.
// fn performs the work. It returns:
//   - result: shown as "✓ [N/M] name — result" when non-empty
//   - subs:   zero or more sub-result labels, each printed as an indented
//             "  ✓ label" line beneath the parent completion line
//   - err:    step failure; the reporter renders it as "✗ [N/M] name — err"
//
// When nil, the pipeline calls fn directly and ignores result/subs.
type StepFn func(name string, fn func() (result string, subs []string, err error)) error
```

### `ui.Progress` — production implementation

`ui.NewProgress(out io.Writer, total int) *Progress` returns a `*Progress` whose `Step`
method matches `StepFn`. It:

1. Increments an internal counter; prefixes labels with `[N/total]`.
2. In TTY environments, starts the existing `ui.Step` spinner; completes it with
   `Done(result)` or `Fail(detail)`.
3. After a successful `Done`, prints each sub-result as `  ✓ <label>` (two-space indent,
   `ui.Success`-styled symbol, plain label text).
4. In non-TTY (CI, pipe, `--dry-run`), uses `StartPlainStep` — no ANSI overwriting.

### Visual output — TTY (live)

```
  ⠙ [1/7] Resolve version
✓ [1/7] Resolve version — v1.2.3
  ⠙ [2/7] Generate changelog
✓ [2/7] Generate changelog
  ⠙ [3/7] Commit changelog
✓ [3/7] Commit changelog
  ⠙ [4/7] Create tag v1.2.3
✓ [4/7] Create tag v1.2.3
  ⠙ [5/7] Push tag
✓ [5/7] Push tag
  ⠙ [6/7] Generate release notes
✓ [6/7] Generate release notes
  ⠙ [7/7] Publish to github
✓ [7/7] Publish to github — https://github.com/org/repo/releases/tag/v1.2.3
     ✓ assets uploaded

Released v1.2.3
  › github   https://github.com/org/repo/releases/tag/v1.2.3
```

### Visual output — non-TTY / dry-run

```
[dry-run] ✓ [1/4] Resolve version    — v1.2.3
[dry-run] ✓ [2/4] Generate changelog — would write CHANGELOG.md
[dry-run] ! [3/4] Release notes      — disabled
[dry-run] ✓ [4/4] Publish to github  — would create release
```

### Step total pre-computation

The total step count is computed once at pipeline construction time from the pipeline's
`Config` struct (all boolean flags and slice lengths are known). This avoids a dynamic
counter and enables the `[N/total]` format without a two-pass approach.

`BuildPipeline` and `BuildChangelogPipeline` compute the total and pass it to
`ui.NewProgress`; the resulting `progress.Step` func is stored on the pipeline struct as
its `reporter StepFn`.

### Sub-results and asset upload

Asset upload is logically part of the "publish to platform" step, not a separate numbered
step. The `UploadAssets` call happens inside the step function passed to `reporter`. On
success, the step function returns a sub-result `"assets uploaded"` (or
`"N assets uploaded"` if the count is available). The parent completion line shows the
release URL; the sub-result appears indented beneath it.

If `UploadAssets` fails, the error is returned from the step function and the reporter
renders it as `✗ [N/M] Publish to github — upload failed: ...`. The release was already
created at this point — the error message should include enough context for the user to
retry manually.

### Nil-safety invariant

When `reporter == nil` (the zero value), each pipeline step calls its work function
directly, returning the error. The `result` and `subs` return values are discarded. This
preserves the existing test-friendly behaviour: pipeline unit tests that use `MockRunner`
and do not inject a reporter continue to work without modification.

---

## Affected files

```
internal/ui/
  progress.go        NEW — Progress type, StepFn type, sub-result rendering

internal/pipeline/
  release.go         reporter StepFn field; each step wrapped in reporter(...)
  changelog.go       reporter StepFn field; same; dry-run rewritten step-by-step

internal/cmd/
  release.go         compute total, create ui.NewProgress(out, total), pass to pipeline
  changelog.go       compute total, create ui.NewProgress(out, total), pass to pipeline

internal/app/
  pipeline.go        BuildPipeline / BuildChangelogPipeline accept and thread the reporter
```

No new interfaces in `internal/port/`. No changes to `config/`, `versioning/`,
`generators/`, or `platforms/`.

---

## Consequences

- `heraut release` and `heraut changelog` provide per-step progress in TTY environments,
  with animated spinners and `[N/M]` counters.
- Non-TTY and dry-run output is unchanged in meaning; the format gains step labels.
- Pipeline unit tests are unaffected (nil reporter = silent).
- The `StepFn` func type lives in `internal/ui` — it is not in `internal/port/` because
  it is a presentation concern, not a domain contract.
- The `ui.Progress` type is extractable into a shared library alongside the other
  `internal/ui` helpers if future CLIs need it (noted in the v1.0 goals, roadmap
  overview).
- The sub-result `[]string` signature means step functions that have no sub-results must
  return `nil, nil` for those slots — a minor callsite verbosity cost.
