# Forge Hardening — user-facing fixes (T169–T173) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every user-facing defect the P2/P3 reviews surfaced, before the `heraut init` wizard (P4) codifies the config surface into every new project.

**Architecture:** Four independent fixes, grouped by the layer they touch: config validation (a release that fails after the tag is pushed, plus a misleading migration hint), `heraut check` (a false CI-gate failure for changelog-only users), the GitLab forge (an author handle rendered with spaces in it), and documentation (self-hosted enrichment guidance plus stale ADRs). No shared design decisions between them.

**Tech Stack:** Go, `testify`. No new dependencies.

## Why now (context for every task)

The forge epic replaced `changelog.remote` + `release.platforms` with `forges:` + `release.targets`. P4 redesigns `heraut init` to *generate* that config — so anything wrong or misleading in the config surface now gets baked into every project the wizard touches. These five items are the ones that can bite a real user; internal cleanups (T168, and the rest of T173) are deliberately deferred.

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven preferred.
- **No new Go dependencies.** `internal/config` imports nothing from heraut; `internal/forge/*` imports only `internal/port` (+`internal/config` in the resolver) + stdlib; `internal/app` is the only place constructing concrete implementations.
- **`internal/platforms/**` and `internal/port/platform.go` must not appear in any diff** — the publishing transport (`gh`/`glab`) stays unchanged, as decided in ADR-0044.
- **No real data** — synthetic placeholders only (`gitlab.example.com`, `github.example.com`, `group/subgroup/project`, `acme/widget`, `alice`, `alice@example.com`).
- **No environment leakage** — tests inject `getenv` or use the existing `clearCIEnv(t)` helper; never real env vars.
- Errors wrapped with `%w`; sentinels `errors.Is`-able.
- Never bypass git hooks. Lint via `hk fix` (never `gofmt`/`yamlfmt` directly). Before committing run BOTH `go test ./...` and `GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./...`.
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — with angle brackets, and **never** a `Claude-Session:` line.
- Each task flips its roadmap entry to `[x]` in `docs/tasks/forge-abstraction-roadmap.md` with a one-paragraph completion note.

---

### Task 1 (T171 + T173-hint): reject unsatisfiable target configs, and fix the migration hint

**Files:**
- Modify: `internal/config/validator.go` (`validateTargetForges`)
- Modify: `internal/config/loader.go` (`releasePlatformsHint`)
- Test: `internal/config/validator_forge_test.go`, `internal/config/migration_test.go`

**Interfaces:**
- Consumes: the existing helper, whose real signature is
  `validateTargetForges(targets []Target, knownForges map[string]bool, forgeCount int, pathPrefix string) []ValidationError`
  (it builds per-entry paths as `fmt.Sprintf("%s[%d].forge", pathPrefix, i)`), and the
  `ValidationError{Path, Message, Hint}` shape. It is already called for both the top-level and the
  per-environment target lists — so a rule added here covers both automatically.
- Produces: no new exported symbols.

#### T171 — two bare targets silently resolve to the same destination

`internal/app/pipeline.go`'s `resolveTargetForge` starts with:

```go
if len(cfg.Forges) == 0 {
    if len(resolved.Forges) == 0 { … }
    return config.Forge{}, resolved.Forges[resolved.EnrichmentIndex], nil
}
```

With no `forges:` block, **every** target resolves to the same auto-detected identity. So
`release: targets: [{}, {draft: true}]` builds two drivers pointing at one repository: the first
`release create` succeeds, the second fails — **after the tag has already been pushed**, leaving the
repo tagged with a half-finished release. That is the worst point in the pipeline to fail, and it is
statically detectable.

**Fix — state the rule generally: no two targets may resolve to the same forge.** That covers both
shapes of the bug:

- two targets carrying the **same non-empty** `forge` (e.g. `[{forge: A}, {forge: A}]`) — an explicit
  duplicate the narrower "bare targets" rule would miss; and
- more than one **bare** target when `forgeCount <= 1` — with zero or one forge every bare target
  resolves to the same identity.

