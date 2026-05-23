# Workflow rules

## Branching

- Every working session starts on a new branch off `main`. Never commit directly to `main`.
- Branch name: `<type>/<short-description>` where type matches the conventional-commit type
  (e.g. `feat/perenv-resolver`, `fix/calver-patch-reset`, `docs/spec-reconciliation`).
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

**Scope** matches the affected package or subcommand. Examples:

- `feat(versioning/perenv): add cycle detection in source chains`
- `fix(generators/gitcliff): clean up temp config on early return`
- `feat(cmd): wire heraut release end-to-end`
- `docs(adr): add 0009 generic perenv design`
- `test(platforms/github): contract test for asset upload`

Keep subject lines ≤ 72 characters. Use the body for the *why*, not the *what*.

## Three-step roadmap flow

For each task in `docs/tasks/todo.md`:

1. **Start** — flip the checkbox from `[ ]` to `[~]` (in progress) and commit alone:
   ```
   chore(roadmap): start T07 — SemVer resolver
   ```
2. **Implement** — code, tests, docs. Commit in logical pieces using the appropriate type.
3. **Complete** — flip `[~]` → `[x]`. Add a one-paragraph note in `docs/tasks/roadmap.md`
   under the task describing actual decisions made, deferred items, or deviations.
   Commit the roadmap and todo changes together with the final implementation commit, or
   as a separate `docs(roadmap):` commit if the work is already pushed.

Never silently mark a task complete without the roadmap note. The note is what makes the
plan a living document.

## Git hooks (hk)

Hooks live in `.config/hk/config.pkl`. They run on every commit:

- **pre-commit**: linters against staged files
- **commit-msg**: validates conventional commit format via `cog`
- **prepare-commit-msg**: `typos` on the commit message

**Never** pass `--no-verify`, `--no-gpg-sign`, or any flag that bypasses hooks. If a hook
fails, fix the underlying issue. Bypassing the hook defeats its purpose and the next
contributor inherits the broken state.

## Lint fixes

When a hook reports a lint failure, fix it through `hk`:

```bash
hk fix             # fix everything fixable
hk fix -S <linter> # target one linter (e.g. hk fix -S golangci-lint, hk fix -S yamlfmt)
```

Do **not** invoke the underlying tool (`gofmt`, `yamlfmt`, etc.) directly — `hk fix`
applies the project's configured file selection and flags from `.config/hk/config.pkl`.

## Pull requests

PRs target `main`. Each PR should correspond to one task (T-id) or one tight cluster of
related changes. A PR title is the conventional-commit subject of its primary change. The
description lists the touched packages and points to the roadmap task, e.g.:

```
## Summary
- Implements T07: SemVer resolver with DetermineBump + BumpVersion
- Ports all edge-case tests verbatim from source (v1.9.0 → v1.10.0, prefix handling, …)

Roadmap: docs/tasks/todo.md → T07
```

## Releases

Releases are cut by pushing a `v*` tag. GoReleaser handles cross-platform builds and the
GitHub Release. Never push tags directly to remote without confirming the GoReleaser
config is current. See `docs/adr/0013-raw-binary-goreleaser-format.md`.
