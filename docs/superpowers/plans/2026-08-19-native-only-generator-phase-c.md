# Native-Only Generator, Phase C — Wizard Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop `internal/scaffold/wizard.go`'s decorative generator-choice step (git-cliff/communique/None — options with zero live effect since Phase A removed the `generator:` config key) and replace it with the forge/target/enrichment questions `docs/tasks/forge-abstraction-roadmap.md`'s T164 originally scoped, built against the final, generator-free config shape.

**Architecture:** All changes are confined to `internal/scaffold` (the `heraut init` wizard package) plus one small additive export from `internal/forge` that the wizard consumes for CI/git-origin platform-type detection — the one new cross-package dependency this phase introduces, requiring a `.claude/rules/coding.md` layer-table addition. No runtime config-consuming code changes; Phases A/B already built that.

**Tech Stack:** Go, `charm.land/huh/v2` (interactive TUI forms), `gopkg.in/yaml.v3`, `stretchr/testify`.

**Spec:** `docs/superpowers/specs/2026-08-17-native-only-generator-phase-c-wizard-design.md` (read this first — it has the full rationale for every decision below, including three corrections found during this plan's own research that are already folded into it). Original high-level scope: `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §4.

## Global Constraints

- TDD required: write the failing test before implementation code, for every task (`.claude/rules/testing.md`).
- `go test ./...` and `hk check` must be clean at the end of every task — no task lands with a broken build, even temporarily.
- Never delete a load-bearing test row to make a rename easier — rename/adapt it in place, keep the behavior it guards covered (`.claude/rules/testing.md`). Several tasks below rename existing tests; none delete test coverage without an equivalent replacement.
- Conventional commits, scope = affected package (e.g. `refactor(scaffold): ...`, `feat(forge): ...`).
- Each task ends by flipping its own task's `[ ]` → `[x]` in `docs/tasks/native-generator-roadmap.md`'s Phase C section and adding a one-paragraph completion note (actual decisions, deviations, deferred items) — commit the roadmap update alongside the implementation commit, per `.claude/rules/workflow.md`'s two-step roadmap flow.
- `internal/forge`'s existing `Resolve`, `ErrAmbiguousForge`, `Resolved` symbols and behavior are unaffected — this phase is purely additive to that package's public API.
- No new config schema fields — `api_mode`, `enrichment_forge`, `enrichment_policy` all already exist in `schema.json`/`internal/config`; this phase only adds wizard prompts that populate fields the config model already supports.
- This repo commits directly to `main` (pre-v1.0, no branches per `.claude/rules/workflow.md`).

---

## Task 195: Export a CI/git-origin platform-type detector from `internal/forge`

**Files:**
- Create: `internal/forge/detect_test.go`
- Modify: `internal/forge/detect.go` (append new exported function)

**Interfaces:**
- Consumes: existing unexported `detectCIForge` and `parseGitOrigin` (`internal/forge/detect.go`) — no changes to either.
- Produces: `forge.DetectForWizard(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string, ok bool)`, consumed by Task 196.

- [ ] **Step 1: Write the failing tests**

Create `internal/forge/detect_test.go`. Note `resolve_test.go` (same `forge_test` package) already defines an `env(map[string]string) func(string) string` helper at line 14-16 — reuse it, do not redefine.

```go
package forge_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/forge"
	"github.com/stretchr/testify/assert"
)

func TestDetectForWizard_GitLabCI(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"GITLAB_CI":       "true",
		"CI_PROJECT_PATH": "group/subgroup/project",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/subgroup/project", project)
}

func TestDetectForWizard_GitHubActions(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_REPOSITORY": "acme/widget",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "github", typ)
	assert.Equal(t, "acme/widget", project)
}

func TestDetectForWizard_AzureDevOpsUsesRepository(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "azure_devops", typ)
	assert.Equal(t, "myproject", project)
}

func TestDetectForWizard_GitOriginFallback(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(nil), "git@gitlab.com:group/project.git")
	assert.True(t, ok)
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/project", project)
}

// TestDetectForWizard_AmbientTokenAloneIsNotEnough pins that DetectForWizard, unlike
// forge.Resolve's zero-config path, never falls back to inspecting which ambient token env var
// happens to be set. The wizard always asks the user to pick (or confirm) a type explicitly when
// detection is inconclusive, rather than guessing from an ambient token.
func TestDetectForWizard_AmbientTokenAloneIsNotEnough(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{"GITHUB_TOKEN": "tok"}), "")
	assert.False(t, ok)
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}

func TestDetectForWizard_UnknownHost(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(nil), "https://git.company.com/team/service.git")
	assert.False(t, ok)
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/forge/... -run TestDetectForWizard -v`
Expected: FAIL with `undefined: forge.DetectForWizard`.

- [ ] **Step 3: Implement**

Append to `internal/forge/detect.go` (after `parseGitOrigin`, end of file):

```go

// DetectForWizard identifies a forge's platform type and project/repository path for wizard
// pre-fill: CI environment markers take priority, then a git origin URL matching one of the
// three known public hosts. ok is false when neither source identifies a recognized type —
// unlike Resolve's zero-config path, this never falls back to inspecting ambient token env vars,
// since the wizard always asks the user to pick (or confirm) a type explicitly when detection is
// inconclusive rather than guessing from which token happens to be set.
func DetectForWizard(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string, ok bool) {
	if ciType, _, _, ciProject, ciRepository, _, _, ciOK := detectCIForge(getenv); ciOK {
		project := ciProject
		if project == "" {
			project = ciRepository
		}
		return ciType, project, true
	}
	if originType, _, originProject, originOK := parseGitOrigin(gitOrigin); originOK {
		return originType, originProject, true
	}
	return "", "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/forge/... -v`
Expected: PASS (all `forge` package tests, including the pre-existing `resolve_test.go` suite, unaffected).

- [ ] **Step 5: Roadmap + commit**

Flip T195's checkbox to `[x]` in `docs/tasks/native-generator-roadmap.md`'s Phase C section, add a one-paragraph completion note.

```bash
git add internal/forge/detect.go internal/forge/detect_test.go docs/tasks/native-generator-roadmap.md
git commit -m "feat(forge): export DetectForWizard for wizard platform pre-fill"
```

---

## Task 196: `internal/scaffold` layer-rule + wire platform-type pre-fill into `runPlatformWizard`

**Files:**
- Modify: `.claude/rules/coding.md` (layer-rules table)
- Modify: `internal/scaffold/wizard.go:1-16` (imports), `:334-364` (detector functions), `:481-487` (Step 1/2 of `runPlatformWizard`)
- Modify: `internal/scaffold/wizard_internal_test.go` (add `detectPlatform` tests)

**Interfaces:**
- Consumes: `forge.DetectForWizard` (Task 195).
- Produces: `detectPlatform(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string)` and `gitRemoteOriginURL() string`, both package-internal to `scaffold`, used by `runPlatformWizard`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/scaffold/wizard_internal_test.go` (same file already has `TestParseRemoteProject` — leave that test untouched, `parseRemoteProject` is not being removed):

```go
func TestDetectPlatform_GitLabCI(t *testing.T) {
	typ, project := detectPlatform(func(k string) string {
		m := map[string]string{"GITLAB_CI": "true", "CI_PROJECT_PATH": "group/project"}
		return m[k]
	}, "")
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/project", project)
}

// TestDetectPlatform_SelfHostedFallsBackToAnyHostParsing pins that self-hosted GitLab/GitHub
// Enterprise remotes — which forge.DetectForWizard cannot type, since parseGitOrigin only
// recognizes github.com/gitlab.com/dev.azure.com — still get their project path pre-filled via
// parseRemoteProject's any-host parsing, exactly like today.
func TestDetectPlatform_SelfHostedFallsBackToAnyHostParsing(t *testing.T) {
	typ, project := detectPlatform(func(string) string { return "" }, "https://git.company.com/team/service.git")
	assert.Equal(t, "", typ)
	assert.Equal(t, "team/service", project)
}

// TestDetectPlatform_AzureDevOpsNotOfferedByWizardFallsBackToPathParsing pins that a detected
// azure_devops type — not one of the wizard's two platform Select options (gitlab/github) — is
// discarded, falling back to any-host path parsing instead of feeding an invalid type into the
// Select.
func TestDetectPlatform_AzureDevOpsNotOfferedByWizardFallsBackToPathParsing(t *testing.T) {
	typ, project := detectPlatform(func(k string) string {
		m := map[string]string{"TF_BUILD": "true", "SYSTEM_TEAMPROJECT": "myproject", "BUILD_REPOSITORY_NAME": "myrepo"}
		return m[k]
	}, "https://dev.azure.com/myorg/myproject/_git/myrepo")
	assert.Equal(t, "", typ)
	assert.Equal(t, "myorg/myproject/_git/myrepo", project)
}

func TestDetectPlatform_NoDetection(t *testing.T) {
	typ, project := detectPlatform(func(string) string { return "" }, "")
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -run TestDetectPlatform -v`
Expected: FAIL with `undefined: detectPlatform` (compile error).

- [ ] **Step 3: Implement**

In `internal/scaffold/wizard.go`, update the import block (lines 1-16):

```go
import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning/calver"
)
```

Replace `detectRemoteProject` (lines 334-342) with `gitRemoteOriginURL` and `detectPlatform`, keeping `parseRemoteProject` (lines 344-364) exactly as-is:

```go
// gitRemoteOriginURL runs `git remote get-url origin` and returns its raw output, or "" when not
// in a git repo or on any error.
func gitRemoteOriginURL() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectPlatform pre-fills a new platform's type and project/repository path: CI
// environment/known-host detection (via internal/forge) identifies the type when possible; the
// project/repository path falls back to parseRemoteProject's any-host parsing when forge
// detection doesn't apply (self-hosted instances, no CI markers) or names a type the wizard's
// platform Select doesn't offer (only "github"/"gitlab" are wizard-supported today).
func detectPlatform(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string) {
	if t, p, ok := forge.DetectForWizard(getenv, gitOrigin); ok && (t == "github" || t == "gitlab") {
		if p == "" {
			p = parseRemoteProject(gitOrigin)
		}
		return t, p
	}
	return "", parseRemoteProject(gitOrigin)
}
```

In `runPlatformWizard`, replace the "Step 1"/"Step 2" preamble (original lines 481-487: the `detected := detectRemoteProject()` line plus the comment above it) so the function reads:

```go
		p := PlatformAnswer{}

		// Pre-fill type and project/repository path from CI env / git origin, when detectable.
		origin := gitRemoteOriginURL()
		detectedType, detectedProject := detectPlatform(os.Getenv, origin)
		p.Type = detectedType

		// Step 1: platform type.
		if err := themedForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Release platform").
					Options(
						huh.NewOption("GitLab", "gitlab"),
						huh.NewOption("GitHub", "github"),
					).
					Value(&p.Type),
			),
		).Run(); err != nil {
			return err
		}

		// Step 2: platform-specific fields.
		switch p.Type {
		case "github":
			if p.Repository == "" {
				p.Repository = detectedProject
			}
```

The `case "gitlab":` branch (currently starting `if p.Project == "" { p.Project = detected }`) changes only its variable name, from `detected` to `detectedProject`:

```go
		case "gitlab":
			if p.Project == "" {
				p.Project = detectedProject
			}
```

The rest of `runPlatformWizard` (the GitLab CI note, Step 3 token selection, appending to `a.Platforms`) is untouched by this task.

Update `.claude/rules/coding.md`'s layer-rules table (in the `## Layer rules` section) — insert a new row right after the `internal/cmd/` row:

```markdown
| `internal/cmd/`      | `internal/{app,ui,config,scaffold,commitwizard}/`                                  |
| `internal/scaffold/` | `internal/{config,ui,versioning,forge}/`                                           |
| `internal/app/`      | `internal/{port,config,pipeline,versioning,generators,platforms,adapter,ui,conventionalcommit}/` |
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v` (all pass, including the pre-existing `TestParseRemoteProject`)
Then: `go build ./...` (confirms the whole repo still compiles with the new layer-rule-consistent import)

- [ ] **Step 5: Roadmap + commit**

Flip T196's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/wizard_internal_test.go .claude/rules/coding.md docs/tasks/native-generator-roadmap.md
git commit -m "feat(scaffold): pre-fill platform type from CI/git-origin detection"
```

---

## Task 197: `Answers.ChangelogGenerator`/`NotesGenerator` → `EnableChangelog`/`EnableReleaseNotes` bools

This is the largest task: it must land atomically (Go requires the whole `scaffold` package to compile together — the struct rename, `mainForm`'s groups, `generate.go`'s gates, `Defaults()`, and `ConfigToAnswers` cannot be split across separate commits without breaking the build).

**Files:**
- Modify: `internal/scaffold/wizard.go:24-53` (`Answers` struct), `:99-110` (`Defaults`), `:113-190` (`ConfigToAnswers`), `:246-269` (`mainForm` groups 5-6)
- Modify: `internal/scaffold/generate.go:82-99` (gates)
- Modify: `internal/scaffold/wizard_test.go` (renamed/updated tests)
- Modify: `internal/scaffold/generate_test.go` (renamed/updated tests)

**Interfaces:**
- Produces: `Answers.EnableChangelog bool`, `Answers.EnableReleaseNotes bool` — consumed by `generate.go`'s `answersToConfig`, and by Tasks 199/201's new prompts (which gate on these).

- [ ] **Step 1: Update the tests first (they will fail to compile)**

In `internal/scaffold/wizard_test.go`, replace `TestDefaults_Generator` (lines 23-26):

```go
func TestDefaults_EnableChangelogAndNotes(t *testing.T) {
	a := scaffold.Defaults()
	assert.True(t, a.EnableChangelog)
	assert.True(t, a.EnableReleaseNotes)
}
```

Replace `TestConfigToAnswers_ChangelogGenerator` (lines 71-82):

```go
func TestConfigToAnswers_EnableChangelog(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Output: "CHANGELOG.md"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.EnableChangelog)
	assert.Equal(t, "CHANGELOG.md", a.ChangelogOutput)
}
```

Replace `TestConfigToAnswers_NoChangelog` (lines 84-91):

```go
func TestConfigToAnswers_NoChangelog(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.False(t, a.EnableChangelog)
}
```

Replace `TestConfigToAnswers_NotesGenerator` (lines 155-165):

```go
func TestConfigToAnswers_EnableReleaseNotes(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Notes: &config.ContentDriver{},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.EnableReleaseNotes)
}
```

Replace `TestConfigToAnswers_GeneratorPresenceSurvivesLoadRoundTrip` (lines 167-201) — same regression class, simplified now that there's no sentinel workaround to guard:

```go
// TestConfigToAnswers_ChangelogAndNotesPresenceSurviveLoadRoundTrip guards against a regression
// where ConfigToAnswers under-reports block presence after a real config.Load round trip (as
// opposed to a hand-built struct literal, which could carry any value and would not catch a
// derivation bug). Re-running `heraut init` against an existing config and accepting the
// pre-populated defaults must not silently drop changelog/release-notes generation.
func TestConfigToAnswers_ChangelogAndNotesPresenceSurviveLoadRoundTrip(t *testing.T) {
	yaml := `
version: "1"
versioning:
  strategy: semver
  tag_prefix: ""
changelog:
  output: CHANGELOG.md
forges:
  - name: Primary GitLab
    platform: gitlab
release:
  notes: {}
  targets:
    - forge: Primary GitLab
`
	cfg, err := config.LoadFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	a := scaffold.ConfigToAnswers(cfg)

	assert.True(t, a.EnableChangelog)
	assert.True(t, a.EnableReleaseNotes)
}
```

In `internal/scaffold/generate_test.go`, apply these exact struct-literal replacements (each function's other lines are unchanged):

`TestGenerateYAML_SemVer` (lines 39-46) — replace the literal:

```go
	a := scaffold.Answers{
		Strategy:           "semver",
		TagPrefix:          "v",
		EnableChangelog:    true,
		ChangelogOutput:    "CHANGELOG.md",
		EnableReleaseNotes: true,
		Platforms:          []scaffold.PlatformAnswer{{Type: "github", Repository: "org/repo"}},
	}
