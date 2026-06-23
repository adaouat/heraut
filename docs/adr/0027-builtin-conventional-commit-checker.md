# ADR-0027: Built-in Conventional-Commit Checker

- **Status**: Accepted
- **Date**: 2026-06-22
- **Deciders**: bchatard

---

## Context

heraut's own `commit-msg` git hook (`.config/hk/config.pkl`) shells out to `cog verify` to
reject non-conventional commit messages before they land. This is the only place in
heraut's own dev tooling that depends on cocogitto for something other than the
`generator: cocogitto` feature it offers users.

Separately, `internal/versioning/semver/bump.go` already contains a second, narrower
conventional-commit parser: `isBreaking`/`isFeat`/`breakingPrefixPattern` exist purely to
classify commits for SemVer bump resolution (does this commit warrant a major or minor
bump). It does not validate grammar — it tolerates anything that isn't recognizably
`feat`/breaking and falls back to a patch bump.

Two independent, hand-rolled conventional-commit parsers in the same codebase is a
divergence risk: a grammar tweak in one (e.g. the breaking-change footer rule) has no
mechanism to stay in sync with the other.

We evaluated existing Go packages before building anything in-house, per this project's
general preference to avoid adding dependencies that don't meet its own stability bar:

| Package | Finding |
|---|---|
| `leodido/go-conventionalcommits` | Most established option (Conventional Commits 1.0 support, footer/trailer parsing), but last release Feb 2024, 88★, 5 open issues — stale. |
| `conventionalcommit/commitlint` | Published Feb 2026 — four months old at evaluation time, no track record. |
| `mkyc/go-conventional`, `gitlab.com/digitalxero/go-conventional-commit` | Niche, low-adoption, no meaningful maintenance signal. |

None clear the bar. Building a small internal grammar package is consistent with how
heraut already treats embedded/owned behavior (see [ADR-0010](0010-embedded-cliff-toml-default.md)'s
stance that embedded content is user-facing and worth owning directly).

`cog verify` itself (the feature being replaced) validates a single message string —
type, optional `(scope)`, optional `!`, `: `, description — via positional arg, `--file
<path>`, or `--file -` (stdin), and returns a structured parse error on failure. `cog
check` (whole-history validation) is a distinct, broader feature and is **not** in scope
here — it may become its own ADR/task later if needed.

## Decision

### `internal/conventionalcommit/` — new domain package

A pure, dependency-free package, peer to `internal/port`/`internal/config` rather than
nested under `internal/versioning/` — conventional-commit grammar is not a versioning
concept (CalVer never consults it), so tying it to the versioning subtree would
misrepresent its scope.

```go
type Footer struct {
    Token string // e.g. "Acked-by", "BREAKING CHANGE", "BREAKING-CHANGE"
    Value string
}

type Commit struct {
    Type        string
    Scope       string
    Breaking    bool
    Description string
    Body        string
    Footers     []Footer
}

func Parse(message string) (*Commit, error)
func IsMergeCommit(message string) bool
func IsFixupCommit(message string) bool
```

`Parse` validates grammar only — it does not enforce a type allow-list (that is a
configurable policy decision, layered on top in `internal/app/`).

**Body and footers are parsed structurally, not pattern-matched.** Per the spec: the body
is free text after a blank line following the header; footers follow another blank line,
each a `token: value` or `token #value` pair (git-trailer-style, `-` for spaces in
multi-word tokens), with `BREAKING CHANGE`/`BREAKING-CHANGE` as the one case-sensitive
exception. `Breaking` is set from the header's `!` **or** the presence of a
`BREAKING CHANGE`/`BREAKING-CHANGE` footer — derived from `Footers`, not from re-scanning
raw lines for a matching prefix. This replaces `bump.go`'s current approach (a regex line
scan that approximates "is this line a footer" via blank-line-before-it, without actually
parsing the footer block), closing a real gap: a body paragraph that merely starts with the
right text but isn't structurally a footer is now correctly excluded.

**Explicitly still out of scope**: this is body/footer *structure* needed to detect
breaking changes correctly, not the broader commitlint-style rule catalog (`body-leading-
blank`, `footer-max-line-length`, `signed-off-by`, `trailer-exists`, casing/length rules
across header/body/footer, etc.). That catalog is ~25 rules deep and is almost entirely
team-style preference, not Conventional Commits compliance — replicating it would mean
rebuilding commitlint inside heraut, which is exactly the scope this ADR's "build small,
own it" reasoning argues against. `Footers` is exposed on `Commit` so future work (the
`heraut commit check` / wizard ideas below) can read arbitrary trailers if needed, but no
validation rule consumes anything beyond `BREAKING CHANGE`/`BREAKING-CHANGE` today.

**Performance.** `Parse` sits on two hot paths: the `commit-msg` hook (every commit, every
contributor) and `DetermineBump` (called once per commit in the resolved range, which can
be the full history back to the last matching tag — hundreds to thousands of commits on an
older repo). Single-message latency is invisible to a human either way, but `DetermineBump`
turns "fast enough" into "scales with repo size," so the same discipline `bump.go`'s
existing regex-based scan already follows applies: package-level compiled `regexp`
patterns only (never compiled per call), anchored/bounded patterns only on the header line
(no nested-quantifier patterns that risk catastrophic backtracking), and a single linear
pass over the body/footer block — no re-scanning the message more than once to extract
`Body`/`Footers`. `Parse` must not allocate more per call than the header regex match plus
one slice append per discovered footer; it must not regress `DetermineBump`'s existing
O(total commits × message length) behavior into something worse. A Go benchmark
(`BenchmarkParse`, covering header-only, header+body, and header+multiple-footers inputs)
is added alongside the unit tests to lock this in and catch future regressions, rather than
relying on inspection alone.

