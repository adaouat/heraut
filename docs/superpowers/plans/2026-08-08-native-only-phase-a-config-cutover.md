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
`config.Load`/`config.LoadFromReader`, used by `internal/config`'s own `mustLoad` test helper AND by
`internal/cmd`'s `writeConfig`/`executeRoot` test helpers. Roughly 30 existing tests in
`internal/config` (`validator_test.go`, `loader_test.go`) and 17 more in `internal/cmd`
(`check_test.go`, `cliff_test.go`, `release_test.go`, `changelog_test.go`) build their fixture YAML
inline with a `generator:`/`config:` line as filler — every one of them breaks the instant Task 1
lands, whether or not they have anything to do with generators. **Task 2 lands in two slices, 2a
(`internal/config`) and 2b (`internal/cmd`)** — both under the single T178 roadmap slot — so Tasks
3-6 (the actual validator.go/merge.go logic changes) each start from a green suite. (The
`internal/cmd` half was discovered during Task 1's own task review, not during the original planning
pass — this note was added then, along with Task 2b itself; see Task 2a's amendment note for the
full story.)

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `github.com/santhosh-tekuri/jsonschema/v6` (schema
tests), `testify` (assert/require).

## Global Constraints

- TDD: write the failing test first, watch it fail, then implement (`.claude/rules/testing.md`,
  `.claude/rules/claude.md`).
- Every commit must be a conventional commit; run `hk fix` if a hook reports a lint failure, never
  bypass hooks (`.claude/rules/workflow.md`).
- `go test ./...` and `hk check` must be clean after **every** task's commit — no task leaves the
  build or test suite broken, even temporarily. (Task 1 is the one narrow exception, explicitly
  called out in its own commit step, and is fixed by Tasks 2a+2b immediately after in the same
  session — run the full `go test ./...` at the end of 2b, not just `internal/config`, to confirm
  nothing is still red.)
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
  Step 7 immediately in the same session; do not stop here.

