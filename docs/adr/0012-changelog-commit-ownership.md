# ADR-0012: Changelog Commit Ownership and Release Workflow Order

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

When a release includes a changelog update (`CHANGELOG.md`), a decision was needed on
whether `heraut release` commits and pushes the file, or whether that responsibility
falls on the caller.

Additionally, the order of operations within a release needed to be defined: commit
first then tag, or tag first then commit.

## Decision

**`heraut release` owns the changelog commit.** The caller (the user, a CI job) does
not need to handle it. The workflow mirrors the `cocogitto bump` pattern:

```
1. Resolve next version
2. Generate changelog content (via configured generator)
3. Update CHANGELOG.md
4. Commit CHANGELOG.md     ← commit message: chore(release): <version>
5. Push commit to branch
6. Create tag on that commit
7. Publish release to configured platforms (gh, glab)
8. Generate and attach release notes (if configured separately from changelog)
```

> **Update (T238, 2026-08-27).** Steps 7-8 are reversed from the actual pipeline order:
> release notes are generated *before* publishing, not after — see
> [ADR-0021](0021-per-platform-release-notes.md) and ADR-0011's own T238 update note for
> the real step order. The commit-ownership decision this ADR makes (steps 1-6) is
> unaffected and still holds exactly as written.

**The tag always points to the changelog commit**, never to the previous commit. This
means the release is self-contained: checking out the tag gives you the code and the
changelog for that version simultaneously.

### Git configuration required

The release command requires the following in the local git config (or environment):

- **Push access**: the user's git credentials (or CI token via the standard git
  credential helper) must have push access to the branch.
- **Git identity**: `git config user.name` and `git config user.email` must be set.
  `heraut check runtime` verifies this. In CI, the caller is expected to configure
  these in a setup step (e.g. `git config user.name "Release Bot"`).

heraut does **not** silently set a default identity. Doing so would create a foot-gun:
a CI job that forgot to configure git would still produce commits attributed to a
generic bot, masking the misconfiguration.

### Commit message format

Default: `chore(release): <version>` (conventional commit, consistent with cocogitto).

Overridable via `.heraut.yml`:

```yaml
release:
  commit_message: "chore(release): ${version}"  # default
```

The `${version}` placeholder is substituted with the resolved version string.

### When `changelog` is omitted

If no `changelog` block is configured in `.heraut.yml`, steps 3–5 are skipped. The tag
is created directly on the current `HEAD`. No commit is made by heraut.

### Per-env `disable_changelog`

If the active environment has `disable_changelog: true` (a `versioning.environments`
field), the changelog steps (3–5) are skipped for that environment even when the root
`changelog:` block is configured. This is useful for non-prod environments where
committing `CHANGELOG.md` is undesirable (dev branches getting churned, etc.).

## Consequences

**Positive**
- Self-contained releases: the tag always points to a commit that includes its own
  changelog entry — consistent with cocogitto, a pattern teams already know.
- Callers do not need to wire up git push steps themselves; heraut handles the full
  release flow.
- Conventional commit message for the release commit keeps the changelog clean if the
  next release is also generated from commits.
- `disable_changelog` per env supports the common multi-env pattern where dev releases
  do not touch `CHANGELOG.md` but prod releases do.

**Negative / trade-offs**
- The user / CI token needs write access to the repository (push to branch + create tag).
  This is a slightly elevated permission compared to a read-only pipeline job. Teams
  must ensure their token has the right role.
- The push step can fail if the branch is protected and the token lacks sufficient
  permissions — the error surfaces `git push`'s native error verbatim with a hint
  pointing at branch protection rules.
