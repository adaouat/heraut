# Native-Only Generator — Phase A: Config Cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `generator:` / `config:` keys under `changelog:` and `release.notes:` (top-level and
per-env) a hard config error, and make native generation work with **no** `generator:` key present
at all — without yet deleting the `git-cliff`/`communique` packages, the `heraut cliff` command, or
touching the wizard (all Phase B/C, separate plans).

**Architecture:** Extend the existing `ErrRemovedConfigKey` / `checkRemovedKeys` mechanism
(`internal/config/loader.go`, built by T160/T163 for `changelog.remote`/`release.platforms`) with
four new removed-key entries. `ContentDriver.Generator`/`.Config` **stay in the Go struct** for now
— removed-key rejection happens at the raw-YAML probe stage, before the struct is ever populated
with a non-empty value, so every config that successfully loads has `Generator == ""` going
forward. Two small compatibility points in `internal/app/pipeline.go` make `Generator == ""` behave
as native (build a native generator; count as "uses native" for forge resolution) so the pipeline
keeps working end-to-end. Everywhere else that reads `.Generator` (`internal/app/check.go`,
`internal/app/cliff.go`, `internal/cmd/check.go`, `internal/scaffold/wizard.go`) is untouched here —
each degrades gracefully to a permanent no-op/skip state and gets properly deleted in Phase B, which
depends on this phase landing first.

**IMPORTANT — bigger blast radius than it looks:** the new removed-key check fires inside
`config.Load`/`config.LoadFromReader`, which is also what `internal/config`'s own test helper
`mustLoad` calls. Roughly 30 existing tests across `validator_test.go` and `loader_test.go` build
their fixture YAML inline with a `generator:`/`config:` line as filler — every one of them breaks
the instant Task 1 lands, whether or not they have anything to do with generators. Task 2 exists
specifically to fix all of that collateral damage in one pass, so Tasks 3-6 (the actual
validator.go/merge.go logic changes) each start from a green suite.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `github.com/santhosh-tekuri/jsonschema/v6` (schema
tests), `testify` (assert/require).

## Global Constraints

- TDD: write the failing test first, watch it fail, then implement (`.claude/rules/testing.md`,
  `.claude/rules/claude.md`).
- Every commit must be a conventional commit; run `hk fix` if a hook reports a lint failure, never
  bypass hooks (`.claude/rules/workflow.md`).
- `go test ./...` and `hk check` must be clean after **every** task's commit — no task leaves the
  build or test suite broken, even temporarily. (Task 1 is the one narrow exception, explicitly
  called out in its own commit step, and is fixed by Task 2 immediately after in the same session.)
- Task IDs continue the global sequence from **T177** (see
  `docs/superpowers/specs/2026-08-08-native-only-generator-design.md`).
- Line numbers cited throughout this plan were read directly from the source at planning time.
  Files shift as earlier tasks in this plan land — always re-`grep`/re-read immediately before
  editing rather than trusting a stale line number blindly. Every step below still tells you the
  exact function name and exact before/after content, so a shifted line number is a minor
  inconvenience, not an ambiguity.
- Do not touch `internal/generators/{gitcliff,communique}/`, `internal/cmd/cliff.go`,
  `internal/app/cliff.go`, `internal/app/check.go`'s generator probes, `internal/cmd/check.go`'s
  cliff check, or `internal/scaffold/` — all Phase B/C, out of scope for this plan.
- Do not touch `docs/specs/02-configuration.md` or `docs/specs/05-generators-and-platforms.md` at
  all — deferred to Phase B so the generator-comparison prose is rewritten once, correctly, when
  git-cliff/communique are actually gone, not twice.

---

### Task 1 (T177): Reject `generator:`/`config:` keys at load time

