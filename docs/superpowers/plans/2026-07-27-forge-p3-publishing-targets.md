# Forge P3 — publishing via `release.targets` (T163) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `release.targets[]` the publishing surface — each target referencing a `forges[].name` — so publishing inherits the same CI/git identity resolution enrichment already uses, and retire `release.platforms`.

**Architecture:** A pure adapter turns a resolved `port.ForgeIdentity` + a `config.Target` into the `config.Platform` value the existing platform drivers already accept, so `internal/platforms/{github,gitlab}` and `port.Platform` stay **byte-for-byte unchanged** and their contract tests remain valid evidence that the transport did not move. `config.EffectiveTargets` mirrors `EffectivePlatforms`'s per-env replacement. The final task is the breaking cutover: `release.platforms` removed with a migration error, consumers (app, `heraut check`, scaffold) and docs updated.

**Tech Stack:** Go, `testify`. No new dependencies.

## Scope decision (READ FIRST)

This phase unifies **configuration**, not transport. Publishing keeps using `gh`/`glab`.
`port.Platform`, `internal/platforms/{github,gitlab}`, and the Docker bundle (ADR-0016) are **not
touched**. Reasoning is recorded in the design spec: the epic's user-visible goal is one config
concept and doesn't require a transport change; the original `CI_JOB_TOKEN` pain was solved in P1;
and P2 shipped two Critical defects that a green suite missed because `httptest` validates no
request shape — a risk that is worst on the path producing real release artifacts.

**Do not** rewrite any driver, change `port.Platform`, or introduce a native publishing client.
If a task seems to require it, stop and report — the plan is wrong.

**Token note (deliberate):** the identity supplies **host and project/repository**; the **token
still resolves via `token_env`** (mapped from the forge entry, defaulting per type as today),
because the drivers own their auth environment (`GH_TOKEN`/`GITLAB_TOKEN`, and `glab`'s gitlab.com
CI autologin). Passing the identity's already-resolved token — which would enable `CI_JOB_TOKEN`
publishing on self-hosted GitLab CI — belongs to the deferred native-publishing work, not here.

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven preferred.
- **No new Go dependencies.** `internal/config` imports nothing from heraut; `internal/app` is the only place constructing concrete implementations.
- **`internal/platforms/**` and `internal/port/platform.go` must not appear in any diff.** Their existing contract tests must pass untouched.
- **No real data** — synthetic placeholders only (`gitlab.example.com`, `group/subgroup/project`, `acme/widget`, `alice`).
- **No environment leakage** — resolution tests inject `getenv` or use the existing `clearCIEnv(t)` helper in `internal/app`; never real env vars.
- Config field changes sync three surfaces: `internal/config/`, `schema.json`, `docs/heraut.sample.yml`.
- Errors wrapped with `%w`; sentinels `errors.Is`-able.
- Never bypass git hooks. Lint via `hk fix` (never `gofmt`/`yamlfmt` directly). Before committing run BOTH `go test ./...` and `GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./...`.
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — with angle brackets, and **never** a `Claude-Session:` line.

## Existing types you will use (already in the tree)

```go
// internal/config/config.go
type Forge struct { Name, Type, Project, Repository, BaseURL, APIURL, APIMode, TokenEnv string }
type Target struct { Forge string; Draft, Prerelease bool; Assets []string }
type Platform struct {
    Name, Type string
    Repository string; Draft, Prerelease bool
    Project string
    BaseURL, TokenEnv string
    Assets []string
    LenientAssets bool
}
// internal/port/forge.go
type ForgeIdentity struct { Type, Host, APIURL, Project, Repository, Token string; TokenKind TokenKind; APIMode string }
// internal/forge
func Resolve(cfg *config.Config, getenv func(string) string, gitOrigin string) (Resolved, error)
type Resolved struct { Forges []port.ForgeIdentity; EnrichmentIndex int }
```

---

### Task 1: `config.EffectiveTargets` (per-env replacement)

**Files:**
- Modify: `internal/config/platforms.go` (add alongside `EffectivePlatforms`)
- Test: `internal/config/platforms_test.go`

**Interfaces:**
- Produces (used by Tasks 2 and 3): `func EffectiveTargets(cfg *Config, env string) []Target`

