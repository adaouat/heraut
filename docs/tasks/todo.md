# Héraut — Task Checklist

> See `docs/tasks/roadmap.md` for full acceptance criteria, dependency graph, architecture
> decisions, and testing strategy.

**TDD is required on all tasks**: write failing tests before writing implementation.  
For CLI interactions (`glab`, `gh`, `git-cliff`, `cog`, `communique`): use `MockRunner`
for contract tests and `testutil.FakeBin` for integration tests.

**Three-step flow** for every task: flip `[ ]` → `[~]` and commit, implement, flip
`[~]` → `[x]` with a roadmap note and commit.

---

## Phase D — Documentation Foundation

- [x] **D01** `CLAUDE.md` + `.claude/rules/{claude,coding,testing,workflow}.md`
- [x] **D02** `docs/specs/` — six numbered specs (01-overview … 06-dx-and-testing)
- [x] **D03** `docs/adr/` — 14 ADRs (0001–0014)
- [x] **D04** `docs/tasks/roadmap.md` + `docs/tasks/todo.md`

## ✦ CHECKPOINT D — Docs foundation in place
- [x] All Phase D files exist; no code yet (`go.mod`, `cmd/`, `internal/` absent)

---

## Phase 0 — Repo Bootstrap

- [ ] **T00** Repository skeleton: `go.mod`, cobra + fang root command, `heraut --help`
      works, ldflags version
- [ ] **T01** GitHub Actions CI: PR pipeline (build + test + lint), `.golangci.yml`
- [ ] **T02** GoReleaser + release pipeline: cross-platform raw binaries, GitHub
      Releases, GHCR Docker image

## ✦ CHECKPOINT A — Build and CI foundation
- [ ] `go build ./...` clean, `go test ./...` passes, PR CI runs, goreleaser snapshot
      succeeds, Docker builds

---

## Phase 1 — Core Contracts and Config

- [ ] **T03** Port interfaces (`Runner`, `Generator`, `Platform`) + `adapter/exec.Runner`
      + `testutil.MockRunner` + `testutil.FakeBin`
- [ ] **T04** Config structs + `Load()` + `LoadFromReader()` + `ResolvePath()` + strict
      YAML + test fixtures
- [ ] **T05** Config semantic validation: composed validators (required → enum →
      strategy-specific → cycle detection)

## ✦ CHECKPOINT B — Config loads and validates
- [ ] All valid fixtures pass; all invalid fixtures fail with expected messages

---

## Phase 2 — First Complete Vertical: SemVer + gitcliff + GitHub

- [ ] **T06** `versioning/tagfmt`: `Render()`, `ParseVersion()`, `GlobPattern()`
- [ ] **T07** SemVer resolver: `Resolve()`, `DetermineBump()`, `BumpVersion()`, prefix
      handling, initial version
- [ ] **T08** gitcliff generator: embedded TOML defaults, config merge, temp file
      lifecycle, `Validate()`
- [ ] **T09** GitHub platform + contract tests: `CreateRelease`, `UploadAssets`,
      repository / token resolution
- [ ] **T10** App resolver factory + release pipeline + `heraut release`:
      `app.NewResolver()`, `app.BuildPipeline()`, `pipeline.Pipeline`, thin
      `cmd/heraut/release.go`

## ✦ CHECKPOINT C — First working release
- [ ] `heraut release --dry-run` on test semver repo prints correct sequence; no factory
      logic in `cmd/`

---

## Phase 3 — Strategy Expansion

- [ ] **T11** CalVer resolver: format parsing, date tokens, PATCH increment / reset,
      `SPRINT` support
- [ ] **T12** Generic per-env resolver (`versioning/perenv/`): wraps `VersionCalculator`,
      auto + promote, E001 / E002 / E003, `source:` field, cycle detection
- [ ] **T13** `heraut version next` + `heraut version current` + `heraut version sprint bump`

## ✦ CHECKPOINT D' — All 4 strategies pass `heraut version next`
- [ ] `semver`, `calver`, `semver-per-env`, `calver-per-env` all work; no
      `semver_per_env` or `calver_per_env` packages exist

---

## Phase 4 — Remaining Generators and GitLab Platform

- [ ] **T14** communique generator + contract tests
- [ ] **T15** cocogitto generator + contract tests: all 4 config-path combinations,
      embedded defaults
- [ ] **T16** GitLab platform + contract tests: `CreateRelease`, `UploadAssets`, project
      / token resolution, catalog flag

## ✦ CHECKPOINT E — All generators and platforms implemented and tested
- [ ] Contract tests verify exact CLI arguments for all generators and both platforms

---

## Phase 5 — Complete Pipeline Surface

- [ ] **T17** Changelog pipeline + `heraut changelog`: `ChangelogPipeline`,
      `app.BuildChangelogPipeline()`, thin `cmd/heraut/changelog.go`, per-env
      `disable_changelog`
- [ ] **T18** `heraut check` subcommands: `config`, `runtime`, `cliff` (with
      `changelog` / `release-notes` sub-subcommands); automatic preflight before release
      / changelog
- [ ] **T19** `heraut cliff changelog` / `heraut cliff release-notes` + per-env
      `disable_changelog` / `disable_notes` in pipeline

## ✦ CHECKPOINT F — Full pipeline surface implemented
- [ ] `go test ./...` passes; all 4 strategies work for `release`, `changelog`,
      `version next`; no domain logic in `cmd/`

---

## Phase 6 — Supporting Features

- [ ] **T20** `heraut init` wizard: `RunWizard()`, `GenerateYAML()`, `ConfigToAnswers()`,
      `Defaults()`, multi-env loop, existing-config prompt
- [ ] **T21** `heraut self-update` (GitHub Releases API): background hint, `--check`,
      download + SHA-256 verify + atomic replace

## ✦ CHECKPOINT G — DX features complete
- [ ] `heraut init --defaults` → `heraut check config` passes; `heraut self-update --check`
      works

---

## Phase 7 — Doc Reconciliation + Public README

- [ ] **T22** Spec reconciliation: walk the 6 `docs/specs/` against the implementation;
      fix drift in the spec
- [ ] **T23** ADR reconciliation: validate 0007 / 0008 / 0009 / 0010 / 0014 against the
      code; amend any `Consequences` sections out of sync
- [ ] **T24** `README.md`: install, quickstart, configuration reference,
      `heraut self-update`

## ✦ CHECKPOINT H — Ready for public launch
- [ ] `go test ./...` passes; goreleaser snapshot succeeds; Docker boots; README covers
      install + quickstart
