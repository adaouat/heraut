# Héraut — Hook-Declared File Staging Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-09-17-hook-file-staging-design.md`](../superpowers/specs/2026-09-17-hook-file-staging-design.md)
> ADRs: new ADR-0061 ("Hook-declared file staging + object-only hook config" — written in T300)
> Main roadmap: tracked as Phase 51 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **hook-declared file staging** design into incrementally shippable
tasks. It lives in its own file per the design doc's own placement decision, matching the
`release-hooks-roadmap.md` precedent.

A hook entry gains an optional `stage` field: `- run: "php composer.phar config version {{
.Version }}"` `stage: ["composer.json"]`. `stage` patterns pass straight to `git add` (no new
glob dependency — git's own pathspec matching handles wildcards), staged into the same commit as
`CHANGELOG.md`. Valid only under `post_bump`/`pre_changelog` (the only points before the
changelog commit) — a config validation error everywhere else. **Deliberate breaking change**:
every hook point's list entries become object-only (`{run: "...", stage: [...]}` — `run` is
required, `stage` optional); the bare-string shorthand ADR-0053 shipped is no longer accepted.
No blanket `git add -A`/cocogitto-style `add_all` — this was explicitly investigated and rejected
during design (see the design doc's Problem section).

## Conventions

- Task IDs **continue the global sequence** (`T295+`) so they never collide with the main
  roadmap or any other dedicated roadmap.
- Same checkbox markers as the other roadmaps: `[ ]` not started, `[x]` done. Follow the
  two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test
  first), then flip `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only
  ([[feedback-no-real-data-in-fixtures]]).
- **This is a breaking config change**, unlike the additive-only release-hooks epic: any existing
  `.heraut.yml` using the bare-string hook form (shipped since v0.64.0) needs migrating to the
  object form. Stated explicitly in ADR-0061 (T300), not left implicit.
- The main `roadmap.md` Phase 51 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description                                                                                   | Status |
|------|-------------------------------------------------------------------------------------------------|--------|
| T295 | `internal/config`: object-only `HookStep` (custom `UnmarshalYAML`) + `schema.json` + sample     | Done |
| T296 | `internal/config/validator.go`: require non-empty `run`; reject `stage` outside post_bump/pre_changelog | Done |
| T297 | `internal/pipeline` + `internal/app`: plumb `[]HookStep` through rendering/execution, all six points, both pipelines | Done |
| T298 | `internal/pipeline`: widen `commitChangelog`, thread staged patterns into the changelog commit  | Done |
| T299 | Integration test: real-git-repo proof a hook-generated file lands in the changelog commit       | Done |
| T300 | Docs: Spec 02 § `hooks`, guide, sample config, README; new ADR-0061                              | Done |

Sequencing: T295 (config schema) has no dependency on anything else and lands first — every other
task consumes `config.HookStep`. T296 (validator) depends only on T295's struct existing. T297
depends on T295 (needs `config.HookStep` to convert from) and is the largest mechanical task —
every hook call site in both pipelines changes shape, but none of them yet stage anything into a
commit. T298 depends on T297 (needs rendered `Stage` patterns flowing out of the hook-point
helpers) and is where the actual new behavior lands — the smallest task by line count but the
one that matters. T299 depends on T297+T298 (exercises the real, wired behavior end to end). T300
documents the settled behavior and lands last, alongside ADR-0061.

**T295→T297 is one non-green unit, unlike the original hooks epic.** ADR-0053's original T267-T273
were purely additive — each task's commit left `go build ./...`/`go test ./...` green on its own.
This epic is different in kind: T295 *changes* an already-shipped type (`config.HookStep` replaces
the `[]string` every existing caller already consumes), so the moment T295's accessors return
`[]HookStep` instead of `[]string`, `internal/config` (T296's own package) and `internal/app`'s
existing `pCfg.PostBumpHooks = cfg.PostBumpHooks()`-style assignments (still typed `[]string` on
the `pipeline.Config` side until T297 retypes them) stop compiling. This is expected, not a
planning error to fix by reordering — a genuine breaking-type refactor cannot be split into
independently-green commits at the "type definition" / "every consumer" boundary without inventing
a temporary parallel-accessor shim purely to keep intermediate commits green, which is unjustified
complexity for a solo-maintainer pre-v1.0 CLI (YAGNI — see `.claude/rules/coding.md` § Don't
expand scope). Treat T295+T296+T297 as one continuous implementation session: write and commit
each task's own tests and production code in order, but don't expect `go build ./...` to pass
again until T297's last call site is fixed — only then run the full suite and confirm green before
moving to T298. T298, T299, and T300 each land as genuinely independent, individually-green
commits, same as any other task in this codebase.

---

## T295 — `internal/config`: object-only `HookStep`

**Files**: `internal/config/hooks.go`, `internal/config/hooks_test.go`, `schema.json`,
`docs/heraut.sample.yml`, `testdata/config/valid/hooks.yml` (update to object form),
`testdata/config/invalid/hooks_bare_string.yml` (new fixture).

Replace the six `[]string` fields with `[]HookStep`:

```go
// HookStep is one command in a hook point's list (ADR-0053, extended by ADR-0061: object-only,
// stage added). Run is required; Stage is optional and only meaningful under post_bump/
// pre_changelog (internal/config/validator.go enforces the scope — see T296).
type HookStep struct {
    Run   string   `yaml:"run"`
    Stage []string `yaml:"stage,omitempty"`
}

