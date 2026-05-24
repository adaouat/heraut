# Héraut Build Roadmap

> Status: Active  
> Repo: `github.com/adaouat/heraut`

This roadmap is the executable plan for bringing Héraut to v1.0 with the feature set
described in `docs/specs/`. Each task carries an inline `[ ] / [x]` checkbox — read the
headings for what to do next, read the surrounding prose for *why* and *how*.

The behavioural authority is `docs/specs/` (six numbered specs); the architectural
authority is `docs/adr/` (14 ADRs). Where this roadmap mentions "behaviour", the specs
win; where it mentions a "decision", the ADR wins. If you find a disagreement between
roadmap and spec/ADR, fix the roadmap.

---

## Overview

Héraut is a Go CLI that orchestrates `git-cliff`, `glab`, `gh`, `cog`, and `communique`
to resolve versions, generate changelogs, and publish releases to GitHub / GitLab. This
roadmap captures the work to take it from an empty repo to a v1.0 release.

The goals of v1.0:

1. Implement the full feature set described in `docs/specs/` (four versioning
   strategies, three generators, two platforms, init/check/cliff/self-update tooling).
2. Establish a clean public home with proper distribution: GitHub Releases (raw
   binaries) and a GHCR container image.
3. Design internal packages with clear boundaries so the foundational ones
   (`port`, `adapter/exec`, `testutil`, `ui`) can be extracted into a shared Go library
   later when other CLIs need them.

The `docs/specs/` (six numbered specs) and the 14 ADRs in `docs/adr/` are authoritative.

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

#### `[ ]` T00: Repository skeleton — module, cobra CLI, basic build

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

---

#### `[ ]` T01: GitHub Actions CI — PR pipeline

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

---

#### `[ ]` T02: GoReleaser + release pipeline

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

### ✦ `[ ]` CHECKPOINT A — Build and CI foundation

- [ ] `go build ./...` clean, `go test ./...` passes
- [ ] PR CI pipeline runs on every push
- [ ] GoReleaser snapshot build succeeds
- [ ] Docker image builds and boots

---

### Phase 1 — Core Contracts and Config

#### `[ ]` T03: Port interfaces + adapter + testutil

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

---

#### `[ ]` T04: Config structs + loader + path resolution

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

---

#### `[ ]` T05: Config semantic validation — composed validators

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

### ✦ `[ ]` CHECKPOINT B — Config loads and validates

- [ ] `go test ./internal/...` passes
- [ ] All `testdata/config/valid/` fixtures pass; all `testdata/config/invalid/`
      fixtures fail with the expected error messages

---

### Phase 2 — First complete vertical: SemVer + gitcliff + GitHub

*Goal: `heraut release` works end-to-end for a semver project publishing to GitHub.*

#### `[ ]` T06: Tag format package (`versioning/tagfmt`)

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

---

#### `[ ]` T07: SemVer resolver

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

---

#### `[ ]` T08: gitcliff generator

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

---

#### `[ ]` T09: GitHub platform + contract tests

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

---

#### `[ ]` T10: App resolver factory + release pipeline + `heraut release`

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

### ✦ `[ ]` CHECKPOINT C — First working release

- [ ] `heraut release --dry-run` on test semver repo prints correct action sequence
- [ ] `heraut --help` and `heraut release --help` show complete usage
- [ ] No strategy-selection logic in `internal/cmd/`

---

### Phase 3 — Strategy Expansion

#### `[ ]` T11: CalVer resolver

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

---

#### `[ ]` T12: Generic per-environment resolver

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

---

#### `[ ]` T13: `heraut version` subcommands

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

### ✦ `[ ]` CHECKPOINT — All 4 strategies pass `heraut version next`

- [ ] `semver`, `calver`, `semver-per-env`, `calver-per-env` all work
- [ ] Only `internal/versioning/perenv/` exists — no `semver_per_env` or `calver_per_env`

---

### Phase 4 — Remaining Generators and GitLab Platform

#### `[ ]` T14: communique generator + contract tests

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

---

#### `[ ]` T15: cocogitto generator + contract tests

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

---

#### `[ ]` T16: GitLab platform + contract tests

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

### ✦ `[ ]` CHECKPOINT E — All generators and platforms implemented and tested

- [ ] Contract tests verify exact CLI arguments for all generators and both platforms

---

### Phase 5 — Complete Pipeline Surface

