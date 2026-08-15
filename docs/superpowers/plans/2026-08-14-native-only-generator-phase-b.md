# Native-Only Generator — Phase B Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the `git-cliff` and `communique` generator packages and every command/config
surface that depends on them, so `native` is heraut's sole, implicit content generator — no
dispatch, no dead code paths, no docs describing generators that no longer exist.

**Architecture:** Work bottom-up through the dependency graph so every intermediate commit
compiles and tests green: first collapse the generator-dispatch logic in `internal/app/pipeline.go`
down to a direct `native.New(...)` call, then delete the two commands (`heraut cliff`, `heraut
check cliff`) that are the last callers of git-cliff-specific `internal/app` functions, then delete
the `internal/generators/gitcliff` and `internal/generators/communique` packages themselves (now
unreferenced) along with the `ContentDriver.Generator`/`.Config` struct fields, then update docs
(README, specs 02/05/06, roadmap, CLAUDE.md, testing.md) and author ADR-0045.

**Tech Stack:** Go, cobra, yaml.v3, testify (assert/require), `forge/exec/exectest` (MockRunner/FakeBin).

**Spec:** `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` (sections 1-3 and
the ADR-0045 outline — §4/§5 are Phase C/D, out of scope here). Roadmap: `docs/tasks/native-generator-roadmap.md`,
Phase 2.5 section, "Phase B scope" note (~line 1069).

## Global Constraints

- TDD: write the failing test before the implementation change, per `.claude/rules/testing.md`.
- Commits land directly on `main` (pre-v1.0, no branches), per `.claude/rules/workflow.md`.
- Every commit must leave `go build ./...`, `go test ./...`, and `hk check` clean — no phase lands
  with a broken build even temporarily.
- Conventional commit type/scope per `.claude/rules/workflow.md` (e.g. `refactor(app)`, `docs(specs)`).
- After each task: flip its `[ ]` → `[x]` in `docs/tasks/native-generator-roadmap.md` (new task IDs
  T185-T194, continuing the sequence from T177-T184) and add a one-paragraph completion note, per
  the two-step roadmap flow. Commit the roadmap update alongside the task's implementation commit.
- Never delete a test row that covers a still-relevant *behavior* (testing.md's hard-won-edge-case
  rule) — only delete a test when its own *scenario* becomes structurally impossible (e.g. a struct
  literal setting a field that no longer exists) or when ADR-0045 documents the behavior itself as
  deliberately removed.
- `internal/scaffold/` (the wizard) is explicitly OUT OF SCOPE for every task in this plan — it
  stays deferred to Phase C. Do not edit `internal/scaffold/wizard.go`, `generate.go`, `cliff.go`,
  or `dropped.go` in this plan, even where a comment references git-cliff/communique.

---

### Task 185: Collapse `buildGenerator` to native-only; delete `usesNative`

**Files:**
- Modify: `internal/app/pipeline.go:470-510` (`buildGenerator`, `usesNative`), `:234` and `:537`
  (call sites), `:241/253/371` (mode plumbing), `:598-603` (`nativeMode`, deleted)
- Modify: `internal/app/pipeline_test.go` (delete/adapt git-cliff/communique-specific tests)
- Modify: `internal/app/forge_internal_test.go` (delete `TestUsesNative`, adapt `TestResolveEnrichForgeIfNeeded`)
- Create: `internal/app/tagglob_internal_test.go` additions (TagPattern coverage, alongside existing TagGlob coverage)

**Interfaces:**
- Consumes: `internal/generators/native.New(runner port.Runner, cfg *config.ContentDriver, mode native.Mode, opts ...native.Option) *native.Generator`, `native.ModeChangelog`/`native.ModeReleaseNotes` (`internal/generators/native/generator.go:15-19,43`).
- Produces: `buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode native.Mode, herautVersion string, regenerateChangelog, force bool, enrichForge port.Forge, degradedReason string) port.Generator` — **signature changes**: drops the `error` return and changes `defaultMode`'s type from `gitcliff.Mode` to `native.Mode`. `usesNative` is deleted entirely (no replacement — callers change instead, see below).

This task does **not** delete `ContentDriver.Generator`/`.Config` — those fields still exist after
this task (Task 188 deletes them once every non-test consumer is gone). Test literals in this task
that set `Generator: "..."` should still have that field stripped from the literal *when the whole
literal is being touched anyway* for another reason (a deleted or adapted test) — but do not make a
pass over untouched literals just to strip the field; Task 188 sweeps whatever remains.

#### Background: why `buildGenerator` can drop its `error` return

`internal/config/loader.go`'s `checkRemovedKeys` (built by T177) makes `generator: <anything>` a
hard parse-time error — `config.Load` never returns a `*config.Config` with a non-empty
`ContentDriver.Generator`. The only way `buildGenerator` today reaches its `default:` case
(`"unsupported generator %q"`) is a caller constructing a `*config.Config` directly in Go, bypassing
`config.Load` — which today's tests exploit on purpose (`TestBuildPipeline_UnknownGenerator`,
`TestBuildPipeline_CocogittoNoLongerSupported`, `TestBuildPipeline_UnknownNotesGenerator`,
`TestBuildChangelogPipeline_UnknownGenerator`). Once the `git-cliff`/`communique` cases are deleted
from the switch, there is no longer a meaningful "unsupported generator" case to guard — every
driver becomes native unconditionally, so these four tests' scenario (a non-empty `Generator` value
reaching `buildGenerator`) has nothing left to assert against and must be deleted, not adapted.

- [ ] **Step 1: Write the failing test — `buildGenerator` ignores any pre-existing `Generator` value and always builds native**

Add to `internal/app/pipeline_test.go` (this replaces the assertions that
`TestBuildPipeline_WithGitcliff` and `TestBuildPipeline_WithCommunique` used to make — those two
tests are deleted in Step 3, this new test is their replacement, proving the collapsed behavior
directly):

```go
func TestBuildPipeline_ChangelogAlwaysBuildsNative(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}
```

(This is nearly identical to the existing `TestBuildPipeline_EmptyGeneratorBuildsNative` — keep
both for now; Step 3 reconciles the duplication by deleting `TestBuildPipeline_EmptyGeneratorBuildsNative`'s
now-redundant T179-era comment and renaming it, see Step 3.)

- [ ] **Step 2: Run test to verify it currently passes (this is a characterization test, not a red step — the collapse is a refactor of already-passing behavior)**

Run: `go test ./internal/app/... -run TestBuildPipeline_ChangelogAlwaysBuildsNative -v`
Expected: PASS (buildGenerator already builds native for an empty `Generator`; this step just
confirms the harness before refactoring).

- [ ] **Step 3: Collapse `buildGenerator`, delete `usesNative`, update the 3 call sites**

In `internal/app/pipeline.go`, replace the `buildGenerator` function (currently lines 470-496):

```go
func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode native.Mode, herautVersion string, regenerateChangelog, force bool, enrichForge port.Forge, degradedReason string) port.Generator {
	// Copy so setting the running version never mutates the shared config.
	nativeDriver := *driver
	nativeDriver.HerautVersion = herautVersion
	nativeDriver.RegenerateChangelog = regenerateChangelog
	nativeDriver.Force = force
	var opts []native.Option
	if enrichForge != nil {
		opts = append(opts, native.WithForge(enrichForge))
	}
	if degradedReason != "" {
		opts = append(opts, native.WithDegraded(degradedReason))
	}
	return native.New(runner, &nativeDriver, defaultMode, opts...)
}
```

Delete `usesNative` (currently lines 498-510) entirely — no replacement function.

Delete `nativeMode` (currently lines 597-603) entirely — no replacement function; callers now pass
`native.ModeChangelog`/`native.ModeReleaseNotes` directly instead of `gitcliff.ModeChangelog`/
`gitcliff.ModeReleaseNotes` converted via `nativeMode`.

Update the 3 `buildGenerator` call sites:

`buildReleasePipelineConfig` (currently lines 239-258): change both calls from
`gen, err := buildGenerator(runner, driver, gitcliff.ModeChangelog, ...)` /
`gitcliff.ModeReleaseNotes` to drop the `err`:

```go
	// Changelog generator
	if effectiveChangelog != nil {
		driver := withEnvDerivations(effectiveChangelog, cfg, env)
		gen := buildGenerator(runner, driver, native.ModeChangelog, herautVersion, regenerateChangelog, force, enrichForge, "")
		pCfg.Changelog = gen
		pCfg.ChangelogFile = effectiveChangelog.Output
		pCfg.ForgeIdentity = forgeID
	}

	// Release notes generator
	if effectiveNotes != nil {
		driver := withEnvDerivations(effectiveNotes, cfg, env)
		gen := buildGenerator(runner, driver, native.ModeReleaseNotes, herautVersion, regenerateChangelog, force, enrichForge, "")
		pCfg.Notes = gen
	}
```

`buildChangelogPipelineConfig` (currently lines 371-374): same pattern, drop the `err`:

```go
		driver := withEnvDerivations(effectiveChangelog, cfg, opts.Env)
		gen := buildGenerator(runner, driver, native.ModeChangelog, opts.HerautVersion, opts.RegenerateChangelog, opts.Force, enrichForge, degradedReason)
		cCfg.Changelog = gen
		cCfg.ChangelogFile = effectiveChangelog.Output
		cCfg.ForgeIdentity = forgeID
```

