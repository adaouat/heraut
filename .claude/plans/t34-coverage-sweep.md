# T34 — Coverage sweep: enforce 80%, target 85%

## Context

Total coverage is 69.6% (466 tests / 22 packages). CI now gates at 70%. This task fills
the meaningful coverage gaps — primarily the `internal/app/` wiring layer (currently 0%)
and several trivial/missing test cases across config, scaffold, generators, versioning,
selfupdate, ui, and platforms. The target is ≥85% actual coverage; T35's CI gate will be
raised from 70% to 80% in the same commit.

Explicitly excluded (untestable without a VT100 harness or binary exec):
`internal/scaffold/wizard.go`, `cmd/heraut/main.go`, `internal/testutil/`.

## Work, package by package

### A. `internal/app/` — new test files (currently 0%)

All functions are wiring factories. Tests use `testutil.NewMockRunner()`,
`testutil.MockGenerator`, `testutil.MockPlatform`, and a local `fakeResolver` struct
(implements `versioning.Resolver` with `Resolve() → (Result{Version:"1.0.0",Tag:"v1.0.0"}, nil)`).

#### `internal/app/resolver_test.go` (new)
- `TestNewResolver_Semver` — passes, returns a Resolver
- `TestNewResolver_Calver` — passes
- `TestNewResolver_SemverPerEnv` — needs env config; passes
- `TestNewResolver_CalverPerEnv` — needs env config + format; passes
- `TestNewResolver_UnknownStrategy` — returns error containing the strategy name
- `TestNewResolver_VersionOverride` — semver with versionOverride set, calls SetVersionOverride

#### `internal/app/pipeline_test.go` (new)

`BuildPipeline` and `BuildChangelogPipeline` path coverage:
- `TestBuildPipeline_Minimal` — no changelog, no platforms; returns non-nil Pipeline
- `TestBuildPipeline_WithGitcliff` — `Changelog.Generator: "git-cliff"` → succeeds
- `TestBuildPipeline_WithCommunique` — `Changelog.Generator: "communique"` → succeeds
- `TestBuildPipeline_WithCocogitto` — `Changelog.Generator: "cocogitto"` → succeeds
- `TestBuildPipeline_UnknownGenerator` — `Changelog.Generator: "unknown"` → error
- `TestBuildPipeline_WithGitHubPlatform` — `Release.Platforms: [{Type:"github"}]` → succeeds
- `TestBuildPipeline_WithGitLabPlatform` — `Release.Platforms: [{Type:"gitlab"}]` → succeeds
- `TestBuildPipeline_UnknownPlatform` — `Release.Platforms: [{Type:"unknown"}]` → error
- `TestBuildPipeline_WithNotes` — `Release.Notes.Generator: "git-cliff"` → succeeds
- `TestBuildPipeline_PerEnvDisableFlags` — env with DisableChangelog+DisableNotes → flags propagated
- `TestBuildPipeline_AnnotatedTagsDefault` — empty TagType → AnnotatedTags true
- `TestBuildPipeline_LightweightTagType` — `TagType: "lightweight"` → AnnotatedTags false
- `TestBuildChangelogPipeline_WithCommitAndTag` — Commit+Tag opts set
- `TestBuildChangelogPipeline_PerEnvDisable` — env DisableChangelog propagated

#### `internal/app/check_test.go` (new)

`PreflightCheck`:
- `TestPreflightCheck_Passes` — queues git version + user.name "Alice" + user.email "a@b.com"
- `TestPreflightCheck_GitMissing` — first Run returns error → error contains "git not found"
- `TestPreflightCheck_UserNameMissing` — git version OK, user.name returns `""` → error "user.name"
- `TestPreflightCheck_UserEmailMissing` — git version + name OK, email `""` → error "user.email"

`RuntimeCheck`:
- `TestRuntimeCheck_MinimalConfig` — no changelog/platform; verifies git + user items returned
- `TestRuntimeCheck_WithGitcliff` — changelog: git-cliff; checks generator item included
- `TestRuntimeCheck_WithGitHubPlatform` — platform: github; checks platform item included

`CheckCliff` (app-level, delegates to gitcliff.Generator.CheckCliff):
- `TestAppCheckCliff_Passes` — mode "changelog", runner returns success
- `TestAppCheckCliff_Fails` — runner returns error → error propagated
- `TestAppCheckCliff_ReleaseNotesMode` — mode "release-notes" → uses ModeReleaseNotes

#### `internal/app/cliff_test.go` (new)

`EffectiveCliffConfig`:
- `TestEffectiveCliffConfig_NilDriver` — returns non-empty TOML (embedded default)
- `TestEffectiveCliffConfig_EmptyDriver` — `&ContentDriver{}` → same as nil
- `TestEffectiveCliffConfig_ReleaseNotesMode` — mode "release-notes" → different TOML
- `TestEffectiveCliffConfig_NonGitcliff` — `ContentDriver{Generator:"communique"}` → error

#### `internal/app/current_test.go` (new)

