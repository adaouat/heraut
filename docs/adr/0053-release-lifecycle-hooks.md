# ADR-0053: Release lifecycle hooks — arbitrary shell execution via config

- **Status**: Accepted
- **Date**: 2026-09-11
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-09-11-release-hooks-design.md`](../superpowers/specs/2026-09-11-release-hooks-design.md)

---

## Context

heraut had no way to run user-defined commands at points in the release lifecycle. Common needs
this blocked: bumping a version string embedded in another file (`package.json`, `Cargo.toml`)
before the changelog commit; running a build or test gate before tagging; triggering `npm
publish` or a deploy after the tag exists; notifying Slack after a GitHub/GitLab release goes
live. cocogitto's `pre_bump_hooks`/`post_bump_hooks` were the closest prior art, but its
two-hook model didn't map cleanly onto heraut's own pipeline: cocogitto has no publish step, no
per-target loop, and no `disable_changelog` — see the design doc above for the full comparison.

Every existing `port.Runner` call in the codebase is a structured invocation of a known binary
(`git`, `gh`, `glab`) with discrete, heraut-controlled args. Hooks are the first capability where
a config file causes arbitrary code execution, and the first place heraut needs a free-form user
string executed via `sh -c`.

## Decision

`hooks:` config entries are shell command strings, executed via `sh -c "<rendered command>"`
through `port.Runner` — in the same `Interactive` mode already built for GPG pinentry (T260):
stdin/stdout/stderr connect directly to the real terminal, streamed live, never captured. Six
hook points map onto the pipeline's actual steps (`post_bump`, `pre_changelog`, `pre_tag`,
`post_tag`, `pre_release`, `post_release` — see [Spec 02 § `hooks`](../specs/02-configuration.md)
for the full point-by-point reference and template variables).

**Trust model, stated explicitly**: anyone who can edit `.heraut.yml` can run anything the
invoking user or CI credentials can run — identical to a CI YAML `run:` step. This is not a new
risk heraut introduces so much as a capability heraut now has for the first time, and it should
be documented as a deliberate boundary crossing rather than something that crept in.

**Failure semantics** are, by design, almost entirely inherited rather than invented: a failing
hook at `post_bump`/`pre_changelog`/`pre_tag`/`post_tag` aborts the run exactly like any other
step failure already does — no new rollback machinery, since none exists anywhere else in the
pipeline either (a `git tag` failure after a successful changelog commit already leaves that
commit in place today). The one deliberate exception is `pre_release`/`post_release`: a hook
failure there is isolated to that publish target only — the loop still attempts the remaining
`release.targets` — whereas an actual publish failure (not a hook) still aborts the whole loop
unchanged. This asymmetry exists because, by the time `post_release` runs, the platform release
already exists; treating a notification hook's failure as equivalent to a publish failure would
make an already-successful release look like a total failure.

## Consequences

- **Additive, zero-risk for every existing config.** `hooks:` is a new, entirely optional
  top-level key; omitting it (or any individual point) changes nothing about existing behavior.
- **POSIX-only in v1.** `sh -c` has no equivalent on Windows despite heraut shipping Windows
  binaries (ADR-0013) — a documented gap, not an oversight.
- **A future contributor "fixing" the per-platform isolation into consistency with the rest of
  the pipeline's all-or-nothing loop behavior would be undoing an intentional decision**, not
  correcting a bug — this ADR is the record of why it's shaped that way.
- **`--no-hooks`** (local to `release` and `changelog`, not a root flag — per the existing
  per-command flag convention) skips every configured hook for one run without touching config.
- **`--dry-run` never executes a hook.** It renders the same command a real run would execute
  (real values substituted, not the raw template) and reports it under the identical step name a
  real run would use, so the two paths' step sequences and counts always agree.

## Alternatives considered

- **Adopting cocogitto's flat two-hook model (`pre_bump`/`post_bump`) as-is.** Rejected: heraut's
  pipeline has real structure cocogitto's doesn't (a `disable_changelog` escape hatch, a separate
  local-tag-then-push step, a multi-target publish loop), and mapping user intent onto only two
  points would have forced awkward workarounds — e.g. no way to say "run before tagging but after
  the changelog commit" without a single `pre_bump` also standing in for that.
- **cocogitto-style rollback (stash uncommitted changes on hook failure).** Rejected — heraut's
  pipeline has no rollback for any other step failure, and adding one uniquely for hooks would be
  new, unproven machinery solving a problem the rest of the codebase already accepts living
  without.
- **A second templating syntax (Tera-style `{{version}}`, matching cocogitto) instead of Go's own
  `text/template`.** Rejected in favor of reusing the exact engine already used for changelog and
  release-notes rendering — one syntax to document, no new dependency, and real template
  conditionals (`{{ if eq .Platform ... }}`) for free instead of a second, narrower substitution
  mechanism.
- **A uniform "hook failure always aborts the whole release," even for `pre_release`/
  `post_release`.** Rejected — by the time a `post_release` hook runs, the platform release
  already exists; aborting the remaining targets over an unrelated notification failure would
  turn a partially-successful multi-target release into what looks like a total failure in the
  exit code, when most of it actually succeeded.
