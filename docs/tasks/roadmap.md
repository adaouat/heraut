# Héraut Build Roadmap

> Status: Active  
> Repo: `github.com/adaouat/heraut`

This roadmap is the executable plan for bringing Héraut to v1.0 with the feature set
described in `docs/specs/`. Each task carries an inline `[ ] / [x]` checkbox — read the
headings for what to do next, read the surrounding prose for *why* and *how*.

The behavioural authority is `docs/specs/` (six numbered specs); the architectural
authority is `docs/adr/` (19 ADRs). Where this roadmap mentions "behaviour", the specs
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

The `docs/specs/` (six numbered specs) and the 19 ADRs in `docs/adr/` are authoritative.

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

### ✦ `[x]` CHECKPOINT I — v1.0.0 shipped via heraut

- [x] T28 resolved — lightweight confirmed or annotated implemented
- [x] CI split: `lint` / `test` / `build` run as independent required checks
- [x] Coverage ≥ 80% enforced in CI; actual coverage ≥ 85%
- [ ] v1.0.0 cut by running `heraut release` on the heraut repo itself

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

#### `[ ]` T57: `heraut release --build` for build-id release flows

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

#### `[ ]` T68: Resolve the context-injection shape (env vars vs. new template variables)

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

#### `[ ]` T72: Integration test — multi-platform release produces distinctly-flavored notes

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

#### `[ ]` T73: Spec update — document communique's link-resolution exclusion

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

#### `[ ]` T74: Fix git-cliff PR link path — `/pulls/` → `/pull/` (pre-existing bug)

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

#### `[ ]` T75: Fat-injection / thin templates — heraut computes URL pieces in Go

**Motivation (user idea, 2026-06-09):** push the `HERAUT_*` approach further — have heraut
compute the *fully-formed* URL pieces in Go (`HERAUT_PR_URL`, `HERAUT_COMPARE_URL`,
`HERAUT_COMMIT_URL`, `HERAUT_PR_LABEL`) so the embedded templates become pure interpolation
with **no `if/else` and no `get_env` fallback chains**. This moves the platform-specific
path knowledge (GitHub `#`+`/pull/` vs GitLab `!`+`/-/merge_requests/`) out of Tera and into
typed, unit-testable Go — heraut already owns `LinkContext.Platform`. Benefits: flatter
templates, testable URL construction, trivial to add a new platform or self-hosted path
quirk, and users who copy the defaults don't re-derive path shapes.

**Why this is gated on host-targeting (not doable cleanly now):** today the *only* branch is
`pr_link`, and it exists to serve **two** modes — heraut-injected (multi-platform) and
ambient-CI fallback (single platform). For a single self-hosted platform heraut cannot
pre-compute a full URL because the [ADR-0020](../adr/0020-platform-base-url.md) gate forbids
a non-default `base_url`, so the real host only arrives via ambient `CI_PROJECT_URL` — the
self-hosted-CI non-regression. So while the gate stands, the ambient fallback branch **must
stay**; injecting more URL vars now yields a *hybrid* (injected path + fallback branch) that
is only marginally flatter and adds env-var surface. Once the host-targeting / multi-instance
thread makes `base_url` authoritative for **every** case, heraut can compute all URL pieces
for single- and multi-platform alike, the ambient `CI_PROJECT_URL` detection can be
**retired**, and the templates collapse to branch-free interpolation — the clean end state.

**Scope of change (when unblocked):**
- Compute the URL pieces in Go (new fields on / derived from `port.LinkContext`; per-platform
  path + label), inject as `HERAUT_*` env vars for **all** runs (git-cliff).
- Strip the `pr_link` branch and the `get_env` fallback chains from both embedded git-cliff
  TOMLs → pure interpolation. cocogitto is already branch-free (uses cog's `repository_url`)
  — minimal/no change.
- Retire the ambient `CI_PROJECT_URL` / `GITHUB_SERVER_URL` detection from the defaults once
  `base_url` is authoritative.

**ADR required:** **yes** — changes the embedded template-variable contract (ADR-0010
user-facing surface) and retires ambient CI detection; supersedes the T71 macro shape.

**Dependencies:** the host-targeting / multi-instance thread (currently an unscoped thread
in the [design note](../../.claude/plans/multi-platform-release-notes-link-resolution.md)
"Related (but distinct) gap" — must be numbered and landed first). **Do not start before
that thread makes `base_url` authoritative for single-platform self-hosted.**

**Scope:** M (deferred)

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
