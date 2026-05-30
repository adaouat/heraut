# Spec: CI Build-Then-Release Pipeline

## Objective

Eliminate the full bootstrap dependency in heraut's own CI release pipeline. Today the
release workflow is triggered by a manually-pushed tag and goreleaser creates the GitHub
Release. If a regression lands in `heraut release`, the only recovery path is a manual
escape hatch (raw git + gh CLI). The new pipeline uses only the stable `heraut version
next` from a previously-installed binary, then uses the freshly-compiled binary from the
current commit to run the actual release. A broken release path is fixed by pushing a
code fix and retrying — no manual escape hatch needed.

**Users:** heraut maintainers (single developer, bchatard).

---

## Pipeline Design

### Step 0 — Resolve next version (previous build)

```bash
VERSION=$(heraut version next)
```

- Uses the heraut binary pinned in the CI toolchain (installed via mise)
- Read-only: no git writes, no platform calls, no network
- Bootstrap surface is deliberately minimal: only the version resolver, which is the most
  stable part of the codebase
- If this fails: update the pinned binary manually — acceptable narrow bootstrap risk

### Step 1 — Build all platform binaries (goreleaser, build-only)

```bash
GORELEASER_CURRENT_TAG=$VERSION goreleaser build --clean
```

- Produces `dist/` with all platform binaries, ldflags baked in (`-X main.Version=VERSION`)
- No GitHub Release creation, no tag, no push
- goreleaser handles cross-compilation; heraut does not replace it

### Step 2 — Preflight + release (fresh binary)

```bash
FRESH=./dist/heraut_linux_amd64_v1/heraut
FRESH_VERSION=$($FRESH version next)
[[ "$FRESH_VERSION" != "$VERSION" ]] && echo "version mismatch: $VERSION → $FRESH_VERSION" && exit 1
$FRESH check
$FRESH release --version $VERSION
```

- **Version sanity check:** re-run `heraut version next` with the fresh binary and compare
  to the version resolved in step 0. A mismatch means a commit landed between steps 0 and
  2 that changes the bump — abort cleanly before any git state is written
- `heraut check` validates config + runtime CLIs + cliff config using the fresh binary —
  any failure aborts before any git state is written
- `heraut release --version $VERSION` skips version resolution; runs
  changelog → commit → tag → push → platform publish + asset upload
- The binary doing the release is the binary being released
- If this fails: push a fix, CI rebuilds and retries — no manual intervention

---

## Changes Required

### 1. `--version` flag on `heraut release`

**Behaviour:**
- When `--version v1.2.3` is provided, the version resolution step is skipped entirely
- The value is injected directly into the pipeline config as the pre-resolved version
- All downstream steps (changelog, commit message, tag name, platform publish) use it
- Validate format on entry: must be a valid semver `vMAJOR.MINOR.PATCH`; reject otherwise
  (CalVer format validation is deferred — this pipeline targets semver heraut releases)
- Error early if the tag already exists locally or on remote (prevents re-release)

**Affected code:**
- `internal/cmd/release.go` — declare `--version` flag, pass to `app.BuildPipeline()`
- `internal/app/` — `BuildPipeline()` accepts optional pre-resolved `version string`
- `internal/pipeline/release.go` — skip resolver call when version is pre-set in config

### 2. `release.assets` in `.heraut.yml`

**Config schema addition:**
```yaml
release:
  platforms:
    - github
  assets:
    - "dist/heraut_*_linux_amd64"
    - "dist/heraut_*_linux_arm64"
    - "dist/heraut_*_darwin_amd64"
    - "dist/heraut_*_darwin_arm64"
    - "dist/heraut_*_windows_amd64.exe"
    - "dist/checksums.txt"
```

**Behaviour:**
- Glob patterns expanded at release time via `filepath.Glob` (relative to CWD)
- Expanded paths passed to `gh release create` / `glab release create` as positional args
- Empty `assets` list → no assets uploaded (valid, not an error)
- Glob that matches nothing → warning logged, release continues (consistent with `gh`
  behaviour — the release itself is still valid without a specific asset)
- Glob expansion order: config declaration order, files within a glob sorted lexicographically

**Affected code:**
- `internal/config/config.go` — add `Assets []string` to `ReleaseConfig`
- `schema.json` — add `assets` array field (items: string) under `release`
- `docs/heraut.sample.yml` — show `assets` with a comment
- `internal/platforms/github/` — expand globs, pass as positional args to `gh release create`
- `internal/platforms/gitlab/` — same for `glab release create`

### 3. GitHub Actions workflow redesign

**Trigger change:** from `push: tags: v*` to `workflow_dispatch` (explicit manual trigger).
The Docker workflows remain unchanged — they continue to trigger on `push: tags: v*`,
which heraut pushes in step 2.