#### `[ ]` T17: Changelog pipeline + `heraut changelog`

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

---

#### `[ ]` T18: `heraut check` subcommands

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

---

#### `[ ]` T19: `heraut cliff` + per-env disable flags

**Acceptance:**
- `heraut cliff changelog` — prints the effective merged git-cliff TOML for changelog mode
- `heraut cliff release-notes` — same for release notes mode
- `EnvVersioning.DisableChangelog` and `EnvVersioning.DisableNotes` — when true for the
  active env, the respective pipeline step is skipped silently
- `app.BuildPipeline()` applies per-env disable flags when building pipeline config

**Dependencies:** T08, T10, T12

**Files:** `internal/cmd/cliff.go`, `internal/app/pipeline.go` (per-env disable flags)

**Scope:** S

### ✦ `[ ]` CHECKPOINT F — Full pipeline surface implemented

- [ ] `go test ./...` passes
- [ ] `heraut release --dry-run` works for all 4 strategies
- [ ] `heraut changelog --dry-run` works for all 4 strategies
- [ ] `heraut version next` works for all 4 strategies
- [ ] `heraut check config` catches all validation errors
- [ ] No domain/factory logic in `internal/cmd/`; all wiring in `internal/app/`

---

### Phase 6 — Supporting Features

#### `[ ]` T20: `heraut init` wizard

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

---

#### `[ ]` T21: `heraut self-update` — GitHub Releases API

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
- `HERAUT_NO_UPDATE_CHECK=1` disables the background hint
- Skipped for `dev` builds (no ldflags) and during `heraut self-update` itself
- macOS: removes `com.apple.quarantine` xattr post-download

**Dependencies:** T00 (ldflags), T03

**Files:** `internal/selfupdate/{updater,manifest,github,selfupdate_test}.go`,
`internal/cmd/self_update.go`

**Scope:** M

### ✦ `[ ]` CHECKPOINT G — DX features complete

- [ ] `heraut init --defaults` produces a valid config that passes `heraut check config`
- [ ] `heraut self-update --check` exits without error against mock GitHub API

---

### Phase 7 — Doc Reconciliation + Public README

Most docs are written upfront in Phase D. This phase handles what *can't* be written until
the implementation is complete.

#### `[ ]` T22: Spec reconciliation

Walk the 6 `docs/specs/` files against the implementation. Any drift (flag rename, behavior
tweak, unimplemented field) gets fixed in the spec, not just the code. The spec is the
source of truth for users.

Files: `docs/specs/*.md`

Scope: S

---

#### `[ ]` T23: ADR reconciliation

Re-read the 14 ADRs from `docs/adr/`; validate that 0007 (E001/E002/E003), 0008
(`source:` field), 0009 (generic perenv), 0010 (embedded cliff), and 0014 (self-update)
still match the code. Amend any ADR whose *Consequences* section no longer reflects what
was built.

Files: `docs/adr/*.md`

Scope: S

---

#### `[ ]` T24: `README.md`

Public-facing, written last. Install (`go install`, GitHub binary,
`ghcr.io/adaouat/heraut`), quickstart (`heraut init` → `heraut check config` →
`heraut release`), short configuration reference linking to
`docs/specs/02-configuration.md` for the full one, `heraut self-update`, links to
`docs/specs/` and `docs/adr/`.

Files: `README.md`

Scope: S

### ✦ `[ ]` CHECKPOINT H — Ready for public launch

- [ ] `go test ./...` passes
- [ ] `goreleaser build --snapshot --clean` succeeds
- [ ] Docker image boots, `heraut --version` prints version
- [ ] `heraut init --defaults` → `heraut check config` → passes
- [ ] `heraut release --dry-run` prints correct sequence for all 4 strategies
- [ ] README covers install + quickstart
- [ ] `docs/specs/` and `docs/adr/` reconciled with the implementation

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
| GoReleaser release ownership for the heraut repo's own releases                     | GoReleaser creates GitHub Releases directly (`release: disable: false`)   |
| Docker image registry                                                               | `ghcr.io/adaouat/heraut`                                                  |
| Dev tooling                                                                         | `mise` + `hk` (already configured in `.config/`)                          |
| Self-update version check                                                           | GitHub Releases API directly (no Pages hosting)                           |
| ADR numbering                                                                       | Sequential from 0001, no gaps                                             |
| Spec layout                                                                         | Six numbered files in `docs/specs/`                                       |