```

`TestGenerateYAML_CalVer` (lines 62-69):

```go
	a := scaffold.Answers{
		Strategy:        "calver",
		TagPrefix:       "",
		Format:          "YYYY.MM.PATCH",
		EnableChangelog: true,
		ChangelogOutput: "CHANGELOG.md",
		Platforms:       []scaffold.PlatformAnswer{{Type: "gitlab"}},
	}
```

`TestGenerateYAML_PerEnv` (lines 107-117):

```go
	a := scaffold.Answers{
		Strategy:        "semver-per-env",
		TagFormat:       "{env}/{version}",
		EnableChangelog: true,
		ChangelogOutput: "CHANGELOG.md",
		Platforms:       []scaffold.PlatformAnswer{{Type: "gitlab"}},
		Environments: []scaffold.EnvAnswer{
			{Name: "dev", Bump: "auto"},
			{Name: "prod", Bump: "promote", Source: "dev"},
		},
	}
```

`TestGenerateYAML_PlatformNamesDefaultedAndDeduped` (lines 133-142):

```go
	a := scaffold.Answers{
		Strategy:        "semver",
		EnableChangelog: true,
		ChangelogOutput: "CHANGELOG.md",
		Platforms: []scaffold.PlatformAnswer{
			{Type: "github", Repository: "acme/widget"},
			{Type: "gitlab", Project: "acme/widget"},
			{Type: "gitlab", Project: "tools/widget-mirror"},
		},
	}
