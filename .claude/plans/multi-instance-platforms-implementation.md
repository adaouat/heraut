# Multi-instance same-platform releases — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `release.platforms` (and per-env overrides) to contain multiple entries of
the same platform type — e.g. a public `gitlab.com` instance and a self-hosted
`gitlab.example.com` instance — by adding a required, unique `name` field per entry,
lifting ADR-0020's "self-hosted hosts are not yet supported" gate, and routing each
platform's `gh`/`glab` calls to the correct host via injected `GH_HOST`/`GITLAB_HOST`
(and `GH_ENTERPRISE_TOKEN` for GitHub Enterprise).

**Architecture:** No new packages. `config.Platform` gains a `Name` field (required,
unique per `release.platforms` scope). `internal/config/validator.go` gains a shared
`validatePlatformEntries` helper and drops the base_url default-only gate.
`internal/platforms/{gitlab,github}/platform.go` gain a `hostEnv() []string` helper that
is merged into every `RunEnv` call when `cfg.BaseURL` is non-default; `Name()` returns
`cfg.Name`; `ReleaseURL()`/`Check()` honor `cfg.BaseURL`. `internal/app/check.go`'s
Platforms section is restructured from "one row per platform *type*" to "one row per
`release.platforms` *entry*", removing `configuredPlatforms`/`findPlatformCfg`. Docs
(`schema.json`, `docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`,
new ADR-0025 superseding ADR-0020) are updated last.

**Tech Stack:** Go, `testify` (assert/require), `exectest.MockRunner` / `exectest.FakeBin`,
`yaml.v3`, JSON Schema.

**Design reference:** [`.claude/plans/multi-instance-platforms-design.md`](multi-instance-platforms-design.md)

---

## Task 1 (T83): Config schema — required `name` field + lift `base_url` gate

**Files:**
- Modify: `internal/config/config.go:100-115` (add `Name` field to `Platform`)
- Modify: `internal/config/validator.go:180-275` (new `validatePlatformEntries` helper;
  trim `validatePlatformBaseURL`; rewire `validateRelease`/`validateEnvRelease`)
- Modify: `internal/config/validator_test.go` (new tests + updated fixtures)
- Modify: `testdata/config/valid/semver.yml`, `testdata/config/valid/calver.yml`,
  `testdata/config/valid/semver-per-env.yml`, `testdata/config/valid/platform-base-url.yml`
- Modify: `.config/heraut.yml`
- Modify: `internal/cmd/check_test.go`, `internal/cmd/release_test.go`
- Modify: `schema.json`, `docs/heraut.sample.yml`
- Modify: `internal/scaffold/generate.go`, `internal/scaffold/generate_test.go`

**Dependencies:** none (first task).

---

- [ ] **Step 1: Write failing tests for the new `name` field**

Open `internal/config/validator_test.go` and insert these two new tests directly after
`TestValidate_multiplePlatforms` (ends at line 325), before the
`// ── platform base_url (ADR-0020) ──` comment (line 327):

```go
func TestValidate_platformNameRequired(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      repository: acme/widget
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_platformNameDuplicate(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: primary
      project: acme/widget
    - platform: gitlab
      name: primary
      project: tools/widget-mirror
      base_url: https://gitlab.example.com
      token_env: GITLAB_INTERNAL_TOKEN
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[1].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/config/... -run 'TestValidate_platformName' -v`

Expected: both `FAIL` — `findErr(errs, "release.platforms[0].name")` returns `nil` because
`config.Platform` has no `Name` field yet (the YAML key `name` is silently rejected by
strict parsing... actually it will fail at `mustLoad` with an "unknown field" error,
since the loader rejects unknown keys). Either way, both tests fail at this point.

- [ ] **Step 3: Add the `Name` field to `config.Platform`**

In `internal/config/config.go`, the `Platform` struct currently reads (lines 100-115):

```go
type Platform struct {
	Type string `yaml:"platform"`
	// GitHub-specific
	Repository string `yaml:"repository,omitempty"`
	Draft      bool   `yaml:"draft,omitempty"`
	Prerelease bool   `yaml:"prerelease,omitempty"`
	// GitLab-specific
	Project string `yaml:"project,omitempty"`
	// Shared
	BaseURL  string   `yaml:"base_url,omitempty"`
	TokenEnv string   `yaml:"token_env,omitempty"`
	Assets   []string `yaml:"assets,omitempty"`
	// LenientAssets is set programmatically when assets come from release.assets (top-level).
	// When true, a glob pattern that matches nothing emits a warning instead of an error.
	LenientAssets bool `yaml:"-"`
}
```

Change it to add `Name` as the first field:

```go
type Platform struct {
	// Name uniquely identifies this platform entry within its release.platforms list
	// (top-level or a single environment override). Required — see ADR-0025.
	Name string `yaml:"name"`
	Type string `yaml:"platform"`
	// GitHub-specific
	Repository string `yaml:"repository,omitempty"`
	Draft      bool   `yaml:"draft,omitempty"`
	Prerelease bool   `yaml:"prerelease,omitempty"`
	// GitLab-specific
	Project string `yaml:"project,omitempty"`
	// Shared
	BaseURL  string   `yaml:"base_url,omitempty"`
	TokenEnv string   `yaml:"token_env,omitempty"`
	Assets   []string `yaml:"assets,omitempty"`
	// LenientAssets is set programmatically when assets come from release.assets (top-level).
	// When true, a glob pattern that matches nothing emits a warning instead of an error.
	LenientAssets bool `yaml:"-"`
}
```

- [ ] **Step 4: Run the new tests again to verify the duplicate test now fails differently**

Run: `go test ./internal/config/... -run 'TestValidate_platformName' -v`

Expected: `mustLoad` no longer errors (the `name` key now parses). Both tests still `FAIL`,
now because `config.Validate` returns no error at path `release.platforms[0].name` /
`release.platforms[1].name` — the validator does not check `Name` yet.

- [ ] **Step 5: Add `validatePlatformEntries` and rewire the validators**

In `internal/config/validator.go`, replace `validateRelease` (lines 180-204),
`validatePlatformBaseURL` (lines 206-233), and the platform loop inside
`validateEnvRelease` (lines 257-273) as follows.

Replace `validateRelease` (lines 180-204) with:

```go
func validateRelease(r *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	errs = append(errs, validateContentDriver(r.Notes, path+".notes")...)
	errs = append(errs, validatePlatformEntries(r.Platforms, path)...)
	return errs
}

// validatePlatformEntries validates one release.platforms list (top-level or a single
// environment override): each entry's platform type, its required unique name
// (scoped to this list — ADR-0025), and its base_url.
func validatePlatformEntries(platforms []Platform, path string) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]int)
	for i, plat := range platforms {
		platPath := fmt.Sprintf("%s.platforms[%d]", path, i)
		if plat.Type == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab",
			})
		} else if !validPlatforms[plat.Type] {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", plat.Type),
				Hint:    "valid platforms: github, gitlab",
			})
		}
		if plat.Name == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".name",
				Message: "required",
				Hint:    `set a unique name for this platform entry, e.g. "gitlab-saas"`,
			})
		} else if first, ok := seen[plat.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    platPath + ".name",
				Message: fmt.Sprintf("duplicate platform name %q (already used by platforms[%d])", plat.Name, first),
				Hint:    "platform names must be unique within this release.platforms list",
			})
		} else {
			seen[plat.Name] = i
		}
		errs = append(errs, validatePlatformBaseURL(plat, platPath)...)
	}
	return errs
}
```

Replace `validatePlatformBaseURL` (lines 206-233) with:

```go
// validatePlatformBaseURL validates a platform's base_url: it must be a well-formed
// absolute http(s) URL. An empty value means "use the platform-type default" and is
// always accepted. Self-hosted (non-default) hosts are accepted — see ADR-0025, which
// supersedes ADR-0020's gate.
func validatePlatformBaseURL(plat Platform, platPath string) []ValidationError {
	if plat.BaseURL == "" {
		return nil
	}
	raw := strings.TrimRight(plat.BaseURL, "/")
	if !isValidBaseURL(raw) {
		return []ValidationError{{
			Path:    platPath + ".base_url",
			Message: fmt.Sprintf("%q is not a valid URL", plat.BaseURL),
			Hint:    "base_url must be an absolute http(s) URL, e.g. https://gitlab.example.com",
		}}
	}
	return nil
}
```

In `validateEnvRelease`, replace the platform loop (lines 257-273):

```go
	for i, plat := range r.Platforms {
		platPath := fmt.Sprintf("%s.platforms[%d]", path, i)
		if plat.Type == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab",
			})
		} else if !validPlatforms[plat.Type] {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", plat.Type),
				Hint:    "valid platforms: github, gitlab",
			})
		}
		errs = append(errs, validatePlatformBaseURL(plat, platPath)...)
	}
```