// UnmarshalYAML rejects the plain-string shorthand ADR-0053 originally allowed (ADR-0061): every
// hook entry must be a mapping with at least run. Re-marshals node and decodes it through a
// fresh KnownFields(true) decoder rather than calling node.Decode directly — verified empirically
// that node.Decode alone does NOT honor the outer forge/config.Decode call's KnownFields(true),
// so an unknown field inside a hook entry (e.g. a typo'd `stag:`) would otherwise silently pass
// instead of erroring.
func (h *HookStep) UnmarshalYAML(node *yaml.Node) error {
    if node.Kind == yaml.ScalarNode {
        return fmt.Errorf("expected a mapping, got a plain string %q — wrap it as { run: %q }", node.Value, node.Value)
    }
    type rawHookStep HookStep
    var raw rawHookStep
    b, err := yaml.Marshal(node)
    if err != nil {
        return fmt.Errorf("re-marshaling hook step: %w", err)
    }
    dec := yaml.NewDecoder(bytes.NewReader(b))
    dec.KnownFields(true)
    if err := dec.Decode(&raw); err != nil {
        return err
    }
    *h = HookStep(raw)
    return nil
}

// Hooks configures shell commands run at points in the release lifecycle (ADR-0053, ADR-0061).
type Hooks struct {
    PostBump     []HookStep `yaml:"post_bump,omitempty"`
    PreChangelog []HookStep `yaml:"pre_changelog,omitempty"`
    PreTag       []HookStep `yaml:"pre_tag,omitempty"`
    PostTag      []HookStep `yaml:"post_tag,omitempty"`
    PreRelease   []HookStep `yaml:"pre_release,omitempty"`
    PostRelease  []HookStep `yaml:"post_release,omitempty"`
}
```

`hooks.go` needs `"bytes"` and `"gopkg.in/yaml.v3"` added to its imports. The six accessor
methods (`PostBumpHooks() []string` etc.) change return type to `[]HookStep`; their nil-safety
logic (`if c.Hooks == nil { return nil }`) is unchanged.

`schema.json`: the `hooks` definition's six properties change from `{"type": "array", "items":
{"type": "string"}}` to an array of `{"type": "object", "required": ["run"], "properties": {"run":
{"type": "string"}, "stage": {"type": "array", "items": {"type": "string"}}},
"additionalProperties": false}`. `docs/heraut.sample.yml`'s `hooks:` section (added in T267)
updates every entry to the object form and gains one `stage` example under `post_bump`.

**Test-first**: table-driven `TestHookStep_UnmarshalYAML` in `hooks_test.go` covering: (1) valid
object with `run` only, (2) valid object with `run` + `stage`, (3) bare string → error containing
`expected a mapping`, (4) object with an unknown field (`stag: [...]`, a plausible typo) → error
containing `field stag not found` (proving `KnownFields` strictness survived the custom
unmarshaler — this is the specific regression the re-marshal step exists to prevent; write this
case first and confirm it fails red against a naive `node.Decode(&raw)` implementation before
adding the re-marshal fix, so the test actually proves something). Then
`TestConfig_HooksAccessors_ReturnsConfiguredList` updated for the new element type. Schema/sample
changes verified by the existing `TestSchema_ValidFixtures` glob picking up the updated
`testdata/config/valid/hooks.yml`, plus a new `TestSchema_InvalidFixtures`-style case (check the
existing invalid-fixture test harness's naming convention before adding) for
`hooks_bare_string.yml`.

- [x] Task complete, roadmap note added, committed

`HookStep` and its `UnmarshalYAML` were transcribed verbatim from the design brief: the
KnownFields-strictness regression was proven with genuine red/green TDD — a naive
`node.Decode(&raw)` implementation was written first, the new `unknown field rejected` subtest of
`TestHookStep_UnmarshalYAML` was confirmed to fail against it (yaml silently accepted a typo'd
`stag:` key), then the re-marshal-through-a-fresh-`KnownFields(true)`-decoder fix was applied and
the same test went green. All six `Hooks` accessors now return `[]HookStep`; `schema.json` gained
a `HookStep` definition (`required: [run]`, `additionalProperties: false`) referenced by all six
`hooks.*` array items; `docs/heraut.sample.yml` and `testdata/config/valid/hooks.yml` were
converted to object form, with one `stage` example each under `post_bump`. New invalid fixture
`testdata/config/invalid/hooks_bare_string.yml` proves the bare-string shorthand is rejected at
both the schema level (`TestSchema_InvalidFixtures`) and the `UnmarshalYAML` level (`TestHookStep_
UnmarshalYAML/bare_string_rejected`). As documented above, these changes leave `go build
./...`/`go test ./...` failing for the whole module — `internal/app/pipeline.go` still assigns
`cfg.PostBumpHooks()` (now `[]HookStep`) into `[]string`-typed pipeline config fields; that's
T297's job. Because `hk`'s pre-commit `golangci_lint` step lints `./...` and therefore needs the
whole module to compile, no commit is possible until T297 lands — per the controller's ledgered
ruling, T295's 8 files stay staged (`git add`), uncommitted, and the controller will commit them
on their own once T297 makes the whole module compile again. `internal/config`'s own suite (375
tests) and `go vet ./internal/config/...` are both clean. Nothing was deferred beyond `stage`'s
actual staging behavior and validator scoping, both explicitly out of scope for this task.

---

## T296 — `internal/config/validator.go`: `stage` scope + required `run`

**Files**: `internal/config/validator.go`, `internal/config/validator_test.go`.

```go
// validateHooks validates every configured hook entry (ADR-0061): run is required everywhere,
// and only post_bump/pre_changelog entries may set stage — pre_tag/post_tag/pre_release/
// post_release all fire after the changelog commit already landed, so a file staged there has no
// commit left to receive it.
func validateHooks(cfg *Config) []ValidationError {
    if cfg.Hooks == nil {
        return nil
    }
    var errs []ValidationError
    points := []struct {
        name       string
        steps      []HookStep
        allowStage bool
    }{
        {"post_bump", cfg.Hooks.PostBump, true},
        {"pre_changelog", cfg.Hooks.PreChangelog, true},
        {"pre_tag", cfg.Hooks.PreTag, false},
        {"post_tag", cfg.Hooks.PostTag, false},
        {"pre_release", cfg.Hooks.PreRelease, false},
        {"post_release", cfg.Hooks.PostRelease, false},
    }
    for _, p := range points {
        for i, step := range p.steps {
            base := fmt.Sprintf("hooks.%s[%d]", p.name, i)
            if step.Run == "" {
                errs = append(errs, ValidationError{
                    Path: base + ".run", Message: "required",
                    Hint: `every hook entry needs a run command, e.g. { run: "echo hi" }`,
                })
            }
            if !p.allowStage && len(step.Stage) > 0 {
                errs = append(errs, ValidationError{
                    Path: base + ".stage", Message: "not allowed here",
                    Hint: "stage is only valid under post_bump or pre_changelog (files staged there have no commit left to land in)",
                })
            }
        }
    }
    return errs
}
```

Register it in `Validate()` (`internal/config/validator.go:45-60`): add
`errs = append(errs, validateHooks(cfg)...)` alongside the existing `validateForges(cfg)` call.

**Test-first**: table-driven `TestValidateHooks` in `validator_test.go`, rows: (1) empty `run` on
`post_bump[0]` → one error at `hooks.post_bump[0].run`; (2) `stage` set on `pre_tag[0]` → one
error at `hooks.pre_tag[0].stage`; (3) `stage` set on `post_bump[0]` → no error; (4) `stage` set
on `pre_changelog[0]` → no error; (5) both a `stage`-on-`pre_release` violation and an empty `run`
in the same config → both errors collected (proving `Validate` doesn't stop at the first); (6)
`cfg.Hooks == nil` → nil, no panic.

- [x] Task complete, roadmap note added, committed

**Completion note (2026-09-18).** `validateHooks` landed exactly as specified — one function,
registered in `Validate()` alongside `validateForges`, both rules (`run` required everywhere,
`stage` scoped to `post_bump`/`pre_changelog`) independent and both collected (no early return).
One fix round: the first draft's `TestValidateHooks` routed the two "stage is valid here" rows
through a shared assertion branch that hardcoded a check against `hooks.post_bump[0].run` — a
tautology for one row, a check against a path that couldn't exist for the other — so neither row
actually exercised the behavior it was named for. Caught in task review, fixed by refactoring the
table-driven test into individual `t.Run()` subtests with `assert.Empty(t, config.Validate(cfg))`
for both valid-stage cases, verified in a scoped re-review. `internal/config`'s own suite green
(382 tests); whole-module build still intentionally broken at this point (T297's job, landed
next). Commit deferred alongside every other task in this epic — see T295's note for why.

---

## T297 — Plumb `[]HookStep` through rendering/execution, both pipelines

**Files**: `internal/pipeline/hooks.go`, `internal/pipeline/hooks_test.go`,
`internal/pipeline/config.go`, `internal/pipeline/release.go`, `internal/pipeline/changelog.go`,
their `_test.go` files, `internal/app/pipeline.go`, `internal/app/hooks_internal_test.go`.

This task is purely mechanical type-plumbing — every hook call site in both pipelines changes
shape to carry `Stage`, but nothing actually stages a file into a commit yet (that's T298). Do
not skip `internal/app` here: T269's own completion note flagged this exact omission as the one
thing that silently breaks real `heraut release`/`heraut changelog` runs when missed.

**`internal/pipeline`** gets its own `HookStep` (deliberately **not** `config.HookStep` — this
package has never imported `internal/config`, translating instead at the `internal/app` boundary
for every other config field; reusing `config.HookStep` here would be the first crack in that
separation):

```go
// HookStep is one command in a hook point's list, translated from config.HookStep by
// internal/app (ADR-0053, ADR-0061) — this package never imports internal/config.
type HookStep struct {
    Run   string
    Stage []string
}