**Files:**
- Modify: `internal/config/loader.go:31-35` (the `removedKeys` table), `:42-84`
  (`checkRemovedKeys`'s probe struct and per-env loop)
- Modify: `internal/config/migration_test.go` (new test cases)
- Modify: `docs/tasks/native-generator-roadmap.md` (add the Phase 2.5 section skeleton + progress
  table row)

**Interfaces:**
- Consumes: `config.ErrRemovedConfigKey` (existing sentinel, `internal/config/loader.go:18`)
- Produces: nothing new consumed by later tasks in this plan — this task is purely additive at the
  YAML-probe layer.

- [ ] **Step 1: Add the roadmap skeleton for Phase 2.5**

  Open `docs/tasks/native-generator-roadmap.md`. In the progress table (around line 39), change:

  ```
  | Phase 2.5 — remove the git-cliff package (own ADR)   | —                      | Deferred    |
  ```

  to:

  ```
  | Phase 2.5 — remove the git-cliff package (own ADR)   | T177–T184              | Active      |
  ```

  Then, after the `## Phase 2.10 — Commit-author attribution (ADR-0039)` section and before
  `## Phase 3 — Raw-HTTP platform clients (deferred)` (currently around line 950), insert:

  ```markdown
  ## Phase 2.5 — Remove the git-cliff package (own ADR)

  > See `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` for the full design
  > (drops `communique` too, in a later phase of this same epic) and ADR-0045 (written as part of
  > this phase) for the decision record.

  Config cutover: `generator:` / `config:` under `changelog:` and `release.notes:` become removed
  keys (hard error, no deprecation window — matching ADR-0028's cocogitto-removal precedent).
  Native becomes implicit. `git-cliff`/`communique` package deletion, the `heraut cliff` command,
  and wizard simplification are separate, later phases in this same file.

  #### `[ ]` T177: reject `generator:`/`config:` keys at load time

  #### `[ ]` T178: fix collateral test damage from T177 (`internal/config`)

  #### `[ ]` T179: empty `Generator` builds native end-to-end

  #### `[ ]` T180: validator — drop generator-required/enum + tag_pattern generator gate

  #### `[ ]` T181: validator — drop template/tickets/rendering generator gates

  #### `[ ]` T182: merge — drop the generator-switch full-replacement branch

  #### `[ ]` T183: schema.json + testdata fixtures go native-only

  #### `[ ]` T184: `docs/heraut.sample.yml` drops `generator:`/`config:`
  ```

  Commit this alone first (docs-only, no code yet):

  ```bash
  git add docs/tasks/native-generator-roadmap.md
  git commit -m "docs(roadmap): open Phase 2.5 — remove the git-cliff package"
  ```

- [ ] **Step 2: Write the failing tests**

  Add to `internal/config/migration_test.go`, inside `TestLoad_RemovedKeys`'s `tests` table
  (after the `"commits.remote_metadata"` case, before the closing `}` of the table):

  ```go
  		{
  			name: "changelog.generator",
  			body: `version: "1"
  versioning: {strategy: semver}
  changelog:
    generator: native
    output: CHANGELOG.md
  `,
  			wantHint: "native is heraut's only generator",
  		},
  		{
  			name: "changelog.config",
  			body: `version: "1"
  versioning: {strategy: semver}
  changelog:
    config: cliff.toml
  `,
  			wantHint: "rendering.templates",
  		},
  		{
  			name: "release.notes.generator",
  			body: `version: "1"
  versioning: {strategy: semver}
  release:
    notes:
      generator: native
  `,
  			wantHint: "native is heraut's only generator",
  		},
  		{
  			name: "release.notes.config",
  			body: `version: "1"
  versioning: {strategy: semver}
  release:
    notes:
      config: comm.yaml
  `,
  			wantHint: "rendering.templates",
  		},
  ```

  Then add two new test functions right after `TestLoad_RemovedKeys_PerEnvChangelogRemote` (which
  ends around line 93):

  ```go
  // TestLoad_RemovedKeys_PerEnvGenerator checks the per-env removed-key error for
  // environments.<env>.changelog.generator and environments.<env>.release.notes.generator, names
  // the specific environment, and carries the same hint as the top-level case.
  func TestLoad_RemovedKeys_PerEnvGenerator(t *testing.T) {
  	tests := []struct{ name, body string }{
  		{
  			name: "changelog.generator",
  			body: `version: "1"
  versioning: {strategy: semver}
  environments:
    staging:
      changelog:
        generator: native
        output: CHANGELOG.md
  `,
  		},
  		{
  			name: "release.notes.generator",
  			body: `version: "1"
  versioning: {strategy: semver}
  environments:
    staging:
      release:
        notes:
          generator: native
  `,
  		},
  	}
  	for _, tc := range tests {
  		t.Run(tc.name, func(t *testing.T) {
  			_, err := config.Load(writeCfg(t, tc.body))
  			require.Error(t, err)
  			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
  			assert.Contains(t, err.Error(), "staging", "the error must name which environment carries the removed key")
  			assert.Contains(t, err.Error(), "native is heraut's only generator", "the hint must be present")
  		})
  	}
  }

  // TestLoad_RemovedKeys_PerEnvConfig mirrors TestLoad_RemovedKeys_PerEnvGenerator for the
  // config: key (external generator config file path — meaningless without git-cliff/communique).
  func TestLoad_RemovedKeys_PerEnvConfig(t *testing.T) {
  	tests := []struct{ name, body string }{
  		{
  			name: "changelog.config",
  			body: `version: "1"
  versioning: {strategy: semver}
  environments:
    staging:
      changelog:
        config: cliff.toml
  `,
  		},
  		{
  			name: "release.notes.config",
  			body: `version: "1"
  versioning: {strategy: semver}
  environments:
    staging:
      release:
        notes:
          config: comm.yaml
  `,
  		},
  	}
  	for _, tc := range tests {
  		t.Run(tc.name, func(t *testing.T) {
  			_, err := config.Load(writeCfg(t, tc.body))
  			require.Error(t, err)
  			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey), "must be the removed-key sentinel")
  			assert.Contains(t, err.Error(), "staging", "the error must name which environment carries the removed key")
  			assert.Contains(t, err.Error(), "rendering.templates", "the hint must point at the native replacement")
  		})
  	}
  }
  ```

- [ ] **Step 3: Run tests to verify they fail**

  Run: `go test ./internal/config/... -run TestLoad_RemovedKey -v`

  Expected: the four new `TestLoad_RemovedKeys` subtests and both new test functions FAIL — the
  configs load successfully today (no error), so `require.Error(t, err)` fails with "An error is
  expected but got nil."

- [ ] **Step 4: Implement — extend the removed-keys table and probe**

  In `internal/config/loader.go`, replace the `removedKeys` var block (lines 30-35):

  ```go
  // generatorRemovedHint is the migration guidance for changelog.generator / release.notes.generator
  // (and their per-env variants): native is now heraut's only generator, so the key carries no
  // information and is removed rather than enum-shrunk to one value.
  const generatorRemovedHint = "native is heraut's only generator now; remove this key"

  // configKeyRemovedHint is the migration guidance for changelog.config / release.notes.config (the
  // external git-cliff/communique config-file path): native has no external config file — use
  // rendering.templates (ADR-0037) for template customization instead.
  const configKeyRemovedHint = "generator-specific config files are gone; use rendering.templates (ADR-0037) for template customization instead"

  // removedKeys maps a removed config path to its replacement guidance.
  var removedKeys = []struct{ path, hint string }{
  	{"changelog.remote", "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)"},
  	{"commits.remote_metadata", "rename to `commits.enrichment_policy` (same values: disabled | optional | required)"},
  	{"release.platforms", releasePlatformsHint},
  	{"changelog.generator", generatorRemovedHint},
  	{"changelog.config", configKeyRemovedHint},
  	{"release.notes.generator", generatorRemovedHint},
  	{"release.notes.config", configKeyRemovedHint},
  }
  ```

  (This appends the four new entries **after** the three existing ones — table order matters:
  `checkRemovedKeys` returns on the first match, so an existing fixture that trips both
  `changelog.remote` and `changelog.generator` still gets the `changelog.remote` error first,
  unchanged from today — verified this holds for `TestLoad_RemovedKey_ChangelogRemoteHintMentionsNativeOnly`
  and `TestLoadFromReader_rejectsRemovedRemoteAPIURLKey`, both of which combine `changelog.remote`
  with a `generator:` line and must keep asserting the `changelog.remote` error specifically.)

  Then replace the `probe` struct and the top-level `present` map inside `checkRemovedKeys`
  (lines 43-69):

  ```go
  	var probe struct {
  		Changelog struct {
  			Remote    any `yaml:"remote"`
  			Generator any `yaml:"generator"`
  			Config    any `yaml:"config"`
  		} `yaml:"changelog"`
  		Commits struct {
  			RemoteMetadata any `yaml:"remote_metadata"`
  		} `yaml:"commits"`
  		Release struct {
  			Platforms any `yaml:"platforms"`
  			Notes     struct {
  				Generator any `yaml:"generator"`
  				Config    any `yaml:"config"`
  			} `yaml:"notes"`
  		} `yaml:"release"`
  		Environments map[string]struct {
  			Changelog struct {
  				Remote    any `yaml:"remote"`
  				Generator any `yaml:"generator"`
  				Config    any `yaml:"config"`
  			} `yaml:"changelog"`
  			Release struct {
  				Platforms any `yaml:"platforms"`
  				Notes     struct {
  					Generator any `yaml:"generator"`
  					Config    any `yaml:"config"`
  				} `yaml:"notes"`
  			} `yaml:"release"`
  		} `yaml:"environments"`
  	}
  	if err := yaml.Unmarshal(raw, &probe); err != nil {
  		return nil // malformed YAML surfaces from the strict parse with better context
  	}
  	present := map[string]bool{
  		"changelog.remote":        probe.Changelog.Remote != nil,
  		"commits.remote_metadata": probe.Commits.RemoteMetadata != nil,
  		"release.platforms":       probe.Release.Platforms != nil,
  		"changelog.generator":     probe.Changelog.Generator != nil,
  		"changelog.config":        probe.Changelog.Config != nil,
  		"release.notes.generator": probe.Release.Notes.Generator != nil,
  		"release.notes.config":    probe.Release.Notes.Config != nil,
  	}
  ```

  Then extend the per-env loop (lines 75-82) to also probe the two new per-env paths:

  ```go
  	for _, env := range slices.Sorted(maps.Keys(probe.Environments)) {
  		envProbe := probe.Environments[env]
  		if envProbe.Changelog.Remote != nil {
  			return fmt.Errorf("%w: `environments.%s.changelog.remote` — replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)", ErrRemovedConfigKey, env)
  		}
  		if envProbe.Release.Platforms != nil {
  			return fmt.Errorf("%w: `environments.%s.release.platforms` — %s", ErrRemovedConfigKey, env, releasePlatformsHintPerEnv)
  		}
  		if envProbe.Changelog.Generator != nil {
  			return fmt.Errorf("%w: `environments.%s.changelog.generator` — %s", ErrRemovedConfigKey, env, generatorRemovedHint)
  		}
  		if envProbe.Changelog.Config != nil {
  			return fmt.Errorf("%w: `environments.%s.changelog.config` — %s", ErrRemovedConfigKey, env, configKeyRemovedHint)
  		}
  		if envProbe.Release.Notes.Generator != nil {
  			return fmt.Errorf("%w: `environments.%s.release.notes.generator` — %s", ErrRemovedConfigKey, env, generatorRemovedHint)
  		}
  		if envProbe.Release.Notes.Config != nil {
  			return fmt.Errorf("%w: `environments.%s.release.notes.config` — %s", ErrRemovedConfigKey, env, configKeyRemovedHint)
  		}
  	}
  	return nil
  }
  ```

  (Renamed the loop variable's per-iteration lookups to `envProbe := probe.Environments[env]` to
  avoid repeating the map index six times — a pure readability change, same behavior as the
  original two-lookup version.)

- [ ] **Step 5: Run tests to verify they pass**

  Run: `go test ./internal/config/... -run TestLoad_RemovedKey -v`

  Expected: PASS — all `TestLoad_RemovedKeys` subtests (including the four new ones),
  `TestLoad_RemovedKeys_PerEnvChangelogRemote`, `TestLoad_RemovedKeys_PerEnvGenerator`,
  `TestLoad_RemovedKeys_PerEnvConfig`, `TestLoad_RemovedKey_ChangelogRemoteHintMentionsNativeOnly`,
  `TestLoad_RemovedKey_ReleasePlatforms`.

  Now run the full `internal/config` package:

  Run: `go test ./internal/config/... 2>&1 | tail -5`

  Expected: a large number of FAILs — every existing fixture/inline-YAML test using `generator:` or
  `config:` now hits the new removed-key error. **This is expected and is Task 2's job, not this
  task's.** Confirm the failures are all inside `internal/config` (run `go test ./... 2>&1 | grep -v ^ok`
  to check nothing outside this package broke) and skim a handful to confirm they're all the new
  removed-key error (`errors.Is(err, config.ErrRemovedConfigKey)` failing at `mustLoad`/`config.Load`),
  not a different kind of breakage, before moving on.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/config/loader.go internal/config/migration_test.go
  git commit -m "feat(config): reject generator:/config: keys, native is now implicit

  changelog.generator, changelog.config, release.notes.generator, and
  release.notes.config (plus their per-env variants) are now removed
  keys, matching the ErrRemovedConfigKey mechanism T160/T163 built for
  changelog.remote/release.platforms. Once there is exactly one
  possible generator, the key carries zero information.

  This intentionally breaks ~30 existing tests using generator:/config:
  as inline-YAML filler — fixed in the very next task (T178).

  Roadmap: docs/tasks/native-generator-roadmap.md -> T177"
  ```

  This commit is expected to leave `go test ./internal/config/...` red. That's fine — proceed to
  Task 2 immediately in the same session; do not stop here.

---

### Task 2 (T178): Fix collateral test damage from T177

**Files:**
- Modify: `internal/config/validator_test.go` (~28 test functions — 17 deleted, 11 edited)
- Modify: `internal/config/loader_test.go` (3 test functions edited, 1 verified unchanged)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — pure test-suite repair, no production code touched in this task.

- [ ] **Step 1: Confirm current locations**

  Line numbers below were read at planning time and will have shifted slightly if anything else in
  these files changed since. Before editing, run:

  ```bash
  grep -n "^func Test" internal/config/validator_test.go | grep -iE "generator|ticket|rendering|tagpattern|collectsAllErrors|disableChangelog|disableNotes|nativePerEnv"
  grep -n "^func Test" internal/config/loader_test.go
  ```

  and use the output to locate each function named below (don't assume the line numbers in this
  plan are still exact).

- [ ] **Step 2: Delete these 17 test functions from `internal/config/validator_test.go`**

  Each of these tests exercises a generator-required/enum/gate/switch behavior that is being
  removed later in this plan (Tasks 4-6) — the test can never pass again once that removal lands,
  and can't even reach its own assertion today because `mustLoad`/`config.Load` now errors first
  (Task 1). Delete each function in full (signature through closing brace):

  1. `TestValidate_changelogMissingGenerator`
  2. `TestValidate_changelogInvalidGenerator`
  3. `TestValidate_releaseNotesInvalidGenerator`
  4. `TestValidate_perEnvChangelogInheritsGenerator`
  5. `TestValidate_perEnvChangelogNoGeneratorAnywhere`
  6. `TestValidate_TicketsNonGitCliffGenerator`
  7. `TestValidate_NativeGenerator`
  8. `TestValidate_TicketsNativeGeneratorOK`
  9. `TestValidate_RenderingTemplatesRequiresNative`
  10. `TestValidate_DriverTemplateRequiresNative`
  11. `TestValidate_NativePerEnvAccepted`
  12. `TestValidate_changelogTagPatternRequiresGitCliff`
  13. `TestValidate_GeneratorCocogittoRejected`
  14. `TestValidate_changelogTagPatternGitCliffValid`
  15. `TestValidate_releaseNotesTagPatternRequiresGitCliff`
  16. `TestValidate_perEnvTagPatternInheritsGitCliff`
  17. `TestValidate_perEnvTagPatternGeneratorSwitchRejected`

  If any section comment (e.g. `// ── native generator ──...`) is left with no tests under it
  after these deletions, remove the now-empty section comment too.

