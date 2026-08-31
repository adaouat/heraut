# ADR-0050: Changelog preamble/postamble always render fresh — no more "frozen until regenerate"

- **Status**: Accepted
- **Date**: 2026-08-31
- **Deciders**: bchatard
- **Supersedes**: the "Preamble — free-form content before the first anchor. Incremental mode
  preserves it verbatim." line in [ADR-0038](0038-incremental-changelog.md); the frozen-until-
  `--regenerate` framing in [ADR-0049](0049-changelog-release-notes-footer-block.md)

---

## Context

Since [ADR-0038](0038-incremental-changelog.md), an ordinary incremental `heraut changelog` run
preserved the changelog's preamble (everything before the first section anchor) verbatim, on the
theory that it might be free-form, hand-edited content worth protecting from being clobbered.
[ADR-0048](0048-changelog-title-subtitle-blocks.md)'s `title`/`subtitle` and
[ADR-0049](0049-changelog-release-notes-footer-block.md)'s `footer` inherited this: all three
blocks rendered fresh only on bootstrap or `--regenerate-changelog`/`--regenerate`, otherwise
whatever was already on disk stayed untouched — confirmed by test
(`TestGenerator_GenerateChangelog_IncrementalPreservesExistingTitle`) and, for `footer`, by a live
run showing an unchanged timestamp across two ordinary `heraut changelog` invocations.

Release notes never had this wrinkle: there is no persisted file to splice against, so every
`heraut release` already renders `title`/`subtitle`/`footer` fresh, every time — this was true
before today and remains true. The asymmetry — release notes always fresh, changelog frozen until
an explicit flag — was surfaced as unwanted: the changelog's `title`/`subtitle`/`footer` should
behave the same way, with no `--regenerate` requirement.

## Decision

`heraut changelog` (and `heraut release`'s changelog step) now re-renders `title`/`subtitle`/
`footer` from current config on **every** run, incremental or not — matching release notes'
always-fresh behavior exactly. `--regenerate-changelog`/`--regenerate` keeps its existing meaning
unchanged: it controls whether **historical sections** are re-enriched (re-fetching PR/MR
metadata, the actually expensive part — one API call per commit on GitLab). Preamble/postamble
freshness is no longer coupled to that flag at all.

**Trade-off, accepted explicitly:** a hand-edited title, subtitle, or footer written directly into
`CHANGELOG.md` no longer survives the next `heraut changelog` run — it is silently replaced by
whatever the current config renders. `TestGenerator_GenerateChangelog_IncrementalPreservesExisting-
Title` is renamed and inverted to `..._IncrementalRefreshesTitle`, asserting exactly this.

### Mechanism

`generateIncremental` now calls `renderPreamble`/`renderPostamble` unconditionally (previously only
`buildAllSections`, the bootstrap/regenerate path, did) and passes the fresh values into
`spliceSection`, which no longer reuses whatever `parseChangelog` found on disk for either.

The historical-sections-preserved guarantee is untouched: `spliceSection` still only replaces the
top section (or inserts a new one above it) and leaves every other section's previously rendered
text — including its PR attribution — byte-for-byte as it was. Only the preamble and postamble are
now discard-and-replace on every call.

**The postamble needed a new structural marker to make this safe.** Historical sections'
text is captured by `parseChangelog` as "everything from this anchor to the next anchor (or end of
content, for the last one)" — so a document footer trailing the last section was, before this
change, indistinguishable from that section's own body; nothing bounded where the section's real
content ended and the footer began. Freshly re-rendering and *discarding* the old footer on every
run needs to know exactly what to discard. A new invisible marker, `<!-- heraut-footer -->`
(`internal/generators/native/changelogfile.go`), placed immediately before the rendered postamble
— structural and non-overridable, the same pattern the section anchors already use — lets
`parseChangelog` strip the whole footer region before finding section boundaries, so the true last
section is correctly bounded and the stale footer is never folded into it.

No equivalent marker is needed for the preamble: it's already unambiguously "everything before the
first section anchor," a boundary `parseChangelog` already computes — freely discardable without
any new bookkeeping.

## Consequences

- **No public release affected.** `title`/`subtitle` shipped in a tagged release under the old
  (frozen-until-regenerate) behavior, but no project has depended on hand-editing them in a way
  that would now break silently in a way heraut itself could detect — this is a behavior change
  users discover the next time they hand-edit and re-run, not a migration.
- **A new on-disk marker for projects using the document-level `footer` block.** Since that block
  was never in a tagged release ([ADR-0049](0049-changelog-release-notes-footer-block.md)'s own
  "Consequences"), there is no real-world unmarked-footer file to migrate — the marker ships as
  part of the same not-yet-released feature.
- **Determinism note in the template-customization guide corrected again.** ADR-0049 documented
  the (then-true) "frozen until regenerate" framing for `.Heraut.GeneratedAt`; that framing is now
  gone. The footer's timestamp changes on every `heraut changelog`/`heraut release` run,
  unconditionally — same as release notes always did.

## Alternatives considered

- **Leave changelog frozen, only fix release notes' (already-correct) behavior.** This was the
  status quo being reported against; rejected per explicit user decision — the asymmetry itself was
  the complaint.
- **Keep preamble/postamble frozen by default, add an opt-in flag to force a refresh.** Rejected:
  adds a new flag for behavior release notes already has unconditionally: no such flag exists there
  and none was requested. Consistency between the two drivers was the goal.