```

`TestGenerateYAML_CalVerSprint` (lines 163-170):

```go
	a := scaffold.Answers{
		Strategy:        "calver",
		Format:          "YYYY.SPRINT.PATCH",
		Sprint:          3,
		EnableChangelog: true,
		ChangelogOutput: "CHANGELOG.md",
		Platforms:       []scaffold.PlatformAnswer{{Type: "gitlab"}},
	}
```

`TestGenerateYAML_NoPlatforms` (lines 196-199):

```go
	a := scaffold.Answers{
		Strategy:        "semver",
		EnableChangelog: true,
	}
```

`TestGenerateYAML_DefaultsEmptyChangelogOutput` (lines 373-377):

```go
	a := scaffold.Answers{
		Strategy:        "semver",
		EnableChangelog: true,
		ChangelogOutput: "",
	}
```

`TestGenerateYAML_PlatformUsesPassthroughName` (lines 279-285):

```go
	a := scaffold.Answers{
		Strategy:           "semver",
		EnableReleaseNotes: true,
		Platforms: []scaffold.PlatformAnswer{
			{Name: "gh-internal", Type: "github", Repository: "org/repo", TokenEnv: "GH_TOKEN"},
		},
	}
```

`TestGenerateYAML_PlatformPassthroughFieldsRoundTrip` (lines 297-306):

```go
	a := scaffold.Answers{
		Strategy:           "semver",
		EnableReleaseNotes: true,
		Platforms: []scaffold.PlatformAnswer{
			{
				Name: "gh-internal", Type: "github", Repository: "org/repo", TokenEnv: "GH_TOKEN",
				BaseURL: "https://github.example.com", Draft: true, Prerelease: true,
			},
		},
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -v`
Expected: FAIL to compile — `unknown field EnableChangelog in struct literal of type scaffold.Answers` (and similar for the other renamed fields/tests).

- [ ] **Step 3: Implement**

In `internal/scaffold/wizard.go`, replace the `Answers` struct's changelog/notes fields (lines 29-32):

```go
	EnableChangelog bool
	ChangelogOutput string // e.g. "CHANGELOG.md"

	EnableReleaseNotes bool
```

Replace `Defaults()` (lines 99-110):

```go
// Defaults returns opinionated non-interactive defaults: semver, prefix "v",
// changelog + release notes enabled, gitlab platform.
func Defaults() Answers {
	return Answers{
		Strategy:           "semver",
		TagPrefix:          "v",
		EnableChangelog:    true,
		ChangelogOutput:    "CHANGELOG.md",
		EnableReleaseNotes: true,
		Platforms:          []PlatformAnswer{{Type: "gitlab"}},
	}
}
```

In `ConfigToAnswers`, replace the changelog block (lines 132-142):

```go
	if cfg.Changelog != nil {
		a.EnableChangelog = true
		a.ChangelogOutput = cfg.Changelog.Output
		if a.ChangelogOutput == "" {
			a.ChangelogOutput = "CHANGELOG.md"
		}
	}
```

And the notes line inside the `if cfg.Release != nil` block (lines 146-147):

```go
		a.EnableReleaseNotes = cfg.Release.Notes != nil
```

Replace `mainForm`'s groups 5-6 (lines 246-269) with three groups:

```go
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate a changelog?").
				Value(&a.EnableChangelog),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Changelog output file").
				Description(`e.g. "CHANGELOG.md"`).
				Value(&a.ChangelogOutput),
		).WithHideFunc(func() bool { return !a.EnableChangelog }),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate release notes?").
				Value(&a.EnableReleaseNotes),
		),
