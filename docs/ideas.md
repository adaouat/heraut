# Future Ideas

Ideas worth tracking for post-v1.0, not yet scheduled. Each entry records enough context
to evaluate and scope the idea when the time comes.

---

## `adaouat/heraut-action` — GitHub Actions wrapper

A reusable GitHub Action that wraps heraut for use in other projects' CI pipelines, in
the spirit of [`orhun/git-cliff-action`](https://github.com/orhun/git-cliff-action).

**Concept:** A Docker container action backed by `ghcr.io/adaouat/heraut` (already bundles
all required CLIs: `git-cliff`, `gh`, `glab`, `cog`, `communique`). Other projects
would consume it as:

```yaml
- uses: adaouat/heraut-action@v1
  with:
    command: release          # release | changelog | version-next | check
    config: .heraut.yml
    env: production
    dry-run: false
    force: false
    version: ''              # version override for bump: manual
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Outputs:** `version` (resolved next version), `tag` (full tag string, e.g. `v1.4.2`),
`release-url`.

**Key decisions to make:**
- Separate repo (`adaouat/heraut-action`) vs. composite action in this repo under
  `.github/actions/`. Separate repo is cleaner and marketplace-publishable.
- Pin the Docker image tag in the action (e.g. `@v1`) and update it on each heraut release,
  or use `latest` (simpler, less predictable).
- Which inputs to expose for the first version (keep it minimal: command + config + env +
  dry-run).

**Relationship to Option B release flow:** the action makes heraut accessible in CI
pipelines, which is also the prerequisite for running `heraut release` in CI rather than
locally (Option B). Implement the action before implementing Option B.

---

## Option B — heraut fully owns its own GitHub Release

Currently (Option A, implemented in T33): heraut creates the version, changelog, and tag
locally; GoReleaser creates the GitHub Release in CI.

Option B completes the bootstrap: heraut is configured with `platform: github` and
`heraut release` creates the GitHub Release directly. GoReleaser becomes a pure build
tool (`release: disable: true`) that uploads prebuilt binaries to the release heraut
already created.

**Trigger change required:** instead of the developer pushing a `v*` tag manually, a
`workflow_dispatch` input (or a merge to `main`) triggers the CI run that executes
`heraut release`. The tag is created in CI, not locally.

**Prerequisite:** `adaouat/heraut-action` exists and is stable, so the CI release step
can use `uses: adaouat/heraut-action@v1` rather than a bespoke shell script.

---

## Shared Go library extraction

`internal/port/`, `internal/adapter/exec/`, `internal/testutil/`, and `internal/ui/`
were designed from the start to be extractable into a shared library for other CLIs in
the same family (see roadmap overview, goal 3). Once a second CLI needs them, extract
into `github.com/adaouat/go-release-kit` (or similar) and replace the local copies with
the import.

**Trigger:** a second CLI starts being built and the first copy-paste would happen.
Don't extract preemptively.

---

## GitLab CI / GitHub Actions reusable workflow templates

Ship `.gitlab/heraut-release.yml` and `.github/workflows/heraut-release.yml` as
ready-to-include pipeline templates. Users drop-in the template and set the required
token secret — zero boilerplate to get a heraut-powered release pipeline.

Distinct from `heraut-action` (which is an Action step); these are full pipeline
templates that cover the multi-job pattern (lint + test + release gate).

---

## Multi-instance same-platform releases (e.g. public GitLab + self-hosted GitLab)

Today `release.platforms` allows multiple entries, but two entries of the *same type*
(two GitLab instances, e.g. `gitlab.com` + a private `gitlab.example.com`) don't work:
[ADR-0020](adr/0020-platform-base-url.md) gates `base_url` to the platform-type default,
so a non-default host fails config validation with "self-hosted hosts are not yet
supported".

**What's needed (per ADR-0020's "multi-instance thread")**:
- Lift the `base_url` validator gate for non-default hosts.
- CLI host targeting: `gh`/`glab` need to be pointed at the configured host
  (`GH_HOST`/`GITLAB_HOST`) so publishing actually reaches the right instance, including
  rework of the CI-context auth probe.
- Disambiguate `findPlatformCfg` (currently first-match-by-type) and the reporter's
  `Name()` (currently a bare `"gitlab"`/`"github"`, ambiguous with two instances of the
  same type).

**Trigger:** a user needs to publish the same release to both a public and a private
instance of the same platform type. `base_url` (ADR-0020) is the prerequisite data; this
idea is the consuming capability.
