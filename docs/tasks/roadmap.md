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
- `HERAUT_NO_UPDATE_CHECK=1` disables the background hint
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

#### `[ ]` T28: Annotated git tags (revisit)

**Description:** Both pipelines currently create lightweight tags (`git tag <tag>` in
`internal/pipeline/release.go` and `changelog.go`). Annotated tags (`git tag -a -m …`)
carry a tagger, date, and message and are what most release tooling expects. Decide
whether to switch, and if so what the annotation message should be (likely the resolved
version or the release notes). Discuss implementation before starting. Surfaced by T22
spec reconciliation; Spec 03 currently documents lightweight tags to match the code.

**Files:** `internal/pipeline/release.go`, `internal/pipeline/changelog.go`,
`docs/specs/03-commands.md`

**Scope:** S

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

#### `[ ]` T31: Batteries-included Docker image + multi-tag release

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

---

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
