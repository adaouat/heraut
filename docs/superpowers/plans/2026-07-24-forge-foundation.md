# Forge Abstraction — P1 Foundation (T154–T157) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the additive foundation of the forge abstraction — the `port.Forge` contract + ADR-0043, the `forges:` / `release.targets:` / `commits.enrichment_*` config, its validation, and environment-based identity resolution — all green and unit-tested, with **zero change to runtime behavior** (the old `changelog.remote` / `release.platforms` paths stay fully functional; the cutover is Plan B).

**Architecture:** New `internal/port/forge.go` defines the `Forge` interface + resolved-identity and enrichment value types (imports nothing from heraut). New config structs (`Forge`, `Target`, `commits.enrichment_forge`/`enrichment_policy`) parse alongside the existing ones. A new pure `internal/forge` package resolves a `port.ForgeIdentity` per forge from **explicit config → CI env → git `origin` → none**, failing loud on ambiguity. Nothing is wired into the pipeline yet — the GitLab forge implementation and the pipeline cutover are **Plan B (T158–T160)**.

**Tech Stack:** Go, `testify`, `yaml.v3`, JSON Schema. No new dependencies.

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven tests preferred.
- **No new Go dependencies.** `internal/port` imports nothing from heraut; `internal/config` imports nothing from heraut; `internal/forge` imports only `internal/{port,config}` + stdlib.
- **No real data** in tests/docs/samples — synthetic placeholders only (`gitlab.example.com`, `group/subgroup/project`, `alice`, `alice@example.com`).
- **Config field changes sync three files in lockstep** (`.claude/rules/coding.md`): `internal/config/config.go`, `schema.json`, `docs/heraut.sample.yml`.
- **This plan is additive — do NOT remove or break `changelog.remote` / `release.platforms` / `commits.remote_metadata`.** The migration error and old-path removal are Plan B. Old config must keep working after every task here.
- **Config keys (exact):** forge entry — `name`, `platform`, `project`, `repository`, `base_url`, `api_url`, `api_mode` (`rest`|`graphql`, default `rest`), `token_env`. Enrichment — `commits.enrichment_forge`, `commits.enrichment_policy` (`disabled`|`optional`|`required`). Release — `release.targets[].forge`, `.draft`, `.prerelease`, `.assets`.
- **Never bypass git hooks.** Fix lint via `hk fix` (never `gofmt`/`yamlfmt` directly).
- **Commit trailer (subagent commits):** `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — no `Claude-Session:` line.

## File Structure

- `docs/adr/0043-forge-abstraction.md` — **new ADR** (decision record; distilled from the design spec).
- `docs/adr/README.md` — index row for ADR-0043.
- `internal/port/forge.go` — **new**: `Forge`, `ForgeIdentity`, `TokenKind`, `Enrichment`, `PullRequest`, `Author`, `Commit`. The stable contract.
- `internal/config/config.go` — **modify**: add `Config.Forges []Forge`, `Forge` struct, `Target` struct, `Release.Targets`, `Commits.EnrichmentForge`, `Commits.EnrichmentPolicy`.
- `internal/config/validator.go` — **modify**: validate the new keys (additive; old-key rejection is Plan B).
- `internal/forge/resolve.go` — **new**: `Resolve(...)` — config/CI/git → `[]port.ForgeIdentity` + the enrichment-source selection, with ambiguity errors.
- `internal/forge/detect.go` — **new**: CI-env + git-origin detection helpers (pure, injected `getenv`/`gitOrigin`).
- `schema.json`, `docs/heraut.sample.yml` — **modify**: document the new keys.

> **Note on `port.PullRequest`/`Author`:** native already defines identical types in `internal/generators/native/model.go`. This plan defines the canonical copies in `port`; native keeps its own for now (unchanged). Plan B's cutover converts `port.Enrichment` → native's internal types at the boundary; unifying native onto the `port` types is deferred to P2. This intentional short-lived duplication keeps the foundation additive.

---

### Task T154: `port.Forge` contract + ADR-0043

**Files:**
- Create: `internal/port/forge.go`
- Create: `docs/adr/0043-forge-abstraction.md`
- Modify: `docs/adr/README.md` (add the 0043 index row)
- Test: `internal/port/forge_test.go`

**Interfaces:**
- Produces (consumed by every later task):
  - `port.TokenKind` (`TokenNone` | `TokenJob` | `TokenPrivate`)
  - `port.ForgeIdentity{ Type, Host, APIURL, Project, Token string; TokenKind TokenKind; APIMode string }`
  - `port.Author{ Username string }`
  - `port.PullRequest{ Number int; URL, Title, AuthorLogin string; Labels []string; RefPrefix string; CreatedAt, MergedAt time.Time; MergedBy Author; Approvers []Author }`
  - `port.Enrichment{ PRs map[string]PullRequest; Authors map[string]string }`
  - `port.Commit{ Hash, Author, Email string; Date time.Time }`
  - `port.Forge` interface: `Type() string`, `Identity() ForgeIdentity`, `CommitURL(sha string) string`, `ChangeURL(number int) string`, `CompareURL(from, to string) string`, `Enrich(commits []Commit) (Enrichment, error)`

- [ ] **Step 1: Write the failing test** — `internal/port/forge_test.go`

```go
package port_test

