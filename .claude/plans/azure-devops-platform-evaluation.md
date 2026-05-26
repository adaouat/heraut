# Effort Evaluation: Azure DevOps Platform Support

## Context

The user wants to know what it takes to add Azure DevOps as a third platform, and which
tooling exists (CLI, Go SDK, APIs). This document evaluates feasibility and maps the
implementation effort before committing to build.

---

## Critical finding: Azure DevOps has no GitHub-style releases

GitHub and GitLab both have a native "Release" object: a tag + metadata + downloadable
binary assets, all reachable via a single URL. **Azure DevOps does not have this concept.**

| Concept | GitHub / GitLab | Azure DevOps |
|---------|----------------|--------------|
| Release object | Native (tag + notes + assets) | ❌ Does not exist |
| Git tag | Metadata only | Metadata only |
| Binary assets | Attached to release | Must use Azure Artifacts |
| Release notes | Field on release object | Git tag annotation only |
| `CreateRelease` API | First-class API call | No equivalent |
| `UploadAssets` API | Attach files to release | Publish Universal Package to a feed |

Azure DevOps "Releases" (`az pipelines release`) are CI/CD deployment definitions, not
distribution-of-binaries objects. They are completely unrelated to git tags.

---

## Available tooling

### CLI: `az` (Azure CLI) with `azure-devops` extension

The Azure CLI (`az`) is the natural analog to `gh` and `glab`. It requires the
`azure-devops` extension to be installed (`az extension add --name azure-devops`).

**Relevant commands:**

| Purpose | Command |
|---------|---------|
| Check CLI + extension | `az extension show --name azure-devops` |
| Check auth | `az account show` or `az devops configure --list` |
| Upload binary assets | `az artifacts universal publish --organization … --feed … --name … --version … --path <dir>` |
| Create annotated tag | `az repos git annotated-tag create` (REST API, not a CLI command) |

**Auth:** `AZURE_DEVOPS_EXT_PAT` environment variable (PAT token).

**Key limitation:** `az artifacts universal publish` takes a **directory**, not individual
files. All assets to publish must be placed in a staging directory first.
This is different from `gh release upload` (per-file) and `glab release upload` (batch files).

### Go SDK: `github.com/microsoft/azure-devops-go-api/azuredevops/v7`

An official Microsoft SDK exists and is actively maintained. It covers:

- `azuredevops/git` — create/list annotated tags
- `azuredevops/universal` — Universal Packages (publish/download assets)
- `azuredevops/release` — pipeline releases (CI/CD only, irrelevant here)

Using the SDK directly would break heraut's CLI-wrapper pattern (`port.Runner`), making
this an architectural departure that requires an ADR. It would also bring in a
non-trivial external dependency.

**Verdict:** CLI approach (`az`) is preferable to stay consistent with the existing
pattern. The SDK is an option only if `az` proves insufficient.

---

## How the Platform interface maps to Azure DevOps

| Interface method | Azure DevOps mapping | Difficulty |
|-----------------|---------------------|------------|
| `Name()` | Returns `"azure-devops"` | Trivial |
| `ReleaseURL(tag)` | URL to Azure Artifacts feed package | Easy (format string) |
| `Check()` | Verify `az` + extension + PAT + org/project/feed config | Moderate |
| `CreateRelease(tag, notes)` | **No native equivalent.** Heraut already creates the git tag; this method becomes a no-op that prints the artifacts URL | Design decision |
| `HasAssets()` | `len(cfg.Assets) > 0` | Trivial |
| `UploadAssets(tag)` | Stage files into tmp dir → `az artifacts universal publish` | Moderate |

`CreateRelease` is the hardest method to map. The recommended approach is to make it a
no-op (the tag + notes are already embedded in the annotated git tag by heraut itself)
and emit the Artifacts URL as confirmation output. This requires a short ADR.

---

## New config fields required

Azure DevOps needs significantly more configuration than GitHub/GitLab because the
assets live outside the git repository (in a feed).

