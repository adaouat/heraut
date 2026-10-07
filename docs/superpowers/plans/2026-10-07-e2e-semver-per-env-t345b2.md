# E2E SemVer per-env scenarios (T345b2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover `semver-per-env` end to end through the real binary: `bump: auto` per environment, `bump: promote` (including chains), the E001/E002/E003 guards with and without `--force`, `tag_format` shapes, `stay_at_v0` under per-env, `--set-version`/`{build}`, `--env auto` and the per-env branch guard, and per-env config errors.

**Architecture:** One new scenario file on the T345a table runner, plus one harness helper (`Repo.Checkout`) so a scenario can sit on a named branch. No production code changes.

**Tech Stack:** Go 1.27, testify, the e2e harness.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane A, "Per-env promotion"); behaviour source `docs/specs/04-versioning.md` § SemVer per environment.

## Global Constraints

- TDD: write the failing test first and watch it fail. Rows that characterise shipped behaviour are first made to fail by a deliberate mutation of one expectation (restored with an exact-string replace, never `sed 0,/x/`, which does not work on macOS).
- Never `--no-verify`; fix lint through `hk fix`. If hooks look wrong in this shell, commit under the repo's own mise.
- Conventional commits, subject ≤ 72 characters (count it). Trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; never `Claude-Session:`.
- Commits land on `main`; do not push. `e2e/` imports no heraut `internal/` package. Exit codes are literals: config 2, runtime 3, promotion 4.
- Error text assertions use the runner's lower-cased, whitespace-collapsed matching.
- Scope: `semver-per-env` only. Maintenance branches, changelog/release flow and CLI surface are T345b3-b5. Do not assert the exit code of a missing or unknown `--env` (T359).
- Every expected value was produced by the real binary on 2026-10-07. If a row fails, investigate before touching the expectation.

## Review Focus

1. `--force` must bypass E001 and E002 but never E003 (three rows).
2. A pre-release tag in the source environment must never be promoted, and one in the destination's own namespace must never be the bump base (two rows).
3. Another environment's tags must not leak into an environment's version (`dev` must ignore `prod/9.0.0`).
4. `stay_at_v0` must apply to `bump: auto` environments and never to `bump: promote` ones.
5. A branch-guard refusal must be a runtime error naming the expected branch, and `--force` must lift it.

---

### Task 1: `Repo.Checkout`

**Files:**
- Modify: `e2e/harness/repo.go`
- Modify: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: `Repo.git` (T345a).
- Produces: `(*Repo).Checkout(branch string)` — creates and switches to `branch` at HEAD (`git checkout -q -b`). Task 2 relies on it.

- [ ] **Step 1: Write the failing test**

Append to `e2e/harness/harness_test.go`:

```go
func TestRepo_Checkout(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")

	r.Checkout("develop")

	assert.Equal(t, "develop", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))
}
```

Run: `go test ./e2e/harness -run TestRepo_Checkout`
Expected: FAIL to compile, `r.Checkout undefined`.

- [ ] **Step 2: Implement**

Append to `e2e/harness/repo.go`:

```go
// Checkout creates branch at HEAD and switches to it.
func (r *Repo) Checkout(branch string) {
	r.t.Helper()
	r.git("checkout", "-q", "-b", branch)
}
```

Run: `go test -count=1 ./e2e/harness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): add Repo.Checkout to the harness

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Per-env scenarios

**Files:**
- Modify: `e2e/scenario_test.go` (add an optional `branch` field)
- Create: `e2e/semver_per_env_test.go`

**Interfaces:**
- Consumes: `scenario`, `runScenarios`, `step`, `commit`, `tag`, `exitOK/exitConfig/exitRuntime/exitPromotion`, `versionNext`, `at` is not needed; `harness.Repo.Checkout` (Task 1).
- Produces: `scenario.branch string` (empty = stay on `main`).

- [ ] **Step 1: Add the `branch` field (test infrastructure)**

In `e2e/scenario_test.go`, add the field `branch string // checked out before the history is replayed (empty: main)` to `scenario`, and in `runScenarios` after `repo.WriteConfig(tc.config)` add:

