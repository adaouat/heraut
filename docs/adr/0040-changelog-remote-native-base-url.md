# ADR-0040: `changelog.remote` for native + unified `base_url` host override

- **Status**: Accepted
- **Date**: 2026-07-22
- **Deciders**: bchatard

---

## Context

`changelog.remote` (ADR-0026) let users declare an explicit metadata remote for the
changelog, but validation restricted it to the `git-cliff` generator, and its only host
override (`api_url`) was honored for `azure_devops` only. Running `heraut changelog
--regenerate` locally with `generator: native` against a self-hosted GitLab produced a
changelog with no links and no enrichment: the changelog-only pipeline's link-context
chain is `changelog.remote → ambient CI`, and locally both were unavailable — the block
was rejected for native, and `remoteLinkContext()` hardcoded `gitlab.com`.

## Decision

1. **Remove the git-cliff-only gate.** `changelog.remote` is valid for any changelog
   generator. Both native and git-cliff consume the resolved `port.LinkContext`; the
   restriction was historical.
2. **Replace `api_url` with `base_url`.** One host override applies to all three remote
   types — `github` (default `https://github.com`), `gitlab` (`https://gitlab.com`),
   `azure_devops` (`https://dev.azure.com`). `remoteLinkContext()` uses it for every type;
   because `LinkContext.BaseURL` already drives both link rendering and `APIEnv()`
   (`GITLAB_HOST`/`GH_HOST`), self-hosted enrichment works with no new plumbing.
3. **Breaking, no shim (pre-v1.0).** `api_url` is removed; a config still using it fails
   strict loading with `unknown key`. Only Azure DevOps Server users set it; the rename to
   `base_url` is mechanical.

## Consequences

- Local self-hosted changelog regeneration now renders commit links, compare/tag links,
  and `in [!N]` MR references against the configured host.
- `by @` commit-author attribution for GitLab is **unchanged** — still unresolved
  (roadmap T150, GitHub-only today). This ADR does not address it.
- Supersedes the `api_url`-specific guidance in ADR-0026 and the `changelog.remote.api_url`
  reference in ADR-0035.
