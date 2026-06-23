# ADR-0028: Drop the `cocogitto` Generator Entirely

- **Status**: Accepted
- **Date**: 2026-06-22
- **Deciders**: bchatard

---

## Context

heraut ships three changelog/release-notes generators: `git-cliff`, `communique`, and
`cocogitto`. [ADR-0027](0027-builtin-conventional-commit-checker.md) removes heraut's own
*dev-tooling* dependency on `cog` (its commit-msg hook), but `cocogitto` also exists as a
first-class `generator: cocogitto` option in `.heraut.yml` — a separate, user-facing
feature (T15, plus several follow-ons: T71b's per-platform template links, T76/T77's
richer default templates).

Per the deprecation discipline this project follows (don't remove something without a
working replacement covering its critical use cases): `cocogitto`'s only job is changelog
generation from conventional commits, and `git-cliff` already does this — more
completely. Every feature heraut has added to its generator support in the last several
phases (Azure DevOps remotes, per-platform release-notes links, ticket linking via
`link_parsers`) landed on `git-cliff` either exclusively or first, because git-cliff's
TOML/Tera surface is more expressive than cog's. `communique` covers the
"generator is opaque, heraut just shells out" case for teams with their own changelog
tooling. `cocogitto` sits in between, doing a subset of what git-cliff does, with no
capability unique to it.

This is heraut's third dependency on the `cog` binary in total: (1) the dev-tooling hook
(removed by ADR-0027), (2) the `generator: cocogitto` feature itself, (3) the real-CLI
smoke tests in `internal/generators/cocogitto/` that assert the embedded `cog.toml` is
still accepted by a real `cog` binary (per `testing.md`'s documented exception). Removing
(2) makes (3) moot as well — there is no longer an embedded cocogitto config to smoke-test.

heraut is pre-v1.0 (trunk-based development, no branch protection yet — see
[`workflow.md`](../../.claude/rules/workflow.md)). The project's own convention for this
phase is direct, undeprecated changes landed on `main`; there is no installed base of
external users whose `generator: cocogitto` configs we need to support through a
transition window.

## Decision

Remove the `cocogitto` generator as a hard cutover, in one task (T117), not a deprecation
cycle:

- `generator: cocogitto` becomes a config validation error (`internal/config/validator.go`)
  with an actionable hint pointing to `git-cliff` or `communique`.
- `internal/generators/cocogitto/` (package, embedded `cog.toml`, Tera templates, contract
  tests, smoke tests) is deleted outright.
- `internal/app/`'s `buildGenerator` drops the `cocogitto` case.
- `cocogitto` is removed from `.config/mise/config.toml`'s `[tools]` — no remaining
  consumer needs it installed (ADR-0027 already removed the `cog` shell alias and dev-hook
  usage; this removes the last reason for the tool itself to be pinned).
- The Dockerfile's bundled-CLI list ([ADR-0016](0016-bundled-docker-image.md)) drops `cog`.
- `schema.json`, `docs/heraut.sample.yml`, and `docs/specs/05-generators-and-platforms.md`
  drop `cocogitto` from the `generator` enum/documentation.
- `testdata/config/` fixtures using `generator: cocogitto` are removed or migrated to
  `git-cliff`.

### What does not change

Historical ADRs that mention `cocogitto` in passing (0006, 0011, 0012, 0021, 0022, 0023,
0024) remain as accurate records of the decisions made *at the time* — they are not
rewritten. ADR-0016's bundled-CLI table is the one exception: it is a living
inventory of what the Docker image actually bundles today, so it is updated to remove the
`cog` row as part of T117's implementation, not left stale.

## Consequences

- Anyone with `generator: cocogitto` in an existing `.heraut.yml` gets a hard config error
  on their next `heraut` invocation, with a migration hint. No deprecation warning period
  precedes this.
- heraut drops its third and final dependency on `cog`/cocogitto entirely — it is no
  longer installed, configured, invoked, or referenced as a live feature anywhere in the
  codebase.
- `docs/specs/06-dx-and-testing.md`'s real-CLI smoke-test exception (`testing.md`) loses
  one of its two examples (git-cliff's remains).
- Future changelog-generator additions (if any) are evaluated against `git-cliff` and
  `communique`'s existing coverage first, to avoid reintroducing a redundant third option.
