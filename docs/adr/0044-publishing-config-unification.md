# ADR-0044: Publishing config unification — `release.targets` replaces `release.platforms`

- **Status**: Accepted
- **Date**: 2026-07-31
- **Deciders**: bchatard
- **Extends / supersedes**: [ADR-0043](0043-forge-abstraction.md) (supersedes its P3 framing,
  which assumed publishing would also move onto a new transport), [ADR-0025](0025-multi-instance-platforms.md)
  (updates the config surface for multi-instance publishing)

---

## Context

[ADR-0043](0043-forge-abstraction.md) unified enrichment config around a top-level `forges:`
list, additive alongside the pre-existing `release.platforms:`. That left the config with two
names for the same underlying thing on the publishing side: a `release.platforms` entry still
carried its own `base_url`, `token_env`, and `repository`/`project` — duplicating what a
`forges:` entry already declared — and publishing resolved none of it from the CI/git
environment the way enrichment now did. `release.targets[]` was added in T155 and validated
from T156 onward, but nothing consumed it: `release.platforms` remained the only surface that
actually created a release.

ADR-0043 originally scoped this as "P3 — fold publishing into `port.Forge`", implying
publishing's HTTP transport would also change (`gh`/`glab` replaced by a native client, an SDK,
or similar), with the transport choice deferred to its own decision. Scoping the actual P3 work
surfaced that the config-unification goal and the transport-replacement goal are independent,
and conflating them added risk without adding user-visible value — see Decision below.

## Decision

`release.targets[]` is the publishing surface. `release.platforms` is removed, with a
migration error (`ErrRemovedConfigKey`) mapping the old shape to the new one: declare a
`forges:` entry carrying `base_url` / `token_env` / `repository`-or-`project`, then reference
it from `release.targets[].forge`, keeping `draft` / `prerelease` / `assets` on the target.

```yaml
forges:
  - name: gitlab-saas
    platform: gitlab
    # project / base_url / token_env optional — inferred from CI or git origin

commits:
  enrichment_forge: gitlab-saas
  enrichment_policy: optional

release:
  notes:
    generator: native
  targets:
    - forge: gitlab-saas
      draft: false
      prerelease: false
      assets: ["dist/*"]
```

