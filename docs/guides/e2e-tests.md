# End-to-end tests

heraut's `e2e/` tree drives the **built `heraut` binary**. It has two lanes
([ADR-0066](../adr/0066-e2e-test-lanes.md), design:
[`2026-10-07-e2e-smoke-suite-design.md`](../superpowers/specs/2026-10-07-e2e-smoke-suite-design.md)):

| Lane | What it runs against | When it runs |
|---|---|---|
| **A, hermetic** | throwaway local git repositories, a local bare remote, fake `gh`/`glab` that record their arguments | every `go test ./...`, so every pull request |
| **B, forge sandbox** | real private GitHub and GitLab repositories, the real `gh`/`glab` | on demand and nightly, never on pull requests (`e2e_forge` build tag) |

Lane A needs nothing. This guide is about Lane B.

## What Lane B covers

Each scenario runs once per forge unless noted. Everything it creates carries a run id
(`e2e-<unix>-<hex>`) and is deleted afterwards.

| # | Scenario |
|---|---|
| B1 | final release: notes in the body, the tag points at the changelog commit |
| B2 | pre-release: GitHub marks it, GitLab creates a plain release |
| B3 | `+` build-metadata tag and the exact release URL spelling |
| B4 | per-env `<id>/{env}/{version}` tag with an uploaded asset |
| B5 | CalVer tag (simulated clock) |
| B6 | one run on a GitLab source publishes to GitLab **and** GitHub |
| B7 | GitHub draft release |
| B8 | a maintenance line: next patch of its range, then a taken version refused |
| B9 | PR/MR enrichment: the changelog links the merged request (needs the second pair below) |
| B10 | `--dry-run` creates nothing on the forge |

## Setting up the sandboxes

Create **private** repositories whose base name matches `*testing*` (override with
`HERAUT_E2E_REPO_PATTERN`):

- one on GitHub and one on GitLab for B1-B8 and B10 (for example `<owner>/<name>-testing`);
- optionally a second pair for B9 (for example `<owner>/<name>-testing-enrich`), with no
  branch protection on the branches the tests create. B9 opens and merges a pull/merge request
  into a per-run base branch, so the default branch is never written. Merged requests cannot be
  deleted; they stay as closed history in that pair.

Each repository needs a default branch called `main` with at least one commit.

## Configuration

Only environment variables; nothing is read from files and nothing real is committed.

| Variable | Meaning |
|---|---|
| `HERAUT_E2E_GITHUB_REPO` | `owner/name` of the GitHub sandbox; unset skips the GitHub scenarios |
| `HERAUT_E2E_GITLAB_PROJECT` | `group/project` of the GitLab sandbox; unset skips the GitLab scenarios |
| `HERAUT_E2E_GITHUB_ENRICH_REPO`, `HERAUT_E2E_GITLAB_ENRICH_PROJECT` | the B9 pair; unset skips B9 only |
| `HERAUT_E2E_REPO_PATTERN` | base-name pattern a sandbox must match (default `*testing*`) |
| `HERAUT_E2E_GITHUB_TOKEN`, `HERAUT_E2E_GITLAB_TOKEN` | tokens; fall back to `GH_TOKEN`/`GITHUB_TOKEN` and `GITLAB_TOKEN`, then to the local `gh`/`glab` login |

## Running locally

Log in with `gh auth login` and `glab auth login`, export the coordinates, then:

```bash
mise run test:e2e          # all Lane B scenarios
go run -tags e2e_forge ./e2e/cmd/sweep -dry-run    # list stale leftovers without deleting
```

The tokens are read in-process and handed to `gh`/`glab` through the environment only.

## Running in CI

`.github/workflows/e2e.yml` runs on `workflow_dispatch` and nightly. It is advisory: nothing
requires it, and a failure produces GitHub's built-in email for failed scheduled workflows. It
never runs on pull requests.

Create in the repository settings:

- **variables**: the four coordinate variables above;
- **secrets**: `HERAUT_E2E_GITHUB_TOKEN` (a fine-grained token limited to the sandbox repositories, the enrichment pair included,
  with Contents and Pull requests read/write) and `HERAUT_E2E_GITLAB_TOKEN` (a token with the `api`
  scope and Maintainer role on the sandbox projects only).

The job first runs the harness's own offline tests (including the safety-guard tests), then the
sweeper, then the scenarios.

## Safety

- A sandbox is refused unless its base name matches the pattern **and** it is private.
- Every deletion targets a name that contains the run id; anything else is refused and reported.
- Each run works on its own `e2e/<run-id>` branch from `main`, and tags carry the run id.
- Cleanup deletes releases, then tags, then branches (deleting a tag first would turn its release
  into a leaked draft) and **fails the test** if anything cannot be deleted.
- `e2e/cmd/sweep` removes `e2e-*` leftovers older than 24 hours from every configured sandbox, the
  enrichment pair included (`-older-than`, `-dry-run`); run it by hand after a crash. It cannot close
  a merge request left open by a crashed B9 on GitLab (deleting the branch closes it on GitHub).
- Scenarios wait for the forges with bounded polling (up to a few minutes in the worst case), so the
  test timeout is 40 minutes; Go's default of 10 would abort the run and skip the cleanup.

## Things to know

- GitHub's release list and its commit-to-pull-request association lag behind by seconds, so the
  tests look releases up by tag and poll with a bounded timeout.
- GitLab has no pre-release or draft flag; B2 and B7 assert them only where the forge has one.
- Each target's notes link commits on its own forge, so B6 compares the notes with the link
  targets removed.