```

In `internal/scaffold/generate.go`, replace lines 82-99:

```go
	if a.EnableChangelog {
		output := a.ChangelogOutput
		if output == "" {
			output = "CHANGELOG.md"
		}
		cfg.Changelog = &config.ContentDriver{
			Output: output,
		}
	}

	hasNotes := a.EnableReleaseNotes
	hasPlatforms := len(a.Platforms) > 0
	hasAssets := len(a.Assets) > 0
	if hasNotes || hasPlatforms || hasAssets {
		cfg.Release = &config.Release{Assets: a.Assets}
		if hasNotes {
			cfg.Release.Notes = &config.ContentDriver{}
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v`
Expected: PASS. Then `go build ./...` to confirm the whole repo compiles.

- [ ] **Step 5: Roadmap + commit**

Flip T197's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/generate.go internal/scaffold/wizard_test.go internal/scaffold/generate_test.go docs/tasks/native-generator-roadmap.md
git commit -m "refactor(scaffold): replace decorative generator selects with confirms"
```

---

## Task 198: `Answers.RemoteMetadata` → `EnrichmentPolicy` rename

**Files:**
- Modify: `internal/scaffold/wizard.go:44-52` (`Answers` struct comment + field), `:119` (`ConfigToAnswers`)
- Modify: `internal/scaffold/generate.go:48-49`
- Modify: `internal/scaffold/generate_test.go` (renamed tests)

**Interfaces:**
- Produces: `Answers.EnrichmentPolicy string` (was `RemoteMetadata`) — consumed by `generate.go`'s `answersToConfig` and, in Task 201, by the new enrichment-policy prompt.

- [ ] **Step 1: Update the tests first**

In `internal/scaffold/generate_test.go`, replace `TestConfigToAnswers_PreservesAssetsTicketsRemoteMetadata` (lines 222-236):

```go
func TestConfigToAnswers_PreservesAssetsTicketsEnrichmentPolicy(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits: &config.Commits{
			EnrichmentPolicy: "required",
			Tickets:          []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}},
		},
		Release: &config.Release{Assets: []string{"dist/*.tar.gz"}},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "required", a.EnrichmentPolicy)
	assert.Equal(t, []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}, a.Tickets)
	assert.Equal(t, []string{"dist/*.tar.gz"}, a.Assets)
}
```

Replace `TestGenerateYAML_AssetsTicketsRemoteMetadata` (lines 238-254):

```go
func TestGenerateYAML_AssetsTicketsEnrichmentPolicy(t *testing.T) {
	a := scaffold.Answers{
		Strategy:         "semver",
		EnrichmentPolicy: "required",
		Tickets:          []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}},
		Assets:           []string{"dist/*.tar.gz"},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	assert.Equal(t, "required", cfg.EnrichmentPolicy())
	assert.Equal(t, []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}, cfg.Tickets())
	require.NotNil(t, cfg.Release)
	assert.Equal(t, []string{"dist/*.tar.gz"}, cfg.Release.Assets)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -v`
Expected: FAIL to compile — `unknown field EnrichmentPolicy in struct literal of type scaffold.Answers`.

- [ ] **Step 3: Implement**

In `internal/scaffold/wizard.go`, replace the `Answers` struct's passthrough-fields comment and field (lines 44-52):

```go
	// Assets, Tickets, and EnrichmentForge are not wizard-editable; they are carried through
	// verbatim from an existing config on "Update it?" (T107). EnrichmentForge references a
	// forges[].name (by the pre-rebuild name, matched positionally like Platforms' passthrough
	// fields — see matchPlatformSnapshot); forge selection has no wizard prompt yet (that
	// redesign is T164/P4).
	Assets           []string
	Tickets          []config.Ticket
	EnrichmentPolicy string
	EnrichmentForge  string
```

In `ConfigToAnswers` (line 119), rename the field in the struct literal:

```go
		EnrichmentPolicy: cfg.EnrichmentPolicy(),
```

In `internal/scaffold/generate.go`, replace lines 48-49:

```go
	if len(a.Tickets) > 0 || a.EnrichmentPolicy != "" {
		cfg.Commits = &config.Commits{Tickets: a.Tickets, EnrichmentPolicy: a.EnrichmentPolicy}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v`

- [ ] **Step 5: Roadmap + commit**

Flip T198's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/generate.go internal/scaffold/generate_test.go docs/tasks/native-generator-roadmap.md
git commit -m "refactor(scaffold): rename Answers.RemoteMetadata to EnrichmentPolicy"
```

---

## Task 199: Independent "Publish releases?" confirm replaces the `NotesGenerator`-gated trigger

**Files:**
- Modify: `internal/scaffold/wizard.go:34` (`Answers` struct — add field near `Platforms`), `:99-110` (`Defaults`), `:113-190` (`ConfigToAnswers`), `:289-293` (post-form gate)
- Modify: `internal/scaffold/wizard_test.go` (new tests)

**Interfaces:**
- Produces: `Answers.PublishReleases bool` — a wizard-flow-control field only (whether `runPlatformWizard` runs). It is **not** consumed by `generate.go`: `answersToConfig`'s existing `hasPlatforms := len(a.Platforms) > 0` check already correctly reflects "no platforms configured" when this is false, since a `false` answer simply means the wizard never populates `a.Platforms`. Do not add any new read of this field to `generate.go`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/scaffold/wizard_test.go`:

```go
func TestDefaults_PublishReleases(t *testing.T) {
	a := scaffold.Defaults()
	assert.True(t, a.PublishReleases)
}

func TestConfigToAnswers_PublishReleasesWhenPlatformsExist(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges:     []config.Forge{{Name: "github", Type: "github", Repository: "acme/widget"}},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github"}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.PublishReleases)
}

func TestConfigToAnswers_NoPublishReleasesWhenNoPlatforms(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.False(t, a.PublishReleases)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -run PublishReleases -v`
Expected: FAIL to compile — `a.PublishReleases undefined`.

- [ ] **Step 3: Implement**

In `internal/scaffold/wizard.go`'s `Answers` struct, add the field right after `Platforms` (line 34):

```go
	Platforms []PlatformAnswer

	// PublishReleases gates whether runPlatformWizard runs; it is not itself a config field —
	// once populated, Platforms alone determines whether forges:/release.targets: are emitted.
	PublishReleases bool
```

In `Defaults()`, add `PublishReleases: true,` alongside `Platforms`:

```go
func Defaults() Answers {
	return Answers{
		Strategy:           "semver",
		TagPrefix:          "v",
		EnableChangelog:    true,
		ChangelogOutput:    "CHANGELOG.md",
		EnableReleaseNotes: true,
		PublishReleases:    true,
		Platforms:          []PlatformAnswer{{Type: "gitlab"}},
	}
}
```

In `ConfigToAnswers`, right before `return a` at the end of the function, add:

```go
	a.PublishReleases = len(a.Platforms) > 0

	return a
```

Replace the post-form gate (lines 289-293):

```go
	if err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Publish releases to a platform (GitHub/GitLab)?").
				Value(&a.PublishReleases),
		),
	).Run(); err != nil {
		return err
	}

	if a.PublishReleases {
		if err := runPlatformWizard(a); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v`

- [ ] **Step 5: Roadmap + commit**

Flip T199's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/wizard_test.go docs/tasks/native-generator-roadmap.md
git commit -m "feat(scaffold): decouple platform setup from notes generation"
```

---

## Task 200: `api_mode` prompt in the GitLab platform branch

**Files:**
- Modify: `internal/scaffold/wizard.go:56-67` (`PlatformAnswer` struct), `:113-190` (`ConfigToAnswers` platform loop), `:532-567` (Step 3 of `runPlatformWizard`)
- Modify: `internal/scaffold/generate.go:110-117` (forge-building loop)
- Modify: `internal/scaffold/wizard_internal_test.go` (new `hideAPIMode` tests)
- Modify: `internal/scaffold/generate_test.go` / `wizard_test.go` (new round-trip tests)

**Interfaces:**
- Consumes: `platformTokenOptions`, `resolveTokenChoice` (existing, unchanged).
- Produces: `PlatformAnswer.APIMode string`, `hideAPIMode(platformType, tokenChoice string) bool` (package-internal, testable).

- [ ] **Step 1: Write the failing tests**

Add to `internal/scaffold/wizard_internal_test.go`:

```go
func TestHideAPIMode(t *testing.T) {
	tests := []struct {
		name         string
		platformType string
		tokenChoice  string
		want         bool
	}{
		{"github always hidden", "github", "GH_TOKEN", true},
		{"gitlab with CI_JOB_TOKEN hidden", "gitlab", "CI_JOB_TOKEN", true},
		{"gitlab with GITLAB_TOKEN shown", "gitlab", "GITLAB_TOKEN", false},
		{"gitlab with custom token shown", "gitlab", "custom", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hideAPIMode(tc.platformType, tc.tokenChoice))
		})
	}
}
```

Add to `internal/scaffold/generate_test.go`:

```go
func TestGenerateYAML_PlatformAPIMode(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		EnableReleaseNotes: true,
		Platforms: []scaffold.PlatformAnswer{
			{Type: "gitlab", Project: "acme/widget", TokenEnv: "GITLAB_TOKEN", APIMode: "graphql"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	require.Len(t, cfg.Forges, 1)
	assert.Equal(t, "graphql", cfg.Forges[0].APIMode)
	assert.Empty(t, config.Validate(cfg))
}

func TestConfigToAnswers_PreservesAPIMode(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges:     []config.Forge{{Name: "gitlab", Type: "gitlab", Project: "acme/widget", APIMode: "graphql"}},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "gitlab"}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	require.Len(t, a.Platforms, 1)
	assert.Equal(t, "graphql", a.Platforms[0].APIMode)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -run "TestHideAPIMode|APIMode" -v`
Expected: FAIL to compile — `hideAPIMode undefined`, `unknown field APIMode`.

- [ ] **Step 3: Implement**

In `internal/scaffold/wizard.go`, add `APIMode` to `PlatformAnswer` (lines 56-67):

```go
type PlatformAnswer struct {
	Type       string // "github" or "gitlab"
	Repository string // github: "owner/repo"
	Project    string // gitlab: "namespace/project"
	TokenEnv   string
	APIMode    string // gitlab only: "rest" (default) or "graphql"

	// Passthrough fields: not wizard-editable, carried verbatim from existing config (T108).
	Name       string
	BaseURL    string
	Draft      bool
	Prerelease bool
}
```

In `ConfigToAnswers`'s platform-rebuilding loop, add `APIMode: f.APIMode,`:

```go
			a.Platforms = append(a.Platforms, PlatformAnswer{
				Type:       f.Type,
				Repository: f.Repository,
				Project:    f.Project,
				TokenEnv:   f.TokenEnv,
				APIMode:    f.APIMode,
				Name:       f.Name,
				BaseURL:    f.BaseURL,
				Draft:      t.Draft,
				Prerelease: t.Prerelease,
			})
```

Add `hideAPIMode` near `resolveTokenChoice` (after it, before `matchPlatformSnapshot`):

```go
// hideAPIMode reports whether the api_mode prompt should stay hidden: for any non-GitLab
// platform, or when CI_JOB_TOKEN is the chosen token — GitLab's GraphQL API structurally rejects
// job tokens, so offering graphql there would let the wizard produce a config guaranteed to fail
// at enrichment time.
func hideAPIMode(platformType, tokenChoice string) bool {
	return platformType != "gitlab" || tokenChoice == "CI_JOB_TOKEN"
}
```

Replace `runPlatformWizard`'s Step 3 (lines 532-567):

```go
		// Step 3: token env var — select from known names or enter custom.
		tokenChoice, customToken := resolveTokenChoice(p.Type, p.TokenEnv)
		knownOpts := platformTokenOptions(p.Type)
		tokenOpts := make([]huh.Option[string], 0, len(knownOpts)+1)
		for _, k := range knownOpts {
			tokenOpts = append(tokenOpts, huh.NewOption(k, k))
		}
		tokenOpts = append(tokenOpts, huh.NewOption("Custom", "custom"))

		apiMode := p.APIMode
		if apiMode == "" {
			apiMode = "rest"
		}

		if err := themedForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Token environment variable").
					Options(tokenOpts...).
					Value(&tokenChoice),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Custom token environment variable").
					Value(&customToken).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("token env var is required")
						}
						return nil
					}),
			).WithHideFunc(func() bool { return tokenChoice != "custom" }),
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("API mode").
					Description("graphql renders linked @usernames but needs a read_api token — CI_JOB_TOKEN cannot use it").
					Options(
						huh.NewOption("rest", "rest"),
						huh.NewOption("graphql", "graphql"),
					).
					Value(&apiMode),
			).WithHideFunc(func() bool { return hideAPIMode(p.Type, tokenChoice) }),
		).Run(); err != nil {
			return err
		}

		if tokenChoice == "custom" {
			p.TokenEnv = strings.TrimSpace(customToken)
		} else {
			p.TokenEnv = tokenChoice
		}

		if !hideAPIMode(p.Type, tokenChoice) {
			p.APIMode = apiMode
		} else {
			p.APIMode = ""
		}

		a.Platforms = append(a.Platforms, p)