(With `forgeCount > 1` a bare target is already rejected by the existing "forge is required when more
than one forge is configured" rule, so those two cases are exhaustive.) Both are expressible from
the helper's existing parameters — track seen `forge` names, and count bare targets against
`forgeCount`. Emit one error at the list path; do not emit one per duplicate entry.

#### T173 — the migration hint omits the required fields

`internal/config/loader.go:23`:

```go
const releasePlatformsHint = "declare a `forges:` entry carrying `base_url` / `token_env` / `repository`-or-`project`, then reference it from `release.targets[].forge`, keeping `draft` / `prerelease` / `assets` on the target"
```

A `forges:` entry **requires** `name` and `platform` (see `validateForges`), so a user following
this literally hits a second round of validation errors immediately after migrating. The per-env
variant additionally needs to say that `forges:` is top-level only — there is no
`environments.<env>.forges`.

**Fix:** rewrite the hint to name `name` and `platform` as required, and add the top-level-only note
to the per-env message. Keep it one sentence per clause and keep the existing backtick style.

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/validator_forge_test.go`:

```go
func TestValidate_UnsatisfiableTargets(t *testing.T) {
	tests := []struct {
		name   string
		forges []config.Forge
		targets []config.Target
		want   string // substring expected in some error; "" = valid
	}{
		{
			name:    "zero-config single bare target is fine",
			forges:  nil,
			targets: []config.Target{{}},
			want:    "",
		},
		{
			name:    "zero-config with two bare targets is unsatisfiable",
			forges:  nil,
			targets: []config.Target{{}, {Draft: true}},
			want:    "release.targets",
		},
		{
			name:    "one forge with two bare targets is unsatisfiable",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}},
			targets: []config.Target{{}, {Draft: true}},
			want:    "release.targets",
		},
		{
			name:    "two forges, each target names one",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}},
			targets: []config.Target{{Forge: "A"}, {Forge: "B"}},
			want:    "",
		},
		{
			name:    "two targets naming the SAME forge is unsatisfiable",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}},
			targets: []config.Target{{Forge: "A"}, {Forge: "A", Draft: true}},
			want:    "release.targets",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Version:    "1",
				Versioning: config.Versioning{Strategy: "semver"},
				Forges:     tc.forges,
				Release:    &config.Release{Targets: tc.targets},
			}
			errs := config.Validate(cfg)
			if tc.want == "" {
				for _, e := range errs {
					assert.NotContains(t, e.Path, "release.targets", "unexpected target error: %v", e)
				}
				return
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Path, tc.want) || strings.Contains(e.Message, tc.want) {
					found = true
				}
			}
			assert.True(t, found, "expected an error mentioning %q, got %v", tc.want, errs)
		})
	}
}
```

Add to `internal/config/migration_test.go`:

```go
// The hint must name every field a forges: entry REQUIRES, or a user following it literally hits a
// second round of validation errors.
func TestLoad_RemovedKey_ReleasePlatformsHintNamesRequiredFields(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`))
	require.Error(t, err)
	require.True(t, errors.Is(err, config.ErrRemovedConfigKey))
	assert.Contains(t, err.Error(), "name", "the hint must name the required `name` field")
	assert.Contains(t, err.Error(), "platform", "the hint must name the required `platform` field")
	assert.Contains(t, err.Error(), "release.targets")
}

// The per-env message must additionally say forges: is top-level only.
func TestLoad_RemovedKey_PerEnvHintSaysForgesIsTopLevel(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      platforms:
        - name: gl
          platform: gitlab
          project: group/subgroup/project
`))
	require.Error(t, err)
	require.True(t, errors.Is(err, config.ErrRemovedConfigKey))
	assert.Contains(t, err.Error(), "top-level")
}
```

Add `strings` to the validator test's imports if absent.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/config/ -run 'TestValidate_UnsatisfiableTargets|TestLoad_RemovedKey_ReleasePlatformsHintNamesRequiredFields|TestLoad_RemovedKey_PerEnvHintSaysForgesIsTopLevel'`
Expected: FAIL — the unsatisfiable cases currently validate clean, and the hint mentions neither `name`/`platform` nor "top-level".

- [ ] **Step 3: Implement**

In `validateTargetForges`, before (or alongside) the existing per-target loop, add the
unsatisfiable-set check described above. Emit one error at the list path (e.g.
`release.targets` / `environments.<env>.release.targets`) with a message naming the problem — more
than one target cannot resolve to distinct destinations without `forges:` entries to name — and a
hint telling the user to declare `forges:` and set `forge:` on each target. Reuse the file's
existing error-construction style.

