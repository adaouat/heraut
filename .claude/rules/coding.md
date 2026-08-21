# Coding rules

## Architecture: hexagonal (ports & adapters)

```
cmd/heraut/main.go            entry point — fang.Execute(cmd.NewRootCmd())
   │
   ▼
internal/cmd/                 cobra commands — parse flags, call app.*, render UI
   │
   ▼
internal/app/                 wiring: NewResolver(), BuildPipeline(), BuildChangelogPipeline()
   │
   ▼
internal/pipeline/            release + changelog flows (no factories)
   │
   ▼  ──→  internal/versioning/  (tagfmt, semver, calver, perenv)
   │
   ├──→  internal/generators/   (gitcliff, communique)  ── implement port.Generator
   │
   ├──→  internal/platforms/    (github, gitlab)                   ── implement port.Platform
   │
   └──→  internal/adapter/exec/ ── implements port.Runner
```

- `cmd/heraut/main.go` is **trivial**: build flags (`Version`, `ProjectURL`, `LatestURL`)
  plus a `fang.Execute(ctx, cmd.NewRootCmd(), …)` call. No flag parsing, no command logic.
- `internal/cmd/` (package `cmd`) holds every cobra command. This keeps commands testable
  as a regular Go package and leaves `cmd/heraut/` reserved for the entry point.
- `internal/port/` defines interfaces (`Runner`, `Generator`, `Platform`). They are stable
  contracts — change them deliberately and update every implementor in one commit.
- `internal/adapter/` and the generator/platform packages provide concrete implementations.
- `internal/app/` is the **only** place that constructs concrete implementations from
  config. `internal/cmd/` never calls `gitcliff.New(...)`, `github.New(...)`, etc. directly.

## Layer rules

| Layer                | Allowed to import                                                                  |
|----------------------|------------------------------------------------------------------------------------|
| `cmd/heraut/`        | `internal/cmd/` only                                                               |
| `internal/cmd/`      | `internal/{app,ui,config,scaffold,commitwizard}/`                                  |
| `internal/scaffold/` | `internal/{config,ui,versioning,forge}/`                                           |
| `internal/app/`      | `internal/{port,config,pipeline,versioning,generators,platforms,adapter,ui,conventionalcommit,forge}/` |
| `internal/pipeline/` | `internal/{port,config,versioning,ui}/`                                            |
| `internal/generators/*` | `internal/{port,config,conventionalcommit}/` (conventionalcommit is a pure leaf — the native generator parses commits with it) |
| `internal/platforms/*`  | `internal/{port,config}/`                                                          |
| `internal/versioning/*` | `internal/{port,config,versioning,conventionalcommit}/`                         |
| `internal/config/`   | nothing from heraut (it is at the bottom)                                          |
| `internal/port/`     | nothing from heraut (it is the contract)                                           |
| `internal/conventionalcommit/` | nothing from heraut (pure, like port/config)                             |

If you find yourself importing `up` the stack, the design is wrong — fix the dependency
direction, do not add the import.

## Error handling

- **Always wrap.** Every `if err != nil` returns `fmt.Errorf("doing X: %w", err)`. The
  `%w` is mandatory; without it, callers cannot `errors.Is` / `errors.As` it.
- **Never string-match errors.** Use `errors.Is(err, target)` for sentinel errors and
  `errors.As(err, &typed)` for typed ones.
- **Never `os.Exit` below `cmd/`.** Return the error and let `cmd/heraut/` decide the exit
  code. The only `os.Exit` call lives in `cmd/heraut/main.go` (or is delegated to
  `fang.Execute`).
- **Sentinel errors at package boundaries.** Per-env exposes `ErrTargetExists` (E001),
  `ErrDestinationAhead` (E002), `ErrNoSourceTags` (E003). Pipeline checks for them with
  `errors.Is` and decides whether `--force` applies.

## Config

- `config.Load(path string) (*Config, error)` is the only entry point for loading
  `.heraut.yml`. Commands never read YAML directly.
- Validation happens in two phases, both inside `internal/config/`:
  1. **Strict parsing** (`loader.go`): unknown keys produce errors with line numbers.
  2. **Semantic validation** (`validator.go`): required fields, enum checks,
     strategy-specific rules, cycle detection. Returns `ValidationErrors` with `Path`,
     `Message`, `Hint`.
- Struct field names in `internal/config/config.go` are **wire-compatible** with existing
  `.heraut.yml` files. Renaming a field is a breaking change requiring an ADR.
- When adding, renaming, or removing a field in `internal/config/config.go`, **also update**:
  - `schema.json` — type, description, enum values, required list
  - `docs/heraut.sample.yml` — show the field in context with a comment
  Failing to keep these in sync silently misleads users who rely on schema autocomplete
  or the sample as their configuration reference.

## Embedded assets

- Default git-cliff TOMLs are embedded via a `//go:embed` directive in
  `internal/generators/gitcliff/`.
- Treat embedded TOML / Tera content as user-facing — changing the bytes changes the
  effective config for every user who relies on the defaults. See
  [ADR-0010](../../docs/adr/0010-embedded-cliff-toml-default.md).
- Effective config (embedded + override merged) is exposed via `EffectiveChangelogConfig()`
  and `EffectiveReleaseNotesConfig()` for `heraut cliff`.

## CLI commands

- `RunE` (never `Run`) so errors propagate to `fang.Execute`.
- Flags declared in the command's constructor function; never package-level globals.
- Command bodies are short: read flags → load config (`config.Load`) → call
  `app.NewResolver(...)` and `app.BuildPipeline(...)` → call `pipeline.Run()` → done.
- No strategy switching, no generator construction, no platform construction in
  `internal/cmd/`.
- Global flags on root: `--config`, `--dry-run`, `--verbose`, `--env`, `--force`.

## UI

- `internal/ui/` owns lipgloss styling, status lines (`Success`, `Warn`, `Err`, `Info`),
  the version banner, and the `Step` spinner.
- Detect TTY at the boundary (`isTerminal(w)`) — fall back to plain lines in CI, dry-run
  mode, and on non-terminal writers.
- `--dry-run` output uses plain steps so the `[dry-run]` lines are never overwritten by a
  spinner.

## Comments

- Default: no comments. Well-named identifiers explain the *what*; the code shows the
  *how*.
- Add a comment only when the *why* is non-obvious — a hidden constraint, a subtle
  invariant, a workaround. Reference the specific bug/ADR if applicable.
- Do not reference current tasks, callers, or fix histories in comments. Those belong in
  the commit message.

## Don't expand scope

- The current task defines the work. Do not refactor surrounding code unless the task
  explicitly asks for it.
- Do not add fields, flags, or interfaces for hypothetical future needs. YAGNI applies.
- If you discover something that needs fixing outside the task, add it to
  `docs/tasks/roadmap.md` as a new task — do not silently implement it.
