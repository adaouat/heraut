# ADR-0071: Machine-readable output — `--json` and one envelope

- **Status**: Accepted
- **Date**: 2026-10-13
- **Deciders**: bchatard

---

## Context

An AI agent (or any script) driving the `heraut` binary has to scrape human text: `version next`
prints a bare tag, `check` prints a styled report, a failure is a panel on stderr plus an exit code.
Scraping breaks when wording changes, and the exit code alone cannot say which guard tripped or
what to try next. Phase 69 adds structured output command by command; this ADR fixes the contract
they share so each command does not invent its own.

## Decision

1. **`--json`, declared locally** on each command that supports it, like `--dry-run` and `--env`
   (T266): no root persistent flag, so `--help` of a command that cannot honour it does not list it.
2. **One envelope, one line, on stdout:**

   ```json
   {"ok":true,"data":{…},"warnings":[]}
   {"ok":false,"warnings":[],"error":{"code":"config","message":"…","hint":"…"}}
   ```

   `data` is the command's own object; `warnings` always exists (empty array when none) and carries
   what the text mode prints to stderr; `hint` is omitted when there is none.
3. **`error.code` is a closed vocabulary:** `usage`, `config`, `runtime`, `promotion`, `internal` —
   the exit-code classes of Spec 01 — except that a promotion guard reports its own identifier,
   `E001`, `E002` or `E003` (ADR-0007). Codes are public surface: adding one is allowed, renaming
   or removing one is a breaking change.
4. **The exit code does not change.** `--json` only changes how the result is printed; a script may
   read either. The failing command returns the same classified error, so `forge/cli` still prints
   its usual panel on stderr; stdout carries only the envelope.
5. **Errors raised before the command body run** (an unknown flag, an unknown subcommand) are the
   cobra layer's and are not enveloped: stdout stays empty, the message is on stderr, exit code 1.

## Consequences

- A shared helper in `internal/cmd` (`writeJSON`) renders the envelope and extracts the code and an
  optional hint from the error (an error exposing `Hint() string`), so each command's JSON support
  is the data object plus one call.
- Field names inside `data` are public surface per command and documented in Spec 03 with the command.
- Markup characters (`<`, `>`, `&`) in messages are not escaped.

## Alternatives considered

- **A global `--output json|text`.** One flag for everything, but a command that cannot honour it
  would still accept it, and `notes` already owns `--format` for a different meaning.
- **Newline-delimited events.** Suits streaming, not the one-shot commands in scope.
- **A JSON-RPC or MCP server first.** More code to maintain and no use for agents with a shell;
  it can be built on these same objects later.
