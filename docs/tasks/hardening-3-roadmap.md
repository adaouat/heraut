# Héraut — Hardening 3 Roadmap

> Status: Not started (drafted 2026-10-20, awaiting approval)
> Source: the three follow-ups left open by the modernization brief
> (`analysis/heraut/MODERNIZATION_BRIEF.md`) after [`hardening-roadmap.md`](hardening-roadmap.md) and
> [`hardening-2-roadmap.md`](hardening-2-roadmap.md). Main roadmap: tracked as Phase 76 in
> [`roadmap.md`](roadmap.md).

Nothing here changes what a user sees. These are structure items the earlier epics deliberately left:
one shell-out that bypasses the runner abstraction, environment-variable names spelled out in several
packages, and the one wizard function that is still long. Each task is a refactor under TDD: a
characterization test first where the current behavior is not already pinned, then the change.

## Conventions

- Task IDs **continue the global sequence** (`T410+`).
- This file is the **single source of truth** for task status. Same markers as the other roadmaps: `[ ]`
  not started, `[x]` done. Follow the two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)):
  failing or characterization test first, then flip `[ ]` → `[x]` and add a one-paragraph completion note.
- One task per session. Order is a recommendation (smallest first), not a dependency.
- No behavior change and no test row removed. A task that finds it cannot keep behavior identical stops and
  says so before changing it.
- A decision a task needs from the maintainer (marked **Ask**) is asked at the start of that task, not
  guessed here.
- Wire names in `internal/config` do not change.

## Baseline (before Phase 76)

Recorded at `b4b8cb0`, Go 1.27.2: `go test -count=1 ./...` passes in every package (including e2e Lane A);
total statement coverage **88.2%** (the CI gate is 85%); `golangci-lint` reports 0 issues. Phase exit
criteria include "the same command still passes and coverage does not drop below the gate".

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 76 | Brief follow-ups | T410–T412 | In progress (T410 done) |

---

### Phase 76 — Brief follow-ups

Goal: close the three structure items the brief listed. Entry: baseline above recorded.

#### `[x]` T410: read the git origin in `scaffold` through the runner

`internal/scaffold/wizard.go` has its own `gitRemoteOriginURL()`, which calls `exec.Command("git", "remote",
"get-url", "origin")` directly. Every other caller goes through `port.Runner`, and T396 added
`forge.GitOriginURL(runner)` for exactly this lookup (`scaffold` may import `forge`). The direct call also
runs in whatever directory the test happens to be in, so the wizard's pre-fill is untestable without a real
repository. Tests first: the origin-driven pre-fill (`detectPlatform` with an SSH and an HTTPS GitHub
origin, a GitLab origin, no origin) is exercised through an injected lookup rather than the process
working directory. Then delete `gitRemoteOriginURL` and have the wizard take the lookup (a
`func() string`, defaulting to `forge.GitOriginURL` on the real runner). Behavior is identical: an error or
no origin gives `""`.

**Completion note:** the lookup now goes through `forge.GitOriginURL` on the real runner
(`execadapter.New(false, false)`, a library outside the layer rules); `gitRemoteOriginURL` stays as a
one-line wrapper so `runPlatformWizard` and the signature of `RunWizard` are untouched, and `os/exec` is no
longer imported by `scaffold`. The draft's premise was partly wrong: the pre-fill logic (`detectPlatform`)
already takes the origin as a parameter and has tests for SSH, HTTPS, self-hosted and no-origin
(`wizard_internal_test.go`), so only the one-line lookup itself was untestable. Chosen over injecting a
lookup into `RunWizard` (an exported signature change that nothing could exercise, since the `huh` forms need
a terminal). Test: `TestGitRemoteOriginURL` (new `origin_internal_test.go`) runs a fake `git` that answers
only the exact `remote get-url origin` call, for the URL (trimmed) and the failure case; it was written
against the old code first and passed before and after, as a characterization test should. The exact-argument
contract of the command is also asserted by `forge`'s own `origin_test.go`. No behavior change.

#### `[ ]` T411: one owner for the CI and token environment variable names

The names of the environment variables heraut reads from a CI job or for a token are spelled out in several
packages: `internal/forge/detect.go` (`GITHUB_ACTIONS`, `GITLAB_CI`, `TF_BUILD`, `CI_SERVER_URL`,
`CI_PROJECT_PATH`, `GITHUB_REPOSITORY`, the three default token names, …), `internal/pipeline/linkctx.go`
(`CI_SERVER_URL`, `CI_PROJECT_PATH`, `CI_PROJECT_URL`, `GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`),
`internal/platforms/{github,gitlab}/platform.go` (`repoEnvVar`, `projectEnvVar`, `defaultTokenEnv`,
`CI_JOB_TOKEN`, `CI_SERVER_URL`, …), `internal/scaffold/decisions.go` (the token options) and the list in
`internal/testutil/env.go` that clears them. A typo or a platform's rename has to be fixed in each. The layer
rules forbid `platforms/*` and `forge/*` importing each other, so the shared definitions must live in a
leaf both may import. **Ask**: where. `internal/port` is the working proposal (it already holds
`ForgeIdentity` and `TokenKind`, and `testutil` imports it); a new leaf package would need a row in the
layer table. Not a pure rename: the default token names differ on purpose (`GITHUB_TOKEN` is the CI
variable `forge` reads, `GH_TOKEN` is what the `gh` CLI wants), and the task keeps both, named for their
role, instead of merging them. Tests first: a table pinning every shared name, and `ClearCIEnv` derived
from the same list so a new name cannot be added without being cleared in tests. Then replace each literal
with the shared constant, one package per commit.

#### `[ ]` T412: split `runPlatformWizard` into steps

`runPlatformWizard` in `internal/scaffold/wizard.go` is still about 160 lines of inline `huh` forms: the
"add another platform" loop, the type select, the repository or project input, the token-env select, and the
draft and assets questions. T397 already moved the decisions out (`finishPlatformToken`,
`enrichmentForgeChoice`, `enrichmentPolicyOrDefault`); this is the forms. Plan: one small function per
question that takes a `*PlatformAnswer` and returns an error (`askPlatformType`, `askPlatformLocation`,
`askPlatformToken`, `askPlatformPublishOptions`), a loop body that calls them in order, and every pure
choice (which options to offer, what to pre-fill, when to ask the token question) kept in `decisions.go`
where the existing tests pin it. Forms cannot run without a terminal, so the characterization test is the
set of decisions each step makes (extend `wizard_internal_test.go`), not the forms themselves. Success is a
loop of at most about 50 lines and no function over about 60, with the generated YAML unchanged for every
wizard path the existing tests cover. If a step cannot be separated without reworking the `huh` layout, stop
and say so rather than reshaping the wizard.

**Phase 76 exit:** the baseline command passes, coverage ≥ 88.2%, lint clean.

---

## Considered, not scheduled

- `internal/versioning/semver/rotation.go` parses version parts with its own lenient `Atoi` (seen while
  fixing `BumpVersion` in T402). Its input comes from tags already parsed strictly, so no behavior is at
  risk; revisit only if the rotation code is touched.
- Per-pattern `optional` markers for release assets, and an opt-in padded `PATCH` for CalVer, were both
  discussed in the hardening-2 epic and deliberately left out (ADR-0076 alternatives; RULE-009 unchanged).
