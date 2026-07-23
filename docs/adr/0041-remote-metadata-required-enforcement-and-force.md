# ADR-0041: `remote_metadata: required` enforcement and `--force` override

- **Status**: Accepted
- **Date**: 2026-07-23
- **Deciders**: bchatard

---

## Context

`commits.remote_metadata` (ADR-0023; native policy in ADR-0034 §6) selects how content
generators treat PR/MR enrichment: `required` (fail if unavailable), `optional` (degrade with
a warning), `disabled` (never fetch). The native generator's `enrichForRelease` only treated
`required` as fatal when a fetch was *attempted* and returned an error. A nil or unsupported
`LinkContext` — no changelog remote and no ambient/platform context — yields no error, so
`required` was silently satisfied when there was nothing to fetch from. Native was permanently
in this state before `changelog.remote` worked with the native generator (ADR-0040): its
changelog `LinkContext` was always nil, so `required` never fired.

Separately, `heraut changelog` read the global `--force` flag but never plumbed it into
generation, so `--force` had no effect on the changelog content step.

## Decision

1. **`required` means required.** Under `remote_metadata: required`, enrichment that cannot be
   satisfied — a fetch failure, *or* a nil / unsupported `LinkContext` (no remote or platform
   to fetch from) — is a hard error. Rendering a metadata-less changelog under `required` is a
   silent contract violation, not a success.

2. **`--force` downgrades `required` to `optional`.** With `--force`, a `required` enrichment
   failure degrades (warn + render without attribution) instead of erroring, for both
   `heraut changelog` and `heraut release`. This mirrors `--force` as the escape hatch for the
   per-env promotion guards (ADR-0007): `required` is the strict default; `--force` is the
   deliberate override for offline / unreachable-remote runs.

`--force` is threaded `PipelineOpts.Force` → `buildGenerator` → the native `ContentDriver`;
`enrichForRelease` reads it. The release pipeline already carried `opts.Force`, so it gains the
same override.

## Consequences

- A `required` policy with no configured remote now fails loudly at generation time instead of
  producing an unattributed changelog — surfacing the misconfiguration the operator asked to be
  strict about.
- `--force` gains a generation-time effect it previously lacked, documented in Spec 03's
  root-flag table and Spec 02's `commits.remote_metadata`.
- `optional` and `disabled` are unchanged; `--offline` still forces `disabled`.