// renderedHookStep is a HookStep after template substitution.
type renderedHookStep struct {
    Run   string
    Stage []string
}

// renderHookStep renders step's Run and every Stage entry against vars.
func renderHookStep(step HookStep, vars hookVars) (renderedHookStep, error) {
    run, err := renderHookCmd(step.Run, vars)
    if err != nil {
        return renderedHookStep{}, err
    }
    stage := make([]string, 0, len(step.Stage))
    for _, s := range step.Stage {
        rendered, err := renderHookCmd(s, vars)
        if err != nil {
            return renderedHookStep{}, err
        }
        stage = append(stage, rendered)
    }
    return renderedHookStep{Run: run, Stage: stage}, nil
}

// renderHookSteps renders each step in order, stopping at the first render error.
func renderHookSteps(steps []HookStep, vars hookVars) ([]renderedHookStep, error) {
    rendered := make([]renderedHookStep, 0, len(steps))
    for _, s := range steps {
        r, err := renderHookStep(s, vars)
        if err != nil {
            return nil, err
        }
        rendered = append(rendered, r)
    }
    return rendered, nil
}

// runHookPointSteps renders steps against vars and executes each rendered Run command in order
// via r, stopping at the first render or execution error. Returns the rendered steps so callers
// that need declared Stage patterns (post_bump/pre_changelog — T298) can collect them; the other
// four points simply discard the return value (config validation, T296, already guarantees their
// Stage is always empty).
func runHookPointSteps(r port.Runner, steps []HookStep, vars hookVars) ([]renderedHookStep, error) {
    rendered, err := renderHookSteps(steps, vars)
    if err != nil {
        return nil, err
    }
    for _, s := range rendered {
        if err := runHook(r, s.Run); err != nil {
            return nil, err
        }
    }
    return rendered, nil
}