```go
			if tc.branch != "" {
				repo.Checkout(tc.branch)
			}
```

Run: `go test -count=1 ./e2e`
Expected: PASS (nothing uses it yet).

- [ ] **Step 2: Write the scenarios**

Create `e2e/semver_per_env_test.go`:

```go
package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

const perEnvCfg = `version: "1"
versioning:
  strategy: semver-per-env
  initial_version: "0.1.0"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`

func envArgs(cmd, env string, extra ...string) []string {
	return append([]string{"version", cmd, "--env", env}, extra...)
}

func TestPerEnv_AutoBump(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "first dev tag uses initial_version", config: perEnvCfg,
			history: []step{commit("feat: a")}, args: envArgs("next", "dev"), wantOut: "dev/0.1.0"},
		{name: "1.9.0 goes to 1.10.0 by SemVer order", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.9.0"), commit("feat: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.10.0"},
		{name: "fix bumps patch", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.4"},
		{name: "breaking commit bumps major", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("feat!: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/2.0.0"},
		{name: "another environment's tags do not leak in", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3", "prod/9.0.0"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.4"},
		{name: "a pre-release in the namespace is not the bump base", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3"), commit("feat: b"), tag("dev/1.3.0-rc.1"), commit("fix: c")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.3.0"},
		{name: "build metadata counts as the release of its core", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.2.3", "dev/1.2.4+77"), commit("fix: z")},
			args:    envArgs("next", "dev"), wantOut: "dev/1.2.5"},
		{name: "version current reads the environment's latest tag", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("current", "dev"), wantOut: "dev/1.0.0"},
		{name: "version current with no tag in the namespace is a runtime error", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("current", "prod"), wantExit: exitRuntime, wantText: []string{"no tags found for \"prod/*\""}},
	})
}

func TestPerEnv_Promotion(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "prod takes dev's latest version", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.0.2"},
		{name: "a pre-release source tag is never promoted", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2", "dev/1.1.0-rc.1")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.0.2"},
		{name: "E003: no source tags", config: perEnvCfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e003", "no dev/* tags exist"}},
		{name: "E003 is not bypassed by --force", config: perEnvCfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod", "--force"),
			wantExit: exitPromotion, wantText: []string{"e003"}},
		{name: "E001: the target tag already exists", config: perEnvCfg,
			history:  []step{commit("feat: a"), tag("dev/1.0.2", "prod/1.0.2")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e001", "prod/1.0.2", "already exists"}},
		{name: "E001 is bypassed by --force", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2", "prod/1.0.2")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.2"},
		{name: "E002: the destination is already ahead", config: perEnvCfg,
			history:  []step{commit("feat: a"), tag("dev/1.0.2", "prod/1.1.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e002", "version regression", "prod/1.1.0"}},
		{name: "E002 is bypassed by --force", config: perEnvCfg,
			history: []step{commit("feat: a"), tag("dev/1.0.2", "prod/1.1.0")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.2"},
		{name: "a chain promotes through the intermediate environment", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  preprod:
    tag_format: "preprod/{version}"
    bump: promote
    source: dev
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: preprod
`,
			history: []step{commit("feat: a"), tag("dev/1.4.0", "preprod/1.3.0")},
			args:    envArgs("next", "prod"), wantOut: "prod/1.3.0"},
		{name: "a chain's last hop has nothing to promote before the middle tag exists", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  preprod:
    tag_format: "preprod/{version}"
    bump: promote
    source: dev
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: preprod
`,
			history:  []step{commit("feat: a"), tag("dev/1.4.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitPromotion, wantText: []string{"e003", "no preprod/* tags exist"}},
	})
}

func TestPerEnv_TagFormats(t *testing.T) {
	bin := harness.Binary(t)
	single := func(format string) string {
		return `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "` + format + `"
    bump: auto
`
	}
	runScenarios(t, bin, nil, []scenario{
		{name: "version first, env last", config: single("{version}/dev"),
			history: []step{commit("feat: a"), tag("1.2.3/dev"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "1.2.4/dev"},
		{name: "version then underscore", config: single("{version}_dev"),
			history: []step{commit("feat: a"), tag("1.2.3_dev"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "1.2.4_dev"},
		{name: "env then underscore", config: single("dev_{version}"),
			history: []step{commit("feat: a"), tag("dev_1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev_1.2.4"},
		{name: "custom prefix", config: single("release/{version}"),
			history: []step{commit("feat: a"), tag("release/1.2.3"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "release/1.2.4"},
		{name: "a shared {env} format applies to every environment", config: `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
  prod:
    bump: promote
    source: dev
`,
			history: []step{commit("feat: a"), tag("dev/2.0.0"), commit("fix: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/2.0.1"},
		{name: "a shared format also promotes", config: `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
  prod:
    bump: promote
    source: dev
`,
			history: []step{commit("feat: a"), tag("dev/2.0.0")},
			args:    envArgs("next", "prod"), wantOut: "prod/2.0.0"},
	})
}

func TestPerEnv_StayAtV0(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
  bump:
    stay_at_v0: true
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`
	runScenarios(t, bin, nil, []scenario{
		{name: "an auto environment holds the major back with a warning", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), commit("feat!: b")},
			args:    envArgs("next", "dev"), wantOut: "dev/0.5.0",
			wantText: []string{"major bump held back by versioning.bump.stay_at_v0", "dev/1.0.0"}},
		{name: "--allow-major lifts it", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), commit("feat!: b")},
			args:    envArgs("next", "dev", "--allow-major"), wantOut: "dev/1.0.0",
			notText: []string{"held back"}},
		{name: "a promote environment is unaffected", config: cfg,
			history: []step{commit("feat: a"), tag("dev/0.4.0"), commit("feat!: b")},
			args:    envArgs("next", "prod"), wantOut: "prod/0.4.0", notText: []string{"held back"}},
	})
}

