# ADR-0031: `heraut commit create` — Interactive Commit Wizard

- **Status**: Accepted
- **Date**: 2026-06-26
- **Deciders**: bchatard

---

## Context

[ADR-0027](0027-builtin-conventional-commit-checker.md) (T116) shipped `heraut commit
verify` and explicitly listed an interactive commit wizard among its "Related future work
(not yet scoped)":

> **Interactive commit wizard** (e.g. `heraut commit create`) — guided prompts (type,
> scope, breaking, description, body) that construct a conventional commit message and run
> `git commit` with it … would naturally reuse this ADR's `conventionalcommit` package and
> `commit_lint` config (types) so wizard-built commits are guaranteed to pass
> `heraut commit verify`. Not designed here; revisit as its own task when prioritized.

[ADR-0030](0030-commit-check-rev-range-validation.md) (T119) shipped `heraut commit
check` and left the wizard note untouched ("unscoped and unaffected by this ADR").

This ADR closes both deferred notes and records the design chosen for T120.

## Decision

### Command surface

`heraut commit create` — a new cobra subcommand registered under `heraut commit`. It
accepts the global flags (`--config`, `--dry-run`, `--verbose`, `--env`) and one local
flag: `--all` / `-a` (stage all tracked changes before committing, equivalent to
`git commit -a`). The command is **TTY-only**: if stdout is not a terminal, it errors with
`exitcode.Usage` and an actionable message before launching the form.

### UX flow

Seven interactive steps, rendered by `huh` (heraut's existing interactive-form library,
already used by `heraut init`):

1. **Type** — single-select from `commit_lint.types` (honoured via `app.AllowedCommitTypes`)
2. **Scope** — optional free-text; if `commit_lint.scopes` lists at least one entry, a
   select is offered instead (see Config section below)
3. **Subject** — short description (single line)
4. **Breaking change** — boolean; if yes, a free-text `BREAKING CHANGE:` footer is added
5. **Body** — optional multi-line description
6. **Footers** — optional `Token: value` or `Token #value` lines, free-text (one block,
   newline-separated; no structured add-loop in this version)
7. **Preview + confirm** — assembled message printed, user confirms or aborts

### Assembly and guard

`commitwizard.Assemble(a)` maps the wizard's `Answers` struct to a
`*conventionalcommit.Commit`. Calling `.Format()` on that result serializes it to the
Conventional Commits wire format — `Format` being a method on `*Commit` and the inverse of
the existing `conventionalcommit.Parse`. Round-trip fidelity is verified in unit tests.

A companion `conventionalcommit.ParseFooterLine(line) (Footer, bool)` handles individual
footer lines (e.g. from the footers step). It preserves the `#` in `Footer.Value` so the
issue reference survives `Format` (which normalizes the separator to `: `).

Before writing the commit, `finalize` calls `app.VerifyCommit(cfg, assembled)` as a guard.
Because `app.AllowedCommitTypes(cfg)` was extracted from `VerifyCommit` as part of this
work (single source of truth for the type allow-list), the wizard's type step and
`VerifyCommit`'s type check are guaranteed to agree. A guard failure is a programming
error, not a user error — `finalize` returns it as an internal error.

### Git mechanism — temp file instead of stdin