with a single call:

```go
	errs = append(errs, validatePlatformEntries(r.Platforms, path)...)
```

- [ ] **Step 6: Run the new tests to verify they pass**

Run: `go test ./internal/config/... -run 'TestValidate_platformName' -v`

Expected: both `PASS`.

- [ ] **Step 7: Run the full config package test suite to find now-failing tests**

Run: `go test ./internal/config/... -v 2>&1 | rtk proxy grep -E 'FAIL|--- FAIL'`

Expected `FAIL`:
- `TestValidate_multiplePlatforms` (missing `name:` on both entries)
- `TestValidate_platformBaseURLDefaultApplied`
- `TestValidate_platformBaseURLDefaultAppliedGitLab`
- `TestValidate_platformBaseURLExplicitDefault`
- `TestValidate_platformBaseURLTrailingSlashNormalized`
- `TestValidate_platformBaseURLNonDefaultGated` (now wrongly expects the lifted gate)
- `TestValidate_envOverridePlatformBaseURLGated` (same)
- `TestValidate_validFixtures` (5 fixtures, 3 of which define platforms without `name:`)

- [ ] **Step 8: Fix `TestValidate_multiplePlatforms` and the base_url-default tests**

In `internal/config/validator_test.go`:

`TestValidate_multiplePlatforms` (lines 314-325) — add `name:` to both entries:

```go
func TestValidate_multiplePlatforms(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
    - platform: gitlab
      name: gitlab
`)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_platformBaseURLDefaultApplied` (lines 329-343) — add `name: github`:

```go
func TestValidate_platformBaseURLDefaultApplied(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
`)
	require.NotNil(t, cfg.Release)
	require.Len(t, cfg.Release.Platforms, 1)
	assert.Equal(t, "https://github.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_platformBaseURLDefaultAppliedGitLab` (lines 345-358) — add `name: gitlab`:

```go
func TestValidate_platformBaseURLDefaultAppliedGitLab(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab
      project: acme/widget
`)
	require.Len(t, cfg.Release.Platforms, 1)
	assert.Equal(t, "https://gitlab.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_platformBaseURLExplicitDefault` (lines 360-372) — add `name: gitlab`:

```go
func TestValidate_platformBaseURLExplicitDefault(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab
      project: acme/widget
      base_url: https://gitlab.com
`)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_platformBaseURLTrailingSlashNormalized` (lines 374-387) — add `name: github`:

```go
func TestValidate_platformBaseURLTrailingSlashNormalized(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      base_url: https://github.com/
`)
	assert.Equal(t, "https://github.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}
```

- [ ] **Step 9: Replace the two "gated" tests with "self-hosted accepted" tests**

`TestValidate_platformBaseURLNonDefaultGated` (lines 389-405) becomes
`TestValidate_platformBaseURLSelfHostedAccepted`:

```go
func TestValidate_platformBaseURLSelfHostedAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
`)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_envOverridePlatformBaseURLGated` (lines 425-444) becomes
`TestValidate_envOverridePlatformBaseURLSelfHostedAccepted`:

```go
func TestValidate_envOverridePlatformBaseURLSelfHostedAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    release:
      platforms:
        - platform: gitlab
          name: gitlab-internal
          project: acme/widget
          base_url: https://gitlab.example.com
`)
	assert.Empty(t, config.Validate(cfg))
}
```

`TestValidate_platformBaseURLMalformed` (lines 407-423) is unaffected (no `name:`
required to reproduce the malformed-URL error path — but add `name: github` anyway for
consistency with the other base_url tests):

```go
func TestValidate_platformBaseURLMalformed(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      base_url: "not a url"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].base_url")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "not a valid URL")
	assert.NotContains(t, e.Message, "not yet supported")
}
```

- [ ] **Step 10: Run the config package tests again**

Run: `go test ./internal/config/... -v 2>&1 | rtk proxy grep -E 'FAIL|--- FAIL'`

Expected remaining `FAIL`: `TestValidate_validFixtures` only (fixture YAML files not yet
updated).

- [ ] **Step 11: Add `name:` to the 4 affected fixture files**

`testdata/config/valid/semver.yml` (full file, 22 lines) — add `name: github` to the one
platform entry:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump: auto

changelog:
  generator: git-cliff
  output: CHANGELOG.md

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      token_env: GH_TOKEN
```

`testdata/config/valid/calver.yml` (full file, 17 lines) — add `name: gitlab`:

```yaml
version: "1"

versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""

changelog:
  generator: git-cliff
  output: CHANGELOG.md

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: gitlab
      name: gitlab
```

`testdata/config/valid/semver-per-env.yml` (full file, 39 lines) — add `name:` to all 4
platform entries (`dev`, `prod` ×2, top-level):

```yaml
version: "1"

versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"

environments:
  dev:
    branch: develop
    bump: auto
    disable_changelog: true
    release:
      platforms:
        - platform: gitlab
          name: gitlab
  staging:
    branch: main
    bump: promote
    source: dev
  prod:
    branch: main
    bump: promote
    source: staging
    release:
      platforms:
        - platform: gitlab
          name: gitlab
        - platform: github
          name: github

changelog:
  generator: git-cliff
  output: CHANGELOG.md
  tag_pattern: "dev/*"

release:
  notes:
    generator: git-cliff
    tag_pattern: "dev/*"
  platforms:
    - platform: gitlab
      name: gitlab
```

`testdata/config/valid/platform-base-url.yml` (full file, 27 lines) — add `name:` to both
entries:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump: auto

changelog:
  generator: git-cliff
  output: CHANGELOG.md

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      base_url: https://github.com
      token_env: GH_TOKEN
    - platform: gitlab
      name: gitlab
      project: acme/widget
      base_url: https://gitlab.com
      token_env: GITLAB_TOKEN
```

`testdata/config/valid/calver-per-env.yml` has no `platforms:` block — no change needed.

- [ ] **Step 12: Run the config package tests once more**

Run: `go test ./internal/config/... -v 2>&1 | rtk proxy grep -E 'FAIL|--- FAIL'`

Expected: no output (all pass).

