# `changelog.remote` for native + `base_url` host override — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `changelog.remote` usable with the `native` generator and let it declare a self-hosted host via a unified `base_url` field (replacing `api_url`), so local `heraut changelog --regenerate` on self-hosted GitLab/GHES/on-prem Azure renders correct links and enrichment.

**Architecture:** Three small changes in the config layer + pipeline link-context resolver: (1) rename the `changelog.remote` struct field `api_url` → `base_url` and honor it in `remoteLinkContext()` for all three remote types; (2) remove the validator gate that restricted `changelog.remote` to the `git-cliff` generator; (3) sync schema/sample/fixtures and records. No generator or enrichment plumbing changes — `LinkContext.BaseURL` already drives both link rendering and `APIEnv()` host routing.

**Tech Stack:** Go, `gopkg.in/yaml.v3` (strict loader), `testify`, JSON Schema (`schema.json`), git-cliff/native generators.

## Global Constraints

- TDD mandatory: write the failing test first, run it red, then implement (repo rule `testing.md`).
- No new Go dependencies.
- `internal/config` imports nothing from heraut; `internal/pipeline` may import `internal/{port,config,...}`. Do not add upward imports.
- Never bypass git hooks (`--no-verify`/`--no-gpg-sign` forbidden). Fix lint via `hk fix` (never call `gofmt`/`yamlfmt` directly).
- Config field changes MUST keep `schema.json`, `docs/heraut.sample.yml`, and `docs/specs/` in sync (repo rule `coding.md`).
- Breaking change is intentional and allowed (pre-v1.0): `api_url` is removed, not deprecated. A config still using `api_url` must fail loudly (`unknown key`).
- `base_url` applies to all three remote types: `github` (default `https://github.com`), `gitlab` (default `https://gitlab.com`), `azure_devops` (default `https://dev.azure.com`).
- Out of scope: GitLab commit-author `by @` (remains T150); offline attribution fallback (deferred); any single-platform fallback for `heraut changelog`.
- Commit trailer: `Co-Authored-By: Claude <model> <noreply@anthropic.com>` (no `Claude-Session:` line). Conventional-commit subjects ≤72 chars.

---

### Task 1: `base_url` field + `remoteLinkContext` host override

**Files:**
- Modify: `internal/config/config.go:152-154` (Remote struct: `APIURL` → `BaseURL`)
- Modify: `internal/pipeline/linkctx.go:22-70` (`remoteLinkContext` — honor `base_url` for all three types; add `remoteBaseURL` helper)
- Modify: `internal/config/validator.go:461-466` (rename `api_url` URL validation → `base_url`)
- Test: `internal/pipeline/linkctx_internal_test.go` (`TestRemoteLinkContext` rows)
- Test: `internal/config/validator_test.go` (replace `TestValidate_changelogRemoteInvalidAPIURL`)
- Test: `internal/config/loader_test.go` (new: `api_url` is now an unknown key)
- Test: `internal/generators/native/enrich_gitlab_internal_test.go` (regression guard: `base_url` → `GITLAB_HOST`)

**Interfaces:**
- Consumes: `config.DefaultBaseURL(platformType string) string` (returns `""` for `azure_devops`); `azureDevOpsDefaultBaseURL` const (`internal/pipeline/linkctx.go:15`); `isValidBaseURL(string) bool` (`internal/config/validator.go:541`).
- Produces: `config.Remote.BaseURL string` (yaml `base_url`); `remoteLinkContext` sets `LinkContext.BaseURL` from `r.BaseURL` (per-type default when empty); `remoteBaseURL(configured, platformType string) string` helper in `internal/pipeline`.

- [ ] **Step 1: Write the failing `remoteLinkContext` tests**

In `internal/pipeline/linkctx_internal_test.go`, inside `TestRemoteLinkContext`'s `tests` slice, **replace** the existing row:

```go
		{
			name: "azure_devops honours api_url override",
			r:    &config.Remote{Type: "azure_devops", Project: "group1/sub-group", Repository: "myApp", APIURL: "https://devops.example.com"},
			want: &port.LinkContext{BaseURL: "https://devops.example.com", Owner: "group1/sub-group", Repo: "myApp", Platform: "azure_devops"},
		},
```