(Both call sites currently wrap a `fmt.Errorf("changelog generator: %w", err)` / `"release notes
generator: %w"` around the old error — delete those wrapping blocks since `buildGenerator` no
longer returns an error. Verify no other error path in these two functions still needs the `err`
variable name shadowed correctly — read the surrounding function after editing to confirm it still
compiles.)

Update line 234's enrichment gate — delete the `usesNative(...)` conjunct entirely, since every
driver is native now and the emptiness of `usesNative` was the only thing it gated:

```go
	var enrichForge port.Forge
	var forgeID *port.ForgeIdentity
	if cfg.EnrichmentPolicy() != "disabled" {
		enrichForge, forgeID = enrichForgeFrom(resolved)
	}
```

Update `resolveEnrichForgeIfNeeded` (currently lines 536-553) — delete the `usesNative` guard
(currently lines 537-539):

```go
func resolveEnrichForgeIfNeeded(runner port.Runner, getenv func(string) string, cfg *config.Config, force bool, drivers ...*config.ContentDriver) (port.Forge, *port.ForgeIdentity, string, error) {
	policy := cfg.EnrichmentPolicy()
	if policy == "disabled" {
		return nil, nil, "", nil
	}
	resolved, err := resolveForge(runner, getenv, cfg)
	if err != nil {
		if policy == "required" && !force {
			return nil, nil, "", err
		}
		return nil, nil, fmt.Sprintf("remote enrichment unavailable; rendering without PR attribution: %v", err), nil
	}
	enrichForge, forgeID := enrichForgeFrom(resolved)
	return enrichForge, forgeID, "", nil
}
```

Note this function's `drivers ...*config.ContentDriver` parameter is now unused inside the body —
its only remaining job was feeding `usesNative`. Leave the parameter in the signature for this task
(its caller in `buildChangelogPipelineConfig` still passes `effectiveChangelog, nil` positionally)
rather than changing the public call shape; do not introduce an unused-parameter lint suppression —
Go does not flag unused function *parameters*, only unused local variables, so this compiles clean
with no lint noise.