Platform drivers (`internal/platforms/{github,gitlab}`) are constructed from the resolved
`port.ForgeIdentity` rather than a standalone `config.Platform` block: `internal/app`
(`platformConfigFromTarget`) builds a driver's config from a target's forge reference plus the
identity `forge.Resolve` already produces for enrichment. Publishing therefore inherits the
same CI/git auto-detection enrichment gained in ADR-0043 — a target typically needs no more
than `forge:` (or nothing at all, in the single-forge case), and omitting `release.targets`
entirely still publishes to the single resolved forge with default options (zero-config
publishing). "`heraut release` requires ≥1 entry in `release.platforms`" becomes "`heraut
release` requires ≥1 resolvable publish destination" — zero resolvable destinations remains a
configuration error, not a silent no-op.

Multi-instance publishing (ADR-0025) is preserved: multiple `forges:` entries, each referenced
by its own `release.targets` entry, construct one driver per target — the same one-driver-per-entry
property ADR-0025 established, now expressed as targets/forges instead of a single
`release.platforms` list.

### Transport decision: `gh`/`glab` are unchanged

`port.Platform` and `internal/platforms/{github,gitlab}` do not change in this phase — only how
they are *configured and constructed* does. Three reasons, recorded here so the decision is not
re-litigated:

1. **The config goal does not require a transport change.** Collapsing two config surfaces into
   one (`forges:` + `release.targets:`) is achievable while keeping the current transport; the
   two goals are independent, and coupling them only adds risk to the config change.
2. **The original pain is already solved.** The forge epic began because `CI_JOB_TOKEN` could
   not enrich (GitLab's GraphQL API rejects job tokens). P1 fixed that with a native GitLab
   enrichment client. Publishing has no equivalent live defect: `glab` already auto-authenticates
   on gitlab.com CI (`inCIAutologin`), so token-free publishing already works there today. There
   is no comparable pain to relieve on the publish path.
3. **Risk asymmetry, demonstrated by P2.** P2 (migrating GitHub and Azure onto `port.Forge`)
   shipped two Critical defects — a wrong GHES GraphQL endpoint and a duplicated Azure
   organization in both the API endpoint and every rendered link — that a fully green test suite
   missed, because `httptest`-backed contract tests validate response handling, not request
   shape, unless a test pins the exact path. Publishing is the worst place to repeat that
   mistake: `gh`/`glab` already handle GitHub's separate `uploads.` host and GitLab's
   package-registry two-step upload correctly, for free. Replacing that with hand-rolled HTTP
   risks shipping a broken artifact upload behind a suite that still reports green.

## Consequences

- **Breaking config change, acceptable pre-v1.0.** Every existing config with
  `release.platforms` fails to load with `ErrRemovedConfigKey` until migrated; the error names
  the exact replacement (`forges:` + `release.targets[].forge`). `schema.json`,
  `docs/heraut.sample.yml`, and `docs/specs/02-configuration.md` update in lockstep. heraut's own
  `.config/heraut.yml` is migrated as part of this change, dogfooding the path.
- **`gh`/`glab` remain runtime dependencies.** They stay required on `PATH` (`heraut check
  runtime` verifies them) and stay bundled in the Docker image ([ADR-0016](0016-bundled-docker-image.md),
  unchanged by this ADR).
- **Native publishing remains available as a future, separately-motivated task.** The drivers
  sit behind `port.Platform`, so the transport can still be swapped later without touching
  config again. That future work is what would actually drop the `gh`/`glab` dependency, shrink
  the Docker image, and enable `CI_JOB_TOKEN` publishing on **self-hosted** GitLab CI — the one
  gap `glab`'s CI autologin does not cover (autologin is gitlab.com-specific). Nothing in this
  ADR forecloses it.
- **`internal/scaffold` emits the new shape mechanically.** The wizard's prompts, flow, and
  `PlatformAnswer` internals are unchanged in this phase — only what it emits (`forges:` +
  `release.targets` instead of `release.platforms`) and what it reads back when round-tripping
  an existing config. The wizard's redesign — forge-aware prompts, an `api_mode` question,
  auto-detection defaults — is a later, separately motivated task (T164), deliberately sequenced
  after the new config schema is battle-tested.

## Alternatives considered

- **Fold publishing into `port.Forge` and replace the transport in the same phase**, as
  ADR-0043 originally scoped P3. Rejected: see Transport decision above — it would have coupled
  an independent, higher-risk change to the config unification, for no corresponding config
  benefit.
- **Alias `release.platforms` to the new shape instead of a hard migration error.** Rejected for
  the same reason ADR-0043 rejected aliasing `changelog.remote`: a config that silently maps old
  fields onto the new shape can misplace `assets`/`draft`, which used to live on the platform
  entry and now belong on the target. A loud, actionable migration error is cheaper than a
  silently wrong mapping.
- **Keep `release.platforms` and `release.targets` both live indefinitely.** Rejected: the
  entire point of this epic is one name for one concept; keeping both permanently reproduces the
  problem ADR-0043 set out to fix, just moved from `changelog.remote` to `release.platforms`.

## References

- Design spec: [`docs/superpowers/specs/2026-07-27-forge-publishing-config-unification-design.md`](../superpowers/specs/2026-07-27-forge-publishing-config-unification-design.md)
- [ADR-0043](0043-forge-abstraction.md) — forge abstraction + unified `forges:` config; this ADR
  supersedes its P3 framing (transport replacement) while completing its publishing-config goal
- [ADR-0025](0025-multi-instance-platforms.md) — multi-instance same-platform releases; its
  config surface (a `release.platforms` list with unique `name`s) is superseded by multiple
  `forges:` entries each referenced by a `release.targets` entry, same one-driver-per-entry
  property
- [ADR-0016](0016-bundled-docker-image.md) — bundled Docker image; unchanged, `gh`/`glab` remain
  bundled