// stagePatterns flattens every Stage pattern across rendered steps, in order.
func stagePatterns(steps []renderedHookStep) []string {
    var patterns []string
    for _, s := range steps {
        patterns = append(patterns, s.Stage...)
    }
    return patterns
}
```

`shouldRunHooks(dryRun, noHooks bool, cmds []string) bool` (`hooks.go`) becomes
`shouldRunHooks(dryRun, noHooks bool, steps []HookStep) bool` — body unchanged (`len(steps) >
0`). `dryRunHookLines(cmds []string, vars hookVars)` becomes `dryRunHookLines(steps []HookStep,
vars hookVars)`, rendering each step and emitting one `"[dry-run] would run: <run>"` line plus
one `"[dry-run] would stage: <pattern>"` line per non-empty `Stage` entry (Design §5 of the
2026-09-17 design doc). `runHookPoint` (the old `[]string`-based executor) is deleted — replaced
by `runHookPointSteps` above.

**`internal/pipeline/config.go`**: `Config`'s and `ChangelogConfig`'s six `PostBumpHooks
[]string`-shaped fields become `[]HookStep`.

**`internal/pipeline/release.go` + `changelog.go`**: every method that threads hook commands
(`runHookPointStep`, `runOrRenderHookPoint`, `dryRunHookLinesOrNil`, `dryRunHookStep`,
`printDryRunHookLinesPlain`) changes its `cmds []string` parameter to `steps []HookStep`.
`runHookPointStep` and `runOrRenderHookPoint` additionally change their return type from `error`
to `([]string, error)` — the first return is the rendered `Stage` patterns collected during a real
(non-dry-run, non-skipped) execution; `nil` in every other case (skipped, dry-run, or no hooks
configured) since T298's caller only needs the value on the real-execution path.

```go
func (p *Pipeline) runHookPointStep(name string, steps []HookStep, vars hookVars) ([]string, error) {
    if !shouldRunHooks(p.dryRun, p.cfg.NoHooks, steps) {
        return nil, nil
    }
    var staged []string
    err := p.runStep(name, func() (string, []string, error) {
        rendered, err := runHookPointSteps(p.git.interactiveOrRunner(), steps, vars)
        if err != nil {
            return "", nil, err
        }
        staged = stagePatterns(rendered)
        return "", nil, nil
    })
    return staged, err
}

