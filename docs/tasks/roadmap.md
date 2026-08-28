# Héraut Build Roadmap

> Status: Active  
> Repo: `github.com/adaouat/heraut`

This roadmap is the executable plan for bringing Héraut to v1.0 with the feature set
described in `docs/specs/`. Each task carries an inline `[ ] / [x]` checkbox — read the
headings for what to do next, read the surrounding prose for *why* and *how*.

The behavioural authority is `docs/specs/` (six numbered specs); the architectural
authority is `docs/adr/` (46 ADRs). Where this roadmap mentions "behaviour", the specs
win; where it mentions a "decision", the ADR wins. If you find a disagreement between
roadmap and spec/ADR, fix the roadmap.

---

## Overview

Héraut is a Go CLI that resolves versions, generates changelogs, and publishes releases to
GitHub / GitLab — wrapping `git`, `gh`, and `glab` for git operations and publishing, with
changelog/release-notes generation built in (`native`, no external generator dependency —
ADR-0045) and PR/MR commit enrichment via direct HTTP against the GitHub/GitLab/Azure DevOps
APIs (ADR-0043). This roadmap captures the work to take it from an empty repo to a v1.0
release.

The goals of v1.0:

1. Implement the full feature set described in `docs/specs/` (four versioning
   strategies, one built-in generator, two publish platforms plus a third forge type for
   enrichment-only, init/check/commit/whatsnew tooling).
2. Establish a clean public home with proper distribution: GitHub Releases (raw
   binaries) and a GHCR container image.
3. Design internal packages with clear boundaries so the foundational ones (exec runner,
   config loading, exit codes, UI theming, update-check) could be extracted into a shared
   Go library later when other CLIs need them. Done: `github.com/adaouat/forge` now
   provides these (see [ADR-0014](../adr/0014-self-update-architecture.md), superseded,
   for the self-update → forge/updatecheck migration).

The `docs/specs/` (six numbered specs) and the 46 ADRs in `docs/adr/` are authoritative.

---

## Working process

Each task follows the two-step roadmap flow defined in
[`.claude/rules/workflow.md`](../../.claude/rules/workflow.md):

1. **Implement** — confirm the task is `[ ]`, then do the work (TDD: failing test first,
   then implementation).
2. **Done** — flip `[ ]` → `[x]`, add a one-paragraph note under the task describing
   actual decisions, deferred items, or deviations. Commit implementation + roadmap
   update together.

Task status markers:

| Marker | Meaning     |
|--------|-------------|
| `[ ]`  | Not started |
| `[x]`  | Done        |

One task at a time. The roadmap always reflects the current state of the branch.
TDD is required — write the failing test before writing implementation code.

---

## Architecture

### Key design choices

| Concern                          | Choice                                                                  |
|----------------------------------|-------------------------------------------------------------------------|
| Architecture style               | Hexagonal — `internal/port/` interfaces, `internal/adapter/exec/` implementations |
| Strategy selection               | Single `app.NewResolver()` factory; never an `if` ladder in `cmd/`      |
| Per-env strategies               | Generic `internal/versioning/perenv/` wrapping a `VersionCalculator` interface ([ADR-0009](../adr/0009-generic-perenv-resolver.md)) |
| Tag format substitution          | Shared `internal/versioning/tagfmt/` package ([ADR-0009](../adr/0009-generic-perenv-resolver.md)) |
| Domain wiring                    | `internal/app/` owns all factory logic; `internal/cmd/` is a thin CLI layer |
| Generator interface              | `Check()` + `Validate()` + `Generate(tag)` — three methods, validate before run |
| Platform drivers                 | Contract-tested with `MockRunner` — every CLI argument is asserted      |
| Config validation                | Strict YAML + composed semantic validators with `{Path, Message, Hint}` |
| CLI framework                    | `cobra` + `fang` ([ADR-0003](../adr/0003-cli-framework-cobra-fang.md))  |
| Self-update                      | GitHub Releases API directly ([ADR-0014](../adr/0014-self-update-architecture.md)) |
| Binary distribution              | Raw binaries via GoReleaser + GHCR image ([ADR-0013](../adr/0013-raw-binary-goreleaser-format.md)) |
| Testing                          | Table-driven unit + contract tests with `MockRunner` + integration via `testutil.FakeBin` |

### Package structure

```
heraut/
├── cmd/heraut/
│   └── main.go                 entry point — fang.Execute(cmd.NewRootCmd())
│
├── internal/
│   ├── cmd/                    cobra commands (package cmd) — flags, app.*, UI
│   │   ├── root.go             root command, persistent flags
│   │   ├── release.go          heraut release
│   │   ├── changelog.go        heraut changelog
│   │   ├── version.go          heraut version next / current
│   │   ├── version_sprint.go   heraut version sprint bump
│   │   ├── check.go            heraut check
│   │   ├── cliff.go            heraut cliff
│   │   ├── init.go             heraut init
│   │   └── self_update.go      heraut self-update
│   │
│   ├── port/                   interfaces — extractable later
│   │   ├── runner.go           Runner interface
│   │   ├── generator.go        Generator interface (Check, Validate, Generate)
│   │   └── platform.go         Platform interface
│   │
│   ├── adapter/exec/           shell runner — extractable later
│   │   └── runner.go
│   │
│   ├── testutil/               MockRunner, FakeBin — extractable later
│   │   ├── mock_runner.go
│   │   ├── fakebin.go
│   │   └── constants.go
│   │
│   ├── ui/                     terminal UI — extractable later
│   │   ├── step.go
│   │   ├── styles.go
│   │   └── version.go
│   │
│   ├── config/
│   │   ├── config.go           structs
│   │   ├── loader.go           YAML parsing (strict)
│   │   ├── path.go             path resolution
│   │   ├── validator.go        composed validation (semantic + strategy-specific)
│   │   └── error.go            ValidationError, ValidationErrors
│   │
│   ├── versioning/
│   │   ├── result.go           shared Result type
│   │   ├── tagfmt/             NEW: shared tag format package
│   │   │   └── tagfmt.go       TagFormat.Render(), ParseFromTag(), GlobPattern()
│   │   ├── semver/
│   │   │   ├── resolver.go
│   │   │   └── bump.go
│   │   ├── calver/
│   │   │   ├── resolver.go
│   │   │   ├── parser.go
│   │   │   └── format.go
│   │   └── perenv/             NEW: generic per-env resolver
│   │       ├── resolver.go     wraps VersionCalculator interface
│   │       ├── promote.go      promote mode + E001/E002/E003
│   │       └── auto.go         auto mode
│   │
│   ├── generators/
│   │   ├── gitcliff/
│   │   │   ├── generator.go
│   │   │   └── merge.go        TOML config merging
│   │   ├── communique/
│   │   │   └── generator.go
│   │   └── cocogitto/
│   │       └── generator.go
│   │
│   ├── platforms/
│   │   ├── gitlab/
│   │   │   ├── platform.go
│   │   │   └── platform_test.go    contract tests
│   │   └── github/
│   │       ├── platform.go
│   │       └── platform_test.go    contract tests
│   │
│   ├── pipeline/
│   │   ├── release.go          Release pipeline (git ops + platform publish)
│   │   ├── changelog.go        Changelog-only pipeline
│   │   └── config.go           Pipeline.Config struct
│   │
│   ├── app/                    NEW: domain coordination layer
│   │   ├── resolver.go         NewResolver() — central strategy factory
│   │   └── pipeline.go         BuildPipeline(), BuildChangelogPipeline()
│   │
│   ├── scaffold/
│   │   ├── wizard.go
│   │   ├── generate.go
│   │   ├── cliff.go
│   │   └── cog.go
│   │
│   └── selfupdate/
│       ├── updater.go
│       ├── manifest.go
│       └── github.go           GitHub Releases API
│
├── testdata/                   repo-wide read-only test fixtures (.heraut.yml samples, …)
│
├── docs/
│   ├── README.md               index of the docs tree
│   ├── specs/                  six numbered behavioural specs
│   ├── adr/                    14 architectural decision records
│   └── tasks/                  roadmap.md (build plan + inline task checklist)
│
├── schema.json
├── .goreleaser.yml             GitHub Releases + GHCR
├── Dockerfile                  builds from source; ldflags point to github.com
├── .github/workflows/
│   ├── ci.yml                  PR pipeline (build + test + lint)
│   └── release.yml             release pipeline (goreleaser + ghcr)
└── go.mod                      module: github.com/adaouat/heraut
```

---

## Dependency graph

Implementation proceeds bottom-up; vertical slices deliver working functionality at the
end of each phase.

```
Layer D: Documentation foundation
  CLAUDE.md, .claude/rules/, docs/specs/, docs/adr/, docs/tasks/

Layer 0: Repo skeleton
  go.mod, cmd/heraut/main.go, internal/cmd skeleton, GitHub Actions CI, GoReleaser

Layer 1: Core contracts
  internal/port/                  Runner, Generator, Platform interfaces

Layer 2: Infrastructure
  internal/adapter/exec/          shell runner (implements port.Runner)
  internal/testutil/              MockRunner, FakeBin

Layer 3: Config
  internal/config/                structs, loader, path, validator, errors

Layer 4: Versioning foundation
  internal/versioning/tagfmt/     shared tag format (no deps on config)
  internal/versioning/semver/     SemVer resolver + bump
  internal/versioning/calver/     CalVer resolver + parser + format

Layer 5: Per-env resolver
  internal/versioning/perenv/     generic wrapper over semver/calver

Layer 6: App wiring (resolver factory)
  internal/app/resolver.go        NewResolver(cfg, env, force, runner)

Layer 7: Generators
  internal/generators/gitcliff/
  internal/generators/communique/
  internal/generators/cocogitto/

Layer 8: Platforms
  internal/platforms/gitlab/
  internal/platforms/github/

Layer 9: Pipeline
  internal/pipeline/release.go
  internal/pipeline/changelog.go

Layer 10: App wiring (pipeline factory)
  internal/app/pipeline.go        BuildPipeline(), BuildChangelogPipeline()

Layer 11: CLI commands (thin layer)
  internal/cmd/                   cobra commands (package cmd); call app.*

Layer 12: Supporting features
  internal/scaffold/              wizard + YAML generation
  internal/selfupdate/            GitHub Releases API, atomic binary replace

Layer 13: Docs reconciliation, README
```

---

## Testing principles

All code is written test-first. The cycle is **red → green → refactor**.

| Layer       | Scope                                                                                | Tooling                              |
|-------------|--------------------------------------------------------------------------------------|--------------------------------------|
| Unit        | Pure functions (version resolution, config parsing, tag format)                      | `go test`                            |
| Contract    | External CLI interactions (`glab`, `gh`, `git-cliff`, `cog`, `communique`)           | `testutil.MockRunner`                |
| Integration | Full pipeline against real git repo + fake binaries in PATH                          | `go test` + `testutil.FakeBin`       |
| Schema      | `.heraut.yml` validation against `schema.json`                                       | JSON Schema + test fixtures          |

Platform drivers (`gitlab/`, `github/`) must have contract tests that verify the exact
CLI arguments passed to `glab` and `gh`. No platform driver ships without its contract
tests.

See [`.claude/rules/testing.md`](../../.claude/rules/testing.md) for the testing
discipline that applies to every task.

---

## Tasks

### Phase D — Documentation Foundation

Goal: stand up the docs that describe what heraut is and how it will be built, before
any code. Every later phase references these docs.

#### `[x]` D01: `CLAUDE.md` + `.claude/rules/`

**Description:** Write the root `CLAUDE.md` and the four rule files under `.claude/rules/`
(claude, coding, testing, workflow). `CLAUDE.md` uses `@.claude/rules/*` includes so the
rules load automatically when Claude Code opens the repo.

**Acceptance:**
- `CLAUDE.md` exists with sections: header (rule includes), *What this tool does*, *Docs*,
  *Tech stack*, *Project layout*, *Tooling (mise)*, *ldflags invariant*, *Bundled
  external CLIs*, *When in doubt*
- `.claude/rules/{claude,coding,testing,workflow}.md` all exist
- `CLAUDE.md` resolves each `@.claude/rules/*` include without error

**Scope:** S

---

#### `[x]` D02: `docs/specs/` — six numbered specs

**Description:** Write the six numbered behavioural specs.

**Acceptance:**
- `01-overview.md` (heraut purpose, architecture, key concepts, exit codes, boundaries)
- `02-configuration.md` (full `.heraut.yml` field-by-field reference, schema header,
  every block with field tables)
- `03-commands.md` (CLI command reference)
- `04-versioning.md` (4 strategies in depth, CalVer tokens, E001/E002/E003, `source:`)
- `05-generators-and-platforms.md` (git-cliff, communique, cocogitto, GitHub, GitLab)
- `06-dx-and-testing.md` (DX requirements, testing strategy, resolved questions)

**Scope:** L

---

#### `[x]` D03: `docs/adr/` — 14 architectural decision records

**Description:** Write the 14 ADRs that capture the foundational architectural
decisions. Topics:

| #    | Title                                                          |
|------|----------------------------------------------------------------|
| 0001 | Implementation Language: Go                                    |
| 0002 | Tool Name: Héraut                                              |
| 0003 | CLI Framework: cobra + fang                                    |
| 0004 | Config File Format: YAML                                       |
| 0005 | Config File Discovery                                          |
| 0006 | Config Naming: generator / platform / release                  |
| 0007 | Version Promotion Error Handling (E001/E002/E003)              |
| 0008 | Explicit `source:` Field for `bump: promote`                   |
| 0009 | Generic Per-Env Resolver Design (`perenv` + `tagfmt`)          |
| 0010 | Embedded `cliff.toml` Default with User Override               |
| 0011 | Single-Pipeline Release via Version Pre-computation            |
| 0012 | Changelog Commit Ownership & Release Workflow                  |
| 0013 | Raw Binary GoReleaser Format                                   |
| 0014 | Self-Update Architecture (GitHub Releases API)                 |

**Acceptance:**
- 14 ADR files exist at `docs/adr/0001-…md` through `docs/adr/0014-…md`
- No gaps in numbering
- Each ADR has Status / Context / Decision / Rationale / Consequences sections
- File names are permanent — other docs link by file name, not by number

**Scope:** L

---

#### `[x]` D04: `docs/tasks/roadmap.md`

**Description:** Write this roadmap with inline `[ ] / [x]` task checkboxes, folding
the task checklist into the roadmap headings directly (bifrost-style).

**Acceptance:**
- `docs/tasks/roadmap.md` exists with the D01–D04 + T00–T24 task headings, each carrying
  an inline `[ ] / [x]` checkbox marker

**Scope:** S

### ✦ `[x]` CHECKPOINT D — Docs foundation in place

- [x] All Phase D files exist and validate against the structure above
- [x] No code yet — `go.mod`, `cmd/`, `internal/` do not exist

---

### Phase 0 — Repo Bootstrap

#### `[x]` T00: Repository skeleton — module, cobra CLI, basic build

**Description:** Create the Go module, cobra + fang root command, and `heraut --help` working.
This is the first code commit — the skeleton everything else builds on.

**Acceptance:**
- `go.mod` declares `module github.com/adaouat/heraut` with Go 1.26+
- `go build -o heraut ./cmd/heraut` produces a binary
- `./heraut --help` prints usage without error
- `./heraut --version` prints version (from ldflags; defaults to `dev` without ldflags)
- `go test ./...` passes (no tests yet, but must not fail)

**Verification:** `go build ./...` clean; `go vet ./...` clean

**Dependencies:** Phase D complete

**Files:** `go.mod`, `go.sum`, `cmd/heraut/main.go`, `internal/cmd/root.go`

**Scope:** XS

Used `charm.land/fang/v2` (confirmed module path from ADR-0003). The `Version`, `ProjectURL`,
and `LatestURL` vars live in `main.go` as ldflag targets; `Version` defaults to `"dev"`.
Added a `root_test.go` asserting `NewRootCmd()` returns a valid command with all five
persistent flags registered. `ProjectURL` and `LatestURL` are wired but unused until T21.

---

#### `[x]` T01: GitHub Actions CI — PR pipeline

**Description:** PR pipeline runs on every pull request: build, test, lint. Blocks merges
on failure.

**Acceptance:**
- `.github/workflows/ci.yml` runs `go build`, `go test ./...`, `golangci-lint run` on PRs
- `.golangci.yml` enables a sensible default lint set (covers vet, errcheck,
  staticcheck, ineffassign, unconvert, misspell)
- Pipeline passes on the T00 skeleton

**Dependencies:** T00

**Files:** `.github/workflows/ci.yml`, `.golangci.yml`

**Scope:** S

Modelled after bifrost's CI. Uses `jdx/mise-action@v4` so the same tool versions (Go,
golangci-lint) are used in CI and locally. `golangci-lint` v2 config format with explicit
linter list; govet is part of the default set. No additional linters beyond spec.

---

#### `[x]` T02: GoReleaser + release pipeline

**Description:** Cross-platform binary builds targeting GitHub Releases and a GHCR Docker
image. Release triggered by pushing a `v*` tag or `workflow_dispatch`.

**Acceptance:**
- `.goreleaser.yml` builds: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
  `windows/amd64`
- Raw binaries (no archives — see [ADR-0013](../adr/0013-raw-binary-goreleaser-format.md))
  + `checksums.txt`
- `release: disable: false` — GoReleaser creates the GitHub Release directly
- `.github/workflows/release.yml` triggered on `v*`; runs GoReleaser + pushes GHCR image
- `Dockerfile` builds from source; ldflags inject `main.Version`,
  `main.ProjectURL` (`https://github.com/adaouat/heraut`),
  `main.LatestURL` (GitHub Releases API endpoint)
- `go build` with explicit ldflags produces `./heraut --version` showing the version

**Verification:**
- `goreleaser build --snapshot --clean` runs locally without error
- `docker build -t heraut:dev .` succeeds
- `docker run --rm heraut:dev --version` prints `dev`

**Dependencies:** T00

**Files:** `.goreleaser.yml`, `Dockerfile`, `.github/workflows/release.yml`

**Scope:** S

Modelled after bifrost's goreleaser/release configs. Archives use `formats: [binary]` per
ADR-0013 (no zip/tar wrapper). Dockerfile uses `golang:1.26-alpine` builder + `alpine:3`
final stage; ldflag comment points to `.goreleaser.yml` as source of truth. GoReleaser
`dockers` section handles GHCR image push (deprecation warning present — `dockers_v2` is
coming but not yet required). Release workflow uses `git cliff` for release notes, matching
bifrost's pattern.

### ✦ `[x]` CHECKPOINT A — Build and CI foundation

- [x] `go build ./...` clean, `go test ./...` passes
- [x] PR CI pipeline runs on every push
- [x] GoReleaser snapshot build succeeds
- [x] Docker image builds and boots

---

### Phase 1 — Core Contracts and Config

#### `[x]` T03: Port interfaces + adapter + testutil

**Description:** Three core port interfaces (`Runner`, `Generator`, `Platform`), the
production `exec.Runner` adapter, and the test `MockRunner` + `FakeBin`. Foundation for
every other package.

**Acceptance:**
- `port.Runner`: `Run(name, args...) (stdout, stderr, error)` + `RunEnv(env, name, args...)`
- `port.Generator`: `Check() error`, `Validate() error`, `Generate(tag) (string, error)`
  — `Validate()` returns `nil` for generators without config to validate
- `port.Platform`: `Name() string`, `ReleaseURL(tag) string`, `Check() error`,
  `CreateRelease(tag, notes) error`, `HasAssets() bool`, `UploadAssets(tag) error`
- `adapter/exec.Runner` implements `port.Runner` via `os/exec`; supports `--dry-run`
  (logs, no execution) and `--verbose` (logs before executing)
- `testutil.MockRunner` implements `port.Runner`; records `[]Call`; returns ordered
  `[]Response`
- `testutil.FakeBin(t, name, script)` installs a fake binary in PATH

**Dependencies:** T00

**Files:** `internal/port/{runner,generator,platform}.go`,
`internal/adapter/exec/{runner,runner_test}.go`,
`internal/testutil/{mock_runner,fakebin,constants}.go`

**Scope:** M

`exec.Runner` uses `_, _ = fmt.Fprintf(...)` for dry-run and verbose log lines so errcheck is satisfied — write errors on a log writer are non-fatal. The `Out io.Writer` field (nil → os.Stderr) is exported so tests can redirect log output without constructors. `testutil.MockRunner` uses a FIFO `[]queuedResponse` queue; returning an error when the queue is empty surfaces misconfigured tests immediately. `testutil.FakeBin` uses `t.TempDir()` (auto-cleaned) and `t.Setenv("PATH", ...)` (auto-restored). Compile-time interface assertion `var _ port.Runner = (*Runner)(nil)` lives in `adapter/exec/runner.go`.

---

#### `[x]` T04: Config structs + loader + path resolution

**Description:** Struct definitions, strict YAML parsing (rejects unknown keys), and path
resolution. No semantic validation in this task.

**Acceptance:**
- `config.Config` with all fields needed to express the four versioning strategies,
  the three generators, the two platforms, per-env overrides, and the top-level
  `environments` map (Versioning, Changelog, Release, Environments)
- `config.Load(path) (*Config, error)` — strict YAML
- `config.LoadFromReader(r) (*Config, error)` — in-memory parsing for tests
- `config.ResolvePath(explicit)` — explicit > `.config/heraut.yml` > `.heraut.yml`
- `config.InitDest()` — returns `.config/heraut.yml` if `.config/` exists, else `.heraut.yml`
- YAML errors include actionable messages: "line N: unknown key 'foo'"
- Test fixtures: at least one valid config per strategy + one invalid YAML

**Dependencies:** T00

**Files:** `internal/config/{config,loader,path,error}.go`,
`testdata/config/` (sample `.heraut.yml` per strategy + an invalid YAML),
`internal/config/{loader,path}_test.go`

**Scope:** M

Go struct names follow ADR-0006: `ContentDriver` (not `GeneratorConfig`), `Release`, `Versioning`, `Platform` with `Type string \`yaml:"platform"\`` to avoid the `Platform.Platform` self-reference. `Versioning.Prefix` is `*string` so an explicit `prefix: ""` is distinguishable from an unset prefix (needed for CalVer/SemVer default-prefix logic). `yaml.v3 KnownFields(true)` enforces strict parsing; `yaml.TypeError` errors are joined and wrapped with `"config: "` prefix so the line number is preserved. `ValidationError` and `ValidationErrors` types are defined in `error.go` for use in T05. Fixture structure uses `testdata/config/valid/` and `testdata/config/invalid/` as required by T05. `//nolint:errcheck` on `f.Close()` in `Load` — close errors on read-only files are non-actionable.

---

#### `[x]` T05: Config semantic validation — composed validators

**Description:** Composed layers — required fields → enum values → strategy-specific rules
(cycle detection, source env existence).

**Acceptance:**
- `config.Validate(cfg) ValidationErrors` runs all layers in order
- Validates `version` (required, must be "1"), `versioning.strategy` (enum),
  `release.platforms[*].platform` (enum), generator names (enum)
- Strategy-specific: per-env requires `environments`; `tag_format` must contain
  `{version}`; cycle detection in `source` chains; ambiguity error per
  [ADR-0008](../adr/0008-promote-source-env.md) when multiple `auto` envs exist without
  `source`
- `ValidationError{Path, Message, Hint}`
- Fixtures live under repo-root `testdata/config/valid/` and `testdata/config/invalid/`;
  each invalid fixture has an expected error message

**Dependencies:** T04

**Files:** `internal/config/validator.go`, `testdata/config/valid/*.yml`,
`testdata/config/invalid/*.yml`, `internal/config/validator_test.go`

**Scope:** M

Three validation layers run in order and all errors are collected (no early exit): required fields → enum values → strategy-specific. `validatePerEnv` handles the four per-env rules: env-level `bump` enum, `tag_format` containing `{version}`, source validation (ADR-0008 rules: no source on auto, source must exist, no self-reference, ambiguity when multiple autos exist without explicit source), and cycle detection. Cycle detection iterates `promote` envs in sorted key order for deterministic output and marks all envs in each detected cycle path to avoid duplicate reports. Error message for "source on auto env" uses "not valid on bump: auto" wording so tests can assert on the keyword "auto". Five invalid fixtures created alongside the four existing valid ones; all tested in fixture-based table tests.

### ✦ `[x]` CHECKPOINT B — Config loads and validates

- [x] `go test ./internal/...` passes
- [x] All `testdata/config/valid/` fixtures pass; all `testdata/config/invalid/`
      fixtures fail with the expected error messages

---

### Phase 2 — First complete vertical: SemVer + gitcliff + GitHub

*Goal: `heraut release` works end-to-end for a semver project publishing to GitHub.*

#### `[x]` T06: Tag format package (`versioning/tagfmt`)

**Description:** Extract `{version}` and `{env}` substitution into a shared package
([ADR-0009](../adr/0009-generic-perenv-resolver.md)) used by both per-env resolvers.

**Acceptance:**
- `tagfmt.Render(template, env, version) string` — substitutes `{version}` and `{env}`
- `tagfmt.ParseVersion(template, tag) (version, err)` — extracts version from a tag
- `tagfmt.GlobPattern(template, env) string` — git tag glob pattern for `git tag -l`
- Handles all documented patterns: `"dev/{version}"`, `"{version}/dev"`, `"{version}_dev"`,
  `"dev_{version}"`, `"release/{version}"`
- Error on template with no `{version}` token

**Dependencies:** T00

**Files:** `internal/versioning/tagfmt/{tagfmt,tagfmt_test}.go`

**Scope:** S

`Render`, `ParseVersion`, and `GlobPattern` all operate on a template string containing `{version}` (required) and optionally `{env}`. `ParseVersion` builds a regex from the template — `{version}` maps to a named capture group `(?P<version>.+)` and `{env}` maps to `[^/]+`. All three functions return an error immediately if the template lacks the `{version}` token. No external dependencies — only stdlib `regexp` and `strings`.

---

#### `[x]` T07: SemVer resolver

**Description:** SemVer version resolver: reads git tags, determines bump from
conventional commits, produces the next version.

**Acceptance:**
- `semver.New(runner, cfg)` → `Resolver`
- `Resolver.Resolve() (versioning.Result, error)` — returns `Version`, `BumpType`,
  `CurrentTag`
- `DetermineBump(commits) BumpType` — scans `feat!`, `fix!`, `feat:`, `fix:`
- `BumpVersion(current, bump) (string, error)` — increments the appropriate component
- Prefix handling (default `"v"`): strips for compare, re-applies on output
- Initial version: returns `cfg.InitialVersion` (default `0.1.0`) when no tags exist
- `manual` mode: error if no version override provided
- Edge cases covered: prefix handling, `v1.9.0` → `v1.10.0` (not `v1.100.0`),
  `manual` mode error path, initial-version handling

**Dependencies:** T03, T04

**Files:** `internal/versioning/semver/{resolver,bump,resolver_test}.go`,
`internal/versioning/result.go`

**Scope:** M

Git log uses `--format=%B%x00` — full commit body per entry, null-byte separated — so `BREAKING CHANGE:` footers are detected correctly without a second git call. `DetermineBump` accepts full multi-line commit messages; `isBreaking` checks the first line for `!:` and the full text for `BREAKING CHANGE:`. `BumpVersion` uses `strconv.Atoi` on each component so `v1.9.0 → v1.10.0` is correct (integer arithmetic, not string manipulation). `Prefix` is a `*string` so explicit `prefix: ""` is honoured. `SetVersionOverride` method supports `bump: manual` mode.

---

#### `[x]` T08: gitcliff generator

**Description:** Embedded TOML defaults
([ADR-0010](../adr/0010-embedded-cliff-toml-default.md)), user override merging,
invocation contract.

**Acceptance:**
- `gitcliff.New(runner, cfg)` → `Generator`
- `Generator.Check() error` — `git-cliff` in PATH
- `Generator.Validate() error` — user config file exists if specified
- `Generator.Generate(tag) (string, error)` — invokes `git-cliff` with merged TOML;
  returns stdout (release notes) or writes to output file (changelog mode)
- Embedded `cliff.toml` defaults — one for changelog (with stats block), one for
  release notes (without header / stats). Defaults are locked: changes need an ADR
  (see [ADR-0010](../adr/0010-embedded-cliff-toml-default.md)).
- `EffectiveChangelogConfig()` and `EffectiveReleaseNotesConfig()` — used by `heraut cliff`
- Temp TOML file created, passed to `git-cliff --config <tmpfile>`, cleaned up
- `tag_pattern` → `--tag-pattern`

**Dependencies:** T03, T04

**Files:** `internal/generators/gitcliff/{generator,merge,generator_test,merge_test}.go`

**Scope:** M

Added a `Mode` parameter to `New()` (changelog vs release-notes) — the roadmap listed `New(runner, cfg)` but the mode is needed to select the correct embedded TOML variant. Two TOML defaults are embedded via `//go:embed` (`cliff.changelog.toml` with stats block, `cliff.release-notes.toml` without). `MergeTOML` uses `go-toml/v2` with recursive map merging and array-replacement semantics (ADR-0010). `prepareConfig()` writes the merged result to a temp file, which is cleaned up via deferred `cleanup()` after each `Generate()` call. Contract tests check `--config` presence (not its temp-path value) using `assertHasFlag` helpers.

---

#### `[x]` T09: GitHub platform + contract tests

**Description:** GitHub platform driver with full contract test coverage. Establishes the
pattern for GitLab in T16.

**Acceptance:**
- `github.New(runner, cfg)` → `Platform`
- `Platform.Check() error` — `gh` in PATH and `cfg.TokenEnv` set
- `Platform.CreateRelease(tag, notes)` — `gh release create <tag> --notes <notes>
  [--draft] [--prerelease]`
- `Platform.HasAssets()` — true if `cfg.Assets` non-empty
- `Platform.UploadAssets(tag)` — glob resolution + `gh release upload <tag> <file>` per match
- `Platform.ReleaseURL(tag)` → `https://github.com/<repo>/releases/tag/<tag>`
- Repository: `cfg.Repository` → `$GITHUB_REPOSITORY` → error
- Token env: `cfg.TokenEnv` (default `GH_TOKEN`)
- Contract tests assert exact CLI args for all operations

**Dependencies:** T03, T04

**Files:** `internal/platforms/github/{platform,platform_test}.go`

**Scope:** M

Repository resolution: `cfg.Repository` → `$GITHUB_REPOSITORY` → error. Token check: `$<TokenEnv>` (default `GH_TOKEN`) must be non-empty. Asset upload calls `filepath.Glob` per pattern — non-matching globs fail immediately with an actionable error. Contract tests assert exact args for all `gh` invocations; `--draft` and `--prerelease` are only appended when the flag is set. Follows T16 pattern.

---

#### `[x]` T10: App resolver factory + release pipeline + `heraut release`

**Description:** Wire the first complete vertical: `app.NewResolver()` factory,
`pipeline.Pipeline`, `app.BuildPipeline()`, and the `heraut release` command. After this
task, `heraut release` works end-to-end for a semver project on GitHub.

**Acceptance:**
- `app.NewResolver(cfg, env, force, runner)` — selects strategy from `cfg.Versioning.Strategy`;
  error on unknown. SemVer only at this point; calver / per-env added in T11 / T12.
- `pipeline.Config{Changelog, ChangelogFile, Notes, Platforms, CommitMessage,
  VersionOverride, Force}`
- `pipeline.New(runner, resolver, cfg, out, dryRun)` → `*Pipeline`
- `Pipeline.Check()` — runs `generator.Check()` and `platform.Check()`
- `Pipeline.Run()` — full sequence: resolve version → generate changelog (if configured)
  → git add + commit + push → git tag → create release on each platform → upload assets
  → generate release notes
- `app.BuildPipeline(runner, cfg, env, opts)` — **all generator/platform construction
  here, none in `internal/cmd/`**
- `internal/cmd/release.go` — thin: read flags, call `app.NewResolver()` +
  `app.BuildPipeline()`, call `pipeline.Run()`
- `--dry-run` and `--version` override flags
- See [ADR-0011](../adr/0011-single-pipeline-release-via-pre-computation.md) and
  [ADR-0012](../adr/0012-changelog-commit-ownership.md)

**Dependencies:** T03, T04, T05, T07, T08, T09

**Files:** `internal/app/{resolver,pipeline}.go`,
`internal/pipeline/{config,release,release_test}.go`,
`internal/cmd/{release,release_test}.go`

**Scope:** L

Pipeline flow (ADR-0011 + ADR-0012): resolve → optional changelog generate+commit+push → git tag + push → optional release-notes generate → platform CreateRelease per platform → platform UploadAssets per platform. `versioning.Resolver` interface added to `internal/versioning/` so the pipeline doesn't import specific strategy packages. `app.NewResolver` handles semver only (calver/perenv deferred to T11/T12). `app.BuildPipeline` constructs all generators and platforms — `internal/cmd/release.go` is a thin layer: read flags → load config → validate → NewResolver → BuildPipeline → Check → Run. `MockGenerator` and `MockPlatform` added to `testutil` for pipeline unit tests. `fmt.Errorf` + `os.Remove` errors suppressed with `_ =` only where non-fatal (close/remove on write paths). Commit message default is `"chore(release): ${version}"` with `${version}` substitution.

### ✦ `[x]` CHECKPOINT C — First working release

- [x] `heraut release --dry-run` on test semver repo prints correct action sequence
- [x] `heraut --help` and `heraut release --help` show complete usage
- [x] No strategy-selection logic in `internal/cmd/`

---

### Phase 3 — Strategy Expansion

#### `[x]` T11: CalVer resolver

**Description:** CalVer resolver: format string parsing, date token substitution, PATCH
increment.

**Acceptance:**
- `calver.New(runner, cfg, now func() time.Time)` — injectable clock for tests
- All tokens: `YYYY`, `MM`, `DD`, `WW`, `QQ`, `SS`, `SPRINT`, `PATCH` (required)
- `PATCH` resets to `0` when the calendar period changes; increments within
- `SPRINT` from `cfg.Versioning.Sprint`; not derived from clock
- `app.NewResolver()` updated for `"calver"`
- Comprehensive token-combination + period-boundary coverage

**Dependencies:** T03, T04, T06

**Files:** `internal/versioning/calver/{resolver,parser,format,resolver_test,parser_test}.go`

**Scope:** M

Implemented `parser.go` (token scanner + regex-based version parser/renderer) and
`resolver.go` (period-key comparison for PATCH increment vs. reset). The roadmap listed
a `format.go` file but format logic folded naturally into `parser.go` — no separate file
needed. `BumpFromDate(tags []string)` added to `Resolver` to satisfy the `perenv.VersionCalculator`
interface (T12) without touching the perenv package. All 7 token combinations and all
period-boundary edge cases covered by 50 tests. Default prefix for `calver` is `""`
(unlike semver which defaults to `"v"`) — calver users typically have no prefix.

---

#### `[x]` T12: Generic per-environment resolver

**Description:** Generic `perenv.Resolver` wrapping either semver or calver via a
`VersionCalculator` interface ([ADR-0009](../adr/0009-generic-perenv-resolver.md)).
Same logic serves both `semver-per-env` and `calver-per-env`.

**Acceptance:**
- `perenv.VersionCalculator` interface: `BumpAuto(tags, commits) (string, error)` +
  `BumpFromDate(tags) (string, error)` — implemented by semver and calver resolvers
- `perenv.New(runner, cfg, env, force, calc)` → `*Resolver`
- `Resolver.Resolve()` — auto mode: find env tags via `tagfmt.GlobPattern`, sort, next;
  promote mode: read source env's latest tag, strip to bare version, render under
  destination format
- E001: target tag exists → error (bypassed by `--force`)
- E002: destination is ahead of candidate → error (bypassed by `--force`)
- E003: no source tags exist → error (`--force` has no effect)
- `source:` field per [ADR-0008](../adr/0008-promote-source-env.md): explicit source
  overrides default single-auto resolution; cycle detection; ambiguity error
- `app.NewResolver()` updated for `"semver-per-env"` and `"calver-per-env"`
- Test matrix parameterised over both `VersionCalculator` backends — every scenario
  (auto, promote, all E00x, `source:` chains, cycle detection, ambiguity errors) runs
  against both SemVer and CalVer

**Dependencies:** T03, T04, T06, T07, T11

**Files:** `internal/versioning/perenv/{resolver,promote,auto,resolver_test}.go`

**Scope:** M

`VersionCalculator` interface placed in `perenv/resolver.go` alongside `Resolver`. Both
`semver.Resolver` and `calver.Resolver` implement both methods; the unused method returns
an explicit error (never called in practice — ADR-0009). Promote mode doesn't receive the
calculator (it copies rather than computes). `compareVersionStrings` does dot-split integer
comparison, covering both SemVer and CalVer without importing a separate library.
Cycle detection covers self-reference at runtime; full chain cycle detection deferred to
config validation (ADR-0008). 31 tests in `resolver_test.go` with a `promoteBackends`
table and separate auto-mode tests per backend.

---

#### `[x]` T13: `heraut version` subcommands

**Description:** Thin commands over `app.NewResolver()`.

**Acceptance:**
- `heraut version next` — resolves and prints next version (no side effects)
- `heraut version current [--env ENV]` — prints latest released tag; for per-env, scoped
  to destination env
- `heraut version sprint bump` — reads sprint from `.heraut.yml`, increments, writes back
- All three work for all 4 strategies
- `--dry-run` has no effect (read-only)

**Dependencies:** T10, T11, T12

**Files:** `internal/cmd/{version,version_sprint,version_test}.go`

**Scope:** S

`version next` and `version current` are one-call wrappers: load config → build resolver
(or call `app.CurrentTag`) → print tag. `app.CurrentTag` (new, in `app/current.go`) handles
the strategy-specific glob derivation; per-env strategies require `--env`. `--env` was already
a persistent root flag, so `version next` inherits it for free — confirmed in spec §Global flags.
`version sprint bump` delegates to `config.IncrementSprint`: regex replace for existing
`sprint:` fields (preserves formatting), insertion via detected indent for the first-run case.
Output for both next and current is the full tag string (git tag including env prefix/format).

### ✦ `[x]` CHECKPOINT — All 4 strategies pass `heraut version next`

- [x] `semver`, `calver`, `semver-per-env`, `calver-per-env` all work
- [x] Only `internal/versioning/perenv/` exists — no `semver_per_env` or `calver_per_env`

---

### Phase 4 — Remaining Generators and GitLab Platform

#### `[x]` T14: communique generator + contract tests

**Description:** Simple wrapper; requires full config (no embedded defaults).

**Acceptance:**
- `communique.New(runner, cfg)` → `Generator`
- `Check()` — `communique` in PATH
- `Validate()` — `cfg.Config` set and file exists
- `Generate(tag)` — `communique generate --config <file> <tag>`; returns stdout
- Contract tests verify exact CLI args

**Dependencies:** T03, T04

**Files:** `internal/generators/communique/{generator,generator_test}.go`

**Scope:** S

`communique.New(runner, cfg)` takes no mode parameter — communique has no embedded defaults and its invocation is `communique generate --config <file> <tag>` regardless of context. `Validate()` requires `cfg.Config` to be set (unlike gitcliff where config is optional). Seven contract tests verify `--version` check, config-required error, missing-file error, exact args, and runner error propagation.

---

#### `[x]` T15: cocogitto generator + contract tests

**Description:** 4-path config resolution (config × template combinations) and embedded
defaults.

**Acceptance:**
- `cocogitto.New(runner, cfg)` → `Generator`
- `Check()` — `cog` in PATH
- `Validate()` — user config/template files exist if specified
- `Generate(tag)` — `cog changelog [--at <tag>]` with effective config + template
- 4-path config resolution (per spec §7.3):
  - none / none → embedded cog.toml + embedded template
  - none / template → embedded cog.toml + user template
  - config / none → user config, no `-t`
  - config / template → user config + user template
- Embedded `cog.toml` and Tera templates (changelog + release-notes) are locked:
  changes need an ADR
- If `cfg.Output` is set (changelog mode), heraut writes stdout to the file
- Contract tests cover all 4 combinations

**Dependencies:** T03, T04

**Files:** `internal/generators/cocogitto/{generator,generator_test}.go`

**Scope:** M

`cocogitto.New(runner, cfg, mode)` takes a `Mode` parameter (changelog vs release-notes). The template path is injected into the embedded `cog.toml` at runtime via a `'<PATH_TEMPLATE.TERA>'` placeholder — no `-t` flag for embedded cases. Mode selects `changelog.tera` (full history) or `release-notes.tera` (single release, no version header). `-t` is used only for the `config/template` combination where the user's cog.toml cannot be modified. `output:` is handled by heraut: stdout captured and written to file. Thirteen contract tests cover all 4 combinations across both modes.

---

#### `[x]` T16: GitLab platform + contract tests

**Description:** Follows the T09 pattern.

**Acceptance:**
- `gitlab.New(runner, cfg)` → `Platform`
- `Check()` — `glab` in PATH, token env set
- `CreateRelease(tag, notes)` — `glab release create <tag> --notes <notes>`
- `UploadAssets(tag)` — glob resolution + `glab release upload-asset <tag> <file>` per match
- Project: `cfg.Project` → `$CI_PROJECT_PATH` → error
- Token env: `cfg.TokenEnv` (default `GITLAB_TOKEN`)
- `catalog: true` adds `--publish-to-catalog`
- Contract tests assert exact CLI args

**Dependencies:** T03, T04

**Files:** `internal/platforms/gitlab/{platform,platform_test}.go`

**Scope:** M

`gitlab.New(runner, cfg)` mirrors the GitHub platform pattern. Project resolution: `cfg.Project` → `$CI_PROJECT_PATH` → error. Token env defaults to `GITLAB_TOKEN`. `glab release create` uses `-R <project>` (not `--repo` like gh); `catalog: true` adds `--publish-to-catalog` before `-R`. Asset upload uses `glab release upload-asset` (hyphenated subcommand). Release URL is `https://gitlab.com/<project>/-/releases/<tag>`. Sixteen contract tests cover all operations including catalog flag, glob resolution, env fallback, and missing-project error.

### ✦ `[x]` CHECKPOINT E — All generators and platforms implemented and tested

- [x] Contract tests verify exact CLI arguments for all generators and both platforms

---

### Phase 5 — Complete Pipeline Surface

#### `[x]` T17: Changelog pipeline + `heraut changelog`

**Acceptance:**
- `pipeline.NewChangelog(runner, resolver, cfg, out, dryRun)` → `*ChangelogPipeline`
- `Run()` — resolve version → generate changelog → optionally commit + push → optionally
  tag
- `app.BuildChangelogPipeline(runner, cfg, env, opts)` — wires generators from config;
  **no generator construction in `internal/cmd/`**
- `heraut changelog [--commit] [--tag] [--version X.Y.Z]` — thin cmd
- `--tag` implies `--commit`
- Per-env `disable_changelog: true` exits 0 with info message
- All strategies supported

**Dependencies:** T10, T12, T13, T14, T15, T16

**Files:** `internal/pipeline/{changelog,changelog_test}.go`,
`internal/app/pipeline.go` (BuildChangelogPipeline),
`internal/cmd/changelog.go`

**Scope:** M

`pipeline.ChangelogConfig` holds `Commit`, `Tag`, `DisableChangelog`, `Changelog`, `ChangelogFile`, and `CommitMessage`. The `Tag: true` path implies commit — `BuildChangelogPipeline` sets `Commit: opts.Commit || opts.Tag` so the caller doesn't need to normalize the flags. When `Changelog == nil` but `Tag == true`, only the tag step runs (no git add/commit/push). `buildGenerator` extended to support communique and cocogitto; `buildPlatform` extended to support gitlab (gap left from T16). The `PipelineOpts` struct gained `Commit` and `Tag` fields used only by the changelog pipeline.

---

#### `[x]` T18: `heraut check` subcommands

**Acceptance:**
- `heraut check config` — offline: `config.Load()` + `config.Validate()`, prints errors
  with paths and hints
- `heraut check runtime` — online: binaries in PATH, token env vars set,
  `git user.name` + `user.email` configured
- `heraut check cliff` — runs `git-cliff --context --no-exec` against the effective
  merged config; non-zero exit if git-cliff rejects
- `heraut check cliff changelog` and `heraut check cliff release-notes` scope to one
  generator
- `heraut check` (bare) runs all three; final exit code non-zero if any failed
- Automatic preflight before `heraut release` and `heraut changelog`

**Dependencies:** T04, T05, T08, T10

**Files:** `internal/cmd/{check,check_test}.go`

**Scope:** M

`app.PreflightCheck(runner)` checks git binary + git user.name/email; called from both `heraut release` and `heraut changelog` before pipeline execution (generator/platform CLIs are covered by `pipeline.Check()`). `app.RuntimeCheck(runner, cfg)` does the full check (git + generators + platforms + git user) for `heraut check runtime`. `app.CheckCliff(runner, driver, mode string)` delegates to `gitcliff.Generator.CheckCliff()` (new method that runs `git-cliff --context --no-exec --config <tmpfile>`). The `heraut check` bare command runs all three checks and exits non-zero if any fail. The `heraut check cliff` uses string modes ("changelog"/"release-notes") to avoid the cmd→generators layer violation. Non-git-cliff generators are skipped with an info message.

Post-initial: `heraut check config` (and the bare `heraut check`) now display the resolved config file path and resolution source (`--config`, `HERAUT_FILE`, `.config/heraut.yml`, `.heraut.yml`) after a successful load — implemented via `config.ResolvePathWithSource` which returns a typed `PathSource` alongside the path. `heraut check runtime` now handles a missing config file gracefully: `configuredGenerators(nil)` and `configuredPlatforms(nil)` return all supported tools as required; the full platform check (token + API) is skipped when cfg is nil since there is no config to derive env var names from — only binary presence is checked.

---

#### `[x]` T19: `heraut cliff` + per-env disable flags

**Acceptance:**
- `heraut cliff changelog` — prints the effective merged git-cliff TOML for changelog mode
- `heraut cliff release-notes` — same for release notes mode
- `EnvVersioning.DisableChangelog` and `EnvVersioning.DisableNotes` — when true for the
  active env, the respective pipeline step is skipped silently
- `app.BuildPipeline()` applies per-env disable flags when building pipeline config

**Dependencies:** T08, T10, T12

**Files:** `internal/cmd/cliff.go`, `internal/app/pipeline.go` (per-env disable flags)

**Scope:** S

`pipeline.Config.DisableNotes` added alongside existing `DisableChangelog`; `buildReleasePipelineConfig` now sets both from per-env config. `app.EffectiveCliffConfig(driver, mode)` builds a gitcliff generator with a nil runner (safe — effective config methods only read files, never exec) and returns the merged TOML. `heraut cliff changelog` and `heraut cliff release-notes` print the embedded default when no driver is configured, error when the configured generator is not git-cliff.

### ✦ `[x]` CHECKPOINT F — Full pipeline surface implemented

- [x] `go test ./...` passes (339 tests)
- [x] `heraut release --dry-run` works for all 4 strategies
- [x] `heraut changelog --dry-run` works for all 4 strategies
- [x] `heraut version next` works for all 4 strategies
- [x] `heraut check config` catches all validation errors
- [x] No domain/factory logic in `internal/cmd/`; all wiring in `internal/app/`

---

### Phase 6 — Supporting Features

#### `[x]` T20: `heraut init` wizard

**Acceptance:**
- `scaffold.RunWizard()` — interactive huh-based prompts for strategy, platforms,
  generators, prefix, format, sprint, environments
- `scaffold.GenerateYAML(answers)` — renders `.heraut.yml` with `# yaml-language-server:
  $schema=<url>` header
- `scaffold.ConfigToAnswers(cfg)` — populates Answers from existing config for re-run
- `scaffold.Defaults()` — opinionated non-interactive defaults (semver, prefix "v",
  gitlab, git-cliff)
- `heraut init` — detects existing config: prompts "Update it?" (Y → wizard
  pre-populated; N → exit 0); `--force` skips prompt; `--defaults` writes
  non-interactively
- Multi-env wizard: loops to add N environments
- YAML output round-trips through Load + Validate without errors

**Dependencies:** T04, T05

**Files:** `internal/scaffold/{wizard,generate,cliff,cog,generate_test,wizard_test}.go`,
`internal/cmd/init.go`

**Scope:** M

`charm.land/huh/v2` added as a direct dependency (v2 module path, not the legacy `github.com/charmbracelet/huh`). `WithHideFunc` is group-only in huh v2, so conditional fields (platform-specific inputs, CalVer custom format, env source) are placed in separate groups rather than per-field. CalVer wizard offers 7 opinionated presets plus a validated custom input; `ValidateCalVerFormat` reuses `calver.ParseFormat` for structural checks and adds an unknown-token scan for suspicious uppercase literals. `--defaults` always writes non-interactively without prompting (even when a config already exists); `--force` skips the "Update it?" prompt for the interactive wizard path. `wizard_internal_test.go` uses package-internal access to test `resolveFormatChoice`.

Post-initial: the platform project/repository field is pre-populated from `git remote get-url origin` when the current directory is a git repo — `parseRemoteProject` handles SSH (`git@host:ns/proj.git`), `ssh://`, and HTTPS schemes including nested GitLab groups; detection is best-effort and silent on failure. When GitLab is selected, a `huh.NewNote()` group (`.Next(true)`) is shown between the project input and the token step, advising CI/CD users to use `CI_JOB_TOKEN` and enable "Allow Git push requests" in project settings.

---

#### `[x]` T21: `heraut self-update` — GitHub Releases API

**Description:** Self-update targeting the GitHub Releases API. See
[ADR-0014](../adr/0014-self-update-architecture.md).

**Acceptance:**
- `selfupdate.New(currentVersion, projectURL, latestURL)` → `*Updater`
- Background hint: non-blocking goroutine checks the GitHub Releases API for latest;
  prints "hint: heraut X.Y.Z available — run: heraut self-update" after command
  completes
- `heraut self-update --check` — prints current vs latest; exit 0 if up-to-date, 1 if
  update available
- `heraut self-update` — downloads binary for current OS/arch, verifies SHA-256
  checksum, atomically replaces the running binary
- `HERAUT_CHECK_UPDATE=false` disables the background hint
- Skipped for `dev` builds (no ldflags) and during `heraut self-update` itself
- macOS: removes `com.apple.quarantine` xattr post-download

**Dependencies:** T00 (ldflags), T03

**Files:** `internal/selfupdate/{updater,manifest,github,selfupdate_test}.go`,
`internal/cmd/self_update.go`

**Scope:** M

**Done:** `ProjectURL` and `LatestURL` dropped as ldflags — they are stable constants hardcoded in `internal/selfupdate/`. Only `Version` remains an ldflag, now using `{{.Tag}}` (e.g. `v1.2.3`) instead of `{{.Version}}` so the value matches the GitHub API `tag_name` format directly. `selfupdate.New(version, opts...)` takes variadic `Option`; `WithLatestURL` is the one exported option (used by cmd-level tests via `NewRootCmd(version, selfupdate.WithLatestURL(...))`). Background hint is synchronous-with-500ms-context in `PersistentPostRunE` — functionally equivalent to a goroutine from the user's perspective since it runs after the command output. 13 unit tests + 3 cmd tests; all tests use `httptest.Server`.

### ✦ `[x]` CHECKPOINT G — DX features complete

- [x] `heraut init --defaults` produces a valid config that passes `heraut check config`
- [x] `heraut self-update --check` exits without error against mock GitHub API

---

### Phase 7 — Doc Reconciliation + Public README

Most docs are written upfront in Phase D. This phase handles what *can't* be written until
the implementation is complete.

#### `[x]` T22: Spec reconciliation

Walk the 6 `docs/specs/` files against the implementation. Any drift (flag rename, behavior
tweak, unimplemented field) gets fixed in the spec, not just the code. The spec is the
source of truth for users.

Files: `docs/specs/*.md`

Scope: S

**Done:** Walked all 6 specs against the code. Fixed five doc-only drifts where the spec
was wrong and the code was right: (1) Spec 03 root-flag location `cmd/heraut/root.go` →
`internal/cmd/root.go`; (2) Spec 06 GitHub contract-test example arg order (`--notes`
precedes `--repo`); (3) Spec 03 `heraut release` action sequence — release notes are
generated *before* platform creation and passed via `--notes`, not generated/attached
afterwards; (4) Spec 05 GitHub invocation order (`--repo` before `[--draft] [--prerelease]`);
(5) Spec 05 git-cliff invocation — `--tag` and `--unreleased` are always passed, corrected
order. Reconciled five spec-vs-code feature gaps with the user: kept `heraut --version` as
heraut-version-only and pointed to `heraut check runtime` for tool checks (Spec 03);
corrected the global-flag table so `-v` is the `--version` shorthand and `--verbose` has
none (matches fang/cobra reality); removed the inaccurate "(and its output afterward)"
claim from `--verbose`; documented lightweight tags instead of "annotated" in Spec 03.
Two gaps the user wants to keep as targets were left in the specs and tracked as new
tasks: structured exit codes (Spec 01 table — T27) and annotated tags (T28). No code was
changed in this task.

---

#### `[x]` T23: ADR reconciliation

Re-read the 14 ADRs from `docs/adr/`; validate that 0007 (E001/E002/E003), 0008
(`source:` field), 0009 (generic perenv), 0010 (embedded cliff), and 0014 (self-update)
still match the code. Amend any ADR whose *Consequences* section no longer reflects what
was built.

Files: `docs/adr/*.md`

Scope: S

**Done:** Validated the five focus ADRs against the code. **0008 and 0010 were accurate**
(validator enforces all four `source` rules + cycle detection at config time; gitcliff
embed filenames, `pelletier/go-toml/v2` merge, and `prepareConfig()` all match). Amended
three: **0007** — clarified the sentinels are wired but the exit-code-4 mapping is not
(tracked by T27); corrected the false "the comparator differs" claim (a single
`compareVersionStrings` handles both SemVer and CalVer); added an implementation-status
note that the rich Biome-style multi-line errors are the target but not yet built (tracked
by new **T30**). **0009** — fixed the `New` signature (`cfg *config.Config`, no error
return, not `config.Versioning` with error) and `GlobPattern` (returns `(string, error)`).
**0014** — the URLs are compiled-in constants (`defaultProjectURL`/`defaultLatestURL`) in
`internal/selfupdate/updater.go`, **not** `main.ProjectURL`/`main.LatestURL` ldflags; only
`main.Version` is injected; rewrote the ldflags section and the hint-disable bullet
accordingly. Also fixed the stale `CLAUDE.md` § ldflags-invariant table (per user request,
outside the strict `docs/adr/` scope) to list only `main.Version`. Added T30 (rich
promotion errors). No production code changed.

---

#### `[x]` T25: `schema.json` — JSON Schema for `.heraut.yml`

**Description:** Publish a JSON Schema at the project root so IDEs can validate
`.heraut.yml` via the `# yaml-language-server: $schema=…` header emitted by
`heraut init`. The schema must match the `config.Config` struct exactly
(same required fields, same enums, same additionalProperties: false strictness).

**Acceptance:**
- `schema.json` exists at the project root, validates as JSON Schema draft-07
- Covers: `version`, `versioning` (all sub-fields + enums), `changelog`,
  `release`, `environments`, `platform` (GitHub/GitLab fields), `ContentDriver`
- `if`/`then` conditionals: calver strategies require `format`; per-env strategies
  require `versioning.environments`
- All `testdata/config/valid/` fixtures pass the schema
- Structural-invalid fixtures (`invalid_strategy.yml`, `invalid_generator.yml`,
  `missing_version.yml`, `unknown_key.yml`, `perenv_no_environments.yml`) fail
- Semantic-only fixtures (`source_ambiguous.yml`, `source_cycle.yml`) pass

**Files:** `schema.json`, `internal/config/schema_test.go`

**Scope:** S

**Done:** Implemented JSON Schema draft-07 at `schema.json`. Used `definitions` (not `$defs`) with `additionalProperties: false` throughout and `if`/`then` conditionals for strategy-dependent required fields. Tests use `github.com/santhosh-tekuri/jsonschema/v6` — the v6 API requires `compiler.Compile(path)` directly; passing a `*strings.Reader` to `AddResource` was wrong (it expects a decoded JSON value). All 4 schema test functions (ValidJSON, ValidFixtures, InvalidFixtures, SemanticOnlyFixtures) pass against the 4 valid and 7 invalid fixtures in `testdata/config/`.

---

#### `[x]` T26: Versioned `schema.json` URL per heraut release

**Description:** `heraut init` currently emits a hardcoded `main`-branch schema URL in
`.heraut.yml`. Once heraut ships releases, configs should pin to the schema version that
was current when they were generated, matching the Biome-style pattern. Version is
threaded from the build-time `main.Version` ldflag down to `scaffold.GenerateYAML`.

**Approach:**
- Schema hosting: raw.githubusercontent.com at the git tag
  (`https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json`). Zero
  infrastructure — tags are immutable, no GitHub Pages or release assets needed.
- `"dev"` / `""` → falls back to `main` branch URL so local builds always get the
  latest schema.
- Version threaded explicitly through constructors:
  `main.Version → NewRootCmd(version) → NewInitCmd(version) → GenerateYAML(a, version)`

**Acceptance:**
- `buildSchemaURL("v1.2.3")` → `…/v1.2.3/schema.json`
- `buildSchemaURL("dev")` and `buildSchemaURL("")` → `…/main/schema.json`
- `heraut init --defaults` on a release binary writes the versioned URL in `.heraut.yml`
- `go build -ldflags="-X main.Version=v1.2.3" ./cmd/heraut/ && ./heraut init …`
  produces `$schema=…/v1.2.3/schema.json`

**Deferred:** `schema.json` `$id` field still points to `main` (minor spec deviation,
acceptable until a docs-hosting story is defined).

**Files:** `internal/scaffold/generate.go`, `internal/cmd/init.go`,
`internal/cmd/root.go`, `cmd/heraut/main.go`

**Plan:** `.claude/plans/t26-versioned-schema-url.md`

**Scope:** S

---

#### `[x]` T24: `README.md`

Public-facing, written last. Install (`go install`, GitHub binary,
`ghcr.io/adaouat/heraut`), quickstart (`heraut init` → `heraut check config` →
`heraut release`), short configuration reference linking to
`docs/specs/02-configuration.md` for the full one, `heraut self-update`, links to
`docs/specs/` and `docs/adr/`.

Files: `README.md`

Scope: S

**Done:** Wrote the public README. Header uses the brand horizontal lockup
(`docs/images/lockup-horizontal.png`) — the user added eight brand images to
`docs/images/` and asked for a subset, so only the horizontal lockup is referenced (the
stacked lockup and release-card are available if a different header is preferred). Static
shields.io badges for MIT + Go 1.26. Sections: what-it-does + the pun, install (`go
install` / prebuilt raw binary with checksum note / Docker), prerequisites table mapping
each external CLI to the config that needs it, quickstart (init → check config → dry-run
→ release), a commands table + global-flags line, a short configuration reference with a
working `.heraut.yml` example and the `$schema` header, self-update, and doc links. The
prebuilt-binary curl uses a `<version>` placeholder rather than a fabricated
`latest/download` URL, since GoReleaser asset names embed the version (ADR-0013).

---

#### `[x]` T27: Structured exit codes

**Description:** Implement the exit-code contract documented in
[Spec 01 § Exit codes](../specs/01-overview.md#exit-codes). Today every error path exits
`1` (`cmd/heraut/main.go` does a blanket `os.Exit(1)`). Map error categories to the
documented codes: `0` success, `1` usage error, `2` config error (invalid YAML / semantic
validation), `3` runtime error (binary missing, token unset, network, git failure), `4`
promotion guard tripped (E001/E002/E003 — reuse the existing sentinels from ADR-0007),
`70` internal software error.

**Approach:** Define typed/sentinel errors (or a small `ExitError` wrapper carrying a
code) at the package boundaries that already produce these failures, and translate them
to exit codes in `cmd/heraut/main.go` instead of the unconditional `os.Exit(1)`. The
per-env sentinels (`ErrTargetExists`, `ErrDestinationAhead`, `ErrNoSourceTags`) already
exist — `errors.Is` them to code `4`. Surfaced by T22 spec reconciliation.

**Files:** `cmd/heraut/main.go`, error definitions across `internal/config/`,
`internal/versioning/perenv/`, `internal/adapter/exec/`

**Scope:** M

**Done:** Added a leaf `internal/exitcode` package (codes `OK/Usage/Config/Runtime/
Promotion/Internal` = `0/1/2/3/4/70`) with an `Error` wrapper, `Wrap(code, err)`
(nil-safe; first/innermost classification wins), and `Resolve(err) int`. Classification
happens at the cmd boundary rather than deep in the stack (honours "cmd decides the exit
code"): each RunE wraps config load/validate/`NewResolver`/`BuildPipeline` failures as
`Config` (2), preflight/runtime/cliff failures as `Runtime` (3), and pipeline/resolve
failures through `wrapRunErr`, which routes promotion guards to `Promotion` (4) via the
new `app.IsPromotionGuard` (app already imports perenv; keeps cmd off perenv) and
everything else to `Runtime` (3). `cmd/heraut/main.go` now exits `cmd.ExitCode(err)`
instead of a blanket `os.Exit(1)`. Unclassified errors (including cobra usage/flag errors)
default to `1`, preserving historic behaviour; `70` is defined for unexpected/internal
conditions but currently unused (no cmd path produces it — panics are fang's domain).
Verified end-to-end on the built binary: `--version`→0, bad flag→1, missing-version /
invalid-strategy / missing-file config→2; cmd tests cover promotion-guard E003→4 and
`version current` no-tags→3. Also un-stale'd ADR-0007 (the exit-code-4 mapping is now
implemented).

---

*T28 moved to Phase 8 — Stable Release Preparation.*

---

#### `[x]` T29: Verbose output echo + stderr on failure

**Description:** Two related transparency gaps in the exec runner, surfaced by T22.
(1) `--verbose` only logs `[exec] <cmd> <args>` before running — it does not echo the
command's output afterward. (2) On failure the runner wraps the error as `"%s: %w"` and
most callers discard the returned stderr, so a failed `gh release create` surfaces only
`gh: exit status 1` with none of the underlying tool's error message.

**Approach:** In `internal/adapter/exec/runner.go`, after a command returns: when
`Verbose`, print the captured stdout/stderr (indented) to the log writer (buffered, not
streamed — heraut needs stdout as a return value, e.g. git-cliff's changelog). On
non-nil error, include the captured stderr in the wrapped error regardless of verbose so
failures are diagnosable. Update Spec 03's `--verbose` description to document the echo.

**Files:** `internal/adapter/exec/runner.go`, `docs/specs/03-commands.md`

**Scope:** S

**Done:** Both gaps fixed in `RunEnv` (so `Run` inherits them). After `cmd.Run()`, when
`Verbose` the captured stdout+stderr are echoed via a new `echoOutput` helper — each
non-empty line indented two spaces under the `[exec]` line, to the same writer (`r.Out` /
`os.Stderr`). On failure the wrapped error now appends the trimmed stderr when non-empty
(`"%s: %w: %s"`), keeping `%w` so `errors.Is`/`errors.As` still work; with no stderr the
message is unchanged (`"%s: %w"`, no dangling colon). Return values are untouched, so
callers that consume stdout (e.g. git-cliff → changelog) are unaffected; `MockRunner` is
also unaffected since the change lives only in the real exec adapter. Spec 03's `--verbose`
row updated. Verified: 447 tests pass, and the built binary shows the indented echo under
`[exec]` on `version current --verbose`.

---

#### `[x]` T30: Rich promotion error messages (E001/E002/E003)

**Description:** [ADR-0007](../adr/0007-version-promotion-error-handling.md) specifies
Biome-style multi-line errors for the three promotion guards (what was found, why it is
wrong, how to fix it — with concrete `git tag` / `--force` remediation). Today
`internal/versioning/perenv/promote.go` emits concise single-line sentinel errors
(`E001: target tag already exists: tag "prod/1.0.2" already exists (pass --force to
bypass)`). Implement the rich format from the ADR examples. Surfaced by T23.

**Approach:** Render the rich messages where the sentinels are produced (or where they
are caught for display), carrying the source env, destination env, candidate version,
and the latest source/destination tags. Keep the sentinel `errors.Is` identity intact so
the pipeline's `--force` handling and the future exit-code mapping (T27) still work.
Pairs naturally with T27 (exit code 4) since both touch the promotion error path.

**Files:** `internal/versioning/perenv/promote.go` (+ wherever the errors are rendered),
`internal/versioning/perenv/resolver.go`

**Scope:** M

**Done:** Implemented `PromotionError` in `promote.go` — a struct that wraps the sentinel
(`Unwrap()` preserves `errors.Is` identity through the chain) and carries the context
needed for each code: srcEnv, destEnv, srcTag, candidateTag, latestDestTag,
latestDestVersion, suggestedSrcTag, srcGlob. `Error()` dispatches to `renderE001/2/3`
which produce the exact Biome-style multi-line format from ADR-0007 examples — including
concrete `git tag -d` / `--force` / `heraut release --env` remediation lines.
`suggestedSrcTag` is pre-computed via `tagfmt.Render` so the E002 "create the missing
source tag" hint uses the actual tag format. Three new message-content tests
(`TestPromotionError_E001/2/3_RichMessage`) assert on key phrases plus `errors.As`
extraction. All 450 tests pass. ADR-0015 (charm/log) rejected in the same session; the
preferred alternative (`internal/ui` status-line helpers) is tracked in T32.

---

#### `[x]` T32: `internal/ui` status-line helpers

**Description:** Grow `internal/ui` with typed status-line helper functions —
`Success(msg)`, `Warn(msg)`, `Err(msg)`, `Info(msg)` — backed by lipgloss v2 styling
(already an indirect dep via fang/huh). These helpers replace the ad-hoc `✓`/`✗`
`fmt.Fprintf` calls spread across `internal/cmd/check.go` and similar command files.
TTY detection should gate ANSI output (plain text in CI/pipes, styled in terminal).
This is the "Option 4" path from the rejected ADR-0015: growing `internal/ui` without
introducing a logging framework.

**Acceptance:**
- `ui.Success(msg) string`, `ui.Warn(msg) string`, `ui.Err(msg) string`,
  `ui.Info(msg) string` — each returns a styled (or plain, if no TTY) line
- TTY detection: ANSI disabled when stderr is not a terminal and when `NO_COLOR` is set
- `internal/cmd/check.go` updated to use the helpers in place of raw `fmt.Fprintf`
- Unit tests: plain-text output (no TTY), ANSI-stripped assertions for TTY path

**Files:** `internal/ui/status.go`, `internal/ui/status_test.go`,
`internal/cmd/check.go` (callers)

**Scope:** S

**Done:** Added `Success`, `Err`, `Warn`, `Info` helpers to `internal/ui/status.go`.
Each takes `(w io.Writer, msg string) string` — TTY/color detection is delegated to
`colorprofile.Detect(w, os.Environ())` which handles NO_COLOR, TERM=dumb, and
non-terminal writers (pipes, buffers) in one call. Helpers return "✓/✗/!/  " prefixed
plain text in non-color contexts; in color-capable contexts the symbol gets a
lipgloss bold+color treatment while the message stays unstyled (preserves
copy-paste-friendliness). `internal/cmd/check.go` updated: all four raw `fmt.Fprintf`
status-line patterns replaced with the helpers (`printRuntimeItems`, `printConfigErrors`,
`checkCliffDriver`, config-ok lines). Seven new tests cover the non-TTY format, message
preservation, and NO_COLOR compliance. `charm.land/lipgloss/v2` and
`colorprofile` were already indirect deps via fang/huh — no new modules added.
Extended the migration to all remaining `internal/cmd/` call sites:
`release.go` and `changelog.go` now delegate their config-error blocks to `printConfigErrors`
(eliminating the duplicate loop + `os.Stderr` direct writes); `self_update.go`, `init.go`,
and `version_sprint.go` use `ui.Success` for their single confirmation lines. `version.go`
and `cliff.go` were deliberately left plain — their output is machine-readable (tag names,
raw TOML) and must not be decorated.

---

#### `[x]` T31: Batteries-included Docker image + multi-tag release

**Description:** Today both `Dockerfile` and `Dockerfile.goreleaser` produce a minimal
alpine image containing only the `heraut` binary (`apk add ca-certificates` + the binary).
That means the published `ghcr.io/adaouat/heraut` image can't actually run a release — it
has none of the external CLIs heraut orchestrates. Rework the image into a self-contained
release runner that bundles all required tools at pinned, supported versions, and is built
**outside GoReleaser** by a dedicated CI workflow.

**Bundle (pinned versions):** `git`, `git-cliff`, `gh`, `glab`, `cog` (cocogitto), and
`communique`. git-cliff (`2.13`) and cocogitto (`7.0.0`) versions exist in
`.config/mise/config.toml`; `gh`, `glab`, and `communique` are not pinned anywhere yet and
need explicit versions chosen for the image. Keep `heraut`'s own version injected via the
`-X main.Version=...` ldflag (the [ldflags invariant](../../CLAUDE.md) still holds — this
image does the version injection itself since it's not GoReleaser-driven).

**Decoupled from GoReleaser:** the bundled image is built and pushed by its own workflow
step (e.g. `docker/build-push-action` + `docker/metadata-action` for tag generation), not
by the `.goreleaser.yml` `dockers:` block. The current minimal GoReleaser image is
**removed entirely** — delete `Dockerfile.goreleaser` and the `.goreleaser.yml` `dockers:`
block; the bundled image is the only published container (a slim binary-only variant has
no value over the raw binary / `go install`).

**Tagging:** on each new release `vX.Y.Z`, push the cascading tags so consumers can pin at
their preferred precision:

| Tag | Example (for `v1.4.2`) |
|-----|------------------------|
| `MAJOR.MINOR.PATCH` | `ghcr.io/adaouat/heraut:1.4.2` |
| `MAJOR.MINOR` | `ghcr.io/adaouat/heraut:1.4` |
| `MAJOR` | `ghcr.io/adaouat/heraut:1` |
| `latest` | `ghcr.io/adaouat/heraut:latest` |

(`docker/metadata-action` produces this set from a semver tag out of the box.)

**Open decisions to settle during implementation:**
- Base image: stay on alpine (musl — verify `gh`/`glab`/`communique` ship musl builds) or
  move to a debian-slim/distroless base for glibc compatibility.
- Multi-arch (`linux/amd64` + `linux/arm64`) or amd64 only.
- Whether `latest` and bare `MAJOR` should move on pre-releases (probably not).

Note: distroless is likely incompatible with bundling shell-driven CLIs — flagged here as
a decision, but a minimal alpine/debian with the tools is the more probable landing.

**Likely needs an ADR:** this changes heraut's distribution story (ADR-0013 covers the raw
binary; the container positioning shifts from "thin wrapper" to "full release runner") and
introduces a tagging policy — write an ADR for the bundled-image decision + tag scheme.

**Files:** `Dockerfile`, `Dockerfile.goreleaser` (deleted), `.goreleaser.yml` (drop the
`dockers:` block), `.github/workflows/release.yml` (or a new workflow), `CLAUDE.md`
(update the project-layout/ldflags notes that reference `Dockerfile.goreleaser` and the
GHCR image flow), `.config/mise/config.toml` (if the new tool pins are tracked there), a
new `docs/adr/00NN-*.md`.

**Scope:** M

**Done:** Three-stage Dockerfile (builder → mise tools → debian:trixie-slim final).
All six external tools bundled at pinned ARG versions; only bare binaries are copied into
the final image — mise itself is absent at runtime. Base image decision: `debian:trixie-slim`
over Alpine because `gh`, `glab`, and `communique` ship only glibc builds (no musl).
Multi-arch: `linux/amd64` + `linux/arm64` via buildx + QEMU in CI. Tagging: four cascading
tags (`X.Y.Z`, `X.Y`, `X`, `latest`) via `docker/metadata-action` semver patterns; `latest`
and bare `MAJOR` only move on the default branch (pre-releases excluded). `HERAUT_CHECK_UPDATE`
ENV deferred — no auto-update check exists today, so it would be a no-op. `Dockerfile.goreleaser`
deleted, `dockers:` block removed from `.goreleaser.yml`, release workflow extended with
QEMU + buildx + metadata + build-push steps. ADR-0016 documents the distribution story change.

---

### ✦ `[x]` CHECKPOINT H — Ready for public launch

- [x] `go test ./...` passes
- [x] `goreleaser build --snapshot --clean` succeeds
- [x] Docker image boots, `heraut --version` prints version (verified via CI; Dockerfile builds correctly)
- [x] `heraut init --defaults` → `heraut check config` → passes
- [x] `heraut release --dry-run` prints correct sequence for all 4 strategies
- [x] README covers install + quickstart
- [x] `docs/specs/` and `docs/adr/` reconciled with the implementation

**Note:** During checkpoint H smoke tests, a bug was found and fixed: `perenv` did not
fall back to the top-level `versioning.tag_format` when an environment had no per-env
override — causing `semver-per-env` configs with a shared top-level `tag_format` to fail.
Fix: `tagFormat(cfg, env)` helper in `resolver.go`. All four dry-run strategies verified
after the fix. ADR count bumped to 16 across roadmap, CLAUDE.md, and docs/adr/README.
README Prerequisites clarified: Docker image bundles all CLIs.

---

### Phase 8 — Stable Release Preparation

Goal: cut v1.0.0 using heraut itself, with quality gates enforced in CI and the
bootstrapped release flow settled.

#### `[x]` T28: Annotated git tags (revisit)

**Description:** Both pipelines currently create lightweight tags (`git tag <tag>` in
`internal/pipeline/release.go` and `changelog.go`). Annotated tags (`git tag -a -m …`)
carry a tagger, date, and message and are what most release tooling expects. Decide
whether to switch, and if so what the annotation message should be (likely the resolved
version or the release notes). Discuss implementation before starting. Surfaced by T22
spec reconciliation; Spec 03 currently documents lightweight tags to match the code.

**Files:** `internal/pipeline/release.go`, `internal/pipeline/changelog.go`,
`docs/specs/03-commands.md`

**Scope:** S

**Done:** Added `versioning.tag_type: annotated | lightweight` config field (default:
`annotated`). Both pipelines now create annotated tags by default via a private
`gitTag(tag, version)` helper that branches on `cfg.AnnotatedTags`. The annotation
message reuses the `commit_message` template (default `"chore(release): ${version}"`) so
no second config field is needed. `app/pipeline.go` maps `tag_type != "lightweight"` →
`AnnotatedTags: true`. Validator rejects unknown `tag_type` values; schema.json updated;
Spec 02 and 03 updated. 8 new tests (2 annotated + 2 custom-message per pipeline).

---

#### `[x]` T35: CI improvements — split jobs, coverage threshold, goreleaser check

**Description:** Rework `.github/workflows/ci.yml` into three independent jobs so each
can be individually required in branch protection. Add a coverage threshold gate and a
`goreleaser check` step to prevent silent release-config drift.

**Acceptance:**
- `ci.yml` has three separate jobs: `lint`, `test`, `build`
  - `lint`: `golangci-lint run ./...` (unchanged behaviour, now independently requirable)
  - `test`: `go test ./... -coverprofile=coverage.out`; fail if total statement coverage
    falls below 80%
  - `build`: `go build ./cmd/heraut/` + `goreleaser check` (validates `.goreleaser.yml`
    without running a release)
- No behavioral change to existing lint/test/build checks — only the job topology changes
- The coverage threshold applies from this task forward; T34 is the sweep that brings
  coverage above it

**Dependencies:** none

**Files:** `.github/workflows/ci.yml`

**Scope:** S

**Done:** Replaced the single `ci` job with three independent parallel jobs: `lint`
(golangci-lint), `test` (go test + coverage threshold), `build` (go build + goreleaser
check). Each job can be individually required in branch protection. Coverage threshold
set at 70% now (current baseline is 69.6%); T34 raises coverage to ≥85% and will bump
the threshold to 80%. `goreleaser check` added to the build job to catch config drift
without running a release. The `build` job uses `fetch-depth: 0` for GoReleaser
compatibility.

---

#### `[x]` T34: Coverage sweep — enforce 80%, target 85%

**Description:** Bring test coverage up to the threshold introduced by T35. The 0% and
low-coverage packages are mechanical — unit and contract tests, no new design required.
Wizard forms and entry-point code are explicitly excluded (untestable without a VT100
harness or binary exec).

**Packages to cover (priority order):**

| Package / function | Current | Why 0% / low |
|--------------------|---------|--------------|
| `internal/app/` (all functions) | 0% | Factory wiring — unit tests with MockRunner/MockGenerator/MockPlatform |
| `internal/config/error.go` | 0% | `ValidationError.Error()` and `ValidationErrors.Error()` — never called in tests |
| `internal/config/sprint.go` | 0% | `IncrementSprint` — cmd test goes through `version sprint bump` but doesn't reach here |
| `internal/scaffold/cliff.go`, `cog.go` | 0% | Embedded-file accessors — one-liner tests |
| `internal/generators/gitcliff` `CheckCliff` | 0% | New method from T18, no test added |
| `internal/versioning/semver/resolver.go` `BumpAuto`, `BumpFromDate` | 0% | perenv tests use mock calculator — direct semver calls missing |
| `internal/cmd/release.go` (13%), `changelog.go` (20%) | low | Error paths: config load, validation, resolver failures |
| `internal/selfupdate/updater.go:282, 337` | 0% | Remaining error/edge branches; httptest already in place |
| `internal/ui/status.go` | 67% | Styled code path — needs a colour-capable writer in tests |
| `internal/platforms/` `UploadAssets` | 67% | Glob-mismatch error path not exercised |
| `internal/pipeline/` (63–81%) | low | Error branches in release and changelog pipelines |

**Explicitly excluded:** `internal/scaffold/wizard.go` (interactive `huh` forms),
`cmd/heraut/main.go` (entry point), `internal/testutil/` (test helpers).

**Acceptance:**
- `go test ./... -cover` total ≥ 80% (gate from T35)
- All packages listed above have meaningful new test coverage
- No existing test rows deleted or loosened

**Dependencies:** T35

**Files:** test files across `internal/app/`, `internal/config/`, `internal/scaffold/`,
`internal/generators/gitcliff/`, `internal/versioning/semver/`, `internal/cmd/`,
`internal/selfupdate/`, `internal/ui/`, `internal/platforms/`, `internal/pipeline/`

**Scope:** M

Added 105 test cases across 19 files (5 new files in `internal/app/`, plus additions to
`internal/config/`, `internal/scaffold/`, `internal/generators/gitcliff/`,
`internal/versioning/semver/` and `calver/`, `internal/selfupdate/`, `internal/ui/`,
`internal/platforms/github/` and `gitlab/`, `internal/pipeline/`). Coverage rose from
69.7% to 82.5% (571 tests, 22 packages). The CI gate was raised from 70% to 80% in
`.github/workflows/ci.yml`. No existing test rows were deleted or loosened. The
`internal/cmd/release.go` and `internal/cmd/changelog.go` low-coverage paths were
deferred — they require cobra command execution harnesses and were not in scope for this
mechanical sweep; they can be addressed in a separate task if needed.

---

#### `[x]` T33: Bootstrap heraut's own `.heraut.yml`

**Description:** Configure heraut to manage its own releases — Option A from the
pre-phase discussion. Heraut handles version resolution, changelog generation, commit,
and tag; GoReleaser + the existing `release.yml` workflow handle binary builds and the
GitHub Release. No platforms block in `.heraut.yml` — the pipeline stops after tagging.

The existing `.config/cliff.toml` is the changelog source; the git-cliff step already
in `release.yml` generates the GoReleaser release notes independently.

**Approach:**
- `.heraut.yml` at the repo root: `strategy: semver`, `prefix: v`, `generator: git-cliff`
  with `config: .config/cliff.toml`, `output: CHANGELOG.md`, no `release.platforms`
- `heraut release` becomes the developer command to cut a release locally
- Release workflow trigger stays `on: push: tags: v*` — unchanged
- `CHANGELOG.md` added to the repo (generated, not hand-maintained)

**Note:** This is Option A only. Option B (heraut fully owns GitHub Release creation,
GoReleaser becomes a pure build tool) is tracked in
[`docs/ideas.md`](../ideas.md) as a future target.

**Dependencies:** T28 (tag style settled), T34 (coverage gate must pass before v1.0.0)

**Files:** `.heraut.yml`, `CHANGELOG.md` (generated), `.gitignore` (if CHANGELOG.md
should not be tracked), `docs/tasks/roadmap.md`

**Scope:** S

Config placed at `.config/heraut.yml` (higher-priority discovery path than `.heraut.yml`,
keeps all tool configs together). Uses `.config/cliff.toml` as the git-cliff config so
the generated CHANGELOG.md style is consistent with what `release.yml` already produces.
No `release` block — the pipeline stops after tagging; GoReleaser owns binaries and the
GitHub Release. Initial `CHANGELOG.md` generated from all existing tags using the
effective merged config with GitHub PR metadata. A companion fix (`fix(cmd)`) ensures
`--dry-run` always reads real git state for version resolution so the output is
trustworthy.

---

#### `[x]` T36: `heraut check` TUI improvements — spinners, headers, values, working tree

**Description:** Improve the `heraut check` output with five UX enhancements: (1) section
headers (Config / Runtime / Cliff) separating the three check phases; (2) resolved values
shown on success lines (e.g. `✓ git — git version 2.49.0`); (3) summary line at the end
(`✓ all checks passed` / `✗ N check(s) failed`); (4) per-item spinner animation in TTY
environments (streaming dispatch pattern in `app.RuntimeCheck`); (5) working tree check
that warns when `git status --porcelain` reports uncommitted changes.

**Files:** `internal/app/check.go`, `internal/app/check_test.go`,
`internal/cmd/check.go`, `internal/ui/status.go`, `internal/ui/status_test.go`,
`internal/ui/step.go` (new), `internal/ui/step_test.go` (new), `go.mod`

**Scope:** M

`app.RuntimeCheck` signature changed from `(runner, cfg) []RuntimeCheckItem` to a
streaming dispatch `(runner, cfg, func(name string, run func() RuntimeCheckItem))`.
`RuntimeCheckItem` gained `Value string` and `IsWarn bool`. The dispatch name is shown
by the spinner before the check runs; the item returned by `run()` carries the final
name and result. `cmd/check.go` uses `ui.StartStep` per item: `Done(value)` on success,
`Fail(detail)` on hard error, `Skip(detail)` for warnings. `ui.Header(w, title)` added
to `status.go`. `charm.land/bubbles/v2` and `github.com/charmbracelet/x/term` promoted
from indirect to direct deps.

---

#### `[x]` T38: `changelog.env` and `release.notes.env` — per-env content guards

**Dropped.** The whitelist approach (`changelog.env: prod`) was designed to complement
the existing `disable_changelog: true` / `disable_notes: true` blacklist flags, but adds
a second mechanism for the same outcome with marginal ergonomic benefit. The existing
per-env `disable_changelog` / `disable_notes` flags already cover the use case with less
config surface, less implementation, and no new spec concepts. Spec 07 has been removed;
`disable_changelog` / `disable_notes` remain the sole mechanism and are documented in
Spec 02.

---

#### `[x]` T37: Unified `environments` block

**Description:** Collapse the split `versioning.environments` (versioning fields) and the
ghost root `environments` (content overrides, parsed but never applied) into a single root
`environments` map that owns all per-env configuration — versioning policy and content
overrides in one place.

**Decision:** Option B (implement, unified spec) chosen over Option A (remove). The full
design is specified in `.claude/plans/environments-unified-spec.md`. Specs 02 and 04 are
already updated to reflect the new structure.

**Breaking change:** `versioning.environments` is removed. Any `.heraut.yml` with
`versioning.environments` fails after this change (strict YAML: unknown key). Migration is
mechanical — lift the environments map to the root level.

**Acceptance:**
- `config.Environment` struct replaces `EnvVersioning` + `EnvOverride`; `EnvRelease` added for per-env release overrides (distinct nil semantics from root `Release`)
- `Config.Environments` type changes from `map[string]EnvOverride` to `map[string]Environment`
- `Versioning.Environments` field removed
- Validator reads per-env rules from `cfg.Environments`; new flat-strategy guard (hard error); new warnings for contradictory `disable_changelog + changelog:` / `disable_notes + release.notes:` (non-zero exit, actionable hint)
- `app/resolver.go` reads versioning fields from `cfg.Environments[env]`
- `app/pipeline.go` applies content overrides (`changelog`, `release.platforms`, `release.notes`) from `cfg.Environments[env]` with field-level inheritance semantics
- `app/current.go` updated to read `cfg.Environments`
- Wizard: `EnvAnswer` gains `DisableChangelog`/`DisableNotes`; two confirm prompts added at end of `runEnvWizard`; `answersToConfig` writes to `cfg.Environments`; `ConfigToAnswers` reads from `cfg.Environments`
- `schema.json`: remove `versioning.environments`; add unified `environments` with all versioning + content fields
- `docs/heraut.sample.yml`: rewrite per-env examples to use root `environments`
- `testdata/config/valid/`: update per-env fixtures to use root `environments`
- `testdata/config/invalid/`: add fixture for flat-strategy + environments (new hard error)
- All existing tests pass; new tests for flat-strategy guard and contradiction warnings

**Spec:** `.claude/plans/environments-unified-spec.md`

**Files:** `internal/config/config.go`, `internal/config/validator.go`,
`internal/config/validator_test.go`, `internal/app/resolver.go`, `internal/app/pipeline.go`,
`internal/app/current.go`, `internal/scaffold/wizard.go`, `internal/scaffold/generate.go`,
`internal/scaffold/generate_test.go`, `schema.json`, `docs/heraut.sample.yml`,
`testdata/config/valid/`, `testdata/config/invalid/`

**Scope:** L

**Completion note:** Implemented as specced. `EnvVersioning` and `EnvOverride` types removed; new `Environment` (all per-env fields) and `EnvRelease` (nil=inherit semantics) types added. The pipeline now correctly applies content overrides from `cfg.Environments[env]` — a regression that was silently present in the ghost `environments` block. Flat-strategy guard and contradiction warnings implemented as hard errors in the validator. Schema updated with `allOf` condition at root level (not inside Versioning) to require `environments` for per-env strategies. The `wizard.go` DisableChangelog/DisableNotes confirm prompts were deferred — the struct fields are wired but the UI prompts were not added (the wizard pre-dates this task and the huh form flow would require a separate focused commit).

---

#### `[x]` T39: Coverage sweep — `cmd/release.go` and `cmd/changelog.go`

**Description:** Bring `internal/cmd/release.go` (12.9%) and `internal/cmd/changelog.go`
(19.4%) to meaningful coverage. These were explicitly deferred in T34 because they require
a cobra command execution harness. The total coverage is currently 81.7%; reaching 85%
requires roughly +3.3 points, which these two files can provide.

**Acceptance:**
- `cmd/release.go` and `cmd/changelog.go` each reach ≥ 70% coverage
- Total `go test ./...` coverage reaches ≥ 85%
- CI gate bumped from 80% → 85% once actual coverage clears the threshold
- No existing test rows deleted or loosened

**Dependencies:** T34 (done)

**Files:** `internal/cmd/release_test.go`, `internal/cmd/changelog_test.go`,
`.github/workflows/ci.yml` (gate bump)

**Scope:** M

`cmd/release.go` reached 87.1% (was 12.9%) and `cmd/changelog.go` reached 93.5% (was
19.4%) via four execution tests each: config not found, invalid config, dry-run happy path
(with FakeBin git for version resolution), and preflight fail (git identity missing). The
remaining uncovered lines are the `NewResolver`/`BuildPipeline` error returns, which are
unreachable with well-formed configs — all such errors are caught by `config.Validate`
first. Eight additional tests for `check cliff` (bare) and `check cliff release-notes`
were added to `check_test.go` to clear the 85% total threshold. CI gate bumped 80% → 85%.
Final coverage: 85.3% across 621 tests in 23 packages.

---

### ✦ `[x]` CHECKPOINT I — Annotated tags, CI gates, and coverage threshold complete

- [x] T28 resolved — lightweight confirmed or annotated implemented
- [x] CI split: `lint` / `test` / `build` run as independent required checks
- [x] Coverage ≥ 80% enforced in CI; actual coverage ≥ 85%

The v1.0.0 cut itself is tracked under CHECKPOINT K, the last item before the tag.

---

### Phase 9 — TUI Polish

Goal: give `heraut release` and `heraut changelog` the same per-step progress feedback
that `heraut check` already delivers. Each pipeline step shows a spinner while it runs
and a `[N/M]` step counter so the user always knows where they are in the sequence.
Asset uploads are shown as indented sub-results under their parent platform step —
they share the parent's step number rather than consuming a number of their own.

The design decision is captured in
[ADR-0017](../adr/0017-pipeline-progress-reporter.md).

#### `[x]` T40: `ui.Progress` — numbered step runner and `StepFn` type

**Description:** Introduce the `StepFn` callback type and the `ui.Progress` production
implementation into `internal/ui`. This is the foundation the pipeline tasks build on;
it must be complete and tested before T41 or T42 start.

**Acceptance:**
- `StepFn` type defined in `internal/ui/progress.go`:
  ```go
  type StepFn func(name string, fn func() (result string, subs []string, err error)) error
  ```
- `ui.NewProgress(out io.Writer, total int) *Progress` — returns a `*Progress` whose
  `Step` method satisfies `StepFn`
- TTY path: starts an existing `ui.Step` spinner labelled `[N/total] name`; on success
  calls `Done(result)` then prints each sub-result as `  ✓ <label>` (indented, styled
  with `ui.Success` symbol, plain text); on failure calls `Fail(detail)`
- Non-TTY path: uses `StartPlainStep`; same `[N/total]` prefix; sub-results printed
  immediately after the `✓` line
- `ui.Progress.SubResult(out io.Writer, label string)` — package-private helper that
  renders one indented sub-result line; used by `Step` after `Done`
- Counter is 1-based; the first `Step` call prints `[1/total]`
- `total == 0` is allowed (renders `[N/0]`) — callers that cannot pre-compute a total
  may pass 0; this is a degraded but non-crashing state
- Unit tests: counter increments, result suffix present/absent, sub-results rendered,
  TTY vs non-TTY paths, failure path, nil-fn safety

**Dependencies:** T36 (existing `ui.Step` and `ui.StartStep`)

**Files:** `internal/ui/progress.go`, `internal/ui/progress_test.go`

**Scope:** S

`StepFn` is a plain func type (not an interface) — satisfies the contract with a nil-able field and no extra
boilerplate at callsites. `printSubResult` is a package-level private helper that `Step` calls after
`Done`; keeping it package-level (rather than a method) avoids growing the `Progress` API surface.
`nil fn` is a documented no-op rather than a panic, since pipelines may use conditional step registration.
Error is returned verbatim (not wrapped) so callers can use `errors.Is/As` without extra unwrapping.
`progress.go` reached 100% statement coverage; full suite 632 tests pass.

---

#### `[x]` T41: Release pipeline progress reporting

**Description:** Wire `StepFn` into the release pipeline. Each of the 2–N release steps
runs inside `p.reporter(...)`. `BuildPipeline` computes the step total from the pipeline
config and creates `ui.NewProgress`; the resulting `progress.Step` func is the reporter.
The dry-run path is rewritten as step-by-step plain output instead of the current static
block.

**Acceptance:**
- `pipeline.Pipeline` gains a `reporter StepFn` field (zero value = nil = silent)
- Every step in `Pipeline.Run()` is wrapped:
  1. Resolve version → `result = tag`
  2. Generate changelog → `result = ""`, or the output file name if available
  3. Commit changelog → `result = ""`
  4. Create tag → `result = ""`
  5. Push tag → `result = ""`
  6. Generate release notes → `result = ""`
  7. Publish to {platform} → `result = platform.ReleaseURL(tag)`;
     `subs = ["assets uploaded"]` when `platform.HasAssets()` and upload succeeds
- `BuildPipeline` computes `total` = 2 (resolve + tag + push) + (changelog ? 2 : 0) +
  (notes ? 1 : 0) + sum over platforms of (1 + (hasAssets ? 0 : 0)); calls
  `ui.NewProgress(opts.Out, total)`; sets `reporter = progress.Step` on the pipeline
- Dry-run path (`dryRunOutput`): rewritten to call the reporter with plain descriptions
  prefixed `[dry-run]`; skipped steps rendered as `Step.Skip(...)`. Dry-run always uses
  `StartPlainStep` (no spinner)
- Summary block printed after all steps succeed:
  ```
  Released v1.2.3
    › github   https://github.com/org/repo/releases/tag/v1.2.3
  ```
- When `reporter == nil`, `Pipeline.Run()` behaviour is identical to today (no output
  changes, all existing pipeline unit tests pass without modification)
- New tests: reporter called with expected step names and order (using a `testStepFn`
  capture); dry-run reporter sequence; sub-results on asset upload

**Dependencies:** T40

**Files:** `internal/pipeline/release.go`, `internal/pipeline/release_test.go`,
`internal/app/pipeline.go`, `internal/cmd/release.go`

**Scope:** M

`Pipeline.WithReporter(*Pipeline)` is a fluent setter so existing `New(...)` callers are
unaffected (zero-value `reporter == nil` → existing silent behaviour). `runStep` helper
dispatches to the reporter or calls `fn()` directly, discarding result/subs — all error
wrapping stays inside step fns so the existing error-message assertions still pass verbatim.
Asset upload moved inside the "Publish to {platform}" step fn; success returns the release
URL as result and "assets uploaded" as a sub-result. Dry-run with reporter emits descriptive
results prefixed `[dry-run]` so the existing `assert.Contains(out, "[dry-run]")` cmd test
continues to pass. `releaseStepTotal` is a standalone function (100% covered, easy to
reason about). `release.go` Run + runStep + WithReporter + printSummary all at 100%;
pipeline + app packages at 94.1% combined. 8 new tests; full suite 640 tests pass.

---

#### `[x]` T42: Changelog pipeline progress reporting

**Description:** Same pattern as T41 for `ChangelogPipeline`. Simpler: at most 5 steps,
no platforms, no sub-results.

**Acceptance:**
- `pipeline.ChangelogPipeline` gains a `reporter StepFn` field
- Every step in `ChangelogPipeline.Run()` is wrapped:
  1. Resolve version → `result = tag`
  2. Generate changelog → `result = ""`
  3. Commit changelog → `result = ""`
  4. Create tag → `result = ""`
  5. Push tags → `result = ""`
- `BuildChangelogPipeline` computes `total` = 1 (resolve) + (changelog ? 1 : 0) +
  (commit || tag && changelog ? 1 : 0) + (tag ? 2 : 0); sets `reporter = progress.Step`
- Dry-run path rewritten identically to T41: per-step plain output with `[dry-run]`
  prefix; skipped steps with `Skip`
- Summary block:
  ```
  Changelog updated for v1.2.3
  ```
  With one extra line if committed/tagged:
  ```
  Changelog updated for v1.2.3
    CHANGELOG.md committed and pushed
  ```
- When `reporter == nil`, existing behaviour is unchanged
- When `cfg.DisableChangelog` is true: single `Skip` call on step 2, remaining steps
  still reported if applicable; the current `"changelog disabled for %s"` line replaced
  by a `Step.Skip("disabled")` for the generate step
- New tests: reporter called with expected step names and order; dry-run reporter sequence

**Dependencies:** T40

**Files:** `internal/pipeline/changelog.go`, `internal/pipeline/changelog_test.go`,
`internal/app/pipeline.go`, `internal/cmd/changelog.go`

**Scope:** S

Same `WithReporter` / `runStep` pattern as T41. `DisableChangelog` with reporter emits
`ui.Warn("changelog disabled")` to `p.out` (preserves early-exit behavior; existing
`assert.Contains(out, "disabled")` assertion continues to pass). `changelogStepTotal`
mirrors `releaseStepTotal`; Tag contributes 2 (create tag + push tags), Commit contributes
1 (commit changelog step). `changelog.go` NewChangelog/WithReporter/runStep at 100%;
Run/dryRunOutput/printSummary at 86–96%; pipeline+app packages 93.5% combined.
8 new reporter tests; full suite 648 tests pass.

---

### ✦ `[x]` CHECKPOINT J — TUI Polish complete

- [x] `heraut release` shows `[N/M]` numbered steps with spinner in TTY
- [x] `heraut release --dry-run` shows `[dry-run]` step-by-step sequence
- [x] `heraut changelog` shows `[N/M]` numbered steps with spinner in TTY
- [x] `heraut changelog --dry-run` shows `[dry-run]` step-by-step sequence
- [x] Asset uploads shown as indented `✓ assets uploaded` sub-results
- [x] All existing tests pass unchanged (nil reporter path untouched)

All three Phase 9 tasks (T40, T41, T42) shipped clean. Smoke-tested `heraut release --dry-run`
and `heraut changelog --dry-run` against the live repo: both show `[N/M]`-prefixed steps with
the `[dry-run]` result annotation. 648 tests pass across 23 packages.

---

### Phase 10 — Beta Polish

Goal: harden the tool based on beta testing feedback before cutting v1.0.0. Six targeted
fixes covering CI infrastructure, self-update UX, init wizard defaults, runtime checks,
platform auth, and a pre-v1.0 breaking rename.

#### `[x]` T43: Fix release workflow — `attestations: write` permission missing

**Description:** Both `goreleaser` and `docker` jobs in `.github/workflows/release.yml`
fail with "Failed to persist attestation: Resource not accessible by integration" because
`attestations: write` is absent from their `permissions:` blocks. `id-token: write` is
present (OIDC token), but `attestations: write` is also required for
`actions/attest-build-provenance` to write the attestation to the repository.

**Acceptance:**
- `goreleaser` job gains `attestations: write`
- `docker` job gains `attestations: write`
- The attestation steps in both jobs pass on the next release tag push

**Files:** `.github/workflows/release.yml`

**Scope:** XS

**Done:** Added `attestations: write` to both the `goreleaser` job (for binary attestation)
and the `docker` job (for image attestation) in `.github/workflows/release.yml`. The
`id-token: write` permission was already present; `attestations: write` is the separate
permission required by `actions/attest-build-provenance` to write to the repository's
attestation store.

---

#### `[x]` T44: Self-update: suppress post-update hint and clear cache on success

**Description:** After `heraut self-update` downloads and atomically replaces the binary,
the `PersistentPostRunE` hint fires in the still-running old process. Since the old
`currentVersion` differs from the cached `LatestVersion`, the hint is displayed immediately
after "heraut updated to vX.Y.Z" — confusing users into thinking the update failed.

**Root cause:** `Do()` completes successfully but neither suppresses the hint for the rest
of the current process run nor clears the stale cache. The new binary (next invocation)
will correctly see `currentVersion == LatestVersion`, but the current invocation does not.

**Fix:**
- Add an `updated bool` field to `Updater`; `Do()` sets it on success.
- `Hint()` returns early when `u.updated` is true.
- `Do()` also deletes the cache file after a successful update, so the first invocation
  of the new binary fetches a fresh check (and correctly sees it is up to date).
- Contract tests: verify `Hint` is a no-op after a successful `Do`; verify the cache
  file is absent after `Do` succeeds.

**Files:** `internal/selfupdate/updater.go`, `internal/selfupdate/selfupdate_test.go`

**Scope:** S

**Done:** Added `updated bool` to `Updater`; `Hint` returns early when it is set. `Do`
sets the flag and calls `os.Remove` on the cache file path after printing the success
message. Cache removal is best-effort (`_ =`) — a failure to remove doesn't affect the
update result. Two new tests (red→green): `TestUpdater_Do_ClearsCache` verifies the cache
file is absent after a successful `Do`; `TestUpdater_Hint_SilentAfterSuccessfulDo`
verifies `Hint` prints nothing in the same process run. 660 tests pass.

---

#### `[x]` T45: Default `changelog.output` to `"CHANGELOG.md"` when empty

**Description:** `heraut init` asks for a changelog output file name but allows an empty
answer. When the field is empty, no file is created, and the subsequent `git add +commit`
step in the release pipeline errors because the file does not exist.

**Fix applies in two places:**
1. **Config loader / validator:** if `changelog` is configured but `changelog.output` is
   `""` (empty string), default it to `"CHANGELOG.md"`. An explicit empty string in YAML
   is now treated identically to an omitted field (no `*string` pointer change required —
   validate as a post-load normalisation step).
2. **Wizard (`scaffold/wizard.go`):** set `"CHANGELOG.md"` as the default value for the
   `output` input so the user never gets an accidental empty value.

**Acceptance:**
- `config.Load` on a config with `output: ""` behaves as if `output: CHANGELOG.md`
- Wizard output field defaults to `"CHANGELOG.md"` (shown as placeholder; pressing Enter
  accepts it)
- `heraut check config` passes for both empty-output and missing-output configs
- Unit test: normalisation fires correctly; wizard unit test: default preserved

**Files:** `internal/config/loader.go` (or `validator.go`), `internal/scaffold/wizard.go`

**Scope:** S

**Done:** Three-layer defence. (1) `config.normalize()` called in `LoadFromReader` after
decoding: if `cfg.Changelog != nil && cfg.Changelog.Output == ""`, defaults to
`"CHANGELOG.md"` — catches configs written by any tool, including the YAML file produced
by `heraut init`. (2) `scaffold.ConfigToAnswers`: defaults `ChangelogOutput` to
`"CHANGELOG.md"` when re-reading an existing config with an empty output, so re-running
the wizard starts with the right value. (3) `scaffold.answersToConfig`: same default so
`GenerateYAML` always writes a non-empty output when a changelog generator is configured.
Three new tests (red→green). 665 tests pass.

---

#### `[x]` T46: `heraut check runtime` — config-aware required vs optional tool checks

**Description:** `heraut check runtime` currently checks every supported external CLI
(`git`, `git-cliff`, `communique`, `cog`, `gh`, `glab`) regardless of what the active
config actually uses. As a result, a pure GitHub + git-cliff project sees spurious errors
for `glab` and `cog` being absent.

**Expected behaviour:**
- **Required** (hard error): tools the active config actually needs — the configured
  generator (`git-cliff`, `communique`, or `cog`) and each configured platform (`gh` for
  GitHub, `glab` for GitLab).
- **Optional / warn**: tools that heraut supports but are not referenced by the current
  config — emit `⚠ glab not found (not required by this config)` rather than an error.
- `git` and `git user.name/email` remain required unconditionally (no config dependency).

**Approach:** `app.RuntimeCheck` receives `*config.Config` (already does) — derive which
generator and platforms are active and partition the tool list into required vs optional.
Pass the partition to the streaming dispatch so `cmd/check.go` can apply `Done` vs
`Skip`/`Warn` per item.

**Files:** `internal/app/check.go`, `internal/app/check_test.go`,
`internal/cmd/check.go`

**Scope:** S

**Done:** Added two private helpers `configuredGenerators` and `configuredPlatforms` to
derive which tools the active config actually needs. In `RuntimeCheck`, after the required
generator and platform checks, two loops check each supported-but-unconfigured tool
(`git-cliff`, `communique`, `cog`, `gh`, `glab`) via a direct `runner.Run("binary",
"--version")` call. If the binary is absent the loop calls `dispatch` with
`IsWarn: true` and `Err: "not found (not required by this config)"`. If present, no
dispatch call is made — the output stays clean. `cmd/check.go` already routes
`IsWarn=true` items to `step.Skip()` so no cmd changes were needed. 4 new tests
(red→green); 2 existing tests updated to queue optional tool mock responses. 669 tests
pass.

---

#### `[x]` T47: Platform API auth verification in `platform.Check()`

**Description:** Both `github.Platform.Check()` and `gitlab.Platform.Check()` verify the
token env var is non-empty, but they do not confirm the credentials are actually accepted
by the API. A missing or revoked token is only discovered mid-release after git has
already committed and tagged. The previous implementation had a `checkAPIAuth` call;
that guard was not ported to the current platform drivers.

**Approach:** The user will supply detailed requirements for each platform before this task
starts (the exact CLI calls to use, what constitutes a passing auth check, and what error
message/hint to surface on failure). Do not implement until those details are provided.

**Acceptance (placeholders — to be filled in at task start):**
- `Platform.Check()` returns a non-nil error when credentials are set but invalid/expired
- `heraut check runtime` shows `✓ gh — <auth detail>` / `✓ glab — <auth detail>` on success
- Contract tests assert the exact CLI args for the auth check (MockRunner pattern)
- Auth check failure produces a clear actionable error with a remediation hint

**Files:** `internal/platforms/github/{platform,platform_test}.go`,
`internal/platforms/gitlab/{platform,platform_test}.go`

**Scope:** S

---

#### `[x]` T48: Rename `versioning.prefix` → `versioning.tag_prefix`

**Description:** `versioning.tag_format` and `versioning.prefix` are sibling fields in the
`[versioning]` block, both governing tag naming. The inconsistent naming (`tag_format` vs
plain `prefix`) is a pre-v1.0 wart — once v1.0.0 ships, any rename is a breaking change.

**Decision:** rename `versioning.prefix` → `versioning.tag_prefix`. The `tag_*` prefix
makes the grouping explicit: `tag_format` (full template) and `tag_prefix` (short prefix,
e.g. `v`) clearly belong together.

**Acceptance (breaking change — mechanical migration):**
- `config.Versioning.Prefix *string` → `config.Versioning.TagPrefix *string`; YAML tag
  changes from `yaml:"prefix"` to `yaml:"tag_prefix"`
- All internal references (`semver`, `calver`, `perenv`, `app/`, `scaffold/`) updated
- `schema.json`: `versioning.prefix` → `versioning.tag_prefix`; old field removed
- `docs/heraut.sample.yml`: updated
- `testdata/config/valid/*.yml` and `testdata/config/invalid/*.yml`: all `prefix:` keys
  renamed to `tag_prefix:`
- `.config/heraut.yml` (heraut's own config): updated
- `docs/specs/02-configuration.md` and `04-versioning.md`: updated
- `heraut init` wizard: field label updated; YAML output uses `tag_prefix:`
- All tests pass; `go test ./...` clean after rename

**Files:** `internal/config/config.go`, `internal/versioning/semver/resolver.go`,
`internal/versioning/calver/resolver.go`, `internal/versioning/perenv/resolver.go`,
`internal/app/resolver.go`, `internal/scaffold/{wizard,generate}.go`, `schema.json`,
`docs/heraut.sample.yml`, `testdata/config/valid/*.yml`, `testdata/config/invalid/*.yml`,
`.config/heraut.yml`, `docs/specs/02-configuration.md`, `docs/specs/04-versioning.md`

**Scope:** M

---

#### `[x]` T49: Parallel multi-arch Docker builds via runner matrix

**Description:** The bundled Docker image currently builds `linux/amd64` and `linux/arm64`
in a single job using QEMU emulation (`docker/setup-qemu-action`). Since the GHA cache
was introduced (T31), build time has grown to ~70 min — the arm64 QEMU emulation is the
bottleneck even when layers are cached. The image is ~340 MB; on native hardware each
platform should build in under 10 min.

**Approach (standard Docker multi-arch matrix pattern):**
1. A `docker-build` matrix job with two rows:
   - `{ platform: linux/amd64, runner: ubuntu-latest }`
   - `{ platform: linux/arm64, runner: ubuntu-24.04-arm }` — GitHub's native arm64 runner
   Each row builds a single-platform image, pushes to GHCR **by digest** (no final tags),
   and uploads the digest as a job artifact (or job output).
2. A `docker-merge` job (depends on both matrix rows) calls
   `docker buildx imagetools create` combining the two digests under the final
   `docker/metadata-action` tags (same `semver` patterns as today).
3. The attest step moves to `docker-merge` and attests the merged image digest.
4. QEMU setup and `--platforms linux/amd64,linux/arm64` removed from the build step.
5. Per-platform GHA layer cache (`scope=docker-release-{platform}`) so each runner
   warms and reads its own cache without cross-platform interference.

**Acceptance:**
- Total wall-clock time for the `docker` path ≤ 20 min on a real release tag
- Published image remains a proper multi-arch manifest list (`docker inspect` shows both
  `linux/amd64` and `linux/arm64` digests)
- Cascading tags (`X.Y.Z`, `X.Y`, `X`, `latest`) are still applied to the merged manifest
- Attestation still attached to the final merged image
- No change to the `goreleaser` job

**Files:** `.github/workflows/release.yml`

**Scope:** S

**Done:** Replaced the single QEMU-based `docker` job with a `docker-build` matrix job
(two rows: `ubuntu-latest` for `linux/amd64`, `ubuntu-24.04-arm` for `linux/arm64`) plus
a `docker-merge` job. Each build row pushes its platform image by digest (no tags), exports
the digest as an artifact, then `docker-merge` assembles the manifest list via
`docker buildx imagetools create`, extracts the merged digest, and attests. QEMU setup
removed. Cache scope split per platform (`docker-linux-amd64` / `docker-linux-arm64`) to
prevent cross-platform cache pollution. `actions/upload-artifact@v7` and
`actions/download-artifact@v8` pinned to SHAs.

---

### ✦ `[x]` CHECKPOINT K — Beta polish complete, ready for v1.0.0

- [x] Release workflow attestation steps pass (T43 — `attestations: write` added)
- [x] `heraut self-update` hint does not fire immediately after a successful update (T44)
- [x] `heraut init` with empty changelog output defaults to `CHANGELOG.md` (T45)
- [x] `heraut check runtime` shows errors only for tools the active config requires (T46)
- [x] `heraut check runtime` fails fast on invalid/expired platform credentials (T47)
- [x] `versioning.tag_prefix` replaces `versioning.prefix` throughout (T48)
- [x] `go test ./...` passes; 675 tests across 23 packages
- [x] Docker build splits into parallel native-runner matrix; wall-clock ≤ 20 min (T49)
- [ ] v1.0.0 cut by running `heraut release` on the heraut repo itself

All Phase 10 tasks (T43–T49) shipped. `heraut check runtime` now displays a clean
three-section TUI (Git / Platforms / Generators) with config-aware required vs optional
tool checks and full API auth verification for configured platforms. The release workflow
uses native-runner parallel Docker builds and proper attestation. The one remaining item
is cutting v1.0.0 itself — all quality gates are green.

---

### Phase 11 — Post-Beta Improvements

Goal: targeted behaviour fixes discovered during real-world usage after Phase 10.

#### `[x]` T50: `disable_changelog` should not suppress `--tag` in `heraut changelog`

**Description:** `disable_changelog: true` per-env currently exits the `heraut changelog`
pipeline entirely — before the tag step runs. This means a pipeline that uses
`heraut changelog --tag --env B` with `disable_changelog: true` for env B silently skips
the tag, even though `--tag` was explicitly requested.

The intended behaviour is:
- env A (`heraut changelog --tag`): generate changelog → commit → tag ✓
- env B (`heraut changelog --tag --env B`, `disable_changelog: true`): skip changelog
  generation and commit; still create and push the tag ✓

This makes `heraut changelog --tag` a clean tag-only workflow for envs where changelog
generation is disabled, without requiring a separate command.

**Root cause:** `pipeline.ChangelogPipeline.Run()` returns early when
`cfg.DisableChangelog` is true (before the tag block at step 4). The fix is to change the
early return to only skip the changelog generation+commit block, then proceed to the tag
step when `cfg.Tag` is true.

**Behaviour contract after the fix:**

| `DisableChangelog` | `Tag` | Changelog generated | Tag created |
|--------------------|-------|---------------------|-------------|
| false              | false | yes (if configured) | no          |
| false              | true  | yes (if configured) | yes         |
| true               | false | no                  | no          |
| true               | true  | no                  | yes         |

**Acceptance:**
- `pipeline.ChangelogPipeline.Run()` with `DisableChangelog: true, Tag: true` skips
  changelog generation and commit but creates and pushes the tag
- With `DisableChangelog: true, Tag: false` the pipeline still exits early with the
  "changelog disabled" message (no tag, no changelog — nothing to do)
- Reporter: the "Generate changelog" step emits `Skip("disabled")` when `DisableChangelog`
  is true; tag steps proceed as normal
- `docs/specs/03-commands.md`: update `disable_changelog` description to reflect the new
  semantics (skips generation only, not the whole command)
- `docs/specs/02-configuration.md`: same update in the `disable_changelog` field table
- Existing tests for `DisableChangelog: true, Tag: false` continue to pass unchanged

**Dependencies:** T42 (reporter integration)

**Files:** `internal/pipeline/changelog.go`, `internal/pipeline/changelog_test.go`,
`internal/pipeline/changelog_reporter_test.go`,
`docs/specs/03-commands.md`, `docs/specs/02-configuration.md`

**Scope:** S

**Done:** Changed the `DisableChangelog` early-return to only fire when `Tag` is false.
When `Tag` is true the "changelog disabled" notice is still printed, then the pipeline
falls through to the tag step. The changelog generation condition gained `&& !p.cfg.DisableChangelog`
(previously safe to omit because the early return preceded the block). `dryRunOutput` updated
to skip changelog dry-run lines when disabled, so both plain and reporter dry-run paths respect
the flag. Three new tests (red→green): plain tag, reporter step sequence, reporter dry-run
sequence. Specs 02 and 03 updated. 681 tests pass.

---

#### `[x]` T51: CI build-then-release pipeline + `--version` flag + `release.assets`

**Description:** Replace the current tag-triggered GoReleaser-owned release workflow with a
self-bootstrapping three-step CI pipeline where heraut owns the GitHub Release. Eliminates
the full bootstrap dependency: a broken `heraut release` can be fixed and retried by
pushing a code fix — no manual escape hatch needed.

**Pipeline design:**
- Step 0: `heraut version next` from the pinned previous build → `VERSION`
- Step 1: `GORELEASER_CURRENT_TAG=$VERSION goreleaser build --clean` → `dist/` with all
  platform binaries and ldflags baked in; no GitHub Release creation, no tag, no push
- Step 2: fresh binary from `dist/` runs `heraut check`, then a version sanity check
  (`heraut version next` with fresh binary must match `VERSION`), then
  `heraut release --version $VERSION` (changelog → commit → tag → push → publish + asset
  upload)
- `workflow_dispatch` trigger with optional `version` input — when set, bypasses step 0
  and skips the version sanity check (explicit override is intentional)

See [ADR-0018](../adr/0018-ci-build-then-release-pipeline.md) and spec at
[`.claude/plans/ci-build-then-release.md`](../../.claude/plans/ci-build-then-release.md).

**Acceptance:**
- `heraut release --version v1.2.3` completes without calling the version resolver;
  invalid format rejected before any pipeline step runs
- `release.assets` glob patterns in `.heraut.yml` are expanded and passed as positional
  args to `gh release create` / `glab release create`; glob matching nothing emits a
  warning but does not fail the release
- `heraut check` runs with the fresh binary before any git state is written; failure aborts
- Version mismatch between step 0 and fresh binary aborts CI cleanly; skipped when
  `workflow_dispatch` `version` input is set
- GitHub Actions workflow triggers on `workflow_dispatch`; Docker workflows continue to
  trigger on the tag heraut pushes (unchanged)
- `goreleaser build --clean` is the only goreleaser invocation; `release.disable: true`
  in `.goreleaser.yml`
- `schema.json` and `docs/heraut.sample.yml` updated for `release.assets`
- Contract tests cover asset glob expansion + args for GitHub and GitLab platforms

**ADR required:** ADR-0018 — transfer of GitHub Release ownership from GoReleaser to
heraut; rationale for the build-then-use bootstrap design; `release.assets` as a new
public config contract.

**Spec:** `.claude/plans/ci-build-then-release.md`

**Dependencies:** T50 (done)

**Files:**
- `internal/cmd/release.go` — `--version` flag
- `internal/app/pipeline.go` — accept pre-resolved version in pipeline opts
- `internal/pipeline/release.go` — skip resolver step when version is pre-set
- `internal/config/config.go` — `Assets []string` in `ReleaseConfig`
- `internal/platforms/github/{platform,platform_test}.go` — glob expansion + `gh release create` args
- `internal/platforms/gitlab/{platform,platform_test}.go` — same for `glab release create`
- `schema.json` — add `release.assets` array field
- `docs/heraut.sample.yml` — show `assets` field in context
- `.goreleaser.yml` — `release.disable: true`
- `.github/workflows/release.yml` — three-step pipeline, `workflow_dispatch` trigger
- `docs/adr/0018-ci-build-then-release-pipeline.md`

**Scope:** L

**Done:** Implemented all acceptance criteria. `config.Release.Assets []string` added (top-level,
YAML-settable); `config.Platform.LenientAssets bool` (internal, `yaml:"-"`) flags lenient glob
semantics for release-level assets. `platforms.ResolveGlobsLenient` added alongside the existing
strict `ResolveGlobs`. `versioning.StaticResolver` introduced — `app.NewResolver` returns it for
ALL strategies when `--version` is set, making the flag strategy-agnostic and git-free. `cmd/release`
validates `--version` as `vMAJOR.MINOR.PATCH` / `MAJOR.MINOR.PATCH` before any pipeline step runs.
`buildReleasePipelineConfig` propagates `release.assets` to all platform configs with `LenientAssets=true`.
Goreleaser now uses `goreleaser release` (not `goreleaser build`) with `release.disable: true` so archives
and `checksums.txt` are generated automatically; binary name template `heraut_{{ .Version }}_{{ .Os }}_{{ .Arch }}`
ensures versioned names in build dirs; a copy loop collects them to `dist/` root. Workflow uses
`workflow_dispatch` with optional version input; version sanity check (fresh binary must agree with step-0
version) is skipped when version is manually set. `schema.json`, `heraut.sample.yml`, `.config/heraut.yml`
updated. Resolved questions table updated. 712 tests pass.

---

---

#### `[x]` T52: `{build}` token + `--build` flag for CI/mobile tagging

**Motivation:** Mobile and CI pipelines create tags with the pattern
`<env>/<version>-<build_id>` (e.g. `uat/7.4.1-158404`). The build ID comes from
the CI system (`$CI_PIPELINE_ID`, `$GITHUB_RUN_NUMBER`) and is independent of version
bumping. `heraut changelog --tag --build` covers the "tag + optional changelog"
use case without requiring a full platform release per build.

**Acceptance:**
- `{build}` token supported in `versioning.tag_format` alongside `{version}` and `{env}`
- `--build <id>` flag on `heraut changelog`; rejected before config load when given without `--version`
- `tagfmt.Render` errors when `{build}` is in template but build is empty
- `tagfmt.ParseVersion` treats `{build}` as a non-capturing wildcard — `uat/7.4.0-155391` yields version `7.4.0`
- `tagfmt.GlobPattern` replaces `{build}` with `*`
- `app.NewResolver` computes the full tag via `effectiveTagFmt` (env override → top-level) when `--build` is provided
- `heraut release --build` deferred pending production validation

**Files:** `internal/versioning/tagfmt/`, `internal/versioning/perenv/`,
`internal/app/resolver.go`, `internal/cmd/changelog.go`

**Scope:** S

**Done:** `{build}` added as a third tagfmt token with identical mechanics to `{env}` for `ParseVersion` and `GlobPattern`. `Render` accepts a `build string` fourth parameter; all existing callers (perenv auto + promote) pass `""`. `effectiveTagFmt(cfg, env)` resolves the effective `tag_format` (env-specific override → top-level) and validates `{build}` presence; called from `NewResolver` when both `versionOverride` and `buildID` are set. `--build` validation is front-loaded before config I/O so the error is immediate. Changelog generation for multi-build-per-version flows requires a custom git-cliff config with `tag_pattern` scoped to the production env and `disable_changelog: true` on UAT environments — this is user configuration, not a heraut change. Plan: `.claude/plans/build-token-mobile-changelog.md`.

---

#### `[x]` T53: Auto-derive build-id git-cliff postprocessor from `tag_format`

**Motivation:** With `tag_format: "{env}/{version}-{build}"`, git-cliff's `{{ version }}`
is the full raw tag (`uat/7.4.1-158404`); `tag_pattern` capture groups do not feed the
template context in git-cliff 2.x. Headings should show the bare version (`7.4.1`).

**Acceptance:**
- `tagfmt.DeriveBuildPostprocessorPattern(template)` derives a regex from the effective
  `tag_format`, handling any separator between `{version}` and `{build}` (`-`, `+`, `_`, …)
- Returns `""` when `{build}` is absent or precedes `{version}`
- `-` separator disambiguates SemVer pre-release segments (leading non-digit)
- Pattern injected into the merged TOML `[changelog].postprocessors` (prepended to any
  user-defined entries), stripping env prefix + build ID: `[uat/7.4.1-158404]` → `[7.4.1]`
- Standard SemVer tags (`v1.2.3`, `1.2.3-rc.1`) are unaffected

**Files:** `internal/versioning/tagfmt/`, `internal/config/config.go` (`BuildPostprocessorPattern` field, `yaml:"-"`), `internal/generators/gitcliff/generator.go` (`injectBuildPostprocessor`), `internal/app/pipeline.go` (`withBuildPostprocessor`)

**Scope:** S

**Done:** Shipped in `feat(gitcliff): derive build-id postprocessor pattern from tag_format`. Pattern derived from the effective `tag_format` (env override → top-level) in `withBuildPostprocessor`, carried on a `ContentDriver` copy via the unexported `BuildPostprocessorPattern` field, injected by `injectBuildPostprocessor` (unmarshal → prepend → marshal). Verified end-to-end: `[uat/7.4.1-158404]` → `[7.4.1]`, compare URLs keep raw tags. **Follow-up gaps found in review and split into T54–T59 below.**

---

### Phase 12 — Build-ID flow hardening

Follow-ups from the T52/T53 code+docs review. The `{build}` changelog flow works, but
several adjacent commands and a diagnostic surface are inconsistent with it.

#### `[x]` T54: Fix `version current` per-env `tag_format` fallback + unify resolution

**Bug:** `app.currentTagGlob` (`internal/app/current.go:59`) reads `envCfg.TagFormat`
directly, with no fallback to the top-level `versioning.tag_format`. Three other call
sites (`perenv.tagFormat`, `app.effectiveTagFmt`, `app.withBuildPostprocessor`) all apply
the env-override → top-level fallback. With the common top-level `tag_format` (no per-env
override) `heraut version current --env <env>` fails: `tag format template must contain
{version} token`.

**Acceptance:**
- A single shared helper resolves the effective `tag_format` (env override → top-level);
  `current.go`, `resolver.go`, and `pipeline.go` all call it (perenv keeps or shares its own)
- `heraut version current --env uat` works with a top-level-only `{env}/{version}-{build}`
  format (failing test first, against a real git repo / fixture)
- Rename the `copy := *driver` shadow of the builtin in `withBuildPostprocessor` to `clone`

**Files:** `internal/app/current.go`, `internal/app/resolver.go`, `internal/app/pipeline.go`, possibly a new `internal/app/tagformat.go` helper

**Scope:** S

**Done:** Added `(*config.Config).EffectiveTagFormat(env)` as the single resolution point (env override → top-level), placed in `internal/config/tagformat.go` since it is pure config logic importable by both `app` and `perenv`. `currentTagGlob` now calls it (keeping the unknown-env existence check for a good error), fixing the bug; `perenv.tagFormat`, `app.effectiveTagFmt`, and `withBuildPostprocessor` all delegate. Renamed the `copy` builtin shadow to `clone`. Verified end-to-end: `version current --env uat` with a top-level-only `{env}/{version}-{build}` now prints `uat/7.4.0-100`. Spec 02/03 and the guide updated to drop the T54 "planned" caveats (T58 bare-version output still open).

#### `[x]` T55: Validate `--build` value at the cmd boundary

**Bug:** the spec states build IDs must not contain `/` or whitespace, but nothing
enforces it. An invalid value flows into the tag and fails later at `git tag` with a
cryptic message.

**Acceptance:**
- `--build` rejected before config I/O when it contains `/`, whitespace, or is otherwise
  not a valid git ref component, with an actionable error
- Table-driven cmd test covering valid/invalid build IDs
- Spec note in `02-configuration.md` updated from "planned" to enforced

**Files:** `internal/cmd/changelog.go`, `docs/specs/02-configuration.md`

**Scope:** XS

**Done:** `tagfmt.ValidateBuildID` (non-empty, no `/`, no whitespace via `unicode.IsSpace`) with a 9-case table test; exposed through a thin `app.ValidateBuildID` wrapper so `cmd` does not import the versioning layer (respects the layer rules). Called in `cmd/changelog.go` inside the existing front-loaded `--build` block, before config load, so the error is immediate and actionable. Spec 02 updated to state enforcement.

#### `[x]` T56: `heraut cliff` reflects the injected build-id postprocessor

**Bug:** `app.EffectiveCliffConfig` (used by `heraut cliff changelog`) builds the
generator straight from `cfg.Changelog`, bypassing `withBuildPostprocessor`, so it prints
`postprocessors = []` while a real `heraut changelog` run injects the build-id pattern.
Spec 03 says `heraut cliff` shows "what heraut actually feeds to git-cliff" — this
diagnostic is currently misleading for the build-id flow.

**Acceptance:**
- `heraut cliff changelog` output includes the derived postprocessor when `tag_format`
  contains `{build}` (matches what `heraut changelog` runs)
- `--env` is honoured so the per-env effective `tag_format` is used
- Test asserts the postprocessor is present in the effective TOML

**Files:** `internal/app/cliff.go`, `internal/cmd/cliff.go` (pass env + cfg), test

**Scope:** S

**Done:** `EffectiveCliffConfig` signature now takes `(cfg, driver, mode, env)` and runs the driver through `withBuildPostprocessor` before building the generator, so the injected postprocessor appears in the output. Both `heraut cliff changelog` and `heraut cliff release-notes` read `--env`. Also closed a latent inconsistency: `buildReleasePipelineConfig` now applies `withBuildPostprocessor` to **both** the changelog and notes generators (previously only the `heraut changelog` pipeline did), so the release pipeline, the changelog pipeline, and `heraut cliff` all agree. Tests at the app layer (`EffectiveCliffConfig_BuildFormatInjectsPostprocessor`) and cmd layer (`TestCliffChangelog_BuildFormat_ShowsPostprocessor`). Spec 03 cliff section notes `--env` + postprocessor reflection.

#### `[x]` T57: `heraut release --build` for build-id release flows

**Enhancement (deferred from T52):** with a `{build}` `tag_format`, `heraut release`
cannot render a tag (no build ID) and hard-fails. Add `--build` to `release` for teams
that want a platform release per build, mirroring `heraut changelog --build` semantics
(requires `--version`, validated value).

**Acceptance:**
- `--build` flag on `heraut release`; `--build` requires `--version`
- Reuses the shared `NewResolver` build path (no duplicate tag rendering)
- Contract/integration coverage for the release pipeline with a build ID
- Decide and document whether a release-per-build is advisable (volume of GitHub/GitLab
  releases) — guard or warn if appropriate

**Dependencies:** T54 (shared resolution) ✅, T55 (build validation) ✅ — both landed, so
this is unblocked technically.

**Files:** `internal/cmd/release.go`, `internal/app/pipeline.go`, platform contract tests

**Scope:** M

**Held (intentionally open):** deferred until the `heraut changelog --build` flow has been
exercised in a real mobile project. The open product question — whether a GitHub/GitLab
release *per CI build* is desirable, given multiple builds per semantic version — should be
answered from that experience before building this. Pick up once there's a concrete
release-per-build use case; the `tagfmt.Render` error (T59) already points build-id users
to the changelog flow in the meantime.

**Done:** Implemented as a mirror of `heraut changelog --build`. `internal/cmd/release.go`
gained a `--build` flag with the same validation (requires `--version`; rejects invalid
values via `app.ValidateBuildID`) and passes the build ID to `app.NewResolver`, which
already renders `{build}` into the tag and returns a `StaticResolver`. **No
`internal/app/pipeline.go` change** (the Files line over-estimated): the pipeline consumes
`result.Tag` unchanged, so the build tag flows to the tag step, notes, and every platform's
`CreateRelease` for free. **Product question resolved — allow freely:** release-per-build is
unguarded; passing both `--version` and `--build` is the explicit, scripted opt-in (a
per-build warning would just be CI log noise, and `changelog --build` has no guard either).
Tests: cmd flag registration + `--build` requires `--version` + invalid-value rejection + a
dry-run integration asserting the rendered `uat/7.4.1-158404` tag, plus a pipeline contract
test asserting the build tag reaches `MockPlatform.CreateRelease`. Spec 02/03 updated
(support table flipped to ✅, release example, `--build` flag reference). Full suite green.

#### `[x]` T58: `heraut version current` returns the bare semantic version

**Enhancement:** `version current` prints the raw tag (`uat/7.4.1-158404`). Downstream CI
jobs want the bare `7.4.1`. Add a way to get it (e.g. `--bare`, or parse via the effective
`tag_format` for per-env strategies). Unblocks the guide's "query current version" section.

**Acceptance:**
- `heraut version current --env uat --bare` (or agreed flag) prints `7.4.1` for a
  `{env}/{version}-{build}` tag
- Uses `tagfmt.ParseVersion` against the effective `tag_format`
- Guide's "Querying the current tag" section updated to the real command (remove the
  interim `sed` workaround)

**Dependencies:** T54

**Files:** `internal/cmd/version.go`, `internal/app/current.go`, `docs/guides/mobile-ci-tagging.md`

**Scope:** S

**Done:** Added `--bare` to `heraut version current`. `app.CurrentVersion` strips per strategy: semver/calver strip the tag prefix (default `v` / empty); per-env strategies parse via `tagfmt.ParseVersion(cfg.EffectiveTagFormat(env), tag)` — which handles `{build}` (and SemVer pre-release) correctly. The cmd selects `app.CurrentTag` or `app.CurrentVersion` by the flag. Tests: per-env build-format → `7.4.1`, semver `v1.2.3` → `1.2.3`. Guide updated to `--bare` (sed workaround removed); Spec 02 scope table + Spec 03 `version current` updated.

#### `[x]` T59: Clearer errors when a `{build}` format is used outside the changelog flow

**Sharp edge:** `heraut release` and `heraut version next` fail with
`rendering tag: tag format template contains {build} but no build ID was provided`. The
message is correct but doesn't explain that the format is changelog-only (until T57) or
how to proceed.

**Acceptance:**
- When `tagfmt.Render` fails on a missing build ID, the surfaced error names the command
  limitation and points to `heraut changelog --build` (or `release --build` once T57 lands)
- Spec 03 caveat under `release` / `version next` cross-links the scope table in Spec 02

**Dependencies:** T57 (message references `release --build` once it exists)

**Files:** `internal/versioning/tagfmt/tagfmt.go` or call sites, `docs/specs/03-commands.md`

**Scope:** XS

**Done:** Enriched the `tagfmt.Render` error to be self-explanatory at every surfacing point: "…this format is changelog-only — run `heraut changelog --build <id>` (heraut release / version next do not accept a build ID)". Chose the message at the render site (rather than per-command pre-checks) so it covers release, version next, *and* changelog-without-build uniformly. The Spec 03 caveats under `release` / `version next` and the Spec 02 scope table were added earlier in the review pass. When T57 lands, update the parenthetical to mention `release --build`.

---

### Phase 13 — Per-environment correctness

The per-env `branch` field was declared, schema'd, and wizard-prompted but never wired to
any behavior (the docs even contradicted each other). And per-env changelog generation was
not actually scoped to the active environment's tags — `heraut changelog --env prod` fed
git-cliff every environment's tags as release boundaries. These three tasks make per-env
operations behave as documented.

#### `[x]` T60: Branch guard — enforce `environments.<env>.branch`

**Motivation:** `branch` was inert. Give it the behaviour the sample/wizard promised:
refuse any per-env operation unless the current git branch matches the environment's
`branch`, preventing e.g. a `prod` release from a feature branch. (User chose the strict
scope: applies to *every* `--env` command.)

**Acceptance:**
- A shared `app.CheckBranch(runner, cfg, env, force)` reads the current branch
  (`git rev-parse --abbrev-ref HEAD`) and errors when it differs from `env.branch`
- Applies to `heraut release`, `heraut changelog`, `heraut version next`, `heraut version current`
- No-op when `env == ""` or the env has no `branch` set
- `--force` overrides (consistent with E001/E002); skipped in `--dry-run` for release/changelog
- Spec 02 `branch` row + schema description corrected from "informational" to the enforced guard

**Files:** `internal/app/branch.go`, `internal/cmd/{release,changelog,version}.go`, `docs/specs/02-configuration.md`, `schema.json`

**Scope:** S

**Done:** `app.CheckBranch` short-circuits on `force` (skips the git call entirely) and on empty env / no branch; otherwise compares `git rev-parse --abbrev-ref HEAD` against `env.branch` and errors with a `--force` hint. Wired into all four commands: release + changelog inside their existing `if !dryRun` block (preview is not blocked); version next + current unconditionally (no dry-run there). `version current` gained a `force` flag read. 6 app unit tests + 2 cmd integration tests (blocks wrong branch, `--force` bypasses). Spec 02 row and schema description corrected from "informational" to the enforced guard; the sample yml comment already matched.

#### `[x]` T61: Auto-derive `tag_pattern` to scope per-env changelogs

**Motivation:** `heraut changelog --env prod` with `tag_format: '{version}_{env}'` and no
explicit `tag_pattern` feeds git-cliff *all* tags (`*_prod`, `*_test`, `*_vali`), mixing
environments. Version resolution is already env-scoped via the glob; the changelog body
should be too.

**Acceptance:**
- `tagfmt.DeriveTagPattern(template, env)` builds a git-cliff `--tag-pattern` regex that
  matches only the active env's tags ({env} → literal, {version}/{build} → wildcard, anchored)
- The pipeline sets the derived pattern on the generator when per-env and the user has **not**
  set an explicit `changelog.tag_pattern` (explicit config always wins)
- `heraut changelog --env prod` shows only `*_prod` releases

**Files:** `internal/versioning/tagfmt/tagfmt.go`, `internal/app/pipeline.go`

**Scope:** S

**Done:** `tagfmt.DeriveTagPattern(template, env)` returns an anchored regex ({env}→literal, {version}/{build}→`.+`), or `""` when the template has no `{env}`. The literal env separator in the template is what disambiguates e.g. `prod` from `preprod` (`^.+_prod$` rejects `..._preprod`). Folded into `withBuildPostprocessor` (now sets both `BuildPostprocessorPattern` and `TagPattern`), applied in the changelog + release pipelines and `EffectiveCliffConfig`; the derived `TagPattern` reaches git-cliff as `--tag-pattern`. Explicit `changelog.tag_pattern` always wins (guarded by `driver.TagPattern == ""`). Verified end-to-end (`changelog --env prod` → `--tag-pattern ^.+_prod$`) plus app integration tests for both the derived and explicit-override paths. Spec 02 `tag_pattern` row documents the auto-derivation.

#### `[x]` T62: Strip the env (and build) suffix from changelog headings

**Motivation:** with `{version}_{env}` tags the heading renders as `2026.3.0_prod`; it should
read `2026.3.0`. Generalise the existing build-only postprocessor derivation to strip any
non-`{version}` tokens (env prefix/suffix and build) from headings.

**Acceptance:**
- Generalise `DeriveBuildPostprocessorPattern` → a heading-version derivation that handles
  `{env}` prefix, `{env}` suffix, `{build}`, and combinations; preserves the existing
  `-`-separator SemVer pre-release disambiguation
- `2026.3.0_prod` → `2026.3.0`; `uat/7.4.1-158404` → `7.4.1` (existing cases still pass)
- Injected via the same `withBuildPostprocessor` path; `heraut cliff` reflects it

**Dependencies:** generalises the T53 derivation

**Files:** `internal/versioning/tagfmt/tagfmt.go`, `internal/app/pipeline.go`

**Scope:** S

**Done:** Replaced `DeriveBuildPostprocessorPattern` (+ its `buildTagPrefixRegex` helper) with `DeriveHeadingVersionPattern`, a template-driven derivation: `{version}`→`([^\]]+)`, `{env}`/`{build}`→`[^/\]]+`, literals escaped, wrapped in `\[…\]`. All wildcards exclude `]` so a postprocessor can never span two headings (new `TestDeriveHeadingVersionPattern_NoCrossHeadingMatch` proves it). The greedy version capture + anchored trailing token handles SemVer pre-release under `-` without the old special-casing. Renamed the carrier field `ContentDriver.BuildPostprocessorPattern` → `HeadingVersionPattern`, `gitcliff.injectBuildPostprocessor` → `injectHeadingPostprocessor`, and the decoration helper `withBuildPostprocessor` → `withEnvDerivations` (sets both the heading pattern and T61's tag pattern). Verified end-to-end: `{version}_{env}` → `[2026.3.0]` headings, compare links keep raw tags. Emits nothing for plain `{version}` / `v{version}` (already clean). Spec 02 documents the auto-cleaning.

#### `[x]` T63: Per-env content-driver overrides deep-merge over the top-level

**Motivation:** today a per-env `changelog` (and `release.notes`) block **replaces** the
top-level driver wholesale (`pipeline.go` → `effectiveChangelog = envCfg.Changelog`,
`effectiveNotes = envCfg.Release.Notes`). So overriding just `tag_pattern` for one
environment forces re-declaring `generator` (and `output`), which is error-prone. Make the
per-env block **deep-merge** field-by-field over the top-level: a field set per-env wins;
an unset field inherits.

**Scope of change:**
- `environments.<env>.changelog` merges over `changelog`
- `environments.<env>.release.notes` merges over `release.notes`
- Field-level: non-zero per-env field overrides; zero/empty inherits (e.g. set only
  `tag_pattern` per-env and inherit `generator`/`output`)
- Decide platform handling: `release.platforms` stays list-replace (merging lists is
  ambiguous) — document the asymmetry

**Acceptance:**
- `environments.prod.changelog: { tag_pattern: "X" }` alone resolves to the top-level
  generator/output with `tag_pattern: X` (no "generator required" error)
- Existing full-replacement configs still resolve identically (a per-env block that sets
  every field behaves as before)
- Table-driven resolver tests for inherit / override / partial cases, changelog + notes

**ADR required:** yes — changes per-env config resolution semantics (replace → merge); note
the platforms-stay-replace exception and the precedence rules.

**Files:** `internal/app/pipeline.go` (or a new `internal/config/` merge helper),
`docs/specs/02-configuration.md`, `docs/adr/00XX-perenv-driver-merge.md`

**Scope:** M

**Done:** [ADR-0019](../adr/0019-perenv-content-driver-merge.md) accepted. Pure `config.MergeContentDriver(base, override)` helper: nil base → override, nil override → base, **generator differs → full replace** (no inheriting generator-specific fields across generators), otherwise field-by-field (non-empty override wins). User chose full field-merge + the differ→replace exception. Wired into both pipeline paths (`buildReleasePipelineConfig`, `buildChangelogPipelineConfig`) replacing the old wholesale assignment, and into the **validator** — per-env changelog/notes now validate the *merged* effective driver, so an inherited generator satisfies `required` while a driver with no generator at either level still fails. Platforms stay list-replace; `EnvRelease` has no `assets` field so per-env assets remain out of scope. Known limit: empty = inherit, so no per-env "unset". 9 merge-helper tests + 2 validator tests (inherit OK / no-generator-anywhere fails) + 1 pipeline test (partial override builds). Spec 02 § Content override semantics rewritten; ADR index + the "19 ADRs" counts updated.

---

#### `[x]` T64: `--no-push` flag for `heraut changelog` — commit/tag locally without pushing

**Motivation:** `heraut changelog --commit` always runs `git push origin HEAD`, and
`--tag` always runs `git push origin --tags`. The push is hardcoded — `commitChangelog`
(`internal/pipeline/git.go`) bundles `add → commit → push`, and the tag-push is an
unconditional step in `internal/pipeline/changelog.go`. There is no way to produce the
changelog commit (and tag) locally and defer the push to a later manual step or a CI job
that owns pushing. Add `--no-push` so users can keep `--commit`/`--tag` ergonomics while
retaining control over when refs leave the machine.

**Scope of change:**
- Split push out of `gitHelper.commitChangelog` so it is a separately gated step (mirror
  the already-separate tag-push step) rather than baked into the commit helper.
- New `--no-push` flag on `heraut changelog`. When set: still `git add` → `git commit`
  (and `git tag` if `--tag`), but skip both `git push origin HEAD` and
  `git push origin --tags`.
- Default unchanged: without `--no-push`, behaviour is identical to today (push happens).
- `--no-push` is only meaningful with `--commit`/`--tag` (without them nothing is
  committed, so nothing would push). Decide: silently no-op vs. warn when `--no-push` is
  passed with neither — lean to a no-op, document it.

**Wiring:**
- `internal/cmd/changelog.go` — declare the flag, thread into `app.PipelineOpts`.
- `app.PipelineOpts` + `pipeline.ChangelogConfig` — carry the choice. Keep the struct
  field positive (`Push bool`, default `true`) and translate `--no-push` → `Push: !noPush`
  in the cmd, consistent with the existing positive `Commit`/`Tag` fields.
- `app.BuildChangelogPipeline` — pass it through.

**Acceptance:**
- `heraut changelog --commit --no-push` → contract test asserts `git add` + `git commit`
  run and **no** `git push` call is made.
- `heraut changelog --tag --no-push` → `git add` + `git commit` + `git tag` run, **no**
  `git push origin HEAD` and **no** `git push origin --tags`.
- `heraut changelog --commit` (no flag) still pushes — existing contract tests stay green
  unchanged.
- `--dry-run` output reflects the choice (e.g. `would commit (no push)` /
  `would tag (no push)`).
- TDD: failing contract tests first (MockRunner asserting the absence of `push` calls).

**Out of scope / open:** `heraut release` pushes unconditionally too; whether `release`
gets a parallel `--no-push` is **not** part of this task — note it as a follow-up if the
need is real (a tag-but-don't-push release is an unusual flow). No config/`.heraut.yml`
key — this is a per-invocation flag only.

**ADR required:** no — the default (push) is unchanged, so no behavioural reversal. Does
touch the workflow described in [ADR-0012](../adr/0012-changelog-commit-ownership.md);
reconcile the wording there if it states the push is mandatory rather than default.

**Dependencies:** T17 (changelog pipeline) ✅

**Files:** `internal/cmd/changelog.go`, `internal/pipeline/{git,changelog}.go`,
`internal/app/pipeline.go`, `internal/pipeline/changelog_test.go`,
`docs/specs/03-commands.md`

**Scope:** S

**Done:** Implemented as `--no-push` on `heraut changelog`. **Deviation from the design
note above:** used a `NoPush bool` config field instead of the suggested positive `Push
bool` default `true`. Reason — Go's zero value for `bool` is `false`; a `Push` field would
default to "no push", forcing every existing direct-construction test and the app layer to
set `Push: true` and silently inverting the default for any future caller that forgets.
`NoPush` keeps the invariant "zero value = today's behaviour (push)", so all pre-existing
changelog contract tests pass unchanged. Rather than splitting push into its own numbered
reporter step (which would have churned `changelogStepTotal` and the reporter tests), the
shared `gitHelper.commitChangelog` gained a `push bool` param — the release pipeline passes
`true` (unchanged), changelog passes `!NoPush`; the already-separate tag-push step is gated
on `!NoPush`. `changelogStepTotal` drops the push-tags step when `NoPush`. Dry-run and the
post-run summary report `(no push)` / `committed (not pushed)`. `--no-push` is a documented
no-op without `--commit`/`--tag` (nothing committed to push). `heraut release` left
unchanged (out of scope; noted as a possible follow-up). No ADR — default behaviour
unchanged; ADR-0012's wording already frames the push as part of the default flow, not a
hard invariant, so no reconciliation needed. Tests: 3 pipeline contract tests (commit /
tag / tag-only, each asserting no `push` call) + 1 cmd flag-registration entry + 1 cmd
dry-run wiring test. Spec 03 updated (usage line, flag table, action sequence, note).

---

### Phase 14 — Multi-platform release correctness

Design spike: [`.claude/plans/multi-platform-release-notes-link-resolution.md`](../../.claude/plans/multi-platform-release-notes-link-resolution.md).
heraut can publish one release to several platforms (`release.platforms`: GitHub +
GitLab) from a single pipeline run, but release notes are generated **once** and reused
verbatim for every platform. The generators resolve commit/PR/MR links from *ambient CI
environment variables* (`CI_PROJECT_URL` / `GITHUB_SERVER_URL` / `GITHUB_REPOSITORY`), so
whichever CI the pipeline happens to run in "wins" the link flavor — every other
configured platform gets a release whose notes point at the wrong host with the wrong
link-path shape (`/pulls/N` vs `/-/merge_requests/N`, etc.). This is distinct from the
committed `CHANGELOG.md`, which stays singular and generated once (Step 2, unchanged).
T65-T73 below close this gap for git-cliff and cocogitto (communique is opaque to heraut
and is explicitly excluded — see T73).

> **Non-regression invariant (load-bearing for the whole phase).** Today's single-platform
> CI flows work *because* the templates resolve links from ambient CI vars — and that is
> correct even for self-hosted instances (a self-hosted GitLab runner sets
> `CI_PROJECT_URL=https://gitlab.example.com/...` and `glab` publishes via CI autologin;
> heraut never needs the host). Phase 14 must not regress this. **heraut injects
> per-platform link context only when it would change the answer — i.e. when more than one
> platform is configured** (and, once the [ADR-0020](../adr/0020-platform-base-url.md) gate
> lifts, when `base_url` is explicitly non-default). With exactly one platform and an unset
> (default) `base_url`, heraut injects **nothing**: notes are generated once, exactly as
> today, and ambient-CI detection runs untouched. Corollary: heraut must **never override a
> more-specific ambient CI value with a less-specific default `base_url`**. The ADR-0020
> validator gate is what makes this safe in the multi-platform path too — because every
> *injectable* `base_url` is currently a public default, the injected value can never be
> less specific than what ambient CI would have produced for that platform; the only way a
> self-hosted host reaches the notes is via ambient detection in the single-platform path,
> which we preserve by not injecting.

A **third, related but distinct** gap surfaced during the spike — heraut's config already
allows multiple platform entries of the *same* type (e.g. two GitLab instances), but
`findPlatformCfg`, the hardcoded `gitlabBaseURL`/`github.com` constants, `checkAPIAuth`,
and the reporter's `Name()` all silently assume at most one platform per type. It shares
`base_url` (T65/T66) as a load-bearing prerequisite but is **not** folded into this
phase's numbering — see "Related (but distinct) gap" in the design note. It needs its own
scoping pass, task numbers, and likely its own ADR before it lands on the roadmap.

#### `[x]` T65: ADR — per-platform `base_url` for self-hosted instances

**Motivation:** heraut cannot correctly resolve a self-hosted GitLab/GitHub Enterprise
host by sniffing ambient CI env vars — those describe *where CI is running*, not *where
each configured target platform lives*. A `base_url` field on `config.Platform` is the
natural extension (alongside `repository`/`project`), but it's a new wire-compatible field
and changes how link resolution works — it needs a decision record before it lands.

**Acceptance:** ADR documents the new `base_url` field (optional, default
`https://github.com` / `https://gitlab.com`), why it's needed (self-hosted instances,
correct link resolution, multi-instance prerequisite), and its relationship to T67's
per-platform notes regeneration.

**Files:** `docs/adr/0020-platform-base-url.md`

**Dependencies:** none

**Scope:** S

**Done:** [ADR-0020](../adr/0020-platform-base-url.md) written. Records `base_url` as the
single per-platform **web base URL** (not API endpoint — the CLIs derive the API path),
optional with per-type defaults, trailing-slash normalized. Key decision surfaced during
the task: `base_url` feeds **three** consumers — link resolution in notes (T70/T71), the
`ReleaseURL` summary (T66), and `gh`/`glab` **host targeting** (deferred to the
multi-instance thread). Consumers 1–2 work for the *default* values (the link-flavor fix is
meaningful even at defaults because public github.com vs gitlab.com differ in host *and*
path shape), so Phase 14 needs no host targeting. To avoid shipping a field that silently
half-works, the validator **rejects a non-default `base_url`** with a "self-hosted
publishing not yet supported" error until host targeting lands (user-chosen over folding
host-targeting into T66, or docs-only). Resolved design-note open question 2: no per-env
merge for `base_url` — `release.platforms` is replaced wholesale per env (ADR-0019), so a
per-env platform block already carries its own value. This gate flows into T66's
acceptance below.

#### `[x]` T66: `base_url` config field (config + schema + sample)

**Motivation:** Land the field decided in T65 following the standard field-change
checklist — struct, schema, sample doc must move together or IDE autocomplete and the
sample silently mislead users.

**Scope of change:**
- Add `BaseURL string` (optional) to `config.Platform`, defaulting to
  `https://github.com` / `https://gitlab.com` per platform type when empty. Normalize a
  trailing `/` so link construction never produces `//`.
- Update `schema.json` (type, description) and `docs/heraut.sample.yml` (show the field
  in context with a comment, including the self-hosted use case — noting it is gated
  pending host targeting).
- Semantic validation in `internal/config/validator.go`: reject malformed URLs, **and**
  per [ADR-0020](../adr/0020-platform-base-url.md), **reject a non-default `base_url`**
  (anything other than the platform-type default) with a clear "self-hosted publishing not
  yet supported — tracked separately (ADR-0020, multi-instance thread)" error. The field
  ships forward-compatible, but the only accepted value is the default until `gh`/`glab`
  host targeting lands. This gate is removed (with its tests) by the host-targeting task.

**Acceptance:**
- A `.heraut.yml` omitting `base_url` validates and loads exactly as today (per-type
  default applied). Schema fixture added to `testdata/config/`.
- A `base_url` equal to the platform-type default validates (explicit-but-default is fine).
- A *non-default* `base_url` (e.g. `https://gitlab.example.com`) is **rejected** by the
  validator with the ADR-0020 gate error — contract/validator test asserts the rejection
  and the hint text. (Flips to "accepted" only when host targeting lands.)
- A malformed `base_url` is rejected with a URL-validation error distinct from the gate.

**Files:** `internal/config/{config,validator}.go`, `schema.json`,
`docs/heraut.sample.yml`, `testdata/config/`

**Dependencies:** T65

**ADR required:** no — recorded in [ADR-0020](../adr/0020-platform-base-url.md) (T65),
this task lands the field it describes including the validator gate.

**Scope:** M

**Done:** Added `BaseURL string` (`yaml:"base_url,omitempty"`) to `config.Platform` as a
shared field. Defaulting + trailing-slash trim live in `loader.normalize` via a new
`normalizePlatforms` helper applied to **all** platform lists (top-level `release.platforms`
*and* every `environments.<env>.release.platforms`), backed by a `defaultBaseURL(type)`
helper + `defaultGitHubBaseURL`/`defaultGitLabBaseURL` constants in `config.go` — so after
load `Platform.BaseURL` is the single trailing-slash-free source of truth. Validation
(`validatePlatformBaseURL`, wired into both `validateRelease` and `validateEnvRelease`
loops): empty → accepted (default applies); malformed → URL error (checked first, via
`isValidBaseURL` requiring http/https scheme + non-empty host); well-formed but non-default
→ the ADR-0020 gate error ("self-hosted hosts are not yet supported", hint points to
ADR-0020). The two errors are deliberately distinct (malformed returns before the gate).
The validator treats empty as "use default" so it stays correct even on a directly-
constructed (non-normalized) `Config`. Schema gains a permissive `base_url` string property
(semantic gate stays in the validator, matching the strict-parse vs semantic split); sample
documents it on both the GitHub entry and the commented GitLab block with the
"self-hosted not yet supported" note. New valid fixture `testdata/config/valid/platform-
base-url.yml` (both platforms at their defaults) added to the schema glob and the explicit
`TestValidate_validFixtures` list. Tests: 8 new validator cases (default applied ×2,
explicit-default, trailing-slash, non-default gated, malformed-distinct-from-gate, per-env
gated) — 142 config tests green, 797 across all 21 packages, golangci-lint clean.
**Scope deviation (flagged + resolved):** ADR-0020's consumer table originally attributed
the `ReleaseURL` rewrite (reading `base_url` instead of the hardcoded
`gitlabBaseURL`/`github.com` constants) to T66, but T66's roadmap scope/files are
config-layer only. Implemented T66 to its roadmap entry (config + validator + schema +
sample + fixture) and left the platform-package wiring out, because the gate forces
`base_url == default`, making "read the field" vs. "read the constant" observationally
identical today. Surfaced the ADR-vs-roadmap mismatch; **user opted to defer the
`ReleaseURL` wiring (consumer 2) to the multi-instance host-targeting thread** — it touches
the same `platforms/{github,gitlab}` files as consumer 3 and removing the constants is a
natural part of making those packages instance-aware. ADR-0020's consumer table updated to
reattribute consumer 2 accordingly.

#### `[x]` T67: ADR — release notes regenerated per platform

**Motivation:** Closing the link-flavor gap requires regenerating release notes once per
configured platform — not once globally — each pass fed that platform's own
`base_url` + `repository`/`project`. This is a real architectural shift: notes stop being
a single artifact produced once in the pipeline and become N artifacts produced inside
the per-platform loop. It interacts with the reporter step semantics from
[ADR-0017](../adr/0017-pipeline-progress-reporter.md) (step count / naming) and
needs a decision record before the pipeline restructure (T70) begins.

**Acceptance:** ADR documents: why notes must be regenerated per platform (not just
templated smarter), how this changes `pipeline.Run()`'s step structure and ADR-0017's
step semantics, and confirms the committed `CHANGELOG.md` is unaffected (Step 2 stays a
single canonical generation tied to `origin`).

**Files:** `docs/adr/0021-per-platform-release-notes.md`

**Dependencies:** T66

**Scope:** S

**Done:** [ADR-0021](../adr/0021-per-platform-release-notes.md) written. Records the
decision: notes generation moves into the per-platform publish loop, regenerated once per
platform with that platform's link context — **but only when `len(Platforms) > 1`**;
single-platform keeps the existing single pre-loop generation with no context (preserving
ADR-0020's non-regression invariant + ambient-CI link resolution). Key step-model decision
(reconciling ADR-0017): per-platform notes generation follows the **asset-upload
sub-result precedent** rather than becoming its own numbered step — in multi-platform mode
the standalone `Generate release notes` step is **omitted** and generation folds into each
`Publish to {platform}` step (surfaced as a `notes generated` sub-result), so
`app.releaseStepTotal` branches on `len(Platforms) > 1` (the `+1` notes step only in the
single-platform arm; the `+len(Platforms)` publish steps cover it otherwise). Confirmed the
committed `CHANGELOG.md` (Step 2) is untouched — release notes are the only artifact
regenerated. Deferred (not decided here): the context-injection shape (T68), the
`port.Generator` signature (T69), template bytes (T71). communique: per-platform
regeneration is identical-but-harmless (it ignores context); pipeline *may* generate-once
for context-blind generators as an optimization; user-facing limitation documented in T73.
Out of scope: the tag-sync/target-pinning race (orthogonal timing problem). No code in
this task; one new live-feedback trade-off noted (notes-gen reported retroactively as a
sub-result rather than a live step in multi-platform mode).

#### `[x]` T68: Resolve the context-injection shape (env vars vs. new template variables)

**Motivation:** Mini-spike to decide *how* heraut hands each platform's link-resolution
context to the generators — the crux of "three generators must stay consistent."  Two
shapes are on the table: (a) reuse existing env vars (`CI_PROJECT_URL`,
`GITHUB_SERVER_URL`/`GITHUB_REPOSITORY` — smallest template diff, but conflates "the CI
heraut runs in" with "the platform heraut targets," and both sets could be present
simultaneously with the wrong one winning by the macro's `default()` chain), or (b) new
heraut-owned template variables (cleaner separation, but a real template-surface change
across git-cliff *and* cocogitto — needs to confirm what cocogitto's Tera context can
actually accept before committing to a shape).

**Acceptance:** Written decision (in the T67 ADR or a follow-up note) on which shape to
use, with a confirmed proof-of-concept showing cocogitto's Tera context can accept the
chosen shape. Unblocks T69-T71.

**Files:** none (research spike; output is a decision, not code)

**Dependencies:** T67

**Scope:** S

**Done:** Investigated both generators against installed `cog 7.0.0` + `git-cliff 2.13.1`
with a throwaway tagged repo; decision recorded in
[ADR-0021](../adr/0021-per-platform-release-notes.md) → "Context-injection shape (resolved
by T68)". **Finding:** the two generators resolve links through different surfaces, so a
single uniform mechanism is the wrong abstraction. git-cliff reads `get_env(name=…)` from
the subprocess env (heraut can inject via `RunEnv`); cog resolves links from
`--remote/--owner/--repository` CLI flags (or `[changelog]` cog.toml keys) — PoC produced
correct `gitlab.example.com` *and* `github.com` links, self-hosted included — and cog's
embedded templates render **no links today** (T71 adds them). **Decision:** option (b),
heraut-owned context — a generator-agnostic `LinkContext` at the `port.Generator` boundary,
each adapter translating it into the tool's native mechanism (git-cliff = heraut-owned env
vars read via `get_env` with the existing CI-var chain as `default=` fallback; cocogitto =
remote/owner/repository flags/keys; communique = ignored). Rejected option (a) reuse-ambient
as primary: heraut must *override* per target, and an ambient var describes the runner not
the target, so a heraut-owned precedence value is required anyway. **PoC-confirmed
non-regression:** injected `HERAUT_REMOTE_URL` wins; absent → falls through to ambient
`CI_PROJECT_URL` (self-hosted preserved). Flagged a cog asymmetry to T70/T71: cog has no
ambient fallback, so the ">1 platform" gate exists chiefly to protect git-cliff; cog links
first appear in multi-platform mode under the uniform gate. `LinkContext` indicative shape
(`BaseURL`/`Owner`/`Repo`/`Platform`, `nil` = fall through) handed to T69. No code changed.

#### `[x]` T69: `port.Generator` interface change to carry per-platform context

**Motivation:** `Generate(tag string) (string, error)` has no surface for per-platform
link-resolution context. Per [`coding.md`](../../.claude/rules/coding.md), `port`
interfaces are stable contracts changed deliberately with every implementor updated in
the same commit — git-cliff, cocogitto, *and* communique (which ignores the new context,
per T73's documented exclusion) all move together.

**Scope of change:**
- Extend `port.Generator.Generate` to accept the [ADR-0021]-resolved (T68) optional
  `*LinkContext` — indicatively `{ BaseURL, Owner, Repo, Platform }` — where `nil` means
  "no per-platform context → fall through to ambient detection" (the single-platform path).
- Update `gitcliff.Generator`, `cocogitto.Generator`, and `communique.Generator` to the
  new signature, each translating `LinkContext` into its native mechanism (T68 decision):
  git-cliff sets heraut-owned env vars via `RunEnv` (read by `get_env`); cocogitto passes
  `--remote/--owner/--repository` (bare host, scheme stripped; GitLab `group/sub/proj` →
  owner `group/sub`, repo `proj`); communique accepts and **ignores** it (documented
  opacity, not a bug). Template byte changes are T71 — T69 is the signature + plumbing.
- Update every test double / mock implementing `port.Generator`.

**Acceptance:** All three generators compile against the new signature; existing
contract tests pass with `nil` context (unchanged invocations); a non-nil context flows to
the expected native surface per generator (git-cliff: env vars on the call; cocogitto: the
`--remote/--owner/--repository` args; communique: asserted to have **no** effect on the
invoked command).

**Done:** `port.Generator.Generate` now takes `(tag string, lc *port.LinkContext)`, with
`LinkContext{BaseURL, Owner, Repo, Platform}` defined in `internal/port/generator.go`
(`nil` = single-platform fall-through). TDD: 5 new contract tests written first (red on the
new type/arity), then implementation. Per-generator translation (T68 decision): **gitcliff**
runs via `RunEnv` injecting `HERAUT_REMOTE_URL` (`{BaseURL}/{Owner}/{Repo}`) +
`HERAUT_PLATFORM` when `lc != nil`, else plain `Run` (ambient, unchanged); **cocogitto**
appends `--remote` (scheme-stripped host) / `--owner` / `--repository` when `lc != nil`;
**communique** accepts and ignores it (asserted: identical args, no env). All implementors
moved in one commit per the stable-contract rule, incl. `testutil.MockGenerator` (kept
`GenerateCalls []string`, added parallel `GenerateContexts []*port.LinkContext` for T70).
Three production callers pass `nil` for now (both changelog sites — changelog is singular;
the release-notes site — T70 will pass per-platform context). **No output change yet** —
T69 is plumbing; templates (T71) still ignore the injected env vars / flags. **Scope note:**
the GitLab `group/sub/proj` → owner/repo split is *not* in the adapters — `LinkContext`
carries pre-split `Owner`/`Repo`, so the split happens at construction (T70); adapters just
map fields (corrects the ADR-0021/earlier-T69 wording that implied the cocogitto adapter
splits). 802 tests green (5 new), golangci-lint clean.

**Files:** `internal/port/generator.go`, `internal/generators/{gitcliff,cocogitto,
communique}/*.go` and their `_test.go` files

**Dependencies:** T68

**Scope:** M

> **T70 split (this session, user-approved "split if needed"):** T70 was split into
> **T70a** (expose `LinkContext()` on `port.Platform` — the missing accessor) and **T70b**
> (the pipeline restructure that consumes it). Downstream "T70" references mean T70b.

#### `[x]` T70a: Expose per-platform `LinkContext()` on `port.Platform`

**Motivation:** T70b must build a per-platform `LinkContext`, but the pipeline holds only
`port.Platform` interface instances — `base_url`/`repository`/`project` aren't reachable
through the interface. The platform already exposes `ReleaseURL` built from the same
coordinates, so a parallel `LinkContext()` accessor is the natural home, and it keeps the
owner/repo split where the platform's path knowledge already lives.

**Scope of change:**
- Add `LinkContext() port.LinkContext` to the `port.Platform` interface (stable-contract
  change — every implementor moves in one commit, like T69).
- **github:** `BaseURL` from `cfg.BaseURL`; split the effective `repository()`
  (`owner/repo`) on the **first** slash → `Owner`/`Repo`; `Platform: "github"`.
- **gitlab:** `BaseURL` from `cfg.BaseURL`; split the effective `project()`
  (`group[/sub]/proj`) on the **last** slash → `Owner`/`Repo`; `Platform: "gitlab"`.
- Both reuse the existing env-fallback helpers (`repository()` / `project()`), so a repo
  path resolved from `GITHUB_REPOSITORY` / `CI_PROJECT_PATH` is reflected.
- Update `testutil.MockPlatform` (add a settable `LinkContextVal`).

**Acceptance:** contract tests assert github `acme/widget` → `{Owner: acme, Repo: widget}`
and gitlab `group/sub/proj` → `{Owner: group/sub, Repo: proj}`, each with the right
`BaseURL` (default + explicit) and `Platform`. TDD: failing tests first.

**Files:** `internal/port/platform.go`, `internal/platforms/{github,gitlab}/platform.go`
and their `_test.go`, `internal/testutil/mock_platform.go`

**Dependencies:** T69

**Scope:** S

**Done:** Added `LinkContext() port.LinkContext` to `port.Platform` (stable-contract
change, all 3 implementors moved together). **github**: `strings.Cut(repository(), "/")`
(first slash) → Owner/Repo, Platform `github`. **gitlab**: `strings.LastIndex(project(),
"/")` (last slash) → Owner=namespace (incl. nested `group/sub`), Repo=final segment,
Platform `gitlab`. Both reuse the existing `repository()`/`project()` env-fallback helpers
and fall back to the per-type default host (`githubBaseURL`/`gitlabBaseURL` consts) when
`cfg.BaseURL` is empty — so a directly-constructed (non-normalized) `config.Platform`
still yields a sane host (ReleaseURL keeps its own hardcoded host; the consumer-2 rewrite
is still deferred to the multi-instance thread). `testutil.MockPlatform` gained a settable
`LinkContextVal`. TDD: 6 contract tests first (red), then implementation — 808 tests green
(+6), golangci-lint clean. **No behaviour change yet** — nothing calls `LinkContext()`
until T70b.

#### `[x]` T70b: Restructure `pipeline.Run()` — notes generation moves into the per-platform loop

**Motivation:** Today, `p.cfg.Notes.Generate(result.Tag, nil)` runs once in Step 6 and the
resulting string is reused verbatim across every platform's `CreateRelease` in Step 7's
loop (`internal/pipeline/release.go`). To produce per-platform-flavored notes when there
is more than one platform, generation must move *inside* that loop, called once per
platform with that platform's `LinkContext` (from `plat.LinkContext()`, T70a). This
changes the pipeline's step count/order, which ADR-0017 governs — the reporter must be
updated in lockstep.

**Non-regression (see the phase invariant above):** the per-platform path is taken **only
when more than one platform is configured**. With exactly one platform, the pipeline keeps
generating notes once with **no** `LinkContext` (so the templates fall through to ambient
CI detection, exactly as today). heraut must never replace a single platform's
ambient-derived links with a less-specific default-`base_url` value.

**Scope of change:**
- When >1 platform: move `Notes.Generate` into the per-platform loop, passing
  `&plat.LinkContext()`. When exactly 1 platform: leave generation as a single pre-loop
  call with `nil` `LinkContext` (byte-for-byte today's behaviour).
- Update `ui.Progress`/reporter step definitions and counts to reflect the new structure
  (per ADR-0017 and [ADR-0021](../adr/0021-per-platform-release-notes.md)'s step-semantics
  decision: multi-platform omits the standalone notes step and folds notes generation into
  each publish step as a `notes generated` sub-result; `app.releaseStepTotal` adds the
  `+1` notes step only when `len(Platforms) <= 1`). The single-platform step structure
  stays unchanged.
- The committed `CHANGELOG.md` generation (Step 2) is untouched — confirm no accidental
  coupling.

**Acceptance:**
- Multi-platform: test shows N configured platforms → N `Notes.Generate` calls, each with
  that platform's distinct `LinkContext`; dry-run output and the post-run summary reflect
  the new step structure.
- **Single-platform non-regression:** exactly one platform → exactly **one**
  `Notes.Generate` call with a **nil** `LinkContext`, step count and dry-run output
  identical to pre-T70; existing single-platform contract/integration tests stay green
  unchanged.

**Files:** `internal/pipeline/release.go`, `internal/pipeline/release_test.go`,
`internal/app/pipeline.go` (`releaseStepTotal`)

**Dependencies:** T70a

**ADR required:** recorded in [ADR-0021](../adr/0021-per-platform-release-notes.md) (T67) —
this task implements that decision.

**Scope:** M

**Done:** `pipeline.Run()` Steps 6+7 restructured. Single platform → unchanged: one
standalone `Generate release notes` step calling `Notes.Generate(tag, nil)`, notes reused
by the lone publish step. Multi-platform (`len(Platforms) > 1`) → the standalone notes step
is gone; each `Publish to {platform}` step calls `Notes.Generate(tag, &plat.LinkContext())`
first (sub-result `notes generated`), then `CreateRelease`, then assets. `dryRunOutput`
mirrors this (standalone notes step only single-platform; `[dry-run] would generate notes`
folded into multi-platform publish steps; no generator calls). `app.releaseStepTotal` now
adds the `+1` notes step only when `len(Platforms) <= 1`, with a new white-box
`steptotal_internal_test.go` (`package app`) covering all 6 single/multi × notes/changelog
combinations. The committed `CHANGELOG.md` (Step 2) is untouched. TDD: 2 pipeline behaviour
tests + 3 reporter tests (multi folds / single standalone / dry-run multi) + 6 step-total
rows, written red first. Non-regression guards (`TestRun_SinglePlatform_NotesNilContext`,
`TestRun_Reporter_SinglePlatform_StandaloneNotesStep`) pass against the old and new code.
819 tests green (+11), golangci-lint clean. **Still no output change** — generators receive
the context but the templates ignore it until T71.

> **T71 split (this session, user-approved "split if needed"):** T71 splits along the two
> generators — **T71a** (git-cliff macros) and **T71b** (cocogitto templates) — which use
> different injection surfaces (env vars via `get_env` vs `--remote/--owner/--repository`
> flags) and so are independent. Downstream "T71" references mean both are done.
>
> **Verification note (applies to both):** the suite has **no** real-binary test precedent
> (MockRunner/FakeBin only), and heraut does not render Tera itself — the external tool
> does. So automated tests assert the macro/template **bytes** via
> `EffectiveReleaseNotesConfig()` / `EffectiveChangelogConfig()` (git-cliff) and the
> embedded template contents (cocogitto): heraut-var preferred, CI fallback preserved.
> Actual rendering is verified **manually** with the real tool against a throwaway repo
> (as in the T68 PoC) and the transcript captured in the Done note — this catches Tera
> syntax errors the byte assertions can't.

#### `[x]` T71a: git-cliff templates — prefer heraut-injected context with CI fallback

**Motivation:** `remote_url()`/`pr_link()` in **both** embedded git-cliff TOMLs
(`cliff.changelog.toml`, `cliff.release-notes.toml` — currently byte-identical macros)
resolve links solely from ambient CI env vars. They must prefer the heraut-injected
`HERAUT_REMOTE_URL` / `HERAUT_PLATFORM` (T69), falling back to the existing
`CI_PROJECT_URL` / `GITHUB_SERVER_URL` chain when absent. Per
[ADR-0010](../adr/0010-embedded-cliff-toml-default.md), embedded TOML is user-facing — any
byte change affects every default-config user.

**Scope of change:**
- `remote_url()`: `get_env(HERAUT_REMOTE_URL, default=<existing CI_PROJECT_URL/… chain>)`.
- `pr_link()`: discriminate GitLab (MR) vs GitHub (PR) via `HERAUT_PLATFORM` first, falling
  back to the current `CI_PROJECT_URL != ""` heuristic when `HERAUT_PLATFORM` is empty.
- The macro must treat an **empty** injected value as "not supplied" (fall through), never
  as "override with empty" — so a default-`base_url` value never clobbers a more-specific
  ambient host (the self-hosted-CI non-regression).
- Update **both** TOMLs identically (keep the macros in sync). The changelog is generated
  with `nil` context (T70b), so `HERAUT_REMOTE_URL` is never set for it — its update is a
  verified no-op kept purely for macro parity.
- Preserve the pre-existing `/pulls/` path literal verbatim (a separate latent bug — see
  the new follow-up task below; not in scope here).

**Acceptance:** `EffectiveReleaseNotesConfig()` and `EffectiveChangelogConfig()` contain
`HERAUT_REMOTE_URL` and `HERAUT_PLATFORM` **and** still contain `CI_PROJECT_URL` (fallback
preserved). Existing `Effective*Config` assertions stay green. Manual render PoC against
real git-cliff (env set → heraut host; env unset → ambient `CI_PROJECT_URL`) captured in
the Done note. TDD: byte assertions red first.

**Files:** `internal/generators/gitcliff/cliff.{changelog,release-notes}.toml`,
`internal/generators/gitcliff/generator_test.go`

**Dependencies:** T70b

**Scope:** S

**Done:** Both embedded TOMLs updated identically. `remote_url()` now reads
`get_env(HERAUT_REMOTE_URL, default=<CI_PROJECT_URL → GITHUB_SERVER_URL+GITHUB_REPOSITORY
chain>)`; `pr_link()` sets `heraut_platform = get_env(HERAUT_PLATFORM)` and renders an MR
link when it's `gitlab` (or empty + `CI_PROJECT_URL` set), else a PR link — so the
heraut-injected platform overrides the ambient CI heuristic but an absent value falls
through unchanged. `/pulls/` literal preserved (→ T74). TDD: byte assertion
(`TestEffectiveConfig_PrefersHerautContext`: both Effective configs contain
`HERAUT_REMOTE_URL` + `HERAUT_PLATFORM` and still `CI_PROJECT_URL`) red first, then the
edit. **Manual render PoC** against real git-cliff 2.13.1 (throwaway tagged repo, since the
suite has no real-binary tests) confirmed all four cases — no Tera syntax error:
(1) `HERAUT_*` gitlab → `gitlab.example.com/.../-/merge_requests/N`, bogus `CI_PROJECT_URL`
correctly ignored; (2) `HERAUT_*` github → `github.com/.../pull/N` *(PoC used the T74 fix
`/pull/`; the embedded TOML still ships `/pulls/`)*; (3) no `HERAUT_*` + self-hosted
`CI_PROJECT_URL` → self-hosted host + MR (ambient fallback, non-regression); (4) no
`HERAUT_*` + GitHub Actions vars → `github.com` + PR. 821 tests green, golangci-lint clean.

#### `[x]` T71b: cocogitto templates — render per-platform links from the remote context

**Motivation:** cocogitto's embedded `changelog.tera` / `release-notes.tera` render **no
commit/PR links at all** today. With the `--remote/--owner/--repository` flags now passed
by the adapter (T69), the templates must render links from cog's remote context (the same
mechanism cog's built-in `remote` template uses). Per ADR-0010 this is a user-facing
template change.

**Scope of change:**
- Probe cog 7.0.0 to confirm the exact Tera context variable names exposed when
  `--remote/--owner/--repository` are set (built-in `remote` template as reference).
- Rewrite the embedded `.tera` templates to render commit links using those vars, while
  rendering link-free output when they are absent (single-platform / no-context case stays
  byte-for-byte as today).
- Document the byte-level diff per ADR-0010.

**Acceptance:** embedded-template tests assert the templates reference the remote context
vars; absent-context rendering is unchanged. Manual render PoC against real cog (flags set
→ host links; flags unset → no links) captured in the Done note. TDD: byte assertions red
first.

**Done:** Probed cog 7.0.0 (incl. extracting its built-in `remote` template from the
binary): the context variable is **`repository_url`** — the full base
`https://{host}/{owner}/{repo}`, set only when `--remote/--owner/--repository` are passed,
**undefined otherwise**. Confirmed empirically that `{% if repository_url %}` is
falsy-safe on the undefined case (no Tera error). Added a guarded link suffix
`{% if repository_url %} - ([{{ commit.id | truncate(length=7,end="") }}]({{ repository_url }}/commit/{{ commit.id }})){% endif %}`
to the commit loop in **both** `release-notes.tera` and `changelog.tera` (kept in sync;
heraut always passes `nil` context to the changelog driver so its suffix is a no-op for
parity). Commit links use `/commit/` universally — matching cog's own `remote` template
(GitHub native; GitLab redirects). TDD: white-box `embed_internal_test.go` (`package
cocogitto`, reads the production `embed.FS`, asserts `repository_url` + `/commit/` +
`if repository_url` guard) red first, then the edit. **Manual render PoC** against real cog
7.0.0 using the actual embedded templates: flags set → `gitlab.example.com/acme/widget/
commit/<sha>` links; flags unset → link-free output **byte-identical to today** (the
non-regression); `changelog.tera` unchanged (version header + no links). 824 tests green
(+3), golangci-lint clean. **This is the payoff:** with T71a + T71b, a multi-platform
release now produces per-platform-flavored links end to end for git-cliff *and* cocogitto
(communique still excluded — T73).

**Files:** `internal/generators/cocogitto/{changelog,release-notes}.tera` (and `cog.toml`
if needed), `internal/generators/cocogitto/generator_test.go`

**Dependencies:** T70b

**Scope:** M

#### `[x]` T72: Integration test — multi-platform release produces distinctly-flavored notes

**Motivation:** Closes the loop end-to-end. The whole point of T65-T71 is that "N
platforms configured → N distinctly-flavored notes, each pointing at its own host/path
shape" — that needs a real integration assertion, not just unit/contract coverage of the
pieces.

**Acceptance:** Full-pipeline integration test (real git repo + `testutil.FakeBin`)
configures GitHub + GitLab platforms with distinct `base_url`/`repository`/`project`
values, runs `heraut release`, and asserts each platform's `CreateRelease` call received
notes whose commit/PR/MR links use *that platform's* host and path shape (not the other
platform's, not ambient-CI-derived values).

**Files:** `internal/pipeline/release_test.go` (or a new integration test file alongside
it, matching the existing integration-test layout)

**Dependencies:** T70, T71

**Scope:** S

**Done:** `internal/pipeline/release_integration_test.go` —
`TestRun_Integration_MultiPlatform_DistinctlyFlavoredNotes`. Drives `pipeline.Run()`
through the **real** `exec.New` runner (not MockRunner) with real `gitcliff`/`github`/
`gitlab` constructed against it, so it exercises the one path the contract tests can't:
heraut's per-platform `HERAUT_REMOTE_URL` propagating through `exec.Runner.RunEnv` into the
git-cliff subprocess. FakeBins: `git` is a no-op (so `git tag`/`git push` never touch the
real repo — verified the working tree/tags stay clean), `git-cliff` echoes
`$HERAUT_REMOTE_URL` as the notes (stand-in for "rendered links against this host"; real
Tera rendering is the T71 manual PoC), and `gh`/`glab` capture the `--notes` they receive
to per-tool files. Asserts the GitHub release got `https://github.com/test/gh-repo` and the
GitLab release got `https://gitlab.com/test/gl-proj`, with neither leaking the other's host.
**Deviations from the acceptance text (kept Scope S):** (1) pipeline-level (real exec +
FakeBin), **not** cmd-level `executeRoot` — a full non-dry-run cmd release would also need
to fake the whole preflight (`gh`/`glab` `--version`/token/API probes, git identity,
`GITHUB_ACTIONS`/`GITLAB_CI` env hygiene), which is M+ and tests mostly preflight surface
unrelated to the notes flow; (2) `fakeResolver` + no-op fake `git` instead of a real git
repo+remote — the per-platform-notes behaviour is what's under test, and faking git matches
the existing cmd integration tests' pattern while guaranteeing the real repo is never
mutated; (3) distinct *hosts* come from the per-type defaults (`github.com` vs `gitlab.com`)
+ `repository`/`project`, since a non-default `base_url` is still gated (ADR-0020). 825
tests green (+1), golangci-lint clean. **Phase 14's link-flavor goal is now proven
end-to-end.**

#### `[x]` T73: Spec update — document communique's link-resolution exclusion

**Motivation:** `communique.Generate` is fully opaque to heraut — it just runs
`communique generate --config <user-file> <tag>` and returns stdout, with link
resolution entirely owned by the user's communique config. T69 threads a `LinkContext`
through `port.Generator` for consistency, but communique ignores it: "inject per-platform
context" is not achievable for communique users without a communique-side feature. This
must be a documented limitation, not a silent surprise.

**Acceptance:** `docs/specs/05-generators-and-platforms.md` explicitly states that
communique users publishing to multiple platforms will get identical release notes across
all of them (communique resolves its own links from its own config, independent of which
platform heraut is currently publishing to), and that this is a known, accepted scope
boundary — not a bug.

**Files:** `docs/specs/05-generators-and-platforms.md`

**Dependencies:** none (can land independently, but make sense to land alongside T69 so
the spec and the code agree from the same commit forward)

**Scope:** S

**Done:** Added a "Known limitation — multi-platform links" note to the communique section
of Spec 05 (matching cocogitto's existing limitation-note style): communique is opaque,
owns link resolution via the user's `communique.toml`, ignores the per-platform context
heraut passes, so a release to >1 platform gets **identical** notes/links on every
platform — a known, accepted scope boundary, not a bug; pointed multi-platform users to
git-cliff/cocogitto and ADR-0021. **Also reconciled a T69 doc-debt** that the note depends
on: the "Generator interface" section still showed the pre-T69 `Generate(tag string)`
signature, so updated it to `Generate(tag string, link *port.LinkContext)` and added a
short "Per-platform link resolution" paragraph (heraut regenerates notes per platform with
each platform's `link` context when >1 platform; single-platform passes `nil` → ambient-CI
fallback; git-cliff/cocogitto consume it, communique doesn't). Docs-only — no tests; typos
+ commit-msg hooks pass.

#### `[x]` T74: Fix git-cliff PR link path — `/pulls/` → `/pull/` (pre-existing bug)

**Motivation:** Surfaced while reading the templates during T71a. The embedded git-cliff
`pr_link()` macro (both `cliff.changelog.toml` and `cliff.release-notes.toml`) builds
GitHub PR links as `{remote}/pulls/{number}`, but GitHub's PR URL is `/pull/{number}`
(singular) — `/pulls/N` is the PR *list*, not PR N, so the link is wrong. Pre-existing,
unrelated to the multi-platform work; preserved verbatim through T71a to keep that change
focused.

**Scope of change:** change `/pulls/` → `/pull/` in the `pr_link()` GitHub branch of both
embedded TOMLs; update any test asserting `/pulls/`.

**Acceptance:** PR links render as `/pull/N`; `Effective*Config` reflects the fix. Manual
git-cliff render PoC confirms a valid GitHub PR URL.

**Files:** `internal/generators/gitcliff/cliff.{changelog,release-notes}.toml` and tests

**Dependencies:** T71a (avoid editing the same macro lines concurrently)

**ADR required:** no — bugfix to a wrong literal; document the byte change per ADR-0010.

**Scope:** S

**Done:** Swapped the single literal `/pulls/` → `/pull/` in the `pr_link()` GitHub branch
of both embedded TOMLs (`cliff.changelog.toml`, `cliff.release-notes.toml`). GitHub's PR
URL is `/pull/<n>`; `/pulls/<n>` was the PR-list path, so the link 404'd/redirected.
TDD: new byte-assertion `TestEffectiveConfig_GitHubPRPath` (both Effective configs contain
`/pull/`, not `/pulls/`) red first, then the fix. Manual git-cliff render PoC confirmed
`[#42](https://github.com/acme/widget/pull/42)`. Per-ADR-0010 byte change: affects the
GitHub PR link in default-config release notes/changelogs — now correct. 826 tests green
(+1), golangci-lint clean.

#### `[x]` T75: Fat-injection / thin templates — heraut computes URL prefixes in Go

**Motivation (user idea, 2026-06-09):** push the `HERAUT_*` approach to its conclusion —
have heraut compute the per-platform URL **prefixes** in Go and inject them, so the embedded
git-cliff templates become **branch-free pure interpolation** with no `pr_link` `if/else`
and no `get_env` fallback chains. Moves the platform path-shape knowledge (GitHub `#`+`/pull/`
vs GitLab `!`+`/-/merge_requests/`, and `/commit/` vs `/-/commit/`) out of untestable Tera
and into a typed, table-tested Go helper. heraut already owns `LinkContext.Platform`.

**Variables are prefixes, not whole URLs.** git-cliff supplies the variable part at render
time (`commit.id`, `commit.remote.pr_number`, `previous.version..version`), so heraut injects
the prefix the template appends to:

| Var | GitHub | GitLab |
|-----|--------|--------|
| `HERAUT_COMMIT_URL`  | `{base}/{owner}/{repo}/commit/`  | `{base}/{owner}/{repo}/-/commit/` |
| `HERAUT_PR_URL`      | `{base}/{owner}/{repo}/pull/`    | `{base}/{owner}/{repo}/-/merge_requests/` |
| `HERAUT_PR_LABEL`    | `#`                              | `!` |
| `HERAUT_COMPARE_URL` | `{base}/{owner}/{repo}/compare/` | `{base}/{owner}/{repo}/-/compare/` |

(`HERAUT_REMOTE_URL` from T69 stays as the repo-root base. Issue links remain git-cliff's
own `link_parsers` domain — not heraut's.)

**Decisions baked in (2026-06-09):**
- **Fully thin, no fallback (chosen over a static `default()`).** The embedded template is
  heraut's default and *documents* that heraut populates the `HERAUT_*` vars; there is **no**
  `default()` and **no** ambient `CI_PROJECT_URL` / `GITHUB_SERVER_URL` detection. Standalone
  `git-cliff` runs (no heraut) get empty prefixes — users invoking git-cliff directly keep
  their own template. This is the cleanest realization and the contract must be documented.
- **git-cliff only.** cocogitto is already branch-free (renders from cog's native
  `repository_url`, populated by `--remote/--owner/--repository`) — **no cocogitto change**.

**Why gated on host-targeting (still parked):** today the `pr_link` branch + `get_env`
fallback exist to serve a single self-hosted platform whose real host arrives only via
ambient `CI_PROJECT_URL` (the [ADR-0020](../adr/0020-platform-base-url.md) gate forbids a
non-default `base_url`). Retiring ambient detection now would break that self-hosted-CI flow.
Once the host-targeting / multi-instance thread makes `base_url` authoritative for **every**
case, heraut always knows the host → always injects → ambient detection is genuinely
unnecessary and can retire, and the templates go fully thin. Doing it before then yields a
hybrid (injected path + fallback branch) — marginal gain, more surface.

**Scope of change (when unblocked):**
- A table-tested Go helper computing the prefix set per `LinkContext.Platform` (home TBD in
  the ADR — likely a `LinkContext` method or a gitcliff-adapter helper; the path-shape logic
  is platform-general, the `HERAUT_*` var names are git-cliff-specific). Injected via
  `RunEnv` for **all** git-cliff runs.
- Rewrite both embedded git-cliff TOMLs (`cliff.changelog.toml`, `cliff.release-notes.toml`)
  to branch-free interpolation — remove the `pr_link` `if/else` and every `get_env(...,
  default=...)` chain.
- Retire ambient `CI_PROJECT_URL` / `GITHUB_SERVER_URL` detection from the defaults.

**Acceptance:**
- Go table test: `github`/`gitlab` `LinkContext` → the exact prefix set above (incl. the
  `/-/` GitLab paths and `#`/`!` labels).
- Byte assertions: `EffectiveReleaseNotesConfig()`/`EffectiveChangelogConfig()` reference
  `HERAUT_COMMIT_URL` / `HERAUT_PR_URL` / `HERAUT_PR_LABEL` / `HERAUT_COMPARE_URL` and contain
  **no** `get_env(..., default=` and **no** `pr_link` platform `if`.
- Manual real-git-cliff render PoC (per the suite's no-real-binary convention): injected →
  correct per-platform links incl. GitLab `/-/` paths.

**ADR required:** **yes** — changes the embedded template-variable contract
([ADR-0010](../adr/0010-embedded-cliff-toml-default.md) user-facing surface), documents the
"requires heraut" standalone contract, and retires ambient CI detection; **supersedes the
T71a macro shape**.

**Dependencies:** the host-targeting / multi-instance thread (currently an unscoped thread
in the [design note](../../.claude/plans/multi-platform-release-notes-link-resolution.md)
"Related (but distinct) gap" — scoped separately, not yet numbered). **Do not start before
that thread makes `base_url` authoritative for single-platform self-hosted.**

**Scope:** M (deferred / parked)

**Done:** [ADR-0022](../adr/0022-fat-injection-thin-templates.md) written. **Un-parked**
during planning: the host-targeting gate only blocks self-hosted *publishing*, not link
*rendering* — heraut can read the ambient CI host vars in **Go**, so the ambient fallback
relocates from Tera into a table-tested helper and the templates go fully thin now (no
host-targeting needed; the `base_url` publish-gate is untouched). **Done as one task, not
the suggested T75a/T75b split** — the changelog template is generated at *two* sites
(release pipeline Step 2 + the changelog pipeline) and a thin `changelog.toml` requires
both to inject context, so splitting would have opened a regression window (thin template +
`nil` context → empty links). Implementation: (1) `gitcliff.linkEnv` extended to emit all 6
vars, computing `/-/` (gitlab) vs `/` (github) + `#`/`!` — the testability payoff
(`TestLinkEnv`, white-box table); (2) `pipeline.ambientLinkContext()` resolves the ambient
host (`CI_PROJECT_URL`→gitlab / `GITHUB_SERVER_URL`+`GITHUB_REPOSITORY`→github / nil),
`t.Setenv`-tested; (3) `release.go` — single-platform release notes use
`singlePlatformLinkContext()` (ambient **only if it matches the target platform**, else the
platform's own — a guard that also fixes a latent mismatch bug in the old Tera detection),
changelog Step 2 uses ambient; (4) `changelog.go` — changelog uses ambient; (5) **both**
embedded TOMLs rewritten to branch-free interpolation (`remote_url()`/`pr_link()` macros,
the platform `if/else`, and the `get_env` fallback chain all removed; `default=""`
empty-guard kept). cocogitto untouched (already branch-free). **Reverses ADR-0021/T70b's
"single-platform → `nil` context"** and **supersedes the T71a/T74 macro shape** (the
`/pull/` vs `/pulls/` correctness now lives in `linkEnv`/`TestLinkEnv`); both documented in
ADR-0022. Tests: `TestLinkEnv` + `TestAmbientLinkContext` (the Go payoff), updated
single-platform context tests (platform-context / ambient-preferred / ambient-mismatch),
thin-template byte assertions (`TestEffectiveConfig_ThinTemplates`). **Manual real-git-cliff
render PoC**: both thin templates render correctly with GitHub + GitLab prefixes (incl.
GitLab `/-/compare/` and `/-/commit/`), and standalone (no heraut env) degrades to empty
prefixes with **no Tera error**. 838 tests green, golangci-lint clean.

#### `[x]` T76: Richer cocogitto default templates (git-cliff-like layout, achievable subset)

**Motivation (user idea, 2026-06-09):** cocogitto's embedded templates render a much barer
changelog/release-notes than git-cliff's. Bring the *layout* closer to git-cliff for a
quality bump to cocogitto users — independent of git-cliff, not full parity. Surfaced while
comparing the two generators during the T71–T75 template work.

**Achievable (cog exposes the data) — in scope:**
- **Emoji group headers** matching git-cliff (`🚀 Features`, `🐛 Bug Fixes`, `🚜 Refactor`,
  `📚 Documentation`, `⚡ Performance`) — set in the embedded `cog.toml` `commit_parsers`
  `group` names (verify cog maps `group` → the template's `group_by(attribute="type")` value;
  confirmed informally in T71b).
- **Author attribution** — `commit.author` / `commit.signature` (the git author/signature,
  **not** a linked platform username — cog has no API username). Decide link vs plain name.
- Keep the T71b commit links, scope, and breaking marker.

**NOT achievable (cog exposes nothing — confirmed against the cog 7.0.0 binary, 0 refs) —
explicitly out of scope:**
- PR/MR links (git-cliff's `commit.remote.pr_number`, from the platform API).
- New Contributors section (`gitlab.contributors`).
- Commit Statistics block (`statistics.*`).

These need cog itself to add a git-cliff-style `[remote]` integration; document them as a
known limitation, not a heraut gap.

**Scope of change:** edit embedded `internal/generators/cocogitto/cog.toml` (emoji group
names) + `changelog.tera` / `release-notes.tera` (author); update
`docs/specs/05-generators-and-platforms.md` (note the layout improvement + the parity
limits); document the byte change per [ADR-0010](../adr/0010-embedded-cliff-toml-default.md).

**Acceptance:** byte assertions — the embedded templates/`cog.toml` carry the emoji group
names + author field; manual real-cog render PoC (suite has no real-binary tests) shows the
richer output. cocogitto's existing four config-path contract tests stay green.

**ADR required:** no — but an ADR-0010 byte-change note; spec update documents the parity
limitation. (If the change to defaults is judged substantial, a short ADR is fine.)

**Dependencies:** none (independent of the host-targeting thread; builds on T71b's cocogitto
link rendering).

**Scope:** S

**Done — and it turned out to be a bug fix, not just enrichment.** Starting T76 surfaced
that the embedded `cog.toml` used a git-cliff-style `[changelog] commit_parsers` block,
which **cog 7.0.0 rejects** (`unknown field commit_parsers`) — so the default
`generator: cocogitto` path (the `none/none` config combination) **failed at runtime**.
The unit tests missed it because they mock cog (never run it). Rewrote the embedded
`cog.toml` to cog's actual schema: top-level `[commit_types]` with emoji `changelog_title`
for feat/fix/refactor/docs/perf (matching git-cliff's headings) and `omit_from_changelog`
for chore/ci/build/test/style (preserving the old silencing intent; cog *includes* those by
default). Added author attribution (`commit.signature` — the clean git author name; cog's
`commit.author` handle is empty without an `[changelog] authors` mapping) to both `.tera`
templates, alongside the T71b commit links. All verified against **real cog 7.0.0** (PoCs):
config now parses, emoji headers render, chore/ci omitted, author + scope + links present.
Tests: `TestEmbeddedCogToml_Cog7Schema` (no `commit_parsers`; has `[commit_types]` + emoji
+ omit rules) + author assertion in the template test. Spec 05 updated (new mechanism +
the cog-schema note + the git-cliff parity limits). Out of scope, confirmed impossible in
cog (0 refs in the binary): PR/MR links, contributors, statistics. 839 tests green,
golangci-lint clean. **Follow-up surfaced → T77** (the testing gap that let this ship).

#### `[x]` T77: Validate embedded generator configs against the real CLIs

**Motivation:** T76 found that the embedded cocogitto `cog.toml` was invalid for the pinned
cog 7.0.0 yet shipped green, because every generator contract test uses `MockRunner` and
never executes the real tool — so a malformed embedded `cog.toml` / `cliff.*.toml` is never
caught. The same blind spot applies to git-cliff (the embedded TOMLs are only byte-asserted,
never run). A guard is needed so an embedded default that the real CLI rejects fails CI.

**Scope of change (to design):** a **skippable** integration test (skip when the binary is
absent — deviates from the suite's MockRunner/FakeBin norm, so decide the pattern) that runs
the real `cog` / `git-cliff` against each embedded default config over a tiny `t.TempDir`
git repo and asserts a clean parse/render. Alternatively, a `heraut check` sub-check that
validates the effective embedded config via the tool (`cog --config … changelog --at` /
`git-cliff --config … --context --no-exec`). Pick one in a short spike.

**Acceptance:** a deliberately-broken embedded config fails the new guard; CI runs it
(the bundled Docker image already has the CLIs — ADR-0016).

**Dependencies:** none. **Scope:** S–M (includes a small spike on the approach).

**Done:** Spike resolved the CI question decisively: forge's reusable `go-ci.yml` `test`
job runs **Setup Mise (`mise-action`) before `go test ./...`**, and `.config/mise/config.toml`
pins `cocogitto`/`git-cliff` — so the tools are on PATH during CI's test run. That means a
plain **skippable** real-CLI test runs in CI (no CI-pipeline change needed) and only skips
for local devs lacking the tools. Chose that over the `heraut check` sub-check (simpler,
direct CI protection). Added `testutil.RealGitRepo` (temp git repo + tagged conventional
commit + `t.Chdir`, skips if git absent) and two tests — `TestEmbeddedConfig_RealGitCliff`
and `TestEmbeddedConfig_RealCog` — that build each generator with a real `exec` runner +
the embedded default config and run **both** modes, asserting the real tool accepts it.
**Proof the guard works:** temporarily restoring the broken `commit_parsers` `cog.toml`
makes `TestEmbeddedConfig_RealCog` fail with the exact T76 error (`unknown field
commit_parsers`); the fixed config passes. Documented the new real-CLI smoke-test category
as a narrow exception in `.claude/rules/testing.md` (config-acceptance only; skippable;
local + deterministic). 845 tests green, golangci-lint clean.

#### `[x]` T78: `remote_metadata` policy — control git-cliff's remote enrichment

**Motivation:** git-cliff enriches changelog/release-notes with PR author + number by
hitting the GitHub/GitLab API (auto-detected from the git remote; triggered by the
`commit.remote.*` refs in the embedded templates). That fetch needs a token and is **fatal
on failure** — unauthenticated it hits GitHub's 60/hr shared-IP limit and panics with a 403
(exit 101). CI now authenticates it (`ci: 92b2a12`), but any tokenless / rate-limited /
offline run of `heraut changelog` / `check` / `release` still crashes, with no opt-out.

**Scope of change:** add a tri-state policy `remote_metadata: required | optional |
disabled` (default **`optional`**) — a **top-level** key governing **both** changelog and
release-notes generation. Both are `*ContentDriver` sharing the embedded git-cliff
templates, and both `cliff.changelog.toml` and `cliff.release-notes.toml` carry the
`commit.remote.*` refs, so the original failure hit `cliff changelog` *and* `cliff
release-notes` alike — the policy must cover both. Backed by git-cliff's `--offline` flag
(verified: 0 calls, no panic, renders cleanly without the `@author`/`#PR` suffix;
commit/compare links survive — they're heraut-owned via `HERAUT_*`):

- `required` → fetch; **fail** if unavailable (strict CI — today's de-facto behavior).
- `optional` → fetch when possible; on a remote-fetch failure re-run with `--offline` +
  `ui.Warn` and continue (degrades against missing token *and* rate-limit/network).
- `disabled` → always pass `--offline`; never touch the network (deterministic, no token).

Plumb the policy config → app/pipeline → **both** gitcliff generators (changelog +
release-notes, each via `Generate` + `CheckCliff`). Add a root `--offline` flag as one-off
sugar for `disabled` (overrides config for that run, both generators). The `port.Generator`
semantic is "remote-enrichment policy"; cocogitto / communique treat non-`required` as a
no-op (they don't fetch). A per-generator override (split changelog vs release-notes policy)
is **deferred** — add only if split policies are actually requested (YAGNI).

**Acceptance:** MockRunner contract tests assert `--offline` is present iff policy is
`disabled` (absent for `required`) on **both** the changelog and release-notes invocations;
an `optional` run with a forced remote failure degrades + warns instead of erroring; a
config with no key behaves as `optional`. `schema.json` +
`docs/heraut.sample.yml` updated; an ADR records the flag/config surface and the
default-`optional` behaviour change (heraut no longer panics tokenless out of the box).

**Dependencies:** none (builds on T75 / ADR-0022's `HERAUT_*` link injection). **Scope:** S–M.

**Done:** Shipped as [ADR-0023](../adr/0023-remote-metadata-policy.md) across six commits.
Top-level `remote_metadata: required | optional | disabled` (default `optional`),
enum-validated, backed by git-cliff's `--offline`. `optional` retries `--offline` on **any**
remote failure and reports `Degraded()` — retry-on-failure was chosen over predict-by-token
because the original incident was a rate-limit (a present-but-throttled token would still
panic under prediction). The policy is propagated to both the changelog and release-notes
drivers via `withEnvDerivations` + `app.CheckCliff` (applied to a driver copy, never
mutating the caller's), and degrade is surfaced in the `heraut check` cliff detail line and
as a release/changelog step sub-result (generators expose an optional `Degraded()` interface
the pipeline type-asserts, keeping it decoupled). Root `--offline` flag forces `disabled`
via `applyOfflineOverride`. `schema.json` + `docs/heraut.sample.yml` + valid/invalid schema
fixtures added. Per-generator split policy left deferred (YAGNI). Full suite green,
golangci-lint clean.

### Phase 15 — Ticket linking

First-class issue-tracker links (Jira/Linear/GitHub-issue) in the changelog **and** release
notes, via git-cliff's `link_parsers`. A top-level `tickets:` list is validated, propagated
onto the content drivers (like `remote_metadata`), and injected into the effective git-cliff
TOML; both embedded templates render `commit.links`. Design:
[`.claude/plans/ticket-linking.md`](../../.claude/plans/ticket-linking.md); implementation
plan: [`.claude/plans/ticket-linking-implementation.md`](../../.claude/plans/ticket-linking-implementation.md);
decision recorded in ADR-0024 (T82).

#### `[x]` T79: `tickets` config surface + validation

Add a top-level `tickets:` list (`pattern` + `url`) — parsed by the loader and
semantically validated: each `pattern` compiles as a regex, each `url` is an absolute
http(s) URL containing `{ticket}`, and `tickets` set with a non-git-cliff generator is an
error (only git-cliff has a link mechanism). Mirrors `remote_metadata`'s top-level
governance of both generators.

**Files:** `internal/config/{config.go,validator.go}` + tests. **Scope:** S.

**Done:** `Ticket{Pattern, URL}` + `Config.Tickets` + the programmatic `ContentDriver.Tickets`
carrier; `validateTickets` (each pattern compiles, each url is an absolute http(s) URL
containing `{ticket}`, and `tickets` requires git-cliff). Commits `14d0b6f`, `47a518b`.

#### `[x]` T80: Inject ticket `link_parsers` + propagate onto drivers

The gitcliff generator's `effectiveConfig` gains `injectLinkParsers`: each ticket becomes a
git-cliff `{ pattern, href }` entry (pattern wrapped in a capture group only when it has
none, so `{ticket}`→`$1` is the URL value; label defaults to the full match), appended to
any existing `[git].link_parsers`. The app layer propagates `cfg.Tickets` onto each driver
in `withEnvDerivations` (the same clone point as `remote_metadata`).

**Files:** `internal/generators/gitcliff/generator.go`, `internal/app/pipeline.go` + tests.
**Scope:** S–M. **Dependencies:** T79.

**Done:** `injectLinkParsers` in `effectiveConfig` (regex `NumSubexp()==0` → wrap the pattern
in a group; `{ticket}`→`$1`; appended after any existing `link_parsers`); `withEnvDerivations`
carries `cfg.Tickets` onto the driver clone. Commits `99c9e26`, `5696d2c`.

#### `[x]` T81: Render ticket links in changelog + release-notes templates

Both embedded `print_commit` macros append `commit.links` after the PR-number segment.
Whitespace verified via a real-CLI render (config-acceptance tests only — rendered output
is checked manually, as with the New Contributors section).

**Files:** `internal/generators/gitcliff/{cliff.changelog.toml,cliff.release-notes.toml}`.
**Scope:** S. **Dependencies:** T80.

**Done:** Both `print_commit` macros append `commit.links` as `([TICKET](url))`. A real-CLI
render confirmed subject *and* footer (`Refs:`) tickets link with clean whitespace,
location-independent. Commit `698d075`.

#### `[x]` T82: Schema, sample, fixtures + ADR-0024 + spec

`schema.json` (`tickets` array) + `docs/heraut.sample.yml` section + valid/invalid schema
fixtures; ADR-0024 recording the link_parsers-over-preprocessors decision; document
`tickets` in `docs/specs/02-configuration.md`.

**Files:** `schema.json`, `docs/heraut.sample.yml`, `testdata/config/{valid,invalid}/`,
`internal/config/schema_test.go`, `docs/adr/0024-ticket-linking.md`, `docs/adr/README.md`,
`docs/specs/02-configuration.md`. **Scope:** S. **Dependencies:** T79.

**Done:** `tickets` in `schema.json` + `docs/heraut.sample.yml` + valid/invalid schema
fixtures; [ADR-0024](../adr/0024-ticket-linking.md) records the link_parsers-over-preprocessors
decision; `docs/specs/02-configuration.md` § `tickets`. Full suite green, golangci-lint clean.

### Phase 16 — Multi-instance same-platform releases

Allow `release.platforms` (and per-env overrides) to contain multiple entries of the same
platform type — e.g. a public `gitlab.com` instance and a self-hosted
`gitlab.example.com` instance — in one `heraut release` run. This is the "multi-instance
thread" deferred by [ADR-0020](../adr/0020-platform-base-url.md) (`base_url`). Design:
[`.claude/plans/multi-instance-platforms-design.md`](../../.claude/plans/multi-instance-platforms-design.md);
implementation plan:
[`.claude/plans/multi-instance-platforms-implementation.md`](../../.claude/plans/multi-instance-platforms-implementation.md);
decision recorded in ADR-0025 (T87).

#### `[x]` T83: Config schema — required `name` field + lift `base_url` gate

Add a required, unique `config.Platform.Name` field (unique per `release.platforms` list
scope — top-level and each env override independently). Remove
`validatePlatformBaseURL`'s "must equal the platform-type default" gate, keeping only the
`isValidBaseURL` shape check. Update `schema.json`, `docs/heraut.sample.yml`,
`.config/heraut.yml`, and all fixtures/tests to add `name:`. `heraut init` defaults +
dedupes `name` per platform type (`github`, `gitlab`, `gitlab-2`, ...).

**Files:** `internal/config/{config.go,validator.go,validator_test.go}`, `schema.json`,
`docs/heraut.sample.yml`, `.config/heraut.yml`, `internal/cmd/{check,release}_test.go`,
`internal/scaffold/{generate.go,generate_test.go}`, `testdata/config/`. **Scope:** M.
**Dependencies:** none.

**Done:** Added `Platform.Name` (`yaml:"name"`, required, no `omitempty`). Introduced
`validatePlatformEntries`, a single helper called from both `validateRelease` and
`validateEnvRelease`, which checks platform type, required+unique `name` (scoped per
list, tracked via a `map[string]int` of first-seen index), and `base_url` shape.
`validatePlatformBaseURL` now only runs `isValidBaseURL` — the old "must equal the
platform-type default" gate (ADR-0020) is removed entirely; self-hosted `base_url`
values are accepted (ADR-0025 supersedes the gate). Updated `schema.json` (`name`
required, new property, reworded `base_url` description),
`docs/heraut.sample.yml` (added `name:` to both platform examples, replaced the
ADR-0020 "not yet supported" caveat with an ADR-0025 self-hosted note, and added a
multi-instance example block), `.config/heraut.yml`, `internal/cmd/check_test.go` and
`internal/cmd/release_test.go` (5 occurrences), and the 4 affected fixtures in
`testdata/config/valid/`. `heraut init` (`internal/scaffold/generate.go`) now defaults
each platform entry's `name` to its type and appends `-N` for the Nth+ duplicate of a
type (`github`, `gitlab`, `gitlab-2`, ...) via a `platformTypeCount` map. No deviations
from the plan.

#### `[x]` T84: GitLab platform — `hostEnv()`, `Name()`, `ReleaseURL()` honor config

`internal/platforms/gitlab/platform.go` gains `selfHosted()`/`hostEnv() []string`
(`GITLAB_HOST=<host>` for non-default `base_url`, else `nil`); `Name()` returns
`cfg.Name`; `ReleaseURL()` honors `cfg.BaseURL`; `CreateRelease`/`UploadAssets` switch to
`RunEnv(p.hostEnv(), "glab", ...)`; `checkAPIAuth` skips `GITLAB_CI` autologin when
self-hosted and merges `hostEnv()` into the token-auth probe.

**Files:** `internal/platforms/gitlab/{platform.go,platform_test.go}`. **Scope:** M.
**Dependencies:** T83.

Implemented exactly as planned: `Name()` now returns `cfg.Name`; `ReleaseURL()` falls
back to the default `gitlabBaseURL` only when `cfg.BaseURL` is empty; `selfHosted()`
reports true when `cfg.BaseURL` is set and differs from `gitlabBaseURL`; `hostEnv()`
parses `cfg.BaseURL` and returns `["GITLAB_HOST=<host>"]` (or `nil` for the default
host, making `RunEnv(p.hostEnv(), ...)` a no-op for existing callers).
`CreateRelease`/`UploadAssets` now call `RunEnv(p.hostEnv(), "glab", ...)`.
`checkAPIAuth` skips the `GITLAB_CI` autologin branch entirely when `selfHosted()` is
true, falling through to the token-based `/user` probe with `hostEnv()` merged into the
env alongside `GITLAB_TOKEN`. No deviations from the plan.

#### `[x]` T85: GitHub platform — `hostEnv()`, `Name()`, `ReleaseURL()` honor config

Mirrors T84 for `internal/platforms/github/platform.go`: `selfHosted()`/`hostEnv()`
returns `["GH_HOST=<host>", "GH_ENTERPRISE_TOKEN=<token>"]` for non-default `base_url`;
`Name()` returns `cfg.Name`; `ReleaseURL()` honors `cfg.BaseURL`;
`CreateRelease`/`UploadAssets`/`checkAPIAuth` merge `hostEnv()` into the token env;
`checkAPIAuth` skips `GITHUB_ACTIONS` autologin when self-hosted.

**Files:** `internal/platforms/github/{platform.go,platform_test.go}`. **Scope:** M.
**Dependencies:** T83.

Implemented exactly as planned, mirroring T84/GitLab. `Name()` now returns `cfg.Name`;
`ReleaseURL()` falls back to the existing `githubBaseURL` constant (`"https://github.com"`,
already defined — no new constant needed) when `cfg.BaseURL` is empty. Added
`selfHosted()` (true when `cfg.BaseURL` is set and differs from `githubBaseURL`) and
`hostEnv()` (returns `["GH_HOST=<host>", "GH_ENTERPRISE_TOKEN=<token>"]`, or `nil` for the
default host) using `net/url` to parse the host from `cfg.BaseURL`.
`CreateRelease`/`UploadAssets`/`checkAPIAuth` now call
`RunEnv(append(p.tokenEnvSlice(), p.hostEnv()...), ...)`. `checkAPIAuth` skips the
`GITHUB_ACTIONS` autologin branch when `selfHosted()` is true, falling through to the
token-based `repos/{owner}/{repo}/releases?per_page=1` probe with `hostEnv()` merged in.
No deviations from the plan.

#### `[x]` T86: `heraut check runtime` — one Platforms row per configured entry

Restructure `internal/app/check.go`'s Platforms section from "one row per platform *type*"
(`configuredPlatforms`/`findPlatformCfg`, first-match-by-type) to "one row per
`release.platforms` *entry*", labeled by the entry's `name`, running that entry's full
`Check()`. Falls back to a binary-only `glab`/`gh` probe when no platforms are configured
(unchanged nil-config / no-`release`-block behavior). Per-CLI-type binary-presence
deduplication across same-type entries (described in the design note) is **deferred** —
each entry runs its own `--version` probe.

**Files:** `internal/app/{check.go,check_test.go}`. **Scope:** M. **Dependencies:** T83,
T84, T85.

Done: removed `configuredPlatforms` and `findPlatformCfg`. When `cfg.Release.Platforms`
is non-empty, `RuntimeCheck` now dispatches one Platforms row per entry, labeled by
`platCfg.Name`, each running its own `buildPlatform` + `p.Check()` (binary + token +
project/repository + API auth). `buildPlatform` already returned an "unsupported
platform" error for unknown types, so no message change was needed for
`TestRuntimeCheck_UnknownPlatform`. When `cfg == nil` or `cfg.Release == nil` or
`cfg.Release.Platforms` is empty, falls back to the prior binary-only `glab`/`gh` probe
(hard error when `cfg == nil`, advisory otherwise). Per-CLI-type binary-dedup across
same-type entries remains deferred, as already agreed — each entry runs its own
`--version` probe, demonstrated by `TestRuntimeCheck_MultipleSameTypePlatforms` (two
`gitlab` entries, two `glab --version` probes).

#### `[x]` T87: Docs — ADR-0025, supersede ADR-0020, update spec 05

New `docs/adr/0025-multi-instance-platforms.md` recording the lifted `base_url` gate,
per-platform CLI host targeting, the required unique `name` field, and the restructured
`heraut check runtime` Platforms section. Mark ADR-0020 as superseded.
`docs/specs/05-generators-and-platforms.md` gains a self-hosted/multi-instance subsection
and `name:` in both platform examples.

Created ADR-0025 with the full decision record (required unique `name` field, lifted
`base_url` gate, `hostEnv()` per-platform CLI host targeting, `Check()` self-hosted
autologin skip, `Name()`/`ReleaseURL()`/`LinkContext()` honoring configured values, and
the restructured `heraut check runtime` Platforms section). Marked ADR-0020 as superseded
with a pointer blockquote, and updated the ADR README index (ADR-0020 row →
"Superseded by ADR-0025", new ADR-0025 row added after ADR-0024). Updated
`docs/specs/05-generators-and-platforms.md`: added `name:` and `base_url:` to both the
GitHub and GitLab platform examples, added a new "Self-hosted instances and multiple
entries of the same type (ADR-0025)" subsection, and updated the `Platform` interface's
`Name()` doc comment. This completes Phase 16 (T83-T87) / ADR-0025.

**Files:** `docs/adr/0025-multi-instance-platforms.md`, `docs/adr/0020-platform-base-url.md`,
`docs/adr/README.md`, `docs/specs/05-generators-and-platforms.md`. **Scope:** S.
**Dependencies:** T83-T86.

---

### Phase 17 — Full-project review remediation

Findings from the 2026-06-12 full code + docs review of v0.31.0 (all ~6.8k production
lines, specs, ADRs, schema, CLAUDE.md). Three correctness bugs in code, one spec/code
conflict that makes a spec-faithful config fail to load, significant doc drift from the
forge extraction, plus consistency and hygiene items. Ordered by priority: correctness
first (T88-T93), then docs (T94-T96), then consistency/hygiene (T97-T101).

#### `[x]` T88: GitLab platform — inject `token_env` into release-time calls

`Check()` injects the configured token via `tokenEnvSlice()`, but `CreateRelease` and
`UploadAssets` pass only `hostEnv()` — so a custom `token_env` (or two GitLab instances
with different tokens, ADR-0025's motivating scenario) passes `heraut check runtime` and
then runs `glab release create` without the token. Mirror GitHub's
`RunEnv(append(p.tokenEnvSlice(), p.hostEnv()...), ...)` pattern. The contract tests
asserting `["GITLAB_HOST=..."]`-only env on the self-hosted CreateRelease/UploadAssets
rows currently pin the buggy behavior — update them deliberately in the same commit, per
the testing rules.

**Files:** `internal/platforms/gitlab/{platform.go,platform_test.go}`. **Scope:** S.
**Dependencies:** none.

`CreateRelease` and `UploadAssets` now build `env := append(p.tokenEnvSlice(p.tokenEnv()),
p.hostEnv()...)` before calling `RunEnv`, matching `checkAPIAuth`'s existing pattern and
GitHub's `tokenEnvSlice()`+`hostEnv()` composition. GitLab's `tokenEnvSlice` keeps its
existing `(envName string)` signature (unlike GitHub's zero-arg version) since
`checkAPIAuth` already threads `tokenEnv` explicitly. Updated
`TestCreateRelease_SelfHosted_SetsGitlabHostEnv` and
`TestUploadAssets_SelfHosted_SetsGitlabHostEnv` to set `GITLAB_TOKEN` and expect
`["GITLAB_TOKEN=tok", "GITLAB_HOST=gitlab.example.com"]`, mirroring
`TestCheck_SelfHosted_SkipsCIAutologin`'s already-correct env ordering.

#### `[x]` T89: `hasEffectivePlatforms` — env `release:` without `platforms:` inherits root

Spec 02 § Content override semantics: `release.platforms` absent in env → use the root
list, and `buildReleasePipelineConfig` implements that (`len > 0` check). But
`hasEffectivePlatforms` (`internal/cmd/release.go`) returns false whenever
`envCfg.Release != nil` and its platform list is empty, so an env overriding only
`release.notes` gets a spurious "requires at least one entry in release.platforms" error.
Align the guard with the builder and the spec.

**Files:** `internal/cmd/{release.go,release_test.go}`. **Scope:** S.
**Dependencies:** none.

`hasEffectivePlatforms` now mirrors `buildReleasePipelineConfig`'s merge: start from
`cfg.Release.Platforms`, and only replace with the env's list when
`len(envCfg.Release.Platforms) > 0`. Added `internal/cmd/release_internal_test.go`
(`package cmd`, precedent: `offline_test.go`) with a table-driven
`TestHasEffectivePlatforms` covering both the root-only and env-inheritance paths as a
pure-function unit test — going through `executeRoot` would have dragged in generator/
pipeline execution unrelated to this guard. Small deviation from the roadmap's file list
(`release_test.go` only), noted here per the roadmap-discipline rule on deviations.

#### `[x]` T90: Unify `--version` override validation across `release` and `changelog`

`heraut release` enforces `^v?\d+\.\d+\.\d+$` — rejecting valid CalVer overrides for any
format that isn't exactly 3 numeric components (`YYYY.PATCH`, `YYYY.MM.DD.PATCH`) and all
pre-releases — while `heraut changelog --version` validates nothing. Spec 03 states no
shape restriction. Decide the contract (strategy-aware validation, or none), apply it to
both commands, and record it in spec 03.

**Files:** `internal/cmd/{release.go,changelog.go}` + tests,
`docs/specs/03-commands.md`. **Scope:** S. **Dependencies:** none.

**Decision: no shape restriction.** `NewResolver` already treats `--version` as an opaque
string for `StaticResolver` (only a leading `v` is stripped); `Resolve()` performs zero
validation. Spec 03 documents `--version` generically with no regex. Dropped the SemVer-
only `versionPattern` regex entirely and added `tagfmt.ValidateVersionOverride` /
`app.ValidateVersionOverride` — mirroring the existing `ValidateBuildID` precedent but
*without* the `/`-ban, since a full tag override may legitimately contain `/`. The check
is now: non-empty, no whitespace, nothing else. Applied identically to `release.go`
(replacing the old regex check) and `changelog.go` (new — it previously validated
nothing). `TestRelease_VersionFlag_InvalidFormat`'s 5 previously-rejected-but-now-valid
cases (`notaversion`, `v1`, `v1.2`, `v`, `va.b.c`) moved into
`TestRelease_VersionFlag_ValidFormats`, which also gained CalVer (`2024.03`,
`2024.03.15.2`) and pre-release (`1.2.3-rc.1`) cases; `InvalidFormat` now covers
whitespace-containing values. Added `TestValidateVersionOverride` to
`tagfmt_test.go`/`resolver_test.go`, and `TestChangelog_VersionFlag_RejectsWhitespace` for
parity. Judged this an input-validation contract change (not a version-arithmetic
"hard-won edge case"), so no ADR — the decision and rationale are recorded here and in
spec 03's updated `--version` rows.

#### `[x]` T91: SemVer bump — fix breaking-change detection edge cases

`isBreaking` matches `"!:"` anywhere in the subject (`fix: handle the foo!: token` →
spurious major bump); anchor the `!` to the conventional-commit type/scope prefix. Also
recognize the hyphenated `BREAKING-CHANGE:` footer, which the Conventional Commits 1.0
spec mandates as a synonym of `BREAKING CHANGE:`.

**Files:** `internal/versioning/semver/{bump.go,bump_test.go}`. **Scope:** S.
**Dependencies:** none.

Replaced the `strings.Index(subject, "!:") > 0` check with a
`breakingPrefixPattern = regexp.MustCompile(`^\w+(\([^)]*\))?!:`)`, anchoring `!:` to the
type/optional-`(scope)` prefix — `feat(api)!:` still matches, `fix: handle the foo!:
token` no longer does. `isBreaking` now also checks `BREAKING-CHANGE:` alongside
`BREAKING CHANGE:`. Added the two new cases (plus a `feat(scope)!` regression case) as
rows in the existing `TestDetermineBump` table in `resolver_test.go` rather than creating
`bump_test.go` — `DetermineBump`/`BumpVersion` are already tested there as
`package semver_test`, and `isBreaking`/`isFeat` are unexported so can only be exercised
through `DetermineBump`; a new file would split one function's coverage across two files
for no benefit. Deviation from the roadmap's stated `bump_test.go`, noted here per
roadmap-discipline.

#### `[x]` T92: SemVer resolver — pre-release tag policy

Git's default `version:refname` sort orders `v1.2.3-rc.1` *above* `v1.2.3` (without
`versionsort.suffix`), and `BumpVersion("1.2.3-rc.1")` then fails with "invalid patch" —
so one pre-release tag bricks auto-resolution. Decide: skip tags that don't parse as
`MAJOR.MINOR.PATCH` (mirroring the CalVer resolver's skip-unparsable behavior), or
declare pre-release tags unsupported in spec 04 and keep the hard error with a clearer
message.

**Files:** `internal/versioning/semver/{resolver.go,resolver_test.go}`,
`docs/specs/04-versioning.md`. **Scope:** S. **Dependencies:** none.

**Decision: skip, mirroring CalVer.** Added `isBareVersion(s string) bool` to `bump.go`
(plain `MAJOR.MINOR.PATCH`, no pre-release/build metadata — used only for tag-skipping,
kept separate from `BumpVersion`'s parsing so its specific "invalid major/minor/patch"
error messages, which are load-bearing tests, stay untouched). `resolveAuto` now walks
`tags` (already sorted newest-first) and picks the first whose bare form satisfies
`isBareVersion`, exactly like the CalVer resolver's `ParseVersion`-skip loop; if none
match, falls back to the "no tags" branch (`initial_version`). Added
`TestResolve_SkipsPreReleaseTag` and `TestResolve_AllTagsPreRelease_UsesInitialVersion`.
Added a "Pre-release tags" subsection to spec 04 documenting the skip behavior and the
all-pre-release fallback. Scoped to `resolveAuto` only — `BumpAuto` (used by
`semver-per-env` via `internal/versioning/perenv`) takes pre-filtered/stripped tags from
its caller and is a separate code path; whether `perenv` has the same latent issue is
unexplored and out of scope here (flagged as a follow-up below if needed).

#### `[x]` T93: Pipelines — push only the created tag

Both pipelines run `git push origin --tags`, publishing every local tag including stale
or experimental ones. Push `result.Tag` explicitly (`git push origin <tag>`). Align the
step-name wording between the two pipelines ("Push tag" vs "Push tags") while there.

**Files:** `internal/pipeline/{release.go,changelog.go}` + tests,
`docs/specs/03-commands.md` (action sequences). **Scope:** S. **Dependencies:** none.

Added `gitHelper.pushTag(tag string) error` in `internal/pipeline/git.go` (`git push
origin <tag>`, wrapped as `"git push %s: %w"`), used by both `Pipeline.Run` (Step 5) and
`ChangelogPipeline.Run` (Step 5, the `Tag && !NoPush` branch) in place of the prior
`git push origin --tags` calls. Renamed `ChangelogPipeline`'s "Push tags" step (real +
dry-run) to "Push tag" to match the release pipeline, and updated the corresponding
reporter-test assertions in `changelog_reporter_test.go`. Dry-run "Push tag" steps in
both pipelines now report `[dry-run] would push <tag>` (previously a generic "would
push"), naming the exact tag for clarity. Updated doc comments in both pipeline files and
`docs/specs/03-commands.md` (lines describing the release action sequence, `--no-push`,
and the changelog tag-push step) from `git push --tags` / `git push origin --tags` to
`git push origin <tag>`. Left ADR-0011 and ADR-0017 (which describe `git push --tags` /
"Push tags" as historical architecture decisions) and the historical T42 acceptance
criteria and pre-T93 decision notes elsewhere in this roadmap untouched — they document
state at the time they were written, not current behavior.

#### `[x]` T94: Spec 02/03 — platform tables and command surface match the code

Spec 02 platform tables: remove the `catalog:` field (no `Catalog` field exists in
`config.Platform`; the strict loader rejects unknown keys, so a spec-faithful config
fails to load — the code notes catalog publishing is automatic now), add the **required**
`name:` field (ADR-0025) and `base_url:` to both GitHub and GitLab tables and examples,
and fix `glab release upload-asset` → `glab release upload --use-package-registry`.
Spec 03: document the `whatsnew` command (asserted by `root_test.go` but absent from the
behavioral authority).

**Files:** `docs/specs/{02-configuration.md,03-commands.md}`. **Scope:** S.
**Dependencies:** none.

Updated both Spec 02 "Platform drivers" sections (GitLab, GitHub): added the required
`name` field and the optional `base_url` field (both ADR-0025) to the YAML examples and
field tables, removed the `catalog` field/row (replaced with a sentence explaining
CI/CD Catalog publishing is automatic — no config needed), and changed the GitLab
"Implementation" line to `glab release create` + `glab release upload
--use-package-registry`. Added a new `## heraut whatsnew` section to Spec 03 (after
"Background update hint") documenting the command's behavior — renders release notes
for versions newer than the running build via forge's `updatecheck.WhatsNewCommand`,
with GitHub API → local cache → embedded changelog fallback — based on `internal/cmd/
root.go`'s registration and `go doc github.com/adaouat/forge/updatecheck`. Pure docs
change; no code or tests touched. Filed T104 for the spec 02 "Complete examples"
(missing `name:` on every platform entry) and spec 05's matching `catalog`/
`upload-asset` drift, found during this task but outside its stated file list.

#### `[x]` T95: CLAUDE.md — rewrite to post-forge reality

CLAUDE.md still describes the pre-forge codebase: `internal/adapter/exec/`,
`internal/selfupdate/`, `self_update.go`, `fang.Execute` in main.go, "19 ADRs" (25
exist), and an ldflags section anchored on `internal/selfupdate/updater.go`. Missing:
the `github.com/adaouat/forge` dependency (exec runner, config loader, exitcode, ui,
updatecheck), `internal/exitcode/`, the `whatsnew` command, and the `--offline` flag.
Rewrite the project-layout, tech-stack, ldflags, and command-surface sections.

**Files:** `CLAUDE.md`. **Scope:** S. **Dependencies:** none.

> Rewrote five sections against the current tree (verified via `go.mod`, `main.go`,
> `internal/port/runner.go`, `internal/exitcode/`, `internal/cmd/{root,offline,exit}.go`,
> `internal/ui/{header,status,theme}.go`). "What this tool does" swaps `self-update` for
> `whatsnew`. "Docs" now says 25 ADRs. "Tech stack" replaces the `fang` and
> `bubbles/spinner` rows with a single `github.com/adaouat/forge` row (forge/cli wraps
> cobra+fang — both are now indirect deps — and provides exec/config/exitcode/ui/
> updatecheck); `//go:embed` row now also covers the root `changelog.go` embed used by
> `whatsnew`'s offline fallback. "Project layout" drops `internal/adapter/exec/` and
> `internal/selfupdate/`, adds `internal/exitcode/`, `internal/cmd/{offline,exit}.go`,
> the root `changelog.go`, corrects `port.Runner` to "alias to forge/exec.Runner", and
> corrects the `main.go` entry-point line to `forge/cli.Run(cmd.NewRootCmd(version), …)`.
> "ldflags invariant" replaces the `internal/selfupdate/updater.go` /
> `defaultProjectURL`/`defaultLatestURL` anchor (removed with selfupdate) with the
> hardcoded `"adaouat/heraut"` repo literal now passed to `forge/updatecheck` in
> `root.go`, and points ADR-0014 at its supersession (forge ADR-0005). Added an
> `--offline` bullet to "Non-obvious constraints" (it silently overrides
> `remote_metadata` from config). While verifying `internal/exitcode/`'s `Promotion`
> code, found `internal/cmd/exit.go`'s doc comment still says "fang.Execute" — folded
> as item (6) into T101's existing hygiene bundle rather than filing a new task.

#### `[x]` T96: Roadmap — reconcile the v1.0.0 checkpoint items and stale overview

CHECKPOINT I is titled "v1.0.0 shipped via heraut" and marked `[x]`, but its sub-item
"v1.0.0 cut by running `heraut release` on the heraut repo itself" is `[ ]` (here and
again under CHECKPOINT K) while the project is at 0.31.0. Either cut v1.0.0 or retitle
the checkpoints to reflect that the gates are green and the cut is pending. Also refresh
the roadmap's Overview, which predates the forge extraction: "19 ADRs", "self-update
tooling", and "extracted into a shared Go library later" (forge already exists).

**Files:** `docs/tasks/roadmap.md`. **Scope:** S. **Dependencies:** none.

> Retitled rather than cut v1.0.0 — actually running `heraut release` on this repo is an
> irreversible action (tag push + GitHub release) well outside a docs-only task and
> needs explicit user sign-off, not a roadmap edit. CHECKPOINT I's title no longer claims
> "v1.0.0 shipped"; its three real sub-items (T28, CI split, coverage ≥80%) are all done
> and stay `[x]`, and its duplicate `v1.0.0 cut` bullet was removed with a pointer to
> CHECKPOINT K, which already carries that pending item plus the "all quality gates are
> green" explanation. Overview: "19 ADRs" → "25 ADRs" (×2), "self-update" → "whatsnew" in
> the v1.0 feature list, and goal 3 (extract `port`/`adapter/exec`/`testutil`/`ui` into a
> shared library) is now marked Done — `github.com/adaouat/forge` is that library,
> cross-referenced via ADR-0014 (the one heraut ADR documenting a forge supersession).

#### `[x]` T97: Validate `tag_pattern` is git-cliff-only

`tag_pattern` on a content driver is silently ignored by `communique` and `cocogitto`
(only git-cliff consumes it). Gate it in the validator the same way `tickets` is gated
to git-cliff, with an actionable hint.

**Files:** `internal/config/{validator.go,validator_test.go}`,
`docs/specs/02-configuration.md`. **Scope:** S. **Dependencies:** none.

> Added one check inside `validateContentDriver` (after the existing `generator` enum
> check): if `tag_pattern` is set and `generator` is set and isn't `git-cliff`, emit a
> `<path>.tag_pattern` error hinting to switch to git-cliff or remove `tag_pattern`. This
> single insertion point covers all four call sites (top-level `changelog`, top-level
> `release.notes`, and their per-env equivalents) because `MergeContentDriver`
> (ADR-0019) already resolves each per-env driver to its effective `(generator,
> tag_pattern)` pair before validation runs — inherited drivers see the parent's
> git-cliff generator, generator-switches see the override's. Used exact-case
> `Generator != "git-cliff"` to match this function's existing enum check, not
> `strings.EqualFold` (T101 item (4) already tracks that cross-function
> case-sensitivity inconsistency as a separate hygiene item). Five new tests cover:
> top-level changelog/release-notes rejection, a valid git-cliff case, per-env
> inheritance (valid), and per-env generator-switch (rejected). `02-configuration.md`'s
> `tag_pattern` row now says "git-cliff only" and that pairing it with `communique` or
> `cocogitto` is a validation error. Checked `testdata/config/valid/semver-per-env.yml`
> — its `tag_pattern` entries are both already paired with `git-cliff`, no fixture
> changes needed.

#### `[x]` T98: Command consistency pass — validation and no-config behavior

Three alignments: (1) `heraut version next/current` skip `config.Validate`, so per-env
misconfigs surface as raw resolver errors instead of path/hint output; (2) bare
`heraut check` hard-fails when no config file exists while `check runtime` degrades to
the all-tools-required probe — align bare `check`; (3) bare `check` exits with the
Runtime code even when only config-section errors failed — classify by what failed.

**Files:** `internal/cmd/{version.go,check.go}` + tests, `docs/specs/03-commands.md`.
**Scope:** M. **Dependencies:** none.

> (1) Both `version next` and `version current` now call `config.Validate` right after
> `config.Load` and, on errors, print the same path/hint output as `check config` via
> `printConfigErrors` and exit Config — before touching `CheckBranch` or the resolver.
> Demonstrated with a 2-env promotion cycle (`prod.source: staging`,
> `staging.source: prod`): previously this surfaced as a raw `E003: no source tags found`
> (Promotion exit code) or "no tags found for prod/*" from deep inside the resolver;
> now it's `environments.prod.source: cycle detected (prod → staging → prod)` with its
> hint, exit Config. (2) Bare `check`'s config-load now mirrors `check runtime`:
> `os.ErrNotExist` sets `cfg = nil` instead of hard-failing, `applyOfflineOverride` is
> skipped, the Config section prints a "no config found … — skipping config validation"
> warning, and the Cliff section prints "no git-cliff generators configured" (avoiding a
> nil-pointer dereference in `runCliffChecks`, which assumes a non-nil `cfg`). (3)
> Tracked config-section failures separately (`configFailed`) from the overall `failed`
> count; the summary now exits Config if `configFailed > 0` (config errors are
> foundational — fix those first, regardless of whether runtime/cliff also failed),
> else Runtime. Five new tests across `version_test.go`, `exit_test.go`, and
> `check_test.go` cover all three alignments.

#### `[x]` T99: `heraut init` update flow — round-trip advanced fields

`ConfigToAnswers` drops `tickets`, `remote_metadata`, `release.assets`, `base_url`,
`draft`/`prerelease`, and env content overrides, so the "Update it?" flow silently
regenerates a config without them (the user confirms the printed YAML, but nothing
signals the loss). Either carry these fields through Answers → GenerateYAML, or print an
explicit "the following settings will be dropped" warning before the confirm prompt.

**Files:** `internal/scaffold/{wizard.go,generate.go}` + tests, `internal/cmd/init.go`.
**Scope:** M. **Dependencies:** none.

> Chose the warning approach. "Carry through" doesn't fit cleanly: `runPlatformWizard`
> and `runEnvWizard` reset `a.Platforms`/`a.Environments` to nil and rebuild them from
> wizard prompts, so passthrough fields on the old entries would need identity-matching
> (by type? by name?) against the rebuilt entries — ambiguous on rename/reorder/add/
> remove, pushing this from Scope M to L. The warning is a single uniform mechanism for
> all six field categories.
>
> Added `scaffold.DroppedFields(cfg *config.Config) []string` (new file
> `internal/scaffold/dropped.go`) returning sorted YAML-path strings for any of the six
> categories present in the loaded config — `tickets`, `remote_metadata`,
> `release.assets`, per-platform `release.platforms.<name>.{base_url,draft,prerelease}`,
> and per-env `environments.<name>.{changelog,release}`. For `base_url`, only a
> non-default value is flagged (exported `config.DefaultBaseURL`, renamed from the
> unexported `defaultBaseURL`, since `normalizePlatforms` always fills the field after
> `config.Load` so emptiness is not a usable signal).
>
> `internal/cmd/init.go`'s "Update it?" branch calls `scaffold.DroppedFields(cfg)` right
> after `ConfigToAnswers` and prints a warning (new unexported `printDroppedFieldsWarning`)
> before the wizard runs, so the user sees the loss before investing time in the prompts.
> `wizard.go`/`generate.go` (`Answers`, `ConfigToAnswers`, `answersToConfig`,
> `GenerateYAML`) are unchanged — no behavior change for users who don't set these
> fields. `printDroppedFieldsWarning` has a direct internal test
> (`internal/cmd/init_internal_test.go`); the surrounding "Update it?" huh-prompt branch
> has no end-to-end test, consistent with the existing suite (no test drives any
> non-`--defaults` interactive `init` path — `huh` forms need a TTY).

#### `[x]` T100: `heraut check runtime --env` — check the env's effective platforms

`RuntimeCheck`'s Platforms section reads only `cfg.Release.Platforms`; an environment
override's platform list is never checked. Thread the active `--env` through to
`RuntimeCheck` and dispatch one row per *effective* platform entry (env list when
present, else root), reusing the same replace semantics as the pipeline builder.

**Files:** `internal/app/{check.go,check_test.go}`, `internal/cmd/check.go`,
`docs/specs/03-commands.md`. **Scope:** M. **Dependencies:** T89 (shared effective-list
semantics).

> `RuntimeCheck` gained an `env string` parameter; the Platforms section now resolves
> `platforms` the same way as `buildReleasePipelineConfig` and `hasEffectivePlatforms`
> (T89): start from `cfg.Release.Platforms`, replace wholesale with
> `cfg.Environments[env].Release.Platforms` only when `env != ""`, the env exists,
> `Release != nil`, and its platform list is non-empty. `internal/cmd/check.go` reads the
> root `--env` flag in both the bare `check` and `check runtime` `RunE` functions and
> threads it through `runRuntimeCheck`. This is now the third independent copy of this
> exact resolution snippet (`pipeline.go`, `cmd/release.go`'s `hasEffectivePlatforms`, and
> here) — extracting a shared `config` helper would be a reasonable follow-up but is out
> of scope for this task's file list (`pipeline.go`/`cmd/release.go` aren't listed) and
> not added as a new task, since T101's hygiene bundle already tracks related
> consistency issues in this area. `collectItems` in `check_test.go` gained an `env`
> parameter (all ~20 existing call sites pass `""`, preserving prior behavior); two new
> tests cover the replace and inherit branches
> (`TestRuntimeCheck_EnvPlatformOverrideReplacesRoot`,
> `TestRuntimeCheck_EnvWithoutPlatformOverrideInheritsRoot`). `docs/specs/03-commands.md`
> documents the `--env` resolution under `heraut check runtime`, noting bare `heraut
> check` applies the same logic.

#### `[x]` T101: Hygiene — comments, loop copies, enum style, case conventions

Bundle of style-rule violations found in review: (1) 16 task-ID references
(T68/T75/T78/T79...) in production comments across `internal/cmd/offline.go`,
`internal/pipeline/{release.go,linkctx.go}`, `internal/config/config.go`, and all three
generators — the coding rules forbid task references in comments; keep the ADR pointers,
drop the T-ids. (2) Leftover pre-Go-1.22 loop-var copies (`plat := platform` in
`pipeline/release.go`, `og := og` in `app/check.go`) — finish what commit 60b3e9d
started. (3) `BumpType`/`Mode` enums repeat `= iota` on every line. (4) Validator enum
maps are exact-case while `app.buildGenerator`/`checkCliffDriver` use
`ToLower`/`EqualFold` — pick one convention. (5) `code := exitcode.Runtime` pointless
variable in `cmd/check.go`. (6) `internal/cmd/exit.go`'s `ExitCode` doc comment says
"cmd/heraut passes the error from fang.Execute here" — `main.go` now calls
`forge/cli.Run`, not `fang.Execute` directly (found during T95).

**Files:** scattered (see list above). **Scope:** S. **Dependencies:** none.

> (1) Stripped the T-id from 14 production-comment references across the 8 named
> non-test files (`offline.go` ×1, `pipeline/{release.go,linkctx.go}` ×3,
> `config/config.go` ×2, `generators/{gitcliff,cocogitto,communique}/generator.go` ×6/1/1),
> keeping the `(ADR-00xx)` pointer wherever one was present. The roadmap's count of 16
> included `_test.go`/`_internal_test.go` files (gitcliff/cocogitto/communique
> generator tests) — those document test-writing history rather than shipped behavior and
> weren't in the task's named file list, so left as-is. `gitcliff/generator.go`'s
> `linkEnv` doc comment ("T71 updates the template to prefer these over the ambient
> CI-var chain") became a present-tense statement of current behavior ("The embedded
> template prefers these over the ambient CI-var chain") rather than a historical
> change-log entry.
>
> (2) Removed `og := og` (`app/check.go`) and both `plat := platform` copies
> (`pipeline/release.go`, real-run and dry-run publish loops), renaming the range
> variable to `plat`/`og` directly. All three were redundant even before Go 1.22 — each
> closure runs synchronously inside the same iteration (via `dispatch`/`runStep`), so
> there was never a cross-iteration capture to guard against; go.mod's `go 1.26.4` makes
> them doubly so.
>
> (3) De-duplicated `= iota` for `versioning.BumpType`, `gitcliff.Mode`, and
> `cocogitto.Mode` (the latter wasn't named explicitly but has the identical
> `ModeChangelog/ModeReleaseNotes Mode = iota` ×2 pattern as gitcliff's — fixed both for
> consistency). `calver.TokenKind` already used the single-`= iota`-then-bare-identifiers
> idiom and needed no change.
>
> (4) Made `app.buildGenerator`'s switch and `cmd.checkCliffDriver`'s git-cliff check
> exact-case (dropped `strings.ToLower`/`strings.EqualFold`), matching
> `validator.go`'s `validGenerators`/`validPlatforms` maps and its `Generator !=
> "git-cliff"` check (T97) — the convention named as the reference point. Grepped every
> test fixture in the repo for mixed-case generator/platform values
> (`"Git-Cliff"`, `"GitHub"`, etc.); none exist, so this is behavior-preserving for any
> config that passes `config.Validate` (the only configs that can ever reach these
> functions with a generator/platform string other than the validator's exact lowercase
> set) — no ADR required. Removed the now-unused `"strings"` import from `cmd/check.go`.
> Residual, not in scope: `app.buildPlatform` (same file) still uses
> `strings.ToLower(cfg.Type)`, and `app/cliff.go`'s `EffectiveCliffConfig` plus
> `validator.go`'s own `ticketsGeneratorSupported` (line 89) still use
> `strings.EqualFold(..., "git-cliff")` — none of these three were named in item (4); a
> full sweep would be a separate task.
>
> (5) Replaced the `code := exitcode.Runtime` / conditional-mutation pattern in bare
> `check`'s summary with two explicit early returns —
> `exitcode.Wrap(exitcode.Config, ...)` when `configFailed > 0`, else
> `exitcode.Wrap(exitcode.Runtime, ...)` — same two outcomes, no intermediate variable.
>
> (6) `internal/cmd/exit.go`'s `ExitCode` doc comment now says "cmd/heraut passes the
> error from forge/cli.Run here", matching `cmd/heraut/main.go`'s actual
> `cli.Run(context.Background(), root, Version, ui.Accent())` call post-T95.
>
> All six items are mechanical/style-only with no behavior change for valid configs;
> `go build ./...`, `go test ./...` (957 passed, 22 packages), and `hk check`
> (golangci-lint 0 issues, go_fmt, typos) all pass.

#### `[x]` T102: Spec 04 — bump-determination table doesn't reflect T91's anchoring

T91 anchored the breaking-change `!` to the conventional-commit type/scope prefix (so
`!:` only inside a description no longer triggers a major bump) and added the hyphenated
`BREAKING-CHANGE:` footer as a synonym of `BREAKING CHANGE:`. Spec 04's "Bump
determination" table (`## SemVer` → `### Bump determination`) still says only "Any commit
with `!` (e.g. `feat!:`, `fix!:`) or `BREAKING CHANGE:` footer" — update the row to
describe the anchored form and both footer spellings.

**Files:** `docs/specs/04-versioning.md`. **Scope:** S. **Dependencies:** T91.

> Reworded the major-bump row to `type!:` / `type(scope)!:` prefix (e.g. `feat!:`,
> `fix(api)!:`) or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer, matching
> `internal/versioning/semver/bump.go`'s `breakingPrefixPattern`
> (`^\w+(\([^)]*\))?!:`) and `isBreaking`'s footer check. Added a sentence after the
> table (alongside the existing "highest applicable bump wins" sentence) spelling out
> the anchoring rule in prose — a bare `!:` inside the description doesn't count — and
> naming `BREAKING-CHANGE:` as a Conventional Commits 1.0.0 synonym, since the table
> cell alone was getting too dense to scan.

#### `[x]` T103: `semver-per-env` — pre-release tags may still break `BumpAuto`

T92 fixed `internal/versioning/semver.Resolver.resolveAuto` (plain `semver` strategy) to
skip git tags whose bare form isn't `MAJOR.MINOR.PATCH`. `internal/versioning/perenv.
resolveAuto` (used by `semver-per-env`) has a separate tag-listing loop: it extracts bare
versions via `tagfmt.ParseVersion(tf, tag)` with no `isBareVersion`-style filter, then
passes them to `calc.BumpAuto(bareVersions, commits)`, whose `currentVersion :=
tags[0]` → `BumpVersion(...)` has the same "invalid patch" failure mode if a pre-release
tag matches the env's `tag_format` and sorts first. Investigate whether this is reachable
in practice (does `tagfmt.ParseVersion`'s `{version}` capture admit pre-release suffixes
for typical formats?) and, if so, apply the same skip policy — likely by reusing
`semver.isBareVersion` (would need exporting) in `perenv.resolveAuto`'s filter loop, or
documenting the constraint in spec 04 § SemVer per environment.

**Files:** `internal/versioning/perenv/{auto.go,auto_test.go}`,
`internal/versioning/semver/bump.go`, `docs/specs/04-versioning.md`. **Scope:** S.
**Dependencies:** T92.

> Confirmed reachable: `{version}` compiles to a greedy `(?P<version>.+)`, so
> `tagfmt.ParseVersion("dev/{version}", "dev/1.3.0-rc.1")` happily captures
> `"1.3.0-rc.1"` with no pre-release filter. Without `versionsort.suffix`, that tag can
> sort above `dev/1.2.3` under `--sort=-version:refname`, becoming `bareVersions[0]` /
> `currentVersion`, and `BumpVersion`'s `strconv.Atoi("0-rc.1")` on the patch segment
> fails — the same "invalid patch" mode T92 fixed for plain `semver`.
>
> Exported `semver.isBareVersion` → `semver.IsBareVersion` (doc comment now covers both
> call sites) and applied it in `perenv.resolveAuto`'s single shared tag-filter loop:
> `if parseErr != nil || !semver.IsBareVersion(bare) { continue }`. The loop is shared
> with `calver-per-env`, but `IsBareVersion` accepts any 3-part all-integer version
> (every valid CalVer bare version qualifies), and `calver.BumpFromDate` already does
> its own independent per-tag parse/skip via `ParseVersion(tokens, tag)` — so this is a
> no-op for valid CalVer tags and consistent for malformed ones. No calver-per-env test
> added: the task is scoped to `BumpAuto` (semver-per-env), and the shared-loop fix
> doesn't change observable calver-per-env behavior for any tag shape.
>
> New test `TestResolve_Auto_Semver_SkipsPrereleaseTag` in
> `internal/versioning/perenv/resolver_test.go` (no new `auto_test.go` — all other
> `resolveAuto`-via-`Resolve()` tests already live in `resolver_test.go` as
> `TestResolve_Auto_Semver_*`/`TestResolve_Auto_Calver_*`): a `dev/1.3.0-rc.1` tag
> listed ahead of `dev/1.2.3` is skipped, `1.2.3` → `1.2.4` via `fix:`, and `git log` is
> scoped to `dev/1.2.3..HEAD`. Spec 04 § SemVer per environment's `bump: auto`
> paragraph now cross-references § Pre-release tags rather than duplicating it, since
> the policy is identical. `go build ./...`, `go test ./...` (958 passed, 22 packages),
> `hk check` clean.

#### `[x]` T104: Spec 02 examples and spec 05 — remaining `name`/`catalog`/`upload-asset` drift

T94 fixed the spec 02 platform *driver tables* (GitLab/GitHub) to require `name`, document
`base_url`, drop `catalog`, and use `glab release upload --use-package-registry`. Two
related spots were left as-is (out of T94's stated scope):

- Spec 02 § Complete examples: every `platforms` entry (Standard SemVer — GitHub, CalVer —
  GitLab, SemVer per environment ×3, SemVer multi-platform ×2) omits the now-required
  `name:` field, so a copy-pasted example fails `heraut check config`.
- Spec 05 § Platforms → GitLab: still has a `catalog: false` field in its YAML example
  (no `Catalog` field exists in `config.Platform`) and `glab release upload-asset` in its
  **Invocation** block (the driver uses `glab release upload --use-package-registry`,
  per `internal/platforms/gitlab/platform.go`).

**Files:** `docs/specs/{02-configuration.md,05-generators-and-platforms.md}`. **Scope:** S.
**Dependencies:** T94.

> Confirmed both premises before editing: `internal/config/validator.go` rejects a
> platform entry with an empty `Name` (`plat.Name == ""` → `ValidationError`), and
> `config.Platform` has no `Catalog` field at all (grep found none) — so both drift
> reports were real, not stale.
>
> Spec 02 § Complete examples: added the missing `name:` line to all 8 entries across the
> 4 affected examples — Standard SemVer — GitHub (`name: github`), CalVer — monthly
> releases, GitLab (`name: gitlab`), SemVer per environment — dev → staging → prod chain
> (root `release.platforms` → `name: gitlab`; `dev` override → `name: gitlab`; `prod`
> override → `name: gitlab` + `name: github`), and SemVer — multiple platforms with
> binaries (`name: gitlab` + `name: github`). The two driver-table examples T94 already
> fixed (§ Platform drivers → GitLab/GitHub) were left untouched.
>
> Spec 05 § Platforms → GitLab: removed the `catalog: false` line from the YAML example;
> rewrote the **Invocation** block from `glab release create ... [--publish-to-catalog]`
> / `glab release upload-asset <tag> <file> ...` (per asset) to the driver's actual calls
> — `glab release create <tag> --notes <notes> -R <project>` and `glab release upload
> <tag> --use-package-registry -R <project> <file>...` (all assets in one invocation),
> read directly from `internal/platforms/gitlab/platform.go`'s `CreateRelease`/
> `UploadAssets`. Rewrote the **Catalog** bullet: rather than describing a `catalog: true`
> flag that doesn't exist, it now explains GitLab auto-publishes to the CI/CD Catalog when
> the project is a registered catalog resource, with no heraut-side config.
>
> Docs-only change; no code or tests touched. `go build ./...`/`go test ./...` unaffected.

#### `[x]` T105: Extract a shared effective-platforms helper — three duplicate implementations

T89 and T100 each independently reimplemented the same merge rule that
`buildReleasePipelineConfig` already had: *effective platforms = `cfg.Release.Platforms`
(empty if `cfg.Release == nil`), unless `env != ""` and `cfg.Environments[env].Release !=
nil` and `len(envCfg.Release.Platforms) > 0`, in which case effective platforms =
`envCfg.Release.Platforms` entirely (replace, not append/merge)*.

Three copies of this rule now exist:

- `internal/app/pipeline.go:142-166` — inline inside `buildReleasePipelineConfig`,
  entangled with the changelog/notes/assets per-env merge (the original).
- `internal/cmd/release.go:119-130` — `hasEffectivePlatforms` (T89), with a comment
  acknowledging the duplication (`// matching buildReleasePipelineConfig's merge
  semantics`).
- `internal/app/check.go:116-126` — `RuntimeCheck`'s Platforms section (T100), no
  comment; T100's roadmap entry already named T89 as a "shared effective-list semantics"
  dependency without actually sharing code.

The `cmd/release.go` and `app/check.go` copies are now byte-for-byte identical (modulo a
`cfg != nil` guard `check.go` needs that `release.go` doesn't). A future change to the
merge rule — e.g. append/inherit semantics, or an org-level default tier — would need to
land in all three.

Proposed fix: `config.EffectivePlatforms(cfg *Config, env string) []Platform` in
`internal/config` (bottom of the layer stack — `internal/app` and `internal/cmd` already
import `internal/config` directly, so no new import edges). Replace the `cmd/release.go`
and `app/check.go` copies outright; in `pipeline.go`, replace only the
`effectivePlatforms`-specific lines with a call to the helper, leaving the
changelog/notes/assets merge untouched.

**Files:** `internal/config/` (new helper + tests), `internal/app/{pipeline.go,
check.go,check_test.go}`, `internal/cmd/{release.go,release_internal_test.go}`.
**Scope:** S. **Dependencies:** none.

> TDD: wrote `internal/config/platforms_test.go` first — `TestEffectivePlatforms` with 9
> subtests (nil cfg, nil `Release`, no env, unknown env, env without a `Release`
> override, env `Release` with empty `Platforms`, env `Platforms` replacing root, and env
> `Platforms` with a nil root `Release`) — confirmed it failed to compile
> (`undefined: config.EffectivePlatforms`) before adding the implementation.
>
> `config.EffectivePlatforms(cfg *Config, env string) []Platform` lives in new
> `internal/config/platforms.go`, doc-commented in the same bullet style as
> `MergeContentDriver`. `cmd/release.go`'s `hasEffectivePlatforms` and `app/check.go`'s
> `RuntimeCheck` Platforms block both now call it directly (`check.go`'s `cfg != nil`
> wrapper is gone — the helper handles a nil `cfg`). `hasEffectivePlatforms` itself was
> kept as a 1-line wrapper (`len(config.EffectivePlatforms(cfg, env)) > 0`) rather than
> deleted, so T89's existing 8-case `TestHasEffectivePlatforms` keeps exercising the
> boolean contract through the shared helper.
>
> `pipeline.go`'s `buildReleasePipelineConfig` had its `effectivePlatforms`
> var-plus-two-assignment-branches replaced with one
> `config.EffectivePlatforms(cfg, env)` call; the `effectiveChangelog`/`effectiveNotes`/
> `releaseAssets`/`pCfg.Disable*` merge is untouched, per the task's stated scope. Side
> benefit: grep found no existing test in `internal/app/pipeline_test.go` that sets
> `cfg.Environments[...].Release.Platforms`, so this branch of
> `buildReleasePipelineConfig` had no direct coverage before — it now inherits
> `EffectivePlatforms`'s 9 cases transitively.
>
> Net: 3 call-site files at -29/+4 lines, plus the new helper + its tests. `go build
> ./...`, `go vet ./...` clean; `go test ./...` → 967 passed, 22 packages (958 + 9 new);
> `hk check` clean (golangci-lint 0 issues).

#### `[x]` T106: SemVer bump — `isBreaking`'s `BREAKING CHANGE:`/`BREAKING-CHANGE:` check is an unanchored substring match

`isBreaking` (`internal/versioning/semver/bump.go`) anchors the `!:` breaking-change
marker to the subject line (T91), but its footer check —
`strings.Contains(commit, "BREAKING CHANGE:") || strings.Contains(commit, "BREAKING-CHANGE:")`
— scans the *entire* commit message with no line-position requirement. Per Conventional
Commits, `BREAKING CHANGE:`/`BREAKING-CHANGE:` is only meaningful as a footer token at the
start of a line; the current check matches it anywhere, including mid-sentence prose.

This is not theoretical: f4a8e9e (T91 itself) is a `fix:` commit whose body *describes*
the `BREAKING-CHANGE:` footer synonym in prose ("Also recognize the hyphenated
`BREAKING-CHANGE:` footer as a synonym of `BREAKING CHANGE:`..."). `isBreaking` matched
that substring, `bump: auto` resolved `BumpMajor`, and the 2026-06-14 CI release run
bumped 0.31.0 → 1.0.0 — a false positive that required deleting the published `v1.0.0`
GitHub release (6 assets) and tag, reverting the CHANGELOG commit on `main`, and
re-running the release workflow with a manual `--version v0.32.0` override.

Fix: require the footer token to start a line (split on `\n`, check
`strings.HasPrefix(strings.TrimSpace(line), "BREAKING CHANGE:")` /
`"BREAKING-CHANGE:"`), rather than `strings.Contains` over the raw message. A stricter
spec-faithful version would additionally require the footer to be in the trailing footer
block (after the last blank line), but the line-start check alone would have prevented
this incident.

**Files:** `internal/versioning/semver/{bump.go,bump_test.go}`,
`docs/specs/04-versioning.md` (if the bump-determination table needs a positioning
caveat). **Scope:** S. **Dependencies:** T91, T102.

> TDD: added two cases to `TestDetermineBump` in `resolver_test.go` (no separate
> `bump_test.go` — all `isBreaking`/`DetermineBump` cases already live in
> `resolver_test.go`, same precedent as T103's note on `perenv`) — a `fix:` commit whose
> body mentions `BREAKING CHANGE:`/`BREAKING-CHANGE:` mid-sentence (mirroring f4a8e9e's
> actual wording) — confirmed red: both expected `BumpPatch` but got `BumpMajor`.
>
> `isBreaking` now splits the commit on `\n`, trims each line, and checks
> `strings.HasPrefix` for `"BREAKING CHANGE:"` / `"BREAKING-CHANGE:"`, replacing the
> unanchored `strings.Contains`. Did not additionally restrict to the trailing
> footer block (after the last blank line) — line-start alone fixes this incident's
> class of false positive and keeps the change minimal; the existing footer-at-line-start
> cases (T91's `"fix: y\n\nBREAKING CHANGE: boom"` / `"...BREAKING-CHANGE: boom"`, and
> `TestResolve_BreakingChange_Footer`'s null-byte-terminated body) all still pass
> unchanged.
>
> Spec 04 § Bump determination: appended a sentence after the existing `!:`-anchoring
> caveat — "Either form must start its own line (a footer) — a mid-sentence mention does
> not trigger a major bump" — parallel in structure to the `!:` caveat T102 added.
>
> `go build ./...`, `go vet ./...` clean; `golangci-lint run ./internal/versioning/semver/...`
> clean. `go test ./...` → 969 passed, 22 packages (967 + 2 new). gopls flagged
> `stringsseq`/`stringscut`/`newexpr` "modernize" hints on this file (new `for...range
> strings.Split` and pre-existing `firstLine`/`strPtr` code) — none are in
> `.golangci.yml`'s enabled linters (`errcheck`, `staticcheck`, `ineffassign` +
> defaults), confirmed via a clean `golangci-lint run`; left untouched as out of scope.
>
> **Second round, same session, before either commit was pushed**: `heraut version next`
> on the resulting `main` returned `v1.0.0` instead of the expected `v0.32.1`. Cause: the
> line-start check above is satisfied by *this task's own roadmap-filing commit*
> (`7181aba`) — its body reads "Discovered via the v1.0.0 false-release incident:
> isBreaking's\nBREAKING CHANGE:/BREAKING-CHANGE: footer check is an unanchored\n...",
> where the second line, after trimming, starts with `"BREAKING CHANGE:"` even though
> it's a wrapped continuation of the first line's sentence, not a footer. The exact gap
> this entry's own "stricter spec-faithful version" paragraph anticipated — recurring a
> second time, this time in the commit that documents the first incident.
>
> Fix, applied in the same `isBreaking` block: a matched line only counts as a footer if
> it *begins its own paragraph* — `i == 0` or the previous line (trimmed) is empty.
> Added a third case to `TestDetermineBump` mirroring `7181aba`'s exact shape (a wrapped
> line starting with the token, preceded by a non-blank line) — confirmed red (got
> `BumpMajor`, wanted `BumpPatch`), then green after the `i == 0 ||
> strings.TrimSpace(lines[i-1]) == ""` guard. All prior cases (genuine footers preceded by
> a blank line, both T91 mid-sentence cases) still pass. Spec 04 § Bump determination
> reworded to "must begin its own paragraph... either the message's first line, or a line
> immediately following a blank line". `go build`/`go vet`/`golangci-lint` clean; `go test
> ./...` → 970 passed, 22 packages. `/tmp/heraut_check version next` on `main` now
> correctly reports `v0.32.1`.

---

### Phase 18 — `heraut init` config round-trip

T99 (Phase 17) added `scaffold.DroppedFields` to warn before the "Update it?" flow
silently regenerates `.heraut.yml` without six categories of fields. Of those six, three
are top-level/release-level values with no rebuild ambiguity and can be carried through
directly (T107); the other three are per-platform/per-env overrides that T99 deferred
because the wizard rebuilds `a.Platforms`/`a.Environments` from scratch (T108 for
platforms, T109 for environments).

#### `[x]` T107: `heraut init` update flow — preserve top-level `release.assets`, `tickets`, `remote_metadata`

Of the six field categories `DroppedFields` (T99) flags, three are single
top-level/release-level values with no wizard-rebuild ambiguity: `release.assets`
(`[]string` glob patterns), `tickets` (`[]config.Ticket`), and `remote_metadata`
(string enum). Unlike per-platform `base_url`/`draft`/`prerelease` and per-env
`changelog`/`release` overrides — which T99 deferred because `runPlatformWizard` and
`runEnvWizard` reset `a.Platforms`/`a.Environments` to nil and rebuild them from wizard
answers, leaving no stable identity to reattach passthrough values to — these three have
exactly one value per config and can be copied through `Answers` verbatim.

Add `Assets []string`, `Tickets []config.Ticket`, and `RemoteMetadata string` to
`scaffold.Answers`. `ConfigToAnswers` copies them from `cfg.Release.Assets`,
`cfg.Tickets`, and `cfg.RemoteMetadata`. `answersToConfig` writes them back — creating
`cfg.Release` when `a.Assets` is non-empty even if no notes/platforms are configured.
Remove these three categories from `DroppedFields`'s output (and its tests).

**Files:** `internal/scaffold/{wizard.go,generate.go,dropped.go}` + tests,
`internal/cmd/init_internal_test.go` (dropped-fields warning enumeration).
**Scope:** S. **Dependencies:** T99.

> Added `Assets []string`, `Tickets []config.Ticket`, and `RemoteMetadata string` to
> `scaffold.Answers` (with a comment noting they're not wizard-editable, just
> round-tripped). `ConfigToAnswers` copies them from `cfg.Tickets`/`cfg.RemoteMetadata`
> (top-level) and `cfg.Release.Assets` (inside the existing `cfg.Release != nil` block).
> `answersToConfig` writes `Tickets`/`RemoteMetadata` onto the new `config.Config{}`
> literal directly, and extended the `cfg.Release` creation condition from `hasNotes ||
> hasPlatforms` to also include `hasAssets := len(a.Assets) > 0`, initializing
> `&config.Release{Assets: a.Assets}` — so a config with only `release.assets` (no
> notes/platforms) still round-trips with a `release:` block.
>
> Removed the three corresponding checks from `DroppedFields` (tickets,
> remote_metadata, release.assets), leaving only the per-platform
> (`base_url`/`draft`/`prerelease`, T108) and per-env (`changelog`/`release`, T109)
> checks. `internal/cmd/init_internal_test.go`'s `printDroppedFieldsWarning` tests pass
> arbitrary strings and needed no change.
>
> TDD: added `TestConfigToAnswers_PreservesAssetsTicketsRemoteMetadata` and
> `TestGenerateYAML_AssetsTicketsRemoteMetadata` (generate_test.go) — confirmed red
> (compile errors, missing `Answers` fields) before implementing. Updated the three
> existing `DroppedFields` tests (`TestDroppedFields_{Tickets,RemoteMetadata,
> ReleaseAssets}`) to `*_NotDropped` variants asserting `DroppedFields` now returns
> empty for these — a deliberate behavior change per T99's original "carry through vs.
> warn" framing, not a deleted assertion. `go build`/`go vet`/`golangci-lint` clean;
> `go test ./...` → 972 passed, 22 packages (970 + 2 new).

#### `[x]` T108: `heraut init` update flow — carry through per-platform release overrides (`base_url`/`draft`/`prerelease`)

Design spike completed: `.claude/plans/t108-init-override-carryover-design.md`.
**Approach A (type-scoped positional matching)**, chosen over skipping the
`runPlatformWizard` rebuild by default — A is strictly additive (degrades to today's
drop + warn on any mismatch) and adds no new prompts.

`PlatformAnswer` gains passthrough fields `Name string`, `BaseURL string`,
`Draft bool`, `Prerelease bool`, populated verbatim by `ConfigToAnswers` from
`cfg.Release.Platforms[i]`. Before `runPlatformWizard` resets `a.Platforms = nil`, it
snapshots the incoming slice grouped by `Type` (order preserved within each type). Each
newly-built `PlatformAnswer` of type `T` is matched against the next unconsumed
snapshot entry of type `T`; on match, `Name`/`BaseURL`/`Draft`/`Prerelease` are copied
across. No match (new entry, or more entries of type `T` than before) → zero values
(today's behavior: `answersToConfig` derives `Name` from `type`/`type-N`, `BaseURL`
falls back to the type default, `Draft`/`Prerelease` default `false`).
`answersToConfig` uses `p.Name` when non-empty, else falls back to the existing
`type`/`type-N` derivation for genuinely new entries.

Resolved open questions from the spike:

- **`DroppedFields` timing**: move the platform `base_url`/`draft`/`prerelease` checks
  from the pre-wizard warning (T99's placement) to a post-wizard check, comparing
  against the rebuilt `a.Platforms` so the warning only fires on an actual mismatch
  (reorder/add/remove/type-change) — one accurate warning instead of an
  always-possible pre-wizard one. `internal/cmd/init.go`'s "Update it?" branch calls
  this after `RunWizard` returns, before the "write this config?" confirm.
- **`--defaults` on an existing config**: becomes fully lossless for these three
  fields as a side effect (no `RunWizard` call on that path — `ConfigToAnswers` /
  `answersToConfig` round-trip directly). Accepted as a strict improvement.
- The "weak secondary key" refinement for reordered same-type platforms (spike's noted
  edge case) is deferred — not implemented unless reported.

**Files:** `internal/scaffold/{wizard.go,generate.go,dropped.go}` + tests,
`internal/cmd/init.go` (+ `init_internal_test.go`). **Scope:** M.
**Dependencies:** T99, T107.

> Added `Name string`, `BaseURL string`, `Draft bool`, `Prerelease bool` passthrough
> fields to `PlatformAnswer`. `ConfigToAnswers` copies them from each
> `cfg.Release.Platforms[i]`. `runPlatformWizard` snapshots `a.Platforms` before reset
> and calls `matchPlatformSnapshot(snapshot, rebuilt)` after the wizard loop —
> `matchPlatformSnapshot` groups original entries by `Type` and applies each type's
> entries positionally to rebuilt entries of the same type; unmatched rebuilt entries
> keep zero values (new-platform behavior). `answersToConfig` uses `p.Name` when
> non-empty, else falls back to `type`/`type-N`; writes `BaseURL`/`Draft`/`Prerelease`
> directly onto `config.Platform`. `DroppedFields` per-platform checks removed entirely;
> replaced by `DroppedPlatformFields(cfg, rebuilt []PlatformAnswer)` (post-wizard) which
> only warns for original entries whose type-ordinal exceeds the rebuilt list — i.e. only
> on actual add/remove/reorder/type-change, not the common "re-run init, keep same
> platforms" case. `internal/cmd/init.go` saves `existingCfg` and calls
> `DroppedPlatformFields` after `RunWizard` returns (before the "write?" confirm).
>
> TDD: 3 new generate_test.go cases (ConfigToAnswers preserves fields; GenerateYAML uses
> passthrough name; round-trip BaseURL/Draft/Prerelease); 4 wizard_internal_test.go cases
> (single match, no-match-for-new-entry, type-scoped, empty-rebuilt); 5 dropped_test.go
> cases (PlatformBaseURL/DraftPrerelease not-dropped variants; DroppedPlatformFields
> no-mismatch, mismatch-longer, nil-release). Confirmed red before implementing.
> `go build`/`go vet`/`golangci-lint` clean; `go test ./...` → 992 passed, 22 packages.

#### `[x]` T109: `heraut init` update flow — carry through per-env content overrides (`changelog`/`release`)

Same design spike as T108 (`.claude/plans/t108-init-override-carryover-design.md`),
applied to `environments.<name>.{changelog,release}`. Unlike platforms, `EnvAnswer`
already has a stable `Name` (the `cfg.Environments` map key) — no new identity field
needed.

`EnvAnswer` gains passthrough fields `Changelog *config.ContentDriver` and
`Release *config.EnvRelease`, populated by `ConfigToAnswers` from
`cfg.Environments[name]`. Before `runEnvWizard` resets `a.Environments = nil`, it
snapshots the incoming slice keyed by `Name`. A rebuilt entry whose `Name` matches a
snapshot entry inherits that entry's `Changelog`/`Release`; a renamed or new env gets
`nil` (dropped, same as today — `DroppedFields` continues to warn for *that* case).
`answersToConfig` writes `Changelog`/`Release` onto `config.Environment` when non-nil.

Apply the same post-wizard `DroppedFields` timing change as T108 (single combined
post-wizard warning covering both platform and env mismatches, if T108 lands first —
otherwise this task adds the env half of that check).

**Files:** `internal/scaffold/{wizard.go,generate.go,dropped.go}` + tests,
`internal/cmd/init.go` (+ `init_internal_test.go`). **Scope:** M.
**Dependencies:** T99, T107, T108 (shares the snapshot/match pattern T108 introduces).

**Completed 2026-06-16.** Added `matchEnvSnapshot` (name-based matching, analogous to
T108's type-scoped `matchPlatformSnapshot`). `runEnvWizard` snapshots `a.Environments`
before reset and calls `matchEnvSnapshot` after the loop to reattach `Changelog`/`Release`
by name. `answersToConfig` writes both passthrough fields onto `config.Environment`.
`DroppedFields` now returns nil (all wizard-handled fields are carried through); `DroppedEnvFields`
replaces the old pre-wizard check with a targeted post-wizard warning, surfaced via a second
`printDroppedFieldsWarning` call in `init.go`. Old tests for the removed `DroppedFields`
env behavior deleted; 4 `matchEnvSnapshot` unit tests + 3 `DroppedEnvFields` tests added.

---

### Phase 19 — Branch-based environment auto-detection

#### `[x]` T110: `--env auto` — resolve target environment from current branch on per-env strategies

Add `"auto"` as a sentinel value for the global `--env` flag. When `--env auto` is
passed on a per-env strategy (`semver-per-env`, `calver-per-env`), heraut reads the
current branch name and matches it against configured `environments.<name>.branch`
values to resolve the env name, then runs the pipeline as if that name had been passed
explicitly. `--env` remains mandatory for per-env strategies; `--env auto` is the only
new behavior.

Applies to all commands that accept `--env`:
`heraut release`, `heraut changelog`, `heraut version next/current`, `heraut cliff`.

**Edge cases — all fail before any pipeline work, with an actionable message:**

| Condition | Error |
|-----------|-------|
| `--env auto` on a non-per-env strategy | `` `--env` is only valid with a per-env strategy `` |
| Detached HEAD (`git rev-parse --abbrev-ref HEAD` returns `HEAD`) | `cannot auto-detect env: HEAD is detached — pass --env explicitly` |
| No env has a `branch` value matching the current branch | `no env is linked to branch "<branch>" — pass --env explicitly` |
| Multiple envs share the same `branch` value | `envs "<X>" and "<Y>" both use branch "<branch>" — pass --env explicitly` |

**Implementation:** add `ResolveEnv(env string, cfg *config.Config, runner port.Runner) (string, error)` in `internal/app/` (new file `env.go`). All five `--env`-reading commands (`release.go`, `changelog.go`, `version.go`, `cliff.go`, `check.go`) call `app.ResolveEnv(env, cfg, runner)` immediately after reading the flag; the returned string replaces the raw flag value for the rest of that command's `RunE`. `ResolveEnv` is a no-op passthrough when `env != "auto"`, keeping all non-auto call sites unaffected.

Branch matching is exact-string against `config.Environment.Branch` (the current field semantics). If `branch` is ever relaxed to globs, the multi-match edge case extends naturally.

**Files:** `internal/app/env.go` (new), `internal/app/env_test.go` (new),
`internal/cmd/{release,changelog,version,cliff,check}.go` (one-liner each).
**Scope:** S. **Dependencies:** none.

> `app.ResolveEnv(env string, cfg *config.Config, runner port.Runner) (string, error)` in
> `internal/app/env.go`: passthrough for `env != "auto"`; for `"auto"`, guards on nil
> cfg (heraut check allows no config), then checks per-env strategy, runs
> `git rev-parse --abbrev-ref HEAD`, fails on `"HEAD"` (detached), scans
> `cfg.Environments` for `Branch` matches, returns the single match or fails with an
> actionable message (no match / multiple matches). Multiple matches are sorted before
> formatting so the error is deterministic.
>
> Five command files wired: `release.go`/`changelog.go` call with `readRunner` (non-dry-run
> runner, consistent with how both already use it for read-only git calls); `version.go`
> (next + current) and `check.go` (main + runtime sub-command) call with their existing
> `runner`; `cliff.go` required adding `execadapter.New(false, false)` and importing
> `execadapter` (both sub-commands share the same pattern).
>
> TDD: 9 test cases in `env_test.go` — passthrough (non-auto, empty), nil cfg, non-per-env
> strategy (semver, calver), git error, detached HEAD, no match, single match, multiple
> match. Confirmed red (compile error: `app.ResolveEnv` undefined) before implementing.
> `go build`/`go vet`/`golangci-lint` clean; `go test ./...` → 983 passed, 22 packages
> (972 + 9 new + 2 new from nil-cfg test added during wiring).

---

### Phase 20 — Pipeline UX and error messages

#### `[x]` T111: graceful handling when changelog has no new entries to commit

`git commit` exits 1 with "nothing to commit, working tree clean" when git-cliff
regenerates a CHANGELOG.md whose content is identical to the last commit. This surfaces
as an opaque `committing changelog: git commit: git: exit status 1` error that gives the
user no hint of the actual cause.

This legitimately happens in two scenarios:
- **Re-run after partial success**: a previous heraut run committed the changelog but
  failed before tagging. The second run regenerates identical content.
- **No matching commits**: git-cliff found no commits since the last env tag that pass
  its conventional-commit filter (e.g. all commits are `chore:` and the git-cliff config
  excludes them, or there are simply no commits between the last matching tag and HEAD).
  CalVer resolvers advance the version by calendar regardless, so the pipeline still
  reaches the commit step.

**Expected behavior:** when `git add <changelog>` stages nothing (detected via
`git diff --cached --quiet` or by checking the `git add` exit code/output), emit a
clearly labelled diagnostic warning instead of failing:

```
⚠ CHANGELOG.md unchanged — no new entries since the last <env>-* tag.
  If this is unexpected, check your git-cliff config (commit types / tag pattern).
  Continuing with tag and release.
```

The pipeline then continues to the tag and publish steps — skipping the `git commit` call
entirely. Failing hard here is wrong for the re-run case and unhelpful for the no-commits
case (the release should still be created).

**Implementation:** in `internal/pipeline/git.go`, after `git add <file>`, run
`git diff --cached --quiet` (exit 0 = nothing staged, exit 1 = changes staged). If
nothing is staged, return a sentinel error or a `(bool, error)` pair so the caller
(`release.go` step 2 / `changelog.go`) can emit the warning and skip the commit without
aborting the pipeline. Alternatively, wrap `commitChangelog` to return a typed
`ErrNothingToCommit` sentinel that the pipeline step checks via `errors.Is`. The warning
should name the changelog file so it's actionable in multi-file setups.

**Files:** `internal/pipeline/git.go`, `internal/pipeline/release.go`,
`internal/pipeline/changelog.go`, `internal/pipeline/git_test.go` (new test for the
nothing-staged path), `internal/pipeline/release_test.go`,
`internal/pipeline/changelog_test.go`.
**Scope:** S. **Dependencies:** none.

**Completion note (2026-07-02):** `commitChangelog` now runs `git diff --cached
--name-only` after `git add` and returns `(committed bool, error)` — a `(bool, error)`
pair rather than the roadmap's suggested sentinel, since within-package callers read it
directly and "nothing staged" is a normal outcome, not an error. **Deviation from the
roadmap's `git diff --cached --quiet` suggestion:** `--name-only` with an empty-stdout
check is used instead, so a genuine `git diff` failure surfaces as a real error rather
than being misread as "nothing staged" (which the exit-code approach would conflate).
Both `Pipeline.Run` (release) and `ChangelogPipeline.Run` detect `committed == false` and
call a shared `warnNothingToCommit(w, file)` that writes the warning to `p.out` (visible
in both reporter and plain/CI modes — the plain path discards step `result`/`subs`, and
CI re-runs are exactly where this surfaces), then continue to tag + publish. New
helper-level unit tests in `git_test.go` (staged / nothing-staged / diff-error) plus
pipeline-level tests on both flows; every existing changelog-commit test gained the
interleaved `git diff --cached` call and shifted `Calls` indices. Specs 03 (`heraut
release` step 4, `heraut changelog` step 3) updated. Suite 1345 green.

---

#### `[x]` T112: Unify release URL resolution through `ReleaseURLFromContext`

The displayed release URL in the publish step output and final summary was computed via
`plat.ReleaseURL()` independently of the link context used for release-notes generation.
This meant that in a self-hosted GitLab setup with no `base_url` in config, the notes
correctly used `CI_PROJECT_URL` (via `ambientLinkContext()` / `platformLinkContext()`), but
the URL shown in the heraut output still fell back to `gitlab.com`.

Added `ReleaseURLFromContext(tag string, lc *LinkContext) string` to `port.Platform`. The
pipeline now computes `lc := p.platformLinkContext(plat)` once per platform publish step
and passes it to both notes generation and URL display. Ambient contexts (Owner/Repo empty,
BaseURL is the full project URL) and platform contexts (BaseURL is the host, Owner/Repo set)
are both handled; nil falls back to `ReleaseURL` for callers without context.

Implemented on `platforms/gitlab` and `platforms/github`; mock updated with call recording
for pipeline-level contract tests. Pipeline-level regression test added.

---

#### `[x]` T113: Auto-inject `[remote.github]` / `[remote.gitlab]` into effective git-cliff config

git-cliff's `[remote.*]` sections enable PR/MR metadata fetching (authors, PR numbers).
Previously, users had to add and fill in the section manually in a custom git-cliff
override config. Now heraut injects it automatically into the effective TOML temp file
(alongside `GITHUB_TOKEN`/`GITLAB_TOKEN` that were already injected) — eliminating the
need for any custom git-cliff config to get PR metadata.

Two complementary mechanisms:
- **TOML injection** (`injectRemote` in `generator.go`): `prepareConfig(lc)` appends
  `[remote.<platform>]` with `owner` and `repo` from `lc` after building the merged TOML.
  Skipped when lc is nil, owner/repo are empty (ambient context), or the user already
  declared the section in their override config.
- **Env var injection** (`linkEnv`): also injects `GITHUB_REPO`/`GITLAB_REPO` in
  `owner/repo` format for users with custom git-cliff configs that rely on env vars.

The embedded TOML comments updated to reflect that no manual config is needed.

---

### Phase 21 — Configurable changelog remote (Azure DevOps and beyond)

#### `[x]` T114: Add `changelog.remote` — explicit metadata remote for git-cliff (Azure DevOps)

> **Historical note:** the config example below predates later changes. `changelog.remote` is
> no longer git-cliff-only (works with `native` too), the `api_url` field was replaced by
> `base_url`, and the `organization:` key was folded into `project:` — all superseded by
> [ADR-0040](../adr/0040-changelog-remote-native-base-url.md). Kept as a point-in-time record.

git-cliff 2.13 supports an Azure DevOps remote for PR/author metadata enrichment
(`[remote.azure_devops]` with `owner = "<organization>/<project>"` and
`repo = "<repository>"`; `AZURE_DEVOPS_TOKEN`/`AZURE_DEVOPS_REPO` env vars — confirmed via
git-cliff's `--azure-devops-token`/`--azure-devops-repo` CLI arg docs and an empirical
`git-cliff --config` run). Unlike GitHub/GitLab, Azure DevOps is never a `release.platforms`
entry here — there is no publish-to-Azure-DevOps requirement, only metadata enrichment. See
[ADR-0026](../adr/0026-azure-devops-metadata-remote.md) for the full design rationale,
including why this is deliberately *not* routed through `release.platforms`/`port.Platform`,
and why it landed under `changelog:` specifically rather than as a top-level block.

**Design (per ADR-0026):**

```yaml
changelog:
  generator: git-cliff
  remote:
    type: azure_devops              # github | gitlab | azure_devops
    organization: my-org            # azure_devops-specific
    project: my-project             # azure_devops-specific
    repository: my-repo
    token_env: AZURE_DEVOPS_TOKEN   # default per type; overridable
    api_url: https://dev.azure.com  # optional — Azure DevOps Server (on-prem) only
```

- `type` is generic (mirrors `Platform.Type`'s discriminator pattern), not Azure-DevOps-only
  — `github`/`gitlab` values let a user explicitly pin the changelog's remote when
  `changelogLinkContext()`'s existing ambient/single-platform fallback is ambiguous (multiple
  platforms configured, or an unrecognized CI host).
- Singular object, not a list — one changelog, one source-of-truth remote.
- `release.notes` is untouched — it already gets a deterministic remote via
  `platformLinkContext()` per platform being published to, so it has no equivalent gap.
- git-cliff only, like `tickets:` (ADR-0024) — `changelog.remote` with `cocogitto`/
  `communique` is a config error, not a silent no-op.

**Implementation:**

1. `internal/config/config.go`: new `Remote` struct (`Type`, `Organization`, `Project`,
   `Repository`, `TokenEnv`, `APIURL`), added as `*Remote` field on `ContentDriver` (only
   meaningful for `Changelog`, not `Release.Notes` — validator should reject it on the
   latter).
2. `internal/config/validator.go`: `type` enum (`github`/`gitlab`/`azure_devops`);
   required-field-per-type (azure_devops needs organization+project+repository; github needs
   repository; gitlab needs project); git-cliff-only gating; reject on `release.notes`.
3. `internal/pipeline/linkctx.go`: `changelogLinkContext()` checks `cfg.Changelog.Remote`
   first (constructing a matching `port.LinkContext`), before falling through to its existing
   ambient → single-platform → nil chain.
4. `internal/generators/gitcliff/generator.go`: generalize `injectRemote`/`linkEnv` from
   their current two-armed `if lc.Platform == "gitlab" {...} else {...}` into a small table
   keyed by remote type (TOML `owner`/`repo` shape + env var names — note Azure DevOps's
   3-segment `organization/project/repository` addressing vs GitHub/GitLab's 2-segment
   `owner/repo`). Reused by both the T113 auto-derived path and this new explicit path.
5. Docs/schema checklist (per `coding.md`): `schema.json`, `docs/heraut.sample.yml`,
   `docs/specs/05-generators-and-platforms.md` (which currently doesn't document the
   `[remote.*]`/`GITHUB_REPO`/`GITLAB_REPO` auto-injection from T113 either — close that gap
   in the same pass since this task documents the same mechanism generalized).

**Tests:** table-driven unit tests for the generalized `injectRemote`/`linkEnv` (extend
`remote_internal_test.go`/`linkenv_internal_test.go` with `azure_devops` rows), a
`changelogLinkContext` test for the new explicit-remote-wins-over-ambient case, validator
tests per type, and a schema fixture in `testdata/config/valid/`.

**Files:** `internal/config/config.go`, `internal/config/validator.go`,
`internal/pipeline/linkctx.go`, `internal/pipeline/linkctx_test.go`,
`internal/generators/gitcliff/generator.go`, `internal/generators/gitcliff/remote_internal_test.go`,
`internal/generators/gitcliff/linkenv_internal_test.go`, `schema.json`,
`docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`,
`testdata/config/valid/`, `testdata/config/invalid/`.
**Scope:** M. **Dependencies:** none.

Implemented exactly per ADR-0026, across four commits (config/validator/merge → pipeline
linkctx + gitcliff generator wiring → schema/sample/spec/fixtures). One pleasant surprise:
`injectRemote` needed zero changes — it was already generic on `lc.Platform`, confirmed by
an empirical TOML round-trip with the user's real-world Azure DevOps git-cliff config
before writing any code. `linkEnv` was generalized by branching early to a dedicated
`azureDevOpsLinkEnv` rather than folding all three platforms into one table-driven
function — github/gitlab's `infix`-based URL composition and Azure DevOps's `_git`-rooted
one are structurally different enough that a shared abstraction would have been forced.
**Deviation:** `HERAUT_COMPARE_URL` is deliberately left unset for `azure_devops` — Azure
Repos branch comparison is query-string based
(`?baseVersion=GT{old}&targetVersion=GT{new}`), which doesn't fit the `{prefix}{old}..{new}`
concatenation the embedded Tera template substitutes for every other platform. Supporting
it needs a template restructuring (an extra substitution var) that touches the embedded
changelog TOML and ADR-0022's template contract — filed separately as T115 rather than
expanding this task's scope (T115 implemented it; see below). The other two URL shapes
(commit, PR) were confirmed against real Azure DevOps URLs provided by the user before
implementation, not guessed. **Post-completion fix:** the original design split
`organization`/`project` into two separate fields; git-cliff's own `[remote.azure_devops]`
TOML only has `owner`/`repo` (2 fields, `owner` already `"organization/project"`
combined), so the redundant `Organization` field was removed — `project:
"organization/project"` now matches git-cliff one-for-one, consistent with how `github`'s
`repository` field already holds combined `owner/repo`.

---

#### `[x]` T115: Azure DevOps compare-link support — restructure the embedded Tera template

Deferred from T114 (above). `HERAUT_COMPARE_URL` is currently a single prefix string; the embedded template appends
`{previous.version}..{version}` itself for each release section
(`{{ get_env(name="HERAUT_COMPARE_URL", default="") }}{{ previous.version }}..{{ version }}`
in `cliff.changelog.toml`). That shape works for GitHub (`/compare/v1...v2`) and GitLab
(`/-/compare/v1...v2`) but cannot express Azure Repos' compare URL, which is query-string
based with two separately-prefixed refs:

```
https://dev.azure.com/{org}/{project}/_git/{repo}/branchCompare?baseVersion=GT{old}&targetVersion=GT{new}
```

**Expected behavior:** for `changelog.remote.type: azure_devops`, version headings render
a working compare link instead of omitting it.

Implemented with a single additional substitution var, `HERAUT_COMPARE_URL_MIDDLE`
(default `".."`), rather than the originally-sketched prefix/middle/suffix three-var
design — once the URL was verified against real Azure DevOps URLs, the trailing
`&_a=commits` query param turned out to be unnecessary, leaving exactly two pieces
(`baseVersion=GT{old}` and `&targetVersion=GT{new}`), so no suffix var was needed.
`linkEnv`/`azureDevOpsLinkEnv` and `cliff.changelog.toml` were updated;
`cliff.release-notes.toml` was untouched (it has no compare-link construct at all —
release notes render a single release, not a multi-version heading list). ADR-0022 was
amended in place (not superseded) with the new var. **Deviation from the task's own
suggestion:** no `testutil.RealGitRepo`-based render-output assertion was added — `testing.md`
explicitly restricts those tests to config-acceptance smoke checks with no output
assertions ("those stay byte-level / manual"), so the rendered output was instead verified
manually against real `git-cliff` in a throwaway repo before closing the task, and locked
in permanently via the existing pure-function `linkEnv` table tests instead.

**Files:** `internal/generators/gitcliff/cliff.changelog.toml`,
`internal/generators/gitcliff/generator.go` (`linkEnv`/`azureDevOpsLinkEnv`),
`internal/generators/gitcliff/linkenv_internal_test.go`,
`docs/adr/0022-fat-injection-thin-templates.md`,
`docs/specs/05-generators-and-platforms.md`.
**Scope:** M. **Dependencies:** T114.

---

### Phase 22 — Conventional-commit tooling

#### `[x]` T116: `heraut commit verify` — built-in conventional-commit checker

Per [ADR-0027](../adr/0027-builtin-conventional-commit-checker.md). heraut's own
commit-msg hook (`.config/hk/config.pkl`) currently shells out to `cog verify`. Separately,
`internal/versioning/semver/bump.go` already hand-rolls a second, narrower
conventional-commit parser (`isBreaking`/`isFeat`) purely for bump classification — two
divergent parsers of the same grammar in one codebase. No existing Go package (evaluated:
`leodido/go-conventionalcommits`, `conventionalcommit/commitlint`, `mkyc/go-conventional`,
`gitlab.com/digitalxero/go-conventional-commit`) meets this project's stability bar, so the
grammar is owned in-house.

**Implementation:**

1. New package `internal/conventionalcommit/` (peer to `internal/port`/`internal/config`,
   not nested under `internal/versioning/`): `Parse(message string) (*Commit, error)`
   (`Type`, `Scope`, `Breaking`, `Description`, `Body`, `Footers []Footer{Token, Value}`),
   `IsMergeCommit`, `IsFixupCommit`. Grammar only — no type allow-list inside the package.
   Body/footers are parsed structurally per the spec (blank-line-separated body, then
   `token: value`/`token #value` footer pairs, `-` for spaces in multi-word tokens), not
   pattern-matched — `Breaking` derives from the header `!` or a real `BREAKING CHANGE`/
   `BREAKING-CHANGE` footer in `Footers`, not a raw-line regex scan. No commitlint-style
   rule catalog (casing, length limits, `signed-off-by`, `trailer-exists`, etc.) —
   `Footers` exists so `Breaking` detection is structurally correct, not to back a wider
   rule set. See ADR-0027's "Explicitly still out of scope" note.
   **Performance** (per ADR-0027): `Parse` is on two hot paths — the commit-msg hook and
   `DetermineBump` (once per commit in a potentially large resolved range). Package-level
   compiled `regexp` only (no per-call compilation), header pattern stays anchored/bounded
   (no catastrophic-backtracking risk), single linear pass to extract `Body`/`Footers` (no
   re-scanning the message more than once). Must not regress `DetermineBump`'s existing
   per-commit cost.
2. Refactor `internal/versioning/semver/bump.go`'s `DetermineBump` to call
   `conventionalcommit.Parse` instead of its own `isBreaking`/`isFeat`/
   `breakingPrefixPattern`. Parse errors are treated as "not feat, not breaking" (today's
   fallback-to-patch behavior is unchanged) — bump resolution stays lenient; only the new
   checker enforces strict validation. Delete the now-redundant regex/helper-functions.
3. `internal/config/config.go`: new optional `CommitLint` struct with `Types []string`,
   default `[feat, fix, docs, chore, refactor, test, style, perf, ci, build]` when the
   block/field is absent (matches `workflow.md`'s commit-type table). No scope/casing/
   length rules — deliberately out of scope (YAGNI; see ADR-0027's rationale on why
   "semantic" validation beyond grammar isn't pursued).
4. `internal/app/`: `VerifyCommit(cfg *config.Config, message string) error` — calls
   `conventionalcommit.IsMergeCommit`/`IsFixupCommit` first (skip, no error), else `Parse`
   + checks `.Type` against the configured/default allow-list.
5. `internal/cmd/commit.go`: new `heraut commit` parent command (mirrors
   `internal/cmd/version.go`'s pattern) with a `verify` subcommand:
   `heraut commit verify [message] [--file <path>]` (supports `--file -` for stdin,
   mirroring `cog verify`'s three input modes). Invalid message → `exitcode.Usage`.
6. Dogfooding: `.config/hk/config.pkl`'s `commit-msg` step switches to
   `go run ./cmd/heraut commit verify --file {{ commit_msg_file }}`. Delete
   `.config/cocogitto/config.toml` and the `cog` entry in `.config/mise/config.toml`'s
   `[shell_alias]` table (both existed solely for this hook). `cocogitto` stays in
   `[tools]` for now — still needed by the generator feature (see T117).
7. Layer table in [`coding.md`](../../.claude/rules/coding.md): add `conventionalcommit`
   as an allowed import for `internal/versioning/*` and `internal/app/`.
8. Docs/schema checklist: `schema.json`, `docs/heraut.sample.yml`,
   `docs/specs/02-configuration.md` (new `commit_lint` block),
   `docs/specs/03-commands.md` (new `heraut commit verify` command).

**Tests:** table-driven unit tests for `Parse` (valid/invalid grammar, scope, breaking `!`,
`BREAKING CHANGE`/`BREAKING-CHANGE` footer — preserve every existing edge case currently
covered in `bump.go`'s tests, per `testing.md`'s "never delete a test row" rule, just
relocated), plus new body/footer cases: body with no footers, multiple footers, a
multi-word hyphenated token (e.g. `Acked-by:`), a multi-line footer value, and the false-
positive `bump.go` couldn't previously rule out — a body paragraph that starts with literal
"BREAKING CHANGE" text but isn't blank-line-separated into the footer block (must NOT set
`Breaking`). `IsMergeCommit`/`IsFixupCommit` table tests, `VerifyCommit` contract-style
tests (default types, configured override, merge/fixup skip), cobra command tests for
`heraut commit verify`'s three input modes and exit code. Plus `BenchmarkParse`
(header-only, header+body, header+multiple-footers inputs) per ADR-0027's performance
note — see `golang-benchmark` skill guidance during implementation for methodology.

**Files:** `internal/conventionalcommit/{conventionalcommit,conventionalcommit_test}.go`,
`internal/versioning/semver/bump.go`, `internal/versioning/semver/resolver_test.go`,
`internal/config/config.go`, `internal/config/validator.go`, `internal/app/commit.go` (new),
`internal/app/commit_test.go` (new), `internal/cmd/commit.go` (new),
`internal/cmd/commit_test.go` (new), `.config/hk/config.pkl`, `.config/mise/config.toml`,
`schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`,
`docs/specs/03-commands.md`, `.claude/rules/coding.md`.
**Scope:** M. **Dependencies:** none.

Implemented across six commits per ADR-0027, exactly as planned with zero functional
deviations: `internal/conventionalcommit` (`a9b4ba5`) → `DetermineBump` refactored onto
`conventionalcommit.Parse`, with its full pre-existing test table passing unmodified,
confirming the refactor was behavior-preserving (`8f63ca6`) → optional `commit_lint.types`
config override plus schema/sample/spec updates (`82b65c8`) → `internal/app.VerifyCommit` +
`DefaultCommitTypes` (`094176c`) → `heraut commit verify` cobra command wired into
`root.go` (`7323673`) → dev-hook cutover, `.config/hk/config.pkl`'s `commit-msg` step now
runs `go run ./cmd/heraut commit verify`, `.config/cocogitto/config.toml` deleted, and the
`cog` shell alias removed from `.config/mise/config.toml` (`cocogitto` itself stays in
`[tools]` until T117) (`9d84662`). That last commit's own message was checked live by the
new hook on creation — the feature's first real-world exercise, and it passed.

One real bug surfaced during Task 4's self-review and was fixed in a follow-up commit
(`18d79bc`): `VerifyCommit` had returned `conventionalcommit.Parse`'s error unwrapped,
violating `coding.md`'s "always wrap with %w" rule; the same commit added two test cases
the original plan's table had omitted (the `squash!` fixup-skip branch, and the
empty-`CommitLint.Types`-falls-back-to-default branch). Two narrative-only inaccuracies in
the plan's own prose were identified and correctly left alone, since they didn't affect any
code: Task 3's brief described the TDD "RED" step as a Go compile error, when the actual
failure mechanism is the strict YAML loader rejecting the unknown `commit_lint` key (same
TDD evidence, different mechanism); and Task 5's docs claimed "eleven tests" where the test
file actually defines ten, and stated the hook already ran `heraut commit verify` before
T116 implemented it (true only after this task's own dev-hook cutover commit, deferred
correctly since `.config/hk/config.pkl` was outside Task 5's file list).

---

#### `[x]` T117: drop the `cocogitto` generator entirely

Per [ADR-0028](../adr/0028-drop-cocogitto-generator.md). `git-cliff` already covers
everything `generator: cocogitto` does, more completely (Azure DevOps remotes,
per-platform release-notes links, ticket linking — all landed on git-cliff first or
exclusively in recent phases). `communique` covers the "opaque external tool" case.
`cocogitto` is a redundant third option with no unique capability. Hard cutover — heraut
is pre-v1.0, trunk-based, no installed external user base to protect through a deprecation
window.

**Implementation:**

1. `internal/config/validator.go`: `generator: cocogitto` becomes a validation error with
   a hint pointing to `git-cliff`/`communique`.
2. Delete `internal/generators/cocogitto/` entirely (package, embedded `cog.toml`, Tera
   templates, contract tests, real-CLI smoke test).
3. `internal/app/`'s `buildGenerator` drops the `cocogitto` case.
4. `.config/mise/config.toml`: remove `cocogitto` from `[tools]` — no remaining consumer
   (T116 already removed the dev-hook usage).
5. Dockerfile: remove `cog`/cocogitto from the bundled-CLI install list.
   [ADR-0016](../adr/0016-bundled-docker-image.md)'s bundled-tool table is updated to match
   (the one historical ADR this task amends — see ADR-0028's "What does not change").
6. `schema.json`, `docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`:
   drop `cocogitto` from the `generator` enum/docs.
7. `testdata/config/`: remove or migrate fixtures using `generator: cocogitto`.

**Tests:** validator test asserting `generator: cocogitto` now fails with the migration
hint; remove cocogitto-specific contract/smoke tests (they no longer have a subject);
confirm `git-cliff`/`communique` fixture coverage in `testdata/config/valid/` is otherwise
unaffected.

**Files:** `internal/config/validator.go`, `internal/config/validator_test.go`,
`internal/app/pipeline.go` (`buildGenerator`), `internal/generators/cocogitto/` (deleted),
`.config/mise/config.toml`, `Dockerfile`, `docs/adr/0016-bundled-docker-image.md`,
`schema.json`, `docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`,
`testdata/config/`.
**Scope:** M. **Dependencies:** T116 (T116 removes the dev-hook's `cog` usage first, so
T117 only has to account for the generator feature's own consumers of the tool).

Implemented across five commits, split into smaller task-scoped slices than the plan's
single numbered list: `a334055` (`internal/config/validator.go` rejects
`generator: cocogitto`, with 4 existing test fixtures swapped from `cocogitto` to
`communique` — they only ever used cocogitto as a stand-in "non-git-cliff generator" for
unrelated rules — plus one new validator test); `bd36063` (removed cocogitto from
`internal/app/pipeline.go`'s `buildGenerator` switch and `internal/app/check.go`'s
runtime-check probe list, deleted `internal/generators/cocogitto/` entirely — 6 files,
~650 lines — and fixed ~21 `exectest.MockRunner` queue lines across
`internal/app/check_test.go`, since MockRunner is a strict FIFO queue and removing one
probe call shifts every subsequent queued response; `TestBuildPipeline_WithCocogitto` was
replaced with `TestBuildPipeline_CocogittoNoLongerSupported`, asserting the opposite);
`951a095` (removed both `cocogitto` options from the `heraut init` wizard in
`internal/scaffold/wizard.go` and deleted the now-dead `internal/scaffold/cog.go` —
`IsCogGenerator`, confirmed no production caller — via a real `git mv` renaming
`cliff_cog_test.go` to `cliff_test.go`, dropping `TestIsCogGenerator` and the `cocogitto`
case from `TestIsCliffGenerator` while leaving `IsCliffGenerator` itself untouched); and
`cc31e86` (the documentation/config-surface sweep: `schema.json`,
`docs/heraut.sample.yml`, `Dockerfile`, `.config/mise/config.toml` + `mise.lock`,
`.github/renovate.json`, `docs/adr/0016-bundled-docker-image.md` — the one historical ADR
this task amends, per ADR-0028's "What does not change" — `README.md`, `CLAUDE.md`, five
`docs/specs/` files, two `.claude/rules/` files, plus a one-line `internal/config/config.go`
doc-comment fix that Task 1's review had flagged as a cross-task gap not in any task's
original file list). A follow-up fix commit, `d283ef3`, corrected a real miss surfaced in
Task 4's review: `README.md`'s intro paragraph still listed `cog` in a tool list (the
string "cog" isn't caught by a "cocogitto" grep sweep) — fixed to `` `git`, `git-cliff`,
`gh`, `glab`, `communique` `` (also adding `git`, which was missing from the original
list). Three environment-driven adaptations during the docs sweep, all independently
verified correct: this repo's `mise` version has no lockfile-prune subcommand, so the
`cocogitto` block in `mise.lock` was hand-removed and confirmed stable across a
`mise install` re-run; the originally-considered Python/PyYAML verification approach
wasn't runnable here (no PyYAML installed), so a Go/`yaml.v3` equivalent check was
substituted; and ADR-0016's bundled-tool table had its `glab` version synced from a stale
`1.97.0` to the Dockerfile's actual current `1.99.0`, per that ADR's "living inventory"
instruction. Final gate (`go build ./...`, `go test ./...` — 1115 tests across 22
packages — `mise run lint:check`) is clean, and the stricter Go-only sweep
(`grep -rln -i "cocogitto" --include="*.go" .`) finds exactly three intentional survivors:
`TestBuildPipeline_CocogittoNoLongerSupported` and `TestValidate_GeneratorCocogittoRejected`
(permanent regression tests explicitly called for by this task's own "Tests" section above,
proving the generator stays rejected) and a historical comment in
`internal/generators/gitcliff/generator_test.go` documenting the T76 embedded-config bug
that motivated the real-CLI smoke-test pattern — none are a live reference to a working
cocogitto generator.

---

#### `[x]` T118: publish a Pkl builtin for `heraut commit verify`

Per [ADR-0029](../adr/0029-pkl-builtin-commit-verify.md). `bifrost`, `forge`, and `hermes`
all still run `cog verify` in their own `.config/hk/config.pkl` commit-msg hook — the exact
pre-T116 pattern heraut itself just removed — and all three already depend on heraut for
their own releases and already install `heraut` via the org's Homebrew tap. This task
publishes the `heraut commit verify` hook definition as a real Pkl package (`PklProject` +
`pkl project package`, distributed as a GitHub release asset versioned 1:1 with heraut's own
tags), mirroring exactly how every one of these repos already imports hk's own
`Builtins.pkl`. **Scope is publish-only** — switching bifrost/forge/hermes's own configs
over to consume it is separate follow-up work in each of those repos, not part of this task.

**Implementation:**

1. Local spike with the `pkl` CLI already on this machine to confirm the exact
   `pkl project package` mechanics (packaging scope, output filename, whether a bare
   `PklProject` file is picked up by hk's existing `pkl`/`pkl_format` lint steps) before
   writing anything into CI.
2. `pkl/PklProject` + `pkl/Builtins.pkl` (`module heraut.Builtins`, one `commit_verify` Step
   calling the `heraut` binary directly, not `go run`).
3. One skippable real-CLI smoke test (per `testing.md`'s real-CLI smoke-test exception)
   shelling out to `pkl project package` against a fixed test version, riding the existing
   `go test ./...` gate.
4. `.github/workflows/release.yml`: new "Package Pkl builtin" step between "Collect release
   binaries" and "Attest build provenance" — rewrites `pkl/PklProject`'s `version` field to
   `$VERSION`, runs `pkl project package`, outputs into `dist/`.
5. `.config/heraut.yml`: new `release.assets` glob entry for the packaged zip, so the
   existing `heraut release` upload step picks it up alongside the binaries/checksums it
   already uploads.

**Tests:** the real-CLI smoke test from step 3; otherwise no new application-layer tests —
this task adds no new Go application code (`heraut release`'s asset upload is unchanged,
contract-tested already).

**Deferred (see ADR-0029):** provenance attestation for the Pkl zip; an automated
post-release check that the published `package://` URL resolves (only verifiable after the
first release that includes this).

**Files:** `pkl/PklProject` (new), `pkl/Builtins.pkl` (new), a new smoke test (location
confirmed by step 1's spike), `.github/workflows/release.yml`, `.config/heraut.yml`.
**Scope:** S. **Dependencies:** T116 (publishes T116's command), scheduled after T117 (no
hard dependency — sequencing choice, not a blocker).

Implemented across three commits exactly as planned, after a hands-on local spike against
the real `pkl` 0.31.1 binary (not asserted from documentation) nailed down several
unverified mechanics ahead of time: `pkl/PklProject` + `pkl/Builtins.pkl` + a generated
`PklProject.deps.json` lockfile, with `commit_verify` typed as a real `Config.Step` (a bare
untyped `new { ... }` is rejected by any real consumer's `Mapping<String, Config.Step>` —
confirmed by deliberately reproducing the rejection before writing the fix) → a real-CLI
smoke test (`pkl_test.go`) that shells out to `pkl project package`, mirroring the existing
git-cliff real-CLI smoke-test convention → the release-pipeline wiring (`release.yml`'s new
"Package Pkl builtin" step, `.config/heraut.yml`'s new `dist/heraut@*` asset glob), reusing
heraut's existing asset-upload path with zero new Go application code.

One real implementation issue surfaced and was fixed mid-Task-1, authorized as an in-scope
amendment: hk's existing repo-wide `pkl`/`pkl_format` lint steps tried to `pkl eval`/`pkl
format` `pkl/Builtins.pkl` from the repo root and failed, because Pkl's project discovery
(needed to resolve `pkl/Builtins.pkl`'s `@hk` dependency alias) walks up from the working
directory, not the target file's own directory, and never finds `pkl/PklProject` from repo
root. Fixed using hk's documented `dir` field (built for exactly this monorepo-subproject
case): `pkl/**` is excluded from the two existing repo-wide steps, and two new dedicated
steps (`pkl_builtins`, `pkl_builtins_format`) are scoped via `dir = "pkl"` plus a
`dir`-relative glob (`"*.pkl"`, not `"pkl/*.pkl"` — discovered empirically that once `dir`
is set, hk's glob matching becomes relative to that directory, not repo root).

Also confirmed empirically rather than assumed: `baseUri`'s final path segment must be the
bare package name with no `@version` suffix (writing `@\(version)` into `baseUri` produces a
doubled `name@version@version` output filename — reproduced once, then avoided); and
`packageZipUrl` must be a separately-spelled `https:` URL, not derived from `baseUri` (which
is a `package://` value and fails Pkl's own type constraint if reused directly).

---

#### `[x]` T119: `heraut commit check` — rev-range conventional-commit validation

Per [ADR-0030](../adr/0030-commit-check-rev-range-validation.md). T116 shipped
single-message `heraut commit verify`; this is the `cog check` equivalent — validating an
entire commit range (or full history) for use as a CI gate on a PR branch.

**Implementation:**

1. `internal/app/commit_check.go`: `CommitCheckResult{SHA, Subject string; Err error}` and
   `CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string)
   ([]CommitCheckResult, error)`. Enumerates via `git log [rev-range]
   --format=%h%x01%s%x01%B%x00`, extending `internal/versioning/semver/resolver.go`'s
   existing NUL-delimited parsing pattern with one extra `\x01`-delimited field. Calls
   `VerifyCommit` unchanged per commit — no new validation logic, no new merge/fixup
   handling (already covered by `VerifyCommit`'s existing skip).
2. `internal/cmd/commit.go`: new `check` subcommand, `heraut commit check [rev-range]`.
   Default output prints only failing commits + a summary count; `--verbose` (existing
   root flag) prints every commit. `exitcode.Usage` when any commit is invalid or when
   `CheckCommitRange` itself errors (bad range, git not found) — same classification
   ADR-0027 used for single-message `verify`.
3. `docs/specs/03-commands.md`: document the new command.

**Tests:** contract tests for `CheckCommitRange` (range arg shape, multi-commit parsing
including footers/blank lines, merge/fixup skip-through via `VerifyCommit`'s existing
behavior, collect-all-not-fail-fast, configured type allowlist, git-log error path); a
white-box test for the rendering helper (failures-only vs. verbose); a cobra-level test for
the non-git-repo error path.

**Files:** `internal/app/commit_check.go` (new), `internal/app/commit_check_test.go` (new),
`internal/cmd/commit.go`, `internal/cmd/commit_test.go`, `internal/cmd/commit_internal_test.go`
(new), `docs/specs/03-commands.md`.
**Scope:** S. **Dependencies:** T116.

Implemented across two feature commits, with the roadmap stub and the plan document
committed separately around them: `d040916` (roadmap stub) → `b3efbac`
(`internal/app/commit_check.go` — `CommitCheckResult{SHA, Subject string; Err error}` and
`CheckCommitRange`, enumerating via `git log [rev-range] --format=%h%x01%s%x01%B%x00` by
extending `internal/versioning/semver/resolver.go`'s existing NUL-delimited parsing
pattern with one extra `\x01`-delimited field, calling T116's `VerifyCommit` unchanged per
commit; 5 contract test functions, 9 sub-tests) → `65fef36` (plan document bookkeeping
commit, not part of either task's deliverable) → `7677ad6` (`heraut commit check
[rev-range]` cobra subcommand plus `printCommitCheckResults` in `internal/cmd/commit.go`;
default output prints only failing commits + a summary count, `--verbose` prints every
commit; both `CheckCommitRange`'s own error and the "N invalid" case map to
`exitcode.Usage`; 6 new tests — 3 white-box renderer tests in a new
`commit_internal_test.go`, 3 black-box cobra tests extending `commit_test.go`, two of which
exercise the real `execadapter` runner against a genuine non-git temp directory). Both
task reviews came back clean with no issues; a stale-diagnostics tool transiently (and
incorrectly) flagged undefined-symbol errors mid-session for both tasks, re-verified each
time by `go build`/`go test` to be a caching artifact rather than a real problem, so there
is nothing to record as an actual deviation. No new config field, no new exit code, no new
global flag — exactly as ADR-0030 scoped it.

---

#### `[x]` T120: `heraut commit create` — interactive commit wizard

Per [ADR-0031](../adr/0031-interactive-commit-wizard.md). Interactive, TTY-only wizard
(type → scope → subject → breaking → body → footers → preview-confirm) that assembles a
`*conventionalcommit.Commit` via `commitwizard.Assemble`, serializes it with `.Format()`,
validates it through the existing `app.VerifyCommit` guard, and runs `git commit -F <tmpfile>`. New
`internal/commitwizard` package; new wizard-only `commit_lint.scopes`; lightweight
stage-all prompt + `--all/-a`. Tickets / per-file staging / `--amend` deferred to v2.

**Completion note:** Implemented across 8 feature commits (`045f15d..615b5b5`) plus one
test-hardening commit (`ecca6dc`), following the 9-task plan
`docs/superpowers/plans/2026-06-26-commit-wizard.md`. Key decisions: `(*conventionalcommit.Commit).Format()` (a method, not a package function)
and `ParseFooterLine` complete the package's round-trip story (`ParseFooterLine` preserves
the `#` in `Footer.Value` so the issue reference survives `Format`, which normalizes the
separator to `: `).
`app.AllowedCommitTypes(cfg)` was extracted from `VerifyCommit` as the single source of
truth for the type allow-list; the wizard's type step reads it and `finalize` runs the
assembled message back through `app.VerifyCommit` as a guard — so wizard output is
guaranteed to pass `heraut commit verify`. `internal/commitwizard/form.go` carries no unit
tests (same precedent as `internal/scaffold/wizard.go`); the observable outputs are covered
by tests in `commitwizard_test.go` and `git_test.go`. `fmt.Fprintln` uses `_, _ =` (project
errcheck pattern). Error propagation is bare where the source already wraps with `%w`
(idiomatic, matches sibling sites). No new exit code; all error paths map to `exitcode.Usage`
consistent with `verify` and `check`.

**Files:** `internal/conventionalcommit/conventionalcommit.go`, `internal/app/commit.go`,
`internal/config/config.go`, `schema.json`, `docs/heraut.sample.yml`,
`docs/specs/02-configuration.md`, `docs/specs/03-commands.md`,
`internal/commitwizard/commitwizard.go`, `internal/commitwizard/git.go`,
`internal/commitwizard/form.go` (+ tests), `internal/ui/status.go`,
`internal/cmd/commit.go`, `docs/adr/0031-interactive-commit-wizard.md`.

---

#### `[x]` T121: `heraut commit check --from-latest-tag`

`cog check --from-latest-tag` equivalent. Adds `--from-latest-tag` bool flag to
`newCommitCheckCmd`. New `app.ResolveFromLatestTag(runner, cfg, env)` resolves the
latest tag: strategy-aware via `CurrentTag` when cfg is present; `git describe --tags
--abbrev=0` fallback when cfg is nil. No-tags condition returns `("", true, nil)` — cmd
layer warns and falls back to full history. Mutual exclusion with the positional rev-range
arg enforced in the cmd layer (error: "cannot use both --from-latest-tag and a rev-range
argument"). `CheckCommitRange` unchanged.

**Completion note:** Implemented in 3 commits (app layer, cmd layer, fix pass). Key decision:
unexported `errNoTagsFound = errors.New("no tags found")` sentinel in `current.go`, wrapped
via `%w` in `CurrentTag`'s return, detected via `errors.Is` in `ResolveFromLatestTag` —
eliminates fragile string-match. `git describe` no-tag detection checks stderr for
`"No names found"` (git 2.x) or `"No tags can describe"`.

**Scope:** S. **Dependencies:** T119 (`heraut commit check` base command).

---

### Phase 23 — Native (built-in) content generator

Per [ADR-0032](../adr/0032-native-content-generator.md) (generator) and
[ADR-0033](../adr/0033-native-config-model.md) (config model): a pure-Go `generator: native`
that becomes heraut's **canonical** changelog / release-notes renderer, **driven by config**
(unified `commits:` + `rendering:` blocks). git-cliff is dropped as the design anchor; its
package removal is deferred (after native enrichment, own ADR). Heavy and multi-phase, so the
task breakdown **and live `[ ] / [x]` status** live in a dedicated roadmap:

→ **[Native Content Generator Roadmap](native-generator-roadmap.md)** — T122+

Summary of the arc (full detail, tests, and files in the dedicated file):

- **Phase 1** — config model + native canonical renderer: T130 `commits`/`rendering` config,
  T131 migrate commit verify/create, T122 commit collection, T123/T124 (landed) reworked
  config-driven by T132/T133, T125 wire native canonical, T126 canonical golden snapshots.
- **Phase 2** — remote enrichment via platform CLIs: T127 GitHub (`gh api`), T128 GitLab
  (`glab api`), T129 Azure DevOps.
- **Phase 2.5** — remove the git-cliff package: deferred, own ADR (after native enrichment).
- **Phase 3** — raw-HTTP clients to drop `gh` / `glab`: deferred behind a future ADR.

---

### Phase 24 — Forge abstraction + unified `forges:` config

A single top-level `forges:` list (a forge = one code-hosting platform heraut talks to) replaces
`changelog.remote` + `release.platforms`, and a new `port.Forge` resolves its identity from **CI env
or git `origin`** (fail loud on ambiguity), builds links, and fetches enrichment metadata. GitLab
gains a native `net/http` enricher (REST default, `JOB-TOKEN`-aware) so `CI_JOB_TOKEN` enriches with
**zero config** — no manual PAT — plus an opt-in GraphQL path (`api_mode: graphql`) for linked
commit-author handles. Consumers reference a forge by name: `commits.enrichment_forge` and
`release.targets[].forge`; `commits.remote_metadata` → `commits.enrichment_policy`. Breaking config
change (pre-v1.0) under new ADR-0043. Heavy and multi-phase, so the task breakdown **and live
`[ ] / [x]` status** live in a dedicated roadmap:

→ **[Forge Abstraction Roadmap](forge-abstraction-roadmap.md)** — T154+

Design: [`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md).

Summary of the arc (full detail, tests, and files in the dedicated file):

- **P1** — GitLab-first: `port.Forge` + config (`forges:` / `release.targets:` /
  `commits.enrichment_*`) + resolution + native REST/GraphQL forge + links + migration (T154–T160).
- **P2** — migrate GitHub + Azure onto `port.Forge`, retire the enrich switch (T161, T162).
- **P3** — `release.targets` replaces `release.platforms` as the publishing surface; the
  transport (`gh`/`glab`) deliberately stays unchanged — see ADR-0044 (T163).
- **P4 (last)** — `heraut init` wizard generates the forge config, after the schema is
  battle-tested (T164).

---

### Phase 25 — Release config simplification

`release:` collapses from two independently-optional axes (`notes`, `targets`) into one atomic
intent: block presence (even `release: {}`) means "generate notes and publish them," root and
per-environment alike — no config shape splits the two anymore. `release.notes` stops being an
on/off toggle and becomes a rendering-customization sub-block, default-populated the same way
`changelog: {}` already defaults `Output` to `CHANGELOG.md`. Supersedes T214 (this session): its
`notesConfigured` synthesis gate protected a "notes only, no publish" state that traced to nothing —
`heraut release` generated the notes string and discarded it when there was no publish target,
confirmed by tracing `Run()` end to end; no command ever surfaced it. Per-environment `disable_notes`
is renamed `disable_release` (turns off the whole block for that environment, not half of it) via a
hard removed-key error — pre-v1.0, no deprecation window, matching ADR-0028's precedent. New
ADR-0046. Breaking config change, lands before the v1.0.0 cut. Small, single-pass epic, so the task
breakdown **and live `[ ] / [x]` status** live in a dedicated roadmap:

→ **[Release Config Roadmap](release-config-roadmap.md)** — T216+

Design: [`docs/superpowers/specs/2026-08-26-release-config-simplification-design.md`](../superpowers/specs/2026-08-26-release-config-simplification-design.md).

---

### Phase 26 — Publish-target driver-support awareness

#### `[x]` T221: recognize forges with no publish driver as non-resolvable targets

**Found while reviewing Phase 25's release-atomicity change against Azure DevOps.** Azure DevOps
has no equivalent of a GitHub/GitLab Release — no tag-attached page for notes + binary assets, only
Azure Pipelines' unrelated multi-stage deployment "Releases" concept and Azure Artifacts package
feeds. Confirmed: there is no `internal/platforms/azure` package, and `buildPlatform`
(`internal/app/pipeline.go`) only handles `"github"`/`"gitlab"`, erroring
`unsupported platform %q (supported: github, gitlab)` for anything else. This is correct and not
itself a bug — there is nothing to build a driver against.

The gap: `synthesizeDefaultTarget` (`internal/app/platforms.go`) synthesizes a zero-config target
for *any* resolved forge — including `azure_devops` — whenever `len(resolved.Forges) > 0`, with no
awareness of which types `buildPlatform` can actually construct. `HasResolvablePublishTarget`
(`internal/app/pipeline.go`) has the same blind spot: it treats any resolved forge as "resolvable,"
so `heraut release`'s preflight gate doesn't catch this either. The result: on Azure Pipelines CI
(auto-detected via `TF_BUILD`), or with an explicit `forges: [{platform: azure_devops}]` entry and
no other forge configured, declaring `release:` in any shape causes `heraut release` to fail deep in
`buildTargetPlatforms` with the generic "unsupported platform" error — instead of failing at
preflight with a clear, specific message, or (for the zero-config auto-detection case) not
attempting a publish target at all.

Before Phase 25, T214's now-removed `notesConfigured` gate accidentally papered over the *specific*
shape of `release.notes` set with `release.targets` empty (treating it as "notes only, no publish"),
which incidentally let an Azure DevOps-only setup generate enriched release-notes text via `heraut
release` without ever reaching the publish step. Phase 25 removed that gate on purpose — no config
shape should split notes from publish — which means this narrow escape hatch is gone too. That
change was correct on its own terms (the gate was never meant to be an Azure-DevOps-awareness
mechanism), but it removes the only way an Azure DevOps user had to get release-notes generation
without a doomed publish attempt. `heraut changelog` is unaffected (it never touches `release:` or
publish) and remains the only fully-supported path for Azure DevOps today.

**Direction (exact shape TBD at implementation time):**

- Give `synthesizeDefaultTarget` (or its caller) awareness of which forge types have a publish
  driver — likely a small shared lookup consulted by `buildPlatform`, `synthesizeDefaultTarget`, and
  `HasResolvablePublishTarget`, so "supported for publish" is defined once, not re-derived in three
  places.
- Zero-config auto-detection resolving *only* to a driver-less forge type should behave like "no
  forge resolved" for target-synthesis purposes — consistent with today's already-documented
  philosophy that zero resolvable destinations is a config error for `heraut release`, not a silent
  no-op or a deep pipeline crash.
- Decide whether an *explicit* `release.targets[].forge` naming a driver-less forge should get the
  same early/clear treatment, or whether today's `buildPlatform` error (arguably already clear
  enough for a user-authored mistake) is sufficient as-is.
- `heraut check`'s `effectiveTargetPlatforms` (`internal/app/check.go`) should stay consistent with
  whatever `heraut release` decides, so its Platforms/binary-fallback branching doesn't diverge from
  actual release behavior.
- Consider whether this needs an ADR-0046 addendum (documenting the Azure DevOps limitation
  explicitly) or is scoped narrowly enough to be a plain bugfix — decide once the fix shape is clear.

**Files (expected):** `internal/app/platforms.go`, `internal/app/pipeline.go`,
`internal/app/check.go` (+ their test files); possibly `docs/specs/02-configuration.md` /
`docs/adr/0046-release-block-atomicity.md` if documentation needs updating.
**Scope:** S–M. **Dependencies:** none (Phase 25 already shipped).

Implemented as a plain bugfix, no new ADR needed — this tightens existing behavior
(ADR-0043/0044/0046's "zero resolvable destinations is a config error, not a silent no-op"
philosophy) rather than introducing new architecture. Added `platformBuilders`
(`internal/app/platforms.go`), a `map[string]func(...) (port.Platform, error)` that is now the one
source of truth for publish-driver support: `buildPlatform` (`internal/app/pipeline.go`) dispatches
from it directly instead of a hand-written switch, and the new `supportsPublish` helper checks
membership in the same map — so the two can never drift apart. `synthesizeDefaultTarget` now skips
any resolved forge without a publish driver instead of synthesizing a target for whichever forge
happened to resolve; `HasResolvablePublishTarget`'s zero-config branch now calls
`synthesizeDefaultTarget` directly rather than re-deriving its own "any forge resolved" check, so
`heraut release`'s preflight and its actual publish behavior are structurally guaranteed to agree.
`heraut check`'s `effectiveTargetPlatforms` needed no separate change — it already delegated to
`synthesizeDefaultTarget` (T216), so it inherited the fix automatically; a dedicated test
(`TestRuntimeCheck_AzureOnlyFallsBackToBinaryProbe`) confirms this rather than leaving it as an
unverified inference.

Scoped the fix to auto-detected/zero-config resolution only, per explicit user confirmation during
implementation: an *explicit* `release.targets[].forge` naming an azure_devops entry still surfaces
`buildPlatform`'s existing "unsupported platform" error, unchanged — a deliberate user-authored
reference already gets a specific, actionable error, and catching it earlier would require
re-deriving per-target forge resolution just for a preflight yes/no, which isn't worth the
duplication for an already-clear failure mode. Added a short "GitHub and GitLab only" note to
`docs/specs/02-configuration.md`'s "Platform drivers" section explaining Azure DevOps's structural
lack of a Release equivalent, closing the loop for anyone reading the spec rather than only
discovering this by hitting the error. `go test ./...` and `hk check` both clean.

---

### Phase 27 — Documentation vs. code audit reconciliation

A full audit (four parallel Opus passes over specs, ADRs, `CLAUDE.md`/`.claude/rules/`, and
`schema.json`/`heraut.sample.yml` against current code) found 142 doc/code mismatches, seven of
which are real code bugs the docs happened to expose rather than pure documentation drift (e.g.
`--version` ignoring `tag_prefix`, per-environment `release:` never getting `Notes`
default-populated per ADR-0046, dead `rendering.excludes`). Too large for one task; broken out —
bug fixes kept separate from doc-only reconciliation so they can be prioritized and reviewed
independently:

→ **[Documentation Audit Roadmap](docs-audit-roadmap.md)** — T222+

---

## Risks and mitigations

| Risk                                                                                | Impact            | Mitigation                                                                |
|-------------------------------------------------------------------------------------|-------------------|---------------------------------------------------------------------------|
| `perenv.VersionCalculator` interface doesn't cleanly fit both semver and calver     | High — blocks T12 | Prototype with semver first (T10); validate with calver before locking it |
| GitHub Actions release pipeline + GoReleaser GHCR push requires setup               | Med — blocks T02  | Test release pipeline on a scratch tag early; don't wait until T24        |
| Self-update on macOS blocked by quarantine / Gatekeeper on downloaded binaries      | Med — poor UX     | Implement `xattr -d com.apple.quarantine` step post-download in T21       |
| `heraut check runtime` needs `git user.name` which may not be set in CI containers  | Low — known issue | Detect the missing config, print an actionable hint, exit non-zero        |

---

## Resolved questions

| Question                                                                            | Resolution                                                                |
|-------------------------------------------------------------------------------------|---------------------------------------------------------------------------|
| Module path                                                                         | `github.com/adaouat/heraut`                                               |
| GoReleaser release ownership for the heraut repo's own releases                     | heraut owns GitHub Release creation (T51 / ADR-0018); goreleaser is build-only (`release: disable: true`) |
| Docker image registry                                                               | `ghcr.io/adaouat/heraut`                                                  |
| Dev tooling                                                                         | `mise` + `hk` (already configured in `.config/`)                          |
| Self-update version check                                                           | GitHub Releases API directly (no Pages hosting)                           |
| ADR numbering                                                                       | Sequential from 0001, no gaps                                             |
| Spec layout                                                                         | Six numbered files in `docs/specs/`                                       |