```

In `internal/scaffold/generate.go`'s forge-building loop, add `APIMode: p.APIMode,`:

```go
			cfg.Forges = append(cfg.Forges, config.Forge{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
				APIMode:    p.APIMode,
				BaseURL:    p.BaseURL,
			})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v`

- [ ] **Step 5: Roadmap + commit**

Flip T200's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/generate.go internal/scaffold/wizard_internal_test.go internal/scaffold/generate_test.go docs/tasks/native-generator-roadmap.md
git commit -m "feat(scaffold): add api_mode prompt for GitLab platforms"
```

---

## Task 201: Enrichment policy/forge prompts; `EnrichmentForge` becomes wizard-editable

**Files:**
- Modify: `internal/scaffold/wizard.go:44-52` (`Answers` struct comment, again — drops the `EnrichmentForge` "not wizard-editable" note), `:293-299` (post-form flow), new function `runEnrichmentWizard` + `shouldPromptEnrichmentForge`
- Modify: `internal/scaffold/generate.go:100-140` (extract `platformDisplayNames`, keep the first-forge fallback)
- Modify: `internal/scaffold/wizard_internal_test.go` (new tests)

**Interfaces:**
- Consumes: `Answers.EnableChangelog`/`EnableReleaseNotes` (Task 197), `Answers.EnrichmentPolicy` (Task 198), `Answers.Platforms` (existing).
- Produces: `platformDisplayNames(ps []PlatformAnswer) []string` (in `generate.go`, package-internal), `shouldPromptEnrichmentForge(platforms []PlatformAnswer) bool`, `runEnrichmentWizard(a *Answers) error` (both in `wizard.go`, package-internal).