In `internal/config/loader.go`, rewrite `releasePlatformsHint` so it names `name` and `platform` as
required alongside the optional coordinates, and add the "`forges:` is top-level only" clause to the
per-env message (keep the two messages consistent — a reader hitting either must learn the same
thing).

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Full suites + lint + roadmap + commit**

Flip **T171** to `[x]` with a completion note; note in **T173** that the migration-hint half is done
and the remaining internal cleanups are still open.

```bash
go test ./... && GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./... && hk fix
git add internal/config/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "fix(config): reject unsatisfiable release.targets and fix the migration hint (T171)"
```

---

### Task 2 (T172): `heraut check` must not fail a user who never publishes

**Files:**
- Modify: `internal/app/check.go`
- Test: `internal/app/check_test.go`

**Interfaces:**
- Consumes: `RuntimeCheckItem{Name, Value, Err, IsWarn}` — note `IsWarn` is documented as "advisory only — shown with ! but does not fail the overall check". That is the mechanism this task uses.

**The problem.** `internal/app/check.go` calls `effectiveTargetPlatforms(...)`, and on a resolution
error emits a hard-failing `forge` row. So a user with **no `forges:` and no `release.targets`** —
who may be changelog-only and never publish — gets a failing `heraut check` whenever the ambient
environment is ambiguous (both `GITHUB_TOKEN` and `GITLAB_TOKEN` exported, no CI markers, and an
origin `parseGitOrigin` doesn't recognise, i.e. any self-hosted host). Before the forge work, that
config produced binary-probe warnings only. `heraut check` is commonly a CI gate, so this is a false
failure.

**The rule to implement:** a forge-resolution failure is
- an **error** when the user explicitly asked for a forge — `len(cfg.Forges) > 0` **or** at least one
  effective `release.targets` entry exists (they configured publishing, so failing to resolve it is
  real); and
- a **warning** (`IsWarn: true`) when neither is configured — heraut was only *attempting*
  zero-config detection, and the user may never publish.

Keep the row's message informative in both cases: it must still say what failed and how to fix it
(declare a `forges:` entry), because a warning that says nothing is worse than none.

- [ ] **Step 1: Write the failing test** — `internal/app/check_test.go`

```go
func TestRuntimeCheck_AmbiguousForgeIsWarnWithoutPublishConfig(t *testing.T) {
	clearCIEnv(t)
	t.Setenv("GITHUB_TOKEN", "gh")
	t.Setenv("GITLAB_TOKEN", "gl")

	t.Run("no publish config -> advisory warning", func(t *testing.T) {
		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
		}
		items := runtimeCheckItemsForTest(t, cfg) // see note below
		forgeRow := findItem(items, "forge")
		require.NotNil(t, forgeRow, "a forge row is still reported")
		assert.True(t, forgeRow.IsWarn, "a user who configured no publishing must not fail the check")
		assert.NotEmpty(t, forgeRow.Err, "the row must still explain what failed")
	})

	t.Run("explicit forges -> hard failure", func(t *testing.T) {
		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Forges: []config.Forge{
				{Name: "A", Type: "gitlab"},
				{Name: "B", Type: "github"},
			},
			Release: &config.Release{Targets: []config.Target{{Forge: "A"}}},
		}
		items := runtimeCheckItemsForTest(t, cfg)
		forgeRow := findItem(items, "forge")
		require.NotNil(t, forgeRow)
		assert.False(t, forgeRow.IsWarn, "explicit publish config means a resolution failure is real")
	})
}
```

> **Adapt to the file's existing conventions.** `internal/app/check_test.go` already exercises
> `RuntimeCheck`; use whatever helper/entry point it uses to collect items rather than inventing
> `runtimeCheckItemsForTest`/`findItem` — read the neighbouring tests first and match them. The
> assertions above (`IsWarn` true vs false, message non-empty) are the requirement; the plumbing
> should look like its neighbours.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run TestRuntimeCheck_AmbiguousForgeIsWarnWithoutPublishConfig`
Expected: FAIL — the no-publish-config case currently reports a hard error (`IsWarn` false).

- [ ] **Step 3: Implement**

In `internal/app/check.go`'s `resolveErr != nil` branch, set `IsWarn` based on whether the user
configured publishing (`len(cfg.Forges) > 0 || len(config.EffectiveTargets(cfg, env)) > 0`). Update
the branch's comment to explain the distinction — the existing comment claims the row exists so the
reason `heraut release` will also fail isn't hidden, which is only true when the user actually
publishes.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/app/`
Expected: PASS.

- [ ] **Step 5: Full suites + lint + roadmap + commit**

Flip **T172** to `[x]` with a completion note.

```bash
go test ./... && GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./... && hk fix
git add internal/app/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "fix(app): warn instead of failing check when no publishing is configured (T172)"
```

---

### Task 3 (T170): unify the no-linked-identity author handle on the email local-part

**Files:**
- Modify: `internal/forge/gitlab/gitlab.go` (`gitAuthors`)
- Modify: `docs/specs/05-generators-and-platforms.md` (the rendered-output description)
- Test: `internal/forge/gitlab/gitlab_test.go`

**The problem.** Two forges render the same "no linked platform identity" fallback differently:

- `internal/forge/gitlab/gitlab.go`'s `gitAuthors` prefers the git **author name**, falling back to
  the email local-part. For `Author: "Alice Smith"` it renders `by @Alice Smith` — a space inside an
  `@handle`.
- `internal/forge/azure/azure.go`'s `authorLogin` prefers the email **local-part**, falling back to
  the display name. It renders `by @alice`.

Both describe the same situation; the local-part is the one that reads as a handle. Azure's
behaviour was the reviewed, shipped decision (ADR-0043 / T151), so GitLab moves to match it.

**This changes user-visible changelog output** for GitLab in `api_mode: rest` (the default), so it is
a deliberate behaviour change, not a silent refactor: record it in the roadmap completion note and
update the spec's description of what GitLab REST renders. GitLab `api_mode: graphql` is unaffected
— it resolves a real linked `@username` and never reaches this fallback.

- [ ] **Step 1: Write the failing test** — `internal/forge/gitlab/gitlab_test.go`

The helper is unexported, so this goes in an internal test file (`package gitlab`); check whether
`internal/forge/gitlab/` already has one and follow that convention.

```go
func TestGitAuthors_PrefersEmailLocalPart(t *testing.T) {
	got := gitAuthors([]port.Commit{
		{Hash: "aaa", Author: "Alice Smith", Email: "alice@example.com"}, // local-part wins over the name
		{Hash: "bbb", Author: "Bob", Email: ""},                          // no email → git name
		{Hash: "ccc", Author: "", Email: "carol@example.com"},            // no name → local-part
		{Hash: "ddd", Author: "", Email: ""},                             // nothing → omitted
	})
	assert.Equal(t, map[string]string{
		"aaa": "alice",
		"bbb": "Bob",
		"ccc": "carol",
	}, got, "an @handle must not contain spaces when an email local-part is available")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/forge/gitlab/ -run TestGitAuthors_PrefersEmailLocalPart`
Expected: FAIL — `aaa` currently maps to `"Alice Smith"`.

- [ ] **Step 3: Implement**

Invert the precedence in `gitAuthors`: try the email local-part first, fall back to the git author
name, omit when neither yields a value. Update the doc comment to say why (a handle should not
contain spaces, and this matches the Azure forge's fallback).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/forge/gitlab/`
Expected: PASS — including the pre-existing REST enrichment test that asserts a rendered author. If
that test asserted the git name, update its expectation and say so in your report; do not weaken the
assertion.

- [ ] **Step 5: Update the spec**

In `docs/specs/05-generators-and-platforms.md`, find where GitLab REST-mode author rendering is
described and correct it to "the git author email's local-part (falling back to the git author
name)", matching how Azure's is already described.

- [ ] **Step 6: Full suites + lint + roadmap + commit**

Flip **T170** to `[x]`, noting that only the author-fallback half is done and the Azure
`api_url`/`api_mode` half remains open (or close it fully if you also document that — see the
roadmap entry).

```bash
go test ./... && GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./... && hk fix
git add internal/forge/gitlab/ docs/specs/05-generators-and-platforms.md docs/tasks/forge-abstraction-roadmap.md
git commit -m "fix(forge/gitlab): render the author handle from the email local-part (T170)"
```

---

### Task 4 (T169): document the self-hosted enrichment requirement, and fix ADR drift

**Files:**
- Modify: `docs/specs/05-generators-and-platforms.md`
- Modify: `docs/adr/0034-native-remote-enrichment.md`, `docs/adr/0043-forge-abstraction.md`
- Modify: `docs/adr/README.md`
- Test: none (documentation only)

**This task is documentation only — change no Go code.**

#### The capability gap to document

`internal/forge/detect.go`'s `parseGitOrigin` recognises only the public hosts (`github.com`,
`gitlab.com`, `dev.azure.com`). So a user on a **self-hosted** GitLab or GitHub Enterprise host,
outside CI, resolves **no forge** — enrichment silently degrades under `enrichment_policy: optional`
and hard-errors under `required`. Before the forge epic, such a user enriched via the platform-
derived link context. The fix for them is to declare an explicit `forges:` entry with `base_url` and
`project`/`repository` — but nothing currently tells them that.

