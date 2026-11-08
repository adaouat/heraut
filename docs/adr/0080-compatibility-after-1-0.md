# ADR-0080: Compatibility after 1.0 — SemVer for the CLI, with a deprecation window

- **Status**: Accepted
- **Date**: 2026-11-08
- **Deciders**: bchatard

---

## Context

Before 1.0, heraut changed its configuration and CLI freely: ADR-0040 ("Breaking, no shim
(pre-v1.0)"), ADR-0046 ("a hard cutover lands directly on `main`, no deprecation window") and the
roadmap's renames ("Breaking rename, no alias — pre-v1.0") all relied on there being no promise yet.
Nothing said what changes once 1.0 ships.

heraut runs in CI pipelines and is driven by scripts and AI agents. Their authors need to know
which upgrades are safe to take without reading the changelog, and how much warning they get before
something they use goes away.

### Options

1. **SemVer for the CLI, with a deprecation window.** A defined stable surface breaks only in a
   major release; what is deprecated keeps working and warns for at least one minor release first.
2. **Removal in a minor after a warning period.** Faster cleanup, but a minor upgrade can still
   break a pipeline.
3. **Best effort.** Breaking changes are only listed in the changelog. No pinning strategy works.

## Decision

Option 1. heraut's own version follows SemVer with respect to this **stable surface**:

- **Configuration**: every `.heraut.yml` key, its type and accepted values, defaults, and how keys
  combine (inheritance, overrides). A config that loads and validates in 1.x loads and validates
  with the same meaning in every later 1.y.
- **Command line**: commands, subcommands, flags, their arguments, and the environment variables
  heraut reads (`HERAUT_FILE`, token variables, CI variables).
- **Exit codes** and their meaning.
- **`--json` output**: the envelope (`ok`, `data`, `warnings`, `error`), the `error.code` values,
  and each command's `data` fields. New fields may be added; existing ones keep their name, type
  and meaning.
- **Template data**: the fields and functions available to `rendering.templates` and template
  files.
- **Tags and files heraut reads back**: the tag shapes it parses and the `<!-- heraut-release: … -->`
  anchors in a changelog it updates.

Changing any of these incompatibly is a **major** release. New keys, flags, commands, exit codes,
`error.code` values and `--json` fields are **minor** releases.

**Not** part of the stable surface, and free to change in a minor release:

- the wording of messages, warnings and help text (scripts read `--json` and exit codes, not
  prose);
- the rendered output of the built-in templates (headings, emoji, layout) — a project that needs a
  fixed shape pins it with its own templates;
- anything under `internal/`, which is not importable.

**Deprecation.** A key, value, flag or command that is to go away is first **deprecated** in a minor
release: it keeps working as before, and every run that uses it warns, naming the replacement
(`! versioning.prefix is deprecated: use versioning.tag_prefix; it will be removed in 2.0`).
`heraut check config` reports the same warning. It is removed no earlier than the next major
release, and at least one minor release after it was deprecated. The changelog of the release that
deprecates it and of the one that removes it both say so.

Security fixes are the exception: when keeping a behaviour would leave users exposed, it may change
in a minor or patch release, with the reason in the release notes.

## Consequences

- Every change to the stable surface needs a deliberate choice of release type; a reviewer checks it
  against this list. The pre-1.0 practice of "no shim" ends with 1.0.
- Decisions that would be breaking later are taken before 1.0 instead (Phase 95: `--force`, the
  `--json` failure shape, strict config keys, the "nothing to release" exit code).
- The loader needs a way to accept a deprecated key and warn; it is built with the first deprecation,
  not before (YAGNI).
- `.claude/rules/coding.md` § Config and CONTRIBUTING.md point here.