- [ ] **Step 13: Add `name:` to `.config/heraut.yml` (this repo's own dogfooding config)**

`.config/heraut.yml` line 18-19 currently:

```yaml
  platforms:
    - platform: github
      repository: adaouat/heraut
```

Change to:

```yaml
  platforms:
    - platform: github
      name: github
      repository: adaouat/heraut
```

- [ ] **Step 14: Add `name:` to `internal/cmd/check_test.go` and `internal/cmd/release_test.go`**

`internal/cmd/check_test.go` line 143:

```yaml
    - platform: github
```

becomes:

```yaml
    - platform: github
      name: github
```

`internal/cmd/release_test.go` has 5 occurrences of:

```yaml
    - platform: github
      repository: test/repo
```

(at lines 59-60, 93-94, 148-149, 178-179, 207-208). For each, change to:

```yaml
    - platform: github
      name: github
      repository: test/repo
```

- [ ] **Step 15: Run the full test suite to confirm the cmd and config packages are green**

Run: `go test ./internal/cmd/... ./internal/config/... 2>&1 | rtk proxy grep -E 'FAIL|ok'`

Expected: `ok` for both packages, no `FAIL`.

- [ ] **Step 16: Update `schema.json`**

In `schema.json`, the `Platform` definition (around line 203-249) currently has
`"required": ["platform"]` and no `name` property. Update the `required` array and add a
`name` property as the first property:

```json
      "type": "object",
      "required": [
        "platform",
        "name"
      ],
      "additionalProperties": false,
      "properties": {
        "name": {
          "type": "string",
          "description": "Unique identifier for this platform entry within its release.platforms list (top-level or a single environment override). Required."
        },
        "platform": {
          "type": "string",
          "enum": [
            "github",
            "gitlab"
          ],
          "description": "Platform type."
        },
```

(leave `repository`, `draft`, `prerelease`, `project` unchanged). Then update the
`base_url` property description (around line 233-236):

```json
        "base_url": {
          "type": "string",
          "description": "Web base URL of the platform instance (e.g. https://gitlab.example.com for a self-hosted instance). Defaults to https://github.com / https://gitlab.com. Self-hosted (non-default) values are supported — see ADR-0025."
        },
```

- [ ] **Step 17: Update `docs/heraut.sample.yml`**

In `docs/heraut.sample.yml`, the GitHub platform block (lines 161-187) and the commented
GitLab block (lines 189-206) need a `name:` field documented, and the `base_url` comment
(lines 171-175) needs the "not yet supported" caveat removed. Replace lines 160-206:

```yaml
  platforms:
    # ── GitHub ───────────────────────────────────────────────────────────────
    - platform: github

      # name — required. Unique identifier for this entry within this
      # release.platforms list (top-level, or a single environment override).
      # Used to label this platform in `heraut check runtime` output.
      name: github

      # repository — owner/repo on GitHub. Required.
      repository: acme/widget

      # token_env — env var holding a GitHub token with contents:write scope.
      # Defaults to GH_TOKEN if omitted.
      token_env: GH_TOKEN

      # base_url — web base URL of the platform instance. Defaults to
      # https://github.com (GitHub) / https://gitlab.com (GitLab).
      # Self-hosted (non-default) values are supported (ADR-0025): heraut points
      # gh/glab at the configured host via GH_HOST / GITLAB_HOST.
      # base_url: https://github.com

      # draft — create the release as a draft (not publicly visible until published).
      # draft: true

      # prerelease — mark as a pre-release on the GitHub UI.
      # prerelease: true

      # assets — platform-specific glob patterns. When set, overrides release.assets
      # entirely for this platform (no merging). Omit to inherit release.assets.
      # assets:
      #   - dist/myapp_*
      #   - dist/checksums.txt

  # ── GitLab ───────────────────────────────────────────────────────────────
  # - platform: gitlab
  #
  #   # name — required, unique within this release.platforms list.
  #   name: gitlab
  #
  #   # project — GitLab namespace/project path.
  #   # Defaults to $CI_PROJECT_PATH if omitted (convenient in CI).
  #   project: acme/widget
  #
  #   # token_env — env var holding a GitLab token with api scope.
  #   # Defaults to GITLAB_TOKEN if omitted.
  #   token_env: GITLAB_TOKEN
  #
  #   # base_url — defaults to https://gitlab.com. Self-hosted values are
  #   # supported (ADR-0025).
  #   # base_url: https://gitlab.com
  #
  #   # assets — platform-specific overrides (overrides release.assets entirely for this platform).
  #   # assets:
  #   #   - dist/myapp_*

  # ── Multiple instances of the same platform type (ADR-0025) ───────────────
  # Two entries with the same `platform:` but different `name:`, `base_url:`,
  # and `token_env:` publish the release to both instances:
  # - platform: gitlab
  #   name: gitlab-saas
  #   project: acme/widget
  # - platform: gitlab
  #   name: gitlab-internal
  #   project: tools/widget-mirror
  #   base_url: https://gitlab.example.com
  #   token_env: GITLAB_INTERNAL_TOKEN
```

- [ ] **Step 18: Write a failing test for default + deduped platform names in `heraut init`**

In `internal/scaffold/generate_test.go`, add a new test after `TestGenerateYAML_PerEnv`
(after line 130, i.e. after its closing `}`):

```go
func TestGenerateYAML_PlatformNamesDefaultedAndDeduped(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		Platforms: []scaffold.PlatformAnswer{
			{Type: "github", Repository: "acme/widget"},
			{Type: "gitlab", Project: "acme/widget"},
			{Type: "gitlab", Project: "tools/widget-mirror"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)
	assert.Empty(t, config.Validate(cfg))

	require.Len(t, cfg.Release.Platforms, 3)
	assert.Equal(t, "github", cfg.Release.Platforms[0].Name)
	assert.Equal(t, "gitlab", cfg.Release.Platforms[1].Name)
	assert.Equal(t, "gitlab-2", cfg.Release.Platforms[2].Name)
}
```

- [ ] **Step 19: Run the new test to verify it fails**

Run: `go test ./internal/scaffold/... -run TestGenerateYAML_PlatformNamesDefaultedAndDeduped -v`

Expected: `FAIL` — `config.Validate(cfg)` returns errors at `release.platforms[*].name`
("required"), since `generate.go` does not set `Name` yet.

- [ ] **Step 20: Set `Name` (defaulted + deduped) in `internal/scaffold/generate.go`**

In `internal/scaffold/generate.go`, the platform-building loop (lines 97-105) currently
reads:

```go
		for _, p := range a.Platforms {
			plat := config.Platform{
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
			}
			cfg.Release.Platforms = append(cfg.Release.Platforms, plat)
		}
```

Replace it with:

```go
		platformTypeCount := make(map[string]int)
		for _, p := range a.Platforms {
			platformTypeCount[p.Type]++
			name := p.Type
			if n := platformTypeCount[p.Type]; n > 1 {
				name = fmt.Sprintf("%s-%d", p.Type, n)
			}
			plat := config.Platform{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
			}
			cfg.Release.Platforms = append(cfg.Release.Platforms, plat)
		}
```

`fmt` is already imported in `generate.go` (line 5).

- [ ] **Step 21: Run the scaffold test suite to verify everything passes**

Run: `go test ./internal/scaffold/... -v 2>&1 | rtk proxy grep -E 'FAIL|ok'`

Expected: `ok`, no `FAIL`. (`TestGenerateYAML_SemVer`, `_CalVer`, `_PerEnv`,
`_CalVerSprint`, `_NoPlatforms` continue to pass because each now produces a `Name` that
satisfies the new required+unique constraint.)

- [ ] **Step 22: Run the full project test suite and linters**

Run: `go test ./... 2>&1 | rtk proxy grep -E 'FAIL|ok' ` then `mise run lint:check`

Expected: all packages `ok`, lint clean.

- [ ] **Step 23: Commit**

```bash
git add internal/config/config.go internal/config/validator.go internal/config/validator_test.go \
  testdata/config/valid/semver.yml testdata/config/valid/calver.yml \
  testdata/config/valid/semver-per-env.yml testdata/config/valid/platform-base-url.yml \
  .config/heraut.yml internal/cmd/check_test.go internal/cmd/release_test.go \
  schema.json docs/heraut.sample.yml internal/scaffold/generate.go internal/scaffold/generate_test.go
git commit -m "$(cat <<'EOF'
feat(config): require unique platform name, allow self-hosted base_url

Adds a required `name` field to release.platforms entries (unique per
list scope) and lifts ADR-0020's "self-hosted hosts are not yet
supported" gate on base_url — both are prerequisites for publishing to
two instances of the same platform type (ADR-0025).

Roadmap: docs/tasks/roadmap.md → T83
EOF
)"
```

---
## Task 2 (T84): GitLab platform — `hostEnv()`, `Name()`, `ReleaseURL()` honor config

**Files:**
- Modify: `internal/platforms/gitlab/platform.go` (full file, 206 lines)
- Modify: `internal/platforms/gitlab/platform_test.go`

**Dependencies:** T83 (`config.Platform.Name` and unrestricted `base_url` must exist).

---

- [ ] **Step 1: Write failing tests**

In `internal/platforms/gitlab/platform_test.go`:

Replace `TestName` (lines 17-20) — `Name()` must now return the configured name, not a
hardcoded constant:

```go
func TestName(t *testing.T) {
	p := gitlab.New(exectest.NewMockRunner(), &config.Platform{Name: "gitlab-internal"})
	assert.Equal(t, "gitlab-internal", p.Name())
}
```

Add the following new tests after `TestReleaseURL_FromEnv` (after line 32):

```go
func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Project: "grp/repo", BaseURL: "https://gitlab.example.com"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.example.com/grp/repo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}
```

Add the following new tests at the end of the file (after `TestCheck_Auth_InCI_NoProjectID`,
which ends at line 373):

```go

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

func TestCreateRelease_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
}

func TestUploadAssets_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{assetPath},
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
}

func TestCheck_SelfHosted_SkipsCIAutologin(t *testing.T) {
	// Even when GITLAB_CI=true, a self-hosted instance must not rely on glab's CI
	// autologin (which targets the gitlab.com job token) — it always authenticates
	// via the configured token, with GITLAB_HOST pointing glab at the right host.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "42")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)   // --version
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // glab api user

	t.Setenv("GITLAB_TOKEN", "ci-token")
	p := gitlab.New(mr, &config.Platform{
		TokenEnv: "GITLAB_TOKEN",
		Project:  "grp/repo",
		BaseURL:  "https://gitlab.example.com",
	})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "user"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=ci-token", "GITLAB_HOST=gitlab.example.com"}, mr.Calls[1].Env)
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/platforms/gitlab/... -run 'TestName|TestReleaseURL_SelfHosted|TestCreateRelease_SelfHosted|TestUploadAssets_SelfHosted|TestCheck_SelfHosted' -v`

Expected: all `FAIL` —
- `TestName`: `Name()` returns `"gitlab"` (hardcoded), not `"gitlab-internal"`.
- `TestReleaseURL_SelfHosted`: `ReleaseURL` uses the hardcoded `gitlabBaseURL` constant.
- `TestCreateRelease_SelfHosted_SetsGitlabHostEnv` / `TestUploadAssets_SelfHosted_SetsGitlabHostEnv`:
  `mr.Calls[0].Env` is `nil` (uses `Run`, not `RunEnv`).
- `TestCheck_SelfHosted_SkipsCIAutologin`: with `GITLAB_CI=true`, `checkAPIAuth` takes the
  CI branch (`glab api projects/42/releases...`), so `mr.Calls[1].Args` is
  `["api", "projects/42/releases?per_page=1"]`, not `["api", "user"]`, and the test
  panics on the unqueued 3rd call or fails the assertion.

- [ ] **Step 3: Add `net/url` import, `selfHosted()` and `hostEnv()` helpers**

In `internal/platforms/gitlab/platform.go`, update the import block (lines 3-12):

```go
import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)
```

Add two new methods after `tokenEnvSlice` (after line 123, before `// CreateRelease runs
\`glab release create\`.` at line 125):

```go

// selfHosted reports whether this platform targets a non-default GitLab host.
func (p *Platform) selfHosted() bool {
	return p.cfg.BaseURL != "" && p.cfg.BaseURL != gitlabBaseURL
}

// hostEnv returns the env vars needed to point glab at a self-hosted instance.
// Returns nil for the default host (gitlab.com), so RunEnv(p.hostEnv(), ...) is a
// no-op for the common case.
func (p *Platform) hostEnv() []string {
	if !p.selfHosted() {
		return nil
	}
	u, err := url.Parse(p.cfg.BaseURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{"GITLAB_HOST=" + u.Host}
}
```

- [ ] **Step 4: `Name()` returns the configured name**

Replace line 33:

```go
func (p *Platform) Name() string { return "gitlab" }
```

with:

```go
func (p *Platform) Name() string { return p.cfg.Name }
```

- [ ] **Step 5: `ReleaseURL()` honors `cfg.BaseURL`**

Replace lines 35-37:

```go
func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("%s/%s/-/releases/%s", gitlabBaseURL, p.project(), tag)
}
```

with:

```go
func (p *Platform) ReleaseURL(tag string) string {
	baseURL := p.cfg.BaseURL
	if baseURL == "" {
		baseURL = gitlabBaseURL
	}
	return fmt.Sprintf("%s/%s/-/releases/%s", baseURL, p.project(), tag)
}
```

- [ ] **Step 6: `CreateRelease` and `UploadAssets` use `RunEnv(p.hostEnv(), ...)`**

In `CreateRelease`, replace line 148:

```go
	if _, _, err := p.runner.Run("glab", args...); err != nil {
```

with:

```go
	if _, _, err := p.runner.RunEnv(p.hostEnv(), "glab", args...); err != nil {
```

In `UploadAssets`, replace line 179:

```go
	if _, _, err := p.runner.Run("glab", args...); err != nil {
```

with:

```go
	if _, _, err := p.runner.RunEnv(p.hostEnv(), "glab", args...); err != nil {
```

- [ ] **Step 7: `checkAPIAuth` skips CI autologin when self-hosted, merges `hostEnv()`**

Replace `checkAPIAuth` (lines 94-115):

```go
func (p *Platform) checkAPIAuth(tokenEnv string, tokenMissing bool) error {
	if os.Getenv("GITLAB_CI") == "true" {
		projectID := os.Getenv("CI_PROJECT_ID")
		if projectID == "" {
			return nil
		}
		endpoint := "projects/" + projectID + "/releases?per_page=1"
		_, stderr, err := p.runner.Run("glab", "api", endpoint)
		if err != nil {
			return fmt.Errorf("gitlab: API call failed (glab api %s): %s\n  hint: ensure CI_JOB_TOKEN has read access (Settings > CI/CD > Job token permissions)", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	_, stderr, err := p.runner.RunEnv(p.tokenEnvSlice(tokenEnv), "glab", "api", "user")
	if err != nil {
		return fmt.Errorf("gitlab: API call failed (glab api user): %s\n  hint: verify %s is valid and has the api scope", strings.TrimSpace(stderr), tokenEnv)
	}
	return nil
}
```

with:

```go
func (p *Platform) checkAPIAuth(tokenEnv string, tokenMissing bool) error {
	if !p.selfHosted() && os.Getenv("GITLAB_CI") == "true" {
		projectID := os.Getenv("CI_PROJECT_ID")
		if projectID == "" {
			return nil
		}
		endpoint := "projects/" + projectID + "/releases?per_page=1"
		_, stderr, err := p.runner.Run("glab", "api", endpoint)
		if err != nil {
			return fmt.Errorf("gitlab: API call failed (glab api %s): %s\n  hint: ensure CI_JOB_TOKEN has read access (Settings > CI/CD > Job token permissions)", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	env := append(p.tokenEnvSlice(tokenEnv), p.hostEnv()...)
	_, stderr, err := p.runner.RunEnv(env, "glab", "api", "user")
	if err != nil {
		return fmt.Errorf("gitlab: API call failed (glab api user): %s\n  hint: verify %s is valid and has the api scope", strings.TrimSpace(stderr), tokenEnv)
	}
	return nil
}
```

Also update the doc comment directly above `checkAPIAuth` (lines 90-93):

```go
// checkAPIAuth verifies API access. For the default GitLab host in GitLab CI, glab is
// already authenticated via CI autologin so no token injection is used; the project
// releases endpoint is probed via CI_PROJECT_ID. For self-hosted instances, and outside
// CI, the configured token (plus GITLAB_HOST when self-hosted) is validated via /user.
```

- [ ] **Step 8: Run the gitlab platform tests**

Run: `go test ./internal/platforms/gitlab/... -v 2>&1 | rtk proxy grep -E 'FAIL|ok'`

Expected: `ok`, no `FAIL`. All pre-existing tests (`TestCreateRelease_BasicArgs`,
`TestCheck_OK_WithAuth`, `TestCheck_Auth_InCI_OK`, etc.) continue to pass because for the
default `gitlab.com` host (`cfg.BaseURL == "" || cfg.BaseURL == gitlabBaseURL`),
`hostEnv()` returns `nil`, and `RunEnv(nil, "glab", ...)` records the same `Call{Env:
nil}` as `Run("glab", ...)`.

- [ ] **Step 9: Run lint**

Run: `mise run lint:check`

Expected: clean.

- [ ] **Step 10: Commit**

```bash
git add internal/platforms/gitlab/platform.go internal/platforms/gitlab/platform_test.go
git commit -m "$(cat <<'EOF'
feat(platforms/gitlab): support self-hosted instances via GITLAB_HOST

Name() now returns the configured platform name, ReleaseURL() honors
base_url, and CreateRelease/UploadAssets/Check inject GITLAB_HOST when
base_url targets a non-default host. CI autologin is skipped for
self-hosted instances in favor of the configured token.

Roadmap: docs/tasks/roadmap.md → T84
EOF
)"
```

---

## Task 3 (T85): GitHub platform — `hostEnv()`, `Name()`, `ReleaseURL()` honor config

**Files:**
- Modify: `internal/platforms/github/platform.go` (full file, 209 lines)
- Modify: `internal/platforms/github/platform_test.go`

**Dependencies:** T83 (`config.Platform.Name` and unrestricted `base_url` must exist).

---

- [ ] **Step 1: Write failing tests**

In `internal/platforms/github/platform_test.go`:

Replace `TestName` (lines 17-20) — `Name()` must now return the configured name, not a
hardcoded constant:

```go
func TestName(t *testing.T) {
	p := github.New(exectest.NewMockRunner(), &config.Platform{Name: "github-internal"})
	assert.Equal(t, "github-internal", p.Name())
}
```

Add the following new test after `TestReleaseURL_FromEnv` (after line 32):

```go
func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Repository: "org/repo", BaseURL: "https://github.example.com"}
	p := github.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://github.example.com/org/repo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}
```

Add the following new tests at the end of the file (after `TestUploadAssets_GlobSkipsDirectories`,
which ends at line 447):

```go

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

func TestCreateRelease_SelfHosted_SetsGhHostEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "ent-token")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		BaseURL:    "https://github.example.com",
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GH_TOKEN=ent-token", "GH_HOST=github.example.com", "GH_ENTERPRISE_TOKEN=ent-token"}, mr.Calls[0].Env)
}

func TestUploadAssets_SelfHosted_SetsGhHostEnv(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	t.Setenv("GH_TOKEN", "ent-token")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{assetPath},
		BaseURL:    "https://github.example.com",
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GH_TOKEN=ent-token", "GH_HOST=github.example.com", "GH_ENTERPRISE_TOKEN=ent-token"}, mr.Calls[0].Env)
}

func TestCheck_SelfHosted_SkipsActionsAutologin(t *testing.T) {
	// Even when GITHUB_ACTIONS=true, a self-hosted GHES instance must not rely on the
	// GITHUB_TOKEN-based autologin (which targets api.github.com) — it always
	// authenticates via the configured token, with GH_HOST/GH_ENTERPRISE_TOKEN pointing
	// gh at the right host.
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "actions-token")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil)

	t.Setenv("GH_TOKEN", "ent-token")
	p := github.New(mr, &config.Platform{
		TokenEnv:   "GH_TOKEN",
		Repository: "org/repo",
		BaseURL:    "https://github.example.com",
	})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "repos/{owner}/{repo}/releases?per_page=1"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GH_TOKEN=ent-token", "GH_HOST=github.example.com", "GH_ENTERPRISE_TOKEN=ent-token"}, mr.Calls[1].Env)
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/platforms/github/... -run 'TestName|TestReleaseURL_SelfHosted|TestCreateRelease_SelfHosted|TestUploadAssets_SelfHosted|TestCheck_SelfHosted' -v`

Expected: all `FAIL` —
- `TestName`: `Name()` returns `"github"` (hardcoded), not `"github-internal"`.
- `TestReleaseURL_SelfHosted`: `ReleaseURL` uses the hardcoded `"https://github.com"` literal.
- `TestCreateRelease_SelfHosted_SetsGhHostEnv` / `TestUploadAssets_SelfHosted_SetsGhHostEnv`:
  `mr.Calls[0].Env` is `["GH_TOKEN=ent-token"]` only (no `GH_HOST`/`GH_ENTERPRISE_TOKEN`).
- `TestCheck_SelfHosted_SkipsActionsAutologin`: with `GITHUB_ACTIONS=true`, `checkAPIAuth`
  takes the Actions branch and calls `gh api repos/org/repo/releases?per_page=1` with
  `GH_TOKEN=actions-token` (from `GITHUB_TOKEN`), so `mr.Calls[1].Args` is
  `["api", "repos/org/repo/releases?per_page=1"]` and `mr.Calls[1].Env` is
  `["GH_TOKEN=actions-token"]`, not the expected self-hosted values.

- [ ] **Step 3: Add `net/url` import, `selfHosted()` and `hostEnv()` helpers**

In `internal/platforms/github/platform.go`, update the import block (lines 3-12):

```go
import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
)
```

Add two new methods after `tokenEnvSlice` (after line 208, end of file):

```go

// selfHosted reports whether this platform targets a non-default GitHub host
// (i.e. a GitHub Enterprise Server instance).
func (p *Platform) selfHosted() bool {
	return p.cfg.BaseURL != "" && p.cfg.BaseURL != githubBaseURL
}

// hostEnv returns the env vars needed to point gh at a self-hosted GHES instance:
// GH_HOST selects the host, and GH_ENTERPRISE_TOKEN (gh's expected var for non-github.com
// hosts) carries the configured token. Returns nil for the default host (github.com), so
// RunEnv(p.hostEnv(), ...) is a no-op for the common case.
func (p *Platform) hostEnv() []string {
	if !p.selfHosted() {
		return nil
	}
	u, err := url.Parse(p.cfg.BaseURL)
	if err != nil || u.Host == "" {
		return nil
	}
	env := []string{"GH_HOST=" + u.Host}
	if token := os.Getenv(p.tokenEnv()); token != "" {
		env = append(env, "GH_ENTERPRISE_TOKEN="+token)
	}
	return env
}
```

- [ ] **Step 4: `Name()` returns the configured name**

Replace line 33:

```go
func (p *Platform) Name() string { return "github" }
```

with:

```go
func (p *Platform) Name() string { return p.cfg.Name }
```

- [ ] **Step 5: `ReleaseURL()` honors `cfg.BaseURL`**

Replace lines 35-37:

```go
func (p *Platform) ReleaseURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/releases/tag/%s", p.repository(), tag)
}
```

with:

```go
func (p *Platform) ReleaseURL(tag string) string {
	baseURL := p.cfg.BaseURL
	if baseURL == "" {
		baseURL = githubBaseURL
	}
	return fmt.Sprintf("%s/%s/releases/tag/%s", baseURL, p.repository(), tag)
}
```

- [ ] **Step 6: `CreateRelease` and `UploadAssets` merge `hostEnv()` into the token env**

In `CreateRelease`, replace line 141:

```go
	if _, _, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", args...); err != nil {
```

with:

```go
	env := append(p.tokenEnvSlice(), p.hostEnv()...)
	if _, _, err := p.runner.RunEnv(env, "gh", args...); err != nil {
```

In `UploadAssets`, replace line 171:

```go
		if _, _, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", "release", "upload", tag, f, "--repo", repo); err != nil {
```

with:

```go
		env := append(p.tokenEnvSlice(), p.hostEnv()...)
		if _, _, err := p.runner.RunEnv(env, "gh", "release", "upload", tag, f, "--repo", repo); err != nil {
```

- [ ] **Step 7: `checkAPIAuth` skips Actions autologin when self-hosted, merges `hostEnv()`**

Replace `checkAPIAuth` (lines 88-110):

```go
func (p *Platform) checkAPIAuth(tokenMissing bool) error {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		githubToken := os.Getenv("GITHUB_TOKEN")
		repo := os.Getenv("GITHUB_REPOSITORY")
		if githubToken == "" || repo == "" {
			return nil
		}
		endpoint := "repos/" + repo + "/releases?per_page=1"
		_, stderr, err := p.runner.RunEnv([]string{"GH_TOKEN=" + githubToken}, "gh", "api", endpoint)
		if err != nil {
			return fmt.Errorf("github: API call failed (gh api %s): %s\n  hint: verify GITHUB_TOKEN has read access to the repository", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	_, stderr, err := p.runner.RunEnv(p.tokenEnvSlice(), "gh", "api", "repos/{owner}/{repo}/releases?per_page=1")
	if err != nil {
		return fmt.Errorf("github: API call failed (gh api repos/{owner}/{repo}/releases): %s\n  hint: verify %s is valid and has the necessary scopes", strings.TrimSpace(stderr), p.tokenEnv())
	}
	return nil
}
```

with:

```go
func (p *Platform) checkAPIAuth(tokenMissing bool) error {
	if !p.selfHosted() && os.Getenv("GITHUB_ACTIONS") == "true" {
		githubToken := os.Getenv("GITHUB_TOKEN")
		repo := os.Getenv("GITHUB_REPOSITORY")
		if githubToken == "" || repo == "" {
			return nil
		}
		endpoint := "repos/" + repo + "/releases?per_page=1"
		_, stderr, err := p.runner.RunEnv([]string{"GH_TOKEN=" + githubToken}, "gh", "api", endpoint)
		if err != nil {
			return fmt.Errorf("github: API call failed (gh api %s): %s\n  hint: verify GITHUB_TOKEN has read access to the repository", endpoint, strings.TrimSpace(stderr))
		}
		return nil
	}
	if tokenMissing {
		return nil
	}
	env := append(p.tokenEnvSlice(), p.hostEnv()...)
	_, stderr, err := p.runner.RunEnv(env, "gh", "api", "repos/{owner}/{repo}/releases?per_page=1")
	if err != nil {
		return fmt.Errorf("github: API call failed (gh api repos/{owner}/{repo}/releases): %s\n  hint: verify %s is valid and has the necessary scopes", strings.TrimSpace(stderr), p.tokenEnv())
	}
	return nil
}
```

Also update the doc comment directly above `checkAPIAuth` (lines 85-87):

```go
// checkAPIAuth verifies API access. For the default GitHub host in GitHub Actions,
// GITHUB_TOKEN is used directly because gh auth status reads config files and won't see
// the injected env var. For self-hosted instances (GHES), and outside Actions, the
// configured token (plus GH_HOST/GH_ENTERPRISE_TOKEN when self-hosted) is validated via a
// repo-scoped API call.
```

- [ ] **Step 8: Run the github and gitlab platform tests**

Run: `go test ./internal/platforms/... -v 2>&1 | rtk proxy grep -E 'FAIL|ok'`

Expected: `ok`, no `FAIL`. All pre-existing tests (`TestCreateRelease_TokenForwarded`,
`TestUploadAssets_TokenForwarded`, `TestCheck_GitHubActions_OK`, etc.) continue to pass
because for the default `github.com` host (`cfg.BaseURL == "" || cfg.BaseURL ==
githubBaseURL`), `hostEnv()` returns `nil`, and `append(p.tokenEnvSlice(), nil...)` is
unchanged from `p.tokenEnvSlice()`.

- [ ] **Step 9: Run lint**

Run: `mise run lint:check`

Expected: clean.

- [ ] **Step 10: Commit**

```bash
git add internal/platforms/github/platform.go internal/platforms/github/platform_test.go
git commit -m "$(cat <<'EOF'
feat(platforms/github): support self-hosted instances via GH_HOST

Name() now returns the configured platform name, ReleaseURL() honors
base_url, and CreateRelease/UploadAssets/Check inject GH_HOST and
GH_ENTERPRISE_TOKEN when base_url targets a non-default (GHES) host.
Actions-token autologin is skipped for self-hosted instances in favor
of the configured token.

Roadmap: docs/tasks/roadmap.md → T85
EOF
)"
```

---

## Task 4 (T86): `internal/app/check.go` — one Platforms row per `release.platforms` entry

**Files:**
- Modify: `internal/app/check.go` (full file, 230 lines)
- Modify: `internal/app/check_test.go`

**Scope note (flagged per `.claude/rules/claude.md` "Architectural decisions"):** the design
doc's §4 also describes checking each CLI binary (`glab`/`gh`) only **once per type** and
reusing the result across multiple same-type entries (e.g. two `gitlab` entries would
share one `glab --version` probe). This task does **not** implement that de-duplication —
each `release.platforms` entry's `p.Check()` runs its own binary probe. De-duplication
adds non-trivial bookkeeping (tracking which types were already probed across dispatch
calls, deciding which entry's row reports a shared binary failure) for a corner case
(multiple entries of the *same* platform type) that is the entire point of this feature
but is not exercised by existing tests. Flagged here so the user can decide whether to
accept the extra `--version` calls (functionally harmless — `p.Check()` already runs them
per platform — just an extra subprocess spawn per duplicate-type entry) or request the
de-duplication as a follow-up task.

**Dependencies:** T83 (`config.Platform.Name`), T84 (gitlab `Name()`/self-hosted), T85
(github `Name()`/self-hosted).

---

- [ ] **Step 1: Write failing tests**

In `internal/app/check_test.go`:

Replace `TestRuntimeCheck_WithGitHubPlatform` (lines 267-297):

```go
func TestRuntimeCheck_WithGitHubPlatform(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTIONS", "") // non-CI path

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("gh 2.67.0", "", nil)          // gh binary — inside p.Check()
	mr.QueueResponse(`[]`, "", nil)                 // gh api auth — inside p.Check().checkAPIAuth()
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github", Name: "gh", Repository: "org/repo"}},
	}
	items := collectItems(mr, cfg)

	found := false
	for _, it := range items {
		if it.Name == "gh" {
			found = true
			assert.NoError(t, it.Err)
		}
	}
	assert.True(t, found, "expected gh item")
}
```

Replace `TestRuntimeCheck_WithGitHubPlatform_MissingToken` (lines 299-329):

```go
func TestRuntimeCheck_WithGitHubPlatform_MissingToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_ACTIONS", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("gh 2.67.0", "", nil)          // gh binary — p.Check() runs binary before token
	// token missing → checkAPIAuth skipped → no API runner call
	mr.QueueResponse("git-cliff 2.0", "", nil)  // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)        // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil) // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "github", Name: "gh", Repository: "org/repo"}},
	}
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "gh" {
			assert.Error(t, it.Err)
			assert.Contains(t, it.Err.Error(), "GH_TOKEN")
			return
		}
	}
	t.Fatal("expected gh item")
}
```

Replace `TestRuntimeCheck_UnknownPlatform` (lines 355-377):

```go
func TestRuntimeCheck_UnknownPlatform(t *testing.T) {
	// An unrecognized platform type is normally caught by config validation before
	// RuntimeCheck runs, but RuntimeCheck still reports a hard error for the entry
	// (labeled by its configured name) rather than silently skipping it.
	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("git-cliff 2.0", "", nil)      // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)            // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil)     // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{{Type: "unknown-plat", Name: "unknown-plat"}},
	}
	items := collectItems(mr, cfg)

	for _, it := range items {
		if it.Name == "unknown-plat" {
			assert.Error(t, it.Err)
			assert.Contains(t, it.Err.Error(), "unsupported platform")
			return
		}
	}
	t.Fatal("expected unknown-plat item")
}
```

Add a new test after `TestRuntimeCheck_UnknownPlatform`:

```go

func TestRuntimeCheck_MultipleSameTypePlatforms(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_CI", "")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("git version 2.40.0", "", nil) // git --version
	mr.QueueResponse("Alice", "", nil)              // user.name
	mr.QueueResponse("a@b.com", "", nil)            // user.email
	mr.QueueResponse("", "", nil)                   // git status
	mr.QueueResponse("glab 1.0", "", nil)           // gitlab-com p.Check() binary
	// token missing → checkAPIAuth skipped for gitlab-com
	mr.QueueResponse("glab 1.0", "", nil) // gitlab-internal p.Check() binary
	// token missing → checkAPIAuth skipped for gitlab-internal
	mr.QueueResponse("git-cliff 2.0", "", nil)  // git-cliff (optional)
	mr.QueueResponse("cog 7.0", "", nil)        // cog (optional)
	mr.QueueResponse("communique 1.0", "", nil) // communique (optional)

	cfg := semverCfg()
	cfg.Release = &config.Release{
		Platforms: []config.Platform{
			{Type: "gitlab", Name: "gitlab-com", Project: "acme/widget"},
			{Type: "gitlab", Name: "gitlab-internal", Project: "acme/widget", BaseURL: "https://gitlab.example.com"},
		},
	}
	items := collectItems(mr, cfg)

	var names []string
	for _, it := range items {
		if it.Name == "gitlab-com" || it.Name == "gitlab-internal" {
			names = append(names, it.Name)
			assert.Error(t, it.Err, "%s: missing GITLAB_TOKEN should be a hard error", it.Name)
			assert.Contains(t, it.Err.Error(), "GITLAB_TOKEN")
		}
	}
	assert.Equal(t, []string{"gitlab-com", "gitlab-internal"}, names)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/... -run 'TestRuntimeCheck' -v 2>&1 | rtk proxy grep -E 'FAIL|PASS|---'`

Expected: `TestRuntimeCheck_WithGitHubPlatform`, `_MissingToken`, `_UnknownPlatform`, and
`_MultipleSameTypePlatforms` all `FAIL` — the current implementation still dispatches
exactly two rows ("glab", "gh") regardless of `cfg.Release.Platforms`, so:
- `TestRuntimeCheck_WithGitHubPlatform`/`_MissingToken` look for an item named `"gh"` but
  the queued responses are now misaligned (no "glab 1.0" response queued at position 5,
  where the current code expects one), causing a `MockRunner: no response queued` panic
  or a mismatched `Args`/`Err`.
- `TestRuntimeCheck_UnknownPlatform` looks for an item named `"unknown-plat"`, but the
  current code only ever dispatches `"glab"`/`"gh"`.
- `TestRuntimeCheck_MultipleSameTypePlatforms` looks for items named `"gitlab-com"` and
  `"gitlab-internal"`, neither of which the current code ever dispatches.

- [ ] **Step 3: Remove `configuredPlatforms` and `findPlatformCfg`**

In `internal/app/check.go`, delete the `configuredPlatforms` function (lines 184-197) and
the `findPlatformCfg` function (lines 199-210):

```go
// configuredPlatforms returns the set of platform types active in cfg.
// When cfg is nil (no config file found) all supported platforms are required.
func configuredPlatforms(cfg *config.Config) map[string]bool {
	if cfg == nil {
		return map[string]bool{"github": true, "gitlab": true}
	}
	m := make(map[string]bool)
	if cfg.Release != nil {
		for _, p := range cfg.Release.Platforms {
			m[p.Type] = true
		}
	}
	return m
}

// findPlatformCfg returns the config for the platform of the given type, or nil.
func findPlatformCfg(cfg *config.Config, typ string) *config.Platform {
	if cfg.Release == nil {
		return nil
	}
	for i := range cfg.Release.Platforms {
		if cfg.Release.Platforms[i].Type == typ {
			return &cfg.Release.Platforms[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Restructure the Platforms section of `RuntimeCheck`**

Replace the Platforms section (lines 105-141):

```go
	// ── Platforms ─────────────────────────────────────────────────────────────
	header("Platforms")

	usedPlats := configuredPlatforms(cfg)
	for _, op := range []struct{ typ, binary, display string }{
		{"gitlab", "glab", "glab"},
		{"github", "gh", "gh"},
	} {
		op := op
		required := usedPlats[op.typ]
		dispatch(op.display, func() RuntimeCheckItem {
			if required && cfg != nil {
				// Full check: binary + token + project + API auth.
				platCfg := findPlatformCfg(cfg, op.typ)
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: op.display, Err: buildErr}
				}
				if err := p.Check(); err != nil {
					return RuntimeCheckItem{Name: op.display, Err: err}
				}
				return RuntimeCheckItem{Name: op.display}
			}
			// Binary-only check (no config available for token/project resolution).
			// Missing binary is a hard error when required, advisory otherwise.
			out, _, err := runner.Run(op.binary, "--version")
			if err != nil {
				if required {
					return RuntimeCheckItem{Name: op.display,
						Err: fmt.Errorf("%s: not found on PATH", op.binary)}
				}
				return RuntimeCheckItem{Name: op.display, IsWarn: true,
					Err: fmt.Errorf("not found (not required by this config)")}
			}
			return RuntimeCheckItem{Name: op.display, Value: strings.TrimSpace(out)}
		})
	}
