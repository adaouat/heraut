# Héraut — Release Hooks Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-09-11-release-hooks-design.md`](../superpowers/specs/2026-09-11-release-hooks-design.md)
> Superseded exploration notes: [`docs/superpowers/specs/2026-08-28-release-hooks-exploration.md`](../superpowers/specs/2026-08-28-release-hooks-exploration.md)
> ADRs: new ADR-0053 ("Release lifecycle hooks: arbitrary shell execution via config" — written in T273)
> Main roadmap: tracked as Phase 43 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **release lifecycle hooks** design into incrementally shippable tasks. It
lives in its own file per the design doc's own placement decision, matching the
`changelog-rotation-roadmap.md` / `forge-abstraction-roadmap.md` precedent.

`.heraut.yml` gains a flat `hooks:` block with six points — `post_bump`, `pre_changelog`,
`pre_tag`, `post_tag`, `pre_release`, `post_release` — each a list of shell command strings
(Go `text/template`, substituting `{{ .Version }}`/`{{ .Tag }}`/`{{ .PreviousTag }}`/
`{{ .Platform }}`). Commands run via `sh -c` through the existing GPG-pinentry interactive
runner mode (T260) so output streams live instead of being captured. `post_bump`/`pre_changelog`/
`pre_tag`/`post_tag` fire in both `heraut release` and `heraut changelog --tag`, following any
other step failure's existing abort-with-no-rollback behavior; `pre_release`/`post_release` fire
only in `heraut release`, once per publish target, with a deliberate new isolation rule — a
failing hook skips only that platform, the loop still attempts the rest. Per-env overrides,
non-POSIX shells, and a configurable working directory are explicitly out of scope for this pass
(see design doc "Non-goals").

## Conventions