**New `release.yml` shape:**
```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: "Override version (e.g. v1.2.3). If empty, heraut version next is used."
        required: false
        default: ""

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      id-token: write
      attestations: write
    steps:
      - checkout (fetch-depth: 0, with push credentials)
      - mise install               # installs goreleaser + pinned heraut
      - heraut version next        # → VERSION env var (skipped if input.version is set)
      - goreleaser build --clean   # GORELEASER_CURRENT_TAG=$VERSION
      - actions/attest (dist/checksums.txt)         # provenance before release
      - fresh version next         # re-run with fresh binary, compare to $VERSION → abort on mismatch
      - heraut check               # fresh binary, full preflight
      - heraut release --version $VERSION           # fresh binary, creates tag + GH release

  docker-build: (unchanged)
  docker-merge: (unchanged)
```

**`.goreleaser.yml` change:** set `release.disable: true` — goreleaser is now build-only.
heraut owns GitHub Release creation via `gh release create`.

**Removed from workflow:**
- `orhun/git-cliff-action` step — heraut generates release notes internally
- `goreleaser/goreleaser-action` with `release` args — replaced by `build --clean`
- `gh release edit --draft=false` step — heraut creates the release as non-draft directly

### 4. `heraut check` as preflight gate

- Runs with the fresh binary before `heraut release`
- Covers: config valid, all external CLIs on PATH (git, git-cliff, gh/glab/cog/communique),
  cliff config renders without error
- Failure → CI job fails, no git state written, clean retry path

---

## Testing Strategy

| Change | Layer | What to assert |
|--------|-------|----------------|
| `--version` bypasses resolver | Unit (pipeline) | resolver `Next()` is never called when version is pre-set |
| `--version` format validation | Unit (cmd/config) | `v1.x.x` accepted; `1.2.3`, `vfoo`, empty rejected |
| Tag pre-existence check | Unit (pipeline) | error returned if tag already exists |
| `release.assets` parsing | Unit (config) | globs loaded; empty list valid; wrong type rejected |
| Glob expansion + gh args | Contract (platforms/github) | `gh release create` receives expanded file paths in order |
| Glob expansion + glab args | Contract (platforms/gitlab) | same for `glab release create` |
| Glob matches nothing | Contract | warning emitted, `gh release create` called without that asset |
| No assets field | Contract | `gh release create` called with no asset args |
| Schema | Schema fixture | valid `.heraut.yml` with `assets` validates; invalid type rejected |

---

## Boundaries

- **Always:** run `heraut check` before `heraut release` in the CI workflow; pin goreleaser
  version in `release.yml`; use `goreleaser build` (never `goreleaser release`) in CI
- **Ask first:** any change to the Docker workflow; changing how goreleaser is invoked;
  adding CalVer validation to `--version`
- **Never:** skip the `heraut check` step; use `--no-verify` or `--force` in the CI
  workflow; let goreleaser create the GitHub Release in the new pipeline

---

## Open Questions

1. **`--version` on `heraut changelog`?** The `changelog` command also runs version
   resolution internally. Should `--version` be a root-level persistent flag, or scoped
   to `release` only? Scoped for now — add to `changelog` only if a concrete need arises.

2. **Glob expansion order:** config declaration order with lexicographic sort within each
   glob — confirm this is acceptable or specify a different sort.

3. **`workflow_dispatch` version input:** confirmed — workflow exposes an optional `version`
   input. When set, it bypasses `heraut version next` in step 0 and is used directly as
   `VERSION`. The fresh-binary version sanity check in step 2 is skipped when the version
   was manually overridden (the human made an explicit choice; a mismatch is expected).

4. **Attestation timing:** attest after `goreleaser build`, before `heraut release` — this
   means provenance is recorded even if the release step fails. Preferred? Or attest only
   on successful release?

---

## Success Criteria

- [ ] `heraut release --version v1.2.3` completes without calling the version resolver
- [ ] Invalid `--version` values are rejected before any pipeline step runs
- [ ] Assets in `release.assets` are passed to `gh release create` (verified by contract test)
- [ ] A glob matching no files emits a warning but does not fail the release
- [ ] CI workflow triggers on `workflow_dispatch`, runs all three steps end-to-end
- [ ] Version mismatch between step-0 and fresh binary aborts CI before any git state is modified
- [ ] Version mismatch check is skipped when `workflow_dispatch` version input is set
- [ ] `heraut check` failure aborts CI before any git state is modified
- [ ] goreleaser is invoked as `build --clean` only in the new workflow; `release.disable: true` in `.goreleaser.yml`
- [ ] Docker workflows continue to trigger correctly on the tag heraut pushes