```yaml
- platform: azure-devops
  organization: mycompany                # required — Azure DevOps org name
  project: myproject                     # required — Azure DevOps project name
  feed: releases                         # required — Azure Artifacts feed name
  package_name: myapp                    # required — package name in the feed
  token_env: AZURE_DEVOPS_EXT_PAT        # optional — defaults to AZURE_DEVOPS_EXT_PAT
  assets:                                # optional — same glob patterns as GitHub/GitLab
    - dist/myapp_*
    - dist/checksums.txt
```

Fields to add to `config.Platform`, `schema.json`, and `docs/heraut.sample.yml`:
- `organization` (string, required for Azure DevOps)
- `project` (string, required for Azure DevOps)
- `feed` (string, required for Azure DevOps)
- `package_name` (string, required for Azure DevOps)

Since `config.Platform` uses `additionalProperties: false` in the schema, these fields
are safe to add (they are ignored by other platforms).

---

## Implementation plan (if approved)

### Step 0 — ADR
Write `docs/adr/0017-azure-devops-platform.md` documenting:
- Why `CreateRelease` is a no-op (no native Azure release object)
- Why Universal Packages are used for assets
- Why `az` CLI is chosen over the Go SDK
- The staging-directory pattern for `az artifacts universal publish`

### Step 1 — Config + Schema + Sample (~30 min)
- Add `Organization`, `Project`, `Feed`, `PackageName` fields to `config.Platform` in
  `internal/config/config.go`
- Add corresponding entries to `schema.json` with descriptions and `"azure-devops only"`
  annotations
- Add Azure DevOps example block to `docs/heraut.sample.yml`

### Step 2 — Platform implementation, TDD (~3–4 h)

Create `internal/platforms/azuredevops/platform_test.go` first, then implement
`internal/platforms/azuredevops/platform.go`.

Contract tests (MockRunner assertions) needed for:
- `Check()`: `az extension show`, token env presence, required fields validation
- `CreateRelease()`: verify it is a no-op (no runner calls) and emits the artifacts URL
- `UploadAssets()`: glob resolution → staging dir creation → `az artifacts universal publish` call with correct args

The staging-directory step in `UploadAssets` is the most complex piece: resolved files
must be copied into `t.TempDir()` before calling `az artifacts universal publish --path <dir>`.

### Step 3 — Wiring (~15 min)
- Add `case "azure-devops":` to `buildPlatform()` in `internal/app/platforms.go`
- Add `buildAzureDevOpsPlatform()` factory function in `internal/app/platforms.go`

### Step 4 — Validator update (~15 min)
- Add validation in `internal/config/validator.go`: when `platform == "azure-devops"`,
  require `organization`, `project`, `feed`, `package_name` to be non-empty

---

## Effort summary

| Phase | Effort |
|-------|--------|
| ADR | ~30 min |
| Config / schema / sample | ~30 min |
| Platform implementation + tests | ~3–4 h |
| Wiring + validator | ~30 min |
| **Total** | **~5–6 h** |

The bulk of the effort is in `UploadAssets`: the file-staging dance before
`az artifacts universal publish` and writing contract tests that verify the correct
directory and CLI arguments.

---

## Open questions before building

1. **Is the Universal Packages approach acceptable to users?** Assets are stored in an
   Azure Artifacts feed, not "attached to a tag". The download experience is
   `az artifacts universal download`, not a browser link. Is this good enough, or do
   users expect a GitHub-like download page?

2. **Should `CreateRelease` emit the feed URL or stay completely silent?** Printing the
   URL gives users feedback; staying silent matches the no-op intent cleanly.

3. **Minimum `az` version / extension version to document?** The `azure-devops` extension
   is required in addition to the base `az` CLI. Should `Check()` also verify the
   extension version?

---

## Verification

```bash
mise run test          # all packages green, including new azuredevops/ tests
mise run lint:check    # no new warnings
heraut check runtime   # (manual) verifies az + extension present on PATH
heraut release --dry-run   # (manual) prints azure-devops platform in plan output
```
