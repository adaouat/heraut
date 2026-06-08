# ADR-0020: Per-Platform `base_url` for Self-Hosted Instances

- **Status**: Accepted
- **Date**: 2026-06-08
- **Deciders**: bchatard

---

## Context

heraut can publish a single release to several platforms in one pipeline run
(`release.platforms`: GitHub + GitLab). Two problems converge on the same missing piece —
a per-platform notion of *which host this platform lives on*:

1. **Link resolution in release notes is wrong on the non-CI platform.** Release notes are
   generated once and reused verbatim for every platform, and the generators resolve
   commit/PR/MR links from *ambient CI environment variables* (`CI_PROJECT_URL`,
   `GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`). Whichever CI the pipeline runs in "wins" the
   link flavor — host **and** path shape (`/pull/N` vs `/-/merge_requests/N`, `/commit/sha`
   host). Every other configured platform gets notes pointing at the wrong place. This is
   Phase 14's headline gap (see
   [`.claude/plans/multi-platform-release-notes-link-resolution.md`](../../.claude/plans/multi-platform-release-notes-link-resolution.md)).

2. **Self-hosted instances cannot be resolved at all.** GitHub Enterprise and self-hosted
   GitLab live on arbitrary hosts. heraut today hardcodes `https://gitlab.com`
   (`internal/platforms/gitlab/platform.go`) and `https://github.com`
   (`internal/platforms/github/platform.go`) in `ReleaseURL`, and never tells `gh`/`glab`
   which host to talk to. Ambient CI env vars describe *where CI is running*, not *where
   each configured target platform lives*, so they cannot stand in for this.

`config.Platform` has `repository` (GitHub) and `project` (GitLab) but **no host field**.
There is nowhere to put "this GitLab target is `gitlab.example.com`, not `gitlab.com`."

This ADR records the decision to add that field. It does **not** restructure the pipeline
(that is T67's decision and T70's work) — it establishes the field, its meaning, its
defaults, and the deliberately phased rollout of its three consumers.

## Decision

Add an optional `base_url` field to `config.Platform`. It is the **single per-platform
source of truth for that platform's web host**, expressed as the browse/web base URL:

```yaml
release:
  platforms:
    - platform: github
      repository: adaouat/heraut
      # base_url omitted → defaults to https://github.com
    - platform: gitlab
      project: group/project
      base_url: https://gitlab.example.com   # self-hosted
```

### Semantics

- **Web base URL, not API endpoint.** `base_url` is the host a human browses
  (`https://gitlab.example.com`), the same root from which commit/PR/MR links are built
  (`{base_url}/{project}/-/commit/{sha}`). It is **not** the API endpoint
  (`https://gitlab.example.com/api/v4`, `https://HOST/api/v3`). The `gh`/`glab` CLIs derive
  the API path from the host themselves; heraut only ever needs the web root. Storing the
  web URL keeps one unambiguous value that all three consumers (below) can use directly.
- **Optional, with per-type defaults.** Empty → `https://github.com` for `github`,
  `https://gitlab.com` for `gitlab`. Existing configs are unaffected (wire-compatible
  additive field).
- **Trailing slashes normalized.** A stored value is trimmed of a trailing `/` so link
  construction never produces `//`.

### Three consumers, phased rollout

`base_url` feeds three distinct consumers, delivered across two threads:

| # | Consumer | Where | Thread |
|---|----------|-------|--------|
| 1 | Link resolution in release notes (per-platform host + path shape) | embedded git-cliff / cocogitto templates fed per platform | Phase 14 — T70/T71 |
| 2 | `ReleaseURL` reporter summary line | `platforms/{github,gitlab}.ReleaseURL` | Phase 14 — T66 |
| 3 | CLI host targeting — pointing `gh`/`glab` at the host so the release actually *publishes* there (`GH_HOST` / `GITLAB_HOST` and the auth probe) | `platforms/{github,gitlab}` Check/CreateRelease | deferred — multi-instance thread |

Consumers 1 and 2 work correctly for the **default** values and require no host targeting:
the link-flavor fix is meaningful precisely because the two *public* defaults already
differ in host and path shape, so regenerating notes per platform with each platform's
default `base_url` already produces correctly-flavored links. Consumer 2 simply reads the
field instead of the hardcoded constant.

Consumer 3 — making `gh`/`glab` talk to a *non-default* (self-hosted) host — is the harder,
separate problem tracked by the multi-instance thread (it also has to rework the
CI-context auth probe and the type-keyed lookups in `app/check.go`). It is **not** in
Phase 14.

### Gate non-default `base_url` until host targeting lands

Because consumer 3 is deferred, a `base_url` that differs from the platform-type default
would today produce correct-looking *links* and a correct-looking *summary URL* while
`gh`/`glab` still target the public host — a release that silently goes to (or fails
against) the wrong place. That is a worse failure mode than not having the field.

Therefore: **the validator rejects a non-default `base_url`** with an explicit error until
the host-targeting thread lands —

```
release.platforms[1].base_url: self-hosted hosts are not yet supported
  hint: base_url currently only accepts the platform default
        (https://gitlab.com); self-hosted publishing is tracked separately
        (ADR-0020, multi-instance thread)
```

