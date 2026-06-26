# ADR-0032: Built-in (Native) Content Generator

- **Status**: Accepted
- **Date**: 2026-06-26
- **Deciders**: bchatard

---

## Context

heraut produces changelogs and release notes through the `port.Generator` interface
(`Check` / `Validate` / `Generate`). Two implementations exist today:

- **`git-cliff`** (default) — embeds two opinionated `cliff.toml` defaults
  ([ADR-0010](0010-embedded-cliff-toml-default.md)), deep-merges an optional user override,
  and shells out to the external `git-cliff` binary to do the actual work.
- **`communique`** — an opaque pass-through for teams that own their changelog tooling;
  heraut just runs `communique generate` and captures stdout.

[ADR-0028](0028-drop-cocogitto-generator.md) deliberately reduced the generator count from
three to two and set an explicit bar: *future generator additions must offer a capability
the existing options lack, to avoid reintroducing a redundant option.* This ADR is
evaluated against that bar (see "Relation to ADR-0028" below).

`git-cliff` performs seven distinct jobs for heraut:

1. walk git history for a tag range (`--tag`, `--latest`, `--tag-pattern`);
2. parse commits against the Conventional Commits grammar;
3. classify and group them (`commit_parsers`: regex→group, `skip` rules, ordering);
4. render Markdown via its Tera templates;
5. **enrich** commits with platform metadata — PR number, author handle, first-time
   contributors, linked-issue statistics — via its `[remote.*]` GitHub / GitLab / Azure
   DevOps API integration;
6. link-parse tickets (`link_parsers`, [ADR-0024](0024-ticket-linking.md));
7. postprocess version headings.

Over several phases heraut has progressively reclaimed logic *out* of the Tera templates
and into Go: [ADR-0021](0021-per-platform-release-notes.md) regenerates notes per platform;
[ADR-0022](0022-fat-injection-thin-templates.md) moved all per-platform link-shape
knowledge (GitLab `/-/` infix, PR `#` vs MR `!`, commit / compare URL composition) into Go
so the templates stay branch-free; [ADR-0026](0026-azure-devops-metadata-remote.md) added
Azure DevOps URL composition in the same Go layer. heraut also already owns a
conventional-commit parser (`internal/conventionalcommit`,
[ADR-0027](0027-builtin-conventional-commit-checker.md)) and already calls `gh api` /
`glab api` through `port.Runner` for its platform auth checks. In other words, heraut
already does — or has the pieces to do — jobs 1–4, 6, and 7; the embedded Tera templates
are now thin wrappers over data heraut largely assembles itself.

The remaining cost of routing generation through the external `git-cliff` binary:

- **Bundling / installation** — git-cliff must be on `PATH` or baked into the Docker image
  ([ADR-0016](0016-bundled-docker-image.md)).
- **Output drift** — the rendered Markdown is pinned to whichever git-cliff version is
  installed; heraut does not control it across upgrades.
- **Offline ergonomics** — even a trivial changelog with no remote enrichment needs the
  binary present.
- **Indirection** — every customization is expressed as TOML / Tera that heraut merges,
  injects into, and post-processes (`injectRemote`, `injectLinkParsers`,
  `injectHeadingPostprocessor`), a layer that exists only because the renderer is external.

Only job 5 (remote enrichment) is something heraut has *no* native substitute for today.

## Decision

Add a third `port.Generator`, selected by `generator: native`, as an **opt-in,
zero-external-dependency** content generator implemented entirely in Go. It assembles the
commit view-model itself (jobs 1–3, 6, 7), renders with `text/template` consuming the
existing `port.LinkContext` directly (job 4), and — in a later phase — performs remote
enrichment (job 5) by calling `gh api` / `glab api` through the existing `port.Runner`.

`native` is **additive, not a replacement**:

- `git-cliff` remains the **default** and the power-user escape hatch. Its full TOML / Tera
  surface — `split_commits`, `commit_preprocessors`, custom Tera, monorepo features — is
  explicitly **out of scope** for `native`. Users who need that keep `generator: git-cliff`.
- `communique` is unaffected.

### Phased delivery

**Phase 1 — native renderer, no remote enrichment.** Walk the tag range, parse + classify +
group commits, render both variants (changelog + release-notes) with Go templates, consume
`LinkContext` for commit / compare links. Output targets **parity with `git-cliff
--offline`** (no PR author / number, no contributors block), verified by golden-file tests.
Ships `generator: native` as a selectable, documented option. This is the low-risk bulk of
the work and is independently useful (single-binary offline changelogs).