func (p *Pipeline) runOrRenderHookPoint(name string, steps []HookStep, vars hookVars) ([]string, error) {
    if !p.dryRun {
        return p.runHookPointStep(name, steps, vars)
    }
    if p.reporter != nil {
        return nil, p.dryRunHookStep(name, steps, vars)
    }
    return nil, p.printDryRunHookLinesPlain(steps, vars)
}
```

(`ChangelogPipeline`'s copies mirror these exactly, matching the existing per-type duplication
convention.) Callers in `Run()` change from `if err := p.runHookPointStep(...); err != nil` to
`if _, err := p.runHookPointStep(...); err != nil` for the four points that don't need the
staged-patterns return (`pre_tag`/`post_tag`, and `post_bump`/`pre_changelog` at their *call
sites* stay bare `error`-checking too at this task — T298 is what actually captures and uses the
first return value).

**`internal/app/pipeline.go`**: new conversion helper (used at every one of the ten existing
`pCfg.XHooks = cfg.XHooks()` / `cCfg.XHooks = cfg.XHooks()` assignment lines, `pipeline.go:323-328`
and `:447-450`):

```go
// toPipelineHookSteps converts config.HookStep (the YAML-facing shape) to pipeline.HookStep
// (the pipeline package's own shape) — internal/pipeline never imports internal/config, so this
// conversion happens at the app-layer boundary like every other config → pipeline.Config field.
func toPipelineHookSteps(steps []config.HookStep) []pipeline.HookStep {
    out := make([]pipeline.HookStep, len(steps))
    for i, s := range steps {
        out[i] = pipeline.HookStep{Run: s.Run, Stage: s.Stage}
    }
    return out
}
```

e.g. `pCfg.PostBumpHooks = toPipelineHookSteps(cfg.PostBumpHooks())`. The two `hookRuns := func(cmds
[]string) bool { ... }` closures (`pipeline.go:121`, `:174`, used by `releaseStepTotal`/
`changelogStepTotal` for the `[N/total]` step count) change their parameter type to `[]config.HookStep`
— body (`!cfg.NoHooks && len(cmds) > 0`) is unchanged, since only the length matters for counting.

**Test-first**: extend the existing hook-rendering/execution tests in `hooks_test.go` for the new
`[]HookStep` shape (`TestRenderHookStep_RendersRunAndStage`, `TestRenderHookSteps_StopsAtFirstError`,
`TestRunHookPointSteps_ReturnsRenderedStepsOnSuccess`,
`TestRunHookPointSteps_StopsAtFirstExecutionFailure`, `TestStagePatterns_FlattensInOrder`). Extend
the existing per-pipeline dry-run/no-hooks table tests in `release_test.go`/`changelog_test.go`/
`dryrun_hooks_test.go` for the new signatures — assert a `stage`-bearing step's dry-run output
includes both the `would run:` and `would stage:` lines. New `TestToPipelineHookSteps` in
`internal/app` covering the empty-slice and populated cases.

- [x] Task complete, roadmap note added, committed

`internal/pipeline` gained its own `HookStep`/`renderedHookStep` plus `renderHookStep`,
`renderHookSteps`, `runHookPointSteps`, and `stagePatterns`, transcribed verbatim from this
brief. `runHookPoint` was deleted as specified; the now-dead `renderHookCmds` and the batch
`runHooks([]string)` helper (nothing called them once `runHookPointSteps`/`dryRunHookLines` were
rewritten against `renderHookSteps`) were deleted too, along with their now-orphaned unit tests —
not explicitly named in the brief but flagged as a judgment call in the task report.
`Config`/`ChangelogConfig`'s six hook fields are now `[]HookStep`; `runHookPointStep`/
`runOrRenderHookPoint` return `([]string, error)` exactly as specified, and every `Run()` call
site for the four non-staging points discards the first value with `_`; `release.go`'s inline
pre_release/post_release block (which called `runHookPoint` directly, not through
`runHookPointStep`) now calls `runHookPointSteps` the same way. `internal/app/pipeline.go` gained
`toPipelineHookSteps` verbatim, wired into all ten `pCfg.XHooks`/`cCfg.XHooks` assignments; the
two `hookRuns` step-count closures were retyped to `[]pipeline.HookStep` (the brief's prose said
`[]config.HookStep`, which doesn't type-check against the `*pipeline.Config`/`*pipeline.
ChangelogConfig` fields actually passed in — treated as a wording slip). One out-of-brief fixture
fix was required to make `go test ./...` pass for the whole module:
`internal/cmd/changelog_hooks_realrepo_test.go`'s two `.heraut.yml` fixtures still used the
bare-string hook shorthand T296's `HookStep.UnmarshalYAML` (ADR-0061) now rejects; updated both
to the `- run: "..."` mapping form. `go build ./...`, `go vet ./...`, and `go test ./...` are all
clean for the entire module (1857 tests, 26 packages) — the whole-module break T295/T296 left
behind is fully closed. Nothing was deferred beyond `stage`'s actual staging-into-commit
behavior, explicitly out of scope for this task (T298's job).

---

## T298 — Thread staged patterns into the changelog commit

**Files**: `internal/pipeline/git.go`, `internal/pipeline/git_test.go`,
`internal/pipeline/release.go`, `internal/pipeline/changelog.go`, their `_test.go` files.

`commitChangelog` widens to accept every path to stage, not just the changelog file:

```go
// commitChangelog stages files (the changelog path plus any hook-declared stage patterns,
// ADR-0061) and commits them with msg, pushing when push is set. Reports whether a commit was
// actually created: when `git add` stages nothing across every path — every file byte-identical
// to the last commit — it returns (false, nil) without committing so the caller can warn and
// continue to tag/publish rather than failing on git's "nothing to commit" exit. A files entry
// that matches nothing on disk is a `git add` failure like any other, propagated as-is — no new
// zero-match detection needed (ADR-0061 Design §4).
func (g *gitHelper) commitChangelog(files []string, msg string, push bool) (bool, error) {
    if err := g.run("git", append([]string{"add"}, files...)...); err != nil {
        return false, fmt.Errorf("git add: %w", err)
    }
    staged, err := g.hasStagedChanges()
    if err != nil {
        return false, err
    }
    if !staged {
        return false, nil
    }
    if err := g.runInteractive("git", "commit", "-m", msg); err != nil {
        return false, fmt.Errorf("git commit: %w", err)
    }
    if push {
        if err := g.run("git", "push", "origin", "HEAD"); err != nil {
            return false, fmt.Errorf("git push: %w", err)
        }
    }
    return true, nil
}
```

`internal/pipeline/release.go`'s `Run()` (`release.go:179-240`): capture `post_bump`'s staged
patterns, accumulate `pre_changelog`'s, pass both into `commitChangelog`:

```go
postBumpStage, err := p.runOrRenderHookPoint("Run post_bump hooks", p.cfg.PostBumpHooks, p.hookVars(result))
if err != nil {
    return err
}