- Task IDs **continue the global sequence** (`T267+`) so they never collide with the main roadmap
  or any other dedicated roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first), then
  flip `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only
  ([[feedback-no-real-data-in-fixtures]]).
- Additive config change — `hooks:` is a new, entirely optional top-level key. No breaking
  migration, no removed keys, no behavior change for any config that omits it.
- The main `roadmap.md` Phase 43 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description                                                                                    | Status |
|------|--------------------------------------------------------------------------------------------------|--------|
| T267 | `internal/config`: `Hooks` struct + nil-safe accessors + `schema.json` + sample config          | Done |
| T268 | `internal/pipeline`: `runHook` execution helper + interactive-runner access beyond `gitHelper`  | Not started |
| T269 | Wire `post_bump`/`pre_changelog`/`pre_tag`/`post_tag` into both pipelines + `--no-hooks` flag   | Not started |
| T270 | Wire `pre_release`/`post_release` into `release.go`'s per-platform loop, with isolation         | Not started |
| T271 | Dry-run rendering for all six hook points, both pipelines                                       | Not started |
| T272 | Integration test: real-git-repo happy path proving hook execution + templating end to end       | Not started |
| T273 | Docs: `docs/specs/`, README (if applicable), new ADR-0053                                        | Not started |

Sequencing follows the design doc's "Roadmap placement": T267 (config schema) and T268 (execution
helper) have no dependency on each other and can build in either order. T269 depends on both
(needs `Hooks` config to read and `runHook` to call) and also adds the `--no-hooks` flag, since
that flag's only job is skipping the same wiring T269 introduces. T270 depends on T268 and T269's
established wiring pattern in `release.go`, but touches a different section of the file (the
per-platform loop) — sequenced after T269 to avoid unnecessary merge friction from both editing
`release.go` at once. T271 depends on T269 and T270 (dry-run must cover all six points, which only
exist once both are wired). T272 depends on T269 and T270 (exercises real execution end to end).
T273 documents the settled behavior and lands last, alongside ADR-0053.

---

## T267 — `internal/config`: `Hooks` struct + nil-safe accessors + schema + sample

**Files**: `internal/config/config.go` (or a new `internal/config/hooks.go` if the existing file is
already large — check current line count before deciding), `internal/config/config_test.go`,
`schema.json`, `docs/heraut.sample.yml`, `testdata/config/valid/hooks.yml` (new fixture).

Add:

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

`Hooks *Hooks `yaml:"hooks,omitempty"`` added to `Config`. Six nil-safe accessors on `*Config`
(`PostBumpHooks() []string`, etc.) each returning `nil` when `c.Hooks` is nil — mirrors
`Config.Tickets()`/`Config.EnrichmentPolicy()`'s existing pattern in the same file. Strict-parsing
(`loader.go`) already rejects unknown keys for free once the struct field exists; no extra
validator work needed for v1 since every field is a plain string list with no enum/cross-field
constraint (unlike ADR-0052's `BumpRule`, hooks have no matcher shape to validate).

**Test-first**: `TestConfig_HooksAccessors_NilSafe` (all six accessors return nil on a `Config{}`
zero value) and `TestConfig_HooksAccessors_ReturnsConfiguredList` (each accessor returns its
field's value when set) in `config_test.go`. Then the schema/sample/fixture updates, verified by
the existing schema-fixture test harness picking up `testdata/config/valid/hooks.yml`.

- [x] Task complete, roadmap note added, committed

**Completion note (2026-09-11).** Landed as `internal/config/hooks.go` (new file, mirroring the
`commits.go`/`platforms.go` split — `config.go` gained only the one `Hooks *Hooks` field), plus
the two accessor tests in `internal/config/hooks_test.go` (no pre-existing `config_test.go` to add
to). `schema.json` gained a `Hooks` definition and top-level `hooks` property matching the
`Rendering`/`Commits` style (array-of-string properties, `additionalProperties: false`).
`testdata/config/valid/hooks.yml` added and picked up automatically by `TestSchema_ValidFixtures`'s
glob. One deviation from the plan: the sample-config hooks example was first appended after
`release.targets` at end-of-file, but `hk fix -S yamlfmt` re-indented the trailing comment block
under `release.targets[0]` (misleading — implies a per-target field) since a comment-only block at
EOF has no following real key to anchor its indentation to. Moved it to a proper `# ── hooks ──`
section between `changelog:` and `release:` instead, matching every other section's header-comment
pattern — same yamlfmt run then leaves it at column 0. No `--no-hooks` flag or matcher-shape
validation needed yet (deferred to T269/T273 per the design). Full suite + `hk check` green.

---

## T268 — `internal/pipeline`: `runHook` execution helper

**Files**: new `internal/pipeline/hooks.go`, new `internal/pipeline/hooks_test.go`.

```go
// runHook executes cmd via sh -c through r, which must be an interactive runner (stdin/stdout/
// stderr connected to the real terminal — see gitHelper.interactiveRunner) so hook output
// streams live rather than being captured.
func runHook(r port.Runner, cmd string) error {
    if _, _, err := r.RunDir("", nil, "sh", "-c", cmd); err != nil {
        return fmt.Errorf("hook %q: %w", cmd, err)
    }
    return nil
}

// runHooks executes cmds in order via runHook, stopping at the first failure.
func runHooks(r port.Runner, cmds []string) error {
    for _, cmd := range cmds {
        if err := runHook(r, cmd); err != nil {
            return err
        }
    }
    return nil
}
```

`gitHelper.interactiveRunner` (`internal/pipeline/git.go`) is git-specific today. This task also
promotes the same optional-field-with-fallback pattern up to `Pipeline`/`ChangelogPipeline`
directly (a new `interactiveRunner port.Runner` field alongside `git gitHelper`, set by
`WithInteractiveRunner` in addition to threading through to `p.git.interactiveRunner` as it does
now) so hook execution can reach an interactive runner without going through `gitHelper`. When
unset, hooks fall back to the regular runner — same fallback rule `gitHelper.runInteractive`
already uses, so a caller that never calls `WithInteractiveRunner` sees no behavior change.