- [ ] **Step 1: Write the failing tests**

Add to `internal/scaffold/wizard_internal_test.go`:

```go
func TestPlatformDisplayNames(t *testing.T) {
	names := platformDisplayNames([]PlatformAnswer{
		{Type: "github"},
		{Type: "gitlab"},
		{Type: "gitlab"},
		{Type: "github", Name: "gh-internal"},
	})
	assert.Equal(t, []string{"github", "gitlab", "gitlab-2", "gh-internal"}, names)
}

func TestShouldPromptEnrichmentForge(t *testing.T) {
	assert.False(t, shouldPromptEnrichmentForge(nil))
	assert.False(t, shouldPromptEnrichmentForge([]PlatformAnswer{{Type: "github"}}))
	assert.True(t, shouldPromptEnrichmentForge([]PlatformAnswer{{Type: "github"}, {Type: "gitlab"}}))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/... -run "TestPlatformDisplayNames|TestShouldPromptEnrichmentForge" -v`
Expected: FAIL to compile — both functions undefined.

- [ ] **Step 3: Implement**

In `internal/scaffold/generate.go`, replace the forge-building loop (the block starting `platformTypeCount := make(map[string]int)` through the closing `}` of that `for` loop) with:

```go
		names := platformDisplayNames(a.Platforms)
		for i, p := range a.Platforms {
			name := names[i]
			cfg.Forges = append(cfg.Forges, config.Forge{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
				APIMode:    p.APIMode,
				BaseURL:    p.BaseURL,
			})
			cfg.Release.Targets = append(cfg.Release.Targets, config.Target{
				Forge:      name,
				Draft:      p.Draft,
				Prerelease: p.Prerelease,
			})
		}
```