Update the doc comment above `resolveEnrichForgeIfNeeded` (currently lines 512-535) — its first
paragraph explains *why* the function used to gate on `usesNative` ("but only when a native
generator is actually in play... Returns (nil, nil, "", nil) when no driver is native"), which is
no longer true (there is no more gate). Every other paragraph is unaffected by this task and stays
exactly as-is. Replace the whole comment with:

```go
// resolveEnrichForgeIfNeeded resolves the configured/ambient forge and constructs the matching
// port.Forge. forge.Resolve shells out to `git remote get-url origin`.
//
// When the effective enrichment policy is "disabled" (including via --offline, which forces it),
// resolution is skipped entirely rather than attempted and its error discarded: enrichment being
// switched off must never be able to *cause* a failure, e.g. an ambiguous multi-token environment
// that forge.Resolve can't disambiguate should not block an explicitly offline run.
//
// A resolution failure under any other policy is fatal only when the policy is "required" and not
// downgraded by force — matching enrichForRelease's "required fails outright" contract
// (internal/generators/native/enrich.go). Under the default/optional policy, which promises "on
// failure, degrade", a resolution failure degrades the same way a post-resolution fetch failure
// does: the returned forge/identity are nil and the third return value carries a non-empty reason
// for the caller to seed onto the generator (native.WithDegraded), instead of failing the whole
// pipeline (T175 — without this, heraut check's warn-only severity for an unconfigured, ambiguous
// changelog-only environment (T172) predicted success while heraut changelog hard-failed on the
// identical resolution error).
//
// getenv is injected rather than reaching for os.Getenv directly: forge.Resolve keys off CI
// markers (GITHUB_ACTIONS, GITLAB_CI, TF_BUILD), so a hardcoded os.Getenv would let the ambient
// CI environment of heraut's *own* pipeline decide what a test resolves — which is exactly how
// this function's tests broke on GitHub Actions while passing locally.
func resolveEnrichForgeIfNeeded(runner port.Runner, getenv func(string) string, cfg *config.Config, force bool, drivers ...*config.ContentDriver) (port.Forge, *port.ForgeIdentity, string, error) {
```

Fix the two remaining `gitcliff`/`communique` imports at the top of `internal/app/pipeline.go`
(currently lines 17-19): delete the `"github.com/adaouat/heraut/internal/generators/communique"`
and `"github.com/adaouat/heraut/internal/generators/gitcliff"` import lines — `native` is already
imported and is now the only generator package this file references.

- [ ] **Step 4: Delete the 5 tests whose scenario becomes structurally impossible**

Delete these test functions from `internal/app/pipeline_test.go` in full (each constructs a
`Generator: "..."` value and asserts `buildGenerator`'s now-deleted error path):
- `TestBuildPipeline_WithGitcliff` (lines 37-44)
- `TestBuildPipeline_WithCommunique` (lines 46-53)
- `TestBuildPipeline_CocogittoNoLongerSupported` (lines 67-74)
- `TestBuildPipeline_UnknownGenerator` (lines 76-83)
- `TestBuildPipeline_UnknownNotesGenerator` (lines 135-144)
- `TestBuildChangelogPipeline_WithGitcliff` (lines 177-185)
- `TestBuildChangelogPipeline_UnknownGenerator` (lines 187-195)
- `TestBuildChangelogPipeline_PerEnvPartialOverrideMerges` (lines 297-320) — this test's entire
  premise is a per-env override supplying only `Config: "cliff.prod.toml"` and inheriting the
  top-level `Generator: "git-cliff"` via `config.MergeContentDriver` (ADR-0019's per-env merge).
  Once `Generator`/`.Config` are gone (Task 188), there is nothing left to inherit — this is a
  deliberate behavior removal documented by ADR-0045 (Task 194), not a shortcut. Delete it here
  rather than leaving it broken until Task 188.

Rename `TestBuildPipeline_EmptyGeneratorBuildsNative` (lines 58-65) to
`TestBuildPipeline_ChangelogBuildsNative` and drop its stale "T179"/"once generator: is a removed
key" comment (that rationale is now the *only* path, not a special case worth calling out):

```go
func TestBuildPipeline_ChangelogBuildsNative(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Changelog = &config.ContentDriver{Output: "CHANGELOG.md"}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}
```

(This makes Step 1's new `TestBuildPipeline_ChangelogAlwaysBuildsNative` a duplicate — delete the
one added in Step 1 now that this rename covers the same ground under a clearer name.)

Adapt `TestBuildPipeline_WithNotes` (line 124-133) — strip the now-pointless `Generator: "git-cliff"`
field from its literal (the test's actual point is "a `release.notes` block builds successfully",
which holds regardless):

```go
func TestBuildPipeline_WithNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := semverCfg()
	cfg.Release = &config.Release{
		Notes: &config.ContentDriver{},
	}
	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
	require.NoError(t, err)
	assert.NotNil(t, p)
}
```

- [ ] **Step 5: Rewrite the two TagPattern/TagGlob pipeline tests as direct `withEnvDerivations` unit tests**

`TestBuildChangelogPipeline_PerEnvDerivesTagPattern` (lines 218-256) and
`TestBuildChangelogPipeline_ExplicitTagPatternWins` (lines 258-295) currently prove their behavior
by asserting on a `git-cliff` subprocess's `--tag-pattern` CLI argument — a mechanism that no longer
exists once native (which never shells out for its own generation) is the only generator. The
*behavior* they cover — `withEnvDerivations` auto-derives `TagPattern` for a per-env tag format
unless the user set one explicitly — is real and not covered anywhere else:
`internal/app/tagglob_internal_test.go` already unit-tests `withEnvDerivations`'s **TagGlob**
derivation directly (`TestWithEnvDerivations_SetsTagGlobForPerEnv`), but nothing today covers its
**TagPattern** derivation or the "explicit value wins" precedence. Delete both tests from
`internal/app/pipeline_test.go` and add their replacements to
`internal/app/tagglob_internal_test.go`, matching that file's existing direct-unit-test style
(package `app`, no MockRunner, call `withEnvDerivations` directly):

```go
// TestWithEnvDerivations_DerivesTagPatternForPerEnv verifies the app layer derives a regex
// TagPattern from the effective tag_format for a per-env strategy, mirroring the TagGlob
// derivation above but for the regex form some generators/tests scope by.
func TestWithEnvDerivations_DerivesTagPatternForPerEnv(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "calver-per-env", TagFormat: "{version}_{env}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "prod")
	assert.Equal(t, "^.+_prod$", got.TagPattern)
	assert.Empty(t, driver.TagPattern, "the original driver is never mutated")
}

// TestWithEnvDerivations_ExplicitTagPatternWins verifies a user-set TagPattern is never
// overridden by the per-env auto-derivation.
func TestWithEnvDerivations_ExplicitTagPatternWins(t *testing.T) {
	driver := &config.ContentDriver{TagPattern: "custom-pattern"}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: "{version}_{env}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "prod")
	assert.Equal(t, "custom-pattern", got.TagPattern, "explicit user tag_pattern must win over derivation")
}
```

- [ ] **Step 6: Delete `TestUsesNative` from `internal/app/forge_internal_test.go` (lines 14-32) — the function it tests no longer exists**

- [ ] **Step 7: Adapt `TestResolveEnrichForgeIfNeeded` in `internal/app/forge_internal_test.go`**

Delete the first subtest, `"skips resolution when no driver is native"` (lines 54-63) — it exercises
the exact `usesNative`-gate behavior deleted in Step 3; there is no more "no driver is native" case.

In every remaining subtest (lines 65-189), strip `Generator: "native"` from each
`&config.ContentDriver{Generator: "native"}` literal — the field no longer affects behavior (every
driver is native now) and these subtests never asserted on it, only used it as boilerplate. Example
(apply the same edit to all 8 remaining subtests' driver literals):

```go
f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, false, &config.ContentDriver{})
```

- [ ] **Step 8: Run the full `internal/app` package test suite**

Run: `go test ./internal/app/... -v`
Expected: PASS, zero references to `gitcliff`/`communique`/`usesNative` remain in non-test code
under `internal/app/pipeline.go` (verify with `grep -n "gitcliff\|communique\|usesNative"
internal/app/pipeline.go` — the only hits should be inside `internal/app/cliff.go` and
`internal/app/check.go`, both deleted/edited in later tasks).

- [ ] **Step 9: Run the full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS. (`internal/app/cliff.go` and `internal/app/check.go` still import `gitcliff` at
this point — that's expected and fine, they're untouched until Tasks 186/187.)

- [ ] **Step 10: Commit**

```bash
git add internal/app/pipeline.go internal/app/pipeline_test.go internal/app/forge_internal_test.go internal/app/tagglob_internal_test.go
git commit -m "refactor(app): collapse buildGenerator to native-only, delete usesNative"
```

---

### Task 186: Delete the `heraut cliff` command

**Files:**
- Delete: `internal/cmd/cliff.go`
- Delete: `internal/app/cliff.go`
- Modify: `internal/cmd/root.go:31` (remove `root.AddCommand(NewCliffCmd())`)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — this is a pure deletion. `internal/app/cliff.go`'s
  `EffectiveCliffConfig` (the only exported symbol) has no other callers (verify in Step 1).

- [ ] **Step 1: Verify `EffectiveCliffConfig` has no callers outside `internal/cmd/cliff.go`**

Run: `grep -rn "EffectiveCliffConfig" --include="*.go" .`
Expected: only `internal/app/cliff.go` (definition) and `internal/cmd/cliff.go` (the two call sites
in `newCliffChangelogCmd`/`newCliffReleaseNotesCmd`). If any other caller appears, stop and
re-scope this task — the design doc did not anticipate one.

- [ ] **Step 2: Write the failing test proving `heraut cliff` no longer exists**

Add to `internal/cmd/root_test.go` (find this file's existing test-helper pattern — likely
`executeRoot(...)` per `internal/cmd/check_test.go`'s usage — and add):

```go
func TestRootCmd_NoCliffCommand(t *testing.T) {
	root := NewRootCmd("dev")
	for _, c := range root.Commands() {
		assert.NotEqual(t, "cliff", c.Name(), "heraut cliff must be removed (Phase B)")
	}
}
```

(If `internal/cmd/root_test.go` does not exist yet, check `internal/cmd/check_test.go` for the
`executeRoot` helper's definition location and add this test to whichever file defines
`TestRootCmd_*`-style tests today — search `grep -rln "func TestRootCmd" internal/cmd/` first.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/cmd/... -run TestRootCmd_NoCliffCommand -v`
Expected: FAIL — `cliff` is still registered.

- [ ] **Step 4: Delete the command and its wiring**

Delete `internal/cmd/cliff.go` and `internal/app/cliff.go` entirely.

In `internal/cmd/root.go`, delete line 31 (`root.AddCommand(NewCliffCmd())`).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cmd/... -run TestRootCmd_NoCliffCommand -v`
Expected: PASS.

- [ ] **Step 6: Check for and delete any now-orphaned tests for the deleted files**

Run: `ls internal/cmd/cliff_test.go internal/app/cliff_test.go 2>&1`
If either exists, delete it (its subject no longer exists to test).

- [ ] **Step 7: Run the full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cmd/root.go internal/cmd/root_test.go
git rm internal/cmd/cliff.go internal/app/cliff.go
git commit -m "feat(cmd): remove the heraut cliff command"
```

---

### Task 187: Delete `heraut check cliff` and the git-cliff/communique runtime probes

**Files:**
- Modify: `internal/cmd/check.go` (delete `newCheckCliffCmd`, `newCheckCliffChangelogCmd`,
  `newCheckCliffReleaseNotesCmd`, `runCliffChecks`, `checkCliffDriver`, the default-output "Cliff"
  section, the `checkCmd.AddCommand(newCheckCliffCmd())` wiring)
- Modify: `internal/app/check.go` (delete `configuredGenerators`, `CheckCliff`, the "Generators"
  section of `RuntimeCheck`; delete the now-unused `gitcliff` import)
- Modify: `internal/cmd/check_test.go` (delete the whole "check cliff" test section, fix the two
  dangling `changelog:` keys, strip git-cliff/communique FakeBin setup from remaining runtime tests)
- Modify: `internal/app/check_test.go` (delete cliff-specific tests, strip git-cliff/communique
  MockRunner queueing from remaining tests)
- Modify: `CLAUDE.md` (the `heraut check` command-table line drops "+ cliff")

**Interfaces:**
- Consumes: nothing new.
- Produces: `RuntimeCheck`'s public signature (`internal/app/check.go`) is unchanged — only its
  internal "Generators" dispatch section is deleted. `heraut check` (bare) and `heraut check
  runtime` keep their existing Git/Platforms sections; the "Generators" header and its two rows
  (`git-cliff`, `communique`) are gone. `heraut check`'s "Cliff" section header and its output line
  are gone entirely — `heraut check` now runs Config + Runtime only.

- [ ] **Step 1: Write the failing test — `heraut check` has no `cliff` subcommand and no Cliff section**

Add to `internal/cmd/check_test.go`, replacing `TestCheckCmd_Structure` (lines 16-40) — this
existing test currently *asserts* `cliff` is present (`assert.True(t, checkSubs["cliff"], "check
cliff missing")`), so it must be rewritten, not just extended:

```go
func TestCheckCmd_Structure(t *testing.T) {
	root := NewCheckCmd()
	var checkSubs = map[string]bool{}
	for _, sub := range root.Commands() {
		checkSubs[sub.Use] = true
	}
	assert.True(t, checkSubs["config"], "check config missing")
	assert.True(t, checkSubs["runtime"], "check runtime missing")
	assert.False(t, checkSubs["cliff"], "check cliff must be removed (Phase B)")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cmd/... -run TestCheckCmd_Structure -v`
Expected: FAIL — `cliff` is still registered.

- [ ] **Step 3: Delete `heraut check cliff` from `internal/cmd/check.go`**

Delete these functions entirely: `newCheckCliffCmd` (lines 162-191), `newCheckCliffChangelogCmd`
(lines 193-213), `newCheckCliffReleaseNotesCmd` (lines 215-239), `runCliffChecks` (lines 270-288),
`checkCliffDriver` (lines 290-312).

Delete line 93: `checkCmd.AddCommand(newCheckCliffCmd())`.

In `NewCheckCmd`'s `RunE` (the bare `heraut check` handler), delete the "Cliff section" block
(currently lines 69-75):

```go
			// Cliff section (best-effort; skip if no git-cliff generators configured)
			ui.Header(out, "Cliff")
			if cfg == nil {
				_, _ = fmt.Fprintln(out, ui.Info(out, "no git-cliff generators configured"))
			} else if f := runCliffChecks(runner, cfg, out); f {
				failed++
			}

```

After deletion, the `RunE` body goes directly from the Runtime section (`failed +=
runRuntimeCheck(...)`) to the Summary block — verify there is no longer a reference to `failed++`
that depended on the deleted Cliff block, and that the function still compiles.

- [ ] **Step 4: Delete `configuredGenerators`, `CheckCliff`, and the "Generators" probe from `internal/app/check.go`**

Delete `configuredGenerators` (lines 231-245) and `CheckCliff` (lines 247-264) entirely.

In `RuntimeCheck` (lines 60-193), delete the "Generators" section (currently lines 172-192):

```go
	// ── Generators ────────────────────────────────────────────────────────────
	header("Generators")

	usedGens := configuredGenerators(cfg)
	for _, og := range []struct{ name, binary, display string }{
		{"git-cliff", "git-cliff", "git-cliff"},
		{"communique", "communique", "communique"},
	} {
		required := usedGens[og.name]
		dispatch(og.display, func() RuntimeCheckItem {
			out, _, err := runner.Run(og.binary, "--version")
			if err != nil {
				if required {
					return RuntimeCheckItem{Name: og.display, Err: fmt.Errorf("%s: not found on PATH", og.binary)}
				}
				return RuntimeCheckItem{Name: og.display, IsWarn: true,
					Err: fmt.Errorf("not found (not required by this config)")}
			}
			return RuntimeCheckItem{Name: og.display, Value: strings.TrimSpace(out)}
		})
	}
}
```

(The closing `}` for `RuntimeCheck` moves up to follow the Platforms section's closing brace —
after this deletion, `RuntimeCheck` ends right after the `default:` case of the Platforms `switch`.)

Update `RuntimeCheck`'s doc comment (lines 38-59) — its "Check order" list currently ends with
`Generators: git-cliff → communique`; delete that line. Its final paragraph ("Configured tools are
hard errors when missing...") still applies to Git/Platforms and stays unchanged.

Delete the now-unused `"github.com/adaouat/heraut/internal/generators/gitcliff"` import at the top
of `internal/app/check.go` (line 9).

- [ ] **Step 5: Delete the "check cliff" test section from `internal/cmd/check_test.go`**

Delete the whole block from the `// ---- check cliff ----` comment through
`TestCheckCliffReleaseNotes_NotConfigured` — i.e. `TestCheckCliff_ConfigNotFound`,
`TestCheckCliff_NoGeneratorsConfigured`, `TestCheckCliffReleaseNotes_ConfigNotFound`,
`TestCheckCliffReleaseNotes_NotConfigured`, and the three section-header comments above them
(`// ---- check cliff ----`, `// ---- check cliff (bare) ----`, `// ---- check cliff release-notes ----`).

- [ ] **Step 6: Fix the two dangling `changelog:` keys and strip git-cliff/communique FakeBin setup from remaining `internal/cmd/check_test.go` tests**

Read the two `changelog:` occurrences (near line 140 and line 269 per this task's own research) and
their surrounding `writeConfig(t, ...)` blocks in full before editing — each is a YAML heredoc where
`changelog:` is followed by nothing (a dangling key that parses to YAML `null`, collapsing
`cfg.Changelog` to `nil` per the same bug class fixed by T177/T183/T184/the Phase A final review).
Change each bare `changelog:` line to `changelog: {}` so the fixture actually exercises "changelog
configured" as its test names/comments intend, matching the `{}` convention already used in
`docs/heraut.sample.yml` and `README.md` since Phase A.

In `TestCheckRuntime_NoConfigFile` (lines 99-131), `TestCheckRuntime_AllGood` (lines 135-170), and
`TestCheckAll_PassesAll` (lines 263-287), delete the `exectest.FakeBin(t, "git-cliff", ...)` and
`exectest.FakeBin(t, "communique", ...)` blocks — the "Generators" probe section they were faking a
binary for no longer exists, so these FakeBin registrations are now unused setup, not merely
harmless: leaving them implies these tests still exercise a Generators check, which is misleading to
a future reader. Re-run each test after removal and confirm it still passes for the same reason as
before (Git + Platforms sections only).

- [ ] **Step 7: Delete cliff-specific tests and strip git-cliff/communique MockRunner queueing from `internal/app/check_test.go`**

Read the full file first (it has ~115+ matches for git-cliff/communique per this task's own
research) to identify every `func Test...` that exclusively tests `CheckCliff`/`configuredGenerators`/
the RuntimeCheck "Generators" dispatch — delete those in full (their subject no longer exists).
`TestRuntimeCheck_WithGitcliff` (starting line 249) is one confirmed example — read its full body
before deciding whether it tests *only* the deleted Generators section (delete) or also exercises
something else still-relevant (adapt instead, keeping only the still-relevant assertions).

For every remaining `TestRuntimeCheck_*` test that queues `mr.QueueResponse("git-cliff ...", "",
nil)` / `mr.QueueResponse("communique ...", "", nil)` as boilerplate before asserting on Git/
Platforms behavior (e.g. `TestRuntimeCheck_GitValue`, `TestRuntimeCheck_UserNameValue`,
`TestRuntimeCheck_WorkingTreeClean`, `TestRuntimeCheck_WorkingTreeDirty`), delete those two queued
responses from each — `MockRunner` is a strict ordered FIFO (per `.claude/rules/testing.md`), so a
leftover queued response that nothing consumes will not fail the test by itself, but leaving it
misrepresents what the test exercises and risks silently masking a call-count mismatch if the
dispatch order ever changes again. `TestRuntimeCheck_DispatchNames` (line 212) explicitly lists
`"git-cliff", "communique"` in its expected dispatch-name slice (line 230) — remove those two
entries from the expected list.

`TestRuntimeCheck_SectionHeaders` (line 234) — read its assertions; if it asserts a "Generators"
header appears, remove that assertion; if the test's only remaining assertions cover Git/Platforms
headers, keep it as-is.

- [ ] **Step 8: Run the full check-related test suites**

Run: `go test ./internal/cmd/... ./internal/app/... -v`
Expected: PASS.

- [ ] **Step 9: Update `CLAUDE.md`'s command description**

In `CLAUDE.md`, find the line:

```
heraut check            # preflight: config + runtime + cliff (effective config)
```

Change to:

```
heraut check             # preflight: config + runtime
```

(Verify the surrounding command-block's alignment/spacing stays consistent with its neighboring
lines after this edit — this is a `\`\`\`` fenced block, not a markdown table, so column alignment
is cosmetic but should still look intentional.)

- [ ] **Step 10: Run the full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS. `internal/generators/gitcliff` and `internal/generators/communique` still exist and
still compile at this point (Task 188 deletes them) — `internal/app/check.go` no longer imports
`gitcliff` after Step 4, so `grep -rn "generators/gitcliff\|generators/communique" internal/app/
internal/cmd/` should now return zero hits; verify this before committing.

- [ ] **Step 11: Commit**

```bash
git add internal/cmd/check.go internal/app/check.go internal/cmd/check_test.go internal/app/check_test.go CLAUDE.md
git commit -m "feat(cmd): remove heraut check cliff and the git-cliff/communique runtime probes"
```

---

### Task 188: Delete the `gitcliff`/`communique` packages and the `Generator`/`Config` struct fields

> **Amendment (2026-08-15, during execution):** this task's original file list undercounted the
> `Generator:`/`.Config` literal blast radius substantially — `internal/generators/native/generator_internal_test.go`
> alone needed 14 literals fixed, not 3, and 9 more files across `internal/app/` and
> `internal/generators/native/` needed the same fix but were never listed. It also missed a real,
> functional (not inert-literal) consumer: `internal/pipeline/release_integration_test.go`'s
> `TestRun_Integration_MultiPlatform_DistinctlyFlavoredNotes` genuinely imported and called
> `gitcliff.New(...)` — deleted in full (ruling: its specific purpose, proving `HERAUT_REMOTE_URL`
> propagates through a *real* subprocess, is structurally moot once native — which never shells out
> for generation — is the only generator; the broader per-platform-distinct-notes behavior it also
> covered is already proven, generator-agnostically, by `internal/pipeline/release_test.go:243`
> `TestRun_MultiPlatform_NotesPerPlatform`). It also missed that `internal/scaffold`'s 4 *test* files
> (`wizard_internal_test.go`, `wizard_test.go`, `generate_test.go`, `dropped_test.go` — as opposed to
> its 4 *production* files, which this task's Global Constraints correctly forbid touching) construct
> `ContentDriver{Generator: ...}` literals and, in `wizard_internal_test.go`, read `.Generator` back
> off a result — both break once the fields are deleted. Ruling: fix these 4 scaffold test files too
> (literal strips + adapting the 2 field-read assertions to a different mechanism, same discipline as
> the `merge_test.go` fixes below), since the plan's Global Constraints, read literally, only forbid
> editing scaffold's 4 named *production* files — the directory-wide "OUT OF SCOPE" framing was about
> not doing Phase C's wizard-redesign work, not about leaving the build broken, and "no phase lands
> with a broken build even temporarily" is this repo's own explicit, repeated, higher-order rule. Full
> reasoning in the SDD ledger (`.superpowers/sdd/2026-08-14-native-only-generator-phase-b/progress.md`).

**Files:**
- Delete: `internal/generators/gitcliff/` (entire directory: `cliff.changelog.toml`,
  `cliff.release-notes.toml`, `embed.go`, `generator.go`, `generator_test.go`,
  `linkenv_internal_test.go`, `merge.go`, `merge_test.go`, `remote_internal_test.go`)
- Delete: `internal/generators/communique/` (entire directory: `generator.go`, `generator_test.go`)
- Modify: `internal/config/config.go:89-91` (`ContentDriver` struct — delete `Generator`, `Config` fields)
- Modify: `internal/config/merge_test.go` (strip `Generator:` from all 11 struct literals)
- Modify: `internal/config/validator_forge_test.go` (strip `Generator:` from its 1 struct literal)
- Modify: `internal/generators/native/generator_internal_test.go` (strip `Generator: "native"` from
  its 3 struct literals — lines 58, 78, 96 per this task's own research)
- Modify: `internal/testutil/constants.go` (delete `GitCliff`, `Communique` constants)
- Modify: `internal/testutil/realgit.go` (reword doc comment)

**Interfaces:**
- Consumes: nothing — by this point (after Tasks 185-187), `internal/generators/gitcliff` and
  `internal/generators/communique` have zero importers anywhere in the non-test codebase. Verify
  this in Step 1 before deleting.
- Produces: `config.ContentDriver` no longer has `Generator`/`Config` fields — any code outside this
  plan's scope that referenced them (there should be none left) will fail to compile, which is the
  intended forcing function to catch anything this research missed.

- [ ] **Step 1: Verify zero remaining non-test importers of `gitcliff`/`communique`**

Run: `grep -rln "internal/generators/gitcliff\|internal/generators/communique" --include="*.go" . | grep -v "_test.go" | grep -v "^./internal/generators/gitcliff/" | grep -v "^./internal/generators/communique/"`
Expected: empty output. If anything appears, stop — it means Tasks 185-187 missed a consumer; do
not proceed with deletion until it's resolved (either this task's scope grows to cover it, or an
earlier task was incomplete and needs revisiting).

- [ ] **Step 2: Write the failing test — `ContentDriver` has no `Generator`/`Config` fields**

This is a compile-time property, not a runtime assertion — Go has no reflection-free way to assert
"a struct field does not exist" as a passing/failing test. Skip the usual red-green test step for
the field removal itself; instead, Step 3 below removes the fields directly and Step 4 proves the
codebase still compiles and every existing test still passes, which is the correct verification for
a pure struct-shrink refactor (matching how T177's own "required-check removal" was verified per the
roadmap's completion note — by the full test suite, not a new field-absence test).

- [ ] **Step 3: Delete the packages, the struct fields, and fix every remaining struct-literal reference**

Delete the directories:

```bash
git rm -r internal/generators/gitcliff internal/generators/communique
```

In `internal/config/config.go`, delete lines 90-91 from the `ContentDriver` struct:

```go
	Generator  string `yaml:"generator,omitempty"`
	Config     string `yaml:"config,omitempty"`
```

(Leave every other `ContentDriver` field — `Output`, `TagPattern`, `TagGlob`, `Template`, etc. —
untouched; this task's scope is exactly these two fields.)

In `internal/config/merge_test.go`, strip `Generator: "git-cliff"` / `Generator: "native"` from all
11 struct literals (lines 13, 19, 25, 34-35, 42-43, 45, 49, 65, 79, 89 per this task's own research)
— each literal keeps its other fields (`Output`, `TagPattern`, `Template`, etc.) unchanged; only the
`Generator:` key/value pair is removed from each. Read the file in full before editing since some of
these literals span multiple fields on one line (e.g. `Generator: "git-cliff", Output: "o",
TagPattern: "p", Template: "t"` becomes `Output: "o", TagPattern: "p", Template: "t"`).

In `internal/config/validator_forge_test.go`, strip `Generator: "native"` from its one literal (line 15).

In `internal/generators/native/generator_internal_test.go`, strip `Generator: "native"` from its 3
literals (`TestGenerator_GenerateReleaseNotes_TagGlob`, `TestGenerator_GenerateChangelog_TagGlob`,
`TestGenerator_GenerateReleaseNotes_TagPatternRegex`).

In `internal/testutil/constants.go`, delete the `GitCliff` and `Communique` constants:

```go
package testutil

// Binary name constants used in contract tests.
const (
	Git  = "git"
	GH   = "gh"
	GLab = "glab"
	Cog  = "cog"
)
```

In `internal/testutil/realgit.go`, reword the doc comment on `RealGitRepo` (currently references
"the real-CLI smoke tests (T77) that run the actual git-cliff / cog against heraut's embedded
default configs" — both of which are now gone; this helper is used today only by
`internal/cmd/commit_test.go` and `internal/commitwizard/commit_integration_test.go`, both
commit-verification tests, not generator smoke tests):

```go
// RealGitRepo creates a temporary git repository with a single tagged conventional commit
// and chdirs into it (restored on test cleanup). Used by tests that need a real git binary
// rather than MockRunner/FakeBin — e.g. commit-verification integration tests. Skips the
// test when git is not on PATH.
```

- [ ] **Step 4: Run the full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS. Also run `grep -rn "\.Generator\b\|GitCliff\|Communique" --include="*.go" internal/
| grep -v "_test.go\|internal/scaffold/"` — expected: empty (scaffold is explicitly out of scope,
per Global Constraints; any other hit means a reference was missed).

- [ ] **Step 5: Run `hk check`**

Run: `mise run lint:check` (or `hk check` directly per this repo's tooling)
Expected: clean — no unused-import or unused-symbol warnings from the deletions.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/merge_test.go internal/config/validator_forge_test.go internal/generators/native/generator_internal_test.go internal/testutil/constants.go internal/testutil/realgit.go
git rm -r internal/generators/gitcliff internal/generators/communique
git commit -m "refactor(config): delete gitcliff/communique packages and ContentDriver.Generator/.Config fields"
```

---

### Task 189: Remove the Real-CLI smoke-test exception from testing docs

**Files:**
- Modify: `.claude/rules/testing.md` (delete the "Real-CLI smoke tests" section)
- Modify: `docs/specs/06-dx-and-testing.md` (delete the matching smoke-test content; update Unit/
  Contract layer descriptions; drop the now-moot "Hard-won edge case" bullet)

**Interfaces:** None — pure documentation.

- [ ] **Step 1: Delete the "Real-CLI smoke tests" section from `.claude/rules/testing.md`**

Delete the entire section (currently lines 54-63, from `## Real-CLI smoke tests (embedded config
validation)` through the paragraph ending "...it is local and deterministic (no network,
`t.TempDir`)."), matching ADR-0028's precedent for cocogitto's removal (that ADR's Consequences
section notes "`docs/specs/06-dx-and-testing.md`'s real-CLI smoke-test exception (`testing.md`)
loses one of its two examples (git-cliff's remains)" — this task removes git-cliff's, the one
example that survived ADR-0028, leaving the exception category with zero live examples).

In its place, per the design doc's §2 instruction, add a one-line note:

```markdown
## Real-CLI smoke tests

heraut previously carved out a narrow exception here for testing embedded external-tool configs
against the real binary (git-cliff, then cocogitto — both since removed, see ADR-0028 and
ADR-0045). `native`, heraut's sole generator since ADR-0045, has zero external-binary dependency
for generation, so this exception category currently has no live example. Revive this pattern if a
future external dependency needs the same "MockRunner can't catch a real-tool rejection" coverage.
```

- [ ] **Step 2: Update `docs/specs/06-dx-and-testing.md`**

In the "Unit" section (lines 73-81), delete the line `- internal/generators/gitcliff/merge.go`
(line 79) from the bullet list of unit-test targets.

In the "Contract" section (lines 83-87), change:

```markdown
`internal/testutil.MockRunner` records every `Run` / `RunEnv` call into `[]Call` and
returns ordered `[]Response`. Used for every CLI invocation: `git`, `git-cliff`, `gh`,
`glab`, `communique`.
```

to:

```markdown
`internal/testutil.MockRunner` records every `Run` / `RunEnv` call into `[]Call` and
returns ordered `[]Response`. Used for every CLI invocation: `git`, `gh`, `glab`.
```

In the "Hard-won edge cases" list (lines 139-152), delete the bullet `- The git-cliff temp config
file lifecycle (cleanup on early return)` (line 148) — git-cliff no longer exists, so there is no
temp config file lifecycle left to regress on. Do not delete any of the other four bullets in this
list (SemVer ordering, CalVer PATCH reset, per-env cycle detection, E001/E002/E003 — all still
live, unrelated behaviors).

- [ ] **Step 3: Verify no other stray git-cliff/communique references remain in these two files**

Run: `grep -n "git-cliff\|communique" .claude/rules/testing.md docs/specs/06-dx-and-testing.md`
Expected: empty output.

- [ ] **Step 4: Commit**

```bash
git add .claude/rules/testing.md docs/specs/06-dx-and-testing.md
git commit -m "docs(testing): drop the git-cliff real-CLI smoke-test exception"
```

---

### Task 190: Rewrite `README.md`

**Files:**
- Modify: `README.md` (intro prose, generator count, Prerequisites table, Commands table, config
  field table)

**Interfaces:** None — pure documentation.

- [ ] **Step 1: Update the intro paragraph (lines 14-19)**

Change:

```markdown
**Héraut** (`heraut`) is a Go CLI that orchestrates release management for git-based
projects. One command resolves the next version, generates the changelog and release
notes, creates the git tag, and publishes the release to GitHub and/or GitLab. It wraps
the tools you already use — `git`, `git-cliff`, `gh`, `glab`, `communique` — and handles
the glue they can't: version resolution for prefixed-tag strategies, generator/platform
composition, and strict config validation.
```

to:

```markdown
**Héraut** (`heraut`) is a Go CLI that orchestrates release management for git-based
projects. One command resolves the next version, generates the changelog and release
notes, creates the git tag, and publishes the release to GitHub and/or GitLab. It wraps
the tools you already use — `git`, `gh`, `glab` — and handles the glue they can't: version
resolution for prefixed-tag strategies, platform composition, and strict config validation.
```

- [ ] **Step 2: Update the strategy/generator/platform summary line (lines 24-26)**

Change:

```markdown
It supports **four versioning strategies** (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), **two content generators** (`git-cliff`, `communique`),
and **two platforms** (`github`, `gitlab`).
```

to:

```markdown
It supports **four versioning strategies** (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), a **built-in content generator** (`native` — no external binary
required), and **two platforms** (`github`, `gitlab`).
```

- [ ] **Step 3: Update the Prerequisites table (lines 97-111)**

Change:

```markdown
## Prerequisites

When running via **binary or `go install`**, heraut does **not** bundle the external CLIs it
orchestrates — install the ones your config uses and make sure they are on `PATH`.
The **Docker image** bundles all of them at pinned versions; no extra setup needed.

| Tool | Needed for |
|------|------------|
| `git` | always |
| `git-cliff` | `generator: git-cliff` |
| `communique` | `generator: communique` |
| `gh` | `platform: github` |
| `glab` | `platform: gitlab` |

Run `heraut check runtime` to verify the tools and tokens for your config are available.
```

to:

```markdown
## Prerequisites

When running via **binary or `go install`**, heraut does **not** bundle the external CLIs it
orchestrates — install the ones your config uses and make sure they are on `PATH`.
The **Docker image** bundles all of them at pinned versions; no extra setup needed.

| Tool | Needed for |
|------|------------|
| `git` | always |
| `gh` | `platform: github` |
| `glab` | `platform: gitlab` |

Changelog and release-notes generation needs no external binary — `native` (heraut's built-in
generator) ships in the `heraut` binary itself.

Run `heraut check runtime` to verify the tools and tokens for your config are available.
```

- [ ] **Step 4: Update the Commands table (lines 129-141)**

Change:

```markdown
| Command | Description |
|---------|-------------|
| `heraut release` | Resolve next version → changelog → commit → tag → publish → notes |
| `heraut changelog` | Generate `CHANGELOG.md` only (optionally `--commit` / `--tag`) |
| `heraut version next` | Print the next version without side effects |
| `heraut version current` | Print the latest released tag for the active strategy / env |
| `heraut version sprint bump` | Increment the CalVer sprint counter in `.heraut.yml` |
| `heraut check` | Preflight: config + runtime + cliff (`config` / `runtime` / `cliff` subcommands) |
| `heraut cliff <mode>` | Print the effective merged git-cliff TOML (`changelog` / `release-notes`) |
| `heraut init` | Interactive wizard to generate `.heraut.yml` |
```

to:

```markdown
| Command | Description |
|---------|-------------|
| `heraut release` | Resolve next version → changelog → commit → tag → publish → notes |
| `heraut changelog` | Generate `CHANGELOG.md` only (optionally `--commit` / `--tag`) |
| `heraut version next` | Print the next version without side effects |
| `heraut version current` | Print the latest released tag for the active strategy / env |
| `heraut version sprint bump` | Increment the CalVer sprint counter in `.heraut.yml` |
| `heraut check` | Preflight: config + runtime (`config` / `runtime` subcommands) |
| `heraut init` | Interactive wizard to generate `.heraut.yml` |
```

- [ ] **Step 5: Update the config field table (lines 177-184)**

Change:

```markdown
| Block | Purpose |
|-------|---------|
| `versioning` | Strategy and options (`semver` / `calver` / `*-per-env`) |
| `changelog` | Generator for `CHANGELOG.md` (committed during `release`) |
| `forges` | Code-hosting connections heraut talks to (`github` / `gitlab`, or both); optional when auto-detected from CI or git origin |
| `release.notes` | Generator for the release-page notes |
| `release.targets` | Publish destinations, each referencing a `forges[].name` |
| `environments` | Per-environment config for `*-per-env` strategies (bump mode, tag format, promotion source, changelog/release overrides) |
```

to:

```markdown
| Block | Purpose |
|-------|---------|
| `versioning` | Strategy and options (`semver` / `calver` / `*-per-env`) |
| `changelog` | `CHANGELOG.md` generation (committed during `release`) |
| `forges` | Code-hosting connections heraut talks to (`github` / `gitlab`, or both); optional when auto-detected from CI or git origin |
| `release.notes` | Release-page notes generation |
| `release.targets` | Publish destinations, each referencing a `forges[].name` |
| `environments` | Per-environment config for `*-per-env` strategies (bump mode, tag format, promotion source, changelog/release overrides) |
```

- [ ] **Step 6: Verify no other stray git-cliff/communique/generator references remain**

Run: `grep -n "git-cliff\|communique\|generator" README.md`
Expected: no hits describing them as a live feature. (If `heraut cliff` or a generator-choice
mention surfaces anywhere else in the file beyond the sections edited above, read that section and
apply the same treatment.)

- [ ] **Step 7: Run the README-validation test**

Run: `go test ./... -run TestShippedExamples_LoadAndValidate -v`
Expected: PASS (this test loads/validates the fenced YAML examples inside README.md — confirm this
task's edits didn't touch a fenced example block in a way that breaks it; the edits above are all
prose/table, not the `.heraut.yml` example block at lines 151-175, so this should pass unchanged).

- [ ] **Step 8: Commit**

```bash
git add README.md
git commit -m "docs(readme): drop git-cliff/communique, native is heraut's sole generator"
```

---

### Task 191: Rewrite `docs/specs/02-configuration.md`'s Content generators table

**Files:**
- Modify: `docs/specs/02-configuration.md:680-694`

**Interfaces:** None — pure documentation.

- [ ] **Step 1: Replace the Content generators section**

Change:

```markdown
## Content generators

Used under `changelog` and `release.notes`. A project can use different generators for
each.

| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `generator`   | Yes      | One of: `git-cliff`, `communique`.                                                                                                                                                                                                                                                 |
| `config`      | No       | Path to the generator config file (relative to project root). For `git-cliff`: optional partial override, deep-merged with heraut's built-in default. For `communique`: required.                                                                                              |
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`).                                                                                                                                                                                                                                            |
| `tag_pattern` | No       | Tag pattern regex for `git-cliff` only. **For per-env strategies heraut auto-derives this from the effective `tag_format` so `--env <env>` only considers that environment's tags** (e.g. `{version}_{env}` + `--env prod` → `^.+_prod$`); set it explicitly to override the derivation. Setting `tag_pattern` with `communique` is a config validation error. |
| `template`    | No       | Path to a custom Tera template. Not used by `git-cliff` or `communique` (vestigial field with no current consumer; kept for forward compatibility).                                                                                                                              |

See [Spec 05 — Generators and Platforms](05-generators-and-platforms.md) for the full
behaviour of each generator.
```

to:

```markdown
## Content generation

Configured under `changelog` and `release.notes`. `native`, heraut's built-in renderer, is the
only generator (ADR-0045) — there is no `generator:` key to set; an empty `changelog: {}` /
`release: {notes: {}}` block means "generate with native, using defaults."

| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`).                                                                                                                                                                                                                                            |
| `tag_pattern` | No       | Tag pattern regex scoping which tags are considered. **For per-env strategies heraut auto-derives this from the effective `tag_format` so `--env <env>` only considers that environment's tags** (e.g. `{version}_{env}` + `--env prod` → `^.+_prod$`); set it explicitly to override the derivation. |
| `template`    | No       | Path to a full custom Go `text/template` file, parsed on top of native's built-ins (ADR-0037). See [Spec 05 § User-customizable templates](05-generators-and-platforms.md#user-customizable-templates-adr-0037). |

See [Spec 05 — Generators and Platforms](05-generators-and-platforms.md) for the full
behaviour of the native generator.
```

(This also fixes a pre-existing inaccuracy in the `template` row surfaced by this same edit: the
old row claimed `template` was "vestigial... with no current consumer," but native's own docs
[Spec 05] describe `<driver>.template` as a live, documented feature since ADR-0037 — the row was
simply never updated when ADR-0037 shipped. Fixing it here is in-scope because this task is already
rewriting this exact row to remove the git-cliff/communique framing that made it wrong in the first
place, not a separate cleanup pass.)

- [ ] **Step 2: Verify no other stray git-cliff/communique/generator references remain in this file**

Run: `grep -n "git-cliff\|communique\|generator" docs/specs/02-configuration.md`
Expected: no hits describing them as a live config option. (A reference inside a historical
cross-link, e.g. "ADR-0026" mentioning git-cliff in passing, is fine and out of scope — only fix
prose that presents git-cliff/communique as something a user configures today.)

- [ ] **Step 3: Commit**

```bash
git add docs/specs/02-configuration.md
git commit -m "docs(specs): drop generator/config fields from Spec 02's content-generation table"
```

---

### Task 192: Rewrite `docs/specs/05-generators-and-platforms.md`

**Files:**
- Modify: `docs/specs/05-generators-and-platforms.md` (the biggest single doc edit in this plan —
  read the whole file fresh before starting, since several subsections nested under `### git-cliff`
  are not actually git-cliff-specific)

**Interfaces:** None — pure documentation.

This file's `## Generators` section currently nests, under an `### git-cliff` heading (lines
154-337), two subsections that are **not** git-cliff-exclusive despite their placement: `####
Remote metadata (PR/author enrichment)` → `##### forges — explicit metadata forge (ADR-0043)`
(lines 221-250) is explicitly "valid as the enrichment source for both the `git-cliff` and `native`
generators" (its own line 248), and `##### Auto-detection and self-hosted hosts` (lines 252-336)
contains a general auto-detection fallback-chain description (lines 254-274) plus a `**native**`
subsection (lines 280-297) that must survive, alongside a `**git-cliff**` subsection (lines
298-308) that must not. Do not delete the whole `### git-cliff` heading's line range as one block —
follow the section-by-section instructions below.

- [ ] **Step 1: Rewrite the file header and generator comparison table (lines 1-17)**

Change:

```markdown
# Spec 05 — Generators and Platforms

Generators produce changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.targets` (each target referencing a `forges[].name`).

## Generators

Three generators are supported: `native` (heraut's canonical built-in renderer), `git-cliff`,
and `communique`. A project can use different generators for `changelog` and `release.notes`.

| Generator   | Strengths                                                        | Limits                                                       |
|-------------|------------------------------------------------------------------|--------------------------------------------------------------|
| `native`    | Built-in renderer, no external binary; changelog / release-notes driven by `commits` / `rendering` config; user-customizable templates (ADR-0037) | Go `text/template` only; git-cliff still owns Tera |
| `git-cliff` | Embedded opinionated default; deep-merged TOML overrides; labels new commits with `--tag <version>` | TOML config only                                            |
| `communique`| AI-assisted release notes from commit history                    | Requires a full config file; no embedded default              |
```

to:

```markdown
# Spec 05 — Generators and Platforms

The `native` generator produces changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.targets` (each target referencing a `forges[].name`).

## Generator

`native` is heraut's sole content generator (ADR-0045) — a built-in, zero-external-dependency
renderer driven by `commits` / `rendering` config, with a user-customizable template API
(ADR-0037).
```

- [ ] **Step 2: Keep the `### native` section (lines 18-153) as-is — no edit**

This entire section (heraut's built-in renderer, its remote-enrichment behavior, the
user-customizable template API, incremental changelog generation) describes native's own behavior
and is unaffected by this epic. Change its heading level from `### native` to `## native` (since
Step 1 removed the now-single-item `## Generators` wrapper and its comparison table — native no
longer needs to nest under a "Generators" plural heading when it is the only one).

- [ ] **Step 3: Delete git-cliff's own subsections, keep and un-nest the shared ones**

Delete entirely (git-cliff-exclusive invocation/config/output details):
- Lines 154-203 (the `### git-cliff` heading through its `--output` bullet — the TOML embed
  description, `heraut cliff` command reference, and both invocation blocks).
- Lines 204-219 (`#### Remote metadata (PR/author enrichment)` intro paragraph — describes
  git-cliff's own `[remote.*]` TOML injection and subprocess env-var setting, which no longer
  exists).

Keep, but re-home and re-heading these two subsections (they describe the general `forges:` config
concept and its auto-detection fallback chain, not git-cliff specifically):

- `##### forges — explicit metadata forge (ADR-0043)` (lines 221-250) → promote to `### forges —
  explicit metadata forge (ADR-0043)`, placed as a subsection of the (now-singular) `## native`
  content from Step 2, immediately after native's own content ends (i.e. right after the old line
  153). Delete the sentence "Valid as the enrichment source for both the `git-cliff` and `native`
  generators" (its own line 248's opening clause) and replace with "Valid as the enrichment source
  for `native`" — drop the "both ... and" framing since there is only one generator now. Leave the
  rest of this subsection's content (the YAML example, the fallback-chain description, the
  publish-vs-enrichment distinction) unchanged — none of it is git-cliff-specific prose.

- `##### Auto-detection and self-hosted hosts` (lines 252-336) → promote to `### Auto-detection and
  self-hosted hosts`, placed immediately after the re-homed `forges` subsection above. Within it:
  - Keep lines 254-274 (the three-source fallback chain description) unchanged — generic.
  - Delete lines 276-278 (the "What happens next depends on... native and git-cliff do not behave
    identically" framing paragraph) and replace with: "What happens next depends on
    `commits.enrichment_policy` and which of the three outcomes above occurred:"
  - Keep the `**native**` bullet (lines 280-297) unchanged — this is native's real, current
    behavior.
  - Delete the `**git-cliff**` bullet (lines 298-308) entirely — describes behavior that no longer
    exists.
  - Keep lines 310-336 (the `forges:` remedy example + the Azure DevOps URL-structure paragraph)
    unchanged — both are generator-agnostic.

Delete entirely (communique-exclusive):
- Lines 338-368 (`### communique` heading through its "Known limitation — multi-platform links"
  paragraph).

Delete entirely, or fold into a one-line note (`### No generator`, lines 369-373): this section's
content ("Omitting `changelog` or `release.notes` skips that output") is still true and worth
keeping, but its framing as one of several "generator" choices is stale. Replace the heading and
content with:

```markdown
### Omitting changelog or release notes

Omitting `changelog` or `release.notes` from `.heraut.yml` skips that output. The release is
still created on the configured platforms (an explicit `release.targets` entry, or the single
resolved forge with default options when `release.targets` is omitted).
```

Place this immediately after the "Auto-detection and self-hosted hosts" subsection.

- [ ] **Step 4: Update `## Generator interface` (lines 375-397)**

Change the `Validate()` doc line (line 387):

```markdown
`Validate()` is called by `heraut check config` and `heraut check cliff` and before
the pipeline runs. For generators with no config-file dependency (e.g. git-cliff with
only embedded defaults), `Validate()` returns `nil`.
```

to:

```markdown
`Validate()` is called by `heraut check config` and before the pipeline runs.
```

Change the "Per-platform link resolution" paragraph (lines 391-397):

```markdown
**Per-platform link resolution**: when a release targets **more than one** platform,
heraut regenerates the release notes once per platform and passes that platform's
`link` context (host, owner, repo, type) so commit/PR/MR links resolve to the correct
host and path shape (see [ADR-0021](../adr/0021-per-platform-release-notes.md)).
`git-cliff` consumes this context. A single-platform release passes
`nil`, and the generator falls through to ambient-CI link detection — today's unchanged
behaviour. **communique does not consume the context** (see its section above).
```

to:

```markdown
**Per-platform link resolution**: when a release targets **more than one** platform,
heraut regenerates the release notes once per platform and passes that platform's
`link` context (host, owner, repo, type) so commit/PR/MR links resolve to the correct
host and path shape (see [ADR-0021](../adr/0021-per-platform-release-notes.md)). A
single-platform release passes `nil`, and the generator falls through to ambient-CI
link detection.
```

- [ ] **Step 5: Keep `## Platforms` and its subsections (lines 399-526) unchanged**

This section (GitHub/GitLab publish drivers) is entirely generator-independent and out of scope
per the design doc's Non-goals ("Rewriting `gh`/`glab` publishing"). No edit.

- [ ] **Step 6: Delete `## Generator/platform combinations` (lines 528-533)**

This section's entire content is an example of combining *different* generators for changelog vs.
release-notes (`communique` for notes, `git-cliff` for changelog) — nonsensical with one generator.
Delete the heading and its paragraph entirely.

- [ ] **Step 7: Keep `## Extensibility` (lines 535-538) unchanged**

Generic, unaffected. No edit — though renumber/re-check its heading level still makes sense given
the sections removed above (it should still read as a top-level `##` section following `##
Platforms`).

- [ ] **Step 8: Verify the file reads coherently top-to-bottom**

Read the fully-edited file once more, straight through, checking: no heading references a deleted
section (e.g. no lingering "see the git-cliff section above"), no orphaned cross-reference link
target, heading levels are consistent (no orphaned `####` under a deleted `###` parent).

- [ ] **Step 9: Verify no other stray git-cliff/communique references remain**

Run: `grep -n "git-cliff\|communique" docs/specs/05-generators-and-platforms.md`
Expected: empty output, or only hits inside an ADR cross-reference link path that itself is a
historical document (e.g. a link to `0021-per-platform-release-notes.md`, unaffected by this
rename) — read any remaining hit before deciding it's acceptable.

- [ ] **Step 10: Commit**

```bash
git add docs/specs/05-generators-and-platforms.md
git commit -m "docs(specs): rewrite Spec 05 for native as heraut's sole generator"
```

---

### Task 193: Update `docs/tasks/roadmap.md`'s intro line

**Files:**
- Modify: `docs/tasks/roadmap.md:19`

**Interfaces:** None — pure documentation.

`docs/tasks/roadmap.md` is a heavily historical planning document (it still describes `cocogitto`,
an old `internal/adapter/exec/` package layout, `internal/selfupdate/`, and "14 ADRs" — all stale
for reasons unrelated to this epic, predating even Phase A). Fixing all of that file's staleness is
out of scope here; this task touches **only** the one line the design doc explicitly calls out.

- [ ] **Step 1: Update line 19**

Change:

```markdown
Héraut is a Go CLI that orchestrates `git-cliff`, `glab`, `gh`, `cog`, and `communique`
to resolve versions, generate changelogs, and publish releases to GitHub / GitLab. This
```

to:

```markdown
Héraut is a Go CLI that orchestrates `glab`, `gh`, and `cog` to resolve versions, generate
changelogs, and publish releases to GitHub / GitLab. This
```

(Leave `cog` in place — it is heraut's own commit-msg hook tool, per ADR-0027/ADR-0029, unrelated
to the content-generator dispatch this epic removes. Leave every other stale reference in this file
— the `internal/generators/{gitcliff,communique,cocogitto}/` file-tree entries at lines 141-148, the
"Layer 7: Generators" dependency-graph entries at lines 231-233, and the Contract-layer table row at
line 265 — untouched. If a future session wants this file's broader staleness fixed, that is a
separate, unrelated documentation-debt task, not part of this epic.)

- [ ] **Step 2: Verify only line 19 changed**

Run: `git diff docs/tasks/roadmap.md`
Expected: a single-line diff.

- [ ] **Step 3: Commit**

```bash
git add docs/tasks/roadmap.md
git commit -m "docs(roadmap): drop git-cliff/communique from the build-roadmap intro line"
```

---

### Task 194: Author ADR-0045; update the ADR index; supersede ADR-0010

**Files:**
- Create: `docs/adr/0045-native-sole-generator.md`
- Modify: `docs/adr/README.md` (add the ADR-0045 row)
- Modify: `docs/adr/0010-embedded-cliff-toml-default.md` (mark superseded)

**Interfaces:** None — pure documentation, but this is the phase's decision record and must be
internally consistent with what Tasks 185-192 actually did (write this task last, after the rest of
the plan is implemented, so its "Decision"/"Consequences" sections describe the real, landed change
rather than the plan's prediction of it).

- [ ] **Step 1: Author `docs/adr/0045-native-sole-generator.md`**

Follow the structure and tone of `docs/adr/0028-drop-cocogitto-generator.md` (the direct precedent
for this exact kind of hard-cutover generator removal) and this repo's standard ADR header format:

```markdown
# ADR-0045: Native as Heraut's Sole Content Generator

- **Status**: Accepted
- **Date**: 2026-08-14
- **Deciders**: bchatard

---

## Context

heraut shipped three changelog/release-notes generators: `native` (pure Go, no external binary),
`git-cliff` (embedded TOML defaults, shells out to the `git-cliff` binary), and `communique`
(fully opaque, shells out to `communique generate`). `native` has been heraut's **canonical**
renderer since [ADR-0032](0032-native-content-generator.md)/[ADR-0033](0033-native-config-model.md)
— git-cliff was explicitly named the *design anchor* to eventually drop then, with its own package
removal "sequenced after native enrichment, in a follow-up ADR." That follow-up point arrived once
native reached full parity: remote enrichment across GitHub/GitLab/Azure DevOps
([ADR-0034](0034-native-remote-enrichment.md)/[ADR-0042](0042-gitlab-graphql-enrichment.md)),
user-customizable templates ([ADR-0037](0037-native-template-api.md)), incremental changelog
generation ([ADR-0038](0038-incremental-changelog.md)), and commit-author attribution
([ADR-0039](0039-commit-author-attribution.md)).

Keeping two more generators around cost real, ongoing tax with no corresponding benefit: every
enrichment/policy change had to be reasoned about **per generator**, because git-cliff and native
had already diverged on `enrichment_policy: required` semantics and author-fallback rendering.
`communique` was never brought into that parity effort at all — a pure passthrough with no
heraut-owned behavior to keep in sync, and unused.

This project's own precedent — [ADR-0028](0028-drop-cocogitto-generator.md), dropping the
`cocogitto` generator — established the cutover mechanics this ADR reuses: a hard, undeprecated
removal landed directly on `main` (pre-v1.0, no branch protection, no installed base of external
users to support through a transition window).

## Decision

Remove `git-cliff` and `communique` as a hard cutover, in one epic phase, not a deprecation cycle:

- `ContentDriver.Generator` and `ContentDriver.Config` are **removed from the Go struct entirely**
  — not enum-shrunk to a single valid value. Once there is exactly one possible generator, the key
  carries zero information; keeping a boilerplate-only-required field would be dead weight in every
  `.heraut.yml` going forward. `changelog: {}` / `release: {notes: {}}` are valid and meaningful on
  their own: "generate with native, using defaults." (This is the first time the enum degenerates to
  one value — ADR-0028's cocogitto removal still left two generators standing, so its own decision
  enum-shrunk rather than field-removed; this decision does not carry that precedent forward,
  because the two situations differ in exactly this respect.)
- `generator:`/`config:` under `changelog:` and `release.notes:` (and their per-env variants)
  became hard config-load errors ahead of this phase, via the existing `ErrRemovedConfigKey`
  mechanism (a prior phase of this same epic, T177) — an existing user's config gets a clear,
  actionable error on its next `heraut` invocation, before the underlying packages ever
  disappeared out from under it mid-transition.
- `internal/generators/gitcliff/` and `internal/generators/communique/` (packages, embedded TOML
  defaults, Tera merge logic, contract tests, and the real-CLI embedded-config smoke test) are
  deleted outright.
- `heraut cliff <mode>` (`internal/cmd/cliff.go`, `internal/app/cliff.go`) is removed — it existed
  solely to show the effective merged git-cliff TOML, meaningless with no git-cliff config to merge.
- `heraut check cliff` and its default-output "Cliff" section, plus `internal/app/check.go`'s
  git-cliff/communique binary-presence probes, are removed — `heraut check runtime` now probes only
  `git`, `gh`, `glab`.
- `internal/app/pipeline.go`'s `buildGenerator` collapses from a three-way switch to an
  unconditional `native.New(...)` call; `usesNative` (used throughout `internal/app` to gate forge
  resolution) is deleted, since every driver is native by construction now.
- `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`, and
  `docs/specs/05-generators-and-platforms.md` drop `generator`/`config` from the documented
  `ContentDriver` shape.
- `.claude/rules/testing.md`'s "Real-CLI smoke tests" exception loses its only remaining example
  (cocogitto's was already dropped by ADR-0028) — native has zero external-binary dependency for
  generation, so the category currently has no live example.

### What does not change

Historical ADRs that mention git-cliff/communique in passing (0006, 0010, 0011, 0012, 0021-0026,
0032-0043...) remain as accurate records of the decisions made *at the time* — they are not
rewritten, per ADR-0028's own explicit rule for this exact situation.
[ADR-0010](0010-embedded-cliff-toml-default.md) is the one deliberate exception: its entire
subject — which embedded TOML defaults heraut ships for git-cliff — becomes moot, not just
tangentially mentioned, so it is marked `Superseded by ADR-0045` (see that ADR's own updated
status).

## Consequences

- Anyone with `generator: git-cliff` or `generator: communique` in an existing `.heraut.yml` gets a
  hard config error on their next `heraut` invocation (already true as of the prior phase of this
  epic, T177) — this phase removes the underlying packages the error was warning them away from.
  No deprecation warning period precedes this.
- heraut ships with zero external-binary dependency for changelog/release-notes generation —
  `native` is pure Go, embedded in the `heraut` binary. `git`/`gh`/`glab` remain required for git
  operations and publishing, unrelated to content generation.
- The Docker image and `.config/mise/config.toml` tool matrix shrink (a later, separate phase of
  this epic — Phase D).
- `internal/scaffold/wizard.go`'s generator-choice prompt (still offering `git-cliff`/`communique`
  as live options today, a known-stale leftover this phase deliberately does not touch) is
  simplified in a later, separate phase of this epic (Phase C) — see that phase's own scope note in
  `docs/tasks/native-generator-roadmap.md`.
- Future changelog-generator additions (if any) are evaluated against `native`'s existing coverage
  first, to avoid reintroducing a generator-dispatch tax this epic just removed.
```

(Adjust the "What does not change" ADR list and any other detail above to match what Tasks 185-192
actually landed, if implementation diverged from this plan in any way — this ADR must describe
reality, not the plan's prediction of it.)

- [ ] **Step 2: Add the ADR-0045 row to `docs/adr/README.md`**

After the existing last row (ADR-0044), add:

```markdown
| [0045](0045-native-sole-generator.md) | Native as Heraut's Sole Content Generator | Accepted |
```

- [ ] **Step 3: Mark `docs/adr/0010-embedded-cliff-toml-default.md` superseded**

Change the header block (lines 1-6):

```markdown
# ADR-0010: Embedded `cliff.toml` Default with Optional User Override

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---
```

to (matching the convention `docs/adr/0014-self-update-architecture.md` uses for a fully-replaced
decision):

```markdown
# ADR-0010: Embedded `cliff.toml` Default with Optional User Override

- **Status**: Superseded by [ADR-0045](0045-native-sole-generator.md)
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

> **Superseded (2026-08-14).** heraut no longer ships `git-cliff` as a generator — `native` is
> its sole content generator ([ADR-0045](0045-native-sole-generator.md)). This ADR's entire
> subject — the embedded `cliff.toml` defaults and their merge semantics — is gone along with
> `internal/generators/gitcliff/`. Kept as a historical record of the decision at the time.

```

Leave the rest of the file (Context/Decision/Consequences/"Changing the embedded defaults")
completely unchanged below the new blockquote — per this repo's established convention for a
superseded ADR (matching ADR-0014, ADR-0020), only the header and a leading blockquote change.

- [ ] **Step 4: Verify ADR numbering and cross-links**

Run: `ls docs/adr/0045-*.md` (expect exactly one file) and `grep -n "0045" docs/adr/README.md
docs/adr/0010-embedded-cliff-toml-default.md` (expect the new row and the new Status/blockquote,
respectively).

- [ ] **Step 5: Commit**

```bash
git add docs/adr/0045-native-sole-generator.md docs/adr/README.md docs/adr/0010-embedded-cliff-toml-default.md
git commit -m "docs(adr): add ADR-0045, native as heraut's sole content generator"
```

---

## Self-Review Notes

- **Spec coverage**: every item in the design doc's §1 (config shape), §2 (package + command
  removal), and §3 (docs + schema — noting schema.json/testdata were already done in Phase A) is
  covered by Tasks 185-192. The ADR-0045 outline is covered by Task 194. §4 (wizard, Phase C) and
  §5 (infra, Phase D) are correctly out of scope. The roadmap's Phase B scope note's five leftover
  items (README, specs 02/05, `configuredGenerators`, `heraut check cliff`'s empty-generator
  render, the two dangling `changelog:` keys) are covered by Tasks 187/190/191/192.
- **Ordering**: Tasks 185→186→187→188 are strictly ordered by compile-safety (each removes the last
  remaining non-test consumer of `gitcliff`/`communique`/`.Generator`/`.Config` before the next task
  deletes what it depends on) — verified by tracing every call site read during this plan's
  research. Tasks 189-194 (docs) have no code dependency and could in principle run in any order
  after 188, but are sequenced to match the design doc's own file grouping.
- **Test preservation**: every test deletion in this plan is justified either by (a) the test's
  scenario becoming structurally impossible (a struct literal setting a field that no longer
  exists) or (b) the underlying behavior being a deliberate, ADR-0045-documented removal — never by
  "this test is inconvenient now." Every behavior-preserving test (TagPattern/TagGlob per-env
  derivation) is explicitly rewritten to a working mechanism, not silently dropped.
