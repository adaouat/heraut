# ADR-0054: Windows hook execution — `cmd /D /C`, no per-OS command syntax

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: bchatard

---

## Context

[ADR-0053](0053-release-lifecycle-hooks.md) shipped `hooks:` as POSIX-only: every command
runs via `sh -c "<rendered command>"`. That ADR called the gap deliberate, not an
oversight, but left it open — heraut ships Windows binaries
([ADR-0013](0013-raw-binary-goreleaser-format.md)) with no way to run a hook on them at
all. This ADR closes that gap.

Two questions have to be settled together, because getting the first one wrong breaks the
one behavior ADR-0053 cannot compromise on — hook failure detection:

1. **Which Windows shell does `runHook` invoke?**
2. **Do hook command strings stay a single, OS-specific shell syntax (the user's problem,
   like a CI YAML `run:` step already is), or does heraut need a second per-OS syntax in
   config?**

### Exit-code propagation is the deciding constraint

`sh -c "<cmd>"` reliably re-exits with the invoked command's own exit status — that's what
makes ADR-0053's failure semantics ("a failing hook aborts the run", isolated per-target
for `pre_release`/`post_release`) work at all. The two obvious Windows shells do not behave
the same way here:

- **`cmd.exe /C "<cmd>"`** re-exits with the invoked command's `ERRORLEVEL` — the direct
  Windows analogue of `sh -c`'s behavior, and reliable without any extra scripting.
- **`powershell -Command "<cmd>"`** does **not** do this by default. PowerShell sets
  `$LASTEXITCODE` from a native command's exit code, but `powershell.exe` itself still
  exits `0` unless the `-Command` script explicitly ends with `exit $LASTEXITCODE` (or the
  command throws a terminating error). This is a widely-documented PowerShell footgun in CI
  contexts, not an edge case — and it would silently defeat hook failure detection on
  Windows unless heraut injected exit-code-propagation boilerplate around every user's
  rendered command string.

That asymmetry decides the shell choice below.

## Decision

**Windows invokes `cmd /D /C "<rendered command>"`.** POSIX keeps `sh -c "<rendered
command>"`, unchanged from ADR-0053.

- `/C` runs the command and terminates, re-exiting with its `ERRORLEVEL` — matching `sh
  -c`'s exit-code semantics with no extra scripting.
- `/D` disables any `AutoRun` command configured in the registry for that shell —
  `cmd.exe`'s equivalent of `sh -c` never sourcing an rc file, and of PowerShell's
  `-NoProfile`: the hook runs in a clean, predictable environment, not whatever a machine's
  `AutoRun` key happens to inject.

**Hook command strings remain a single OS-specific shell string — no per-OS `hooks:` keys.**
A `.heraut.yml` intended to run on Windows must contain `cmd.exe`-shaped syntax in its
hook commands, exactly as ADR-0053 already frames the trust model (identical to a CI YAML
`run:` step, which is POSIX shell on a Linux/macOS runner and `cmd`/`pwsh` on a Windows
runner — never both at once). A user who wants POSIX syntax on a Windows box can already
get it without any heraut-side support: nothing stops a hook command from being
`bash -c "make lint"` if Git for Windows or WSL is present — heraut only ever supplies the
outermost shell.

The GOOS → shell mapping is a small, pure, injectable function (mirrors the CalVer
resolver's `now func()` pattern) so both branches are unit-tested deterministically via
`MockRunner`, regardless of which OS actually runs `go test`.

## Consequences

- **Additive, zero-risk for every existing config.** POSIX behavior is byte-for-byte
  unchanged; this only adds a second branch reached exclusively on `runtime.GOOS ==
  "windows"`.
- **No config-schema change.** `hooks:`'s shape, `schema.json`, and
  `docs/heraut.sample.yml` are untouched — this ADR is pure execution-layer.
- **Real Windows execution stays untested by CI.** `ci.yml` and `release.yml` run
  exclusively on `ubuntu-latest` today; GoReleaser cross-compiles the Windows binary but
  nothing ever executes it. The Windows branch ships covered by mocked unit tests
  (`MockRunner` asserting `cmd`/`/D`/`/C`/the rendered string as args) but not by a real
  `cmd.exe` invocation. Adding a `windows-latest` CI leg is an explicit non-goal of this
  ADR — a separate, later decision given its ongoing cost, not a silent gap.
- **A Windows-targeted `.heraut.yml` is not portable to POSIX unmodified, and vice versa**,
  for any hook using shell-specific syntax (`&&`, redirection, env-var expansion,
  quoting). This is a documented constraint, not a bug — same as a GitHub Actions
  `run:` step is never OS-portable across `shell: bash` and `shell: pwsh` without the
  author making it so.

## Alternatives considered

- **`powershell -Command` (or `pwsh -Command`) as the Windows shell.** Rejected: the
  `$LASTEXITCODE`-doesn't-propagate footgun above would silently break hook failure
  detection — the single behavior this feature cannot get wrong — unless heraut appended
  `; exit $LASTEXITCODE`-style boilerplate to every rendered command, which is exactly the
  kind of hidden, syntax-specific glue this ADR is trying to avoid. `pwsh` also isn't
  guaranteed present on an arbitrary Windows machine the way `powershell.exe` (or `cmd.exe`)
  is, unlike POSIX's `sh` guarantee.
- **Detecting `sh` on `PATH` (via Git for Windows / WSL / MSYS2) and preferring it over
  `cmd.exe` when found.** Rejected for this phase: it would make heraut's Windows behavior
  depend on what else happens to be installed — the exact same `.heraut.yml` would run
  POSIX-syntax hooks successfully on one Windows machine and fail with "command not found"
  on another, which is a worse authoring experience than one deterministic shell a user can
  rely on. A user who wants POSIX syntax can still get it explicitly (`bash -c "..."` as
  the hook command itself) without heraut adding PATH-probing logic.
- **Per-OS `hooks:` config keys** (e.g. `hooks.pre_tag.windows:` / `hooks.pre_tag.unix:`,
  or a `{{ if eq .OS "windows" }}` template variable). Rejected as unnecessary schema
  growth for a need with no evidence yet — the existing per-platform `{{ if eq .Platform
  ... }}` branching (ADR-0053) already proves the templating engine *can* express OS
  branching inside one command string if a user ever needs it, without heraut adding a new
  `.OS` variable or a second config axis. Revisit only if real usage shows single-string
  branching isn't enough.