`port.Runner` (heraut's exec abstraction) has no stdin-passing capability. Piping to
`git commit --file=-` is therefore not available without changing the `Runner` interface, a
change that would ripple through every call site and every test double.

Instead, `finalize` writes the assembled message to a `os.CreateTemp` file and calls
`git commit -F <tmpfile>`. The temp file is removed on return (deferred). This pattern
avoids any interface change and is fully testable via `MockRunner`.

### Staging helpers

`internal/commitwizard/git.go` implements three unexported helpers that the wizard calls
via the `port.Runner`:

- `hasStaged(r)` — `git diff --cached --name-only` to detect whether anything is staged
- `hasWorkingTreeChanges(r)` — `git status --porcelain` to detect whether there is anything
  to commit at all (staged, unstaged, or untracked); `hasStaged` only sees the index
- `stageAll(r)` — `git add -A` (run when the user confirms the stage prompt)
- `commit(r, message, all)` — writes the temp file and runs `git commit -F <tmpfile>`

When `--all`/`--dry-run` are unset and nothing is staged, `resolveStaging` inspects the
working tree: a clean tree aborts early with "nothing to commit — working tree clean" (no
prompt, no wasted form); otherwise the wizard offers to stage everything (`git add -A`) or
cancel — a lightweight prompt, not a per-file picker (see Deferred section). `resolveStaging`
takes the confirm function as a parameter, so the whole decision is unit-tested without a TTY.

### Package layout

```
internal/commitwizard/
    commitwizard.go   Answers, Options, Assemble, parseFooterLines, finalize, Run, typeOptionLabel
    git.go            hasStaged, stageAll, commit (git interactions via port.Runner)
    form.go           the huh form definition (no unit tests — same precedent as internal/scaffold/wizard.go)
```

`internal/cmd/commit.go` gains `newCommitCreateCmd()`, registered alongside `verify` and
`check`. The runner passed to `commitwizard.Run` is always the real runner (never
dry-run-wrapped); dry-run is gated inside `finalize` so the form renders in both modes.

`internal/app/commit.go` gains `AllowedCommitTypes(cfg *config.Config) []string`, extracted
from `VerifyCommit` (which now delegates to it) — the single place that reads
`commit_lint.types` and applies the fallback default list.

### Config — `commit_lint.scopes`

A new optional `commit_lint.scopes` list field is added to `internal/config/config.go`,
`schema.json`, `docs/heraut.sample.yml`, and `docs/specs/02-configuration.md`. This field
is **wizard-only**: `heraut commit verify` and `heraut commit check` do not read or enforce
it. If the list is non-empty, the wizard's scope step presents a select; otherwise it falls
back to free-text input.

This explicit opt-in scope enforcement model — where the wizard can guide users but
validation commands do not police scope — matches the design intent of `commit_lint.types`
having its own defaults distinct from the validation command's rule set.

### Error handling

All errors from `commitwizard.Run` map to `exitcode.Usage` in `internal/cmd/commit.go`,
consistent with `verify` and `check`. This covers: non-TTY environment, user cancel or
decline at the confirm step, git failures (not staged, `git commit` failure), and the
`VerifyCommit` guard.

`--dry-run` prints the assembled message and exits without calling `git commit`. No commit
is written; the temp-file path is never created.

### Deferred to v2

- **Ticket-pattern integration** ([ADR-0024](0024-ticket-linking.md)) — footer
  autocomplete from configured `commit_lint.ticket` patterns; held until the config
  extension is designed.
- **Per-file staging picker** — interactive file selection before committing; the
  `port.Runner` abstraction and `huh` could support it, but the UX design is non-trivial
  and the lightweight stage-all prompt covers the common case.
- **`--amend`** — amend the last commit via `git commit --amend -F <tmpfile>`;
  straightforward mechanically but raises UX questions (edit previous message vs. replace)
  that need a separate design session.
- **Structured footer add-loop** — multiple rounds of `Token: value` input with dynamic
  add/remove; deferred in favour of the simpler free-text block for the initial version.

## Consequences

- heraut now covers the full commit workflow: lint a message (`commit verify`), validate a
  range (`commit check`), create a message interactively (`commit create`).
- No new exit code is introduced; all error paths map to the existing `exitcode.Usage`,
  consistent with T116 and T119.
- `commit_lint.scopes` is wired only through the wizard; `verify` and `check` behaviour is
  unchanged.
- `internal/commitwizard/form.go` carries no unit tests. The `huh` form is a layout
  concern — the same rationale documented for `internal/scaffold/wizard.go` applies here.
  The observable outputs (assembled message, git calls) are covered by unit and contract
  tests in `commitwizard_test.go` and `git_test.go`.
- `internal/app/commit.go` now exports `AllowedCommitTypes` — a small surface expansion,
  but it eliminates duplicated type-list reads between `VerifyCommit` and the wizard.
- The `(*Commit).Format()` method and `ParseFooterLine` addition complete the package's
  round-trip story: every caller that parses a commit can also serialize one without
  leaving the package.

## Related

- [ADR-0027](0027-builtin-conventional-commit-checker.md) — `heraut commit verify` (T116);
  this ADR closes its "Interactive commit wizard" future-work note.
- [ADR-0030](0030-commit-check-rev-range-validation.md) — `heraut commit check` (T119);
  this ADR closes its remaining deferred wizard note.
- [ADR-0024](0024-ticket-linking.md) — ticket linking via `link_parsers`; integration into
  the wizard footer step is deferred to v2.
