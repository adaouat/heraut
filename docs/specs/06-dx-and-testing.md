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
2. When set, it logs the would-be invocation and returns immediately with empty stdout /
   nil error — no git writes, no network calls, no file writes outside `/tmp`.
3. **Exception — version resolution**: the resolver always uses a real (non-dry-run)
   runner for its read-only git calls (`git tag -l`, `git log`), so `--dry-run` output
   shows the correct resolved version rather than falling back to `initial_version`.
4. Production code paths get the same execution shape with no other side effects.

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

Every exported function has tests. Table-driven where the input space is enumerable.

### Contract

`github.com/adaouat/forge/exec/exectest.MockRunner` (heraut's shared plumbing dependency —
not `internal/testutil`, which holds only heraut-specific fixtures: `MockGenerator`,
`MockPlatform`, `RealGitRepo`, and CI-env-clearing helpers) records every `Run` / `RunEnv`
call into `[]Call` and returns ordered `[]Response`. Used for every CLI invocation: `git`,
`gh`, `glab`.

A platform driver does not ship without contract tests. The tests assert the **exact
CLI arguments** passed — flag names, value formats, order, env vars. Example shape:

```go
mr := exectest.NewMockRunner()
mr.QueueResponse("", "", nil)
plat := github.New(mr, &config.Platform{Repository: "acme/widget", TokenEnv: "GH_TOKEN"})

require.NoError(t, plat.CreateRelease("v1.2.3", "release notes body"))

require.Len(t, mr.Calls, 1)
assert.Equal(t, "gh", mr.Calls[0].Name)
assert.Equal(t, "release", mr.Calls[0].Args[0])
assert.Contains(t, mr.Calls[0].Args, "--notes-file") // notes go to a temp file, never passed inline
assert.Contains(t, mr.Calls[0].Args, "acme/widget")
```

The `internal/forge/{github,gitlab,azure}` HTTP enrichment clients (ADR-0043) are contract-
tested differently, against a real `httptest.Server` rather than `MockRunner` — there is no
CLI invocation to assert arguments on for a direct HTTP call.

### Integration

`github.com/adaouat/forge/exec/exectest.FakeBin(t, name, script)` installs a shell script as
a fake binary in `PATH` for the test. Used when the test needs the production `exec.Runner`
path — verifying env-var propagation, exit-code mapping, stdin/stdout forwarding.

Integration tests target a real local git repo (`internal/testutil.RealGitRepo`, `git init`
in `t.TempDir()`) exercised through cobra commands in-process (`internal/cmd/*_test.go`) or
through the native generator's own integration test
(`internal/generators/native/integration_test.go`), which drives real git history through
`forge/exec`. No test builds or executes the `heraut` binary itself today — that would be a
genuinely stronger guarantee for the full CLI wiring, but it isn't part of the current suite.

### Schema

JSON Schema fixtures live in the repo-root `testdata/config/`:

- `testdata/config/valid/<strategy>.yml` — one happy config per strategy
- `testdata/config/invalid/<reason>.yml` — one config per validation failure, each
  paired with the expected error message

`heraut check config` is tested against these fixtures via the validator/schema directly
(`internal/config/schema_test.go`). Golden-output comparison is a separate, unrelated
mechanism used for native's rendered changelog/release-notes output
(`internal/generators/native/render_internal_test.go`), not for config-check output.

## Determinism

- **No real time.** Calver resolver takes a `now func() time.Time` so tests can fix the
  clock.
- **No real network.** Platform driver tests use `MockRunner`; `gh`/`glab` are never
  invoked. The `internal/forge/*` HTTP enrichment clients use a real `httptest.Server`
  instead (§ Contract above) — heraut has no self-update mechanism to test (superseded by
  forge ADR-0005; update checking is `forge/updatecheck`'s responsibility).
- **No filesystem outside `t.TempDir()`.** Embedded Go `text/template` content
  (`internal/generators/native/*.tmpl`) is accessed through production functions, not by
  reading the source tree.
- **No environment leakage.** Tests that depend on env vars set them with
  `t.Setenv(...)`.

## Hard-won edge cases

The test suite covers several edge cases that are easy to regress on without explicit
coverage:

- `v1.9.0` → `v1.10.0` (not `v1.100.0`) — SemVer ordering, not lexicographic
- CalVer `PATCH` reset on calendar period boundary
- Per-env cycle detection in `source:` chains
- E001 / E002 / E003 `--force` bypass semantics (E003 is not bypassable)

These cases are kept in the test suite indefinitely. A test row is removed only when the
behaviour it covers is deliberately changed, with an ADR documenting the change.

## CI

`.github/workflows/ci.yml` runs three jobs on every PR:

1. **`ci`** — delegates to forge's shared reusable workflow
   (`adaouat/forge/.github/workflows/go-ci.yml`), which runs `go test ./...` and
   `golangci-lint run` with an 85% coverage threshold enforced.
2. **`build`** — `go build ./cmd/heraut/` plus `goreleaser check` (validates
   `.goreleaser.yml` without building).
3. **`hk`** — `hk check --all --check --skip-step golangci_lint`: every other linter
   configured in `.config/hk/config.pkl` (hadolint, actionlint, yamlfmt, typos, pkl, go
   fmt, …) against the full tree, not just staged files. `golangci_lint` is skipped here
   since the `ci` job already ran it.

A failing job blocks merge. Branch protection on `main` requires all three. Go linters are
configured in `.golangci.yml`; everything else in `.config/hk/config.pkl`.

`.github/workflows/release.yml` triggers only on `workflow_dispatch` — there is no `v*`
tag push trigger; cutting a release is a manually-invoked action, not a reaction to a
tag someone already pushed. GoReleaser is **build-only** here (`release: disable: true`
in `.goreleaser.yml`, [ADR-0018](../adr/0018-ci-build-then-release-pipeline.md)): it
cross-compiles the binaries and nothing else. The freshly-built `heraut` binary then runs
`heraut release --version <version>` **against itself** — this is what actually creates
the git tag and the GitHub Release (dogfooding), uploads the checksums/binaries as
release assets, and attaches build-provenance attestation. Separate `docker-build` /
`docker-merge` jobs build and push the multi-arch `ghcr.io/adaouat/heraut` image.

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
| Update availability check                                                                      | GitHub Releases API via forge's `updatecheck`; daily hint, no binary self-replacement. See [ADR-0014](../adr/0014-self-update-architecture.md) (superseded). |
| Per-env strategy code shape                                                                    | Generic `internal/versioning/perenv/` wrapping a `VersionCalculator` interface. See [ADR-0009](../adr/0009-generic-perenv-resolver.md). |