import (
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

// fakeForge proves the Forge interface is implementable and the value types compose.
type fakeForge struct{ id port.ForgeIdentity }

func (f fakeForge) Type() string                          { return f.id.Type }
func (f fakeForge) Identity() port.ForgeIdentity          { return f.id }
func (f fakeForge) CommitURL(sha string) string           { return f.id.Host + "/-/commit/" + sha }
func (f fakeForge) ChangeURL(n int) string                { return f.id.Host + "/-/merge_requests/" }
func (f fakeForge) CompareURL(from, to string) string     { return f.id.Host + "/-/compare/" + from + "..." + to }
func (f fakeForge) Enrich(_ []port.Commit) (port.Enrichment, error) {
	return port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc": {Number: 42, RefPrefix: "!", MergedBy: port.Author{Username: "bob"}}},
		Authors: map[string]string{"abc": "alice"},
	}, nil
}

func TestForge_InterfaceComposes(t *testing.T) {
	var f port.Forge = fakeForge{id: port.ForgeIdentity{
		Type: "gitlab", Host: "https://gitlab.example.com", APIURL: "https://gitlab.example.com/api/v4",
		Project: "group/subgroup/project", Token: "t", TokenKind: port.TokenJob, APIMode: "rest",
	}}
	assert.Equal(t, "gitlab", f.Type())
	assert.Equal(t, port.TokenJob, f.Identity().TokenKind)
	assert.Equal(t, "https://gitlab.example.com/-/commit/deadbeef", f.CommitURL("deadbeef"))

	en, err := f.Enrich([]port.Commit{{Hash: "abc", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	assert.NoError(t, err)
	assert.Equal(t, "alice", en.Authors["abc"])
	assert.Equal(t, "!", en.PRs["abc"].RefPrefix)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/port/ -run TestForge_InterfaceComposes`
Expected: compile failure — `undefined: port.Forge` (and the value types).

- [ ] **Step 3: Create `internal/port/forge.go`**

```go
package port

import "time"

// TokenKind selects how a forge token is presented to the API. GitLab distinguishes a CI job
// token (JOB-TOKEN header) from a personal/project access token (PRIVATE-TOKEN header); other
// forges ignore it. See ADR-0043.
type TokenKind int

const (
	TokenNone TokenKind = iota
	TokenJob
	TokenPrivate
)

// ForgeIdentity is a forge's fully resolved connection facts (from config, CI env, or git origin).
type ForgeIdentity struct {
	Type      string // "github" | "gitlab" | "azure_devops"
	Host      string // web host, e.g. "https://gitlab.example.com"
	APIURL    string // API base, e.g. "https://gitlab.example.com/api/v4"
	Project   string // "owner/repo" | "group/subgroup/project" | "organization/project"
	Token     string
	TokenKind TokenKind
	APIMode   string // "rest" | "graphql"
}

// Author is a resolved platform user handle.
type Author struct{ Username string }

// PullRequest is the per-commit PR/MR metadata a Forge resolves. RefPrefix is "#" for a GitHub PR
// and "!" for a GitLab MR.
type PullRequest struct {
	Number      int
	URL         string
	Title       string
	AuthorLogin string
	Labels      []string
	RefPrefix   string
	CreatedAt   time.Time
	MergedAt    time.Time
	MergedBy    Author
	Approvers   []Author
}

// Enrichment is a forge's per-commit remote data: sha→PR and sha→author-handle.
type Enrichment struct {
	PRs     map[string]PullRequest
	Authors map[string]string
}

// Commit is the minimal per-commit input a Forge needs to resolve enrichment: the SHA plus the
// local git author name/email/date.
type Commit struct {
	Hash   string
	Author string // git author name (%an)
	Email  string // git author email (%ae)
	Date   time.Time
}

// Forge is one code-hosting platform heraut talks to: it exposes its resolved identity, builds web
// links, and fetches per-commit PR/MR + author metadata. One implementation per platform type lives
// in internal/forge/. See ADR-0043.
type Forge interface {
	Type() string
	Identity() ForgeIdentity
	CommitURL(sha string) string
	ChangeURL(number int) string
	CompareURL(from, to string) string
	Enrich(commits []Commit) (Enrichment, error)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/port/ -run TestForge_InterfaceComposes`
Expected: PASS.

- [ ] **Step 5: Write ADR-0043**

Create `docs/adr/0043-forge-abstraction.md` distilling the design spec (`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`). Use the established ADR shape (see `docs/adr/0042-gitlab-graphql-enrichment.md` for the exact section layout: `# ADR-0043: …`, Status **Accepted**, Date **2026-07-24**, Deciders bchatard, then `## Context`, `## Decision`, `## Consequences`, `## Alternatives considered`, `## References`). Content, condensed:
  - **Context:** the three problems — `CI_JOB_TOKEN` cannot use GraphQL and `glab` sends `PRIVATE-TOKEN`; `changelog.remote` vs `release.platforms` naming mismatch; no clean extension point for new forges.
  - **Decision:** a `port.Forge` (resolve/links/enrich); a top-level `forges:` list (connection only) + `release.targets:` (publish) + `commits.enrichment_forge`/`enrichment_policy`; identity auto-resolved from CI env / git origin with fail-on-ambiguity; GitLab native `net/http` (REST default `JOB-TOKEN`, GraphQL opt-in `PRIVATE-TOKEN`). Reference the design spec for full detail.
  - **Consequences:** breaking config change (pre-v1.0), migration error maps old→new; extends/supersedes ADRs 0006/0020/0023/0025/0026/0034/0035/0039/0040/0041/0042; phased P1–P4.
  - Add the index row to `docs/adr/README.md`: `| [0043](0043-forge-abstraction.md) | Forge abstraction + unified forges: config | Accepted |` (insert after the 0042 row).

- [ ] **Step 6: Commit**

```bash
git add internal/port/forge.go internal/port/forge_test.go docs/adr/0043-forge-abstraction.md docs/adr/README.md
git commit -m "feat(port): add Forge contract + ADR-0043 (forge abstraction)"
```

---

### Task T155: config — `forges:` / `release.targets:` / `commits.enrichment_*`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `schema.json`
- Modify: `docs/heraut.sample.yml`
- Test: `internal/config/loader_forge_test.go` (new)
- Test: `internal/config/testdata/` — add fixtures (see Step 1)

**Interfaces:**
- Consumes: nothing (config is the bottom layer).
- Produces (consumed by T156, T157, and Plan B):
  - `config.Forge{ Name, Type, Project, Repository, BaseURL, APIURL, APIMode, TokenEnv string }` with yaml keys `name`, `platform`, `project`, `repository`, `base_url`, `api_url`, `api_mode`, `token_env`.
  - `config.Target{ Forge string; Draft, Prerelease bool; Assets []string }` with yaml keys `forge`, `draft`, `prerelease`, `assets`.
  - `Config.Forges []Forge` (`yaml:"forges,omitempty"`), `Release.Targets []Target` (`yaml:"targets,omitempty"`), `Commits.EnrichmentForge string` (`yaml:"enrichment_forge,omitempty"`), `Commits.EnrichmentPolicy string` (`yaml:"enrichment_policy,omitempty"`).

- [ ] **Step 1: Write the failing test** — `internal/config/loader_forge_test.go`

First add the fixture `internal/config/testdata/forge-minimal.yml` (synthetic only):

```yaml
version: "1"
versioning:
  strategy: semver
  tag_prefix: ""
changelog:
  generator: native
  output: CHANGELOG.md
commits:
  enrichment_forge: Primary GitLab
  enrichment_policy: optional
forges:
  - name: Primary GitLab
    platform: gitlab
    api_mode: rest
release:
  notes:
    generator: native
  targets:
    - forge: Primary GitLab
      draft: false
```

Then the test:

```go
package config_test

import (
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ForgesAndTargets(t *testing.T) {
	c, err := config.Load(filepath.Join("testdata", "forge-minimal.yml"))
	require.NoError(t, err)

	require.Len(t, c.Forges, 1)
	assert.Equal(t, "Primary GitLab", c.Forges[0].Name)
	assert.Equal(t, "gitlab", c.Forges[0].Type)
	assert.Equal(t, "rest", c.Forges[0].APIMode)

	require.NotNil(t, c.Commits)
	assert.Equal(t, "Primary GitLab", c.Commits.EnrichmentForge)
	assert.Equal(t, "optional", c.Commits.EnrichmentPolicy)

	require.NotNil(t, c.Release)
	require.Len(t, c.Release.Targets, 1)
	assert.Equal(t, "Primary GitLab", c.Release.Targets[0].Forge)
	assert.False(t, c.Release.Targets[0].Draft)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_ForgesAndTargets`
Expected: FAIL — `c.Forges undefined` (and the other new fields).

- [ ] **Step 3: Add the config structs** — `internal/config/config.go`

Add `Forges` to the top-level `Config` struct (next to `Release`):

```go
	Forges []Forge `yaml:"forges,omitempty"`
```

Add the two new structs (near `Platform`/`Remote`):

```go
// Forge is one code-hosting platform heraut talks to — connection/identity only. What to publish
// (draft/assets) lives in release.targets; the enrichment source is commits.enrichment_forge.
// See ADR-0043.
type Forge struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"platform"` // discriminator; "platform" key avoids Forge.Forge self-reference (ADR-0006)
	Project    string `yaml:"project,omitempty"`    // gitlab: group[/subgroup]/repo; azure: organization/project
	Repository string `yaml:"repository,omitempty"` // github: owner/repo; azure: repo name
	BaseURL    string `yaml:"base_url,omitempty"`
	APIURL     string `yaml:"api_url,omitempty"`
	APIMode    string `yaml:"api_mode,omitempty"` // "rest" (default) | "graphql"
	TokenEnv   string `yaml:"token_env,omitempty"`
}

// Target is one release publish destination: a reference to a forges[].name plus publish options.
type Target struct {
	Forge      string   `yaml:"forge,omitempty"` // → forges[].name; optional when exactly one forge
	Draft      bool     `yaml:"draft,omitempty"`
	Prerelease bool     `yaml:"prerelease,omitempty"`
	Assets     []string `yaml:"assets,omitempty"`
}
```

Add `Targets` to the `Release` struct (next to `Platforms`):

```go
	Targets []Target `yaml:"targets,omitempty"`
```

Add the two fields to the `Commits` struct (find `type Commits struct`; it already holds `RemoteMetadata`):

```go
	EnrichmentForge  string `yaml:"enrichment_forge,omitempty"`
	EnrichmentPolicy string `yaml:"enrichment_policy,omitempty"`
```

> Keep `RemoteMetadata` for now (Plan B renames/removes it). The loader is strict (unknown keys error), so these fields MUST be declared before the fixture parses.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/ -run TestLoad_ForgesAndTargets`
Expected: PASS.

- [ ] **Step 5: Sync `schema.json` and `docs/heraut.sample.yml`**

In `schema.json`: add a top-level `forges` array property (items: object with `name`, `platform` enum `["github","gitlab","azure_devops"]`, `project`, `repository`, `base_url`, `api_url`, `api_mode` enum `["rest","graphql"]`, `token_env`; `required: ["name","platform"]`). Add `enrichment_forge` (string) and `enrichment_policy` (enum `["disabled","optional","required"]`) under the `commits` properties. Add `targets` (array) under `release` properties, items with `forge`, `draft`, `prerelease`, `assets`. Mirror the description style of the existing `release.platforms` block. Do **not** remove the existing `changelog.remote` / `release.platforms` schema (Plan B).

In `docs/heraut.sample.yml`: add a commented `forges:` block, a `commits.enrichment_forge`/`enrichment_policy` example, and a `release.targets:` example, with inline comments matching the sample's style — synthetic placeholders only.

- [ ] **Step 6: Add a schema fixture test row + lint**

Add `forge-minimal.yml` (from Step 1) to whatever schema-validation fixture list exists (grep for how `testdata/config/` fixtures are enumerated in the schema test; add a row asserting it validates). Then:

Run: `go test ./... && hk fix`
Expected: all PASS; lint clean.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/loader_forge_test.go internal/config/testdata/forge-minimal.yml schema.json docs/heraut.sample.yml
git commit -m "feat(config): add forges/release.targets/commits.enrichment_* (additive)"
```

---

### Task T156: validation of the new keys

**Files:**
- Modify: `internal/config/validator.go`
- Test: `internal/config/validator_forge_test.go` (new)

**Interfaces:**
- Consumes: `config.Forge`, `config.Target`, `Commits.EnrichmentForge`, `Commits.EnrichmentPolicy` (T155).
- Produces: `ValidationErrors` (existing type — grep `type ValidationErrors` / `ValidationError` in `validator.go` for the exact `{Path, Message, Hint}` shape) for the new rules. No new exported symbols.

Validation rules (additive — only when `forges:` is present; do **not** touch old-key paths):
1. Each forge: `name` non-empty and **unique**; `platform` ∈ `{github, gitlab, azure_devops}`; `api_mode` ∈ `{"", rest, graphql}`.
2. `commits.enrichment_policy` ∈ `{"", disabled, optional, required}`.
3. `commits.enrichment_forge`: if set, must name an existing forge. If `len(forges) > 1` and it is empty → error (ambiguous source). If `len(forges) == 1` it may be empty (defaults to that forge).
4. `release.targets[].forge`: if set, must name an existing forge. If `len(forges) > 1` and empty → error. `len(forges) == 1` may be empty.
5. `api_mode: graphql` on a `gitlab` forge is allowed here; the "graphql needs a token" check is a **resolution-time** concern (T157/Plan B), not static config — do not add it here.

- [ ] **Step 1: Write the failing tests** — `internal/config/validator_forge_test.go`

```go
package config_test

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func cfgWithForges(forges []config.Forge, enrichForge string) *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
		Commits:    &config.Commits{EnrichmentForge: enrichForge, EnrichmentPolicy: "optional"},
		Forges:     forges,
	}
}

// forgeErr reports whether some validation error's Path or Message contains want.
func forgeErr(errs config.ValidationErrors, want string) bool {
	for _, e := range errs {
		if strings.Contains(e.Path, want) || strings.Contains(e.Message, want) {
			return true
		}
	}
	return false
}

func TestValidate_Forges(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string // substring in some error Path/Message; "" = no forge error
	}{
		{"single forge, no enrichment_forge ok",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}}, ""), ""},
		{"unknown platform",
			cfgWithForges([]config.Forge{{Name: "A", Type: "bitbucket"}}, ""), "platform"},
		{"duplicate name",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}, {Name: "A", Type: "github"}}, "A"), "duplicate"},
		{"multi forge requires enrichment_forge",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}}, ""), "enrichment_forge"},
		{"enrichment_forge names unknown forge",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}}, "Z"), "unknown"},
		{"bad api_mode",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", APIMode: "grpc"}}, ""), "api_mode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := config.Validate(tc.cfg)
			if tc.want == "" {
				// Robust to unrelated config incompleteness: assert only that no
				// forge/enrichment error was produced (not that the whole config is valid).
				assert.False(t, forgeErr(errs, "forges"), "unexpected forge error: %v", errs)
				assert.False(t, forgeErr(errs, "enrichment_forge"), "unexpected error: %v", errs)
				return
			}
			assert.True(t, forgeErr(errs, tc.want), "expected an error mentioning %q, got %v", tc.want, errs)
		})
	}
}
```

> **Note:** `config.Validate(cfg)` returns a `config.ValidationErrors` **slice** (empty = valid; each element has `.Path`/`.Message`/`.Hint`) — confirmed against `internal/config/validator_test.go`. `Load` does **not** call `Validate`, so the T155 fixture (which uses `Load`) parses without semantic checks; this task's rules are exercised by calling `Validate` directly, as above. Make the duplicate-name message contain "duplicate", the unknown-reference messages contain "unknown", the platform error path/message contain "platform", and the api_mode error contain "api_mode" so the substring assertions hold.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestValidate_Forges`
Expected: FAIL — the new rules aren't enforced yet (valid cases may pass, invalid cases don't error).