**Test-first**: `TestRunHook_Success`/`TestRunHook_CommandFails` and
`TestRunHooks_StopsAtFirstFailure`/`TestRunHooks_RunsAllOnSuccess` using `exectest.MockRunner`,
asserting the exact `sh`, `-c`, `"<cmd>"` args reach the runner and that a second command is never
invoked after the first fails.

- [ ] Task complete, roadmap note added, committed

---

## T269 — Wire the four shared hook points + `--no-hooks` flag

**Files**: `internal/pipeline/release.go`, `internal/pipeline/changelog.go`, their `_test.go`
files, `internal/cmd/release.go`, `internal/cmd/changelog.go`, their `_test.go` files,
`internal/pipeline/config.go` (add `Hooks *config.Hooks` — or the resolved four string-slice
fields, whichever keeps `pipeline` from importing `internal/config` if it doesn't already; check
current imports first).

Four new `runStep` calls per pipeline, gated by a `noHooks bool` field (set via a new
`WithNoHooks(bool) *Pipeline`/`WithNoHooks(bool) *ChangelogPipeline` chained option, following the
existing `WithReporter`/`WithLogger`/`WithInteractiveRunner` builder pattern):

- `release.go`: `post_bump` immediately after "Resolve version" (before the dry-run
  short-circuit — a `post_bump` hook is meaningful even for the info a dry-run reports, but per
  the design doc it should not actually *execute* during `--dry-run`; gate hook execution on
  `!p.dryRun` the same way step 2+ already is), `pre_changelog` immediately before "Generate
  changelog", `pre_tag` immediately before "Create tag", `post_tag` immediately after "Push tag".
- `changelog.go`: same four points at the equivalent steps (`post_bump` after "Resolve version",
  `pre_changelog` before "Generate changelog", `pre_tag`/`post_tag` inside the existing `if
  p.cfg.Tag` block around "Create tag"/"Push tag").
- `--no-hooks` added locally to `release` and `changelog` cobra commands only (not root — T266
  convention), threaded to `WithNoHooks(true)` when set.

**Test-first** (per pipeline): a table-driven test asserting each of the four hook points fires at
the right moment with the right templated values, using `exectest.MockRunner` and a fixed
`versioning.Result{Version: "1.2.3", Tag: "v1.2.3", CurrentTag: "v1.2.2"}` — assert the rendered
`sh -c` string contains the substituted version/tag/previous-tag. A second test asserts
`--no-hooks`/`WithNoHooks(true)` skips all four calls to the mock runner entirely (call count
unchanged from a no-hooks-configured baseline). A third confirms a `post_bump` hook still fires
when `DisableChangelog: true` and no `--tag` (the "fires unconditionally on resolve" rule from the
design doc), while `pre_changelog` does not fire in that same case. A fourth asserts a failing
hook at any of the four points aborts the run immediately — `Run()` returns a wrapped error and no
step after the failing hook's point executes (e.g. a failing `pre_tag` hook means "Create tag"
and everything after it never runs) — matching every other `runStep` failure's existing behavior,
with no new rollback of a changelog commit that already landed.

- [ ] Task complete, roadmap note added, committed

---

## T270 — Wire `pre_release`/`post_release` with per-platform isolation

**Files**: `internal/pipeline/release.go`, `internal/pipeline/release_test.go`.

