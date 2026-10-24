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

## Addendum (2026-10-20): `--set-version` under the CalVer strategies

The rule review (RULE-032) found the third command-line value that reaches tags and hooks still
unchecked. `--set-version` is validated as SemVer under `semver` and `semver-per-env` (ADR-0064), but
`calver` and `calver-per-env` kept "non-empty, no whitespace" (Spec 03), so `--set-version 'x$(id)'`
became a tag component and a `{{ .Version }}` hook variable there. The override was strategy-agnostic by
design; it stops being so for CalVer.

5. **A CalVer `--set-version` must be in canonical form for the configured `versioning.format`.** After
   the `tag_prefix` (or leading `v`) is stripped, the value must parse under the format and equal its
   own rendering. `2026.05.3` passes for `YYYY.MM.PATCH`; `2026.5.3` parses but is rejected with a hint
   to write `2026.05.3`, and so is `x$(id)`. Impossible dates (month 13) are not checked: the override
   is a deliberate choice and the ordering code already copes with it.
6. **Why canonical and not just parsed.** The tag-collision probe (ADR-0073) compares exact tag
   strings. The parser accepts one or two digits for `MM`, `DD` and `WW`, so `2026.5.3` and `2026.05.3`
   would be two different tags for one release, and the second would slip past the probe.
7. **Consequences.** A script that passed a free-form value to a CalVer repository now fails with a
   Config error naming the format. The SemVer strategies are unchanged, and there is no "full tag"
   mode: the value is a version, and the tag is rendered from it as before.

## Addendum (2026-10-24): a character allowlist on `--set-version`

The security re-scan (NEW-003) noted that `tagfmt.ValidateVersionOverride`, the check every strategy
shares, rejects only an empty value and whitespace. Shell safety of `--set-version` rests on the two
per-strategy checks (SemVer, ADR-0064; CalVer, addendum above). No bypass exists today, but a strategy
added later that skipped both would let `$(…)` reach a tag and the hook variables, which is the exact
class this ADR closes for `--set-build-id` and `--env`.

8. **`--set-version` may contain only `[0-9A-Za-z._+/-]`.** The shared check refuses any other
   character with a Config error that names the value and the allowed set. `/` stays because a value
   may carry a full tag wrapping, and `+` because a SemVer value may carry build metadata. Every value
   the SemVer and CalVer checks accept today is inside this set, so no valid invocation changes.
9. **It is a floor, not a replacement.** The per-strategy checks still run and still decide shape; the
   allowlist only guarantees that whatever a strategy accepts is shell-inert.

The earlier decisions above are unchanged.
