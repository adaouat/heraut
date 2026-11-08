# ADR-0081: `--force` means "release from here"

- **Status**: Accepted
- **Date**: 2026-11-08
- **Deciders**: bchatard
- **Amends**: [ADR-0007](0007-version-promotion-error-handling.md) (E001 is no longer bypassable)

---

## Context

`--force` had grown six unrelated meanings on `heraut release`: it bypassed E001 (the promotion's
target tag exists), E002 (the destination is ahead), the unlisted-branch refusal
(`versioning.branches`), the per-environment branch guard, the per-env `--set-version` tag probe, and
it downgraded `commits.enrichment_policy: required` to `optional`. A CI job that needed one of them got
all of them; before T494 that included every GitLab CI per-env job, which had to pass `--force` to get
past the branch guard and so silently dropped its promotion guards too.

One of the six could never work. With E001 forced, the run regenerated the changelog, committed and
pushed it, then failed at `git tag` because the tag exists — leaving a pushed `chore(release)` commit
behind.

Splitting `--force` into one flag per guard was considered and rejected as too heavy to use.

## Decision

`--force` means "I know where I am releasing from". On `release`, `changelog` and `version next` it
lifts:

- **E002**, the destination being ahead of the promotion candidate;
- the **branch guards**: the unlisted-branch refusal and the per-environment `branch:` guard.

On `version current` it lifts the per-environment branch guard, the only one that applies there.

It no longer lifts anything else:

- **E001 is never bypassed.** The run stops before anything is written. Re-releasing a version means
  deleting its tag first, which the E001 message explains. The per-env `--set-version` probe runs
  under `--force` too, for the same reason.
- **`enrichment_policy: required` is never downgraded.** To release without PR/MR data, pass
  `--offline`, which already forces `disabled` for the run. `heraut notes` loses its `--force` flag,
  whose only meaning this was.

E003 was never bypassable and stays so.

## Consequences

- No new flag, and each remaining meaning of `--force` is about where the release comes from.
- A config with `enrichment_policy: required` that used `--force` to get past a forge outage now uses
  `--offline` (no PR data at all, rather than a best-effort fetch).
- ADR-0007's matrix changes for E001; the specs and guides list what `--force` lifts.