The comment and fallback logic right after it (the `commits.enrichment_forge is required...` block, currently lines 124-140) **stays exactly as-is** — per the design doc's correction, this fallback is validator-required for any non-wizard caller and must not be deleted. Only update its comment to reflect that the wizard now asks explicitly:

```go
		// commits.enrichment_forge is required once more than one forge is configured
		// (validateForges). The wizard (runEnrichmentWizard) now asks explicitly whenever
		// there are 2+ platforms, so a.EnrichmentForge is normally already set correctly by the
		// time this runs. This fallback exists for direct/non-wizard callers of answersToConfig
		// (e.g. a hand-built Answers, or a future --defaults preset with multiple platforms):
		// preserve an existing choice when it still names one of the rebuilt forges, else fall
		// back to the first configured forge.
		if len(cfg.Forges) > 1 {
			if cfg.Commits == nil {
				cfg.Commits = &config.Commits{}
			}
			enrichmentForge := a.EnrichmentForge
			if !forgeNameExists(cfg.Forges, enrichmentForge) {
				enrichmentForge = cfg.Forges[0].Name
			}
			cfg.Commits.EnrichmentForge = enrichmentForge
		}
```

Add `platformDisplayNames` right after `answersToConfig` (before `forgeNameExists`):

```go
// platformDisplayNames returns the forges[].name each platform in ps will be assigned once
// generated — type-based, deduplicated with a "-N" suffix for repeats. answersToConfig and the
// wizard's enrichment-forge prompt both need the exact same computation so the names offered to
// the user match what GenerateYAML actually writes.
func platformDisplayNames(ps []PlatformAnswer) []string {
	names := make([]string, len(ps))
	typeCount := make(map[string]int)
	for i, p := range ps {
		typeCount[p.Type]++
		name := p.Name
		if name == "" {
			name = p.Type
			if n := typeCount[p.Type]; n > 1 {
				name = fmt.Sprintf("%s-%d", p.Type, n)
			}
		}
		names[i] = name
	}
	return names
}
```

In `internal/scaffold/wizard.go`, update the `Answers` struct comment (from Task 198's version) to drop the `EnrichmentForge` "not wizard-editable" note, since both fields become wizard-editable in this task:

```go
	// Assets and Tickets are not wizard-editable; they are carried through verbatim from an
	// existing config on "Update it?" (T107).
	Assets           []string
	Tickets          []config.Ticket
	EnrichmentPolicy string
	EnrichmentForge  string
```

Add `shouldPromptEnrichmentForge` and `runEnrichmentWizard` after `runPlatformWizard` (before `runEnvWizard`):

```go
// shouldPromptEnrichmentForge reports whether the enrichment-forge select should be shown: only
// when 2+ platforms are configured, since with 0 or 1 the choice is unambiguous and the field is
// left for runtime auto-detection or the single configured forge to resolve.
func shouldPromptEnrichmentForge(platforms []PlatformAnswer) bool {
	return len(platforms) >= 2
}

// runEnrichmentWizard asks the PR/MR enrichment policy whenever changelog or release-notes
// generation is enabled — independent of whether publishing is configured, since enrichment can
// work off a zero-config auto-detected forge even with no explicit forges: block — and, only
// when the choice is actually ambiguous (2+ configured platforms), which one supplies the data.
func runEnrichmentWizard(a *Answers) error {
	if !a.EnableChangelog && !a.EnableReleaseNotes {
		return nil
	}

	if a.EnrichmentPolicy == "" {
		a.EnrichmentPolicy = "optional"
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("PR/MR enrichment policy").
				Options(
					huh.NewOption("optional (fetch when possible, else warn)", "optional"),
					huh.NewOption("required (fail if unavailable)", "required"),
					huh.NewOption("disabled (never fetch)", "disabled"),
				).
				Value(&a.EnrichmentPolicy),
		),
	}

	if shouldPromptEnrichmentForge(a.Platforms) {
		names := platformDisplayNames(a.Platforms)
		forgeOpts := make([]huh.Option[string], len(names))
		for i, name := range names {
			forgeOpts[i] = huh.NewOption(name, name)
		}
		if !slices.Contains(names, a.EnrichmentForge) {
			a.EnrichmentForge = names[0]
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Enrichment forge").
				Description("Which platform supplies PR/MR metadata?").
				Options(forgeOpts...).
				Value(&a.EnrichmentForge),
		))
	}

	return themedForm(groups...).Run()
}
```

Wire it into `RunWizard`'s post-form flow, right after the `PublishReleases`-gated `runPlatformWizard` call and before the per-env branch (lines 293-299 in the pre-Task-199 numbering — by this point in the plan it directly follows Task 199's new gate):