- [ ] **Step 3: Edit these 11 test functions in `internal/config/validator_test.go`**

  Each of these tests some *other* behavior and only needs its `generator:` filler removed.

  **`TestValidate_collectsAllErrors`** — remove the `  generator: bad-gen` line from the inline
  YAML, and remove the `assert.NotNil(t, findErr(errs, "changelog.generator"))` line. Leave
  `assert.GreaterOrEqual(t, len(errs), 3, "expected at least 3 errors")` as-is — it's still true
  (version + versioning.strategy + forges[0].platform = 3 remaining errors).

  **`TestValidate_disableChangelogAndChangelogOverride`** — replace the line
  `      generator: git-cliff` with `      output: CHANGELOG.md` (keeps the per-env `changelog:`
  override block non-empty so the "unreachable" contradiction check still has something to flag).

  **`TestValidate_disableNotesAndNotesOverride`** — replace the line `        generator: git-cliff`
  with `        tag_pattern: "v[0-9]*"` (same reasoning, for the `release.notes` override block).

  **`TestValidate_TicketsValid`** — remove the two lines `changelog:` and `  generator: git-cliff`
  entirely (the whole `changelog:` block). Ticket validation no longer depends on a changelog block
  existing — its sibling tests `TestValidate_TicketsInvalidRegex` and
  `TestValidate_TicketsURLMissingPlaceholder` already have no `changelog:` block at all; match them.

  **`TestValidate_RenderingTemplatesNativeValid`**, **`TestValidate_RenderingTemplatesBadSnippet`**,
  **`TestValidate_RenderingTemplatesUnknownBlock`**, **`TestValidate_RenderingTemplatesHyphenatedBlockValid`**
  — in each, remove the two lines `changelog:` and `  generator: native` entirely. `rendering.templates`
  validation doesn't read `cfg.Changelog` at all (confirm by reading `validateRendering` in
  `internal/config/validator.go` before editing, if you want to double check) — the block was only
  ever there to satisfy the generator-required check Task 1 already made unreachable.

  **`TestValidate_DriverTemplateFileMissing`** — remove just the line `  generator: native`, keep
  `  template: .config/heraut/does-not-exist.tmpl` (this one can't drop the whole block — the test
  is specifically about `.template`).

  **`TestValidate_NativeTagPatternAccepted`** — remove just the line `  generator: native`, keep
  `  tag_pattern: "^v.*-prod$"`.

  **`TestValidate_NativeTagPatternInvalidRegex`** — remove just the line `  generator: native`,
  keep `  tag_pattern: "["`.