```

with:

```go
	// ── Platforms ─────────────────────────────────────────────────────────────
	header("Platforms")

	if cfg != nil && cfg.Release != nil && len(cfg.Release.Platforms) > 0 {
		// One row per configured platform entry: full check (binary + token +
		// project/repository + API auth), labeled by the entry's configured name.
		for i := range cfg.Release.Platforms {
			platCfg := &cfg.Release.Platforms[i]
			name := platCfg.Name
			dispatch(name, func() RuntimeCheckItem {
				p, buildErr := buildPlatform(runner, platCfg)
				if buildErr != nil {
					return RuntimeCheckItem{Name: name, Err: buildErr}
				}
				if err := p.Check(); err != nil {
					return RuntimeCheckItem{Name: name, Err: err}
				}
				return RuntimeCheckItem{Name: name}
			})
		}
	} else {
		// No platforms configured: fall back to a binary-only probe of both
		// supported CLIs. Required (hard error) when cfg is nil (no config file
		// found, so both could plausibly be needed); advisory otherwise.
		required := cfg == nil
		for _, bin := range []string{"glab", "gh"} {
			bin := bin
			dispatch(bin, func() RuntimeCheckItem {
				out, _, err := runner.Run(bin, "--version")
				if err != nil {
					if required {
						return RuntimeCheckItem{Name: bin,
							Err: fmt.Errorf("%s: not found on PATH", bin)}
					}
					return RuntimeCheckItem{Name: bin, IsWarn: true,
						Err: fmt.Errorf("not found (not required by this config)")}
				}
				return RuntimeCheckItem{Name: bin, Value: strings.TrimSpace(out)}
			})
		}
	}
