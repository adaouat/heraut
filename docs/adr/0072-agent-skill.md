# ADR-0072: A bundled agent skill — `heraut skill`

- **Status**: Accepted
- **Date**: 2026-10-13
- **Deciders**: bchatard

---

## Context

An AI coding agent that finds `heraut` on `PATH` can read `--help`, but it cannot tell which commands
publish irreversibly, in what order to run the safe ones, what the exit codes mean, or that
`--json` exists (ADR-0071). Without that it either guesses or asks the user to explain the tool each
session.

## Decision

1. **Ship the guide as a skill file, in the binary.** `internal/skill/SKILL.md` is embedded with
   `//go:embed`, so the text always matches the CLI version that carries it. It has the frontmatter
   (`name`, `description`) agent skill loaders expect, then the rules an agent must follow (no
   `release` or `changelog --commit/--tag` without the user's approval), the safe command order, a
   command table, the JSON envelope, exit codes and how to read a release plan.
2. **`heraut skill install [--dir] [--force]`** writes it to `<dir>/heraut/SKILL.md`, default
   `.claude/skills`; **`heraut skill print`** writes it to stdout for agents with another layout. An
   installed file that differs is replaced only with `--force`, so a user's edits survive an upgrade
   until they choose otherwise.
3. **A test keeps it honest:** every runnable command of the CLI must appear in the skill by its full
   path, so a new command cannot ship undocumented to agents.
4. **No MCP server for now.** The skill plus `--json` covers agents with a shell; a server can be built
   on the same JSON objects if a client without one needs it.

## Consequences

- `internal/skill` is a pure leaf package (no heraut imports); `internal/cmd` may import it.
- The skill text is public surface: its rules about what an agent may run unprompted are a product
  decision, changed through review like any other doc.
- Installing is opt-in. Nothing writes into a project's `.claude/` unless the user (or the agent they
  authorised) runs `heraut skill install`.
- Whether the packslip install channel can carry the file next to the binary is unverified; the
  embedded copy does not depend on it.

## Alternatives considered

- **A `llms.txt` or docs page only.** Not versioned with the binary and not discovered by skill loaders.
- **Generate the skill from the cobra tree.** Keeps the command list current but cannot carry the safety
  rules or the order; the drift test gives the same guarantee.
- **Install automatically from `heraut init`.** Writes outside what the user asked for.