- [ ] **Step 3: Implement the validation** — `internal/config/validator.go`

Add a `validateForges(c *Config) ValidationErrors` (match the file's existing helper signature/return convention) covering rules 1–4 above, appended into the aggregate the top-level validator returns. Emit `{Path, Message, Hint}` errors, e.g. path `forges[1].platform`, `commits.enrichment_forge`, `release.targets[0].forge`. Wire the call into the existing validation aggregation (grep for where `validatePlatforms`/similar helpers are collected).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/ -run TestValidate_Forges`
Expected: PASS.

- [ ] **Step 5: Full suite + lint + commit**

```bash
go test ./... && hk fix
git add internal/config/validator.go internal/config/validator_forge_test.go
git commit -m "feat(config): validate forges/enrichment_forge/targets (additive)"
```

---

### Task T157: forge identity resolution (config / CI / git / ambiguity)

**Files:**
- Create: `internal/forge/detect.go`
- Create: `internal/forge/resolve.go`
- Test: `internal/forge/resolve_test.go`

**Interfaces:**
- Consumes: `config.Config`/`config.Forge` (T155), `port.ForgeIdentity`/`port.TokenKind` (T154).
- Produces (consumed by Plan B):
  - `forge.Resolved{ Forges []port.ForgeIdentity; EnrichmentIndex int }` — the resolved identities (by config order, or the single auto-detected one) and the index into `Forges` that is the enrichment source.
  - `func Resolve(cfg *config.Config, getenv func(string) string, gitOrigin string) (Resolved, error)` — `getenv`/`gitOrigin` injected for testability (production passes `os.Getenv` and the parsed `git remote get-url origin`).
  - Sentinel: `var ErrAmbiguousForge = errors.New("ambiguous forge")`.

Resolution rules:
- **If `cfg.Forges` non-empty:** build one `ForgeIdentity` per entry. Per field, fill gaps: explicit config → CI env (when the CI type matches the entry's `platform`) → git origin (when its host matches) → default host. Token: `token_env` → the CI token (kind `TokenJob` for `CI_JOB_TOKEN`, else `TokenPrivate`) → the type's default token env (`GITLAB_TOKEN`/`GITHUB_TOKEN`/`AZURE_DEVOPS_TOKEN`, kind `TokenPrivate`). `APIMode` defaults to `rest`. EnrichmentIndex = index of `commits.enrichment_forge` (or 0 when single/empty).
- **If `cfg.Forges` empty (zero-config):** auto-detect exactly one forge. CI env pins the type (`GITLAB_CI` / `GITHUB_ACTIONS` / `TF_BUILD`); else parse `gitOrigin` host for a known public host. If more than one candidate type is detected and none is pinned by CI/origin → `ErrAmbiguousForge`. Zero candidates → `Resolved{}` with empty `Forges` (offline; not an error here — callers decide).

- [ ] **Step 1: Write the failing tests** — `internal/forge/resolve_test.go`

```go
package forge_test

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolve_GitLabCIZeroConfig(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"GITLAB_CI":       "true",
		"CI_SERVER_URL":   "https://gitlab.example.com",
		"CI_API_V4_URL":   "https://gitlab.example.com/api/v4",
		"CI_PROJECT_PATH": "group/subgroup/project",
		"CI_JOB_TOKEN":    "jobtok",
	}), "")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[got.EnrichmentIndex]
	assert.Equal(t, "gitlab", f.Type)
	assert.Equal(t, "https://gitlab.example.com", f.Host)
	assert.Equal(t, "https://gitlab.example.com/api/v4", f.APIURL)
	assert.Equal(t, "group/subgroup/project", f.Project)
	assert.Equal(t, "jobtok", f.Token)
	assert.Equal(t, port.TokenJob, f.TokenKind)
	assert.Equal(t, "rest", f.APIMode)
}

func TestResolve_GitOriginLocalGitLab(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{"GITLAB_TOKEN": "pat"}),
		"git@gitlab.com:group/subgroup/project.git")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[0]
	assert.Equal(t, "gitlab", f.Type)
	assert.Equal(t, "https://gitlab.com", f.Host)
	assert.Equal(t, "group/subgroup/project", f.Project)
	assert.Equal(t, "pat", f.Token)
	assert.Equal(t, port.TokenPrivate, f.TokenKind)
}

func TestResolve_AmbiguousZeroConfig(t *testing.T) {
	_, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"GITLAB_TOKEN": "a", "GITHUB_TOKEN": "b",
	}), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, forge.ErrAmbiguousForge))
}

func TestResolve_ExplicitForgeFillsFromCI(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{Name: "P", Type: "gitlab", APIMode: "graphql", TokenEnv: "MY_PAT"}}}
	got, err := forge.Resolve(cfg, env(map[string]string{
		"GITLAB_CI": "true", "CI_SERVER_URL": "https://gitlab.example.com",
		"CI_API_V4_URL": "https://gitlab.example.com/api/v4", "CI_PROJECT_PATH": "group/project",
		"MY_PAT": "pat",
	}), "")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "https://gitlab.example.com", f.Host)     // filled from CI
	assert.Equal(t, "group/project", f.Project)               // filled from CI
	assert.Equal(t, "pat", f.Token)                           // token_env wins
	assert.Equal(t, port.TokenPrivate, f.TokenKind)           // explicit token_env → private
	assert.Equal(t, "graphql", f.APIMode)                     // explicit
}

func TestResolve_NoForgeOffline(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{}), "")
	require.NoError(t, err)
	assert.Empty(t, got.Forges)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/forge/`
Expected: compile failure — package `forge` / `Resolve` / `ErrAmbiguousForge` undefined.

- [ ] **Step 3: Implement `internal/forge/detect.go`**

Pure helpers (no `os` — take `getenv`):
- `detectCIForge(getenv) (typ, host, apiURL, project, token string, kind port.TokenKind, ok bool)` — GitLab (`GITLAB_CI`), GitHub (`GITHUB_ACTIONS`), Azure (`TF_BUILD`); read the vars from §3 of the design spec. `CI_JOB_TOKEN` → `TokenJob`; `GITHUB_TOKEN`/`SYSTEM_ACCESSTOKEN` → `TokenPrivate`.
- `parseGitOrigin(url string) (typ, host, project string, ok bool)` — handle `git@host:path.git` and `https://host/path.git`; strip `.git`; map known hosts (`github.com`→github, `gitlab.com`→gitlab, `dev.azure.com`→azure_devops); unknown host → `ok=false`.
- `defaultHostFor(typ) string` and `defaultTokenEnvFor(typ) string` — reuse the constants already in `internal/pipeline/linkctx.go` (`GITHUB_TOKEN`/`GITLAB_TOKEN`/`AZURE_DEVOPS_TOKEN`, `https://github.com`/`https://gitlab.com`/`https://dev.azure.com`); copy them here (this package must not import `internal/pipeline`).

- [ ] **Step 4: Implement `internal/forge/resolve.go`**

```go
package forge

import (
	"errors"
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

var ErrAmbiguousForge = errors.New("ambiguous forge")

type Resolved struct {
	Forges          []port.ForgeIdentity
	EnrichmentIndex int
}

func Resolve(cfg *config.Config, getenv func(string) string, gitOrigin string) (Resolved, error) {
	if len(cfg.Forges) > 0 {
		return resolveExplicit(cfg, getenv, gitOrigin)
	}
	return resolveAuto(getenv, gitOrigin)
}
```

Implement `resolveExplicit` (one identity per `cfg.Forges` entry, gap-filling per the rules; EnrichmentIndex from `cfg.Commits.EnrichmentForge` name lookup, default 0) and `resolveAuto` (CI detect → else git-origin → else offline; `ErrAmbiguousForge` when multiple token candidates and nothing pins a single type). Wrap errors with `%w`. Keep it pure — all env access via `getenv`.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/forge/`
Expected: PASS (all five tests).

- [ ] **Step 6: Full suite + lint + commit**

```bash
go test ./... && hk fix
git add internal/forge/
git commit -m "feat(forge): resolve ForgeIdentity from config/CI/git with ambiguity guard"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] `hk check` (or `mise run lint:check`) → clean.
- [ ] `git grep -n "changelog.remote\|release.platforms\|remote_metadata"` still shows the **old paths intact** (this plan is additive; Plan B removes them).
- [ ] No confidential/real data in any changed file — synthetic placeholders only.
- [ ] Confirm `docs/tasks/forge-abstraction-roadmap.md` T154–T157 can be flipped `[ ]` → `[x]` with completion notes (done as the tasks land, per the two-step roadmap flow).

## Handoff to Plan B

Plan B (**GitLab forge + cutover, T158–T160**) implements `internal/forge/gitlab` (REST default via `net/http` with `JOB-TOKEN`/`PRIVATE-TOKEN` per `TokenKind`; GraphQL opt-in), then wires resolution → forge → enrichment through the pipeline, switches publishing to `release.targets`, adds the **migration error** for the removed `changelog.remote` / `release.platforms` / `commits.remote_metadata` keys, and removes the old paths. Write it (via `superpowers:writing-plans`) once this foundation is merged and battle-tested.