```

- [ ] **Step 5: Run the app package tests**

Run: `go test ./internal/app/... -v 2>&1 | rtk proxy grep -E 'FAIL|ok'`

Expected: `ok`, no `FAIL`.

- [ ] **Step 6: Run the full test suite and lint**

Run: `mise run test && mise run lint:check`

Expected: both clean. `mise run test` confirms no other package (e.g.
`internal/cmd/check_test.go`) depends on the removed `configuredPlatforms`/
`findPlatformCfg` or on the old two-row "glab"/"gh" Platforms output when
`release.platforms` is configured.

- [ ] **Step 7: Commit**

```bash
git add internal/app/check.go internal/app/check_test.go
git commit -m "$(cat <<'EOF'
feat(app): one heraut check runtime Platforms row per configured entry

RuntimeCheck now dispatches one Platforms row per release.platforms
entry, labeled by the entry's configured name, running each entry's
full Check() (binary + token + project/repository + API auth). With
no platforms configured, falls back to a binary-only probe of glab
and gh. Removes configuredPlatforms/findPlatformCfg, which assumed at
most one entry per platform type.

Roadmap: docs/tasks/roadmap.md → T86
EOF
)"
```

---

## Task 5 (T87): Docs — ADR-0025, supersede ADR-0020, update spec 05

**Files:**
- Create: `docs/adr/0025-multi-instance-platforms.md`
- Modify: `docs/adr/0020-platform-base-url.md`
- Modify: `docs/adr/README.md`
- Modify: `docs/specs/05-generators-and-platforms.md`

**Dependencies:** T83-T86 (all implementation complete; this task documents the shipped
behavior).

---

- [ ] **Step 1: Create `docs/adr/0025-multi-instance-platforms.md`**

```markdown
# ADR-0025: Multi-Instance Same-Platform Releases

