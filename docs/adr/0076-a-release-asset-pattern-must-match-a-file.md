# ADR-0076: A release asset pattern must match a file

- **Status**: Accepted
- **Date**: 2026-10-20
- **Deciders**: bchatard

---

## Context

`release.assets` and `release.targets[].assets` are glob patterns. A pattern that matched nothing only
warned and was skipped (Spec 02 § `release.assets`, Spec 05 § Asset resolution), so a typo in a pattern
published a release without its files and nobody noticed. The rule review (RULE-039) ruled that wrong.

The leniency had a reason: assets are resolved while the release is created, after the tag has been
created and pushed, so a strict failure there leaves a pushed tag with no release (the failure-recovery
guide's state C). Failing at that point only moves the damage.

There is an earlier point. The release prelude already runs an asset gate (`CheckAssets`, T381) after the
`post_bump` hooks and before the changelog commit, the tag and every push. It enforces that assets stay
inside the repository, and it skips patterns that match nothing because hooks that run later may still
build the files.

## Decision

1. **A pattern that matches no file is an error at that gate.** It runs after the `post_bump` hooks and
   before anything is committed, tagged or pushed, so a refusal leaves the repository untouched. The
   message names the pattern and the three ways out: build the files in a `post_bump` hook (or earlier),
   fix the pattern, or set `release.allow_missing_assets: true`.
2. **`release.allow_missing_assets: true` restores today's behavior**: a pattern that matches nothing
   warns at publish time and is skipped. It is one boolean on the root `release` block, default `false`,
   and applies to `release.assets` and to every target's `assets`. It is not overridable per target or per
   environment, and the `assets:` lists keep their shape.
3. **Assets built by a later hook need the flag.** `pre_changelog`, `pre_tag`, `post_tag` and
   `pre_release` all run after the gate, so a file they build does not exist yet when it is checked. A
   project that builds there either moves the build to `post_bump` or sets `allow_missing_assets: true`.
4. **A dry run only warns.** Hooks are rendered, never run, in a dry run, so the files a hook would build
   cannot exist. A pattern that matches nothing prints a warning there instead of failing, and the real
   run applies the rule. (`release --dry-run --json` does not run the gate today and still does not.)
5. **Resolution at publish time is unchanged.** It still warns and skips a pattern that matches nothing,
   because failing there is the partial-state failure above. With decision 1 in force it only matters when
   `allow_missing_assets` is set or when a later hook deleted a file.

## Consequences

- A configuration with a pattern that matches nothing at `post_bump` time now fails `heraut release`
  with a Config error, before anything is written. Typos stop shipping empty releases.
- Projects that build their artifacts after `post_bump` must set `allow_missing_assets: true` (losing the
  typo check for that release block) or move the build step earlier.
- The new key is an addition to `internal/config`, so `schema.json`, `docs/heraut.sample.yml` and the
  `testdata/config/valid` fixtures change with it. No existing key is renamed.
- Specs 02 and 05 lose their "resolved leniently" statement for the default case, and the
  failure-recovery guide gains a sentence for state C.

## Alternatives considered

- **Fail at publish time.** Rejected: the tag is already pushed, so a typo would leave a tag with no
  release. It is the failure the original leniency avoided.
- **Mark each pattern optional** (`assets:` entries become strings or `{glob, optional: true}`).
  Rejected: a scalar-or-mapping shape that ADR-0061 argued against, and every consumer of `Assets` would
  change. The boolean is coarser but costs nothing in shape.
- **A separate `assets_optional:` list.** Rejected for now: two keys to learn, and a target-level
  override would need both. It can be added later without undoing this decision.
- **Re-check at publish time for patterns marked late-built.** Rejected as more machinery than the
  problem needs; the boolean already says "I know these may be missing".