The field and schema ship now (forward-compatible; IDE autocomplete and the sample can
document it), but the only accepted value is the per-type default until consumer 3 exists.
When host targeting lands, the gate is lifted in that same change. This keeps the field
from ever silently half-working.

### Non-regression invariant: single-platform CI flows are untouched

Today's single-platform CI flows work *because* the templates resolve links from ambient
CI vars — and that is already correct for self-hosted instances: a self-hosted GitLab
runner sets `CI_PROJECT_URL=https://gitlab.example.com/...` and `glab` publishes via CI
autologin, so heraut never needs the host. This must not regress. Therefore:

**heraut injects per-platform link context only when it would change the answer — when
more than one platform is configured** (and, once the gate above lifts, when `base_url` is
explicitly non-default). With exactly one platform and an unset (default) `base_url`,
heraut injects **nothing**: notes generate once, exactly as today, and the templates fall
through to ambient-CI detection. **Corollary: heraut must never override a more-specific
ambient CI value with a less-specific default `base_url`** — the injected variable being
empty must mean "fall through to ambient", never "use the default".

The validator gate is what makes this safe in the multi-platform path as well: because
every *injectable* `base_url` is currently a public default, the injected value can never
be less specific than what ambient CI would have produced for that platform. The only way
a self-hosted host reaches the notes is via ambient detection in the single-platform path
— which is preserved precisely by not injecting there. (Enforced by the T70/T71
non-regression acceptance tests.)

### Scope and non-goals

- **No per-environment merge logic.** `release.platforms` is replaced wholesale per
  environment (per [ADR-0019](0019-perenv-content-driver-merge.md), lists stay replace), so
  a per-env platform block already carries its own `base_url`. Platform-level granularity
  is sufficient; no special deep-merge for `base_url` is needed. (Resolves design-note
  open question 2.)
- **Not the multi-instance capability.** Two platforms of the *same type* (two GitLab
  instances) is a distinct architectural capability with its own gaps (`findPlatformCfg`
  first-match, ambiguous reporter `Name()`, CI-context auth probe). `base_url` is a
  *prerequisite* for it, not the capability itself. That work is deliberately out of scope
  here.
- **No API-endpoint field.** Only the web base URL is stored; the API path is the CLIs'
  concern.

### Relationship to T67 (per-platform notes regeneration)

This ADR adds the *data* (each platform's host); [T67's ADR] decides the *behavioural
shift* that makes consumer 1 possible — moving release-notes generation inside the
per-platform loop so each platform's notes are rendered with its own `base_url` +
`repository`/`project` rather than once globally from ambient CI vars. The two are
sequenced: `base_url` (T65/T66) must exist before notes can be regenerated *per its value*
(T67/T70). T71 then teaches the embedded templates to prefer the injected per-platform
context over the ambient-CI fallback.

## Consequences

**Positive**

- One unambiguous per-platform host value, consumed by link resolution, the summary URL,
  and (later) CLI targeting — no second wire-format change when host targeting lands.
- The link-flavor fix (the Phase 14 driver) is unblocked immediately for the common
  public-GitHub + public-GitLab case, with no dependency on the harder self-hosted work.
- Self-hosted users get a clear, actionable error instead of a release published to the
  wrong host.
- `ReleaseURL` stops hardcoding `gitlab.com`/`github.com`, removing a latent bug.

**Negative / trade-offs**

- The field is visible in schema and sample before it is fully functional for self-hosted
  hosts. Mitigated by the validator gate and an explicit "tracked separately" hint — the
  field never silently half-works, and the limitation is discoverable at config-load time
  rather than at publish time.
- The validator carries a temporary gate that must be removed when consumer 3 lands. This
  is a small, well-marked piece of debt (a single check with a pointer to this ADR), and
  removing it is itself a natural acceptance criterion for the host-targeting task.

## Alternatives considered

- **Fold host targeting into T66 (make `base_url` fully live on landing).** Wire
  `GH_HOST`/`GITLAB_HOST`, the auth probe, `ReleaseURL`, and link resolution together so
  `base_url` means "publish here" from day one. Rejected for *this* phase: it pulls the
  multi-instance thread's hardest pieces (CI-context auth rework, type-keyed lookup fixes)
  forward into what is meant to be the link-flavor fix, inflating T66 from M to L and
  coupling two threads the design note deliberately separated. The gate lets the field land
  cleanly now and the host-targeting work proceed on its own schedule.
- **Accept the phased state with loud docs only (no validator gate).** Ship `base_url` for
  link-resolution + display, document that self-hosted *publishing* isn't wired. Rejected:
  documentation does not stop a release from going to the wrong host at runtime; a
  config-time error is strictly safer and costs one validator check.
- **Store the API endpoint instead of the web URL.** Rejected: link construction needs the
  web root, the CLIs derive the API path themselves, and GitHub Enterprise's
  `/api/v3` vs GitLab's `/api/v4` split would force per-type endpoint logic for no benefit.
- **Sniff the host from ambient CI env vars (status quo, extended).** Rejected as the
  root cause: those vars describe the CI runner's own project, not each configured target
  platform — the exact conflation this ADR exists to remove.
