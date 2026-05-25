# ADR-0015: Unified Logging with `charm.land/log`

- **Status**: Rejected
- **Date**: 2026-05-25
- **Deciders**: bchatard

> **REJECTED.** See the Decision section below for the rationale.

---

## Context

heraut currently produces terminal output through several uncoordinated channels:

- **`charm.land/fang/v2`** styles errors, help, and the `--version` banner.
- **`internal/adapter/exec/runner.go`** writes plain `[dry-run] …` and `[exec] …` lines
  (plus, since T29, the indented echo of a command's captured output) to an `io.Writer`
  (`r.Out`, defaulting to `os.Stderr`).
- **Commands** (`internal/cmd/*`) write results to `cmd.OutOrStdout()` and ad-hoc status
  text to stdout/stderr via `fmt.Fprint*`.
- **`internal/selfupdate`** prints the background update hint to stderr.
- Misc `fmt.Fprint*` sites elsewhere (≈61 call sites across `internal/` + `cmd/`).

There is no notion of log *levels* (a single `--verbose` bool gates the `[exec]` trace),
no consistent styling for status lines, and no single place that decides where
diagnostics go. The question: should heraut adopt `charm.land/log/v2`
(the charmbracelet/log library) as one leveled, styled logger?

This extends the CLI-stack decision in [ADR-0003](0003-cli-framework-cobra-fang.md).

## What would change (if adopted)

- A new owner for diagnostics — e.g. `internal/ui` (or a new `internal/log`) exposes a
  configured `*log.Logger` writing to **stderr**, with level set from `--verbose`
  (`Debug`) vs default (`Info`/`Warn`).
- The runner's `[exec]`/`[dry-run]` lines and output echo become `logger.Debug(...)`
  calls; the selfupdate hint and command status lines route through the logger.
- TTY/CI detection and color on/off centralised in one place instead of per-writer.
- The logger would be injected (the exec adapter takes a logger instead of, or alongside,
  its `Out io.Writer`).

## Options considered

1. **Adopt `charm.land/log/v2`** (this ADR's subject) — leveled, styled, composes with
   lipgloss; same vendor as the rest of the stack.
2. **Status quo** — keep `fmt.Fprintf` to an `io.Writer` + the `--verbose` bool. Zero new
   surface.
3. **stdlib `log/slog`** — structured leveled logging with no new third-party dep, but no
   styling and a key-value format that suits services more than a human CLI.
4. **Expand `internal/ui` only** — grow the existing lipgloss helpers (Success/Warn/Err/
   Info) into the single status-line path without a logging library or levels.

## Impacts

**Stdout purity (the hard constraint).** `heraut version next` / `version current` print
*only* the resolved tag to stdout so CI can capture it (`TAG=$(heraut version next)`). Any
logger MUST write to stderr and never leak to stdout. `charm.land/log` defaults to
`os.Stderr`, which fits — but the boundary has to be enforced and tested.

**Dry-run / CI plain output.** [Spec 06](../specs/06-dx-and-testing.md) and the dry-run
contract require plain, self-non-overwriting lines in CI and dry-run. A styled logger must
disable ANSI when not a TTY (charm/log honours `NO_COLOR` and a `NoColor` setting) and the
`[dry-run]` lines must stay parseable.

**Overlap with fang.** fang already styles errors/help/version. Introducing charm/log adds
a second styling system; they share lipgloss so the look is consistent, but ownership of
"who styles what" must be drawn clearly (fang = framework chrome; logger = diagnostics).

**Layering.** Today `internal/adapter/exec` depends only on `io.Writer` — dumb and
testable. Injecting a logger couples the adapter to a logging type. Options: keep the
`io.Writer` seam and attach the logger at the cmd boundary, or define a tiny logging port.
Pulling a framework into the bottom adapter layer is a smell to weigh against the
consistency gain.

**Testing & determinism.** [testing.md](../../.claude/rules/testing.md) forbids
time-of-day dependencies; charm/log timestamps must be disabled (or fixed) in tests.
Current runner tests capture a `bytes.Buffer` and assert plain substrings — level prefixes
and ANSI would force `NoColor`/no-timestamp config or substring-tolerant assertions.
`testutil.MockRunner` is unaffected (separate implementation).

**Dependency weight.** Marginal: `charm.land/lipgloss/v2` is already an (indirect)
dependency via fang/huh, and charm/log builds on it. No new heavy transitive tree.

**Blast radius.** ≈61 `fmt.Fprint*` sites. A migration need not be all-at-once, but a
half-migrated state (some logger, some `fmt`) is worse than either endpoint, so scope must
be decided up front.

## Pros / Cons

**Pros**
- One styling/level story; consistent look with the charm stack already in use.
- Real log levels make `--verbose` (and a future `--debug`/quiet) trivial and uniform.
- Centralised TTY/`NO_COLOR`/output-channel decisions instead of per-call-site choices.
- Low marginal dependency cost (lipgloss already present).

**Cons**
- Couples low-level adapters to a logging framework unless an extra seam is added.
- Two styling systems (fang + logger) to keep visually coherent.
- Risk to stdout purity and to the plain dry-run/CI contract if the boundary slips.
- Test churn: determinism (timestamps) and ANSI handling must be configured.
- Migration touches ~61 sites; partial adoption is a trap.
- Marginal real benefit over the current simple `io.Writer` approach for a tool whose
  diagnostic output is intentionally minimal.

## Open questions

- Scope: only the `--verbose` trace, or all status/diagnostic output?
- Logger ownership: `internal/ui` vs a dedicated `internal/log`; injected vs package-level.
- Does the exec adapter take a logger, or keep its `io.Writer` and log at the cmd boundary?
- Quiet/`--debug` levels — wanted now, or YAGNI?

## Consequences (if it were adopted)

- A single configured logger on stderr; `--verbose` maps to `Debug`. Adapters/pipeline log
  through it; stdout stays reserved for machine-readable command results.
- New tests assert log behaviour with timestamps/color disabled for determinism.
- ADR-0003's stack section would be updated to mention the logger alongside fang.

## Decision

**Rejected.** The `io.Writer` + `--verbose` approach, improved in T29 to echo captured
output and include stderr in failures, already covers the immediate diagnostic need.
Adopting charm/log before v1.0 introduces more cost than benefit:

- The ~61 `fmt.Fprint*` migration must be all-or-nothing (a half-migrated state is worse
  than either endpoint), which is unjustified scope pre-v1.0.
- Low-level adapters (`adapter/exec`) would need to be coupled to a logging framework
  unless an extra seam is added — a real layering cost for minimal gain.
- fang already handles styled error output; adding charm/log creates a second styling
  system to keep coherent.
- The stdout-purity constraint (machine-readable `version next` / `version current`
  output) requires enforced stderr-only logging; the current `io.Writer` seam already
  encodes this boundary.

If consistent TTY/color detection or log levels become a real need after v1.0, the
preferred path is **Option 4 from the Considered section** — grow `internal/ui` with
typed status-line helpers (`Success`, `Warn`, `Err`, `Info`) that commands call directly,
without a third-party logging framework. This is tracked in the roadmap.
