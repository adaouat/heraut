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
| Contract    | CLI invocation (`git`, `gh`, `glab`) or direct HTTP (`internal/forge/*` enrichment) | `exectest.MockRunner` (CLI) / `httptest.Server` (HTTP) |
| Integration | Full pipeline against a real git repo, or cobra commands in-process | `go test` + `exectest.FakeBin` / `internal/testutil.RealGitRepo` |
| Schema      | `.heraut.yml` validates against `schema.json`                       | JSON Schema + fixtures               |

Every code change needs unit tests. Contract tests are mandatory for every CLI invocation
of an external tool — no platform driver, no generator, no `git` subcall ships without
asserting the exact arguments passed. The `internal/forge/*` HTTP enrichment clients get
the same rigor against a real `httptest.Server` instead, since there's no CLI invocation
to assert arguments on.

## MockRunner — the contract test workhorse

`github.com/adaouat/forge/exec/exectest.MockRunner` (not `internal/testutil` — that package
holds only heraut-specific fixtures: `MockGenerator`, `MockPlatform`, `RealGitRepo`,
CI-env-clearing helpers) implements `port.Runner` and records every call:

```go
mr := exectest.NewMockRunner()
mr.QueueResponse("", "", nil) // stdout, stderr, err — ordered FIFO

plat := github.New(mr, &config.Platform{Repository: "acme/widget", TokenEnv: "GH_TOKEN"})
require.NoError(t, plat.CreateRelease("v1.2.3", "release notes body"))

require.Len(t, mr.Calls, 1)
assert.Equal(t, "gh", mr.Calls[0].Name)
assert.Contains(t, mr.Calls[0].Args, "--notes-file") // notes go to a temp file, never passed inline
assert.Contains(t, mr.Calls[0].Args, "acme/widget")
```

When the assertion is about *which CLI args were passed*, use `MockRunner`. Never reach
for actual exec.

## FakeBin — integration tests

`github.com/adaouat/forge/exec/exectest.FakeBin` installs a fake binary in `PATH` so the
production `exec.Runner` can find it. Use this when the test needs the real exec path (e.g.
testing that `Runner.Run` correctly forwards stdin/stdout, that env vars propagate, that
exit codes map to errors).

Reach for FakeBin sparingly — most behavior can be verified at the contract layer.

## Real-CLI smoke tests

heraut previously carved out a narrow exception here for testing embedded external-tool configs
against the real binary (git-cliff, then cocogitto — both since removed, see ADR-0028 and
ADR-0045). `native`, heraut's sole generator since ADR-0045, has zero external-binary dependency
for generation, so this exception category currently has no live example. Revive this pattern if a
future external dependency needs the same "MockRunner can't catch a real-tool rejection" coverage.

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
- No network calls. Platform driver tests use `MockRunner` — never call the real
  GitHub/GitLab API. `internal/forge/*` enrichment clients use a real `httptest.Server`
  instead (§ Four test layers above). heraut has no self-update mechanism to test
  (superseded by forge ADR-0005; update checking is `forge/updatecheck`'s responsibility).
- No filesystem outside `t.TempDir()`. Embedded native template defaults
  (`internal/generators/native/*.tmpl`) are tested by reading through the production
  accessor, not by reaching into the source tree.
- No environment leakage. If a test needs `GH_TOKEN`, set it with `t.Setenv("GH_TOKEN", …)`
  so it is restored on test exit.

## Coverage discipline

- Unit + contract: every exported function in `internal/versioning/`, `internal/config/`,
  `internal/generators/`, `internal/platforms/`, `internal/forge/`, `internal/app/` has
  tests.
- Integration: reserved for the full release/changelog pipeline flows. One happy-path +
  one dry-run path per pipeline is the minimum.
- Schema: every value of `versioning.strategy` and every value of `forges[].platform` has
  at least one valid fixture in `testdata/config/valid/`. `generator:` is a removed config
  key (ADR-0045) — `testdata/config/invalid/invalid_generator.yml` covers its removal, not
  a set of generator values to enumerate.

## When a hook or test fails

Fix the root cause. Do not:

- Comment out the assertion
- Add `t.Skip()` without explanation
- Loosen the assertion to make it pass
- Suppress the linter warning

Each of these defeats the test's purpose. If the test is wrong, fix the test in a
separate commit with an explanation.