**`internal/versioning/semver/bump.go` is refactored to use this package.**
`DetermineBump` calls `conventionalcommit.Parse` and inspects `.Type`/`.Breaking` instead
of its own `isBreaking`/`isFeat`/`breakingPrefixPattern`. Parse errors are treated the same
as today's "not feat, not breaking" case (defaults to patch) — bump resolution stays
lenient about non-conventional commits; only the new checker enforces strict validation.
This removes the second parser rather than leaving two sources of truth for the same
grammar.

This requires one addition to the layer table in
[`coding.md`](../../.claude/rules/coding.md): `internal/versioning/*` and `internal/app/`
gain `conventionalcommit` as an allowed import.

### Config — `commit_lint:` (optional, default-on)

```yaml
commit_lint:
  types: [feat, fix, docs, chore, refactor, test, style, perf, ci, build]
```

Works with zero config: the default type list is the same 10 types already documented in
[`workflow.md`](../../.claude/rules/workflow.md)'s commit-type table. The block exists
only to override/restrict that list — `commit_lint.types`, if present, replaces rather
than extends the default.

Out of scope deliberately (YAGNI — no demonstrated need yet): scope allow-lists,
subject-casing rules, length limits. These map to commitlint's `scope-enum`/`subject-case`
rules in the wider ecosystem, but they're still grammar/style concerns, not "semantic"
validation in any deeper sense — no mainstream tool (cog, commitlint, or otherwise)
validates whether a commit's *type* actually matches its *diff*; that would require
diff/code analysis or LLM judgment and isn't standard practice. We are not pursuing it.

Merge commits (`Merge branch ...`, `Merge pull request ...`) and fixup commits
(`fixup!`/`squash!` prefix) are skipped automatically and unconditionally — not
configurable for now, matching git's own conventions for these message shapes.

### Command — `heraut commit verify`

```
heraut commit verify [message] [--file <path>]
```

Mirrors `cog verify`'s three input modes (positional, `--file <path>`, `--file -` for
stdin). `internal/cmd/commit.go` follows the existing parent/subcommand pattern
(`internal/cmd/version.go`), calling into `internal/app` (never the domain package
directly, per the layer rules) for the actual `Parse` + allow-list check. An invalid
message exits with `exitcode.Usage` — this is bad input, not a config or runtime failure,
so no new exit code is introduced.

### Dogfooding — heraut drops its own dependency on `cog` for linting

- `.config/hk/config.pkl`'s `commit-msg` hook step switches from
  `cog --config .config/cocogitto/config.toml verify --file {{ commit_msg_file }}` to
  `go run ./cmd/heraut commit verify --file {{ commit_msg_file }}`.
- `.config/cocogitto/config.toml` and the `cog` entry in `.config/mise/config.toml`'s
  `[shell_alias]` table are deleted — both existed solely to support this hook.
- `cocogitto` stays in `.config/mise/config.toml`'s `[tools]` list at this point — it is
  still required by the `generator: cocogitto` feature's contract/smoke tests. Its removal
  is a separate decision; see [ADR-0028](0028-drop-cocogitto-generator.md).

## Consequences

- A second, divergent conventional-commit parser is eliminated; bump resolution and commit
  validation share one grammar implementation.
- `schema.json`, `docs/heraut.sample.yml`, and `docs/specs/02-configuration.md` need the
  standard config-field checklist treatment ([`coding.md`](../../.claude/rules/coding.md))
  for the new `commit_lint` block.
- `docs/specs/03-commands.md` documents the new `heraut commit verify` command.
- heraut's own commit-msg hook no longer depends on `cog` being installed/configured for
  contributors — only on the heraut source tree itself (`go run`).
- True semantic (diff-aware) commit validation is explicitly not pursued; this ADR is the
  record of that evaluation should the question resurface.

## Related future work (not yet scoped)

Two ideas surfaced while designing this that are deliberately **not** part of T116 — each
would need its own brainstorming session and roadmap task before implementation:

- **`heraut commit check <rev-range>`** — the `cog check` equivalent: validate an entire
  commit range/history (e.g. all commits on a PR branch) rather than a single message, for
  use in CI. `heraut commit verify` (this ADR) only covers the single-message case (the
  commit-msg hook use case); range checking is a distinct command with its own design
  questions (how to enumerate the range, how to report multiple failures, merge-commit
  handling across a range vs. a single message).
- **Interactive commit wizard** (e.g. `heraut commit create`) — guided prompts (type,
  scope, breaking, description, body) that construct a conventional commit message and run
  `git commit` with it, in the spirit of [meteor](https://github.com/stefanlogue/meteor).
  Feasible with heraut's existing `huh` dependency (already used by `heraut init`'s
  wizard) and would naturally reuse this ADR's `conventionalcommit` package and
  `commit_lint` config (types) so wizard-built commits are guaranteed to pass
  `heraut commit verify`. Not designed here; revisit as its own task when prioritized.