func TestPerEnv_BuildMetadata(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}+{build}"
environments:
  uat:
    bump: auto
`
	runScenarios(t, bin, nil, []scenario{
		{name: "set-version with a build id fills {build}", config: cfg,
			history: []step{commit("feat: a")},
			args:    envArgs("next", "uat", "--set-version", "7.4.1", "--set-build-id", "158404"),
			wantOut: "uat/7.4.1+158404"},
		{name: "a {build} format without an id is a config error", config: cfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "uat", "--set-version", "7.4.1"),
			wantExit: exitConfig, wantText: []string{"contains {build} but no build id was provided"}},
		{name: "--set-build-id requires --set-version", config: cfg,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "uat", "--set-build-id", "5"),
			wantExit: exitConfig, wantText: []string{"--set-build-id requires --set-version"}},
	})
}

func TestPerEnv_EnvSelectionAndBranchGuard(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    branch: develop
    bump: auto
  prod:
    tag_format: "prod/{version}"
    branch: main
    bump: promote
    source: dev
`
	runScenarios(t, bin, nil, []scenario{
		{name: "--env auto picks the environment linked to the branch", config: cfg, branch: "main",
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("next", "auto"), wantOut: "prod/1.0.0"},
		{name: "--env auto on an unlinked branch asks for --env", config: cfg, branch: "feature/x",
			history:  []step{commit("feat: a"), tag("dev/1.0.0")},
			args:     envArgs("next", "auto"),
			wantExit: exitConfig, wantText: []string{`no env is linked to branch "feature/x"`}},
		{name: "--env auto requires a per-env strategy", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "auto"),
			wantExit: exitConfig, wantText: []string{"--env auto requires a per-env strategy"}},
		{name: "operating an environment from the wrong branch is refused", config: cfg, branch: "develop",
			history:  []step{commit("feat: a"), tag("dev/1.0.0")},
			args:     envArgs("next", "prod"),
			wantExit: exitRuntime, wantText: []string{`must be operated from branch "main"`, "current branch is \"develop\""}},
		{name: "--force lifts the branch guard", config: cfg, branch: "develop",
			history: []step{commit("feat: a"), tag("dev/1.0.0")},
			args:    envArgs("next", "prod", "--force"), wantOut: "prod/1.0.0"},
	})
}

