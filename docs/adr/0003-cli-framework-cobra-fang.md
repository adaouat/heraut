# ADR-0003: CLI Framework — cobra + fang

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

[ADR-0001](0001-language-go.md) selected Go as the implementation language. heraut needs
a CLI framework that handles:

- Subcommand routing (`heraut release`, `heraut version next`, `heraut check cliff`, …)
- Persistent and per-command flags
- `--help` rendering
- `--version` rendering with our custom banner + bundled-tool versions
- Errors styled consistently
- Shell completions (bash, zsh, fish)

Three options were considered.

## Options

### `cobra` alone

The de-facto standard CLI library in the Go ecosystem (used by `gh`, `glab`, `kubectl`,
`docker`, `helm`, …). Mature, well-documented, exhaustive feature set. Default help and
error output is functional but plain — no colors, no Unicode embellishments.

### `cobra` + `fang`

[Fang](https://github.com/charmbracelet/fang) is a Charmbracelet wrapper around cobra
that adds:

- Styled help output (colors, headers, layout via `lipgloss`)
- Styled error output (clear error prefix, hint formatting)
- `--version` flag with optional custom version rendering
- Manpage generation
- Shell completion subcommand wiring

Fang is purely additive — every cobra concept (commands, flags, `RunE`, hooks) works
unchanged. The change to existing cobra code is a single line: `fang.Execute(ctx, root,
opts...)` instead of `root.ExecuteContext(ctx)`.

### A non-cobra alternative (`urfave/cli`, custom dispatcher)

Rejected without deep evaluation. cobra is the ecosystem standard; the libraries we
orchestrate (`gh`, `glab`) use cobra, so contributors moving between codebases see the
same patterns. Switching to a different framework would optimise for nothing and lose
ecosystem knowledge.

## Decision

Use **cobra + fang**.

- `cobra` for the core CLI structure: commands, flags, `RunE` handlers, `PersistentPreRunE`
  hooks.
- `fang` for execution and rendering: `fang.Execute(ctx, root, fang.WithVersion(Version),
  fang.WithoutVersion(), fang.WithColorSchemeFunc(...))`.

Within heraut:

- `cmd/heraut/main.go` calls `fang.Execute`.
- `cmd/heraut/root.go` constructs the root `*cobra.Command` and registers all
  subcommands.
- Each subcommand lives in its own file (`cmd/heraut/release.go`, `cmd/heraut/version.go`,
  etc.) and exposes a constructor function returning `*cobra.Command`.
- Commands use `RunE` (never `Run`) so errors bubble up to fang's styled error handler.

## Consequences

**Positive**
- Help output is visually pleasant by default, with no per-command styling work.
- Error output is consistently formatted; the "what + hint" pattern in
  `.claude/rules/coding.md` renders well under fang's styling.
- Adopting fang means heraut is visually consistent with other Charmbracelet-using CLIs
  in the same author's ecosystem (e.g. bifrost — the deployment CLI that the project
  structure mirrors).
- Shell completions and manpages are one-line additions.

**Negative / trade-offs**
- Fang is a Charmbracelet project; its API has historically moved between minor releases.
  Pin to `charm.land/fang/v2` and update deliberately.
- Fang adds ~2 MB to the binary (lipgloss, charmtone, ansi terminal handling). Acceptable
  for a binary that already pulls in `huh` for the interactive wizard.
- Fang owns the `--version` flag rendering. We use `fang.WithVersion(Version)` to set the
  version string and `fang.WithoutVersion()` plus a custom `-V` flag to display the
  heraut banner with bundled-tool versions (see `internal/ui/version.go`).
