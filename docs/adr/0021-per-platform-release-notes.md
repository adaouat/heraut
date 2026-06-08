# ADR-0021: Release Notes Regenerated Per Platform

- **Status**: Accepted
- **Date**: 2026-06-08
- **Deciders**: bchatard

---

## Context

`heraut release` can publish one release to several platforms in a single run. Today the
release-notes body is generated **once** and reused verbatim for every platform
(`internal/pipeline/release.go`):

```go
// Step 6 — generated once
notes, genErr = p.cfg.Notes.Generate(result.Tag)

// Step 7 — the same string reused for every platform
for _, platform := range p.cfg.Platforms {
    plat.CreateRelease(result.Tag, notes)
}
```

The generators resolve commit/PR/MR links from ambient CI environment variables
(`CI_PROJECT_URL` / `GITHUB_SERVER_URL` + `GITHUB_REPOSITORY`). Whichever CI the pipeline
runs in "wins" the link flavor — host **and** path shape (`/pull/N` vs
`/-/merge_requests/N`). Every other configured platform then receives a release whose
notes point at the wrong host with the wrong link paths. [ADR-0020](0020-platform-base-url.md)
added `base_url` as the per-platform host data; this ADR records the behavioural change
that *uses* it.

[ADR-0017](0017-pipeline-progress-reporter.md) governs the pipeline's step model: the step
**total** is pre-computed from config at construction (`app.releaseStepTotal`) so the
`[N/total]` counter is known up front, and logically-nested work (asset upload) is rendered
as a **sub-result** of its parent step, not as a separate numbered step. Any change to
where notes are generated changes that step inventory, so it must be reconciled with
ADR-0017.

## Decision

**Release-notes generation moves into the per-platform publish loop, regenerated once per
platform with that platform's link-resolution context — but only when more than one
platform is configured.** With a single platform, the existing single pre-loop generation
is kept unchanged.

### Why regenerate, not "smarter templates" alone

A smarter template that prefers a heraut-supplied host is inert if the rendered string is
produced once and reused: the string is frozen after the first render. Per-platform links
require per-platform **rendering**. Regeneration is the load-bearing change; the template
update ([T71]) only decides *what* each render reads. (This is why the design spike's
options C + D are jointly necessary and option D alone is not.)

### The single-platform path is unchanged (non-regression)

Per ADR-0020's non-regression invariant, heraut injects per-platform context **only when
it would change the answer** — i.e. when more than one platform is configured (and, once
the ADR-0020 gate lifts, when `base_url` is explicitly non-default). With exactly one
platform and a default `base_url`:

- notes are generated **once**, before the loop, with **no** injected context;
- the templates fall through to ambient-CI detection, exactly as today — including
  self-hosted instances whose correct host arrives via `CI_PROJECT_URL`.

heraut must never replace a single platform's ambient-derived links with a less-specific
default-`base_url` value. The gate guarantees this stays safe in the multi-platform path
too (every injectable `base_url` is currently a public default, so the injected value is
never less specific than ambient would have produced).

### Effect on the step model (reconciling ADR-0017)

Per-platform notes generation is logically part of *producing each platform's release* —
the same relationship asset upload already has to "Publish to {platform}" under ADR-0017.
It therefore follows the sub-result precedent rather than becoming its own numbered step:

- **Single platform (the common case): step inventory is unchanged.** A standalone
  `Generate release notes` step (when notes are enabled) followed by one
  `Publish to {platform}` step — byte-for-byte today's structure and count.
- **Multiple platforms:** the standalone `Generate release notes` step is **omitted**;
  notes generation folds into each `Publish to {platform}` step (generate that platform's
  notes → `CreateRelease` → optional asset upload) and is surfaced as a sub-result
  (`notes generated`) beside the existing `assets uploaded` line.

`app.releaseStepTotal` branches on `len(cfg.Platforms) > 1`: it adds the `+1` notes step
only in the single-platform case; in the multi-platform case the `+len(Platforms)` publish
steps already cover notes generation. Because the total is pre-computed from config and
`len(Platforms)` is known, the `[N/total]` counter stays correct without a two-pass scheme.

Dry-run output mirrors this: single-platform keeps today's `Generate release notes` line;
multi-platform reports the would-be generation per platform within the publish step.

### The committed `CHANGELOG.md` is unaffected

This decision concerns **release notes only** — the ephemeral, per-release-body artifact.
The committed `CHANGELOG.md` (Step 2, `cfg.Changelog.Generate`) stays a **single canonical
generation** tied to wherever `origin` is, generated once and committed once. A committed
file cannot have two link flavors, and nothing here changes Step 2. This is "main platform"
thinking (design-spike option A) correctly scoped to the one artifact that genuinely needs
a single answer.

### Context-injection shape (resolved by T68)

T68 investigated *how* the per-platform context reaches each generator, with an empirical
proof-of-concept against the installed `cog 7.0.0` and `git-cliff 2.13.1`. The two
generators turn out to resolve links through **completely different surfaces**, so a single
uniform mechanism is the wrong abstraction:

- **git-cliff** resolves links in its Tera template via `get_env(name=…)`, reading the
  subprocess environment. heraut already writes the full merged TOML per call and can pass
  extra env vars via `port.Runner.RunEnv`.
- **cocogitto** resolves links from `--remote` / `--owner` / `--repository` **CLI flags**
  (equivalently `[changelog] remote/owner/repository` keys in the cog.toml heraut already
  writes). cog's `-t remote` template then renders host-correct links. PoC confirmed the
  same flags produce `https://gitlab.example.com/...` *and* `https://github.com/...`
  correctly, self-hosted included. cog's Tera *also* supports `get_env`, but cog's
  idiomatic surface is the remote/owner/repository config — not env vars — and the embedded
  cog templates render **no links at all** today (T71 must add link rendering regardless).