Document it in `docs/specs/05-generators-and-platforms.md` where enrichment and its policy are
described: state which hosts auto-detect, what happens when detection fails (degrade vs error per
policy), and the explicit `forges:` remedy. Keep it factual — verify each claim against
`internal/forge/detect.go` and `internal/forge/resolve.go` before writing it.

#### The ADR drift to fix

- `docs/adr/0034-native-remote-enrichment.md` is still `Accepted` while describing `gh api` /
  `glab api` as the enrichment transports — both deleted when GitHub and Azure moved onto
  `port.Forge`. Add a dated update note (the style used by `docs/adr/0039-commit-author-attribution.md`,
  a `> **Update (YYYY-MM-DD):** …` blockquote after the metadata block) recording that the CLI
  transports were replaced by native `net/http` forges, and annotate its `docs/adr/README.md` row.
- `docs/adr/0043-forge-abstraction.md` says "GitHub and Azure keep their current transports
  (`gh api graphql`, …)" — true when written, false now. Add the same kind of update note pointing at
  the phase that changed it.

Do not rewrite either ADR's historical content — they are records of decisions that were correct
when made.

- [ ] **Step 1: Read the ground truth**

Read `internal/forge/detect.go` (`parseGitOrigin`, `detectCIForge`), `internal/forge/resolve.go`
(`resolveAuto`, the ambiguity error), and `internal/generators/native/enrich.go`
(`enrichForRelease`'s policy handling) so every statement you write is verifiable. Where a doc and
the code disagree, the code wins — and say so in your report.

- [ ] **Step 2: Write the spec section**

Add the self-hosted guidance to `docs/specs/05-generators-and-platforms.md`, matching the file's
existing tone, heading depth, and formatting. Synthetic placeholders only.

- [ ] **Step 3: Add the ADR update notes + index annotations**

As described above, for ADR-0034 and ADR-0043, plus their `docs/adr/README.md` rows.

- [ ] **Step 4: Verify and commit**

Run: `go test ./...` (you changed no code — this only confirms nothing drifted) and `hk fix`.
`git diff --stat` must show only files under `docs/`.

Flip **T169** to `[x]` with a completion note.

```bash
git add docs/
git commit -m "docs: document self-hosted enrichment requirement and fix ADR drift (T169)"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS, and again under `GITHUB_ACTIONS=true …`.
- [ ] `hk check` → clean.
- [ ] Transport untouched — with `BASE` = the commit before Task 1, `git diff --stat BASE..HEAD -- internal/platforms/ internal/port/platform.go` must be **empty** (ADR-0044).
- [ ] A config with two bare `release.targets` and no `forges:` is rejected at validation — verify by hand with `heraut check config`, since the failure it prevents happens after a tag is pushed.
- [ ] `heraut check` on a changelog-only config in an ambiguous environment reports a warning, not a failure.
- [ ] No real data in any changed file.

## Deferred (deliberately not in this plan)

**T168** (decide the fate of `port.Forge`'s uncalled `CommitURL`/`ChangeURL`/`CompareURL` and the
dead `lc` parameter) and the remainder of **T173** (the `needsForge` tautology, the missing nil-guard
in `HasResolvablePublishTarget`, forge resolution running twice per release, and the triplicated
`clearCIEnv` helper) are internal-only. They are worth doing, but none can affect a user's release,
so they should not delay P4.