**Phase 2 — remote enrichment via platform CLIs.** Add PR-number / author / first-time-
contributor / linked-issue enrichment by calling `gh api` / `glab api` through
`port.Runner`, gated by the existing `remote_metadata` policy
([ADR-0023](0023-remote-metadata-policy.md)) and surfaced through the existing `Degraded()`
signal. Contract-tested with `MockRunner` — no real network. GitHub first; GitLab and Azure
DevOps ([ADR-0026](0026-azure-devops-metadata-remote.md)) follow.

**Phase 3 — raw-HTTP clients (deferred, separate decision).** Replacing `gh` / `glab` with
direct `net/http` platform clients — which would let heraut drop those binaries entirely —
is explicitly **not** decided here. It reimplements asset upload, pagination, and rate-limit
handling that the CLIs currently absorb, and it shifts ongoing API-churn maintenance onto
heraut. It requires its own ADR before any work starts.

### Config model

`native` ships a **fixed, embedded default taxonomy** (the same conventional-commit groups
and ordering as the git-cliff defaults), mirroring ADR-0010's "opinionated default in the
binary" stance. Because Go templates cannot be deep-merged the way TOML tables can, the
override surface is **structured knobs + whole-template replacement**, not TOML-style
partial merge:

- the existing top-level `tickets:` ([ADR-0024](0024-ticket-linking.md)) and
  `remote_metadata:` apply unchanged;
- an optional `template:` path replaces the body template wholesale (no merge);
- group taxonomy / skip customization, if offered, is a small explicit list — not the full
  git-cliff `commit_parsers` grammar.

`schema.json`, `docs/heraut.sample.yml`, and `docs/specs/05-generators-and-platforms.md`
gain `native` in the `generator` enum and document its (smaller) surface.

### Parity and CHANGELOG churn

heraut commits `CHANGELOG.md`. Switching an existing project from `git-cliff` to `native`
must not produce a large spurious diff from whitespace differences between Tera and Go
templates. Mitigations:

- `native` is **opt-in** and **not** the default — no project changes generator without an
  explicit edit.
- Phase 1 is gated on **golden-file parity tests** against captured `git-cliff --offline`
  output for a representative repo, so drift is caught in CI, not in a user's changelog.
- The real-CLI smoke test that today asserts git-cliff accepts the embedded config
  (`testing.md`) stays for `git-cliff`; `native` is covered by golden + table tests instead.

### Relation to ADR-0028

ADR-0028 dropped `cocogitto` because it "[did] a subset of what git-cliff does, with no
capability unique to it," and asked that future generators clear that bar. `native` clears
it: its unique capability is **zero external dependency** — single-binary,
offline-by-default changelog / release-notes generation with output heraut fully controls
and version-pins itself. That is precisely the axis on which both `git-cliff` (needs the
binary, version-drifting output) and `communique` (needs the binary, opaque) do *not*
compete. Going from two generators back to three is therefore not a redundancy regression;
it adds an option differentiated on dependency footprint, not on feature overlap.

## Consequences

- heraut gains a generator that needs **no external binary** for jobs 1–4 / 6 / 7; a project
  using `generator: native` without remote enrichment has no git-cliff dependency at all.
- The dependency win is **partial until Phase 3**: `gh` / `glab` remain required for
  `release create` and (Phase 2) enrichment, so the Docker image (ADR-0016) cannot drop
  those until/unless Phase 3 happens. Phase 1 lets the image drop only `git-cliff`, and only
  for `native`-configured projects.
- heraut **takes on maintenance** it previously delegated: rendering fidelity and (Phase 2)
  the platform API surfaces for enrichment — concentrated exactly where third-party APIs
  change most. This is the explicit trade for control and dependency reduction.
- The generator count returns to **three**. Per ADR-0028's standing request, this is
  recorded as a deliberate, differentiated addition (see "Relation to ADR-0028"), not a
  reopening of the redundant-third-option door.
- `git-cliff`'s full TOML / Tera customization remains available and **default**; no
  existing configuration changes behavior. `native` is a new, smaller-surfaced option users
  opt into.
- Work is tracked in a dedicated roadmap, `docs/tasks/native-generator-roadmap.md` (T122+),
  linked as Phase 23 from the main `docs/tasks/roadmap.md`; Phase 3 is explicitly fenced
  behind a future ADR.