- [ ] **Step 4: Edit `internal/config/loader_test.go`**

  **`TestLoadFromReader_withChangelog`** — remove the line `  generator: git-cliff` from `src`, and
  remove the line `assert.Equal(t, "git-cliff", cfg.Changelog.Generator)`.

  **`TestLoadFromReader_withRelease`** — remove the line `    generator: git-cliff` from `src`, and
  remove the line `assert.Equal(t, "git-cliff", cfg.Release.Notes.Generator)`.

  **`TestLoadFromReader_DefaultsChangelogOutput`** — two subtests in its `tests` table:
  - `"empty string"`: remove the line `  generator: git-cliff`, keep `  output: ""`.
  - `"omitted field"`: replace the two lines
    ```
    changelog:
      generator: git-cliff
    ```
    with the single line `changelog: {}` (explicit empty flow-mapping — parses to a non-nil,
    zero-valued `*ContentDriver`, which is what this subtest needs to test "output defaults when
    omitted"; a bare `changelog:` with nothing indented under it parses to YAML `null`, which would
    make `cfg.Changelog` nil and break the test's `require.NotNil(t, cfg.Changelog)` a few lines
    down for the wrong reason).

  **`TestLoadFromReader_rejectsRemovedRemoteAPIURLKey`** — **no change needed.** Its `src` combines
  `changelog.remote` (still first in `removedKeys`' table order) with `generator: git-cliff`; it
  already asserts the `changelog.remote` error specifically, which still fires first. Confirm this
  by running it in isolation in Step 5 rather than skipping it.

- [ ] **Step 5: Run tests to verify they pass**

  Run: `go test ./internal/config/... 2>&1 | tail -60`

  Expected: PASS for everything in `validator_test.go` and `loader_test.go`. Remaining failures (if
  any) are in `merge_test.go` and `schema_test.go` — out of scope for this task, handled by Tasks 6
  and 7. Confirm the failure count/location matches that expectation before moving on (if
  `validator_test.go` or `loader_test.go` still has a red test, you missed one of the 30 —
  re-run `grep -rn "generator:" internal/config/validator_test.go internal/config/loader_test.go`
  and it should now report zero matches).

- [ ] **Step 6: Commit**

  ```bash
  git add internal/config/validator_test.go internal/config/loader_test.go
  git commit -m "test(config): fix collateral damage from the generator:/config: cutover

  ~30 existing tests used generator:/config: as inline-YAML filler and
  broke the instant T177 rejected those keys at load time. Deleted the
  17 whose entire premise (generator-required/enum/gate/switch
  behavior) is being removed in this same phase (T180-T182); edited
  the 11 testing unrelated behavior to drop the now-illegal filler
  line.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T178"
  ```

---

### Task 3 (T179): Empty `Generator` builds native end-to-end

**Files:**
- Modify: `internal/app/pipeline.go:470-493` (`buildGenerator`), `:497-505` (`usesNative`)
- Modify: `internal/app/pipeline_test.go` (new test), `internal/app/forge_internal_test.go`
  (`TestUsesNative` new case)

**Interfaces:**
- Consumes: `config.ContentDriver.Generator` (existing field, now always `""` after Task 1 for any
  config that loads)
- Produces: nothing new — `buildGenerator`/`usesNative` stay unexported, same signatures.

- [ ] **Step 1: Write the failing tests**

  Add to `internal/app/pipeline_test.go`, right after `TestBuildPipeline_WithCommunique` (which
  ends around line 53):

  ```go
  // TestBuildPipeline_EmptyGeneratorBuildsNative pins T179: once generator: is a removed key
  // (T177), every ContentDriver that loads has Generator == "" — this must build a native
  // generator, not fail with "unsupported generator".
  func TestBuildPipeline_EmptyGeneratorBuildsNative(t *testing.T) {
  	mr := exectest.NewMockRunner()
  	cfg := semverCfg()
  	cfg.Changelog = &config.ContentDriver{Output: "CHANGELOG.md"} // no Generator set
  	p, err := app.BuildPipeline(mr, cfg, defaultResolver, defaultOpts)
  	require.NoError(t, err)
  	assert.NotNil(t, p)
  }
  ```

  Add to `internal/app/forge_internal_test.go`'s `TestUsesNative` table (in the `tests` slice,
  after the `"native changelog"` row):

  ```go
  		{name: "empty generator counts as native (T179)", drivers: []*config.ContentDriver{{}}, want: true},
  ```

- [ ] **Step 2: Run tests to verify they fail**

  Run: `go test ./internal/app/... -run 'TestBuildPipeline_EmptyGeneratorBuildsNative|TestUsesNative' -v`

  Expected: `TestBuildPipeline_EmptyGeneratorBuildsNative` FAILs with an error containing
  `unsupported generator ""`. `TestUsesNative/empty_generator_counts_as_native_(T179)` FAILs:
  `usesNative` returns `false`, `want` is `true`.