Inside the existing `for _, plat := range p.cfg.Platforms` loop (`release.go`, the "Publish to
%s" `runStep`): a `pre_release` hook run before `plat.CreateRelease(...)`, templated with
`{{ .Platform }}` = `plat.Name()` in addition to the four shared variables. On failure: skip
`CreateRelease`/`UploadAssets`/`post_release` for this platform, record the failure, `continue` to
the next platform instead of `return err`. A `post_release` hook runs after `UploadAssets` (or
after `CreateRelease` when `!plat.HasAssets()`); on failure, warn (via the existing `ui.Warn`
helper `warnNothingToCommit` already demonstrates the pattern for) and `continue`. After the loop,
if any platform was skipped or warned due to a hook failure, `Run()` returns a non-nil error
(a new sentinel or wrapped error listing which platforms failed their hooks) even though every
platform was attempted.

**Test-first**: a three-platform table-driven test using `exectest.MockRunner` and
`testutil.MockPlatform` — (1) a failing `pre_release` hook on platform 1 skips platform 1's
`CreateRelease` entirely but platforms 2 and 3 still get published, and `Run()` returns a non-nil
error; (2) a failing `post_release` hook on platform 2 doesn't prevent platform 2's own
`CreateRelease`/`UploadAssets` (already happened) nor platform 3's attempt; (3) the existing
behavior — a real `CreateRelease` failure (not a hook failure) still aborts the whole loop,
platform 2/3 never attempted — is asserted unchanged, proving the new isolation is scoped to hook
failures only.

- [ ] Task complete, roadmap note added, committed

---

## T271 — Dry-run rendering for all six hook points

**Files**: `internal/pipeline/release.go` (`dryRunOutput`), `internal/pipeline/changelog.go`
(`dryRunOutput`), their `_test.go` files.

Each hook point gets a dry-run line showing the *rendered* command (real values substituted),
consistent with how dry-run already shows real resolved tag names rather than placeholders
elsewhere in both `dryRunOutput` methods — e.g. `[dry-run] would run pre_tag hook: go build
./...`. When multiple commands are configured for one point, one line per command. When
`--no-hooks` is set, dry-run shows no hook lines at all (matching the real-run behavior of
skipping them entirely).

**Test-first**: table-driven dry-run tests (both pipelines, both reporter and plain-writer modes
per the existing `dryRunOutput` test coverage pattern) asserting the exact rendered line text for
each of the six points, and asserting no hook lines appear when `--no-hooks`/`WithNoHooks(true)`.

- [ ] Task complete, roadmap note added, committed

---

## T272 — Integration test: real-git-repo hook execution

**Files**: new test in the existing full-pipeline integration test file (find via
`internal/testutil.RealGitRepo` usage — likely alongside the existing release/changelog
integration tests; check `internal/pipeline/*_test.go` or a dedicated integration test file
before creating a new one).

One happy-path run against a real git repo with `pre_tag` and `post_tag` hooks each configured to
append a marker line to a scratch file (`echo "pre_tag: {{ .Tag }}" >> hooks.log`), proving: (1)
real execution actually happens (not just that `MockRunner` was called correctly), (2) ordering —
`pre_tag`'s line appears before the tag exists (`git tag -l` at that point returns nothing yet),
`post_tag`'s line appears after (`git tag -l` returns the new tag), (3) template substitution
produces the real resolved tag value, not the literal `{{ .Tag }}` string.

**Test-first**: this test is inherently "written and immediately run" rather than red-green in
the usual unit-test sense (there's no separate minimal-implementation step once T269 lands) — but
per the codebase's "no test without a code change" habit, write it expecting failure against a
version of the code checked out before T269/T270 land conceptually, confirm it fails for the
right reason (hooks not implemented / file never written), then confirm it passes on top of
T269+T270's actual code.

- [ ] Task complete, roadmap note added, committed

---

## T273 — Docs and ADR-0053

**Files**: `docs/adr/0053-release-lifecycle-hooks.md` (new), `docs/specs/03-commands.md` (or
wherever `release`/`changelog` command flags and config are documented today — confirm exact file
before editing), `docs/heraut.sample.yml` (already touched in T267; add prose comments explaining
each hook point if not already covered), `README.md` only if it documents config surface at this
level of detail (check before assuming it needs a change).

ADR-0053 follows the design doc's "New ADR-0053 outline" section verbatim: decision (shell
execution via `sh -c` through `port.Runner` in interactive mode — heraut's first config-driven
arbitrary code execution, same trust model as a CI YAML `run:` step), why it needs its own ADR
(first free-form user string passed to any runner; every other call is a structured invocation of
a known binary), and the per-platform hook-failure isolation asymmetry (T270) as a documented
deliberate exception to the pipeline's otherwise-uniform failure handling.

- [ ] Task complete, roadmap note added, committed
