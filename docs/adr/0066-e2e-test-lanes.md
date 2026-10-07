# ADR-0066: End-to-end test lanes — hermetic binary lane and opt-in forge lane

- **Status**: Accepted
- **Date**: 2026-10-07
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md`](../superpowers/specs/2026-10-07-e2e-smoke-suite-design.md)

---

## Context

The four test layers (unit, contract, integration, schema) cannot see two classes of defect: a
forge rejecting what heraut sends (T332's manual smoke run found T335 that way), and wiring bugs
between layers that each pass their own tests (ldflags, flag plumbing, exit-code mapping, real git
history). heraut is now used on many projects, so those escapes cost more.

`.claude/rules/testing.md` forbids network calls and requires determinism, so a forge e2e suite needs
an explicit, narrow exemption rather than an ad-hoc one.

## Decision

Add a fifth test layer, **E2E**, in a top-level `e2e/` tree that drives the **built `heraut` binary**,
split into two lanes:

- **Lane A, hermetic.** Real binary, real `git`, local repositories under `t.TempDir()`, no network.
  It lives inside the existing rules and runs in plain `go test ./...`, so it gates every PR. It covers
  version resolution and local release flows.
- **Lane B, forge sandbox.** Real binary against private GitHub/GitLab sandbox repositories. It is
  behind the `e2e_forge` build tag, runs only from its own workflow (`workflow_dispatch` plus
  nightly, advisory, never on pull requests), and takes coordinates and tokens from CI variables.
  Only this lane is exempt from "no network calls", and only for the sandbox hosts.

CalVer scenarios need a controllable clock. `internal/app` resolves its clock through a `clock()`
seam: `time.Now` in every shipped build, and a clock pinned by `HERAUT_TEST_NOW` (RFC 3339) only when
compiled with `-tags heraut_testclock`. Released binaries never contain the pinned-clock code.

## Alternatives considered

- **`libfaketime` / `LD_PRELOAD`:** unreliable, Go reads the clock through the vDSO and direct
  syscalls, not libc; blocked by SIP on macOS.
- **A production `HERAUT_NOW` variable or `--now` flag:** a user-visible knob that changes release
  versions is a footgun and a new CLI surface for a testing need.
- **Changing the machine clock:** needs root, breaks TLS and git, and cannot run on a developer
  machine.
- **Forge scenarios in `go test ./...`:** violates determinism and would need write tokens on pull
  requests.

## Consequences

- The e2e binary differs from the shipped one by one build-tagged file. Only CalVer scenarios use the
  tagged binary.
- `go test ./...` builds the binary once per tag set (about one second warm).
- Lane B needs sandbox repositories, scoped tokens and a cleanup sweeper; they are specified in the
  design doc and delivered in T345c/T345d.
- A failing Lane A scenario is a regression to fix, never a test to loosen.
