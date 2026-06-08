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

### Deferred to subsequent tasks (not decided here)

- **The context-injection shape** — how the per-platform context reaches the generator
  (reuse existing env vars vs. new heraut-owned template variables) — is the [T68]
  mini-spike. It is the crux of keeping git-cliff and cocogitto consistent and must
  confirm what cocogitto's Tera context accepts.
- **The `port.Generator` interface change** to carry the context is [T69]; the embedded
  template updates are [T71]. This ADR records *that* notes are regenerated per platform
  and *where* in the pipeline — not the interface signature or template bytes.

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
