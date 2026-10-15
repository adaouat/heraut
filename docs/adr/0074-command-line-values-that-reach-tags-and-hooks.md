# ADR-0074: Command-line values that reach tags and hooks are restricted

- **Status**: Accepted
- **Date**: 2026-10-15
- **Deciders**: bchatard

---

## Context

Hook commands are shell text with the template variables pasted in unquoted (ADR-0053, Spec 02
§ Quoting and trust). Most of those values are built from the config or from a version heraut parsed.
Two come from the command line and were barely checked:

- `--set-build-id` ends up in `Tag`. The SemVer strategies already require dot-separated
  `[0-9A-Za-z-]` identifiers (ADR-0064), but `tagfmt.ValidateBuildID`, which every other strategy
  relies on, rejected only `/` and whitespace. Under `calver-per-env`, `--set-build-id 'x$(id)'` was a
  valid tag component and reached the hooks.
- `--env` ends up in `Env`. The per-env strategies reject a name that is not in `environments:`, but
  under `semver` and `calver` the flag was accepted and passed through untouched
  (`--env 'x$(id)'` printed `ENV=[x$(id)]` in a `post_bump` hook).

A CI job that feeds a branch name or a PR title to those flags would hand it to the shell. The
hardening review (T388) recorded this as T400.

## Decision

1. **A build ID is `[0-9A-Za-z._-]+`.** `tagfmt.ValidateBuildID` enforces it for every strategy: letters,
   digits, `.`, `_` and `-`. That keeps every realistic CI value (a run number, a pipeline ID, a short
   SHA, `1.2.3-beta`) and drops the characters a shell or git treats specially. The SemVer strategies
   keep their stricter rule on top.
2. **`--env` needs a per-env strategy.** A non-empty `--env` under `semver` or `calver` is a
   configuration error, like `--env auto` already is. Under the per-env strategies the name must be in
   `environments:`, as before. The check lives in `app.ResolveEnv`, the one place every command passes
   `--env` through.

## Consequences

- A script that passed `--env` to a non-per-env repository, or a build ID with other characters, now
  fails with a clear message instead of running. Both were accidents of leniency, not documented uses.
- The values that can still reach a hook unvetted are the ones the config author wrote, which is the
  trust boundary Spec 02 already describes.

## Alternatives considered

- **Quote the variables for the hook author.** Rejected in ADR-0053/0054: the right quoting depends on
  the shell, and a `sh` quoting would be wrong for `cmd.exe`.
- **Restrict only `calver-per-env`.** Rejected: `tagfmt.ValidateBuildID` is shared, and one rule is
  simpler to state than one per strategy.
- **Silently ignore `--env` under a non-per-env strategy.** Rejected: an ignored flag hides a
  misconfigured pipeline; the other flags that cannot apply (`--pre-release` on calver) are errors.