Semantics must mirror `EffectivePlatforms` exactly — read it first (same file). A non-empty per-env
list **replaces** the top-level list (it does not merge); an empty or absent per-env list leaves the
top-level list in force; a nil config yields nil.

- [ ] **Step 1: Write the failing test** — append to `internal/config/platforms_test.go`

```go
func TestEffectiveTargets(t *testing.T) {
	base := []config.Target{{Forge: "Primary", Draft: true}}
	envOverride := []config.Target{{Forge: "Mirror"}}

	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		want []config.Target
	}{
		{name: "nil config", cfg: nil, env: "", want: nil},
		{
			name: "top-level only",
			cfg:  &config.Config{Release: &config.Release{Targets: base}},
			env:  "", want: base,
		},
		{
			name: "env with no override keeps top-level",
			cfg: &config.Config{
				Release:      &config.Release{Targets: base},
				Environments: map[string]config.Environment{"staging": {}},
			},
			env: "staging", want: base,
		},
		{
			name: "env override replaces (does not merge)",
			cfg: &config.Config{
				Release: &config.Release{Targets: base},
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Targets: envOverride}},
				},
			},
			env: "staging", want: envOverride,
		},
		{
			name: "empty env override keeps top-level",
			cfg: &config.Config{
				Release: &config.Release{Targets: base},
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Targets: []config.Target{}}},
				},
			},
			env: "staging", want: base,
		},
		{
			name: "unknown env keeps top-level",
			cfg:  &config.Config{Release: &config.Release{Targets: base}},
			env:  "nope", want: base,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.EffectiveTargets(tc.cfg, tc.env))
		})
	}
}
```

> `config.EnvRelease` currently has `Notes` and `Platforms`. Add `Targets []Target` to it (yaml
> `targets,omitempty`) in Step 3 — the per-env override needs a home. Check the struct in
> `internal/config/config.go` before writing.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestEffectiveTargets`
Expected: FAIL — `EffectiveTargets` undefined (and `EnvRelease.Targets` undefined).

- [ ] **Step 3: Implement**

Add `Targets []Target \`yaml:"targets,omitempty"\`` to `EnvRelease` in `internal/config/config.go`,
then in `internal/config/platforms.go`:

```go
// EffectiveTargets resolves the publish targets for env: a non-empty per-environment list replaces
// the top-level one (it does not merge), mirroring EffectivePlatforms.
func EffectiveTargets(cfg *Config, env string) []Target {
	if cfg == nil {
		return nil
	}

	var targets []Target
	if cfg.Release != nil {
		targets = cfg.Release.Targets
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil && len(envCfg.Release.Targets) > 0 {
			targets = envCfg.Release.Targets
		}
	}

	return targets
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Sync schema + sample, then commit**

Add `targets` under the per-environment `release` object in `schema.json` (same item shape as the
top-level `release.targets`), and show a per-env `targets:` example in `docs/heraut.sample.yml`
consistent with how per-env `platforms:` is shown today.

```bash
go test ./... && hk fix
git add internal/config/ schema.json docs/heraut.sample.yml
git commit -m "feat(config): add EffectiveTargets with per-env replacement (T163)"
```

---

### Task 2: build platform drivers from resolved forge identities

**Files:**
- Modify: `internal/app/platforms.go` (add the adapter)
- Modify: `internal/app/pipeline.go` (build from targets)
- Test: `internal/app/platforms_test.go` (or a new `internal/app/targets_internal_test.go`)

**Interfaces:**
- Consumes: `config.EffectiveTargets` (Task 1); `forge.Resolve` / `forge.Resolved`; `port.ForgeIdentity`.
- Produces (used by Task 3): `func platformConfigFromTarget(t config.Target, f config.Forge, id port.ForgeIdentity) config.Platform`.

**The point of this task:** the drivers are unchanged. We construct the `config.Platform` they
already accept, from the resolved identity, so publishing inherits auto-detection.

Field mapping (note the platform-specific split — GitHub keeps `owner/repo` in `Repository`, GitLab
keeps the path in `Project`):

| `config.Platform` | source |
|---|---|
| `Name` | the forge's name (target's `forge` reference, or the sole forge's name) |
| `Type` | `id.Type` |
| `BaseURL` | `id.Host` |
| `Repository` | `id.Project` when `id.Type == "github"`, else `id.Repository` |
| `Project` | `id.Project` when `id.Type != "github"` |
| `TokenEnv` | `f.TokenEnv` (empty ⇒ driver's per-type default, unchanged behaviour) |
| `Draft` / `Prerelease` / `Assets` | from the target |

- [ ] **Step 1: Write the failing test**

```go
func TestPlatformConfigFromTarget(t *testing.T) {
	t.Run("github maps project into Repository", func(t *testing.T) {
		got := platformConfigFromTarget(
			config.Target{Forge: "GH", Draft: true, Assets: []string{"dist/*"}},
			config.Forge{Name: "GH", Type: "github", TokenEnv: "MY_GH_TOKEN"},
			port.ForgeIdentity{Type: "github", Host: "https://github.example.com", Project: "acme/widget"},
		)
		assert.Equal(t, "GH", got.Name)
		assert.Equal(t, "github", got.Type)
		assert.Equal(t, "https://github.example.com", got.BaseURL)
		assert.Equal(t, "acme/widget", got.Repository)
		assert.Empty(t, got.Project, "github carries the path in Repository, not Project")
		assert.Equal(t, "MY_GH_TOKEN", got.TokenEnv)
		assert.True(t, got.Draft)
		assert.Equal(t, []string{"dist/*"}, got.Assets)
	})

	t.Run("gitlab maps project into Project", func(t *testing.T) {
		got := platformConfigFromTarget(
			config.Target{Forge: "GL"},
			config.Forge{Name: "GL", Type: "gitlab"},
			port.ForgeIdentity{Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project"},
		)
		assert.Equal(t, "gitlab", got.Type)
		assert.Equal(t, "https://gitlab.example.com", got.BaseURL)
		assert.Equal(t, "group/subgroup/project", got.Project)
		assert.Empty(t, got.Repository)
		assert.Empty(t, got.TokenEnv, "no token_env leaves the driver's per-type default in force")
	})
}
```

Add a wiring test proving a target with no `forge:` resolves against a single forge, and that
publishing works with **no** `release.targets` at all (zero-config: one resolved forge ⇒ one
driver). Use the hermetic `fakeEnv`/`clearCIEnv` helpers already in `internal/app`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run TestPlatformConfigFromTarget`
Expected: FAIL — `platformConfigFromTarget` undefined.

- [ ] **Step 3: Implement the adapter** — `internal/app/platforms.go`

```go
// platformConfigFromTarget builds the config.Platform an existing platform driver accepts from a
// resolved forge identity plus a target's publish options. The drivers are unchanged (ADR-0043 P3):
// the identity supplies host and project/repository, so publishing inherits CI/git auto-detection,
// while the token still resolves via TokenEnv because the drivers own their auth environment.
func platformConfigFromTarget(t config.Target, f config.Forge, id port.ForgeIdentity) config.Platform {
	cfg := config.Platform{
		Name:       f.Name,
		Type:       id.Type,
		BaseURL:    id.Host,
		TokenEnv:   f.TokenEnv,
		Draft:      t.Draft,
		Prerelease: t.Prerelease,
		Assets:     t.Assets,
	}
	if id.Type == "github" {
		cfg.Repository = id.Project
	} else {
		cfg.Project = id.Project
		cfg.Repository = id.Repository
	}
	return cfg
}
```

- [ ] **Step 4: Wire it in `internal/app/pipeline.go`**

In `buildReleasePipelineConfig`, build `pCfg.Platforms` from `config.EffectiveTargets(cfg, env)`
instead of `config.EffectivePlatforms(cfg, env)`:

1. resolve the forges once (reuse the resolution already performed for enrichment rather than
   calling `forge.Resolve` twice — read the existing `resolveEnrichForgeIfNeeded` and extend or
   factor it so both consumers share one `forge.Resolved`; do not add a second `git remote` call),
2. for each effective target, find its forge by `t.Forge` (or the sole forge when `t.Forge` is
   empty) and its matching identity by index,
3. `platformConfigFromTarget(...)` → `buildPlatform(runner, &platCfg)` → append.

Keep the existing top-level `release.assets` propagation (assets applied to targets that declare
none, with `LenientAssets = true`). When `release.targets` is empty but a forge resolves, build one
target with default options for the enrichment/sole forge. Error clearly when a target names an
unknown forge, or when no forge resolves at all.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/app/ && go test ./internal/pipeline/`
Expected: PASS.

- [ ] **Step 6: Full suites + lint + commit**

Existing tests that configure `release.platforms` may now build zero platforms. **Do not delete
them** — migrate each to `release.targets` + `forges:`, preserving what it asserts. If a test
asserts multi-platform behaviour (ADR-0025), it must still assert it with multiple targets.

```bash
go test ./... && GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./... && hk fix
git add internal/app/ internal/pipeline/
git commit -m "feat(app): build publish platforms from resolved forge identities (T163)"
```

---

### Task 3: retire `release.platforms` (breaking cutover)

**Files:**
- Modify: `internal/config/config.go` (delete `Release.Platforms`, `EnvRelease.Platforms`), `internal/config/platforms.go` (delete `EffectivePlatforms`, `normalizePlatforms` if unused), `internal/config/validator.go` (drop platform-entry validation), `internal/config/loader.go` (migration error)
- Modify: `internal/cmd/release.go` (`hasEffectivePlatforms`), `internal/cmd/check.go`, `internal/app/check.go`
- Modify: `internal/scaffold/{wizard,generate,dropped}.go` (**mechanical emit-change only**)
- Modify: `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`, `.heraut.yml`
- Modify: `docs/tasks/forge-abstraction-roadmap.md`
- Test: `internal/config/migration_test.go`

**Interfaces:**
- Consumes: `EffectiveTargets` (Task 1), `platformConfigFromTarget` (Task 2), the existing `ErrRemovedConfigKey` mechanism in `internal/config/loader.go` (added in T160).

**Scaffold boundary — read carefully.** Removing `release.platforms` forces `internal/scaffold/` to
change. The change here is **mechanical only**: keep the wizard's prompts, flow, and its internal
`PlatformAnswer` type exactly as they are; change only what it **emits** (a `forges:` entry plus a
`release.targets` entry referencing it) and what it **reads back** when round-tripping an existing
config. The wizard's *redesign* — forge-aware prompts, an `api_mode` question, auto-detection
defaults — is **T164/P4** and must not be started here.

- [ ] **Step 1: Write the failing migration test** — add to `internal/config/migration_test.go`

```go
func TestLoad_RemovedKey_ReleasePlatforms(t *testing.T) {
	tests := []struct{ name, body string }{
		{
			name: "top-level",
			body: `version: "1"
versioning: {strategy: semver}
release:
  platforms:
    - name: gl
      platform: gitlab
      project: group/subgroup/project
`,
		},
		{
			name: "per-environment",
			body: `version: "1"
versioning: {strategy: semver}
environments:
  staging:
    release:
      platforms:
        - name: gl
          platform: gitlab
          project: group/subgroup/project
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeCfg(t, tc.body))
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrRemovedConfigKey))
			assert.Contains(t, err.Error(), "release.targets", "the error must name the replacement")
			assert.Contains(t, err.Error(), "forges:", "and where the coordinates move to")
		})
	}
}
```

`writeCfg` already exists in that file. Read the existing `checkRemovedKeys` probe struct — it
already handles per-env `changelog.remote`, so follow that shape for per-env `release.platforms`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_RemovedKey_ReleasePlatforms`
Expected: FAIL — the config still parses.

- [ ] **Step 3: Add the migration entries**

Extend `removedKeys` and the per-env probe in `internal/config/loader.go` with `release.platforms`
and `environments.<env>.release.platforms`, hinting: declare a `forges:` entry carrying
`base_url` / `token_env` / `repository`-or-`project`, then reference it from
`release.targets[].forge`, keeping `draft` / `prerelease` / `assets` on the target.

- [ ] **Step 4: Delete `release.platforms` and follow the compiler**

Remove `Release.Platforms`, `EnvRelease.Platforms`, `EffectivePlatforms`, the platform-entry
validation, and the `normalizePlatforms` call if it becomes unused. Then fix every consumer the
compiler reports — `internal/cmd/release.go`'s `hasEffectivePlatforms` (now "has ≥1 resolvable
forge/target"), `internal/cmd/check.go` and `internal/app/check.go` (the Platforms section reads
effective **targets**; keep verifying `gh`/`glab` on PATH — the transport is unchanged), and the
scaffold per the boundary note above.

**Preserve behaviour, not code:** any test asserting per-env override, multi-instance publishing
(ADR-0025), or asset propagation must survive, migrated to targets — not deleted.

- [ ] **Step 5: Sync the three config surfaces + the spec + heraut's own config**

Remove `release.platforms` from `schema.json` and `docs/heraut.sample.yml`; document
`release.targets` in `docs/specs/02-configuration.md` (T165 deliberately deferred this while the key
was inert — the section already flags it as "not yet functional"; update that). Migrate this repo's
own `.heraut.yml` to `forges:` + `release.targets`, which dogfoods the migration.

- [ ] **Step 6: Full suites + lint**

Run: `go test ./... && GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./... && hk fix`
Expected: all PASS. **`internal/platforms/**` and `internal/port/platform.go` must not appear in
`git diff --stat`** — verify before committing.

- [ ] **Step 7: Write ADR-0044**

Create `docs/adr/0044-publishing-config-unification.md`, mirroring the section layout of a recent
ADR (`docs/adr/0043-forge-abstraction.md` is the closest sibling). It must record:

- **Decision:** `release.targets[]` is the publishing surface, referencing `forges[].name`;
  `release.platforms` is removed with a migration error; platform drivers are constructed from the
  resolved `port.ForgeIdentity`, so publishing inherits CI/git auto-detection.
- **The transport decision and its reasoning** — publishing deliberately keeps `gh`/`glab`;
  `port.Platform` and `internal/platforms/*` are unchanged. Give the three reasons from the design
  spec (the config goal doesn't require a transport change; the `CI_JOB_TOKEN` pain was solved in
  P1 and `glab` already autologins on gitlab.com CI; P2 demonstrated hand-written request shapes
  ship broken past a green suite, which is worst on the artifact path). This section exists so the
  decision is not re-litigated.
- **Consequences:** breaking config change (pre-v1.0); `gh`/`glab` remain runtime dependencies and
  stay in the Docker image (ADR-0016 unchanged); native publishing remains available as a future,
  separately-motivated task, which is what would drop those dependencies and enable `CI_JOB_TOKEN`
  publishing on self-hosted GitLab CI.
- **Supersession:** it supersedes ADR-0043's P3 framing (which assumed a transport replacement) and
  updates ADR-0025's config surface (multi-instance publishing now means multiple targets/forges).

Add the index row to `docs/adr/README.md` following that table's existing convention, and annotate
the ADR-0043 and ADR-0025 rows with their partial supersession the way sibling rows already do.

- [ ] **Step 8: Roadmap + commit**

Flip **T163** to `[x]` with a completion note recording the scope decision (config unification only;
transport deliberately unchanged) and the P3/P4 wizard boundary. Update the P3 row in "Progress at a
glance".

```bash
git add internal/ schema.json docs/ .heraut.yml
git commit -m "feat(config)!: publish via release.targets, remove release.platforms (T163)"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS, and again under `GITHUB_ACTIONS=true …` (forge resolution reads CI markers).
- [ ] `hk check` → clean.
- [ ] Transport untouched — with `BASE` = the commit before Task 1, `git diff --stat BASE..HEAD -- internal/platforms/ internal/port/platform.go` must be **empty**.
- [ ] ADR-0044 exists, records the transport decision, and is listed in `docs/adr/README.md`.
- [ ] `git grep -n "EffectivePlatforms\|release.platforms"` → only migration-error strings, tests asserting them, ADRs, and roadmap history.
- [ ] `heraut check config` against this repo's migrated `.heraut.yml` resolves without error.
- [ ] No real data in any changed file.

## Handoff to P4

With `forges:` + `release.targets` the only config surface, T164 can redesign the `heraut init`
wizard against a stable schema: forge-aware prompts, an `api_mode` question, and auto-detection
defaults ("detected GitLab CI — use it?"). The mechanical emit-change made here is the floor, not
the finished wizard.
