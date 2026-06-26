# Design: Interactive commit wizard (`heraut commit create`)

- **Date**: 2026-06-25
- **Status**: Approved (brainstorming) — pending implementation plan
- **Author**: bchatard (with Claude)
- **Roadmap task**: T120 (to be added)
- **ADR**: 0031 (to be written during implementation)
- **Supersedes notes in**: ADR-0027 "Related future work", ADR-0030 "Future ideas"

---

## Summary

`heraut commit create` is an interactive, TTY-only, [meteor](https://github.com/stefanlogue/meteor)-style
wizard that guides the user through building a Conventional Commits message
(type → scope → subject → breaking → body → footers → preview-confirm), assembles
the canonical message text, validates it through heraut's existing `VerifyCommit`
logic, and runs `git commit`. It reuses the `internal/conventionalcommit` grammar
package and the `commit_lint` config so that any commit the wizard produces is
guaranteed to pass `heraut commit verify`.

It joins the existing `heraut commit` family:

| Command | Role | Use case |
|---|---|---|
| `heraut commit verify [message]` | validate a single message | commit-msg hook |
| `heraut commit check [rev-range]` | validate a range/history | CI |
| **`heraut commit create`** | **interactively author a message + commit** | **local authoring** |

---

## Goals

- Guided authoring of a valid Conventional Commits message with zero grammar knowledge required.
- Output is **guaranteed** to pass `heraut commit verify` (same defaults-or-config logic, no drift).
- Reuse existing building blocks: `conventionalcommit` (grammar), `commit_lint` (types), the
  `huh` dependency, and the `port.Runner` git abstraction.
- Keep `internal/cmd` thin and keep the interactive form isolated from testable logic.

## Non-goals (v1)

- **No ticket-pattern integration.** Footers are generic trailers in v1; coupling to the
  top-level `tickets:` config (ADR-0024) is deferred to v2.
- **No per-file staging picker.** Only a lightweight "stage all / cancel" prompt.
- **No `--amend`, `--signoff`, or other git passthroughs** beyond `--all/-a`.
- **No non-interactive / flag-driven assembly.** Scripts use `git commit` or `commit verify`.
- **`commit_lint.scopes` is NOT enforced by `verify`/`check`.** It guides the wizard only
  (ADR-0027 deliberately kept `verify` minimal — types only). Enforcing scopes in `verify`
  would expand that contract and is its own future decision.

---

## Command surface

```
heraut commit create
```

- **Inherited global flags**: `--config`, `--dry-run`, `--verbose`.
- **Local flag**: `--all`, `-a` — passthrough to stage tracked modifications (`git commit -a`).
- **No aliases** in v1.

---

## UX flow (TTY)

```
$ heraut commit create
  ① pre-check: anything staged?   (skipped entirely when --all/-a is passed)
       └─ no → ? Nothing staged. › Stage all (git add -A) / Cancel
  ② ? type     › feat  fix  docs  chore  refactor  test  style  perf  ci  build
       sourced from app.AllowedCommitTypes(cfg); the 10 known types show a built-in
       one-line description; custom configured types show a bare label.
  ③ ? scope    › <configured scopes> / (custom…) / (none)   [if commit_lint.scopes set]
       ? scope    ____________                              [else: free text, optional]
  ④ ? subject  ____________   (required, single-line, non-empty)
  ⑤ ? breaking change?  (y/N)
       └─ yes → ? describe ____________  → adds "!" to header + "BREAKING CHANGE:" footer
  ⑥ ? body     (optional, multi-line editor)
  ⑦ ? footers  (optional, multi-line; each line validated against the footer grammar)
  ⑧ preview the assembled message + ? Commit this? (Y/n)
  ⑨ app.VerifyCommit(cfg, message) guard → git commit -F <tmpfile>
```

### Field rules

| Field | Required | Source / validation |
|---|---|---|
| type | yes | select from `app.AllowedCommitTypes(cfg)` |
| scope | no | select from `commit_lint.scopes` (+ custom/none) if set; else free text |
| subject | yes | non-empty, single-line |
| breaking | no | confirm; if yes, capture a description → `!` in header + `BREAKING CHANGE:` footer |
| body | no | free multi-line |
| footers | no | one optional multi-line block; each line validated against `footerLinePattern` |

### `--dry-run`

Runs steps ②–⑧, prints the assembled message and `[dry-run] would run: git commit`, then
exits without staging or committing.

### Non-TTY

If stdin or stdout is not a terminal, exit immediately (before any prompt) with
`commit create requires an interactive terminal` and the Usage exit code.

---

## Architecture (Approach A / A2a)

### New serializer in `internal/conventionalcommit/`

```go
// Format renders a Commit back to its canonical Conventional Commits message text.
// Round-trips with Parse: for any c built by the wizard, Parse(c.Format()) reproduces c.
func (c *Commit) Format() string
```

Assembly: `type(scope)!: description`, blank line, body, blank line, footer block
(one trailer per line as `Token: Value`; the breaking trailer uses the spec's literal
`BREAKING CHANGE` token). Scope, `!`, body, and footers are omitted when empty.

### New package `internal/commitwizard/`

Sibling to `internal/scaffold` (the `heraut init` precedent).

| Unit | Purpose | Tested |
|---|---|---|
| `Answers` struct | `Type, Scope, Subject, Body string; Breaking bool; BreakingDesc string; Footers []conventionalcommit.Footer` | — |
| `Assemble(Answers) *conventionalcommit.Commit` | pure mapping Answers → Commit | unit |
| `Run(runner port.Runner, cfg *config.Config, opts Options) error` | orchestrator: TTY check → staging → `collectAnswers` → `Assemble` → `Format` → `VerifyCommit` guard → commit | partial |
| `hasStaged`, `stageAll`, `commit` | git helpers (`git diff --cached --quiet`, `git add -A`, `git commit -F <tmpfile>`) | contract (MockRunner) |
| `collectAnswers(cfg) (Answers, error)` | the huh form | excluded from coverage |

**Imports**: `commitwizard` → `conventionalcommit`, `config`, `port`, `app`, `ui`, `huh`.

**Layering note (A2a)**: `commitwizard` importing `app` is a new edge (nothing imports into
`app` today). It is acyclic — `cmd → commitwizard → app`, and `app` carries no UI deps.
This was chosen over pushing the type-list/verify logic into a lower package, to keep a
single source of truth for "types come from config-or-default" without spreading it.

### Reuse of `VerifyCommit` logic (per user instruction)

Extract a shared helper so the wizard and the verifier never drift:

```go
// AllowedCommitTypes returns cfg.CommitLint.Types when set, else DefaultCommitTypes.
func AllowedCommitTypes(cfg *config.Config) []string
```

- `VerifyCommit` is refactored to use it (behavior-preserving; existing tests stay green).
- The wizard's type step (②) reads from it.
- After assembly, `Run` calls `app.VerifyCommit(cfg, message)` as a guard **before** `git commit`.

### `internal/cmd/commit.go`

Add a thin `newCommitCreateCmd()`: read `--all` + global flags → build the dry-run-aware
Runner (`execadapter.New(dryRun, verbose)`) → load + validate config (same prelude as
`verify`/`check`) → `commitwizard.Run(...)` → map errors to exit codes.

### Git mechanism

`port.Runner.Run(name, args...)` has **no stdin** (forge v0.14.0). Multi-line messages are
written to a temp file and committed with `git commit -F <tmpfile>` — mirroring how
`gitcliff` writes a temp config. The temp file is always removed (`defer os.Remove`,
including on the error path).

---

## Config addition: `commit_lint.scopes`

```go
type CommitLint struct {
    Types  []string `yaml:"types,omitempty"`
    Scopes []string `yaml:"scopes,omitempty"` // wizard-only; not enforced by verify/check
}
```

Optional; no semantic validation beyond "list of strings". Empty/unset → free-text scope step.

Per the coding rules, keep these in sync (all touched by the implementation task):

- `internal/config/config.go` — the field + wizard-only comment
- `schema.json` — `scopes` under `commit_lint` (array of strings, description)
- `docs/heraut.sample.yml` — show `scopes:` in context
- `docs/specs/02-configuration.md` — document the field
- `docs/specs/03-commands.md` — document `heraut commit create`

---

## Error handling & edge cases

| Situation | Behavior | Exit |
|---|---|---|
| Not a TTY (stdin or stdout) | `commit create requires an interactive terminal` | Usage |
| Nothing staged → "Stage all" | `git add -A`, continue | — |
| Nothing staged → "Cancel" | abort, nothing-to-do message | OK (0) |
| `--all/-a` passed | skip pre-check ① and staging prompt; `git commit -a` (git's own error if truly nothing) | — |
| User declines at preview-confirm | abort; print the assembled message so work isn't lost | OK (0) |
| `VerifyCommit` guard fails (defense-in-depth) | abort **before** committing; print message + reason | Usage |
| `git commit` fails (hooks reject, etc.) | wrap + surface git stderr | maps from git exit |
| `--dry-run` | print message; no staging, no commit | OK (0) |
| Config present but invalid | same prelude as `verify`/`check`: print config errors | Config |

- **Hooks fire normally.** Running real `git commit` means this repo's `commit-msg` (cog) and
  `prepare-commit-msg` (typos) hooks validate the output — which is why the `VerifyCommit`
  guard runs first (fail in heraut with a clear message rather than at the hook).

---

## Testing strategy

TDD (red → green), per `testing.md`'s four layers.

**Unit (table-driven):**
- `conventionalcommit.Format` — header with/without scope, `!`, body, footer block,
  BREAKING CHANGE footer; **round-trip**: `Parse(c.Format())` reproduces `c` (driven off the
  existing Parse corpus).
- `commitwizard.Assemble` — breaking sets both `!` and the `BREAKING CHANGE:` footer; empty
  scope/body/footers omitted; footer ordering.
- `app.AllowedCommitTypes` — config override vs. default; existing `VerifyCommit` tests stay green.

**Contract (MockRunner):**
- `hasStaged` → `git diff --cached --quiet` exact args; exit-1 (staged) vs exit-0 mapping.
- `stageAll` → `git add -A`.
- `commit` → `git commit -F <tmpfile>` (flag shape; tmpfile path dynamic); `--all` variant adds `-a`.
- Guard path: a message that would fail `VerifyCommit` records **no** `git commit` call.

**Excluded from coverage:** `collectAnswers` (the huh form), added to the same exclusion list
as `internal/scaffold/wizard.go`, same rationale (untestable VT100 forms).

**Fixtures:** add a `testdata/config/` sample exercising `commit_lint.scopes` so the loader
round-trips it. No new schema enum fixture needed (`scopes` is not a strategy/platform/generator enum).

---

## ADR & roadmap

- **ADR-0031** — "Interactive commit wizard (`heraut commit create`)": records the command,
  the `git commit -F` mechanism, reuse of `AllowedCommitTypes`/`VerifyCommit` (A2a edge),
  `commit_lint.scopes` as wizard-only (and explicit non-enforcement in `verify`), staging UX,
  tickets deferred. Supersedes the future-work notes in ADR-0027/0030.
- **Roadmap T120** — added `[ ]` with this design; flipped to `[x]` with a completion note on
  implementation (two-step flow).

## v2 backlog (noted, not built)

- Ticket-pattern integration in the footer step (validate/propose against `tickets[].pattern`).
- Per-file staging picker (`git status --porcelain` → multi-select → `git add <files>`).
- `--amend`.
- Structured footer add-loop (token select + value) instead of the free multi-line block.
