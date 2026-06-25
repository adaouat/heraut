# Testing rules

## TDD is required

Write the failing test before writing implementation code. The cycle is **red → green →
refactor**. If you are tempted to skip the test "because the change is small", stop —
that is exactly when the test is most valuable.

If the user asks you to skip tests, push back and explain why the test is needed. Do not
silently agree.

## Four test layers

| Layer       | Scope                                                               | Tooling                              |
|-------------|---------------------------------------------------------------------|--------------------------------------|
| Unit        | Pure functions (version resolvers, config parsers, tag format)      | `go test ./...`                      |
| Contract    | External CLI interactions (`gh`, `glab`, `git-cliff`, `cog`, …)     | `testutil.MockRunner`                |
| Integration | Full pipeline against a real git repo + fake binaries in `PATH`     | `go test` + `testutil.FakeBin`       |
| Schema      | `.heraut.yml` validates against `schema.json`                       | JSON Schema + fixtures               |

Every code change needs unit tests. Contract tests are mandatory for every CLI invocation
of an external tool — no platform driver, no generator, no `git` subcall ships without
asserting the exact arguments passed.

## MockRunner — the contract test workhorse

`internal/testutil/mock_runner.go` implements `port.Runner` and records every call:

```go
mr := testutil.NewMockRunner()
mr.QueueResponse("", "", nil) // stdout, stderr, err — ordered FIFO

gen := gitcliff.New(mr, cfg)
_, err := gen.Generate("v1.2.3")
require.NoError(t, err)

require.Len(t, mr.Calls, 1)
assert.Equal(t, "git-cliff", mr.Calls[0].Name)
assert.Equal(t, []string{"--config", "<tmpfile>", "--tag", "v1.2.3", ...}, mr.Calls[0].Args)
```

When the assertion is about *which CLI args were passed*, use `MockRunner`. Never reach
for actual exec.

## FakeBin — integration tests

`internal/testutil/fakebin.go` installs a fake binary in `PATH` so the production
`exec.Runner` can find it. Use this when the test needs the real exec path (e.g. testing
that `Runner.Run` correctly forwards stdin/stdout, that env vars propagate, that exit
codes map to errors).

Reach for FakeBin sparingly — most behavior can be verified at the contract layer.

## Real-CLI smoke tests (embedded config validation)

A narrow, deliberate exception to "mock the externals": a **skippable** test runs the
*real* `git-cliff` against heraut's **embedded default config** (via
`testutil.RealGitRepo`), asserting the tool *accepts* the config. MockRunner can't catch an
embedded TOML the real tool rejects — that gap once shipped a broken default for a
generator heraut has since dropped (T117/ADR-0028). This test `t.Skip`s when the binary is
absent and runs in CI, where `mise` installs the pinned tool. Keep it to a
config-acceptance smoke check (no output assertions — those stay byte-level / manual);
it is local and deterministic (no network, `t.TempDir`).

## Table-driven tests preferred

Group related cases into a single test with a `[]struct` of inputs and expected outputs.
Each row gets a `name`, ideally describing the scenario:

```go
tests := []struct {
    name      string
    commits   []string
    want      semver.BumpType
}{
    {"feat → minor", []string{"feat: add x"}, semver.BumpMinor},
    {"fix → patch", []string{"fix: y"}, semver.BumpPatch},
    {"feat! → major", []string{"feat!: breaking"}, semver.BumpMajor},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) { /* … */ })
}
```

## Preserve hard-won edge cases

The test suite contains hard-won edge cases — `v1.9.0` → `v1.10.0` (not `v1.100.0`),
CalVer `PATCH` reset on period boundary, per-env cycle detection, and more.

**Never delete a test row to make a change easier.** If a test asserts something, that
assertion is load-bearing until proven otherwise. Drop a row only when the behaviour it
tested is deliberately changed — and only after writing an ADR documenting the change.

## Determinism

- No time-of-day dependencies. CalVer resolver takes a `now func() time.Time` so tests
  can fix the clock.
- No network calls. Self-update tests use an `httptest.Server`. Platform tests use
  MockRunner — never call the real GitHub/GitLab API.
- No filesystem outside `t.TempDir()`. Embedded TOML defaults are tested by reading
  through the production accessor, not by reaching into the source tree.
- No environment leakage. If a test needs `GH_TOKEN`, set it with `t.Setenv("GH_TOKEN", …)`
  so it is restored on test exit.

## Coverage discipline

- Unit + contract: every exported function in `internal/versioning/`, `internal/config/`,
  `internal/generators/`, `internal/platforms/`, `internal/app/` has tests.
- Integration: reserved for the full release/changelog pipeline flows. One happy-path +
  one dry-run path per pipeline is the minimum.
- Schema: every value of `versioning.strategy`, every value of `platform`, and every
  value of `generator` has at least one valid fixture in `testdata/config/`.

## When a hook or test fails

Fix the root cause. Do not:

- Comment out the assertion
- Add `t.Skip()` without explanation
- Loosen the assertion to make it pass
- Suppress the linter warning

Each of these defeats the test's purpose. If the test is wrong, fix the test in a
separate commit with an explanation.
