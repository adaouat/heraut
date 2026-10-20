# ADR-0075: `security` is a built-in commit type, and the body heuristic goes

- **Status**: Accepted
- **Date**: 2026-10-20
- **Deciders**: bchatard

---

## Context

The native generator groups each commit by its conventional-commit type. A commit whose type is not a
configured type falls through three built-in fallbacks: a "Security" group, a "Revert" group and a
catch-all "Other". The Security fallback is `strings.Contains(raw.Body, "security")`
(`internal/generators/native/group.go`), left over from the git-cliff taxonomy heraut replaced
(ADR-0045). The rule review of 2026-10-16 (RULE-013) found three problems:

- It matches a lowercase substring of the body, so "insecurity" matches and "Security fix" does not.
- It ignores the subject and applies only to commits of unconfigured types, so it surprises exactly the
  users who did not ask for it.
- No spec describes it.

Every other section comes from a type in `defaultTypes()` (`internal/config/commits.go`), which also
feeds the `commit verify` allow-list (ADR-0027) and the `commit create` picker. A user who wants a
Security section can already add a `security` type, but nothing does it for them.

## Decision

1. **Remove the body heuristic.** `classify` no longer looks at the commit body. A commit lands in
   Security only when its type is `security`.
2. **Add `security` to the built-in types**, rendered as `🛡️ Security`, with the description
   "A security fix or hardening". It sits right after Bug Fixes, so the default orders become: feat 0,
   fix 1, security 2, refactor 3, docs 4, perf 5, style 6, test 7, chore and ci 8. The relative order of
   the existing sections does not change; only their numbers shift by one.
3. **Users keep control.** The type merges like any other: a `commits.types` entry named `security`
   replaces it wholesale, and `remove: true` drops it from the allow-list and the changelog.
4. The Security fallback group and its order constant are deleted. Revert and Other stay fallbacks.

## Consequences

- `security: patch the token leak` and `security(auth): …` now pass `heraut commit verify` for every
  user, and `commit create` offers the type. This is a loosening of the ADR-0027 allow-list, limited to
  one word. A repository that wants it stricter sets `remove: true`.
- A commit of an unconfigured type that mentions "security" in its body moves from Security to Other.
  Such commits were misfiled by a substring match; writing the type is the supported way to get the
  section.
- The tests that pin the old behavior change on purpose: the table row "non-type subject + security body
  → Security" now expects Other, and the numeric orders of refactor and the sections after it move by
  one. No row is deleted.
- Spec 02 lists the default types and Spec 05 describes classification; both gain the new type, and
  Spec 05 states that classification looks at the type only.

## Alternatives considered

- **Whole-word, case-insensitive body match.** Fixes the false matches but keeps a hidden rule that
  depends on prose, and it still would not be documented anywhere a user looks.
- **Document a `commits.types` snippet instead of a built-in type.** Leaves the allow-list unchanged,
  but every user who wants the section has to write config that heraut can ship.