with:

```go
		{
			name: "azure_devops honours base_url override",
			r:    &config.Remote{Type: "azure_devops", Project: "group1/sub-group", Repository: "myApp", BaseURL: "https://devops.example.com"},
			want: &port.LinkContext{BaseURL: "https://devops.example.com", Owner: "group1/sub-group", Repo: "myApp", Platform: "azure_devops"},
		},
		{
			name: "github honours base_url (GHES)",
			r:    &config.Remote{Type: "github", Repository: "acme/widget", BaseURL: "https://github.acme.com"},
			want: &port.LinkContext{BaseURL: "https://github.acme.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
		{
			name: "gitlab honours base_url (self-managed, subgroup)",
			r:    &config.Remote{Type: "gitlab", Project: "group/subgroup/project", BaseURL: "https://git.example.com"},
			want: &port.LinkContext{BaseURL: "https://git.example.com", Owner: "group/subgroup", Repo: "release-notes", Platform: "gitlab"},
		},
		{
			name: "base_url trailing slash trimmed",
			r:    &config.Remote{Type: "gitlab", Project: "grp/proj", BaseURL: "https://git.example.com/"},
			want: &port.LinkContext{BaseURL: "https://git.example.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
		},
```

(The existing default-host rows — `github`→`https://github.com`, `gitlab nested namespace`→`https://gitlab.com`, `azure_devops uses project as owner`→`https://dev.azure.com` — stay unchanged and assert the empty-`base_url` fallback.)

- [ ] **Step 2: Run the tests to verify they fail to compile**

Run: `go test ./internal/pipeline/ -run TestRemoteLinkContext`
Expected: **compile failure** — `config.Remote{}.BaseURL` and `.APIURL` referenced but the struct field is still named `APIURL` (the new rows use `BaseURL`). This confirms the test is exercising the not-yet-renamed field.

- [ ] **Step 3: Rename the struct field**

In `internal/config/config.go`, replace the `APIURL` field (lines ~152-154):

```go
	// APIURL overrides the remote's default API host. Only meaningful for
	// type: azure_devops (Azure DevOps Server / on-prem).
	APIURL string `yaml:"api_url,omitempty"`
```

with:

```go
	// BaseURL overrides the remote's default web/API host. Applies to every type:
	// github (github.com / GitHub Enterprise Server), gitlab (gitlab.com /
	// self-managed), and azure_devops (dev.azure.com / on-prem Server). Empty uses
	// the per-type default.
	BaseURL string `yaml:"base_url,omitempty"`
```

- [ ] **Step 4: Honor `base_url` in `remoteLinkContext`**

In `internal/pipeline/linkctx.go`, replace the whole `remoteLinkContext` function body's three branches so each sets `BaseURL` via a new helper, and rewrite the `azure_devops` branch to read `r.BaseURL`:

```go
func remoteLinkContext(r *config.Remote) *port.LinkContext {
	if r == nil {
		return nil
	}
	switch r.Type {
	case "github":
		owner, repo, _ := strings.Cut(r.Repository, "/")
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "github"),
			Owner:    owner,
			Repo:     repo,
			Platform: "github",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, githubDefaultTokenEnv)),
		}
	case "gitlab":
		owner, repo := "", r.Project
		if i := strings.LastIndex(r.Project, "/"); i >= 0 {
			owner, repo = r.Project[:i], r.Project[i+1:]
		}
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "gitlab"),
			Owner:    owner,
			Repo:     repo,
			Platform: "gitlab",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, gitlabDefaultTokenEnv)),
		}
	case "azure_devops":
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "azure_devops"),
			Owner:    r.Project,
			Repo:     r.Repository,
			Platform: "azure_devops",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, azureDevOpsDefaultTokenEnv)),
		}
	default:
		return nil
	}
}

// remoteBaseURL returns the configured base URL (trailing slash trimmed) when set, else the
// per-type default web/API host. azure_devops has no config.DefaultBaseURL entry, so its
// default is applied here.
func remoteBaseURL(configured, platformType string) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	if platformType == "azure_devops" {
		return azureDevOpsDefaultBaseURL
	}
	return config.DefaultBaseURL(platformType)
}
```