- **Status**: Accepted
- **Date**: 2026-06-12
- **Deciders**: bchatard

---

## Context

[ADR-0020](0020-platform-base-url.md) introduced `base_url` as the per-platform source of
truth for a platform's web host, but deferred two of its three consumers (`ReleaseURL`/
`LinkContext` and CLI host targeting via `GH_HOST`/`GITLAB_HOST`) to a "multi-instance
thread", and — until that thread landed — the validator rejected any `base_url` that
differed from the platform-type default:

```
release.platforms[1].base_url: self-hosted hosts are not yet supported
```

This ADR is that thread. The motivating scenario: a project publishes releases to both a
public GitLab instance (`gitlab.com`, e.g. for a CI/CD Catalog component) and a private
self-hosted GitLab instance (`gitlab.example.com`, the project's primary remote), in one
`heraut release` run. `release.platforms` already supports a list, so two entries of type
`gitlab` are structurally possible — but three things break with two entries of the same
type:

1. **`base_url` is gated** to the type default (ADR-0020) — a self-hosted second instance
   cannot be configured at all.
2. **`gh`/`glab` are not told which host to talk to.** Both CLIs default to
   `github.com`/`gitlab.com`; without `GH_HOST`/`GITLAB_HOST` (and, for GitHub Enterprise,
   `GH_ENTERPRISE_TOKEN`), a release "published" to the self-hosted entry would actually
   hit the public host.
3. **Two entries of the same type are indistinguishable.** `internal/app/check.go`'s
   `findPlatformCfg` returns the *first* match by type, so `heraut check runtime` would
   report on only one of the two GitLab entries — silently dropping the other — and
   `Platform.Name()` returned a bare `"gitlab"`/`"github"` constant, so even error
   messages and the `heraut check runtime` table couldn't tell the two apart.

## Decision

### 1. `config.Platform` gains a required, unique `name` field

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab-com
      project: acme/widget-catalog
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
```

`name` is a free-form label (not a platform type) that **must be unique** within each
`release.platforms` list — checked independently for the top-level list and for each
environment override's list (per [ADR-0019](0019-perenv-content-driver-merge.md), env
overrides replace the list wholesale, so uniqueness is scoped to whichever list is
actually in effect). It is the label used in `heraut check runtime`'s Platforms section
and in any per-entry error message. There is no default — a missing `name` is a config
error, because a silently-generated name (e.g. `"gitlab-2"`) would be useless in error
output and would change if entries are reordered.

### 2. The `base_url`-equals-default gate is lifted

`validatePlatformBaseURL`'s check that `base_url` must equal the platform-type default is
removed entirely. The remaining shape check (`isValidBaseURL`: must be an absolute
`http(s)://` URL) is kept — heraut still rejects garbage values, just not non-default
*hosts*.

### 3. `hostEnv()` — per-platform CLI host targeting

Both `internal/platforms/{github,gitlab}` gain:

```go
func (p *Platform) selfHosted() bool {
    return p.cfg.BaseURL != "" && p.cfg.BaseURL != <type default>
}

func (p *Platform) hostEnv() []string // nil for the default host
```

- **GitLab self-hosted**: `hostEnv()` returns `["GITLAB_HOST=<host>"]`.
- **GitHub self-hosted (GHES)**: `hostEnv()` returns `["GH_HOST=<host>",
  "GH_ENTERPRISE_TOKEN=<token>"]`, where `<token>` is the value of the platform's
  configured `token_env` — GitHub Enterprise Server requires `GH_ENTERPRISE_TOKEN` rather
  than `GH_TOKEN` for non-`github.com` hosts.

`hostEnv()` is merged into every `RunEnv` call the platform makes (`CreateRelease`,
`UploadAssets`, and the `Check()` API-auth probe). For the default host, `hostEnv()`
returns `nil`, so `RunEnv(append(tokenEnv, nil...), ...)` is byte-identical to the
pre-T84/T85 `RunEnv(tokenEnv, ...)` call — existing single-instance configs are
unaffected.

### 4. `Check()` skips CI autologin for self-hosted instances

Both `gh` and `glab` support ambient CI autologin (`GITHUB_ACTIONS`/`GITHUB_TOKEN` and
`GITLAB_CI`/`CI_JOB_TOKEN` respectively) — but that autologin always targets the CI
runner's *own* host (`github.com` / `gitlab.com` for hosted runners), never a self-hosted
target configured separately. So when `selfHosted()` is true, `Check()` skips the
CI-autologin branch entirely and always validates the configured `token_env` via
`RunEnv(tokenEnvSlice + hostEnv, "gh"/"glab", "api", ...)`. This is a hard requirement: a
self-hosted entry with no token configured is a `Check()` failure, even in CI, because
there is no autologin path that could possibly reach it.

### 5. `Name()`, `ReleaseURL()`, `LinkContext()` honor configured values

- `Name()` returns `p.cfg.Name` (was a hardcoded `"github"`/`"gitlab"`).
- `ReleaseURL()` and `LinkContext()` use `p.cfg.BaseURL`, falling back to the type default
  (`https://github.com` / `https://gitlab.com`) when unset — this was ADR-0020's deferred
  "consumer 2", now wired alongside consumer 3 since both touch the same files.

### 6. `heraut check runtime` — one Platforms row per configured entry

`internal/app/check.go`'s Platforms section previously dispatched exactly two fixed rows
("glab", "gh") via `configuredPlatforms`/`findPlatformCfg` (first-match-by-type). It now
dispatches **one row per `release.platforms` entry**, labeled by that entry's `name`,
running that entry's full `Check()`. `configuredPlatforms` and `findPlatformCfg` are
removed — they assumed at most one entry per platform type. When no platforms are
configured (or `cfg` is `nil`), the section falls back to a binary-only probe of `glab`
and `gh`, matching the pre-existing nil-config / no-`release`-block behavior.

**Deferred**: checking each CLI binary (`glab`/`gh`) only once per type and reusing the
result across same-type entries (e.g. two `gitlab` entries sharing one `glab --version`
probe) is *not* implemented — each entry's `Check()` runs its own binary probe. This is
functionally harmless (an extra subprocess spawn per duplicate-type entry) and can be
revisited if it becomes a real cost.

## Consequences

**Positive**

- Two instances of the same platform type — the entire point of this ADR — now work:
  configurable, individually named, individually checked, and individually targeted by
  `gh`/`glab`.
- `ReleaseURL`/`LinkContext` and `heraut check runtime` no longer silently collapse
  multiple entries of the same type into one.
- Single-instance configs are byte-for-byte unaffected: default `base_url` → `hostEnv()`
  is `nil` → identical `RunEnv` calls; `name` becomes required but is a one-line addition
  to existing configs (and `heraut init` defaults/dedupes it — see T83).

**Negative / trade-offs**

- `name` is a new required field — a wire-compatibility break for existing `.heraut.yml`
  files (mitigated: `heraut init`-generated configs are updated, and the migration is a
  single added line per platform entry; `heraut check config` reports the missing field
  with a clear hint).
- The binary-presence-dedup optimization described in the original design note is
  deferred (see "Deferred" above) — `heraut check runtime` makes one extra `--version`
  call per additional same-type entry. Negligible in practice.
- `GH_ENTERPRISE_TOKEN` vs `GH_TOKEN` is a real `gh` CLI distinction that heraut now has to
  track per platform entry — a small but permanent piece of GitHub-specific knowledge in
  `internal/platforms/github`.

## Alternatives considered

- **Auto-generate names from type + index (e.g. `gitlab-1`, `gitlab-2`).** Rejected: names
  appear in error messages and the `heraut check runtime` table; an index-derived name is
  meaningless to a user and changes if entries are reordered. An explicit, required `name`
  is one line and is permanently stable.
- **Keep `findPlatformCfg`'s first-match semantics and add a second type-keyed map for
  "extra" instances.** Rejected: every consumer (`heraut check runtime`, error messages,
  `ReleaseURL`) would need to handle "the first one" vs "the others" differently, doubling
  the surface area for a problem that a per-entry loop solves uniformly.
- **Deep-merge env-level platform overrides instead of replace.** Rejected: out of scope —
  [ADR-0019](0019-perenv-content-driver-merge.md) already settled list-replace semantics
  for `release.platforms`, and revisiting it here would couple two unrelated decisions.
- **Implement binary-presence dedup now.** Rejected for this ADR: see "Deferred" above —
  the bookkeeping cost is real and the benefit (avoiding one extra `--version` subprocess
  call) is marginal; flagged as a candidate follow-up rather than blocking this ADR.
```

- [ ] **Step 2: Mark ADR-0020 as superseded**

In `docs/adr/0020-platform-base-url.md`, update the status line (line 3):

```markdown
- **Status**: Accepted
```

to:

```markdown
- **Status**: Superseded by [ADR-0025](0025-multi-instance-platforms.md)
```

Add a note immediately after the `---` separator (after line 7), before `## Context`:

```markdown
> **Superseded**: the validator gate described below ("Gate non-default `base_url` until
> host targeting lands") and the deferred consumers 2/3 in the table below were resolved
> by [ADR-0025](0025-multi-instance-platforms.md), which lifts the gate and implements
> per-platform host targeting (`GH_HOST`/`GITLAB_HOST`/`GH_ENTERPRISE_TOKEN`),
> `Name()`/`ReleaseURL()`/`LinkContext()` honoring `base_url`, and a `name` field for
> disambiguating multiple entries of the same platform type. The rest of this ADR (the
> `base_url` field itself, its semantics, and consumer 1 / Phase 14) remains accurate.

```

- [ ] **Step 3: Update the ADR index**

In `docs/adr/README.md`, change the ADR-0020 row (line 27):

```markdown
| [0020](0020-platform-base-url.md) | Per-Platform `base_url` for Self-Hosted Instances | Accepted |
```

to:

```markdown
| [0020](0020-platform-base-url.md) | Per-Platform `base_url` for Self-Hosted Instances | Superseded by ADR-0025 |
```

Add a new row after the ADR-0024 row (after line 31):

```markdown
| [0025](0025-multi-instance-platforms.md) | Multi-Instance Same-Platform Releases | Accepted |
```

- [ ] **Step 4: Update `docs/specs/05-generators-and-platforms.md`**

Replace the GitHub config example (lines 202-212):

```yaml
release:
  platforms:
    - platform: github
      repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
      token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

with:

```yaml
release:
  platforms:
    - platform: github
      name: github               # required, must be unique within this platforms list
      repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
      token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
      base_url: https://github.com  # optional, defaults to https://github.com
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

Replace the GitLab config example (lines 232-240):

```yaml
release:
  platforms:
    - platform: gitlab
      project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
      token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
      catalog: false              # optional, set true for a CI/CD Catalog release
      assets:
        - dist/myapp_*
```

with:

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab               # required, must be unique within this platforms list
      project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
      token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
      base_url: https://gitlab.com  # optional, defaults to https://gitlab.com
      catalog: false              # optional, set true for a CI/CD Catalog release
      assets:
        - dist/myapp_*
```

Add a new subsection after the GitLab section (after line 254, before `### Platform
interface` at line 256):

```markdown
### Self-hosted instances and multiple entries of the same type (ADR-0025)

`base_url` may be set to any absolute `http(s)://` URL — including a host other than the
platform's default (`github.com` / `gitlab.com`). When `base_url` is self-hosted, heraut:

- Points `gh`/`glab` at that host: `GITLAB_HOST=<host>` for GitLab, `GH_HOST=<host>` +
  `GH_ENTERPRISE_TOKEN=<token>` for GitHub Enterprise Server.
- Skips CI autologin (`GITHUB_ACTIONS`/`GITLAB_CI`) for that entry — autologin always
  targets the CI runner's own (public) host, never a separately-configured self-hosted
  target — and instead always validates the configured `token_env`.
- Resolves `ReleaseURL`/`LinkContext` against `base_url` instead of the type default.

Because `release.platforms` is a list, multiple entries of the *same* platform type are
supported — e.g. publishing to both `gitlab.com` and a self-hosted
`gitlab.example.com`:

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab-com
      project: acme/widget-catalog
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
```

Each entry's `name` must be unique within its `release.platforms` list and is used to
label that entry's row in `heraut check runtime`'s Platforms section and in any
per-entry error message.
```

Update the `Platform interface` code block's `Name()` comment (line 262):

```go
    Name() string                                  // "github" or "gitlab"
```

to:

```go
    Name() string                                  // the configured platform entry's name
```

- [ ] **Step 5: Run lint**

Run: `mise run lint:check`

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add docs/adr/0025-multi-instance-platforms.md docs/adr/0020-platform-base-url.md \
        docs/adr/README.md docs/specs/05-generators-and-platforms.md
git commit -m "$(cat <<'EOF'
docs(adr): add ADR-0025, supersede ADR-0020 (multi-instance platforms)

ADR-0025 documents the lifted base_url gate, per-platform CLI host
targeting (GH_HOST/GITLAB_HOST/GH_ENTERPRISE_TOKEN), the required
unique name field, and the restructured heraut check runtime
Platforms section delivered by T83-T86. Spec 05 gains the self-hosted
/ multi-instance subsection and the name field in both platform
examples.

Roadmap: docs/tasks/roadmap.md → T87
EOF
)"
```

---