**Decision: a generator-agnostic `LinkContext` at the `port.Generator` boundary, translated
by each adapter into that tool's native mechanism.** This is "new heraut-owned context"
(option b) rather than "reuse ambient CI env vars" (option a) — but the unification lives at
the `LinkContext` abstraction, not at the wire mechanism:

- **git-cliff adapter** injects heraut-owned env vars (e.g. `HERAUT_REMOTE_URL`, plus a
  platform-type marker for the PR vs MR path shape) via `RunEnv`; the embedded macro reads
  them with `get_env`, keeping the existing `CI_PROJECT_URL` / `GITHUB_SERVER_URL` chain as
  the `default=` fallback.
- **cocogitto adapter** passes `--remote/--owner/--repository` (or injects the equivalent
  `[changelog]` keys into the temp cog.toml) and selects cog's remote-linking template.
- **communique adapter** ignores the context (opaque — see below).

This was rejected for option (a): reusing ambient CI vars *as the primary mechanism* can't
work, because heraut needs to **override** the ambient value per target platform, and an
ambient var describes the CI runner's own host, not the target. Overriding requires a
heraut-owned value that takes precedence — which *is* option (b). Forcing cog onto `get_env`
would also fight its idiomatic remote/owner/repository surface.

**PoC-confirmed non-regression (git-cliff):** with `HERAUT_REMOTE_URL` set, the injected
value wins; with it unset, the macro falls through to ambient `CI_PROJECT_URL` (self-hosted
host preserved). Injecting only in the multi-platform path therefore leaves single-platform
CI runs byte-for-byte unchanged (ADR-0020 invariant). Note the asymmetry handed to T70/T71:
cog has **no** ambient-CI fallback (it renders no links without explicit remote/owner), so
the ">1 platform" injection gate exists primarily to protect git-cliff's ambient behavior;
cog could safely receive context even single-platform, but applying the uniform gate is
simplest and merely means cog links first appear in multi-platform mode.

**Shape for T69 (`port.Generator`):** carry an optional per-platform context, where absent
(`nil`) means "no per-platform context → fall through to ambient detection" (the
single-platform path). Indicative fields, to be finalized in T69:

```go
type LinkContext struct {
    BaseURL  string // resolved per-platform web base URL, e.g. https://gitlab.example.com
    Owner    string // org / namespace (GitLab group[/subgroup])
    Repo     string // repository / project name
    Platform string // "github" | "gitlab" — selects PR (/pull/N) vs MR (/-/merge_requests/N) shape
}
```

(GitLab project paths like `group/sub/proj` split into `owner=group/sub`, `repo=proj`;
the adapter owns that parsing. cog wants `--remote` as the bare host without scheme, so the
cocogitto adapter strips the scheme from `BaseURL`.)

- **The `port.Generator` interface change** to carry the context is [T69]; the embedded
  template updates are [T71]. This ADR records *that* notes are regenerated per platform,
  *where* in the pipeline, and *by what mechanism* per generator — not the final interface
  signature or template bytes.

### communique

communique is opaque to heraut (`communique generate --config <user-file> <tag>`); it has
no heraut-controllable link-resolution surface, so per-platform regeneration produces
**identical** output for every platform. Regenerating it per platform is therefore
redundant but harmless. The pipeline **may** generate once and reuse for a context-blind
generator as an optimization, but correctness does not depend on it. The user-facing
limitation (communique users get identical notes across platforms) is documented in [T73].

### Out of scope

- **The tag-sync / target-pinning race** (a mirrored secondary platform may not have the
  tag yet when its release is created) is an orthogonal timing problem, not a link-flavor
  problem. It is not addressed here.

## Consequences

**Positive**

- Each platform's release notes carry that platform's own host and link-path shape — the
  Phase 14 goal.
- Single-platform and CI-native flows are provably unchanged: same step count, same
  ambient-CI link resolution, same dry-run output (enforced by the T70 non-regression
  acceptance tests).
- Reuses ADR-0017's sub-result mechanism rather than inventing a new step category.

**Negative / trade-offs**

- In multi-platform mode the notes generator runs N times instead of once — N external
  CLI invocations. Notes generation is cheap relative to the publish/upload calls that
  already run per platform, so the cost is marginal; noted rather than optimized.
- Notes generation in multi-platform mode is no longer its own numbered step — it runs
  inside the publish step's spinner and is reported retroactively as a sub-result. This is
  slightly less granular live feedback than a dedicated step, but it matches the existing
  treatment of asset upload, so the model stays consistent.
- `releaseStepTotal` gains a `len(Platforms) > 1` branch — a small, well-contained
  conditional that the step-count tests must cover for both arms.

## Alternatives considered

- **Smarter templates only, single global generation.** Rejected: a string rendered once
  cannot carry per-platform links no matter how smart the template — see "Why regenerate".
- **Always fold notes into the publish step (single-platform too).** Rejected: it removes
  the standalone `Generate release notes` step for single-platform users, changing today's
  step count and dry-run output for the overwhelming-majority case — a gratuitous
  regression for no benefit, since single-platform needs no per-platform rendering.
- **A separate `Generate notes for {platform}` numbered step per platform (2N steps).**
  More transparent live feedback, but noisier and inconsistent with ADR-0017's decision to
  treat logically-nested per-platform work (asset upload) as a sub-result. Rejected in
  favour of the sub-result rendering.
- **Always regenerate per platform, including communique.** Acceptable for correctness but
  wasteful for a generator that ignores the context; left as implementation latitude (the
  pipeline may special-case context-blind generators) rather than mandated either way.
