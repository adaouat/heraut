# Spec 06 — DX and Testing

## DX requirements

### Schema validation

`schema.json` lives at the repo root and is published unchanged on every release. Users
reference it from `.heraut.yml` with:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
```

This gives IDE autocomplete and inline error highlighting in VS Code, IntelliJ, Helix,
and any editor with YAML Language Server support.

### `heraut check config`

Explicit offline validation, usable in CI before any release job runs and locally by
developers before committing config changes. No tokens, no network, no git operations
— just `config.Load` + `config.Validate`. See [Spec 03 — Commands § heraut check](03-commands.md#heraut-check).

### Dry-run

Every command supports `--dry-run`. The implementation:

1. The `exec.Runner` adapter checks its `DryRun` flag before invoking each command.
2. When set, it logs the would-be invocation (with all args, env vars, working dir)
   and returns immediately with empty stdout / nil error.
3. Production code paths get the same execution shape with no side effects.

In dry-run mode the UI uses plain `Info` / `Success` lines instead of spinner
animations, so the `[dry-run]` output never overwrites itself.

### Actionable errors

Config validation errors carry `Path`, `Message`, and `Hint`:

```
✗ versioning.environments.prod.source: cycle detected (prod → staging → prod)
  hint: each promotion source must trace back to an auto env without revisiting envs
```

Runtime errors include the env var name, expected binary, or git command that failed:

```
✗ check runtime: gh not found in PATH
  hint: install with `brew install gh` or download from https://cli.github.com/

✗ check runtime: GITHUB_REPOSITORY not set
  hint: heraut needs the repo as <owner>/<repo>; set it explicitly or run in a GitHub Actions context
```

The principle: never just "invalid config" — every error explains the *what* and
suggests the *fix*.

### Documentation

- This `docs/specs/` directory — the behavioural authority for users
- `docs/adr/` — design decisions for contributors
- `README.md` — install, quickstart, configuration crash-course (with links into
  `docs/specs/`)

Each strategy, generator, and platform has a dedicated section in one of the specs.

## Testing strategy

Four layers, with strict discipline:

### Unit

`go test` against pure functions. Targets:

- `internal/versioning/{tagfmt,semver,calver,perenv}/`
- `internal/config/` (loader, path, validator)
- `internal/generators/gitcliff/merge.go`

Every exported function has tests. Table-driven where the input space is enumerable.

### Contract

`internal/testutil.MockRunner` records every `Run` / `RunEnv` call into `[]Call` and
returns ordered `[]Response`. Used for every CLI invocation: `git`, `git-cliff`, `gh`,
`glab`, `cog`, `communique`.

A platform driver does not ship without contract tests. The tests assert the **exact
CLI arguments** passed — flag names, value formats, order, env vars. Example shape:

```go
mr := testutil.NewMockRunner()
mr.QueueResponse("", "", nil)
plat := github.New(mr, github.Config{Repository: "acme/widget", TokenEnv: "GH_TOKEN"})

require.NoError(t, plat.CreateRelease("v1.2.3", "release notes body"))

require.Len(t, mr.Calls, 1)
assert.Equal(t, "gh", mr.Calls[0].Name)
assert.Equal(t, []string{
    "release", "create", "v1.2.3",
    "--notes", "release notes body",
    "--repo", "acme/widget",
}, mr.Calls[0].Args)
```

### Integration

`internal/testutil.FakeBin(t, name, script)` installs a shell script as a fake binary in
`PATH` for the test. Used when the test needs the production `exec.Runner` path —
verifying env-var propagation, exit-code mapping, stdin/stdout forwarding.

Integration tests target a real local git repo (created with `git init` in `t.TempDir()`).
The test sequence is: set up repo → create commits → run `heraut release --dry-run`
through the binary → assert on the printed action plan or on the resulting tags.

### Schema

JSON Schema fixtures live in the repo-root `testdata/config/`:

- `testdata/config/valid/<strategy>.yml` — one happy config per strategy
- `testdata/config/invalid/<reason>.yml` — one config per validation failure, each
  paired with the expected error message

`heraut check config` is tested against these fixtures both via the validator directly
and via the binary (golden output comparison).

## Determinism

- **No real time.** Calver resolver takes a `now func() time.Time` so tests can fix the
  clock. Self-update tests use `httptest.Server` instead of the real GitHub API.
- **No real network.** Platform tests use `MockRunner`; `gh`/`glab` are never invoked.
- **No filesystem outside `t.TempDir()`.** Embedded TOML / Tera content is accessed
  through production functions, not by reading the source tree.
- **No environment leakage.** Tests that depend on env vars set them with
  `t.Setenv(...)`.

## Hard-won edge cases

The test suite covers several edge cases that are easy to regress on without explicit
coverage:

- `v1.9.0` → `v1.10.0` (not `v1.100.0`) — SemVer ordering, not lexicographic
- CalVer `PATCH` reset on calendar period boundary
- Per-env cycle detection in `source:` chains
- E001 / E002 / E003 `--force` bypass semantics (E003 is not bypassable)
- The four cocogitto config × template combinations
- The git-cliff temp config file lifecycle (cleanup on early return)

These cases are kept in the test suite indefinitely. A test row is removed only when the
behaviour it covers is deliberately changed, with an ADR documenting the change.

## CI

`.github/workflows/ci.yml` runs on every PR:

1. `go build ./...`
2. `go test ./...`
3. `golangci-lint run`

A failing job blocks merge. Branch protection on `main` requires the CI workflow to
pass. Linters are configured in `.golangci.yml`.

`.github/workflows/release.yml` triggers on `v*` tags and `workflow_dispatch`. It runs
`goreleaser release --clean`, which builds the cross-platform binaries, creates the
GitHub Release, uploads `checksums.txt`, and pushes the `ghcr.io/adaouat/heraut` image.

## Resolved questions

Foundational design questions and their resolutions:

| Question                                                                                       | Resolution                                                                |
|------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------|
| Distribution model                                                                             | Raw binary via GitHub Releases + GHCR image. See [ADR-0013](../adr/0013-raw-binary-goreleaser-format.md). |
| Config format                                                                                  | YAML. See [ADR-0004](../adr/0004-config-format-yaml.md).                  |
| Should `bump: promote` hard-fail when the destination is at or ahead of the source candidate?  | Yes, on all edge cases, with rich errors + `--force` escape hatch (E001/E002/E003). See [ADR-0007](../adr/0007-version-promotion-error-handling.md). |
| Should the release flow commit `CHANGELOG.md`, or leave the commit to the caller?              | heraut owns the commit. Order: generate → commit → tag → publish. See [ADR-0012](../adr/0012-changelog-commit-ownership.md). |
| Tool name                                                                                      | Héraut (brand) / `heraut` (binary). See [ADR-0002](../adr/0002-tool-name-heraut.md). |
| `source:` field for promote with multiple `auto` envs                                          | Explicit `source:` field, with cycle detection and chaining. See [ADR-0008](../adr/0008-promote-source-env.md). |
| Module path                                                                                    | `github.com/adaouat/heraut`                                               |
| Self-update version check                                                                      | GitHub Releases API directly. See [ADR-0014](../adr/0014-self-update-architecture.md). |
| Per-env strategy code shape                                                                    | Generic `internal/versioning/perenv/` wrapping a `VersionCalculator` interface. See [ADR-0009](../adr/0009-generic-perenv-resolver.md). |
