# T117: Drop the `cocogitto` Generator Entirely — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `generator: cocogitto` feature from heraut entirely — config validation, generator wiring, the `internal/generators/cocogitto/` package, the `heraut init` wizard option, and every config/doc surface that mentions it — as a hard cutover (no deprecation window), per ADR-0028.

**Architecture:** No new abstractions. This is subtractive: `git-cliff` and `communique` already cover every real use case cocogitto served (ADR-0028's Context section). The work touches five layers in order: config validation (the gate that rejects the value), generator construction + runtime check (the code that builds/probes the cocogitto binary), the `internal/generators/cocogitto/` package itself (deleted outright), the `heraut init` wizard (stops offering it), and finally every config/doc surface (schema, sample config, Dockerfile, mise, renovate, README, CLAUDE.md, specs, project rules) that still names it as a live option.

**Tech Stack:** Go 1.26, `stretchr/testify` (`assert`/`require`), `exectest.MockRunner` (contract tests), no new dependencies — this plan removes a dependency (`cog`/cocogitto) from the Dockerfile/mise, never adds one.

## Global Constraints

- TDD: write/update the failing test before the implementation, every task (`testing.md`, `.claude/rules/claude.md`).
- Every `if err != nil` propagating an existing error wraps with `%w` (`coding.md`); this plan introduces no new error-wrapping call sites (the one error message touched — `buildGenerator`'s `default` case — already wraps nothing, since it originates the error itself via `fmt.Errorf` with no inner `%w` target).
- Never `os.Exit` below `cmd/heraut/`. This plan touches no `cmd/` code.
- Layer rules (`coding.md`) are unaffected — no new imports, only the removal of `internal/app/pipeline.go`'s `cocogitto` import.
- `cocogitto` remains in `.config/mise/config.toml` until **this task's own Task 4** removes it — do not remove it earlier, since `internal/generators/cocogitto/`'s real-CLI smoke test (deleted in Task 2) needs the binary on `PATH` for any task run before Task 2 lands.
- Never pass `--no-verify`/`--no-gpg-sign` to git; never bypass `hk` hooks. If a hook fails, fix the root cause.
- Conventional-commit subject lines for this work's own commits, per `workflow.md`'s type table (`feat`, `fix`, `refactor`, `chore`, `docs`, `test`, ...). Removing a feature and its docs is most naturally `feat`/`chore`/`docs` depending on which files a given task touches — see each task's commit step for the exact type chosen.
- **Never delete a test row to make a change easier** (`testing.md`) — every test assertion removed in this plan is removed because the *behavior it tested no longer exists* (ADR-0028 is the documenting ADR this rule requires), not because it was inconvenient. Tests that assert a *different* behavior using `cocogitto` merely as a stand-in value (e.g. "any non-git-cliff generator") are *not* deleted — their fixture is swapped to `communique` so they keep testing the same rule.
- Reference docs for this work: [ADR-0028](../../docs/adr/0028-drop-cocogitto-generator.md), [T117 in the roadmap](../../docs/tasks/roadmap.md).

---

### Task 1: Validator rejects `generator: cocogitto`

**Files:**
- Modify: `internal/config/validator.go:16-19,209-220`
- Modify: `internal/config/validator_test.go:995-1006,1013-1026,1082-1098,1192-1204` (swap fixture generator), plus one new test

**Interfaces:**
- Produces: `generator: cocogitto` now fails `config.Validate()` with `Message: "\"cocogitto\" is not a valid generator"` and `Hint: "valid generators: git-cliff, communique"`. Task 2 (generator wiring) and Task 3 (wizard) do not depend on this task's exact error text, only on the fact that `cocogitto` is no longer accepted by config loading — they can run independently of this task's completion order, but Task 1 must land before Task 5 (final gate) since it's part of the same removal.

This task does **not** touch `internal/generators/cocogitto/`, `internal/app/`, or the wizard — after this task lands alone, `generator: cocogitto` is rejected by `.heraut.yml` loading, but the dead code in those other layers still compiles and its own tests still pass unchanged (confirmed: `internal/app/pipeline_test.go`'s `TestBuildPipeline_WithCocogitto` builds a `config.Config` directly in Go, bypassing `Validate()` entirely, so it is unaffected by this task and remains green until Task 2 removes it).

**`testdata/config/` already has zero fixtures referencing `cocogitto`** (confirmed: `grep -rl -i cocogitto testdata/` returns no matches against the 20 existing fixture files) — despite the roadmap's T117 description listing "remove or migrate `testdata/config/` fixtures" as a checklist item, there is nothing to do there. Do not spend time searching for fixtures that don't exist; Task 4's Step 17 repo-wide sweep re-confirms this as part of its broader verification.

- [ ] **Step 1: Write the failing test**

Add to `internal/config/validator_test.go` (anywhere among the other `generator`-related tests, e.g. directly after `TestValidate_changelogTagPatternRequiresGitCliff`):

```go
func TestValidate_GeneratorCocogittoRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: cocogitto
`)
	e := findErr(config.Validate(cfg), "changelog.generator")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "cocogitto")
	assert.Contains(t, e.Hint, "git-cliff")
	assert.Contains(t, e.Hint, "communique")
	assert.NotContains(t, e.Hint, "cocogitto")
}
```

- [ ] **Step 2: Run it to verify it currently fails**

Run: `go test ./internal/config/... -run TestValidate_GeneratorCocogittoRejected -v`
Expected: FAIL — `cocogitto` is currently a valid generator, so `findErr` finds no `changelog.generator` error and `e` is `nil`, failing the `require.NotNil(t, e)` line.

- [ ] **Step 3: Remove `cocogitto` from the valid-generators set and hint strings**

In `internal/config/validator.go`, change:

```go
	validGenerators = map[string]bool{
		"git-cliff": true, "communique": true, "cocogitto": true,
	}
```

to:

```go
	validGenerators = map[string]bool{
		"git-cliff": true, "communique": true,
	}
```

Then in `validateContentDriver` (same file), change:

```go
	if d.Generator == "" {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: "required",
			Hint:    "set generator to one of: git-cliff, communique, cocogitto",
		})
	} else if !validGenerators[d.Generator] {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: fmt.Sprintf("%q is not a valid generator", d.Generator),
			Hint:    "valid generators: git-cliff, communique, cocogitto",
		})
	}
```

to:

```go
	if d.Generator == "" {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: "required",
			Hint:    "set generator to one of: git-cliff, communique",
		})
	} else if !validGenerators[d.Generator] {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: fmt.Sprintf("%q is not a valid generator", d.Generator),
			Hint:    "valid generators: git-cliff, communique",
		})
	}
```

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test ./internal/config/... -run TestValidate_GeneratorCocogittoRejected -v`
Expected: PASS

- [ ] **Step 5: Swap the 4 existing fixtures that use `cocogitto` only as a stand-in "non-git-cliff generator"**

These four tests assert a rule unrelated to cocogitto's validity (tickets/tag_pattern/remote requiring git-cliff) and used `cocogitto` merely as *an* invalid-for-that-rule generator. Now that `cocogitto` itself is invalid, these fixtures would start failing for the *wrong* reason (an "invalid generator" error, not the rule under test). Swap each fixture's `generator: cocogitto` to `generator: communique` — a generator that is valid but still not `git-cliff`, preserving exactly what each test is meant to prove.

In `internal/config/validator_test.go`:

`TestValidate_TicketsNonGitCliffGenerator` (~line 1004) — change:
```yaml
changelog:
  generator: cocogitto
tickets:
```
to:
```yaml
changelog:
  generator: communique
tickets:
```

`TestValidate_changelogTagPatternRequiresGitCliff` (~line 1021) — change:
```yaml
changelog:
  generator: cocogitto
  tag_pattern: "v[0-9]*"
```
to:
```yaml
changelog:
  generator: communique
  tag_pattern: "v[0-9]*"
```

`TestValidate_perEnvTagPatternGeneratorSwitchRejected` (~line 1095) — change:
```yaml
environments:
  prod:
    bump: auto
    changelog:
      generator: cocogitto
      tag_pattern: "prod/*"
```
to:
```yaml
environments:
  prod:
    bump: auto
    changelog:
      generator: communique
      tag_pattern: "prod/*"
```

`TestValidate_changelogRemoteRequiresGitCliff` (~line 1201) — change:
```yaml
changelog:
  generator: cocogitto
  remote:
    type: azure_devops
```
to:
```yaml
changelog:
  generator: communique
  remote:
    type: azure_devops
```

- [ ] **Step 6: Run the full config package test suite**

Run: `go test ./internal/config/... -v`
Expected: PASS — all tests green, including the 4 swapped fixtures and the new `TestValidate_GeneratorCocogittoRejected`.

- [ ] **Step 7: Commit**

```bash
git add internal/config/validator.go internal/config/validator_test.go
git commit -m "feat(config): reject generator: cocogitto (ADR-0028)"
```

---

### Task 2: Remove the `cocogitto` generator from runtime wiring and delete the package

**Files:**
- Modify: `internal/app/pipeline.go:11,278-293`
- Modify: `internal/app/pipeline_test.go` (`TestBuildPipeline_WithCocogitto`)
- Modify: `internal/app/check.go:53,160-164,184`
- Modify: `internal/app/check_test.go` (~20 line removals + 2 length assertions + 1 name-list assertion + 2 warn-set assertions + 2 doc comments — full enumeration in Step 5)
- Delete: `internal/generators/cocogitto/` (all 6 files: `generator.go`, `cog.toml`, `changelog.tera`, `release-notes.tera`, `generator_test.go`, `embed_internal_test.go`)

**Interfaces:**
- Consumes: nothing from Task 1 directly (Task 1 only touches the config layer); this task is independently runnable, but per the Global Constraints, must land before Task 5.
- Produces: `buildGenerator` and `RuntimeCheck` no longer recognize `cocogitto` as anything but an arbitrary unsupported string. Task 3 (wizard) and Task 4 (docs) do not depend on any new symbol this task introduces — they depend only on the *absence* of `internal/generators/cocogitto/` and the absence of cocogitto's row in `check.go`'s generator list.

**Why ~20 `check_test.go` lines change, not just the rows actually mentioning "cocogitto":** `check.go`'s `RuntimeCheck` calls `runner.Run` once per generator probe, in a fixed loop order (`git-cliff`, `cocogitto`, `communique`). `exectest.MockRunner.QueueResponse` is a strict FIFO queue — each test pre-loads exactly one canned response per expected `runner.Run` call, in call order. Removing the `cocogitto` row from that loop means the `cog --version` call **never happens** — every queued response that follows it in a test's setup would shift left by one and get matched to the wrong call (the `communique` probe would receive the response meant for `cog`, and the last queued response would go unconsumed). Every test that calls `runner.Run` enough times to reach the generators section must have its `cog`-related queued response removed, or its assertions will silently desync.

- [ ] **Step 1: Update `internal/app/pipeline_test.go`**

`TestBuildPipeline_WithCocogitto` currently asserts cocogitto builds a pipeline successfully — that assertion is now false, per ADR-0028. Per `testing.md`'s "never delete a test row... drop a row only when the behaviour it tested is deliberately changed, and only after writing an ADR documenting the change" — ADR-0028 is that ADR. Replace the assertion (not just delete it) with one asserting `cocogitto` now hits the same "unsupported generator" path as any other unrecognized string, preserving a regression guard for this specific literal string rather than silently losing it:

Change:
```go
func TestBuildPipeline_WithCocogitto(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "cocogitto", Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}
```
to:
```go
func TestBuildPipeline_CocogittoNoLongerSupported(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Generator: "cocogitto", Output: "CHANGELOG.md"}
	_, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cocogitto")
}
```

- [ ] **Step 2: Run it to verify it currently fails**

Run: `go test ./internal/app/... -run TestBuildPipeline_CocogittoNoLongerSupported -v`
Expected: FAIL — `buildGenerator`'s switch still has a `case "cocogitto":` branch, so `BuildPipeline` currently succeeds; `require.Error(t, err)` fails because `err` is `nil`.

- [ ] **Step 3: Remove the `cocogitto` case from `buildGenerator`**

In `internal/app/pipeline.go`, remove the import (line 11):

```go
	"github.com/adaouat/heraut/internal/generators/cocogitto"
```

Then change `buildGenerator`:

```go
func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode gitcliff.Mode) (port.Generator, error) {
	switch driver.Generator {
	case "git-cliff":
		return gitcliff.New(runner, driver, defaultMode), nil
	case "communique":
		return communique.New(runner, driver), nil
	case "cocogitto":
		mode := cocogitto.ModeChangelog
		if defaultMode == gitcliff.ModeReleaseNotes {
			mode = cocogitto.ModeReleaseNotes
		}
		return cocogitto.New(runner, driver, mode), nil
	default:
		return nil, fmt.Errorf("unsupported generator %q (supported: git-cliff, communique, cocogitto)", driver.Generator)
	}
}
```

to:

```go
func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode gitcliff.Mode) (port.Generator, error) {
	switch driver.Generator {
	case "git-cliff":
		return gitcliff.New(runner, driver, defaultMode), nil
	case "communique":
		return communique.New(runner, driver), nil
	default:
		return nil, fmt.Errorf("unsupported generator %q (supported: git-cliff, communique)", driver.Generator)
	}
}
```

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test ./internal/app/... -run TestBuildPipeline_CocogittoNoLongerSupported -v`
Expected: PASS

- [ ] **Step 5: Update `internal/app/check.go`**

Change the doc comment (line 53):
```go
//	Generators: git-cliff → cocogitto → communique
```
to:
```go
//	Generators: git-cliff → communique
```

Change the generator probe list (lines 160-164):
```go
	for _, og := range []struct{ name, binary, display string }{
		{"git-cliff", "git-cliff", "git-cliff"},
		{"cocogitto", "cog", "cocogitto"},
		{"communique", "communique", "communique"},
	} {
```
to:
```go
	for _, og := range []struct{ name, binary, display string }{
		{"git-cliff", "git-cliff", "git-cliff"},
		{"communique", "communique", "communique"},
	} {
```

Change the nil-cfg fallback in `configuredGenerators` (line 184):
```go
		return map[string]bool{"git-cliff": true, "cocogitto": true, "communique": true}
```
to:
```go
		return map[string]bool{"git-cliff": true, "communique": true}
```

- [ ] **Step 6: Update `internal/app/check_test.go` — the shared `queueSuccess` helper**

Change (lines 28-40):
```go
// with all tools present. Call order: git, user.name, user.email, git status,
// glab, gh, git-cliff, cog, communique.
func queueSuccess(mr *exectest.MockRunner) {
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("glab 1.0.0", "", nil)         // glab --version
	mr.QueueResponse("gh 2.0.0", "", nil)           // gh --version
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // git-cliff --version
	mr.QueueResponse("cog 7.0.0", "", nil)          // cog --version
	mr.QueueResponse("communique 1.0.0", "", nil)   // communique --version
}
```
to:
```go
// with all tools present. Call order: git, user.name, user.email, git status,
// glab, gh, git-cliff, communique.
func queueSuccess(mr *exectest.MockRunner) {
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // git config user.name
	mr.QueueResponse("a@b.com", "", nil)            // git config user.email
	mr.QueueResponse("", "", nil)                   // git status --porcelain (clean)
	mr.QueueResponse("glab 1.0.0", "", nil)         // glab --version
	mr.QueueResponse("gh 2.0.0", "", nil)           // gh --version
	mr.QueueResponse("git-cliff 2.9.0", "", nil)    // git-cliff --version
	mr.QueueResponse("communique 1.0.0", "", nil)   // communique --version
}
```

This single helper is used by 5 tests (`TestRuntimeCheck_MinimalConfig`, `TestRuntimeCheck_SectionHeaders`, `TestRuntimeCheck_OptionalToolsSilentWhenPresent`, `TestRuntimeCheck_NilConfig_AllToolsPassWhenPresent`, and one more — confirm by running `grep -n "queueSuccess(" internal/app/check_test.go`), so this one edit fixes all of them at once; do not edit those 5 call sites individually.

- [ ] **Step 7: Update every test that manually builds its own `MockRunner` queue**

The following test functions each contain exactly one line of the shape `mr.QueueResponse("cog 7.0", "", nil) // cog ...` (or, for the two "missing" tests, `mr.QueueResponse("", "", errors.New("cog: not found")) // cog ...`) that must be deleted outright — no replacement line, the queue simply has one fewer entry, matching the one fewer `runner.Run` call `RuntimeCheck` now makes:

| Function | Approx. line | Current line to delete |
|---|---|---|
| `TestRuntimeCheck_GitValue` | 112 | `mr.QueueResponse("cog 7.0", "", nil)              // cog` |
| `TestRuntimeCheck_UserNameValue` | 137 | `mr.QueueResponse("cog 7.0", "", nil)             // cog` |
| `TestRuntimeCheck_WorkingTreeClean` | 162 | `mr.QueueResponse("cog 7.0", "", nil)` |
| `TestRuntimeCheck_WorkingTreeDirty` | 188 | `mr.QueueResponse("cog 7.0", "", nil)` |
| `TestRuntimeCheck_WithGitcliff` | 249 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_WithGitHubPlatform` | 279 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_WithGitHubPlatform_MissingToken` | 310 | `mr.QueueResponse("cog 7.0", "", nil)        // cog (optional)` |
| `TestRuntimeCheck_EnvPlatformOverrideReplacesRoot` | 340 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_EnvWithoutPlatformOverrideInheritsRoot` | 377 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_UnknownChangelogGenerator` | 410 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_UnknownPlatform` | 432 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_MultipleSameTypePlatforms` | 466 | `mr.QueueResponse("cog 7.0", "", nil)        // cog (optional)` |
| `TestRuntimeCheck_UserNameMissing` | 498 | `mr.QueueResponse("cog 7.0", "", nil)` |
| `TestRuntimeCheck_UserEmailMissing` | 522 | `mr.QueueResponse("cog 7.0", "", nil)` |
| `TestRuntimeCheck_WithReleaseNotes` | 546 | `mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)` |
| `TestRuntimeCheck_OptionalGeneratorsWarnWhenMissing` | 576 | `mr.QueueResponse("", "", errors.New("cog: not found"))        // cog (optional, missing)` |
| `TestRuntimeCheck_OptionalPlatformsWarnWhenMissing` | 604 | `mr.QueueResponse("cog 7.0.0", "", nil)                  // cog (optional, found)` |
| `TestRuntimeCheck_ConfiguredGeneratorExcludedFromOptional` | 644 | `mr.QueueResponse("", "", errors.New("cog: not found"))        // cog (optional, missing)` |
| `TestRuntimeCheck_NilConfig_MissingBinaryIsHardError` | 692 | `mr.QueueResponse("cog 7.0.0", "", nil)                  // cog` |
| `TestRuntimeCheck_NilConfig_MissingGeneratorIsHardError` | 716 | `mr.QueueResponse("cog 7.0.0", "", nil)                       // cog` |

Delete each of these 20 lines (line numbers will shift as you go top-to-bottom through the file — work from the top down, or search by the function name + the literal `"cog "` text each time rather than relying on the line numbers above once earlier deletions have shifted later ones).

- [ ] **Step 8: Fix the assertions that reference cocogitto's name or count, beyond the queue lines themselves**

`TestRuntimeCheck_DispatchNames` (~line 219) — change:
```go
	assert.Equal(t, []string{
		"git", "git user.name", "git user.email", "working tree",
		"glab", "gh",
		"git-cliff", "cocogitto", "communique",
	}, names)
```
to:
```go
	assert.Equal(t, []string{
		"git", "git user.name", "git user.email", "working tree",
		"glab", "gh",
		"git-cliff", "communique",
	}, names)
```

`TestRuntimeCheck_MinimalConfig` (~line 91) — change:
```go
	require.Len(t, items, 9)
```
to:
```go
	require.Len(t, items, 8)
```

`TestRuntimeCheck_NilConfig_AllToolsPassWhenPresent` (~line 673) — change:
```go
	require.Len(t, items, 9)
```
to:
```go
	require.Len(t, items, 8)
```

`TestRuntimeCheck_OptionalGeneratorsWarnWhenMissing` (~line 589) — delete this line entirely (no replacement; there is no longer a `cocogitto` item to assert a warning for):
```go
	assert.True(t, warnNames["cocogitto"], "expected optional warn for cocogitto")
```

`TestRuntimeCheck_ConfiguredGeneratorExcludedFromOptional` (~line 662) — delete this line entirely:
```go
	assert.True(t, warnNames["cocogitto"])
```

`TestRuntimeCheck_UnknownChangelogGenerator` (~line 399) — update the comment (the code below it is unaffected, it already correctly says "checks only the N supported generators" generically, just fix the count and the rationale text):
```go
	// "unknown-gen" is not a recognized generator; config validation would
	// normally catch this. RuntimeCheck checks only the 3 supported generators.
	// An unknown configured generator produces no runtime check item.
```
to:
```go
	// "unknown-gen" is not a recognized generator; config validation would
	// normally catch this. RuntimeCheck checks only the 2 supported generators.
	// An unknown configured generator produces no runtime check item.
```

- [ ] **Step 9: Verify no `cog`/`cocogitto` reference remains in either test file**

Run: `grep -n "cog\b\|cocogitto" internal/app/check_test.go internal/app/pipeline_test.go`
Expected: no output (zero matches). If anything remains, it was missed in Steps 6-8 above — find and fix it before proceeding.

- [ ] **Step 10: Run the full `internal/app` test suite**

Run: `go test ./internal/app/... -v`
Expected: PASS — every `TestRuntimeCheck_*` and `TestBuildPipeline_*` test green, including the corrected counts/lists from Step 8.

- [ ] **Step 11: Delete the `internal/generators/cocogitto/` package**

```bash
rm -rf internal/generators/cocogitto
```

- [ ] **Step 12: Run the full build and test suite to confirm nothing else references the deleted package**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass. (If `go build` fails with an unresolved import, something outside this task's file list still references `internal/generators/cocogitto` — Task 4's repo-wide grep in its own Step 1 is a second, independent check for this, but `go build` failing here means it must be fixed in *this* task before committing, not deferred.)

- [ ] **Step 13: Commit**

```bash
git add internal/app/pipeline.go internal/app/pipeline_test.go internal/app/check.go internal/app/check_test.go
git rm -r internal/generators/cocogitto
git commit -m "feat(app): remove the cocogitto generator and its package (ADR-0028)"
```

---

### Task 3: Remove `cocogitto` from the `heraut init` wizard

**Files:**
- Modify: `internal/scaffold/wizard.go:29,32,228,243`
- Delete: `internal/scaffold/cog.go`
- Modify: `internal/scaffold/cliff_cog_test.go` → rename to `internal/scaffold/cliff_test.go`

**Interfaces:**
- Consumes: nothing from Tasks 1-2 (the wizard builds an `Answers` struct that's later serialized to YAML and validated by the same `config.Validate()` Task 1 already updated — no direct code dependency, just consistency: leaving this task undone would let `heraut init` produce a config that's immediately rejected).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing test**

`scaffold.IsCogGenerator` currently has no test asserting it no longer exists (you can't test for a function's absence with a unit test) — instead, write a compile-time-equivalent check by confirming the symbol is gone after Step 3 below. There is no separate RED step here in the usual sense, since "remove a function" doesn't have a meaningful failing-test-first form — go straight to Step 2, which is itself the verification that the existing test (`TestIsCogGenerator`) currently passes (proving the function exists today, the thing being removed).

Run: `go test ./internal/scaffold/... -run TestIsCogGenerator -v`
Expected: PASS (confirms `IsCogGenerator` exists and works today, before removal).

- [ ] **Step 2: Remove the wizard's `cocogitto` options**

In `internal/scaffold/wizard.go`, change the two doc comments (lines 29, 32):
```go
	ChangelogGenerator string // git-cliff, communique, cocogitto, or "" (none)
	ChangelogOutput    string // e.g. "CHANGELOG.md"

	NotesGenerator string // git-cliff, communique, cocogitto, or "" (none)
```
to:
```go
	ChangelogGenerator string // git-cliff, communique, or "" (none)
	ChangelogOutput    string // e.g. "CHANGELOG.md"

	NotesGenerator string // git-cliff, communique, or "" (none)
```

Then remove the two `cocogitto` select options (lines 228, 243):
```go
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("cocogitto", "cocogitto"),
					huh.NewOption("None", ""),
				).
				Value(&a.ChangelogGenerator),
```
to:
```go
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("None", ""),
				).
				Value(&a.ChangelogGenerator),
```
and:
```go
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("cocogitto", "cocogitto"),
					huh.NewOption("None", ""),
				).
				Value(&a.NotesGenerator),
```
to:
```go
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("None", ""),
				).
				Value(&a.NotesGenerator),
```

- [ ] **Step 3: Delete the dead `IsCogGenerator` helper**

```bash
rm internal/scaffold/cog.go
```

`IsCogGenerator` has no production caller anywhere in the repo (confirmed: `grep -rn "scaffold.IsCogGenerator" --include="*.go" .` matches only its own test file) — it is safe to delete outright rather than leave as unreferenced dead code.

- [ ] **Step 4: Rename and edit the test file**

```bash
git mv internal/scaffold/cliff_cog_test.go internal/scaffold/cliff_test.go
```

Then edit `internal/scaffold/cliff_test.go` — change:
```go
func TestIsCliffGenerator(t *testing.T) {
	assert.True(t, scaffold.IsCliffGenerator("git-cliff"))
	assert.False(t, scaffold.IsCliffGenerator("communique"))
	assert.False(t, scaffold.IsCliffGenerator("cocogitto"))
	assert.False(t, scaffold.IsCliffGenerator(""))
}

func TestIsCogGenerator(t *testing.T) {
	assert.True(t, scaffold.IsCogGenerator("cocogitto"))
	assert.False(t, scaffold.IsCogGenerator("git-cliff"))
	assert.False(t, scaffold.IsCogGenerator("communique"))
	assert.False(t, scaffold.IsCogGenerator(""))
}
```
to:
```go
func TestIsCliffGenerator(t *testing.T) {
	assert.True(t, scaffold.IsCliffGenerator("git-cliff"))
	assert.False(t, scaffold.IsCliffGenerator("communique"))
	assert.False(t, scaffold.IsCliffGenerator(""))
}
```

(`TestIsCogGenerator` is deleted entirely, not merely emptied — the function it tested no longer exists. The `"cocogitto"` negative case inside `TestIsCliffGenerator` is also dropped since it is now testing against a generator that doesn't exist anywhere in the system, no different from any other arbitrary unrelated string — the test's remaining `"communique"` and `""` cases already cover "returns false for any non-git-cliff value.")

- [ ] **Step 5: Run the scaffold test suite**

Run: `go test ./internal/scaffold/... -v`
Expected: PASS — `TestIsCliffGenerator` passes with its reduced assertion set; `TestIsCogGenerator` no longer exists (correctly — confirm via `go test ./internal/scaffold/... -run TestIsCogGenerator -v`, which should report `no tests to run`, not a failure).

- [ ] **Step 6: Run the full build and test suite**

Run: `go build ./... && go test ./...`
Expected: clean build (confirms nothing else imports `scaffold.IsCogGenerator`), all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/wizard.go internal/scaffold/cliff_test.go
git rm internal/scaffold/cog.go internal/scaffold/cliff_cog_test.go
git commit -m "feat(scaffold): remove cocogitto from the heraut init wizard (ADR-0028)"
```

---

### Task 4: Drop `cocogitto` from every config and documentation surface

**Files:**
- Modify: `schema.json:171-179`
- Modify: `docs/heraut.sample.yml:118,124,132,137`
- Modify: `Dockerfile:10,38,47,53`
- Modify: `.config/mise/config.toml:7`
- Modify: `.config/mise/mise.lock` (regenerated, not hand-edited — see Step 5)
- Modify: `.github/renovate.json:56-62`
- Modify: `docs/adr/0016-bundled-docker-image.md:14,39` (the one historical ADR ADR-0028 explicitly says to amend, not leave as a historical record)
- Modify: `README.md:25,107`
- Modify: `CLAUDE.md:29,50,86`
- Modify: `docs/specs/README.md:9`
- Modify: `docs/specs/01-overview.md:62,101`
- Modify: `docs/specs/02-configuration.md:390,438-442`
- Modify: `docs/specs/05-generators-and-platforms.md:9,16,144,147-213,239,362`
- Modify: `docs/specs/06-dx-and-testing.md:148`
- Modify: `.claude/rules/coding.md:19,87`
- Modify: `.claude/rules/testing.md:59-60,88`

**Interfaces:** None — this task touches no Go code and produces no symbols. It is purely documentation/config-surface cleanup, verified by tooling (`go build`, `hadolint`, JSON validity, `mise`/`hk` lint) rather than `go test`.

This task has no TDD red/green cycle (there is no test to fail-then-pass for "a sentence in a markdown file no longer mentions cocogitto") — each step is a direct edit followed by a verification command appropriate to that file type.

**Note on ordering:** Tasks 2 and 3 already deleted `internal/generators/cocogitto/` and the wizard's options. This task removes cocogitto from the surfaces that *describe* the feature (schema validation hints, sample config comments, the Dockerfile that bundles `cog`, the architecture docs) — it can run independently of Tasks 1-3's exact completion order, but should land before Task 5's final repo-wide grep.

- [ ] **Step 1: `schema.json`**

Change:
```json
        "generator": {
          "type": "string",
          "enum": [
            "git-cliff",
            "communique",
            "cocogitto"
          ],
          "description": "Content generator."
        },
```
to:
```json
        "generator": {
          "type": "string",
          "enum": [
            "git-cliff",
            "communique"
          ],
          "description": "Content generator."
        },
```

Verify: `jq . schema.json > /dev/null && echo "valid JSON"` — must print `valid JSON` (confirms no trailing-comma or syntax error was introduced).

- [ ] **Step 2: `docs/heraut.sample.yml`**

Change (lines 114-138, the `changelog:` block's comment lines):
```yaml
changelog:
  # generator — required. Tool used to produce the changelog content.
  #   git-cliff   recommended; uses cliff.toml deep-merged with heraut's built-in default
  #   communique  requires config: pointing to a communique config file
  #   cocogitto   uses cog.toml; heraut captures stdout and writes the output file
  generator: git-cliff

  # config — path to a generator config file (relative to project root).
  #   git-cliff:  optional partial cliff.toml; deep-merged with heraut's built-in default
  #   communique: required
  #   cocogitto:  optional path to cog.toml
  # config: cliff.toml

  # output — path where the changelog file is written.
  output: CHANGELOG.md

  # tag_pattern — tag glob passed to the generator for commit filtering.
  # Required with prefixed-tag strategies so the generator only sees commits
  # from the right namespace. Not used by communique or cocogitto.
  #   semver bare tags: "v[0-9]*"
  #   per-env prod:     "prod/*"
  # tag_pattern: "v[0-9]*"

  # template — path to a custom Tera template. cocogitto only (passed as -t).
  # template: .config/changelog.tera
```
to:
```yaml
changelog:
  # generator — required. Tool used to produce the changelog content.
  #   git-cliff   recommended; uses cliff.toml deep-merged with heraut's built-in default
  #   communique  requires config: pointing to a communique config file
  generator: git-cliff

  # config — path to a generator config file (relative to project root).
  #   git-cliff:  optional partial cliff.toml; deep-merged with heraut's built-in default
  #   communique: required
  # config: cliff.toml

  # output — path where the changelog file is written.
  output: CHANGELOG.md

  # tag_pattern — tag glob passed to the generator for commit filtering.
  # Required with prefixed-tag strategies so the generator only sees commits
  # from the right namespace. Not used by communique.
  #   semver bare tags: "v[0-9]*"
  #   per-env prod:     "prod/*"
  # tag_pattern: "v[0-9]*"

  # template — path to a custom Tera template. Not used by git-cliff or communique.
  # template: .config/changelog.tera
```

(The `template:` comment previously said "cocogitto only" — since cocogitto is gone, no generator consumes this field anymore via this sample file's documented options. Leave the field itself in the schema/sample as a vestigial-but-harmless no-op for `git-cliff`/`communique`, since removing the field entirely is out of scope for this task — confirm during Step 1's schema edit that `template` was not itself cocogitto-exclusive in `schema.json`; it is a generic property shared across generators per the Task 1 research, so no schema removal is needed here, only this comment's wording.)

Verify: `cat docs/heraut.sample.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin)" 2>&1 | tail -5` — should not error (sample file remains valid YAML; it's all comments in this block so this mainly guards against an accidental indentation break).

- [ ] **Step 3: `Dockerfile`**

Change:
```dockerfile
ARG GIT_CLIFF_VERSION=2.13.1
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0
ARG COCOGITTO_VERSION=7.0.0
ARG COMMUNIQUE_VERSION=1.1.3
```
to:
```dockerfile
ARG GIT_CLIFF_VERSION=2.13.1
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0
ARG COMMUNIQUE_VERSION=1.1.3
```

Change:
```dockerfile
ARG GIT_CLIFF_VERSION
ARG GLAB_VERSION
ARG GH_VERSION
ARG COCOGITTO_VERSION
ARG COMMUNIQUE_VERSION
```
to:
```dockerfile
ARG GIT_CLIFF_VERSION
ARG GLAB_VERSION
ARG GH_VERSION
ARG COMMUNIQUE_VERSION
```

Change:
```dockerfile
RUN mise use -g \
        git-cliff@${GIT_CLIFF_VERSION} \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
        cocogitto@${COCOGITTO_VERSION} \
        communique@${COMMUNIQUE_VERSION} \
    && mkdir /tools \
    && cp "$(mise which git-cliff)"  /tools/git-cliff \
    && cp "$(mise which glab)"       /tools/glab \
    && cp "$(mise which gh)"         /tools/gh \
    && cp "$(mise which cog)"        /tools/cog \
    && cp "$(mise which communique)" /tools/communique
```
to:
```dockerfile
RUN mise use -g \
        git-cliff@${GIT_CLIFF_VERSION} \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
        communique@${COMMUNIQUE_VERSION} \
    && mkdir /tools \
    && cp "$(mise which git-cliff)"  /tools/git-cliff \
    && cp "$(mise which glab)"       /tools/glab \
    && cp "$(mise which gh)"         /tools/gh \
    && cp "$(mise which communique)" /tools/communique
```

Verify: `hadolint Dockerfile` — must report no new warnings/errors (this is already a gated `hk` lint step in this repo; running it directly here catches the issue before the commit hook does).

- [ ] **Step 4: `.config/mise/config.toml`**

Change:
```toml
[tools]
actionlint = "latest"
cocogitto = "7.0.0"
git-cliff = "2.13"
```
to:
```toml
[tools]
actionlint = "latest"
git-cliff = "2.13"
```

- [ ] **Step 5: Regenerate `.config/mise/mise.lock`**

Run: `mise install`
Then verify: `grep -i cocogitto .config/mise/mise.lock`
Expected: no output. If the lockfile still contains a `[[tools.cocogitto]]` block after `mise install`, the installed `mise` version may not auto-prune removed tools from the lockfile — check `mise --help` for a prune/sync subcommand on the version installed in this environment (e.g. `mise prune` removes lockfile entries for tools no longer declared in `config.toml`), run it, then re-run `mise install` and re-verify with the same `grep` command above until it returns no output.

- [ ] **Step 6: `.github/renovate.json`**

Change:
```json
    {
      "customType": "regex",
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG GH_VERSION=(?<currentValue>[\\d.]+)"],
      "depNameTemplate": "cli/cli",
      "datasourceTemplate": "github-releases"
    },
    {
      "customType": "regex",
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG COCOGITTO_VERSION=(?<currentValue>[\\d.]+)"],
      "depNameTemplate": "cocogitto/cocogitto",
      "datasourceTemplate": "github-releases"
    },
    {
      "customType": "regex",
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG COMMUNIQUE_VERSION=(?<currentValue>[\\d.]+)"],
      "depNameTemplate": "jdx/communique",
      "datasourceTemplate": "github-releases"
    }
```
to:
```json
    {
      "customType": "regex",
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG GH_VERSION=(?<currentValue>[\\d.]+)"],
      "depNameTemplate": "cli/cli",
      "datasourceTemplate": "github-releases"
    },
    {
      "customType": "regex",
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG COMMUNIQUE_VERSION=(?<currentValue>[\\d.]+)"],
      "depNameTemplate": "jdx/communique",
      "datasourceTemplate": "github-releases"
    }
```

Verify: `jq . .github/renovate.json > /dev/null && echo "valid JSON"` — must print `valid JSON`.

- [ ] **Step 7: `docs/adr/0016-bundled-docker-image.md`** (the one historical ADR amended per ADR-0028's "What does not change" clause)

Change the prose (lines 13-14, in the Context section):
```markdown
also configured to push a minimal Alpine container image to GHCR — just `ca-certificates`
and the heraut binary. That image cannot actually run a release: heraut orchestrates
`git`, `git-cliff`, `gh`, `glab`, `cog`, and `communique`, none of which are present.
```
to:
```markdown
also configured to push a minimal Alpine container image to GHCR — just `ca-certificates`
and the heraut binary. That image cannot actually run a release: heraut orchestrates
`git`, `git-cliff`, `gh`, `glab`, and `communique`, none of which are present.
```

Change the bundled-tool-versions table:
```markdown
| Tool          | Version     | Source                        |
|---------------|-------------|--------------------------------|
| git-cliff     | `2.13.1`    | `.config/mise/config.toml`    |
| cocogitto     | `7.0.0`     | `.config/mise/config.toml`    |
| glab          | `1.97.0`    | chosen for image              |
| gh            | `2.92.0`    | chosen for image              |
| communique    | `1.1.3`     | chosen for image              |
```
to:
```markdown
| Tool          | Version     | Source                        |
|---------------|-------------|--------------------------------|
| git-cliff     | `2.13.1`    | `.config/mise/config.toml`    |
| glab          | `1.97.0`    | chosen for image              |
| gh            | `2.92.0`    | chosen for image              |
| communique    | `1.1.3`     | chosen for image              |
```

(Re-check the live `gh`/`glab` version numbers against the current `Dockerfile` ARGs before committing this table — the Dockerfile's `GLAB_VERSION`/`GH_VERSION` may have moved since this ADR was last touched; bring the table in sync with whatever the Dockerfile actually says today, not the values shown above if they've drifted, since ADR-0016 is meant to be a living inventory per ADR-0028's instruction.)

- [ ] **Step 8: `README.md`**

Change (line 25):
```markdown
It supports **four versioning strategies** (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), **three content generators** (`git-cliff`, `communique`, `cocogitto`),
and **two platforms** (`github`, `gitlab`).
```
to:
```markdown
It supports **four versioning strategies** (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), **two content generators** (`git-cliff`, `communique`),
and **two platforms** (`github`, `gitlab`).
```

Change the bundled-tool table (around line 107) — delete this row entirely:
```markdown
| `cog` (cocogitto) | `generator: cocogitto` |
```

- [ ] **Step 9: `CLAUDE.md`**

Change (line 29):
```markdown
Four versioning strategies are supported (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), three content generators (`git-cliff`, `communique`, `cocogitto`),
and two platforms (`github`, `gitlab`). See [`docs/specs/`](docs/specs/) for the full
behavioural spec.
```
to:
```markdown
Four versioning strategies are supported (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), two content generators (`git-cliff`, `communique`),
and two platforms (`github`, `gitlab`). See [`docs/specs/`](docs/specs/) for the full
behavioural spec.
```

Change the `//go:embed` tech-stack table row (line 50):
```markdown
| **`//go:embed`**              | Embedded git-cliff / cocogitto defaults + `CHANGELOG.md` (offline fallback for `whatsnew`) |
```
to:
```markdown
| **`//go:embed`**              | Embedded git-cliff defaults + `CHANGELOG.md` (offline fallback for `whatsnew`) |
```

Change the project-layout comment (line 86):
```markdown
   generators/
      gitcliff/                 embedded TOML defaults + user override merge
      communique/               wrapper around `communique generate`
      cocogitto/                4-path config resolution + embedded cog.toml + Tera
```
to:
```markdown
   generators/
      gitcliff/                 embedded TOML defaults + user override merge
      communique/               wrapper around `communique generate`
```

Change the "Bundled external CLIs" section's prose (line 155):
```markdown
heraut invokes `git`, `git-cliff`, `glab`, `gh`, `cog`, and `communique` via the
`port.Runner` abstraction. None of these are bundled with the heraut binary — users
install them separately. `heraut check runtime` verifies they are on `PATH`.
```
to:
```markdown
heraut invokes `git`, `git-cliff`, `glab`, `gh`, and `communique` via the
`port.Runner` abstraction. None of these are bundled with the heraut binary — users
install them separately. `heraut check runtime` verifies they are on `PATH`.
```

- [ ] **Step 10: `docs/specs/README.md`**

Change (line 9):
```markdown
| [05 — Generators and Platforms](05-generators-and-platforms.md) | git-cliff, communique, cocogitto + GitHub, GitLab |
```
to:
```markdown
| [05 — Generators and Platforms](05-generators-and-platforms.md) | git-cliff, communique + GitHub, GitLab |
```

- [ ] **Step 11: `docs/specs/01-overview.md`**

Change (line 62):
```markdown
**Generator** — produces text. `git-cliff`, `communique`, or `cocogitto`. Used
independently for `changelog` (writes a `CHANGELOG.md` to the repo) and `release.notes`
(text attached to the platform release). See
[Spec 05 — Generators and Platforms](05-generators-and-platforms.md).
```
to:
```markdown
**Generator** — produces text. `git-cliff` or `communique`. Used
independently for `changelog` (writes a `CHANGELOG.md` to the repo) and `release.notes`
(text attached to the platform release). See
[Spec 05 — Generators and Platforms](05-generators-and-platforms.md).
```

Change (line 101):
```markdown
- Three content generators: git-cliff, communique, cocogitto
```
to:
```markdown
- Two content generators: git-cliff, communique
```

- [ ] **Step 12: `docs/specs/02-configuration.md`**

Change (line 390, the `tickets` field description):
```markdown
A top-level list that links issue-tracker references found in commit messages — **subject,
body, or footer** (e.g. `Refs: PROJ-123`) — in both the changelog and the release notes.
**git-cliff only**; setting `tickets` with a `cocogitto`/`communique` generator is a
configuration error (ADR-0024).
```
to:
```markdown
A top-level list that links issue-tracker references found in commit messages — **subject,
body, or footer** (e.g. `Refs: PROJ-123`) — in both the changelog and the release notes.
**git-cliff only**; setting `tickets` with the `communique` generator is a
configuration error (ADR-0024).
```

Change the content-generators field-reference table (lines 436-442):
```markdown
| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `generator`   | Yes      | One of: `git-cliff`, `communique`, `cocogitto`.                                                                                                                                                                                                                                   |
| `config`      | No       | Path to the generator config file (relative to project root). For `git-cliff`: optional partial override, deep-merged with heraut's built-in default. For `communique`: required. For `cocogitto`: optional path to `cog.toml`.                                                  |
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`). For `cocogitto`, heraut captures stdout and writes this file (cog itself has no `--output` flag).                                                                                                                                          |
| `tag_pattern` | No       | Tag pattern regex for `git-cliff` only. **For per-env strategies heraut auto-derives this from the effective `tag_format` so `--env <env>` only considers that environment's tags** (e.g. `{version}_{env}` + `--env prod` → `^.+_prod$`); set it explicitly to override the derivation. Setting `tag_pattern` with `communique` or `cocogitto` is a config validation error. |
| `template`    | No       | Path to a custom Tera template for `cocogitto` (passed as `-t`). Not used by `git-cliff` or `communique`.                                                                                                                                                                          |
```
to:
```markdown
| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `generator`   | Yes      | One of: `git-cliff`, `communique`.                                                                                                                                                                                                                                                 |
| `config`      | No       | Path to the generator config file (relative to project root). For `git-cliff`: optional partial override, deep-merged with heraut's built-in default. For `communique`: required.                                                                                              |
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`).                                                                                                                                                                                                                                            |
| `tag_pattern` | No       | Tag pattern regex for `git-cliff` only. **For per-env strategies heraut auto-derives this from the effective `tag_format` so `--env <env>` only considers that environment's tags** (e.g. `{version}_{env}` + `--env prod` → `^.+_prod$`); set it explicitly to override the derivation. Setting `tag_pattern` with `communique` is a config validation error. |
| `template`    | No       | Path to a custom Tera template. Not used by `git-cliff` or `communique` (vestigial field with no current consumer; kept for forward compatibility).                                                                                                                              |
```

- [ ] **Step 13: `docs/specs/05-generators-and-platforms.md`**

Change the generator count and list (line 9):
```markdown
Three generators are supported: `git-cliff`, `communique`, `cocogitto`. A project can
use different generators for `changelog` and `release.notes`.
```
to:
```markdown
Two generators are supported: `git-cliff`, `communique`. A project can
use different generators for `changelog` and `release.notes`.
```

Change the comparison table — delete the `cocogitto` row entirely:
```markdown
| Generator   | Strengths                                                        | Limits                                                       |
|-------------|------------------------------------------------------------------|--------------------------------------------------------------|
| `git-cliff` | Embedded opinionated default; deep-merged TOML overrides; labels new commits with `--tag <version>` | TOML config only                                            |
| `communique`| AI-assisted release notes from commit history                    | Requires a full config file; no embedded default              |
| `cocogitto` | Native conventional-commit grouping; rich Tera templating         | Cannot label unreleased commits with a target version        |
```
to:
```markdown
| Generator   | Strengths                                                        | Limits                                                       |
|-------------|------------------------------------------------------------------|--------------------------------------------------------------|
| `git-cliff` | Embedded opinionated default; deep-merged TOML overrides; labels new commits with `--tag <version>` | TOML config only                                            |
| `communique`| AI-assisted release notes from commit history                    | Requires a full config file; no embedded default              |
```

Change the per-platform-links cross-reference (line 144):
```markdown
multiple platforms should use `git-cliff` or `cocogitto`
(see [ADR-0021](../adr/0021-per-platform-release-notes.md)).
```
to:
```markdown
multiple platforms should use `git-cliff`
(see [ADR-0021](../adr/0021-per-platform-release-notes.md)).
```

Delete the entire `### cocogitto` subsection — every line from the `### cocogitto` heading through the blank line immediately before `### No generator`:
```markdown
### cocogitto

```yaml
release:
  notes:
    generator: cocogitto
    config: cog.toml        # optional
    output: CHANGELOG.md    # optional (changelog mode only; written by heraut from stdout)
    template: my.tera       # optional custom Tera template
```

**Four config-path combinations**:

| `config:`  | `template:` | Effective behaviour                                                                          |
|------------|-------------|----------------------------------------------------------------------------------------------|
| _(none)_   | _(none)_    | embedded `cog.toml` + embedded Tera template (full opinionated defaults)                     |
| _(none)_   | `my.tera`   | embedded `cog.toml` + user's Tera template                                                   |
| `cog.toml` | _(none)_    | user's `cog.toml`, no `-t` flag (cog uses the template referenced in `cog.toml` or its own default) |
| `cog.toml` | `my.tera`   | user's `cog.toml` + user's Tera template                                                     |

The embedded `cog.toml` sets `tag_prefix = "v"`, `from_latest_tag = false`,
`ignore_merge_commits = true`, and uses cog's top-level `[commit_types]` table to give the
kept types emoji titles matching the git-cliff generator (🚀 Features, 🐛 Bug Fixes,
🚜 Refactor, 📚 Documentation, ⚡ Performance) while omitting chore/ci/build/test/style via
`omit_from_changelog`. The embedded Tera templates render grouped entries with the scope,
a `**[BREAKING]**` marker, a commit link (when heraut supplies the platform context — see
above), and the author (`commit.signature`).

> **cog schema note:** type titling/omission uses cog's top-level `[commit_types]` table —
> **not** a git-cliff-style commit-parser array under `[changelog]`, which cog rejects
> ("unknown field"). Grouping itself is cog's built-in commit-type mapping.

**Parity limits vs git-cliff** (cog exposes no such template context, so these are *not*
available with the `cocogitto` generator, by design): PR/MR links, a "New Contributors"
section, and the commit-statistics block. Teams that need those should use `git-cliff`.

**Invocation**:

- Changelog mode (full history): `cog [--config <path>] changelog [-t <template.tera>]`
- Release notes mode (single release): `cog [--config <path>] changelog [-t <template.tera>] --at <tag>`

`--config` is a **global** flag for the `cog` binary (must precede the subcommand);
`-t` is a `changelog` subcommand flag.

cocogitto always writes to stdout; there is no `--output` flag. When `output:` is set,
heraut captures stdout and writes the file itself.

**Differences from git-cliff**:

| Aspect                                  | git-cliff                                      | cocogitto                                  |
|------------------------------------------|------------------------------------------------|--------------------------------------------|
| Output file                             | `--output` flag (written by git-cliff)         | stdout redirect (written by heraut)        |
| Embedded config                         | TOML partial-override, deep-merged at runtime  | TOML + Tera, written as temp files         |
| Version label for unreleased commits    | `--tag <version>`                              | not supported                              |
| Tag pattern                             | `--tag-pattern <regex>`                        | not supported                              |

**Known limitation — changelog mode**: heraut creates the git tag *after* committing the
changelog, so when cocogitto generates the full `CHANGELOG.md`, the new version's tag
does not yet exist in the repository. As a result, the new commits appear under an
"Unreleased" section rather than under the version heading. Teams that require a
correctly versioned heading in `CHANGELOG.md` should use `git-cliff`, which supports
`--tag <version>` to label unreleased commits with the target version.

**`tag_pattern:` field**: not used by cocogitto. Setting `tag_pattern` when
`generator: cocogitto` has no effect. For prefixed tag strategies, heraut passes the
exact resolved tag via `--at` in release notes mode. Full changelog mode scans all tags.

```
(the trailing blank line immediately before `### No generator`)

Change the per-platform-link-consumption note (line 239):
```markdown
`git-cliff` and `cocogitto` consume this context. A single-platform release passes
`nil`, and the generators fall through to ambient-CI link detection — today's unchanged
behaviour. **communique does not consume the context** (see its section above).
```
to:
```markdown
`git-cliff` consumes this context. A single-platform release passes
`nil`, and the generator falls through to ambient-CI link detection — today's unchanged
behaviour. **communique does not consume the context** (see its section above).
```

Change the generator/platform combination example (line 362):
```markdown
heraut does not constrain combinations. Any generator can produce text for any
platform. A common pattern: `cocogitto` for `release.notes` (rich Tera template for
the release page) and `git-cliff` for `changelog` (versioned `CHANGELOG.md` in the
repo, thanks to `--tag <version>`).
```
to:
```markdown
heraut does not constrain combinations. Any generator can produce text for any
platform. A common pattern: `communique` for `release.notes` (AI-assisted summary for
the release page) and `git-cliff` for `changelog` (versioned `CHANGELOG.md` in the
repo, thanks to `--tag <version>`).
```

- [ ] **Step 14: `docs/specs/06-dx-and-testing.md`**

Change (line 148, in the "hard-won edge cases" list):
```markdown
- The four cocogitto config × template combinations
- The git-cliff temp config file lifecycle (cleanup on early return)
```
to:
```markdown
- The git-cliff temp config file lifecycle (cleanup on early return)
```

(This is a deliberate row removal, justified the same way as the test-row removals in Task 2: the behavior this line documented — cocogitto's four config×template combinations — no longer exists, and ADR-0028 is the ADR documenting why. This is the one line `testing.md`'s "preserve hard-won edge cases" rule explicitly anticipates removing under exactly this condition.)

- [ ] **Step 15: `.claude/rules/coding.md`**

Change (line 19, the architecture diagram):
```markdown
   ├──→  internal/generators/   (gitcliff, communique, cocogitto)  ── implement port.Generator
```
to:
```markdown
   ├──→  internal/generators/   (gitcliff, communique)  ── implement port.Generator
```

Change (line 87, the embedded-assets note):
```markdown
- Default git-cliff TOMLs, cocogitto `cog.toml`, and Tera templates are embedded via
  `//go:embed` directives in their owning packages
  (`internal/generators/gitcliff/`, `internal/generators/cocogitto/`).
```
to:
```markdown
- Default git-cliff TOMLs are embedded via a `//go:embed` directive in
  `internal/generators/gitcliff/`.
```

- [ ] **Step 16: `.claude/rules/testing.md`**

Change (lines 56-60, the real-CLI smoke-test exception — this now has only one example, git-cliff's, not two):
```markdown
A narrow, deliberate exception to "mock the externals": a few **skippable** tests run the
*real* `git-cliff` / `cog` against heraut's **embedded default configs** (via
`testutil.RealGitRepo`), asserting the tool *accepts* the config. MockRunner can't catch an
embedded TOML the real tool rejects — that gap shipped a broken cocogitto default once (the
embedded `cog.toml` used a field cog rejects). These tests `t.Skip` when the binary is
absent and run in CI, where `mise` installs the pinned tools. Keep them to
config-acceptance smoke checks (no output assertions — those stay byte-level / manual);
they are local and deterministic (no network, `t.TempDir`).
```
to:
```markdown
A narrow, deliberate exception to "mock the externals": a **skippable** test runs the
*real* `git-cliff` against heraut's **embedded default config** (via
`testutil.RealGitRepo`), asserting the tool *accepts* the config. MockRunner can't catch an
embedded TOML the real tool rejects — that gap once shipped a broken default for a
generator heraut has since dropped (T117/ADR-0028). This test `t.Skip`s when the binary is
absent and runs in CI, where `mise` installs the pinned tool. Keep it to a
config-acceptance smoke check (no output assertions — those stay byte-level / manual);
it is local and deterministic (no network, `t.TempDir`).
```

Change (line 88, the "preserve hard-won edge cases" list):
```markdown
The test suite contains hard-won edge cases — `v1.9.0` → `v1.10.0` (not `v1.100.0`),
CalVer `PATCH` reset on period boundary, per-env cycle detection, the 4 cocogitto
config-path combinations, and more.
```
to:
```markdown
The test suite contains hard-won edge cases — `v1.9.0` → `v1.10.0` (not `v1.100.0`),
CalVer `PATCH` reset on period boundary, per-env cycle detection, and more.
```

- [ ] **Step 17: Repo-wide verification sweep**

Run: `grep -rln -i "cocogitto" --include="*.go" --include="*.md" --include="*.json" --include="*.toml" --include="*.yml" --include="Dockerfile" . | grep -v "^\./CHANGELOG.md$" | grep -v "^\./docs/tasks/roadmap.md$" | grep -v "^\./docs/adr/00[0-2][0-9]-" | grep -v "^\./.claude/plans/"`

Expected: no output, **except** historical files this task deliberately does not touch:
- `CHANGELOG.md` (auto-generated historical record, never hand-edited)
- `docs/tasks/roadmap.md` (historical task log — T117's own entry will be closed out by Task 5, but old task entries like T15/T65-T77/T116 documenting cocogitto's original build-out stay as-is)
- `docs/adr/0004-*.md`, `0006-*.md`, `0011-*.md`, `0012-*.md`, `0020-*.md`, `0021-*.md`, `0022-*.md`, `0023-*.md`, `0024-*.md`, `0027-*.md`, `0028-*.md` (historical ADRs that mention cocogitto in passing as an accurate record of decisions made at the time — per ADR-0028's "What does not change" clause, only ADR-0016 is amended, already done in Step 7)
- `.claude/plans/` (past planning artifacts — historical, not live source)

If the grep surfaces anything **not** in this list, it was missed by an earlier step in this task — find and fix it before committing.

- [ ] **Step 18: Run the full build/lint/test gate**

Run: `go build ./... && go test ./... && mise run lint:check`
Expected: clean build, all tests pass, all linters pass (this exercises `hadolint` on the `Dockerfile`, `tombi`/JSON-schema checks if configured, and confirms nothing in this doc-only task broke any tooling).

- [ ] **Step 19: Commit**

```bash
git add schema.json docs/heraut.sample.yml Dockerfile .config/mise/config.toml .config/mise/mise.lock .github/renovate.json docs/adr/0016-bundled-docker-image.md README.md CLAUDE.md docs/specs/README.md docs/specs/01-overview.md docs/specs/02-configuration.md docs/specs/05-generators-and-platforms.md docs/specs/06-dx-and-testing.md .claude/rules/coding.md .claude/rules/testing.md
git commit -m "docs: drop cocogitto from config schema and documentation (ADR-0028)"
```

---

### Task 5: Final gate and roadmap closure

**Files:**
- Modify: `docs/tasks/roadmap.md` (flip T117's checkbox, add closing note)

**Interfaces:** None — this task verifies the assembled whole and closes the roadmap entry, mirroring the precedent set by T116's and T118's own closing tasks.

- [ ] **Step 1: Run the full gate**

Run: `go build ./... && go test ./... && mise run lint:check`
Expected: clean build, all tests pass, all linters pass. If this fails, STOP — do not flip the roadmap checkbox or commit. Diagnose and fix the failure (it means an earlier task left something broken that its own task-scoped gate didn't catch, e.g. an interaction between Task 4's doc edits and something else).

- [ ] **Step 2: Confirm no remaining references via a final, broader sweep**

Run: `grep -rln -i "cocogitto" --include="*.go" .`
Expected: no output at all (zero Go files anywhere reference cocogitto — this is stricter than Task 4's Step 17 sweep, which excluded doc-file categories; this one is Go-only and has no historical-file exceptions, since no `.go` file should ever mention a removed feature).

- [ ] **Step 3: Flip the roadmap checkbox and add the closing note**

In `docs/tasks/roadmap.md`, change:
```markdown
#### `[ ]` T117: drop the `cocogitto` generator entirely
```
to:
```markdown
#### `[x]` T117: drop the `cocogitto` generator entirely
```

Then append a closing paragraph after the existing `**Scope:** M. **Dependencies:** T116 ...` line, describing the actual implementation (write this once the prior 4 tasks' real commit SHAs and any deviations are known — do not write a generic "implemented exactly as planned" placeholder if anything differed; name the actual deviations, e.g. if `internal/scaffold/`'s wizard fix was folded in as part of this task per the user's explicit approval before this plan was written, say so explicitly since it's not in ADR-0028's original file list).

- [ ] **Step 4: Commit**

```bash
git add docs/tasks/roadmap.md
git commit -m "docs(roadmap): mark T117 complete"
```