- [ ] **Step 3: Implement**

  In `internal/app/pipeline.go`, in `buildGenerator` (around line 470), change:

  ```go
  	case "native":
  ```

  to:

  ```go
  	case "", "native":
  		// Empty is the only value Generator ever takes once generator: is a removed key
  		// (T177) — git-cliff/communique below are unreachable in practice but kept until
  		// Phase B deletes their packages and this whole switch collapses to native only.
  ```

  In `usesNative` (around line 498), change:

  ```go
  func usesNative(drivers ...*config.ContentDriver) bool {
  	for _, d := range drivers {
  		if d != nil && d.Generator == "native" {
  			return true
  		}
  	}
  	return false
  }
  ```

  to:

  ```go
  // usesNative reports whether either content driver is configured for the native generator —
  // the forge is only consumed there (gitcliff/communique dispatch remotes themselves). Empty
  // counts as native (T179): generator: is a removed key (T177), so every driver that loads has
  // Generator == "" — the empty check will become the only check once Phase B removes the
  // git-cliff/communique cases from buildGenerator.
  func usesNative(drivers ...*config.ContentDriver) bool {
  	for _, d := range drivers {
  		if d != nil && (d.Generator == "" || d.Generator == "native") {
  			return true
  		}
  	}
  	return false
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `go test ./internal/app/... -run 'TestBuildPipeline_EmptyGeneratorBuildsNative|TestUsesNative' -v`

  Expected: PASS.

  Then run the full `internal/app` package:

  Run: `go test ./internal/app/... 2>&1 | tail -30`

  Expected: PASS — every existing `internal/app` test still sets `Generator: "native"` or
  `Generator: "git-cliff"` explicitly via struct literals (not YAML), so Task 1's loader change
  doesn't affect them, and this task's `case "", "native"` / `d.Generator == ""` additions are
  purely additive (nothing that matched before stops matching).

- [ ] **Step 5: Commit**

  ```bash
  git add internal/app/pipeline.go internal/app/pipeline_test.go internal/app/forge_internal_test.go
  git commit -m "feat(app): empty ContentDriver.Generator builds native

  Once generator: is a removed key (T177), every driver that loads has
  Generator == \"\" — buildGenerator and usesNative now treat that the
  same as generator: native explicitly. The git-cliff/communique
  switch cases are unreachable in practice now but kept until Phase B
  deletes their packages.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T179"
  ```

---

### Task 4 (T180): validator — drop generator-required/enum + `tag_pattern` generator gate

**Files:**
- Modify: `internal/config/validator.go:13-34` (`validGenerators` removal), `:521-559`
  (`validateContentDriver`)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — `validateContentDriver` keeps its existing signature
  `func(d *ContentDriver, path string) []ValidationError`.

Task 2 already deleted every test whose assertion depended on this behavior
(`TestValidate_changelogMissingGenerator`, `TestValidate_changelogInvalidGenerator`,
`TestValidate_releaseNotesInvalidGenerator`, `TestValidate_GeneratorCocogittoRejected`,
`TestValidate_changelogTagPatternRequiresGitCliff`, `TestValidate_changelogTagPatternGitCliffValid`,
`TestValidate_releaseNotesTagPatternRequiresGitCliff`) — this task is pure code removal against an
already-green suite, verified by re-running the suite at the end, not by a new RED/GREEN pair.

- [ ] **Step 1: Implement — remove the generator-required/enum checks**

  In `internal/config/validator.go`, remove the `validGenerators` map entirely from the `var (...)`
  block (lines 13-34):

  ```go
  	validGenerators = map[string]bool{
  		"native": true, "git-cliff": true, "communique": true,
  	}
  ```

  Replace `validateContentDriver` (lines 521-559 — read the file first to get its exact current
  body, since Task 3 didn't touch this file but confirm nothing else shifted it):

  ```go
  func validateContentDriver(d *ContentDriver, path string) []ValidationError {
  	if d == nil {
  		return nil
  	}
  	var errs []ValidationError
  	if d.Generator == "" {
  		errs = append(errs, ValidationError{
  			Path:    path + ".generator",
  			Message: "required",
  			Hint:    "set generator to one of: native, git-cliff, communique",
  		})
  	} else if !validGenerators[d.Generator] {
  		errs = append(errs, ValidationError{
  			Path:    path + ".generator",
  			Message: fmt.Sprintf("%q is not a valid generator", d.Generator),
  			Hint:    "valid generators: native, git-cliff, communique",
  		})
  	}
  	if d.TagPattern != "" && d.Generator != "" &&
  		!strings.EqualFold(d.Generator, "git-cliff") && !strings.EqualFold(d.Generator, "native") {
  		errs = append(errs, ValidationError{
  			Path:    path + ".tag_pattern",
  			Message: "tag_pattern requires the git-cliff or native generator",
  			Hint:    fmt.Sprintf("set generator to git-cliff or native, or remove tag_pattern (current generator: %s)", d.Generator),
  		})
  	}
  	// With native, tag_pattern is a Go regex applied in-process; validate it compiles.
  	if d.TagPattern != "" && strings.EqualFold(d.Generator, "native") {
  		if _, err := regexp.Compile(d.TagPattern); err != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".tag_pattern",
  				Message: fmt.Sprintf("invalid regex: %v", err),
  				Hint:    "with generator: native, tag_pattern is a Go regex (e.g. ^v.*-prod$)",
  			})
  		}
  	}
  	errs = append(errs, validateContentDriverTemplates(d, path)...)
  	return errs
  }
  ```

  with:

  ```go
  func validateContentDriver(d *ContentDriver, path string) []ValidationError {
  	if d == nil {
  		return nil
  	}
  	var errs []ValidationError
  	// tag_pattern is a Go regex applied in-process by the (only) generator, native; validate it
  	// compiles. No generator gate needed — native is the only generator (T177/T180).
  	if d.TagPattern != "" {
  		if _, err := regexp.Compile(d.TagPattern); err != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".tag_pattern",
  				Message: fmt.Sprintf("invalid regex: %v", err),
  				Hint:    "tag_pattern is a Go regex (e.g. ^v.*-prod$)",
  			})
  		}
  	}
  	errs = append(errs, validateContentDriverTemplates(d, path)...)
  	return errs
  }
  ```

  After this edit, run `grep -n '"strings"' internal/config/validator.go` and
  `grep -c 'strings\.' internal/config/validator.go` — if the count drops to 0 the `strings` import
  is now unused and `go build` will fail; remove the import in that case. (It likely won't — Task 5
  below still uses `strings` elsewhere in this same file, but check rather than assume, since Task 5
  hasn't landed yet at this point in the sequence.)

- [ ] **Step 2: Run tests to verify nothing regressed**

  Run: `go test ./internal/config/... 2>&1 | tail -40`

  Expected: PASS — Task 2 already removed every test this deletion would otherwise break.

  Run: `go build ./... && go test ./... 2>&1 | tail -20`

  Expected: clean.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/config/validator.go
  git commit -m "refactor(config): drop generator-required/enum validation

  Native is the only generator now (T177 removed the key entirely), so
  there is nothing left to validate: no required check, no enum check,
  no tag_pattern-requires-git-cliff-or-native cross-field check. The
  Go-regex tag_pattern check becomes unconditional. Tests for this
  behavior were already removed in T178.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T180"
  ```

---

### Task 5 (T181): validator — drop template/tickets/rendering generator gates