`CurrentTag`:
- `TestCurrentTag_Semver` — runner returns `"v1.2.3\nv1.2.2"`, expects `"v1.2.3"`
- `TestCurrentTag_Calver` — runner returns `"2026.05.1"`, empty prefix
- `TestCurrentTag_SemverPerEnv` — needs env config with tag_format; verifies glob arg
- `TestCurrentTag_PerEnvMissingEnvArg` — strategy=semver-per-env, env="" → error "--env required"
- `TestCurrentTag_NoTags` — runner returns `""` → error "no tags"
- `TestCurrentTag_GitError` — runner returns error → error propagated
- `TestCurrentTag_UnknownStrategy` — strategy="unknown" → error

---

### B. `internal/config/error_test.go` (new, trivial)

- `TestValidationError_Error_WithHint` — path+message+hint → expected string format
- `TestValidationError_Error_NoHint` — no hint → no hint line
- `TestValidationErrors_Error_Multiple` — two errors joined by `\n`
- `TestValidationErrors_Error_Empty` — empty slice → `""`

---

### C. `internal/config/sprint_test.go` (new)

Uses `t.TempDir()` for real file I/O.

- `TestIncrementSprint_FastPath` — file has `sprint: 3`; returns 4, file contains `sprint: 4`
- `TestIncrementSprint_SlowPath` — file has no sprint field; returns 1, file contains `sprint: 1`
- `TestIncrementSprint_FileNotFound` — nonexistent path → error

---

### D. `internal/scaffold/cliff_cog_test.go` (new, trivial)

- `TestIsCliffGenerator` — "git-cliff" → true, "communique" → false
- `TestIsCogGenerator` — "cocogitto" → true, "git-cliff" → false

---

### E. `internal/generators/gitcliff/generator_test.go` (add to existing)

`CheckCliff`:
- `TestCheckCliff_Passes` — runner returns success; assert args: `["--context", "--no-exec", "--config", <path>]`
- `TestCheckCliff_Fails` — runner returns error → error contains "git-cliff rejected config"

---

### F. `internal/versioning/semver/resolver_test.go` (add to existing)

Direct calls to `BumpAuto` and `BumpFromDate` (currently 0%):
- `TestBumpAuto_NoTags` — tags nil → returns `initialVersion` ("0.1.0")
- `TestBumpAuto_WithCommits` — tags=["1.2.3"], commits=["feat: x"] → "1.3.0"
- `TestBumpAuto_NoCommitsSinceTag` — tags=["1.2.3"], commits=[] → error "no commits"
- `TestBumpFromDate_Unsupported` — always returns error containing "not supported"

---

### G. `internal/selfupdate/` — new internal test file

`defaultCacheDir` and `permissionError` are unexported; use `package selfupdate` (internal).
New file: `internal/selfupdate/helpers_test.go` (package `selfupdate`).

- `TestDefaultCacheDir_XDGSet` — `t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")` → `"/tmp/xdg/heraut"`
- `TestDefaultCacheDir_Default` — unset XDG → result contains `"heraut"`, no error
- `TestPermissionError_PermissionDenied` — pass `&os.PathError{Err: syscall.EACCES}` → contains "permission denied" and "sudo"
- `TestPermissionError_OtherError` — non-permission error → contains "replacing binary"

---

### H. `internal/ui/status_test.go` (add to existing)

Styled code path. Use `t.Setenv("CLICOLOR_FORCE", "1")` to force color detection with a
`bytes.Buffer` writer (colorprofile honors CLICOLOR_FORCE for non-TTY writers).
- One test per function (`Success`, `Err`, `Warn`, `Info`) asserting:
  - Result still contains the symbol (✓/✗/!/  )
  - Result is different from the plain-text form (ANSI codes added)

---

### I. `internal/platforms/github/platform_test.go` and `gitlab/platform_test.go` (add)

`UploadAssets` glob error paths (currently 66.7%):
- `TestUploadAssets_NoMatchingFiles` — asset pattern `"*.xyz"` in a tempdir with no matches
  → error contains "no files matched"
- Note: `filepath.Glob` never returns an error for valid patterns (only for malformed ones
  like `[invalid`), so the invalid-pattern error path is OS-dependent and not worth testing.

---

## CI threshold update

In the same commit as the final coverage check: bump the threshold in `.github/workflows/ci.yml`
from 70% to 80%.

```yaml
# Before
if (pct+0 < 70) {
# After
if (pct+0 < 80) {
```

---

## Order of implementation

1. config/error_test.go, config/sprint_test.go, scaffold/cliff_cog_test.go — trivial, verify baseline
2. semver BumpAuto/BumpFromDate, gitcliff CheckCliff — add to existing test files
3. selfupdate helpers_test.go — internal test file for private functions
4. ui/status_test.go styled path, platforms glob error
5. app/ — the bulk: resolver, pipeline, check, cliff, current (5 new test files)
6. Verify `go test ./... -coverprofile=coverage.out` → total ≥ 80%
7. Bump CI threshold from 70% → 80% in ci.yml
8. Commit all test changes + ci.yml threshold + roadmap T34 done

## Verification

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
# Must show ≥ 80.0%
```