func TestPerEnv_ConfigErrors(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "an ambiguous promotion source is a config error", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  a:
    tag_format: "a/{version}"
    bump: auto
  b:
    tag_format: "b/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
`,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "prod"),
			wantExit: exitConfig, wantText: []string{"multiple auto environments exist; source is ambiguous"}},
		{name: "a tag_format without {version} is a config error", config: `version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/"
    bump: auto
`,
			history:  []step{commit("feat: a")},
			args:     envArgs("next", "dev"),
			wantExit: exitConfig, wantText: []string{"must contain {version}"}},
	})
}
```

Note: the `--env auto` rows need the error exit codes shown (2). They were observed with the binary; `--env auto` mismatch exits 2 and the unlinked-branch message is `No env is linked to branch "feature/x" — pass --env explicitly.`

- [ ] **Step 3: Run, see the table can fail**

Run: `go test -count=1 ./e2e -run 'TestPerEnv_'`
Expected: PASS. Then break one expectation to prove the table can fail: replace the exact string `wantOut: "dev/1.10.0"` with `wantOut: "dev/1.100.0"` (Python exact replace), run `go test -count=1 ./e2e -run TestPerEnv_AutoBump`, confirm the failure shows `actual  : "dev/1.10.0"`, restore with an exact replace, and `grep -c '1.100.0' e2e/semver_per_env_test.go` must print 0.

- [ ] **Step 4: Full suite, lint, commit**

Run: `go test -count=1 ./... && hk fix && hk check`
Expected: all pass.

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover semver-per-env bumps, promotion guards and branch rules

Per-env auto bumps (SemVer order, namespace isolation, pre-release and
build-metadata handling), promotion with E001/E002/E003 and --force,
chained sources, every tag_format shape, stay_at_v0 on auto but not
promote environments, {build} handling, --env auto and the branch guard.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Close T345b2 and file T359

**Files:**
- Modify: `docs/tasks/roadmap.md`

**Interfaces:**
- Consumes: results of Task 2 (row count).
- Produces: T345b2 marked done; T359 filed.

- [ ] **Step 1: Close T345b2 and add T359**

In the T345b list change `- \`[ ]\` **T345b2**:` to `- \`[x]\` **T345b2**:` and append, indented under it, a completion note with real facts: the number of new rows (`go test -count=1 -v ./e2e 2>&1 | grep -c -- '--- PASS: .*/'` minus 72), that `Repo.Checkout` was added, and that no heraut defect surfaced other than T359. Update the Phase 60 status row to mention T345b2. Then add, after the T358 block and before `### Phase 61`:

```markdown
#### `[ ]` T359: an unknown or missing `--env` exits Runtime (3) instead of Config (2)

Found while preparing T345b2. With a per-env strategy, `heraut version next` (no `--env`) and
`heraut version next --env nope` fail with `Environment "…" not found in config` and exit **3**
(runtime), while `--env auto` mistakes (unlinked branch, non-per-env strategy) exit **2**. Spec 01
reserves 3 for binary/token/network/git failures and 2 for configuration problems, and the missing
or misspelled environment is a usage/config problem. Decide the intended code (2 or 1), fix the
resolver error classification (`perenv.Resolver` / `app.current`), and add the two rows to
`e2e/semver_per_env_test.go` (they were left out of T345b2 on purpose so a test does not cement the
current code).
```

- [ ] **Step 2: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): close T345b2 and file T359

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane A "Per-env promotion": first tag in an env, promotion from source env, E001/E002/E003 each with and without `--force`, `tag_format` with `{env}`/`{build}`; stay_at_v0 under per-env deferred from T345a): first tag, SemVer order, promotion, chain, E001/E002/E003 ± force, all four formats plus shared `{env}`, `{build}`, stay_at_v0, branch guard and `--env auto`, config errors. The two `--env` exit-code rows are deliberately held back for T359.

**Type consistency:** `Repo.Checkout(branch string)` (Task 1) is used only through `scenario.branch` (Task 2). `envArgs`, `perEnvCfg` are defined in the new file and used only there; `semverCfg` comes from T345a.

**Placeholders:** the Task 3 note is filled from measured counts at execution time.
