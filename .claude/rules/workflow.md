# Workflow rules

## Branching

Every working session starts on a new branch off `main`, and its changes reach `main`
through a pull request. Never commit directly to `main`. (Before v1.0.1, heraut's first
1.x release, commits went straight to `main`; that build-phase rule no longer applies.)

- Branch name: `<type>/<short-description>` where type matches the
  conventional-commit type (e.g. `feat/perenv-resolver`, `fix/calver-patch-reset`,
  `docs/spec-reconciliation`).
- Fetch with prune before branching:
  ```bash
  git fetch --prune --prune-tags --all --tags
  git checkout -b <type>/<short-description> origin/main
  ```

## Conventional commits

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/). Allowed types:

| Type       | Use for                                                        |
|------------|----------------------------------------------------------------|
| `feat`     | New user-visible behavior                                      |
| `fix`      | Bug fix in existing behavior                                   |
| `docs`     | `docs/specs/`, `docs/adr/`, README, in-code doc comments       |
| `chore`    | Tool config, repo housekeeping, dependency bumps               |
| `refactor` | Code change with no behavior change                            |
| `test`     | Adding or rewriting tests, no production change                |
| `style`    | Formatting, whitespace, lint-only fixes                        |
| `perf`     | Performance-only change                                        |
| `ci`       | `.github/workflows/*`, GoReleaser, Docker, release tooling     |
| `build`    | `go.mod`, build system, ldflags                                |
| `security` | A security fix or hardening (built-in since ADR-0075)          |
| `revert`   | Undoing an earlier commit (built-in, see ADR-0075's addendum)  |

**Scope** matches the affected package or subcommand. Examples:

- `feat(versioning/perenv): add cycle detection in source chains`
- `fix(generators/native): drop trailing whitespace from rendered headings`
- `feat(cmd): wire heraut release end-to-end`
- `docs(adr): add 0009 generic perenv design`
- `test(platforms/github): contract test for asset upload`

Keep subject lines ≤ 72 characters. Use the body for the *why*, not the *what*.

## Two-step roadmap flow

Task status is tracked inline in `docs/tasks/roadmap.md` via checkbox markers next to
each task heading:

| Marker | Meaning     |
|--------|-------------|
| `[ ]`  | Not started |
| `[x]`  | Done        |

For each task:

1. **Implement** — confirm the task is `[ ]` in `docs/tasks/roadmap.md`, then do the
   work (TDD: failing test first, then implementation). Commit in logical pieces using
   the appropriate conventional-commit type.
2. **Complete** — flip `[ ]` → `[x]`. Add a one-paragraph note under the task in
   `docs/tasks/roadmap.md` describing actual decisions made, deferred items, or
   deviations. Commit the roadmap update alongside the final implementation commit, or
   as a separate `docs(roadmap):` commit if the work is already pushed.

Never silently mark a task complete without the roadmap note. The note is what makes the
roadmap a living document.

## User docs

heraut has two kinds of documentation, for two readers:

- **User docs**, for people who use heraut: `README.md` → `docs/README.md`, then `docs/install.md`,
  `docs/getting-started.md`, `docs/guides/`, `docs/concepts/`, `docs/reference/` and
  `docs/heraut.sample.yml`.
- **Contributor docs**, for people and agents who change heraut: `CONTRIBUTING.md` and `CLAUDE.md`,
  then `docs/specs/` (the behavioural authority), `docs/adr/` and `docs/tasks/`.

A change to behaviour, a flag, a config key or a message a user sees updates both in the same commit
series:

1. **The spec first**, as today.
2. **Every user page that shows it.** Find them with
   `grep -rn '<flag or key>' README.md docs/*.md docs/guides docs/concepts docs/heraut.sample.yml`,
   and fix the text, the examples and any quoted output.
3. **The generated reference**: run `mise run docs:gen`. It rewrites `docs/reference/commands/` from
   the commands' help text and the table in `docs/reference/configuration.md` from `schema.json`; a
   test fails while either is stale.

Rules for user pages:

- **Never link to `docs/specs/` or `docs/adr/`**, and never cite an ADR or a task id. Put what the
  reader needs on the page, or link the user reference.
- **Run every example against the built binary** and quote its real output. Use synthetic names
  (`acme/app`, `gitlab.example.com`, `ada@example.com`), never real projects or people.
- **Write for someone who doesn't know heraut's internals**: no package names, no `native`.

`TestDocumentationLinks` (`docs_links_test.go`) fails on a relative link or anchor that doesn't
resolve, anywhere in the living docs, and on a user page linking the specs or ADRs. It runs in
`go test ./...` and as hk's `docs_links` step when Markdown is staged. `docs/tasks/archive/` and
`docs/superpowers/` are historical snapshots and are not checked.

## Git hooks (hk)

Hooks live in `.config/hk/config.pkl`. They run on every commit:

- **pre-commit**: linters against staged files
- **commit-msg** (`heraut-commit-lint` step): validates conventional commit format via
  `heraut commit verify` itself — dogfooding, not `cog` (removed, ADR-0028; see
  ADR-0029/ADR-0030 for the builtin checker this replaced it with)
- **prepare-commit-msg**: `typos` on the commit message

**Never** pass `--no-verify`, `--no-gpg-sign`, or any flag that bypasses hooks. If a hook
fails, fix the underlying issue. Bypassing the hook defeats its purpose and the next
contributor inherits the broken state.

## Lint fixes

When a hook reports a lint failure, fix it through `hk`:

```bash
hk fix             # fix everything fixable
hk fix -S <linter> # target one linter (e.g. hk fix -S golangci_lint, hk fix -S yamlfmt)
```

Do **not** invoke the underlying tool (`gofmt`, `yamlfmt`, etc.) directly — `hk fix`
applies the project's configured file selection and flags from `.config/hk/config.pkl`.

## Plans

Plans live in `.claude/plans/` (see `.claude/settings.json` → `plansDirectory`). Each
plan captures one discrete unit of work — a phase, a milestone, a non-trivial task, or a
research / design spike.

**File naming:** descriptive, lowercase kebab-case, with a phase or task prefix where
applicable. Examples:

- `phase-d-docs-foundation.md` — phase-scoped plan
- `t07-semver-resolver.md` — single-task plan
- `perenv-design-spike.md` — research / design exploration

Do **not** keep the auto-generated random name (e.g. `playful-dancing-wozniak.md`).
Rename the file to its real subject before the first commit that references it.

## Pull requests

Each PR corresponds to one task (T-id) or one
tight cluster of related changes. A PR title is the conventional-commit subject of its
primary change. The description lists the touched packages and points to the roadmap
task, e.g.:

```
## Summary
- Implements T07: SemVer resolver with DetermineBump + BumpVersion
- Edge cases covered: prefix handling, v1.9.0 → v1.10.0, manual mode error path

Roadmap: docs/tasks/roadmap.md → T07
```

## Releases

Releases are cut by manually running the `Release` workflow (`workflow_dispatch` —
`.github/workflows/release.yml`), not by pushing a tag. GoReleaser is **build-only**
there (cross-compiles the binaries; `release: disable: true` in `.goreleaser.yml`) — the
freshly-built `heraut` binary then runs `heraut release` against itself to create the git
tag and the GitHub Release (dogfooding). See
`docs/adr/0013-raw-binary-goreleaser-format.md` and
`docs/adr/0018-ci-build-then-release-pipeline.md`.