- [ ] **Step 5: Fix the validator reference so the tree compiles**

In `internal/config/validator.go`, replace the `api_url` validation block (lines ~461-466):

```go
	if r.APIURL != "" && !isValidBaseURL(strings.TrimRight(r.APIURL, "/")) {
		errs = append(errs, ValidationError{
			Path:    remotePath + ".api_url",
			Message: fmt.Sprintf("%q is not a valid URL", r.APIURL),
			Hint:    "api_url must be an absolute http(s) URL, e.g. https://dev.azure.com",
		})
	}
```

with:

```go
	if r.BaseURL != "" && !isValidBaseURL(strings.TrimRight(r.BaseURL, "/")) {
		errs = append(errs, ValidationError{
			Path:    remotePath + ".base_url",
			Message: fmt.Sprintf("%q is not a valid URL", r.BaseURL),
			Hint:    "base_url must be an absolute http(s) URL, e.g. https://git.example.com",
		})
	}
```

- [ ] **Step 6: Run the `remoteLinkContext` tests to verify they pass**

Run: `go test ./internal/pipeline/ -run TestRemoteLinkContext`
Expected: PASS.

- [ ] **Step 7: Write the failing validator `base_url` test**

In `internal/config/validator_test.go`, **replace** `TestValidate_changelogRemoteInvalidAPIURL` (the whole function) with:

```go
func TestValidate_changelogRemoteInvalidBaseURL(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: gitlab
    project: my-org/my-repo
    base_url: "not-a-url"
`)
	e := findErr(config.Validate(cfg), "changelog.remote.base_url")
	require.NotNil(t, e)
}
```

- [ ] **Step 8: Write the failing loader test (`api_url` now unknown)**

In `internal/config/loader_test.go`, add:

```go
func TestLoadFromReader_rejectsRemovedRemoteAPIURLKey(t *testing.T) {
	src := `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
    api_url: https://dev.azure.com
`
	_, err := config.LoadFromReader(strings.NewReader(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_url")
}
```

- [ ] **Step 9: Run the config tests to verify they pass**

Run: `go test ./internal/config/ -run 'TestValidate_changelogRemoteInvalidBaseURL|TestLoadFromReader_rejectsRemovedRemoteAPIURLKey'`
Expected: PASS (the struct rename from Step 3 makes `api_url` unknown and `base_url` known).

- [ ] **Step 10: Add a regression guard for the base_url → GITLAB_HOST seam**

This asserts the self-hosted host reaches the `glab` enrichment call. It is a regression
guard (green once Task 1's code is in place, since `LinkContext.APIEnv()` already derives
`GITLAB_HOST`); it locks the composed path so a future refactor cannot silently break it.

In `internal/generators/native/enrich_gitlab_internal_test.go`, add:

```go
func TestEnrichGitLab_SelfHostedHostInAPIEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"https://git.example.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	lc := &port.LinkContext{Platform: "gitlab", BaseURL: "https://git.example.com", Owner: "g", Repo: "p", Token: "tok"}

	_, err := enrichGitLab(mr, lc, []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_TOKEN=tok")
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_HOST=git.example.com")
}
```

Run: `go test ./internal/generators/native/ -run TestEnrichGitLab_SelfHostedHostInAPIEnv`
Expected: PASS.

- [ ] **Step 11: Run the full package suites + lint**

Run: `go test ./internal/config/ ./internal/pipeline/ ./internal/generators/native/ && hk fix`
Expected: all PASS; `hk fix` reports no remaining issues.

- [ ] **Step 12: Commit**

```bash
git add internal/config/config.go internal/pipeline/linkctx.go internal/config/validator.go \
        internal/pipeline/linkctx_internal_test.go internal/config/validator_test.go \
        internal/config/loader_test.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(config): base_url host override on changelog.remote (replaces api_url)"
```

---

### Task 2: Lift the git-cliff-only gate on `changelog.remote`

**Files:**
- Modify: `internal/config/validator.go:408-414` (remove the generator gate)
- Test: `internal/config/validator_test.go` (replace `TestValidate_changelogRemoteRequiresGitCliff`)

**Interfaces:**
- Consumes: `validateContentDriverRemote(d *ContentDriver, path string) []ValidationError` (existing); `findErr`, `mustLoad` test helpers.
- Produces: `changelog.remote` is valid for any generator (native + git-cliff); no `remote requires the git-cliff generator` error.

- [ ] **Step 1: Write the failing "native allowed" test**

In `internal/config/validator_test.go`, **replace** `TestValidate_changelogRemoteRequiresGitCliff` (which used the dropped `communique` generator) with:

```go
func TestValidate_changelogRemoteAllowsNative(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
  remote:
    type: gitlab
    project: my-org/my-repo
    base_url: https://git.example.com
`)
	errs := config.Validate(cfg)
	for _, e := range errs {
		assert.NotContains(t, e.Path, "remote", "unexpected remote error: %+v", e)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/config/ -run TestValidate_changelogRemoteAllowsNative`
Expected: FAIL — a `ValidationError` at path `changelog.remote` with message `remote requires the git-cliff generator` is still produced.

- [ ] **Step 3: Remove the generator gate**

In `internal/config/validator.go`, delete this block (lines ~408-414):

```go
	if d.Generator != "" && d.Generator != "git-cliff" {
		errs = append(errs, ValidationError{
			Path:    remotePath,
			Message: "remote requires the git-cliff generator",
			Hint:    fmt.Sprintf("set generator to git-cliff, or remove remote (current generator: %s)", d.Generator),
		})
	}
```

Leave the surrounding `var errs []ValidationError` declaration and the `r := d.Remote` line intact. (The `.notes`-driver rejection earlier in the function is unchanged.)

- [ ] **Step 4: Verify no now-unused import**

Run: `go build ./internal/config/`
Expected: PASS. (`fmt` is still used elsewhere in the file — e.g. the base_url and type messages — so no import removal is needed. If the build reports `fmt` unused, remove it from the import block; it should not.)

- [ ] **Step 5: Run the config suite + lint**

Run: `go test ./internal/config/ && hk fix`
Expected: all PASS. Confirm the existing `TestValidate_changelogRemoteAzureDevOpsValid` and the other remote validation tests still pass (they use `generator: git-cliff`, still valid).

- [ ] **Step 6: Commit**

```bash
git add internal/config/validator.go internal/config/validator_test.go
git commit -m "feat(config): allow changelog.remote with the native generator"
```

---

### Task 3: Sync schema, sample, and fixtures

**Files:**
- Modify: `schema.json` (`definitions.Remote`: remove `api_url`, add `base_url`; update `Remote` + `ContentDriver.remote` descriptions)
- Modify: `docs/heraut.sample.yml:190-205` (remote example: `api_url` → `base_url`, all-types note, gitlab self-hosted example)
- Modify: `testdata/config/valid/changelog-remote.yml` (add a `base_url` line to exercise it)
- Create: `testdata/config/valid/changelog-remote-native.yml` (native + gitlab + self-hosted `base_url`)
- Create: `testdata/config/invalid/remote_api_url_removed.yml` (uses `api_url` → additionalProperties violation)
- Modify: `internal/config/schema_test.go:71-78` (add the invalid-fixture table row)

**Interfaces:**
- Consumes: `config.Remote.BaseURL` (from Task 1); `TestSchema_ValidFixtures` globs `testdata/config/valid/*.yml`; `TestSchema_InvalidFixtures` iterates a hardcoded table.
- Produces: schema `Remote.base_url` property; two new fixtures.

- [ ] **Step 1: Update `schema.json` Remote definition**

In `schema.json`, in `definitions.Remote.properties`, remove the `api_url` property:

```json
    "api_url": {
      "type": "string",
      "description": "API/web host override. Azure DevOps Server (on-prem) only; defaults to https://dev.azure.com."
    }
```

and add in its place:

```json
    "base_url": {
      "type": "string",
      "description": "Web/API host override for all types: github (github.com / GHES), gitlab (gitlab.com / self-managed), azure_devops (dev.azure.com / on-prem Server). Must be an absolute http(s) URL. Defaults per type when unset."
    }
```

Also update the `ContentDriver.remote` description (`schema.json:306`) from the current git-cliff-only wording to:

```json
        "description": "Explicit metadata remote for PR/author enrichment and link hosts (git-cliff or native). Changelog only — release notes resolve this from release.platforms and reject this block. See ADR-0026 / ADR-0040."
```

(Keep the `"$ref": "#/definitions/Remote"` sibling exactly as it is.)

- [ ] **Step 2: Update the `Remote` definition description**

In `schema.json`, `definitions.Remote.description`, keep the existing "metadata-only remote ... never grants publish capability" sentence — it is still accurate. No change required unless it mentions git-cliff (it does not). Skip if unchanged.

- [ ] **Step 3: Add `base_url` to the existing valid fixture**

In `testdata/config/valid/changelog-remote.yml`, add a `base_url` line under the remote block:

```yaml
changelog:
  generator: git-cliff
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
    base_url: https://devops.example.com
```

- [ ] **Step 4: Create the native + self-hosted fixture**

Create `testdata/config/valid/changelog-remote-native.yml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver

changelog:
  generator: native
  output: CHANGELOG.md
  remote:
    type: gitlab
    project: group/subgroup/project
    base_url: https://git.example.com
```

- [ ] **Step 5: Create the invalid `api_url` fixture + register it**

Create `testdata/config/invalid/remote_api_url_removed.yml`:

```yaml
version: "1"

versioning:
  strategy: semver

changelog:
  generator: git-cliff
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
    api_url: https://dev.azure.com
```

In `internal/config/schema_test.go`, add a row to the `TestSchema_InvalidFixtures` table (after the `invalid_remote_type.yml` row, ~line 76):

```go
		{"remote_api_url_removed.yml", "changelog.remote.api_url additionalProperties violation"},
```

- [ ] **Step 6: Update `docs/heraut.sample.yml`**

In `docs/heraut.sample.yml`, in the commented `remote:` example (~line 204), replace:

```yaml
  #   api_url: https://dev.azure.com # optional; Azure DevOps Server (on-prem) only
```

with:

```yaml
  #   base_url: https://git.example.com # optional host override; all types (GHES /
  #                                     # self-managed GitLab / on-prem Azure). Absolute http(s) URL.
```

- [ ] **Step 7: Run the schema tests to verify all fixtures behave**

Run: `go test ./internal/config/ -run TestSchema`
Expected: PASS — the two new valid fixtures validate; `remote_api_url_removed.yml` fails schema validation (additionalProperties).

- [ ] **Step 8: Lint (yaml + json via hk)**

Run: `hk fix`
Expected: no remaining issues. (Do not run `yamlfmt`/`gofmt` directly.)

- [ ] **Step 9: Commit**

```bash
git add schema.json docs/heraut.sample.yml testdata/config/valid/changelog-remote.yml \
        testdata/config/valid/changelog-remote-native.yml \
        testdata/config/invalid/remote_api_url_removed.yml internal/config/schema_test.go
git commit -m "docs(config): sync schema + sample + fixtures for remote base_url"
```

---

### Task 4: ADR-0040 + spec + roadmap

**Files:**
- Create: `docs/adr/0040-changelog-remote-native-base-url.md`
- Modify: `docs/specs/05-generators-and-platforms.md:200-210` (remote block: native support, `api_url` → `base_url`)
- Modify: `docs/tasks/native-generator-roadmap.md` (add a task entry + completion note)

**Interfaces:**
- Consumes: the final behavior from Tasks 1-3.
- Produces: records only (no code).

- [ ] **Step 1: Write ADR-0040**

Create `docs/adr/0040-changelog-remote-native-base-url.md`:

```markdown
# ADR-0040: `changelog.remote` for native + unified `base_url` host override

- **Status**: Accepted
- **Date**: 2026-07-22
- **Deciders**: bchatard

---

## Context

`changelog.remote` (ADR-0026) let users declare an explicit metadata remote for the
changelog, but validation restricted it to the `git-cliff` generator, and its only host
override (`api_url`) was honored for `azure_devops` only. Running `heraut changelog
--regenerate` locally with `generator: native` against a self-hosted GitLab produced a
changelog with no links and no enrichment: the changelog-only pipeline's link-context
chain is `changelog.remote → ambient CI`, and locally both were unavailable — the block
was rejected for native, and `remoteLinkContext()` hardcoded `gitlab.com`.

## Decision

1. **Remove the git-cliff-only gate.** `changelog.remote` is valid for any changelog
   generator. Both native and git-cliff consume the resolved `port.LinkContext`; the
   restriction was historical.
2. **Replace `api_url` with `base_url`.** One host override applies to all three remote
   types — `github` (default `https://github.com`), `gitlab` (`https://gitlab.com`),
   `azure_devops` (`https://dev.azure.com`). `remoteLinkContext()` uses it for every type;
   because `LinkContext.BaseURL` already drives both link rendering and `APIEnv()`
   (`GITLAB_HOST`/`GH_HOST`), self-hosted enrichment works with no new plumbing.
3. **Breaking, no shim (pre-v1.0).** `api_url` is removed; a config still using it fails
   strict loading with `unknown key`. Only Azure DevOps Server users set it; the rename to
   `base_url` is mechanical.

## Consequences

- Local self-hosted changelog regeneration now renders commit links, compare/tag links,
  and `in [!N]` MR references against the configured host.
- `by @` commit-author attribution for GitLab is **unchanged** — still unresolved
  (roadmap T150, GitHub-only today). This ADR does not address it.
- Supersedes the `api_url`-specific guidance in ADR-0026 and the `changelog.remote.api_url`
  reference in ADR-0035.
```

- [ ] **Step 2: Update the behavioural spec**

In `docs/specs/05-generators-and-platforms.md`, locate the `changelog.remote` block (~line 200-210) and update it: change the `api_url` line to `base_url`, note it applies to all three types (not azure-only), and add a sentence that the block is valid with both the `git-cliff` and `native` generators. Concretely, replace:

```
    api_url: https://dev.azure.com  # optional, Azure DevOps Server (on-prem) only
```

with:

```
    base_url: https://git.example.com  # optional host override; all types (GHES /
                                       # self-managed GitLab / on-prem Azure). Absolute http(s) URL.
```

and add, near the block's prose, a line: "Valid with both the `git-cliff` and `native`
generators (ADR-0040)."

- [ ] **Step 3: Add the roadmap task + note**

In `docs/tasks/native-generator-roadmap.md`, add a completed task entry near the Phase 2 remote/enrichment section:

```markdown
#### `[x]` T152: changelog.remote for native + base_url host override (ADR-0040)

`heraut changelog --regenerate` locally on self-hosted GitLab produced no links/attribution
because the changelog-only pipeline's link-context chain is `changelog.remote → ambient`,
and locally the block was rejected for native and `remoteLinkContext()` hardcoded gitlab.com.
Lifted the git-cliff-only gate on `changelog.remote` and replaced `api_url` with a unified
`base_url` host override across github/gitlab/azure_devops. Because `LinkContext.BaseURL`
already drives both links and `APIEnv()` host routing, no generator/enrichment plumbing
changed. Breaking (pre-v1.0): `api_url` removed. GitLab commit-author `by @` stays out of
scope (T150); an offline attribution fallback was deferred. **Scope:** S. **Dependencies:**
Phase 2.7 (unified enrichment), ADR-0026.
```

- [ ] **Step 4: Verify docs build cleanly (lint)**

Run: `hk fix`
Expected: no issues (markdown/typos pass).

- [ ] **Step 5: Commit**

```bash
git add docs/adr/0040-changelog-remote-native-base-url.md docs/specs/05-generators-and-platforms.md docs/tasks/native-generator-roadmap.md
git commit -m "docs(adr): 0040 changelog.remote for native + base_url"
```

---

## Final verification (after all tasks)

- [ ] Run the full suite: `go test ./...` → all PASS.
- [ ] Run `hk check` (or `mise run lint:check`) → clean.
- [ ] Manual integration check against `/tmp/release-notes`: add
  `base_url: https://git.example.com` under `changelog.remote` (type gitlab, project
  `group/subgroup/project`) with `generator: native`, then run
  `./heraut changelog --regenerate --verbose` and confirm (a) `glab api` calls now appear in
  the trace, (b) version headers render `## [x.y.z](…git.example.com/…/compare/…)`,
  (c) commit lines render `([sha](…git.example.com/…/commit/…))`, (d) `in [!N]` MR
  references appear. Expect **no** `by @` (T150). Do not commit or push in that repo.
```
