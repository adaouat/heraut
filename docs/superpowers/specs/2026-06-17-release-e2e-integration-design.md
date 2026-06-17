# E2E Integration Tests for heraut Release Workflow

**Date:** 2026-06-17
**Status:** Approved

## Problem

heraut's existing test suite has four layers (unit, contract, integration with FakeBin, schema), but none of them touch real platform APIs. Contract tests with `MockRunner` verify exact CLI arguments but cannot catch:
- Real token/auth failures
- Platform API changes or regressions
- Multi-platform release flows where both GitHub and GitLab receive real API calls

This design adds a real-API e2e layer that runs inside the release workflow before `heraut release` executes, using the actual GoReleaser-built binary.

## Goals

- Validate that the GoReleaser-built binary can successfully create releases on GitHub and GitLab
- Cover all four platform combinations: GitHub-only, GitLab-only, GitHub+GitLab, GitLab+GitHub
- Run inside the existing `release.yml` workflow (CI-only, not local)
- Fail the release workflow before any commit/tag/release is created if e2e tests fail
- Provide a `skip-e2e` bypass for hotfix releases

## Non-goals

- Content/notes validation (smoke only: exit 0 + release exists)
- Cleanup of test releases (dedicated repos, accumulate freely)
- Testing every versioning strategy (semver only, sufficient to exercise the platform path)

## Architecture

### Placement in release workflow

The e2e tests are inserted as a step inside the `release` job, between `Preflight check` and `Release`:

```
Setup → Build binaries → Collect → Attest → Sanity check → Preflight check
  → [E2E tests]  ← NEW
  → Release → Homebrew cask
```

If any e2e scenario fails, the job stops before `heraut release` runs. At that point:
- No changelog commit has been made
- No tag has been pushed
- No GitHub release exists

This gives a clean failure state with no rollback needed.

### Bypass

A `skip-e2e` boolean input on `workflow_dispatch` gates the step. When `true`, the e2e step is skipped and `heraut release` proceeds directly. Intended for hotfix releases only.

## Infrastructure

### Dedicated test repos

Two purpose-built repos exist permanently — never deleted between runs:

| Repo | Platform |
|------|----------|
| `adaouat/heraut-e2e` | GitHub |
| `adaouat/heraut-e2e` | GitLab |

Both repos are initialized as mirrors of heraut's commit history (realistic conventional commits for git-cliff to parse) with a `v0.0.0` baseline tag. Each e2e run pushes new synthetic commits, so the version increments naturally across runs. Releases accumulate in the test repos with no cleanup.

### Secrets

Two new secrets added to `adaouat/heraut` on GitHub:

| Secret | Purpose |
|--------|---------|
| `E2E_GITHUB_TOKEN` | Write access to `adaouat/heraut-e2e` on GitHub (create releases, push tags) |
| `E2E_GITLAB_TOKEN` | API access to `adaouat/heraut-e2e` on GitLab (create releases) |

## Test scenarios

Four scenarios, run sequentially:

| Scenario | Git remote | Platforms |
|----------|-----------|-----------|
| `github` | GitHub e2e repo | GitHub only |
| `gitlab` | GitLab e2e repo | GitLab only |
| `github+gitlab` | GitHub e2e repo | GitHub + GitLab |
| `gitlab+github` | GitLab e2e repo | GitLab + GitHub |

For multi-platform scenarios, the tag is pushed to the primary platform's repo (the git remote). Releases are created on both platforms independently via their CLIs.

## Script: `scripts/e2e.sh`

A single parameterised shell script. Called once per scenario from the workflow step.

**Interface:**

```bash
scripts/e2e.sh --scenario <name> --binary <path>
```

Tokens are read from environment variables: `GH_TOKEN`, `GITLAB_TOKEN` (matching the heraut platform driver defaults).

**Per-scenario execution:**

1. Clone the test repo for the scenario's primary platform
2. Configure git identity (name/email for synthetic commits)
3. Push 1–2 synthetic conventional commits (`feat: e2e run <timestamp>`)
4. Set `HERAUT_FILE` to the matching config fixture in `testdata/e2e/`
5. Run `$BINARY release`
6. Assert: `gh release view --repo adaouat/heraut-e2e` exits 0 (latest release — no need to know the exact tag); `glab release list --repo adaouat/heraut-e2e` for GitLab scenarios
7. Exit 0 on success, non-zero on failure

**Git authentication:**
- GitHub clone URL: `https://x-access-token:${E2E_GITHUB_TOKEN}@github.com/adaouat/heraut-e2e.git`
- GitLab clone URL: `https://oauth2:${E2E_GITLAB_TOKEN}@gitlab.com/adaouat/heraut-e2e.git`

## Config fixtures: `testdata/e2e/`

Four `.heraut.yml` files, one per scenario. All use:
- `versioning.strategy: semver`
- `changelog.generator: git-cliff`
- Hardcoded `repository`/`project`: `adaouat/heraut-e2e`

Example (`github+gitlab.yml`):

```yaml
versioning:
  strategy: semver

release:
  changelog:
    generator: git-cliff
  platforms:
    - name: github
      type: github
      repository: adaouat/heraut-e2e
    - name: gitlab
      type: gitlab
      project: adaouat/heraut-e2e
```

## Workflow changes (`release.yml`)

### New input

```yaml
skip-e2e:
  description: "Skip e2e integration tests (hotfix bypass)"
  required: false
  default: "false"
  type: boolean
```

### New step

```yaml
- name: E2E integration tests
  if: inputs.skip-e2e != true
  run: |
    for scenario in github gitlab "github+gitlab" "gitlab+github"; do
      bash scripts/e2e.sh --scenario "$scenario" --binary "./$FRESH_BIN"
    done
  env:
    # GH_TOKEN / GITLAB_TOKEN: the names heraut platform drivers read by default.
    # E2E tokens target adaouat/heraut-e2e, not adaouat/heraut.
    GH_TOKEN: ${{ secrets.E2E_GITHUB_TOKEN }}
    GITLAB_TOKEN: ${{ secrets.E2E_GITLAB_TOKEN }}
    # GITHUB_TOKEN + GITHUB_REPOSITORY: used by heraut's GitHub auth probe in CI
    # (checkAPIAuth reads these when GITHUB_ACTIONS=true). Leave GITHUB_REPOSITORY
    # unset so it falls back to the Actions default (adaouat/heraut), which the
    # main token can read. The actual release creation uses GH_TOKEN above.
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Files changed / created

| File | Change |
|------|--------|
| `.config/mise/config.toml` | Add `glab` (required for GitLab e2e scenarios) |
| `.github/workflows/release.yml` | Add `skip-e2e` input + E2E step |
| `scripts/e2e.sh` | New — e2e runner script |
| `testdata/e2e/github.yml` | New — GitHub-only config fixture |
| `testdata/e2e/gitlab.yml` | New — GitLab-only config fixture |
| `testdata/e2e/github+gitlab.yml` | New — GitHub+GitLab config fixture |
| `testdata/e2e/gitlab+github.yml` | New — GitLab+GitHub config fixture |

Two secrets must be added to the repo: `E2E_GITHUB_TOKEN` and `E2E_GITLAB_TOKEN`.

Two test repos must be created manually before the first e2e run:
- `adaouat/heraut-e2e` on GitHub
- `adaouat/heraut-e2e` on GitLab

Both initialized with heraut's commit history and a `v0.0.0` baseline tag.