**Files:**
- Modify: `internal/config/validator.go:54-88` (`validateTickets`), `:366` and `:377-391`
  (`validateRendering`'s call site + `allContentGeneratorsNative`), `:449-463`
  (`ticketsGeneratorSupported`), `:561-602` (`validateContentDriverTemplates`)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new.

Task 2 already deleted every test whose assertion depended on this behavior
(`TestValidate_TicketsNonGitCliffGenerator`, `TestValidate_RenderingTemplatesRequiresNative`,
`TestValidate_DriverTemplateRequiresNative`) — this task is pure code removal against an
already-green suite.

- [ ] **Step 1: Implement — remove the three generator gates**

  In `internal/config/validator.go`, `validateTickets`: replace

  ```go
  // validateTickets checks the ticket-link config: each pattern compiles as a regex, each url
  // is an absolute http(s) URL containing {ticket}, and tickets require the git-cliff generator.
  func validateTickets(cfg *Config) []ValidationError {
  	tickets := cfg.Tickets()
  	if len(tickets) == 0 {
  		return nil
  	}
  	var errs []ValidationError
  	if !ticketsGeneratorSupported(cfg) {
  		errs = append(errs, ValidationError{
  			Path:    "commits.tickets",
  			Message: "ticket linking requires the git-cliff or native generator",
  			Hint:    "set changelog.generator / release.notes.generator to git-cliff or native, or remove tickets",
  		})
  	}
  	for i, t := range tickets {
  ```

  with:

  ```go
  // validateTickets checks the ticket-link config: each pattern compiles as a regex, and each url
  // is an absolute http(s) URL containing {ticket}.
  func validateTickets(cfg *Config) []ValidationError {
  	tickets := cfg.Tickets()
  	if len(tickets) == 0 {
  		return nil
  	}
  	var errs []ValidationError
  	for i, t := range tickets {
  ```

  (the rest of the function — the `for i, t := range tickets` loop body — is unchanged).

  Delete the `ticketsGeneratorSupported` function entirely:

  ```go
  // ticketsGeneratorSupported reports whether every configured top-level content generator is
  // git-cliff (the only generator with a link mechanism). An empty generator (inherits the
  // default) is allowed.
  func ticketsGeneratorSupported(cfg *Config) bool {
  	drivers := []*ContentDriver{cfg.Changelog}
  	if cfg.Release != nil {
  		drivers = append(drivers, cfg.Release.Notes)
  	}
  	for _, d := range drivers {
  		if d != nil && d.Generator != "" && !strings.EqualFold(d.Generator, "git-cliff") && !strings.EqualFold(d.Generator, "native") {
  			return false
  		}
  	}
  	return true
  }
  ```

  In `validateRendering`, find:

  ```go
  	if len(cfg.Rendering.Templates) > 0 && !allContentGeneratorsNative(cfg) {
  ```

  and delete that whole `if` block (read the surrounding function to get its exact span — it ends
  at the block's closing `}`, appending one `ValidationError` about `rendering.templates` requiring
  native).

  Delete the `allContentGeneratorsNative` function entirely:

  ```go
  // allContentGeneratorsNative reports whether every configured content generator is native
  // (the only generator whose output is driven by rendering.templates / the template file).
  // An empty generator (inherits the default) is allowed.
  func allContentGeneratorsNative(cfg *Config) bool {
  	drivers := []*ContentDriver{cfg.Changelog}
  	if cfg.Release != nil {
  		drivers = append(drivers, cfg.Release.Notes)
  	}
  	for _, d := range drivers {
  		if d != nil && d.Generator != "" && !strings.EqualFold(d.Generator, "native") {
  			return false
  		}
  	}
  	return true
  }
  ```

  Replace `validateContentDriverTemplates` (its exact current body — read the file to confirm,
  Task 4 already changed content earlier in this same file so line numbers have shifted):

  ```go
  // validateContentDriverTemplates validates a driver's native template customization (ADR-0037):
  // rendering.templates and the template file require generator: native; each inline snippet parses;
  // the template file, when set, exists and parses.
  func validateContentDriverTemplates(d *ContentDriver, path string) []ValidationError {
  	isNative := strings.EqualFold(d.Generator, "native")
  	hasInline := d.Rendering != nil && len(d.Rendering.Templates) > 0
  	var errs []ValidationError

  	if d.Template != "" && d.Generator != "" && !isNative {
  		errs = append(errs, ValidationError{
  			Path:    path + ".template",
  			Message: "template requires the native generator",
  			Hint:    fmt.Sprintf("set generator to native, or remove template (current generator: %s)", d.Generator),
  		})
  	}
  	if hasInline && d.Generator != "" && !isNative {
  		errs = append(errs, ValidationError{
  			Path:    path + ".rendering.templates",
  			Message: "rendering.templates requires the native generator",
  			Hint:    fmt.Sprintf("set generator to native, or remove rendering.templates (current generator: %s)", d.Generator),
  		})
  	}

  	if hasInline {
  		errs = append(errs, validateTemplateSnippets(d.Rendering.Templates, path+".rendering.templates")...)
  	}
  	if d.Template != "" && (isNative || d.Generator == "") {
  		if b, err := os.ReadFile(d.Template); err != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".template",
  				Message: fmt.Sprintf("template file not readable: %v", err),
  				Hint:    "the path is relative to the project root; check it exists and is readable",
  			})
  		} else if perr := parseTemplateSnippet(string(b)); perr != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".template",
  				Message: fmt.Sprintf("invalid template: %v", perr),
  				Hint:    "the file must be a valid Go text/template",
  			})
  		}
  	}
  	return errs
  }
  ```

  with:

  ```go
  // validateContentDriverTemplates validates a driver's template customization (ADR-0037): each
  // inline rendering.templates snippet parses, and the template file, when set, exists and parses.
  // No generator gate — native is the only generator (T177/T181).
  func validateContentDriverTemplates(d *ContentDriver, path string) []ValidationError {
  	hasInline := d.Rendering != nil && len(d.Rendering.Templates) > 0
  	var errs []ValidationError

  	if hasInline {
  		errs = append(errs, validateTemplateSnippets(d.Rendering.Templates, path+".rendering.templates")...)
  	}
  	if d.Template != "" {
  		if b, err := os.ReadFile(d.Template); err != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".template",
  				Message: fmt.Sprintf("template file not readable: %v", err),
  				Hint:    "the path is relative to the project root; check it exists and is readable",
  			})
  		} else if perr := parseTemplateSnippet(string(b)); perr != nil {
  			errs = append(errs, ValidationError{
  				Path:    path + ".template",
  				Message: fmt.Sprintf("invalid template: %v", perr),
  				Hint:    "the file must be a valid Go text/template",
  			})
  		}
  	}
  	return errs
  }
  ```

  After these edits, check whether `strings` is still used anywhere in `validator.go`
  (`grep -c 'strings\.' internal/config/validator.go`); remove the import if the count is 0.

- [ ] **Step 2: Run tests to verify nothing regressed**

  Run: `go test ./internal/config/... 2>&1 | tail -40`

  Expected: PASS.

  Run: `go build ./... && go test ./... 2>&1 | tail -20`

  Expected: clean.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/config/validator.go
  git commit -m "refactor(config): drop template/tickets/rendering generator gates

  template, rendering.templates, and commits.tickets all required
  generator: native (or git-cliff, for tickets) — now unconditional
  since native is the only generator (T177/T181). Deleted
  allContentGeneratorsNative and ticketsGeneratorSupported, both now
  unreachable-true. Tests for this behavior were already removed in
  T178.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T181"
  ```

---

### Task 6 (T182): merge — drop the generator-switch full-replacement branch

**Files:**
- Modify: `internal/config/merge.go` (full file — small)
- Modify: `internal/config/merge_test.go` (delete one subtest)

**Interfaces:**
- Consumes: nothing new.
- Produces: `MergeContentDriver(base, override *ContentDriver) *ContentDriver` — signature
  unchanged.

- [ ] **Step 1: Delete the now-impossible subtest**

  In `internal/config/merge_test.go`, inside `TestMergeContentDriver`, find and delete the subtest
  whose `t.Run(...)` name is `"different generator is full replace"` (its body sets `base` and
  `override` with different `Generator` values and asserts the override fully replaces the base
  with no field inheritance). Leave every other subtest untouched, even ones setting
  `Generator: "git-cliff"` as filler on an otherwise-unrelated case — the field still exists on the
  struct; only the *switch behavior* is gone.

- [ ] **Step 2: Run tests to confirm the target subtest is gone**

  Run: `go test ./internal/config/... -run TestMergeContentDriver -v`

  Expected: PASS, with the `"different_generator_is_full_replace"` subtest no longer appearing in
  output.

- [ ] **Step 3: Implement**

  Replace the whole of `internal/config/merge.go` with:

  ```go
  package config

  import "maps"

  // MergeContentDriver returns the effective ContentDriver for a per-environment override applied
  // over a top-level base, per ADR-0019: nil base -> override (nothing to inherit); nil override ->
  // base; otherwise field-by-field merge — a non-empty override field wins, an empty field inherits
  // from base. (Prior to T177/T182, a generator switch between base and override triggered a full
  // replacement instead of a field merge; that branch is gone now that Generator can never be
  // non-empty — native is the only generator.)
  //
  // Neither argument is mutated; the result is always a fresh value (or the sole non-nil argument
  // when one side is nil).
  func MergeContentDriver(base, override *ContentDriver) *ContentDriver {
  	if override == nil {
  		return base
  	}
  	if base == nil {
  		return override
  	}
  	merged := *base
  	if override.Output != "" {
  		merged.Output = override.Output
  	}
  	if override.TagPattern != "" {
  		merged.TagPattern = override.TagPattern
  	}
  	if override.Template != "" {
  		merged.Template = override.Template
  	}
  	merged.Rendering = mergeRendering(base.Rendering, override.Rendering)
  	return &merged
  }

  // mergeRendering deep-merges a per-env rendering override over a base: Excludes are replaced
  // wholesale when the override sets them; Templates merge key-by-key (override wins per key,
  // unset keys inherit). A nil side contributes nothing; both nil yields nil.
  func mergeRendering(base, override *Rendering) *Rendering {
  	if override == nil {
  		return base
  	}
  	if base == nil {
  		return override
  	}
  	merged := *base
  	if len(override.Excludes) > 0 {
  		merged.Excludes = override.Excludes
  	}
  	if len(override.Templates) > 0 {
  		templates := make(map[string]string, len(base.Templates)+len(override.Templates))
  		maps.Copy(templates, base.Templates)
  		maps.Copy(templates, override.Templates)
  		merged.Templates = templates
  	}
  	return &merged
  }
  ```

  (Dropped the `Generator != "" && Generator != base.Generator` full-replacement branch and the
  `merged.Generator = ...` / `merged.Config = ...` field-copy lines — both fields still exist on
  the struct, so `merged := *base` still copies whatever zero value they hold; there's just no
  override logic left to write, since they can never be non-empty.)

- [ ] **Step 4: Run tests to verify they pass**

  Run: `go test ./internal/config/... 2>&1 | tail -40`

  Expected: full `internal/config` package PASSES — this is the last task touching
  `merge.go`/`merge_test.go`/`validator.go`/`loader.go` in Phase A.

  If any test outside the ones already handled fails on a `.Generator`/`.Config` assertion tied to
  `MergeContentDriver`'s output specifically, read that failure before changing anything — it means
  Task 1/2's research missed one, and the fix is the same "field still exists, just never
  overridden by merge now" reasoning applied to that specific case.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/config/merge.go internal/config/merge_test.go
  git commit -m "refactor(config): drop MergeContentDriver's generator-switch branch

  A per-env override setting a different generator than the root used
  to fully replace the base instead of field-merging. That behavior is
  gone: Generator can never be non-empty once generator: is a removed
  key (T177), so there is no switch left to detect.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T182"
  ```

---

### Task 7 (T183): schema.json + testdata fixtures go native-only

**Files:**
- Modify: `schema.json:317-352` (the `ContentDriver` definition)
- Modify: `testdata/config/valid/{enrichment-policy,calver,platform-base-url,forge-minimal,semver-per-env,native,rendering-templates,tickets,semver}.yml`
  (9 files — drop every `generator: ...` line)
- Modify: `testdata/config/invalid/rendering_unknown_template_block.yml` (drop its `generator: native`
  line so it purely tests the unknown-template-block scenario it's named for)
- Modify: `internal/config/schema_test.go:73` (relabel the `invalid_generator.yml` reason)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — this is a data/schema-only task.

- [ ] **Step 1: Confirm current failure state**

  Run: `go test ./internal/config/... -run 'TestSchema_ValidFixtures' -v 2>&1 | tail -20`

  Expected: PASS for all fixtures right now — schema.json hasn't changed yet, so `generator:
  git-cliff`/`generator: native` still validate fine against the (unchanged) schema. This step is
  just a baseline; the real RED state comes after Step 2.

- [ ] **Step 2: Implement — update schema.json**

  In `schema.json`, find the `ContentDriver` definition
  (`grep -n '"ContentDriver": {' schema.json`) and replace it — from the opening
  `"ContentDriver": {` through its matching closing `},` — from:

  ```json
      "ContentDriver": {
        "type": "object",
        "required": [
          "generator"
        ],
        "additionalProperties": false,
        "properties": {
          "generator": {
            "type": "string",
            "enum": [
              "native",
              "git-cliff",
              "communique"
            ],
            "description": "Content generator."
          },
          "config": {
            "type": "string",
            "description": "Path to a generator-specific config file."
          },
          "output": {
            "type": "string",
            "description": "Changelog output file path (e.g. \"CHANGELOG.md\")."
          },
          "tag_pattern": {
            "type": "string",
            "description": "Regex selecting this scope's release tags — passed to git-cliff as --tag-pattern (Rust regex) and applied in-process by the native generator (Go regex). Not used by communique. E.g. \"^v[0-9]\" for bare semver or \"^prod/\" for a per-env prefix."
          },
          "template": {
            "type": "string",
            "description": "Path to a full native template file (Go text/template) parsed on top of the built-in blocks: it may redefine the document root and/or any block. native only; parsed last, so it wins over inline rendering.templates for whatever it defines (ADR-0037)."
          },
          "rendering": {
            "$ref": "#/definitions/Rendering",
            "description": "Per-driver rendering overrides (template snippets + excludes), deep-merged over the global rendering block. native only (ADR-0037)."
          }
        }
      },
  ```

  to:

  ```json
      "ContentDriver": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "output": {
            "type": "string",
            "description": "Changelog output file path (e.g. \"CHANGELOG.md\")."
          },
          "tag_pattern": {
            "type": "string",
            "description": "Regex selecting this scope's release tags, applied in-process (Go regex). E.g. \"^v[0-9]\" for bare semver or \"^prod/\" for a per-env prefix."
          },
          "template": {
            "type": "string",
            "description": "Path to a full template file (Go text/template) parsed on top of the built-in blocks: it may redefine the document root and/or any block; parsed last, so it wins over inline rendering.templates for whatever it defines (ADR-0037)."
          },
          "rendering": {
            "$ref": "#/definitions/Rendering",
            "description": "Per-driver rendering overrides (template snippets + excludes), deep-merged over the global rendering block (ADR-0037)."
          }
        }
      },
  ```

  Run: `go test ./internal/config/... -run TestSchema_ValidJSON -v`

  Expected: PASS (still valid JSON).

  Run: `go test ./internal/config/... -run TestSchema_ValidFixtures -v 2>&1 | tail -20`

  Expected: FAIL for the 9 fixtures listed in this task's Files section — each now has a disallowed
  `generator` property.

- [ ] **Step 3: Migrate the 9 valid fixtures**

  For each of these 9 files, remove every line matching `  generator: ...` (2-space indented, under
  either `changelog:` or `release: notes:`) — do not change anything else in the file:

  - `testdata/config/valid/enrichment-policy.yml` — one line, under `changelog:`
  - `testdata/config/valid/calver.yml` — two lines, under `changelog:` and `release: notes:`
  - `testdata/config/valid/platform-base-url.yml` — two lines, under `changelog:` and
    `release: notes:`
  - `testdata/config/valid/forge-minimal.yml` — two lines (verify with
    `grep -n generator testdata/config/valid/forge-minimal.yml` first)
  - `testdata/config/valid/semver-per-env.yml` — two lines, under `changelog:` and
    `release: notes:`
  - `testdata/config/valid/native.yml` — one line, under `changelog:`
  - `testdata/config/valid/rendering-templates.yml` — one line, under `changelog:`
  - `testdata/config/valid/tickets.yml` — one line, under `changelog:`
  - `testdata/config/valid/semver.yml` — two lines, under `changelog:` and `release: notes:`

  After editing each file, re-run `grep -rn "generator:" testdata/config/valid/*.yml` — expect zero
  output.

  Also fix `testdata/config/invalid/rendering_unknown_template_block.yml`: remove its
  `  generator: native` line (under `changelog:`), so the fixture purely tests the unknown
  `rendering.templates` block name it's named for, without incidentally also tripping the new
  generator-removal rule.

- [ ] **Step 4: Relabel the `invalid_generator.yml` schema-test reason**

  `testdata/config/invalid/invalid_generator.yml` (content: `generator: unknown-generator` under
  `changelog:`) stays as-is — do not edit or delete the fixture file. It now demonstrates
  "`generator` is a disallowed key regardless of value" rather than "an enum violation." In
  `internal/config/schema_test.go`'s `TestSchema_InvalidFixtures` table, change:

  ```go
  		{"invalid_generator.yml", "generator enum violation"},
  ```

  to:

  ```go
  		{"invalid_generator.yml", "generator is a disallowed property (removed key, T177)"},
  ```

  (This is a label-only change — the table's `reason` field is documentation for the reader, never
  asserted against in the test body, so this alone doesn't change pass/fail.)

- [ ] **Step 5: Run tests to verify they pass**

  Run: `go test ./internal/config/... 2>&1 | tail -40`

  Expected: full `internal/config` package PASSES — this is the last task touching this package in
  Phase A, so confirm zero failures, not just fewer.

  Run: `go build ./... 2>&1`

  Expected: clean (no compile errors anywhere in the repo — Task 3 already made `internal/app`
  compile against `Generator == ""`, and no other package's compilation depends on schema.json).

  Run: `go test ./... 2>&1 | tail -40`

  Expected: full repo test suite PASSES. If anything outside `internal/config`/`internal/app` still
  fails, it's a fixture this plan's research didn't find — grep the failure's package for
  `generator:`/`.Generator` usage and apply the same fix pattern.

- [ ] **Step 6: Commit**

  ```bash
  git add schema.json testdata/config/valid testdata/config/invalid/rendering_unknown_template_block.yml internal/config/schema_test.go
  git commit -m "feat(schema): drop generator/config from ContentDriver

  generator and config are no longer valid ContentDriver properties —
  native is implicit. Migrated the 9 valid fixtures and one invalid
  fixture that used generator: as filler; invalid_generator.yml stays
  as a disallowed-property example (relabeled, not deleted).

  Roadmap: docs/tasks/native-generator-roadmap.md -> T183"
  ```

---

### Task 8 (T184): `docs/heraut.sample.yml` drops `generator:`/`config:`

**Files:**
- Modify: `docs/heraut.sample.yml` (four locations)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing — this is the last task in Phase A.

- [ ] **Step 1: No automated test — this is a docs-accuracy fix**

  `docs/heraut.sample.yml` isn't itself schema-validated by any existing test (confirm with
  `grep -rln "heraut.sample.yml" internal --include=*_test.go` — expect no hits). Skip the
  RED/GREEN cycle; this task is reviewed by reading the diff, matching this project's precedent for
  docs-only tasks (e.g. T169). Still re-run the full test suite at the end (Step 3) — a stray typo
  in surrounding YAML could still matter if something unexpected does reference this file.

- [ ] **Step 2: Implement**

  In `docs/heraut.sample.yml`, make these four edits:

  **(a)** Around line 102-108 (inside the commented-out per-env example block), replace:

  ```
  #     # changelog override — replace root changelog config for this environment.
  #     # changelog:
  #     #   generator: git-cliff
  #     #   tag_pattern: "^dev/"
  #
  #     # release overrides — independently override notes and/or targets.
  #     # release:
  #     #   notes:
  #     #     generator: git-cliff
  #     #   targets:              # ADR-0043/ADR-0044; replaces the root list entirely for this env
  #     #     - forge: gitlab-saas
  ```

  with:

  ```
  #     # changelog override — replace root changelog config for this environment.
  #     # changelog:
  #     #   tag_pattern: "^dev/"
  #
  #     # release overrides — independently override notes and/or targets.
  #     # release:
  #     #   notes: {}
  #     #   targets:              # ADR-0043/ADR-0044; replaces the root list entirely for this env
  #     #     - forge: gitlab-saas
  ```

  **(b)** Around line 252-260 (the `changelog:` block's leading comments), replace:

  ```yaml
  changelog:
    # generator — required. Tool used to produce the changelog content.
    #   native      recommended; heraut's built-in renderer (no external binary), driven by
    #               the commits / rendering config blocks
    #   git-cliff   uses cliff.toml deep-merged with heraut's built-in default
    #   communique  requires config: pointing to a communique config file
    generator: native

    # config — path to a generator config file (relative to project root).
    #   git-cliff:  optional partial cliff.toml; deep-merged with heraut's built-in default
    #   communique: required
    # config: cliff.toml

    # output — path where the changelog file is written.
    output: CHANGELOG.md
  ```

  with:

  ```yaml
  changelog:
    # output — path where the changelog file is written.
    output: CHANGELOG.md
  ```

  **(c)** A few lines below (same block), replace:

  ```
    # tag_pattern — regex selecting this scope's release tags. Passed to
    # git-cliff as --tag-pattern (Rust regex) and applied in-process by native
    # (Go regex); not used by communique. Handy with prefixed-tag / per-env
    # strategies to keep the generator scoped to one namespace. (Per-env strategies
    # auto-derive this from tag_format — set it only to override.)
  ```

  with:

  ```
    # tag_pattern — regex (Go regexp syntax) selecting this scope's release tags. Handy
    # with prefixed-tag / per-env strategies to keep generation scoped to one namespace.
    # (Per-env strategies auto-derive this from tag_format — set it only to override.)
  ```

  **(d)** Around line 302 (the `release: notes:` block), replace:

  ```yaml
    notes:
      generator: git-cliff
      # config, tag_pattern, template — same semantics as the changelog block above.
      # tag_pattern: "v[0-9]*"
  ```

  with:

  ```yaml
    notes:
      # tag_pattern, template — same semantics as the changelog block above.
      # tag_pattern: "v[0-9]*"
  ```

  Then search for anything this plan's research missed:

  Run: `grep -n "generator:\|config: cliff\|config: comm" docs/heraut.sample.yml`

  Expected after the edits above: no output.

- [ ] **Step 3: Run the full suite one last time to close out Phase A**

  Run: `go test ./... 2>&1 | tail -40`

  Expected: clean.

  Run: `hk check 2>&1 | tail -60`

  Expected: clean (run `hk fix` if any gofmt/lint drift accumulated across the phase's 8 commits).

- [ ] **Step 4: Update the roadmap — close out Phase 2.5's config-cutover tasks**

  In `docs/tasks/native-generator-roadmap.md`, flip all eight task checkboxes from Task 1's
  skeleton (`#### [ ] T177: ...` through `#### [ ] T184: ...`) to `[x]`, and add one consolidated
  completion note after the last one (`T184`), summarizing: the `ErrRemovedConfigKey` extension
  (T177), the ~30-test collateral-damage sweep and why it was needed (T178), the
  `buildGenerator`/`usesNative` compatibility shim and why it's temporary (T179), the
  validator/merge cleanup and which functions were deleted (T180-T182), the schema + 9+1 fixture
  migration (T183), the sample.yml pass (T184), and explicitly note that Phase B (package deletion,
  `heraut cliff` removal, `docs/specs/02`'s "Content generators" section + `docs/specs/05` rewrite)
  is a separate, not-yet-started plan.

  Also update the progress table row — e.g.
  `| Phase 2.5 — remove the git-cliff package (own ADR) | T177–T184 | Config cutover complete; package deletion pending |`.

- [ ] **Step 5: Commit**

  ```bash
  git add docs/heraut.sample.yml docs/tasks/native-generator-roadmap.md
  git commit -m "docs(sample): drop generator:/config: from heraut.sample.yml

  Closes out Phase A (config cutover) of the native-only-generator
  epic: native is implicit everywhere in the sample config now.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T184"
  ```

---

## Self-Review Notes

- **Spec coverage:** every §1 requirement in the design doc (drop the key, not enum-shrink;
  `ErrRemovedConfigKey` mechanism; schema/sample/fixture updates) has a task. §§2/4/5 (package
  deletion, wizard, infra) are explicitly out of scope, deferred to Phase B/C/D plans not yet
  written — this plan covers Phase A only.
- **Placeholder scan:** no TBD/TODO; every step shows exact before/after code, exact file
  locations, or an exact enumerated list of test functions with their precise fix. Task 2's
  original draft used a vague "grep for it" instruction that turned out to miss most of the actual
  blast radius (verified by actually running the greps during planning and finding they returned
  nothing) — replaced with the full, concrete 32-item classification (17 delete / 11 edit in
  validator_test.go, 4 in loader_test.go) derived by reading every affected test function in full.
- **Type/signature consistency:** `buildGenerator`, `usesNative`, `validateContentDriver`,
  `validateContentDriverTemplates`, `MergeContentDriver`, `mergeRendering` all keep their existing
  signatures across every task that touches them — confirmed by re-reading each function's
  signature line in every task that references it.
- **Phase boundary:** confirmed explicitly in the plan header and Global Constraints that
  `ContentDriver.Generator`/`.Config` fields are NOT deleted from the Go struct in this phase (only
  Task 1 through T184 land) — deleting them would cascade into `internal/app/check.go`,
  `internal/app/cliff.go`, `internal/cmd/check.go`, `internal/scaffold/wizard.go`, breaking Phase
  A's "small and self-contained" promise from the design doc. This is the single most important
  scoping decision in this plan and is called out repeatedly (header, constraints, Task 3) so it
  isn't lost by a reader jumping to a middle task.
- **Sequencing correction from the first draft:** originally Tasks "3" and "4" (now Tasks 4 and 5)
  each had their own "find and delete the relevant tests" step, duplicating work and getting the
  actual test locations wrong (the grep patterns used didn't match how the tests actually assert
  errors — via `findErr`/`assert.Contains`, not literal error-message string matches). Consolidated
  all test repair into one dedicated Task 2, immediately after Task 1, so Tasks 4-6's own code
  changes land against an already-green suite and don't need their own test-discovery step.