- [ ] **Step 7 (plan amendment, added mid-execution): remove validator.go's "required" check — this
  cannot wait for Task 4**

  This step exists because Step 6's commit, landed alone, deadlocks every commit in this repository.
  `internal/cmd/commit.go`'s `newCommitVerifyCmd` — this repo's own `heraut commit verify`, which
  runs as the project's `commit-msg` hook (`.config/hk/config.pkl`) — calls `config.Load` **and then
  `config.Validate`**, failing the whole command on any validation error. `.config/heraut.yml` (this
  repo's own dogfooded config) has `generator: native` under both `changelog:` and `release.notes:`.
  After Step 6 alone:
  - `generator:` present → `config.Load` rejects it (Step 6's new removed-key check).
  - `generator:` absent → `config.Validate` still rejects it (`validateContentDriver`'s pre-existing
    "required" check, untouched by Step 6, not scheduled for removal until Task 4).

  There is no valid state for `.config/heraut.yml` in between — which means no commit in this
  repository can pass its own hook until this is fixed. The original plan deferred removing the
  "required" check to Task 4 on the theory that the loader change (Step 6) and the validator change
  were independent; they are not — for any *real* config going through the full
  `Load`-then-`Validate` path (not just the `Load`-only path this plan's own tests use), they are two
  halves of one atomic change and must land together. Task 4 still removes the *enum* check
  (`validGenerators`) and the `tag_pattern` generator-gate later — those don't block an *absent*
  generator, only a *present-but-invalid* one, so they can still wait.

  - [ ] **Step 7a: Write the failing test**

    In `internal/config/validator_test.go`, delete `TestValidate_changelogMissingGenerator` (its
    exact opposite assertion — that a missing `generator:` is a "required" error — is what this step
    removes) and replace it with:

    ```go
    // TestValidate_changelogAbsentGeneratorIsValid pins the T177 follow-up (Step 7): once
    // generator: is a removed key, an absent generator must not also be a validator error — native
    // is implicit. This specific test exists ahead of T180's broader cleanup because a real config
    // going through Load-then-Validate (e.g. this repo's own .config/heraut.yml, loaded by
    // `heraut commit verify`, this project's commit-msg hook) has no valid state otherwise.
    func TestValidate_changelogAbsentGeneratorIsValid(t *testing.T) {
    	cfg := mustLoad(t, `
    version: "1"
    versioning:
      strategy: semver
    changelog:
      output: CHANGELOG.md
    `)
    	errs := config.Validate(cfg)
    	assert.Nil(t, findErr(errs, "changelog.generator"))
    }
    ```

    Run: `go test ./internal/config/... -run TestValidate_changelogAbsentGeneratorIsValid -v`

    Expected: FAIL — `findErr` returns a non-nil `"required"` error.

  - [ ] **Step 7b: Implement**

    In `internal/config/validator.go`, find `validateContentDriver` and replace:

    ```go
    	if d.Generator == "" {
    		errs = append(errs, ValidationError{
    			Path:    path + ".generator",
    			Message: "required",
    			Hint:    "set generator to one of: native, git-cliff, communique",
    		})
    	} else if !validGenerators[d.Generator] {
    ```

    with:

    ```go
    	// Empty is valid — native is implicit (T177). Only a present-but-unknown value errors;
    	// validGenerators and this whole branch are fully removed in T180, once nothing can ever
    	// set Generator to a non-empty value at all.
    	if d.Generator != "" && !validGenerators[d.Generator] {
    ```

    (The `else` disappears along with the `if`; the following `errs = append(...)` block for the
    "not a valid generator" message is unchanged, now under the single `if`.)

  - [ ] **Step 7c: Run tests to verify they pass**

    Run: `go test ./internal/config/... -run TestValidate_changelogAbsentGeneratorIsValid -v`

    Expected: PASS.

    Run: `go test ./internal/config/... 2>&1 | tail -10`

    Expected: the failure count drops by exactly one relative to Step 6's closing run (only
    `TestValidate_changelogMissingGenerator` is gone, replaced by the new test) — everything else is
    still the expected ~30 removed-key failures Task 2a fixes next.

  - [ ] **Step 7d: Fix `.config/heraut.yml`**

    This repo's own config still has the now-invalid `generator:` keys. Edit
    `.config/heraut.yml`:
    - Remove the line `  generator: native` from under `changelog:` (keep `output:` and anything
      else under `changelog:`).
    - Under `release:`, change
      ```yaml
      release:
        notes:
          generator: native
      ```
      to
      ```yaml
      release:
        notes: {}
      ```
      (keep `notes:` present as an empty mapping, not absent — an absent `notes:` key disables
      release-notes generation entirely, since `internal/app/pipeline.go` treats `cfg.Release.Notes
      == nil` as "no release-notes driver configured"; that would be a silent functional regression
      for this repo's own releases, not just a config-key cleanup).

    Verify: `go run ./cmd/heraut check config` (from the repo root) reports the config valid, and
    `echo "chore: test" | go run ./cmd/heraut commit verify` succeeds.

  - [ ] **Step 7e: Run the full suite and `hk check`**

    Run: `go test ./... 2>&1 | grep -v ^ok` — expect only `internal/config` and `internal/cmd`
    failures (the known, still-pending Task 2a/2b collateral damage) — nothing new.

    Run: `hk check 2>&1 | tail -30` — fix anything flagged (`hk fix`).

  - [ ] **Step 7f: Update the roadmap and commit**

    In `docs/tasks/native-generator-roadmap.md`: change the line
    ```
    #### `[ ]` T178: fix collateral test damage from T177 (`internal/config`)
    ```
    to two lines
    ```
    #### `[ ]` T178a: fix collateral test damage from T177 (`internal/config`)

    #### `[ ]` T178b: fix collateral test damage from T177 (`internal/cmd`)
    ```
    (This reflects the Task 2a/2b split documented in this plan file itself — Task 1's original
    scope didn't anticipate `internal/cmd`'s own collateral damage; see Task 2a's amendment note.)

    Flip T177's own checkbox to `[x]` and add a short completion note covering both the original
    loader.go/migration_test.go work (Steps 1-6) and this atomicity fix (Step 7) — name the
    `heraut commit verify` / `.config/heraut.yml` deadlock explicitly, since it's the reason Task 4's
    scope shrank (the "required" check moved here) and it's non-obvious from the code alone.

    ```bash
    git add internal/config/validator.go internal/config/validator_test.go .config/heraut.yml docs/tasks/native-generator-roadmap.md
    git commit -m "fix(config): drop the generator-required check — can't wait for T180

    heraut commit verify (this repo's own commit-msg hook) calls
    config.Validate, not just config.Load. Between T177's removed-key
    rejection and T180's planned enum-check removal, no config could
    satisfy both a present generator: (rejected) and an absent one
    (still \"required\") — deadlocking every commit in this repository,
    including this one, until fixed. The loader and validator halves of
    this change are not independently stageable for any config that
    goes through the full Load-then-Validate path; only the enum check
    and tag_pattern gate (which don't fire on an absent generator) can
    still wait for T180.

    Also fixes .config/heraut.yml, this repo's own dogfooded config,
    which still had the now-removed generator: keys.

    Roadmap: docs/tasks/native-generator-roadmap.md -> T177"
    ```

---

### Task 2a (T178a): Fix collateral test damage from T177 — `internal/config`

> **Plan amendment (mid-execution):** Task 1's implementer over-scoped into `internal/config/validator.go`
> and `.config/heraut.yml` (neither in Task 1's Files list) while chasing test failures, and Task 1's
> own review surfaced a second gap this plan had missed entirely: ~17 more `generator:` occurrences
> across 4 files in `internal/cmd` break the same way once T177 lands, and no task in the original
> plan owned them. Rather than renumber every task after T178 (the roadmap skeleton already committed
> by Task 1 names T177–T184), this single slot splits into 2a (`internal/config`, this task —
> unchanged from the original plan text below) and 2b (`internal/cmd`, new) — mirroring this
> project's own precedent for landing one task ID in lettered slices (see T160a/T160b in
> `docs/tasks/forge-abstraction-roadmap.md`).

**Files:**
- Modify: `internal/config/validator_test.go` (16 deleted, 11 edited, 1 more row dropped — see
  Step 4.5)
- Modify: `internal/config/loader_test.go` (3 test functions edited, 1 verified unchanged)
- Modify: `internal/config/validator.go` (one line — see Step 4.5; added mid-execution, not the
  original scope)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — mostly test-suite repair; the one production-code line (Step 4.5a) doesn't
  change any function's signature.

- [ ] **Step 1: Confirm current locations**

  Line numbers below were read at planning time and will have shifted slightly if anything else in
  these files changed since. Before editing, run:

  ```bash
  grep -n "^func Test" internal/config/validator_test.go | grep -iE "generator|ticket|rendering|tagpattern|collectsAllErrors|disableChangelog|disableNotes|nativePerEnv"
  grep -n "^func Test" internal/config/loader_test.go
  ```

  and use the output to locate each function named below (don't assume the line numbers in this
  plan are still exact).

- [ ] **Step 2: Delete these 16 test functions from `internal/config/validator_test.go`**

  Each of these tests exercises a generator-required/enum/gate/switch behavior that is being
  removed later in this plan (Tasks 4-6) — the test can never pass again once that removal lands,
  and can't even reach its own assertion today because `mustLoad`/`config.Load` now errors first
  (Task 1). Delete each function in full (signature through closing brace). (Note: this list was
  originally 17 items; `TestValidate_changelogMissingGenerator` was already deleted — and replaced
  with `TestValidate_changelogAbsentGeneratorIsValid` — by Task 1's own Step 7, added mid-execution
  to fix a commit-hook deadlock. If you still find the old test present, delete it now; don't touch
  its replacement.)

  1. `TestValidate_changelogInvalidGenerator`
  2. `TestValidate_releaseNotesInvalidGenerator`
  3. `TestValidate_perEnvChangelogInheritsGenerator`
  4. `TestValidate_perEnvChangelogNoGeneratorAnywhere`
  5. `TestValidate_TicketsNonGitCliffGenerator`
  6. `TestValidate_NativeGenerator`
  7. `TestValidate_TicketsNativeGeneratorOK`
  8. `TestValidate_RenderingTemplatesRequiresNative`
  9. `TestValidate_DriverTemplateRequiresNative`
  10. `TestValidate_NativePerEnvAccepted`
  11. `TestValidate_changelogTagPatternRequiresGitCliff`
  12. `TestValidate_GeneratorCocogittoRejected`
  13. `TestValidate_changelogTagPatternGitCliffValid`
  14. `TestValidate_releaseNotesTagPatternRequiresGitCliff`
  15. `TestValidate_perEnvTagPatternInheritsGitCliff`
  16. `TestValidate_perEnvTagPatternGeneratorSwitchRejected`

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

  **Deviation, discovered during implementation, confirmed correct:** `TestLoadFromReader_withRelease`
  needs one more line than the brief above states — removing just `generator: git-cliff` leaves a
  bare `notes:` key (parses to YAML `null`, same pitfall as `DefaultsChangelogOutput`'s "omitted
  field" case). Apply the same fix: `notes: {}` instead of a bare `notes:` key.

- [ ] **Step 4.5 (plan amendment, added mid-execution): two more fixes this task's own scope
  surfaced**

  Two of Task 2a's own 30 assigned fixes exposed problems the original plan didn't anticipate.
  Both are small, both are squarely within (or directly adjacent to and required by) this task's
  work — fix them here rather than leaving Task 2a red or inventing yet another task letter.

  **(a) `internal/config/validator.go:545` has the same "doesn't treat empty Generator as native"
  bug Task 1's Step 7 already fixed once, in a sibling check.** After editing
  `TestValidate_NativeTagPatternInvalidRegex` per Step 3 above (removing `generator: native`, keeping
  `tag_pattern: "["`), it still fails — `find Err` returns nil where the test expects an invalid-regex
  error. The cause: the tag_pattern regex-compile check requires `Generator` to be **literally**
  `"native"`:

  ```go
  	// With native, tag_pattern is a Go regex applied in-process; validate it compiles.
  	if d.TagPattern != "" && strings.EqualFold(d.Generator, "native") {
  ```

  Since `Generator` can now only ever be `""`, this branch is permanently unreachable — tag_pattern
  regex validation is silently disabled for every config, right now, on `main`. This is the same
  bug class as the "required" check Task 1's Step 7 fixed (a generator-gate that never learned empty
  means native), just a sibling check three lines further down that Step 7 didn't touch. It can't
  wait for T180 for the same reason Step 7 couldn't: it's a live, silent validation gap affecting any
  real config right now. Fix:

  ```go
  	// With native, tag_pattern is a Go regex applied in-process; validate it compiles.
  	if d.TagPattern != "" && (d.Generator == "" || strings.EqualFold(d.Generator, "native")) {
  ```

  This is the only change to make in `validator.go` — do not touch the enum check or the
  `tag_pattern`-requires-git-cliff-or-native gate above it; both stay exactly as they are for T180.
  After this fix, also re-verify `TestValidate_NativeTagPatternAccepted` (the sibling positive-case
  test) — it was passing before for the wrong reason (the branch was skipped, not because the regex
  was judged valid); confirm it still passes now that the branch actually runs.

  **(b) `TestValidate_invalidFixtures`'s `invalid_generator.yml` row.** This table-driven test (in
  `validator_test.go`, not `schema_test.go` — a different test from the one Task 7/T183 relabels)
  calls `config.Load(tc.fixture)` with `require.NoError(t, err, "fixture should load without parse
  error")`, then checks a semantic-validation error. Its row for
  `testdata/config/invalid/invalid_generator.yml` now fails at the `Load` step (removed-key error)
  before ever reaching `Validate` — the whole scenario the row exercises (a config that loads fine
  but fails semantic *generator-enum* validation) no longer exists once `generator:` is rejected at
  load time. Delete that one row from the table (find it via
  `grep -n "invalid_generator.yml" internal/config/validator_test.go` — it's in a different function
  from the schema-level test of a similar name). Leave every other row in that table untouched.

  **Explicitly out of scope for this task (confirmed, not overlooked):** `TestLoad_fromFixtures` and
  `TestValidate_validFixtures` (both in this task's own files) will stay red until Task 7 (T183)
  migrates the `testdata/config/valid/*.yml` fixtures they load — no code in `validator_test.go` or
  `loader_test.go` can fix them, only the external fixture files can, and that's T183's job. Do not
  attempt to fix them here; Step 5 below accounts for this.

- [ ] **Step 5: Run tests to verify they pass**

  Run: `go test ./internal/config/... 2>&1 | tail -60`

  Expected: every test in `validator_test.go` and `loader_test.go` passes **except**
  `TestLoad_fromFixtures` and `TestValidate_validFixtures`, which stay red until T183 lands (see
  Step 4.5) — that's expected, not a miss. Remaining failures outside these two files (in
  `merge_test.go`, `schema_test.go`, `loader_forge_test.go`, `shipped_examples_test.go`) are out of
  scope for this task, handled by Tasks 6/7/8. Confirm the failure set matches this expectation
  before moving on — re-run `grep -rn "generator:" internal/config/validator_test.go
  internal/config/loader_test.go` and it should report exactly one match (the untouched
  `TestLoadFromReader_rejectsRemovedRemoteAPIURLKey`).

- [ ] **Step 6: Commit**

  ```bash
  git add internal/config/validator_test.go internal/config/loader_test.go internal/config/validator.go
  git commit -m "test(config): fix collateral damage from the generator:/config: cutover

  ~30 existing tests used generator:/config: as inline-YAML filler and
  broke the instant T177 rejected those keys at load time. Deleted the
  16 whose entire premise (generator-enum/gate/switch behavior) is
  being removed in this same phase (T180-T182) — one more,
  TestValidate_changelogMissingGenerator, was already handled by
  T177's own Step 7; edited the 11 testing unrelated behavior to drop
  the now-illegal filler line; dropped one more row from
  TestValidate_invalidFixtures whose scenario no longer exists.

  Also fixes a second validator.go generator-gate that never learned
  empty means native (the tag_pattern regex-compile check) — same bug
  class as T177's Step 7, silently disabling regex validation for
  every config until now.

  TestLoad_fromFixtures and TestValidate_validFixtures stay red until
  T183 migrates their fixtures.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T178a"
  ```

---

### Task 2b (T178b): Fix collateral test damage from T177 — `internal/cmd`

**Files:**
- Modify: `internal/cmd/check_test.go` (10 occurrences across 10 functions — 8 deleted, 2 edited)
- Modify: `internal/cmd/cliff_test.go` (4 occurrences across 4 functions — all 4 deleted)
- Modify: `internal/cmd/release_test.go` (1 occurrence, 1 function edited)
- Modify: `internal/cmd/changelog_test.go` (2 occurrences across 2 functions — both edited)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — pure test-suite repair, no production code touched in this task.

These tests all go through `writeConfig`/`executeRoot`, which load real YAML through `config.Load` —
the same removed-key check Task 1 added. The `heraut cliff` command and `heraut check`'s `cliff`
subsection are both git-cliff-specific features slated for wholesale deletion in Phase B (a separate,
not-yet-written plan) — most of the deletions below are tests that can no longer be reached with any
valid config, not tests being weakened.

- [ ] **Step 1: Confirm current locations**

  ```bash
  grep -n "^func Test" internal/cmd/check_test.go internal/cmd/cliff_test.go internal/cmd/release_test.go internal/cmd/changelog_test.go
  ```

  Line numbers below were accurate at the time this task was added to the plan (mid-execution, after
  Task 1's review) but may have shifted.

- [ ] **Step 2: Delete these 8 test functions from `internal/cmd/check_test.go`**

  Each requires `generator: git-cliff`/`communique` to reach its assertion — none of these scenarios
  can be constructed with a valid config anymore, and all test `heraut check cliff` /
  `heraut check runtime`'s git-cliff-specific behavior, which Phase B removes entirely:

  1. `TestCheckRuntime_GeneratorMissing`
  2. `TestCheckCliffChangelog_Valid`
  3. `TestCheckCliffChangelog_Invalid`
  4. `TestCheckCliffChangelog_NotGitCliff`
  5. `TestCheckCliff_WithChangelog_Valid`
  6. `TestCheckCliff_WithChangelog_Invalid`
  7. `TestCheckCliffReleaseNotes_Valid`
  8. `TestCheckCliffReleaseNotes_Invalid`

  Leave `TestCheckCliff_ConfigNotFound`, `TestCheckCliff_NoGeneratorsConfigured`,
  `TestCheckCliffReleaseNotes_ConfigNotFound`, `TestCheckCliffReleaseNotes_NotConfigured` untouched —
  none of them set `generator:`, and `TestCheckCliff_NoGeneratorsConfigured`'s "no git-cliff
  generators configured" assertion is now the universal case for `heraut check cliff` (bare), not a
  special one — still correct, still worth keeping until Phase B deletes the command.

- [ ] **Step 3: Edit 2 test functions in `internal/cmd/check_test.go`**

  **`TestCheckRuntime_AllGood`** — remove the line `  generator: git-cliff` from the inline YAML
  (leave `forges:`/`release:` as-is). This test's purpose is the overall `check runtime` happy path,
  not git-cliff specifically — `assert.Contains(t, out, "git")` doesn't depend on which content
  generator is configured.

  **`TestCheckAll_PassesAll`** — remove the line `  generator: git-cliff` from the inline YAML
  (leave `changelog:` present with no other change). Same reasoning — this is the overall `heraut
  check` composite happy path.

- [ ] **Step 4: Delete these 4 test functions from `internal/cmd/cliff_test.go`**

  All 4 need `generator: git-cliff`/`communique` to reach their assertion; `heraut cliff` prints the
  *effective* git-cliff config regardless of what's configured (its own doc comment: "if driver is
  nil or has no generator set, the embedded default TOML is returned") — so once `generator:
  git-cliff` can't be configured, these 4 become redundant with `TestCliffChangelog_NoChangelogConfigured_PrintsDefault`
  / `TestCliffReleaseNotes_NotConfigured_PrintsDefault`, which stay:

  1. `TestCliffChangelog_WithGitCliff_PrintsTOML`
  2. `TestCliffChangelog_BuildFormat_ShowsPostprocessor`
  3. `TestCliffChangelog_NotGitCliff_Error`
  4. `TestCliffReleaseNotes_WithGitCliff_PrintsTOML`

  Leave `TestCliffCmd_Structure` (structural, no config), `TestCliffChangelog_NoChangelogConfigured_PrintsDefault`,
  and `TestCliffReleaseNotes_NotConfigured_PrintsDefault` untouched.

- [ ] **Step 5: Edit `internal/cmd/release_test.go`**

  **`TestRelease_NoPlatforms_Error`** — replace the two lines
  ```
  changelog:
    generator: git-cliff
    output: CHANGELOG.md
  ```
  with
  ```
  changelog:
    output: CHANGELOG.md
  ```
  This test is about the "no resolvable publish destination" error, unrelated to which generator
  would have produced the changelog.

- [ ] **Step 6: Edit `internal/cmd/changelog_test.go`**

  **`TestChangelog_DryRun_OutputsVersion`** and **`TestChangelog_DryRun_NoPush`** — in each, replace
  ```
  changelog:
    generator: git-cliff
    output: CHANGELOG.md
  ```
  with
  ```
  changelog:
    output: CHANGELOG.md
  ```
  Both tests assert on the resolved version string and dry-run/no-push messaging, never on changelog
  content — confirmed by the FakeBin `git` script in each test having no `git-cliff` entry at all, so
  the generator binary was never actually *invoked* even before this change.

  > **Correction (plan amendment, added mid-execution):** the reasoning above about the binary never
  > being invoked is correct but incomplete — it doesn't mean the *generator object* is never
  > constructed. `internal/app/pipeline.go`'s `buildGenerator` runs unconditionally, `--dry-run` or
  > not, and its `switch driver.Generator` has no case for `""` until Task 3 (T179) adds one. **These
  > two tests will not pass until Task 3 lands** — this was discovered by Task 2b's own implementer,
  > who correctly stopped rather than guess a fix in `internal/app` (forbidden for this task) or
  > commit a suite that wasn't green. **Dispatch/land Task 3 (T179) before Step 7 below**, even though
  > it appears later in this document — the two tasks' actual dependency runs the opposite direction
  > from their reading order. Once T179 lands, these two tests need no further edit; they'll simply
  > pass.

- [ ] **Step 7: Run tests to verify they pass (after Task 3/T179 has landed — see the correction
  above)**

  Run: `go test ./internal/cmd/... 2>&1 | tail -60`

  Expected: PASS.

  Run: `go test ./... 2>&1 | grep -v ^ok`

  Expected: **not fully clean, and that's expected** — Task 1's original brief predicted this check
  would show nothing, but that was wrong (see the corrections accumulated across T177/T178a's own
  execution). At this point in the plan, the only remaining failures should be: `internal/scaffold`
  and `internal/cmd/init_test.go`'s `TestInitCmd_DefaultsProducesValidConfig` (Task 2c/T178c, not yet
  dispatched), and in `internal/config`: `TestLoad_fromFixtures`, `TestValidate_validFixtures`,
  `TestLoad_ForgesAndTargets`, and `TestShippedExamples_LoadAndValidate` (all four blocked on Task
  7/T183's fixture migration — including `internal/config/testdata/forge-minimal.yml`, a
  **second, package-local copy** of the fixture Task 7's original file list names only once, at
  `testdata/config/valid/forge-minimal.yml` — see Task 7's own amendment note). If anything else
  appears, stop and report it rather than assuming it's one of these.

- [ ] **Step 8: Commit**

  ```bash
  git add internal/cmd/check_test.go internal/cmd/cliff_test.go internal/cmd/release_test.go internal/cmd/changelog_test.go
  git commit -m "test(cmd): fix collateral damage from the generator:/config: cutover

  12 tests exercising heraut cliff / heraut check cliff's git-cliff-
  specific behavior can no longer be reached with any valid config
  (both features are removed wholesale in the separate, not-yet-
  written Phase B plan) — deleted. 5 more used generator: as inert
  filler on tests about something else — stripped the line. Two of
  those five (the changelog dry-run tests) only pass once T179 lands
  (buildGenerator didn't yet treat an empty Generator as native) —
  landed out of reading order for exactly that reason.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T178b"
  ```

---

### Task 2c (T178c): Fix `heraut init` — it now generates configs it can't load

> **Plan amendment (mid-execution):** discovered while fixing Task 1's own deadlock (its Step 7):
> `internal/scaffold/generate.go` marshals a real `config.Config` via `yaml.Encoder`, and
> unconditionally sets `Generator: a.ChangelogGenerator` / `Generator: a.NotesGenerator` on the
> emitted `ContentDriver` values whenever the wizard's generator prompt was answered (which it is by
> default — `ChangelogGenerator`/`NotesGenerator` default to `"git-cliff"`). This is not test
> collateral like 2a/2b — it's a real product regression: **`heraut init`, run today, produces a
> `.heraut.yml` that fails to load on the very next `heraut` invocation.** Unlike 2a/2b, this task
> touches production code, not just tests — a third letter under the same T178 roadmap slot, for the
> same reason as before (avoid renumbering every later task).

**Files:**
- Modify: `internal/config/config.go` (one yaml tag)
- Modify: `internal/scaffold/generate.go` (drop 2 field assignments)
- Modify: `internal/scaffold/generate_test.go` (one test, swap its passthrough-field example — see
  Step 2.5; added mid-execution, not the original scope)

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — `GenerateYAML`/`answersToConfig` keep their existing signatures.

The wizard's generator-choice prompt itself (`internal/scaffold/wizard.go`) is untouched here — it
still asks "Changelog generator" / "Release notes generator" with git-cliff/communique/none options,
and the project's own Global Constraints for this plan say not to touch `internal/scaffold/`'s
wizard UI. That prompt becoming pointless (its answer no longer reaches the emitted config) is
exactly the gap Phase C (wizard simplification, a separate not-yet-written plan) closes properly, by
removing the question. This task's only job is to stop `heraut init` from producing broken output in
the meantime — the minimum fix, not the redesign.

- [ ] **Step 1: Confirm the current failure**

  Run: `go test ./internal/scaffold/... -v 2>&1 | grep -c '^--- FAIL'`

  Expected: `11` — all 11 fail with `Received unexpected error: removed config key:
  changelog.generator` (or `release.notes.generator`, or the per-env variant), because every one of
  these tests calls `scaffold.GenerateYAML(...)` and then round-trips the result through
  `config.LoadFromReader`/`config.Load`. None of the 11 need their own assertions changed — they
  will pass again once the production fix below lands, because the round-trip will start succeeding.
  Confirm this by reading two or three of them (e.g. `TestGenerateYAML_SemVer` in
  `internal/scaffold/generate_test.go`, `TestDroppedFields_DefaultsConfig` in
  `internal/scaffold/dropped_test.go`) — each sets `ChangelogGenerator`/`NotesGenerator` on the
  `Answers` struct (the wizard's *internal* answer representation, not the emitted YAML), generates,
  reloads, and asserts no error — nothing in any of the 11 asserts on the literal string `"generator"`
  appearing in the output.

- [ ] **Step 2: Implement**

  In `internal/config/config.go`, in the `ContentDriver` struct, change:

  ```go
  	Generator  string `yaml:"generator"`
  ```

  to:

  ```go
  	Generator  string `yaml:"generator,omitempty"`
  ```

  (`.Config`'s tag already has `omitempty` — this was a pre-existing inconsistency between the two
  fields, not something T177 introduced. Without this, any future code that marshals a zero-value
  `ContentDriver` — not just `internal/scaffold` — would emit a literal `generator: ""`, which is
  just as rejected as `generator: native`, since the removed-key probe checks *presence*, not value.)

  In `internal/scaffold/generate.go`'s `answersToConfig`, remove the `Generator:` field from both
  `ContentDriver` literals:

  ```go
  	if a.ChangelogGenerator != "" {
  		output := a.ChangelogOutput
  		if output == "" {
  			output = "CHANGELOG.md"
  		}
  		cfg.Changelog = &config.ContentDriver{
  			Generator: a.ChangelogGenerator,
  			Output:    output,
  		}
  	}
  ```

  becomes:

  ```go
  	if a.ChangelogGenerator != "" {
  		output := a.ChangelogOutput
  		if output == "" {
  			output = "CHANGELOG.md"
  		}
  		cfg.Changelog = &config.ContentDriver{
  			Output: output,
  		}
  	}
  ```

  and:

  ```go
  		if hasNotes {
  			cfg.Release.Notes = &config.ContentDriver{
  				Generator: a.NotesGenerator,
  			}
  		}
  ```

  becomes:

  ```go
  		if hasNotes {
  			cfg.Release.Notes = &config.ContentDriver{}
  		}
  ```

  Leave the `if a.ChangelogGenerator != ""` / `hasNotes := a.NotesGenerator != ""` conditions
  themselves unchanged — they still gate *whether* a `changelog:`/`release.notes:` block is emitted
  at all (a real, still-meaningful choice: "do you want a changelog"), just no longer which generator
  string ends up inside it.

- [ ] **Step 2.5 (plan amendment, added mid-execution): one test doesn't self-resolve**

  10 of the 11 originally-failing tests self-resolve with zero test-file changes, as predicted. One
  doesn't: `TestGenerateYAML_EnvPassthroughFieldsRoundTrip`
  (`internal/scaffold/generate_test.go:338-359`) constructs a per-environment `EnvAnswer.Changelog`/
  `.Release` **directly** (not via the wizard), deliberately populating `Generator: "git-cliff"` /
  `Generator: "communique"` as its example of a field that should survive
  `answersToConfig`'s per-env passthrough (`internal/scaffold/generate.go`'s `Changelog: e.Changelog,
  Release: e.Release` — copied verbatim, unlike the two top-level literals Step 2 above touches).
  Since `Generator` specifically can no longer round-trip by design (that's this whole task's point),
  the test's choice of *which* field to use as its passthrough example is now wrong — not the
  passthrough mechanism itself, which still works correctly for every other field.

  Confirmed this path is unreachable from real `heraut init` usage: every production caller of
  `EnvAnswer.Changelog`/`.Release` (`wizard.ConfigToAnswers`, `wizard.matchEnvSnapshot`) only ever
  copies from a `*config.Config` that already passed `config.Load` — and since `config.Load` rejects
  `generator:` at every nesting level including per-env (T177), no such config can ever carry a
  non-empty `Generator` anywhere. Only a test that constructs the struct directly, bypassing `Load`,
  can hit this. **Do not change the production fix (Step 2) to also strip `Generator` from the
  per-env passthrough** — that would be defense-in-depth for a path this trace confirms is dead, not
  a fix for a live bug, and would touch a third file this task doesn't otherwise need.

  Fix the test itself instead — swap its passthrough-field example from `Generator` to `TagPattern`
  (a field that, unlike `Generator`, is still meaningful and still round-trips):

  ```go
  			Changelog: &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"},
  			Release:   &config.EnvRelease{Notes: &config.ContentDriver{Generator: "communique"}},
  ```

  becomes:

  ```go
  			Changelog: &config.ContentDriver{TagPattern: "prod/changelog/*", Output: "CHANGELOG.md"},
  			Release:   &config.EnvRelease{Notes: &config.ContentDriver{TagPattern: "prod/notes/*"}},
  ```

  and:

  ```go
  	assert.Equal(t, "git-cliff", prod.Changelog.Generator)
  	require.NotNil(t, prod.Release)
  	assert.Equal(t, "communique", prod.Release.Notes.Generator)
  ```

  becomes:

  ```go
  	assert.Equal(t, "prod/changelog/*", prod.Changelog.TagPattern)
  	require.NotNil(t, prod.Release)
  	assert.Equal(t, "prod/notes/*", prod.Release.Notes.TagPattern)
  ```

  The test's actual purpose (proving per-env `Changelog`/`Release` fields survive the
  generate-then-reload round trip verbatim) is unchanged — only the specific field used to prove it.

- [ ] **Step 3: Run tests to verify they pass**

  Run: `go test ./internal/scaffold/... -v 2>&1 | tail -20`

  Expected: all 11 previously-failing tests PASS now — 10 self-resolved by Step 2, 1
  (`TestGenerateYAML_EnvPassthroughFieldsRoundTrip`) by Step 2.5. If any *other* test still fails,
  read its specific assertion — it means this task's research missed another case, and that one test
  needs its assertion updated too, following the same "only touch what actually changed" principle.

  Run: `go test ./... 2>&1 | grep -v ^ok`

  Expected: only the known-pending `internal/config` (Task 2a — but see the note below) and
  `internal/cmd` (Task 2b) failures remain — `internal/scaffold` is fully green. By this point in
  execution, Tasks 2a and 2b may have already landed and left a *smaller*, specific residual set
  (`TestLoad_fromFixtures`, `TestValidate_validFixtures`, `TestLoad_ForgesAndTargets`,
  `TestShippedExamples_LoadAndValidate` — all blocked on Task 7/8's fixture and doc migration) rather
  than the full original ~47-test count; either way, confirm `internal/scaffold` itself shows zero
  failures and nothing appears outside the already-tracked set.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/config/config.go internal/scaffold/generate.go internal/scaffold/generate_test.go
  git commit -m "fix(scaffold): stop heraut init from generating unloadable configs

  answersToConfig unconditionally wrote generator: <choice> into the
  emitted YAML — a real product regression, not just test collateral:
  heraut init, run today, produces a .heraut.yml that fails to load on
  the very next heraut invocation, since generator: is now a removed
  key (T177). Dropped the Generator field from both emitted
  ContentDriver literals; the wizard's generator-choice prompt itself
  is untouched (Phase C removes it properly). Also added omitempty to
  ContentDriver.Generator's yaml tag, matching .Config's existing tag,
  so no future struct-marshal path can reintroduce this class of bug.

  TestGenerateYAML_EnvPassthroughFieldsRoundTrip used Generator as its
  example of a per-env passthrough field, which no longer round-trips
  by design — swapped to TagPattern; confirmed the path it exercised
  (populating per-env Generator directly, bypassing config.Load) is
  unreachable from any real heraut init flow.

  Roadmap: docs/tasks/native-generator-roadmap.md -> T178c"
  ```

  Also update `docs/tasks/native-generator-roadmap.md`: add a third line under the T178 split,
  `#### \`[ ]\` T178c: fix heraut init — it now generates configs it can't load (\`internal/scaffold\`)`,
  alongside the existing T178a/T178b lines.

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

> **Plan amendment (mid-execution):** Two earlier tasks each removed one more piece of
> `validateContentDriver` ahead of schedule, because each could not wait (a real, live bug/deadlock,
> not a preference): Task 1's Step 7 removed the "required" check (fixing a commit-hook deadlock),
> and Task 2a's Step 4.5a fixed the `tag_pattern` regex-compile check's own instance of the same
> "doesn't treat empty Generator as native" bug (it was silently disabling regex validation for every
> config). The "before" code block below reflects `validateContentDriver`'s state **after both**
> fixes, not its original pre-Task-1 state. This task now only removes the *enum* check
> (`validGenerators`) and the `tag_pattern` git-cliff/native **gate** (the first `tag_pattern` `if`
> block, which restricts which generators may use it at all — a different check from the
> regex-compile one Task 2a already fixed) — a smaller diff than originally planned.

Task 2a already deleted every test whose assertion depended on this behavior
(`TestValidate_changelogInvalidGenerator`, `TestValidate_releaseNotesInvalidGenerator`,
`TestValidate_GeneratorCocogittoRejected`, `TestValidate_changelogTagPatternRequiresGitCliff`,
`TestValidate_changelogTagPatternGitCliffValid`, `TestValidate_releaseNotesTagPatternRequiresGitCliff`)
— this task is pure code removal against an already-green suite, verified by re-running the suite at
the end, not by a new RED/GREEN pair.

- [ ] **Step 1: Implement — remove the generator-enum + tag_pattern-gate checks**

  In `internal/config/validator.go`, remove the `validGenerators` map entirely from the `var (...)`
  block (lines 13-34):

  ```go
  	validGenerators = map[string]bool{
  		"native": true, "git-cliff": true, "communique": true,
  	}
  ```

  Replace `validateContentDriver` (read the file first to get its exact current body — Task 1's
  Step 7 already changed it from what an earlier draft of this plan showed here, and Task 3 didn't
  touch this file):

  ```go
  func validateContentDriver(d *ContentDriver, path string) []ValidationError {
  	if d == nil {
  		return nil
  	}
  	var errs []ValidationError
  	// Empty is valid — native is implicit (T177). Only a present-but-unknown value errors;
  	// validGenerators and this whole branch are fully removed in T180, once nothing can ever
  	// set Generator to a non-empty value at all.
  	if d.Generator != "" && !validGenerators[d.Generator] {
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
  	if d.TagPattern != "" && (d.Generator == "" || strings.EqualFold(d.Generator, "native")) {
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
- Modify: `internal/config/testdata/forge-minimal.yml` (a **second, package-local copy** of
  `testdata/config/valid/forge-minimal.yml`, used by `TestLoad_ForgesAndTargets` in
  `internal/config/loader_forge_test.go` — found by Task 2b, not in this plan's original research;
  drop the same two `generator:` lines)
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

  Also fix `internal/config/testdata/forge-minimal.yml` — a separate, package-local copy of the file
  above (not a symlink; a real duplicate with a slightly different header) — remove its two
  `generator:` lines the same way. Verify with
  `go test ./internal/config/... -run TestLoad_ForgesAndTargets -v` (PASS).

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

  Expected: full `internal/config` package PASSES, **including** `TestLoad_fromFixtures` and
  `TestValidate_validFixtures` (in `loader_test.go`/`validator_test.go` respectively) — Task 2a left
  those two red deliberately, since only this task's fixture migration (Step 3) can fix them; they
  should self-resolve here with no test-code changes. Also confirm `TestLoad_ForgesAndTargets`
  (`internal/config/loader_forge_test.go`, not otherwise touched by this plan) now passes — it loads
  `testdata/config/valid/forge-minimal.yml`, already in this task's Step 3 migration list.

  This is the last task touching `internal/config` in Phase A, so confirm zero failures in that
  package, not just fewer.

  Run: `go build ./... 2>&1`

  Expected: clean (no compile errors anywhere in the repo — Task 3 already made `internal/app`
  compile against `Generator == ""`, and no other package's compilation depends on schema.json).

  Run: `go test ./... 2>&1 | tail -40`

  Expected: full repo test suite PASSES, **except** `internal/config/shipped_examples_test.go`'s
  `TestShippedExamples_LoadAndValidate` — its `docs/heraut.sample.yml` subtest self-resolves once
  Task 8 (T184) lands, and its `README.md` subtest needs Task 8's own new step (README.md wasn't in
  this plan at all until that step was added — see Task 8). If anything else outside
  `internal/config`/`internal/app` still fails, it's a fixture this plan's research didn't find —
  grep the failure's package for `generator:`/`.Generator` usage and apply the same fix pattern.

- [ ] **Step 6: Commit**

  ```bash
  git add schema.json testdata/config/valid internal/config/testdata/forge-minimal.yml testdata/config/invalid/rendering_unknown_template_block.yml internal/config/schema_test.go
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
- Modify: `README.md` (two locations — see Step 2.5; added mid-execution, not the original scope)

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

- [ ] **Step 2.5 (plan amendment, added mid-execution): `README.md` also needs fixing**

  Discovered during Task 2a: `internal/config/shipped_examples_test.go`'s
  `TestShippedExamples_LoadAndValidate` extracts every full (`version:`-containing) fenced ` ```yaml `
  block from `README.md` and round-trips it through `config.LoadFromReader` + `config.Validate`. No
  task in this plan's original scope touched `README.md` at all, even though it has the identical
  problem `docs/heraut.sample.yml` had.

  In `README.md`, find the full example config block (`grep -n "generator:" README.md` — expect two
  hits inside a fenced yaml block, plus two more in a prose comparison table further down that are
  NOT inside a loadable block and don't need to change here — confirm which is which before editing;
  the table is prose describing generator choice, in scope for Phase B's docs pass alongside specs
  02/05, not this task).

  In the fenced yaml block, remove the `generator: git-cliff` line under `changelog:` and the
  `generator: git-cliff` line under `release: notes:` (leave `output: CHANGELOG.md` and everything
  else in the block untouched).

  Verify: `go test ./internal/config/... -run TestShippedExamples_LoadAndValidate -v` — expect PASS
  (both the `docs/heraut.sample.yml` subtest, fixed by Step 2 above, and every `README.md` block
  subtest).

- [ ] **Step 3: Run the full suite one last time to close out Phase A**

  Run: `go test ./... 2>&1 | tail -40`

  Expected: clean.

  Run: `hk check 2>&1 | tail -60`

  Expected: clean (run `hk fix` if any gofmt/lint drift accumulated across the phase's 8 commits).

- [ ] **Step 4: Update the roadmap — close out Phase 2.5's config-cutover tasks**

  In `docs/tasks/native-generator-roadmap.md`, flip all remaining task checkboxes (`T178a`/`T178b`/
  `T178c` and `T179` through `T184`; `T177` is already `[x]` with its own note) to `[x]`, and add one
  consolidated completion note after the last one (`T184`), summarizing: the `ErrRemovedConfigKey`
  extension and the validator "required"-check fix forced together by the commit-hook deadlock
  (T177); the three-way collateral-damage split and why (T178a `internal/config` — including a
  second validator.go generator-gate bug found and fixed along the way, and a dropped
  `TestValidate_invalidFixtures` row; T178b `internal/cmd`; T178c `internal/scaffold`, a real
  product regression in `heraut init`, not just tests); the `buildGenerator`/`usesNative`
  compatibility shim and why it's temporary (T179); the validator/merge cleanup and which functions
  were deleted (T180-T182); the schema + fixture migration, including the extra fixes this plan's
  original research missed — `internal/config/loader_forge_test.go`'s `TestLoad_ForgesAndTargets`
  self-resolving via the same fixture list (T183); the sample.yml **and README.md** pass, README
  having no owning task until Task 2a found it (T184). Explicitly note that Phase B (package
  deletion, `heraut cliff` removal, `docs/specs/02`'s "Content generators" section + `docs/specs/05`
  rewrite) is a separate, not-yet-started plan.

  Also update the progress table row — e.g.
  `| Phase 2.5 — remove the git-cliff package (own ADR) | T177–T184 | Config cutover complete; package deletion pending |`.

- [ ] **Step 5: Commit**

  ```bash
  git add docs/heraut.sample.yml README.md docs/tasks/native-generator-roadmap.md
  git commit -m "docs(sample): drop generator:/config: from heraut.sample.yml and README

  Closes out Phase A (config cutover) of the native-only-generator
  epic: native is implicit everywhere in the sample config and the
  README's example now. The README half was missing from this plan's
  original scope — found by Task 2a via
  TestShippedExamples_LoadAndValidate.

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
- **Two corrections made during Task 1's execution (not caught by planning-time self-review):**
  (1) Task 1's own review found `internal/cmd` has 17 more `generator:`/`config:` occurrences across
  4 files that this plan never accounted for — split the T178 slot into 2a/2b rather than renumber
  every later task. (2) Task 1's own implementer, then its fix pass, both independently discovered
  that `internal/cmd/commit.go`'s `newCommitVerifyCmd` (this repo's own `commit-msg` hook) calls
  `config.Validate`, not just `config.Load` — meaning the loader-rejects-the-key change (Task 1) and
  the validator-stops-requiring-it change (originally all of Task 4) are not independently stageable
  for any config on the full Load-then-Validate path, including this repository's own
  `.config/heraut.yml`. Moved the "required" check removal into Task 1 itself (its Step 7); Task 4
  kept only the enum check and `tag_pattern` gate, which don't fire on an absent generator. Neither
  gap was visible from reading the plan's target code in isolation — both only surfaced once a real
  implementer tried to make an actual commit against the actual repository mid-task.
