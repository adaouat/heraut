# ADR-0067: CalVer `YYYY` is the ISO year when the format has `WW`

- **Status**: Accepted
- **Date**: 2026-10-07
- **Deciders**: bchatard

---

## Context

`YYYY` rendered the calendar year and `WW` the ISO week, both from the same instant. Around New Year
the two come from different years, so `YYYY.WW.PATCH` produced pairings that do not exist and, in
late December, a tag that may already exist:

- 2027-01-01 is ISO week 53 of 2026 and rendered `2027.53.x` (and 2027-01-04 `2027.01.x`).
- 2025-12-29 is ISO week 1 of 2026 and rendered `2025.01.x`, so with `2025.01.0` already tagged the
  next version was `2025.01.0` again: a collision with January's tag, or a version going backwards.

The e2e suite found it (T345b1, T358).

## Decision

When the format contains `WW`, the `YYYY` value is the ISO year (`time.Time.ISOWeek` year). Every
other format keeps the calendar year. The change is in `valuesFromTime`; period comparison already
includes the year, so `PATCH` reset needs no change. No new token, no config option.

## Alternatives considered

- **A separate ISO-year token (`GGGG`):** explicit, but leaves every existing `YYYY.WW.PATCH`
  repository with the collision until it edits its format, and adds a token to the parser, spec,
  schema and samples.
- **Document the behaviour:** a duplicate tag is a defect, not a quirk.

## Consequences

- Only `YYYY.WW` formats change, and only for the few days around New Year, from wrong to right.
- Tags minted earlier (e.g. `2027.53.0`) keep parsing and ordering as plain numbers.
- Spec 04 documents `YYYY` as the ISO year in the presence of `WW`.
