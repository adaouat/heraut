# Future Ideas

Ideas worth tracking for post-v1.0, not yet scheduled. Each entry records enough context
to evaluate and scope the idea when the time comes.

---

## `adaouat/heraut-action` — GitHub Actions wrapper

A reusable GitHub Action that wraps heraut for use in other projects' CI pipelines, in
the spirit of [`orhun/git-cliff-action`](https://github.com/orhun/git-cliff-action).

**Concept:** A Docker container action backed by `ghcr.io/adaouat/heraut`, which already
bundles everything heraut needs at run time: the binary and `git` (ADR-0016; publishing and
enrichment call the forge APIs directly, no `gh` or `glab`). Other projects would consume it as:

```yaml
- uses: adaouat/heraut-action@v1
  with:
    command: release          # release | changelog | version next | check
    config: .heraut.yml
    env: production
    dry-run: false
    force: false
    version: ''              # --set-version, required for bump: manual
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

heraut's own release workflow does not need the action: it runs the binary it has just built
(ADR-0018).

---

## GitLab CI / GitHub Actions reusable workflow templates

Ship `.gitlab/heraut-release.yml` and `.github/workflows/heraut-release.yml` as
ready-to-include pipeline templates. Users drop-in the template and set the required
token secret — zero boilerplate to get a heraut-powered release pipeline.

Distinct from `heraut-action` (which is an Action step); these are full pipeline
templates that cover the multi-job pattern (lint + test + release gate).

---

## Implemented since

Ideas that used to be listed here and have shipped:

- **heraut owns its own GitHub Release** ("Option B"): the `Release` workflow is a
  `workflow_dispatch`, GoReleaser only builds, and the fresh binary runs `heraut release`
  ([ADR-0018](adr/0018-ci-build-then-release-pipeline.md)).
- **Shared Go library extraction:** the exec runner, config loading, exit codes, UI and update
  check live in `github.com/adaouat/forge`.
- **Multi-instance same-platform releases** (for example `gitlab.com` and a self-hosted GitLab):
  named `forges:` entries with their own `base_url`
  ([ADR-0025](adr/0025-multi-instance-platforms.md), [ADR-0043](adr/0043-forge-abstraction.md)).