// ... (dry-run early return unchanged) ...

if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
    preChangelogStage, err := p.runHookPointStep("Run pre_changelog hooks", p.cfg.PreChangelogHooks, p.hookVars(result))
    if err != nil {
        return err
    }
    stage := append(append([]string{}, postBumpStage...), preChangelogStage...)

    // ... (Generate changelog step unchanged) ...

    file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
    var committed bool
    if err := p.runStep("Commit changelog", func() (string, []string, error) {
        var cerr error
        committed, cerr = p.git.commitChangelog(append([]string{file}, stage...), commitMessage(p.cfg.CommitMessage, result.Version), true)
        if cerr != nil {
            return "", nil, fmt.Errorf("committing changelog: %w", cerr)
        }
        return "", nil, nil
    }); err != nil {
        return err
    }
    if !committed {
        warnNothingToCommit(p.out, file)
    }
}
```

`internal/pipeline/changelog.go`'s `Run()` (`changelog.go:179-253`) mirrors this exactly, with one
wrinkle: `postBumpStage` is captured before the `DisableChangelog` branch (`changelog.go:200-210`)
that can return early — when it does, `postBumpStage` is simply never used, which is fine (Go
doesn't complain about an unused named-but-assigned variable used in at least one branch; this is
already the existing pattern for `result`). The `commitChangelog` call site (`changelog.go:239`)
gets the same `append([]string{file}, stage...)` treatment, `push: !p.cfg.NoPush` unchanged.

Every other `commitChangelog` caller in tests (`git_test.go`) updates its `file` argument from a
bare string to a one-element `[]string{file}` slice — confirm no other production caller exists
before assuming this is the only production call-site change (grep `commitChangelog(` across
`internal/pipeline` first).

**Test-first**: extend `git_test.go`'s existing `TestCommitChangelog_*` table for the widened
signature (a `files []string` column instead of a single `file string` — every existing row
becomes `files: []string{"CHANGELOG.md"}` to stay equivalent), plus new rows:
`TestCommitChangelog_StagesMultipleFiles` (two files given, `MockRunner` asserts a single `git
add CHANGELOG.md composer.json` call, not two separate `git add` calls), `TestCommitChangelog_
ChangelogUnchangedButStagedFileChanged_StillCommits` (only the second file has a real diff —
`hasStagedChanges` still returns true, a commit is still created), `TestCommitChangelog_
ZeroMatchStagePattern_PropagatesGitAddError` (a `MockRunner` queued response simulating git's
"pathspec did not match any files" exit — `commitChangelog` returns that error unwrapped-friendly,
no commit attempted). Extend `release_test.go`/`changelog_test.go`'s existing `TestRun_
CommitsChangelog`-shaped tests with a new row where `post_bump` declares `stage: ["extra.txt"]` —
assert the `git add` call in the `MockRunner`'s recorded calls includes both `CHANGELOG.md` and
`extra.txt`.

- [x] Task complete, roadmap note added, committed

**Completion note (2026-09-19).** Landed exactly as specified: `commitChangelog` widened to
`(files []string, msg string, push bool)`, one `git add` call staging every path;
`release.go`/`changelog.go` both capture `postBumpStage`/`preChangelogStage` and thread
`append([]string{file}, stage...)` into the commit, with the other four hook points still
discarding the return value. No new zero-match detection code — a `git add` failure on an
unmatched `stage` pattern propagates as-is, exactly per the design. One interruption mid-task:
the first implementer session was cut off by an infrastructure rate limit after finishing all
three production files correctly but before finishing `git_test.go` or staging anything; a second
session (with no code to redo, only tests to finish) completed the three brief-named
`TestCommitChangelog_*` cases plus a `Run()`-level stage-inclusion test per pipeline, all using
real `MockRunner` argument assertions (exact `git add` call contents, not just absence of error).
Task review: Approved on the first pass, zero fix rounds — 3 Minor findings deferred to the final
whole-branch review (no `Run()`-level test for `pre_changelog`'s own `Stage` contribution or the
combined `post_bump`+`pre_changelog` order; duplicate stage paths across both points neither
deduplicated nor tested, almost certainly harmless since `git add` tolerates repeated pathspecs;
the `append(append(...))` combine pattern could use `slices.Concat` — purely stylistic). Whole
module: `go build`/`go vet`/`go test` all clean, 1864 tests across 26 packages. Commit deferred
alongside every other task in this epic — see T295's note for why.

---

## T299 — Integration test: real-git-repo proof

**Files**: `internal/cmd/changelog_hooks_realrepo_test.go` (existing file from T272 — add to it
rather than creating a new one; confirm current contents/structure before editing).

One additional case in the existing real-git-repo hook test file: a `post_bump` hook writes a
scratch file (`echo "1.0.0" > version.txt`) and declares `stage: ["version.txt"]`; run `heraut
changelog --tag --no-push` against a real repo (mirroring T272's existing pattern exactly, same
`GIT_CONFIG_GLOBAL=/dev/null`/`GIT_CONFIG_SYSTEM=/dev/null` isolation); assert via `git show
--name-only HEAD` that the resulting commit's tree contains both `CHANGELOG.md` and `version.txt`.
A second case: a `post_bump` hook declares `stage: ["does-not-exist.txt"]` (nothing creates that
file) — assert the command exits non-zero, the error mentions `git add`, and `git log --oneline`
shows no new commit was created (the changelog commit never happened, since `git add` failed
before reaching `git commit`).

**Test-first**: per T272's own convention — write expecting failure against pre-T298 behavior
(staged file never lands, or the zero-match case never errors since nothing stages it today),
confirm it fails for the right reason, then confirm it passes on top of T297+T298's actual code.

- [x] Task complete, roadmap note added, committed

**Completion note (2026-09-19).** Two real-git-repo cases added to the existing
`internal/cmd/changelog_hooks_realrepo_test.go` (from T272): a `post_bump` hook that writes a
scratch file and declares `stage: ["version.txt"]` produces a `heraut changelog --tag --no-push`
commit whose tree contains both `CHANGELOG.md` and `version.txt` (verified via `git show
--name-only HEAD`); a `post_bump` hook declaring `stage` on a file nothing creates aborts with a
non-zero exit, `git add` visible in the CLI's own output, and no new commit (`git log --oneline`
byte-identical before/after). Both tests proven to have teeth by temporarily severing the `stage`
wiring in `internal/pipeline/changelog.go`, confirming both failed for the right reason, then
reverting — confirmed independently by the reviewer, who also re-ran both 3× with `-race` (12/12
green). The two pre-existing T272 tests in this file needed one narrow, necessary edit: their YAML
fixtures moved from the bare-string hook shorthand to the `run:` mapping form, forced by T296's
already-staged validation (bare strings are now a config error) — their own assertions are
unchanged. Whole module: `go build`/`go vet` clean, 1866 tests. Commit deferred alongside every
other task in this epic — see T295's note for why.

---

## T300 — Docs and ADR-0061

**Files**: `docs/adr/0061-hook-file-staging.md` (new — confirm 0061 is still the next free number
before creating; another ADR may have landed in the meantime), `docs/specs/02-configuration.md`,
`docs/guides/release-pipeline-and-hooks.md`, `docs/heraut.sample.yml` (already touched in T295;
confirm prose comments cover `stage` too), `README.md` only if it documents hook config at this
level of detail (T273's precedent found it didn't need touching — check the same reasoning still
holds), `CLAUDE.md` (ADR count, same drift-prevention habit as every prior ADR-adding task).

ADR-0061 follows the design doc's "New ADR-0061 outline" section verbatim: the two coupled
decisions (object-only hook entries; opt-in `stage` scoped to post_bump/pre_changelog), why
`add_all` was rejected (the design doc's Problem section, including the cocogitto source citation:
`crates/cocogitto/src/command/bump/standard.rs`, `self.repository.add_all()`), and why `stage` is
scope-validated rather than silently inert elsewhere.

`docs/specs/02-configuration.md`'s existing `## hooks` section (added T273) needs: its YAML
example updated to the object form throughout, a new `### stage (ADR-0061)` subsection (semantics,
the scope-validation table, the "no glob dependency — git's own pathspec matching" note, the
zero-match-is-an-error behavior), and its `hooks:` intro paragraph's "each key is a list of shell
commands" line corrected to describe the object shape.
`docs/guides/release-pipeline-and-hooks.md`'s two mermaid diagrams don't need new nodes (`stage`
doesn't add a pipeline step, just an effect of `post_bump`/`pre_changelog`), but the prose above
each diagram should note that those two hook points' commands may also declare files staged into
the following changelog commit, with a pointer to Spec 02's new subsection.

**Test-first**: none — this is a docs-only task, verified by proofreading against the design doc
and by `hk check` (`typos`, `yamlfmt`) passing on the new/changed files, matching T273's own
precedent.

- [x] Task complete, roadmap note added, committed

**Completion note (2026-09-19).** New `docs/adr/0061-hook-file-staging.md` expands the design
doc's outline into a full ADR (Context/Decision/Consequences/Alternatives), citing the cocogitto
`add_all` source investigation (`crates/cocogitto/src/command/bump/standard.rs`,
`self.repository.add_all()` → `git2::Index::add_all(["."], ...)`) as the explicit reason a
blanket-stage mode was rejected, and recording both coupled decisions (object-only hook entries;
`stage` scoped to `post_bump`/`pre_changelog`) with their own Alternatives-considered entries.
`docs/adr/README.md`'s index gained the ADR-0061 row — not in T300's own file list, but every
prior ADR-adding task in this repo's history updates it in the same commit (verified via `git log
--diff-filter=A -- docs/adr/*.md`), so treated as part of "write ADR-0061," not scope creep. Spec
02's `## hooks` section: YAML example converted to the object form throughout (with one `stage`
example under `post_bump`), the "each key is a list of shell commands" line corrected, and a new
`### \`stage\` (ADR-0061)` subsection added (semantics, the six-row scope-validation table, the
no-glob-dependency note, the zero-match-is-a-`git add`-error behavior) — placed after "Template
variables" since `stage` entries render through the same engine. The guide
(`docs/guides/release-pipeline-and-hooks.md`) needed no new mermaid nodes (confirmed: `stage` is
an effect of `post_bump`/`pre_changelog`, not a new pipeline step) — added one prose bullet under
each of the two diagrams pointing at Spec 02's new subsection, including a note that a
disabled-changelog-and-no-`--tag` `heraut changelog` run discards any `post_bump`-declared `stage`
silently (no commit exists to receive it, and that's not a validation error — validated scope is
about *which hook point*, not *whether a commit happens this run*). `docs/heraut.sample.yml`'s
existing `hooks:` comment block (from T295) already explained `stage`'s shape; tightened the
wording from "optional, only meaningful under post_bump/pre_changelog" (read as merely inert
elsewhere) to state plainly that it is a config validation error elsewhere, plus the no-new-glob-
dependency and zero-match-error notes, matching Spec 02's fuller treatment. README.md: re-verified
T273's precedent — it still doesn't mention `hooks:` at all, and its Configuration section still
states "that's the shape, not the whole schema" pointing to Spec 02 — left untouched. CLAUDE.md's
ADR count was stale at "58 ADRs" in two places (already off before this task, per the project's
known drift pattern); corrected both to "61 ADRs" — `ls docs/adr/*.md` counted 61 files before
this task (60 numbered ADRs, 0001-0060, plus the index `README.md`); adding `0061-hook-file-
staging.md` brings the numbered-ADR count to 61. All touched files pass `hk check`
(`yamlfmt`, `typos`) clean, matching T273's own docs-only verification precedent. Commit deferred
alongside every other task in this epic — see T295's note for why; this is the epic's final task,
so all of T295–T300 land together once the controller commits them in order.

This closes the hook-file-staging epic once all of T295–T300 are done.