```go
	if a.PublishReleases {
		if err := runPlatformWizard(a); err != nil {
			return err
		}
	}

	if err := runEnrichmentWizard(a); err != nil {
		return err
	}

	if a.Strategy == "semver-per-env" || a.Strategy == "calver-per-env" {
		if err := runEnvWizard(a); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/... -v` — this must also confirm the pre-existing `TestGenerateYAML_PlatformNamesDefaultedAndDeduped`, `TestGenerateYAML_PreservesEnrichmentForgeOnSecondForge`, and `TestGenerateYAML_EnrichmentForgeDefaultsToFirstWhenUnset` still pass unchanged (the `platformDisplayNames` extraction is a pure refactor of already-tested behavior).

- [ ] **Step 5: Roadmap + commit**

Flip T201's checkbox to `[x]`, add a completion note.

```bash
git add internal/scaffold/wizard.go internal/scaffold/generate.go internal/scaffold/wizard_internal_test.go docs/tasks/native-generator-roadmap.md
git commit -m "feat(scaffold): add enrichment policy/forge prompts"
```

---

## Task 202: Delete `internal/scaffold/cliff.go`; cleanup sweep; verification

**Files:**
- Delete: `internal/scaffold/cliff.go`, `internal/scaffold/cliff_test.go`

**Interfaces:**
- Consumes: nothing (confirmed in this plan's own research: `IsCliffGenerator` has no callers outside its own test).
- Produces: nothing new — this task only removes dead code and verifies the whole phase.

- [ ] **Step 1: Delete the files**

```bash
git rm internal/scaffold/cliff.go internal/scaffold/cliff_test.go
```

- [ ] **Step 2: Confirm the build is unaffected**

Run: `go build ./...`
Expected: succeeds — `IsCliffGenerator` has no other callers in the repo.

- [ ] **Step 3: Run the cleanup sweep**

Run: `grep -rn "gitcliff\|git-cliff\|communique" internal/scaffold/`
Expected: **no output**. By this point in the plan, Task 197 already replaced every git-cliff/communique-referencing line in `wizard.go` (the struct field comments, `Defaults()`'s literals, the sentinel-string lines, and the `huh.NewOption` entries) and their corresponding test literals in `wizard_test.go`/`generate_test.go`; this task's deletion removes the last two files (`cliff.go`, `cliff_test.go`) that still referenced them. If the grep finds anything, it is a plan gap — investigate and fix before proceeding, do not skip this check.

- [ ] **Step 4: Full-suite verification**

Run: `go test ./... && hk check`
Expected: clean.

- [ ] **Step 5: Best-effort manual wizard verification**

Run: `go run ./cmd/heraut init --defaults --config /tmp/heraut-phase-c-smoke.heraut.yml` in a scratch directory and inspect the output — confirms the non-interactive `--defaults` path (which exercises `Defaults()`'s new bool fields end-to-end) still produces a config that validates.

If an interactive TTY is available in this environment, also run `go run ./cmd/heraut init --config /tmp/heraut-phase-c-smoke2.heraut.yml` and step through the wizard, confirming: "Generate a changelog?"/"Generate release notes?" toggle their respective config blocks; "Publish releases?" gates the platform loop; the GitLab platform branch's api_mode prompt is hidden when `CI_JOB_TOKEN` is chosen and shown otherwise; the enrichment policy prompt appears whenever changelog or notes is on, and the enrichment forge prompt appears only with 2+ configured platforms. If no TTY is available (a headless subagent environment, for instance), state that explicitly rather than claiming the interactive flow was verified — the automated test suite already covers every pure-function decision point (`hideAPIMode`, `shouldPromptEnrichmentForge`, `detectPlatform`, `platformDisplayNames`); only the `huh` form wiring itself lacks automated coverage, matching this codebase's existing pattern (no prior test in this package drives `RunWizard` interactively either).

- [ ] **Step 6: Roadmap + commit**

Flip T202's checkbox to `[x]`. Add a completion note summarizing the whole phase (all 8 tasks, any deviations found during execution), matching the style of Phase B's closing summary in the same roadmap file.

```bash
git add -A
git commit -m "chore(scaffold): delete dead git-cliff scaffold helper"
```

---

## Self-Review Notes (from plan authoring)

- **Spec coverage:** all five design-doc decisions are covered — Decision 1 (Task 196), Decision 2 (Task 197), Decision 3 (Task 199), Decision 4 (Task 200), Decision 5 (Task 201). The cleanup sweep (Task 202) and the `internal/forge` export + layer-rule change (Tasks 195-196) are covered. The three corrections found during this plan's own research (the first-forge fallback staying, `parseRemoteProject` staying, no forced default-seeding) are reflected in Tasks 196, 197, and 201's exact code — not just noted in the spec.
- **No placeholders:** every step above has real code, not a description of code. Every renamed test's full new body is written out; no "similarly update the others."
- **Type consistency checked:** `Answers.EnableChangelog`/`EnableReleaseNotes`/`EnrichmentPolicy`/`PublishReleases` (bool/bool/string/bool) and `PlatformAnswer.APIMode` (string) are used with the same names and types everywhere they appear across Tasks 197-201, including in `generate.go`'s consumption and every test literal.
