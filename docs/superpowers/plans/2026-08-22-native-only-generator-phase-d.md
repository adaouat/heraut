# Native-Only Generator Epic — Phase D (Infra Housekeeping) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the native-only-generator epic by removing the last `git-cliff`/`communique`
traces from heraut's build/runtime infrastructure — the Docker image, the mise dev-toolchain
pin, and ADR-0016's bundled-tool inventory — closing out the epic's final phase.

**Architecture:** Three independent, single-file edits (`Dockerfile`, `.config/mise/config.toml`
+ its generated lockfile, `docs/adr/0016-bundled-docker-image.md`) followed by one integration
task that sweeps for stragglers, runs the full verification suite, does a real `docker build` +
smoke test, and closes out the roadmap phase. No Go code changes — this phase is pure
infra/docs, so "tests" are hadolint, `hk check`, `go test ./...` (regression guard — no
production Go code is touched, so this should be a no-op pass), and a real Docker build +
`heraut --version` + `gh`/`glab`/`git` presence smoke test, in that order per task.

**Tech Stack:** Docker (`docker build`, `docker/dockerfile:1` syntax), mise (`.config/mise/`),
hadolint (Dockerfile linting, wired into `hk check`), Markdown (ADR).

**Spec:** `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §5 ("Infra
housekeeping (Phase D)", ~line 163) — the complete, unambiguous scope for this phase. No
separate design doc was written; this plan implements §5 directly.

## Global Constraints

- Pre-v1.0: commits land directly on `main`, no branches (`.claude/rules/workflow.md`).
- Conventional commits required; this phase's changes are `chore` (Dockerfile ARGs, mise pin)
  and `docs` (ADR-0016) by type — see each task for the exact type/scope.
- Never bypass `hk` hooks (`--no-verify`, etc.) — fix root causes instead
  (`.claude/rules/workflow.md`).
- Two-step roadmap flow: each task's commit bundles its implementation change with its own
  `docs/tasks/native-generator-roadmap.md` checkbox flip + completion note
  (`.claude/rules/workflow.md`).
- **Do not touch the `HERAUT_VERSION`/ldflags mechanism** — CLAUDE.md's "ldflags invariant"
  requires `Dockerfile` and `.goreleaser.yml` to stay byte-identical there; this phase's scope
  never needs to change it.
- `gh`/`glab` stay in the image (ADR-0044, unrelated to this epic) — only `git-cliff` and
  `communique` are being removed.
- Historical ADRs normally stay as-is; ADR-0016's bundled-CLI table is the one documented
  living-inventory exception (ADR-0028 precedent) — its table and the directly-adjacent prose
  that describes *why* the base image is what it is are both fair game for this phase, per the
  design doc's own line-number call-outs.
- Never delete a load-bearing test row without a documenting ADR (`.claude/rules/testing.md`)
  — not directly relevant here (no test rows touched), but keep it in mind if the sweep task
  (T208) finds a stray test asserting `git-cliff`/`communique` presence in the image.

---

## File Structure

| File | Change |
|---|---|
| `Dockerfile` | Drop `GIT_CLIFF_VERSION`/`COMMUNIQUE_VERSION` ARGs, their `mise use -g` entries, and their `cp` steps. Rewrite the stage-3 base-image comment (verified finding: only `communique` was dynamically linked against glibc; `git-cliff`, `gh`, `glab` are all static). |
| `.config/mise/config.toml` | Drop the `git-cliff = "2.13"` dev-toolchain pin. |
| `.config/mise/mise.lock` | Drop the now-orphaned `[[tools.git-cliff]]` block (generated file — see T206 for why this is a manual, scoped edit rather than a `mise lock` re-run). |
| `CLAUDE.md` | One-line fix: "Go, golangci-lint, goreleaser, and git-cliff are installed via mise" is no longer true once T206 lands. |
| `docs/adr/0016-bundled-docker-image.md` | Drop `git-cliff`/`communique` from the tool-orchestration list, the bundled-tool table, and the base-image-choice rationale (rewritten with the verified static/dynamic finding); update the "Upgrading git-cliff…" and "bundling five CLIs…" Consequences lines to match the current two-tool (`gh`, `glab`) reality, citing a real measured image-size delta. |
| `docs/tasks/native-generator-roadmap.md` | New "Phase D" section (T205–T208), a new row in the "Progress at a glance" table, and (via T208) a new filed-not-implemented follow-up task for a discovered stale `git-cliff` comment in `.github/workflows/release.yml` (out of scope for this phase — CI/CD files need explicit user confirmation to touch, per `.claude/rules/claude.md`). |

---

## Verified findings (carry these into the task steps below — do not re-derive)

Gathered via a real `docker build --target tools` of the current `Dockerfile` plus `file` on
the extracted binaries, and a full before/after image build:

- **`git-cliff`**: `ELF … statically linked, stripped` (musl static build — its own GitHub
  release asset is literally named `git-cliff-2.13.1-aarch64-unknown-linux-musl.tar.gz`).
- **`glab`**: `ELF … statically linked` (Go binary, `CGO_ENABLED=0`-style static build).
- **`gh`**: `ELF … statically linked` (same).
- **`communique`**: `ELF … dynamically linked, interpreter /lib/ld-linux-aarch64.so.1 … for
  GNU/Linux` — the **only** one of the four with a real glibc dependency.
- **Conclusion**: `git-cliff` never drove the `debian:trixie-slim`-over-`alpine` choice;
  `communique` was the sole reason. The actual remaining reason to stay off Alpine is that
  `git`/`ca-certificates` are installed via `apt` (Debian's package manager, glibc-based) in
  the final stage — switching to Alpine would mean swapping to `apk`, a larger change than
  this phase's scope. Do **not** attempt that swap in this phase; T208 files it as a
  discovered-but-deferred idea if you want it noted (optional — use judgment, it's speculative,
  not "broken").
- **Image size**: local `arm64` build, current `Dockerfile` (baseline): **334MB**. Same build
  with T205's Dockerfile changes applied: **307MB** (`docker images … --format "{{.Size}}"`,
  uncompressed layer size). A real, modest (~8%) reduction — cite this exact pair of numbers in
  T207's ADR update, not a fabricated estimate.
- `mise lock` (re-run from scratch) correctly drops `git-cliff` from `mise.lock` when it's
  removed from `config.toml` — **but** on this machine's locally installed mise version
  (`2026.8.8`, newer than whatever generated the committed lockfile), it also rewrote ~63
  unrelated lines across every other pinned tool (adding `url_api` fields the committed
  lockfile doesn't have for some entries). That's out-of-scope churn. T206 instructs a manual,
  scoped block removal instead, verified equivalent for the one entry that actually changes.

---

## Task 1 — T205: Dockerfile

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Confirm T205 is `[ ]` in `docs/tasks/native-generator-roadmap.md`**

It won't exist yet — this plan's own Task 4 (T208's roadmap-registration prerequisite) adds
the Phase D section with all four tasks marked `[ ]` before any implementation starts. If
you're executing this plan via `subagent-driven-development`, the roadmap section is added as
part of kicking off the phase (see "Roadmap registration" note at the end of this plan) —
confirm the T205 entry exists and is `[ ]` before proceeding.

- [ ] **Step 2: Record the current hadolint + build baseline**

```bash
hadolint Dockerfile
```

Expected: no output (exit 0) — same as after the change, so this isn't a red/green test in the
usual sense, but running it now catches any pre-existing lint drift before you start.

- [ ] **Step 3: Edit the ARGs block**

Current (top of file):

```dockerfile
# syntax=docker/dockerfile:1

# Tool versions — bump here to upgrade across all stages.
ARG GO_VERSION=1.26
ARG HERAUT_VERSION=dev
ARG MISE_VERSION=2026.5.15
ARG GIT_CLIFF_VERSION=2.13.1
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0
ARG COMMUNIQUE_VERSION=1.1.3
```

Replace with:

```dockerfile
# syntax=docker/dockerfile:1

# Tool versions — bump here to upgrade across all stages.
ARG GO_VERSION=1.26
ARG HERAUT_VERSION=dev
ARG MISE_VERSION=2026.5.15
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0
```

- [ ] **Step 4: Edit the tools stage**

Current:

```dockerfile
# ── Stage 2: install external tools via mise ─────────────────────────────────
FROM ghcr.io/jdx/mise:${MISE_VERSION} AS tools

ARG GIT_CLIFF_VERSION
ARG GLAB_VERSION
ARG GH_VERSION
ARG COMMUNIQUE_VERSION

# mise use -g installs each tool globally and makes it active.
# mise which resolves the real binary path (not the shim) for clean extraction.
RUN mise use -g \
        git-cliff@${GIT_CLIFF_VERSION} \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
        communique@${COMMUNIQUE_VERSION} \
    && mkdir /tools \
    && cp "$(mise which git-cliff)"  /tools/git-cliff \
    && cp "$(mise which glab)"       /tools/glab \
    && cp "$(mise which gh)"         /tools/gh \
    && cp "$(mise which communique)" /tools/communique
```

Replace with:

```dockerfile
# ── Stage 2: install external tools via mise ─────────────────────────────────
FROM ghcr.io/jdx/mise:${MISE_VERSION} AS tools

ARG GLAB_VERSION
ARG GH_VERSION

# mise use -g installs each tool globally and makes it active.
# mise which resolves the real binary path (not the shim) for clean extraction.
RUN mise use -g \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
    && mkdir /tools \
    && cp "$(mise which glab)" /tools/glab \
    && cp "$(mise which gh)"   /tools/gh
```

- [ ] **Step 5: Rewrite the stage-3 base-image comment**

Current:

```dockerfile
# ── Stage 3: final image ──────────────────────────────────────────────────────
# trixie matches the mise tools stage (Debian 13, glibc 2.40+), ensuring
# dynamically-linked binaries like communique resolve their glibc requirements.
FROM debian:trixie-slim
```

Replace with (per the verified findings above — do not re-derive, this was confirmed by
building the tools stage and running `file` on the extracted binaries):

```dockerfile
# ── Stage 3: final image ──────────────────────────────────────────────────────
# debian:trixie-slim over alpine: `git` and `ca-certificates` come from Debian's `apt`
# package index, which requires a glibc-based distro. The bundled CLI binaries impose
# no such requirement of their own — `gh` and `glab` are statically-linked Go binaries
# (verified via `file` on the tools stage's output). `golang:trixie` (the builder stage)
# is kept consistent with this base to avoid glibc version surprises.
FROM debian:trixie-slim
```

- [ ] **Step 6: Verify hadolint still passes**

```bash
hadolint Dockerfile
```

Expected: no output (exit 0).

- [ ] **Step 7: Build the tools stage and confirm only `glab`/`gh` are present**

```bash
docker build --target tools -t heraut-t205-tools-check -f Dockerfile .
docker run --rm --entrypoint /bin/sh heraut-t205-tools-check:latest -c 'ls /tools'
```

Expected: `gh` and `glab` only — no `git-cliff`, no `communique`.

- [ ] **Step 8: Build the full image and smoke-test it**

```bash
docker build -t heraut-t205-check -f Dockerfile .
docker run --rm heraut-t205-check --version
docker run --rm --entrypoint /bin/sh heraut-t205-check -c 'gh --version | head -1; glab --version | head -1; git --version'
```

Expected: the version banner prints, and `gh`/`glab`/`git` all report their versions with no
errors.

- [ ] **Step 9: Clean up the local diagnostic images**

```bash
docker rmi heraut-t205-tools-check heraut-t205-check
```

- [ ] **Step 10: Flip T205 to done in the roadmap and add a completion note**

In `docs/tasks/native-generator-roadmap.md`, change `#### `[ ]` T205: Dockerfile — drop
git-cliff/communique` to `#### `[x]` T205: …` and add a completion note underneath following
the file's existing style (see any `[x]` task above it for the format), covering: what was
dropped, the base-image comment rewrite and why (cite the `file` findings), and that a full
`docker build` + smoke test passed locally.

- [ ] **Step 11: Commit**

```bash
git add Dockerfile docs/tasks/native-generator-roadmap.md
git commit -m "chore(docker): drop git-cliff/communique from the bundled image

git-cliff and communique are fully removed as of ADR-0045 (native is heraut's sole
generator); nothing in the pipeline still shells out to either, so bundling them in
the release-runner image serves no purpose. gh/glab stay (ADR-0044, publishing).

Verified via a real docker build + file(1) on the tools-stage output that only
communique was dynamically linked against glibc — git-cliff, gh, and glab are all
static binaries. The stage-3 base-image comment is rewritten to reflect that: the
remaining reason to stay on debian:trixie-slim over alpine is apt-installed git/
ca-certificates, not the bundled CLI tools themselves."
```

---

## Task 2 — T206: `.config/mise/config.toml` + `mise.lock` + `CLAUDE.md`

**Files:**
- Modify: `.config/mise/config.toml`
- Modify: `.config/mise/mise.lock`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Confirm T206 is `[ ]` in `docs/tasks/native-generator-roadmap.md`**

- [ ] **Step 2: Drop the `git-cliff` pin from `config.toml`**

Current `[tools]` block:

```toml
[tools]
actionlint = "latest"
git-cliff = "2.13"
go = "1.26"
"go:golang.org/x/tools/gopls" = "latest"
"go:golang.org/x/vuln/cmd/govulncheck" = "latest"
golangci-lint = "2.12"
goreleaser = "2.16"
hadolint = "latest"
hk = "1.46"
pkl = "latest"
tombi = "latest"
typos = "latest"
yamlfmt = "latest"
```

Replace with (just the one line removed, everything else — including alphabetical order —
untouched):

```toml
[tools]
actionlint = "latest"
go = "1.26"
"go:golang.org/x/tools/gopls" = "latest"
"go:golang.org/x/vuln/cmd/govulncheck" = "latest"
golangci-lint = "2.12"
goreleaser = "2.16"
hadolint = "latest"
hk = "1.46"
pkl = "latest"
tombi = "latest"
typos = "latest"
yamlfmt = "latest"
```

- [ ] **Step 3: Remove the orphaned `git-cliff` block from `mise.lock`**

`mise.lock` is `# @generated`, but do **not** regenerate it with a bare `mise lock` run — that
command updates every entry it touches, and on a mise version newer than whatever generated
the committed file, it rewrites unrelated `url_api` metadata across every other pinned tool
(confirmed: ~63 unrelated added/changed lines locally, none of them meaningful). Manually
remove just the orphaned block instead — its content doesn't change when regenerated, only
its *presence* does.

Current (the block sits between the `actionlint` and `go` entries):

```toml

[[tools.git-cliff]]
version = "2.13.1"
backend = "aqua:orhun/git-cliff"

[tools.git-cliff."platforms.linux-arm64"]
checksum = "sha256:4054c124b926c117f3fa048939bc8be0a954f29f3b6f367627e8cb22c1971882"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-aarch64-unknown-linux-musl.tar.gz"

[tools.git-cliff."platforms.linux-arm64-musl"]
checksum = "sha256:4054c124b926c117f3fa048939bc8be0a954f29f3b6f367627e8cb22c1971882"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-aarch64-unknown-linux-musl.tar.gz"

[tools.git-cliff."platforms.linux-x64"]
checksum = "sha256:200d2535da6d9703f3bcc8a4d159c3b55eacdb01cf2148c55b3eee9dd04d5249"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-x86_64-unknown-linux-musl.tar.gz"

[tools.git-cliff."platforms.linux-x64-musl"]
checksum = "sha256:200d2535da6d9703f3bcc8a4d159c3b55eacdb01cf2148c55b3eee9dd04d5249"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-x86_64-unknown-linux-musl.tar.gz"

[tools.git-cliff."platforms.macos-arm64"]
checksum = "sha256:21547ae4a0421164070ab75c2522864ea5565858a011fabc5f583061b20f1226"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-aarch64-apple-darwin.tar.gz"

[tools.git-cliff."platforms.macos-x64"]
checksum = "sha256:6e60ae390d375cecb9d8008c49f0e724a8dfe40390b532ef5501e421d2cc8acb"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-x86_64-apple-darwin.tar.gz"

[tools.git-cliff."platforms.windows-x64"]
checksum = "sha256:3ae3a5549e85c7ad5b20192ebcfee4371269deca51255f6f2f2e051c6541f5ca"
url = "https://github.com/orhun/git-cliff/releases/download/v2.13.1/git-cliff-2.13.1-x86_64-pc-windows-msvc.zip"

[[tools.go]]
```

Replace with (the block deleted, exactly one blank line preserved before `[[tools.go]]`,
matching the file's existing spacing convention between entries):

```toml

[[tools.go]]
```

- [ ] **Step 4: Fix the now-stale `CLAUDE.md` line**

`CLAUDE.md`'s "Tooling (mise)" section (~line 122) reads:

```markdown
Go, golangci-lint, goreleaser, and git-cliff are installed via mise (see
`.config/mise/config.toml`).
```

This becomes false the moment Step 2 lands — it's a direct, same-fact consequence of this
task's own change (not new scope; same precedent as T189's "directly-adjacent, actively-broken
reference" fix earlier in this epic). Replace with:

```markdown
Go, golangci-lint, and goreleaser are installed via mise (see
`.config/mise/config.toml`).
```

- [ ] **Step 5: Verify no repo-tracked references to the removed pin remain in these three files**

```bash
grep -n "git-cliff" .config/mise/config.toml .config/mise/mise.lock CLAUDE.md
```

Expected: no output (exit 1 / no matches).

- [ ] **Step 6: Sanity-check the lockfile still resolves for the tools that remain**

```bash
mise install --locked
```

Expected: no error mentioning `git-cliff`. (If this reports errors about unrelated tools not
present in this repo's `config.toml` — e.g. anything from your own global/personal mise
config outside this repo — that's environment noise, not a regression; only errors naming a
tool actually listed in `.config/mise/config.toml` matter here. If `mise` isn't available in
your execution environment at all, skip this step — Steps 2–5 are the load-bearing checks.)

- [ ] **Step 7: Flip T206 to done in the roadmap and add a completion note**

Cover: the pin removal, the manual (not `mise lock`-regenerated) lockfile edit and why
(cite the unrelated-churn finding), and the `CLAUDE.md` line fix.

- [ ] **Step 8: Commit**

```bash
git add .config/mise/config.toml .config/mise/mise.lock CLAUDE.md docs/tasks/native-generator-roadmap.md
git commit -m "chore(mise): drop the git-cliff dev-toolchain pin

Nothing in the dev toolchain or CI needs git-cliff installed anymore — native has
been heraut's sole generator since ADR-0045. mise.lock's orphaned git-cliff block is
removed by hand rather than via a fresh \`mise lock\` run, which would have rewritten
unrelated url_api metadata across every other pinned tool on a newer local mise
version. Also fixes CLAUDE.md's now-false 'git-cliff is installed via mise' line."
```

---

## Task 3 — T207: `docs/adr/0016-bundled-docker-image.md`

**Files:**
- Modify: `docs/adr/0016-bundled-docker-image.md`

**Depends on:** T205 (cites its verified `file`/size findings — already recorded above, no
need to wait for T205's commit to land first if running tasks in parallel, but do read T205's
"Verified findings" section above before writing this task's prose).

- [ ] **Step 1: Confirm T207 is `[ ]` in `docs/tasks/native-generator-roadmap.md`**

- [ ] **Step 2: Update the Context section's tool-orchestration list**

Current:

```markdown
ADR-0013 established raw binaries as the primary distribution format. GoReleaser was
also configured to push a minimal Alpine container image to GHCR — just `ca-certificates`
and the heraut binary. That image cannot actually run a release: heraut orchestrates
`git`, `git-cliff`, `gh`, `glab`, and `communique`, none of which are present.
```

Replace with:

```markdown
ADR-0013 established raw binaries as the primary distribution format. GoReleaser was
also configured to push a minimal Alpine container image to GHCR — just `ca-certificates`
and the heraut binary. That image cannot actually run a release: heraut orchestrates
`git`, `gh`, and `glab`, none of which are present.
```

- [ ] **Step 3: Update the Decision bullet list's base-image line**

Current:

```markdown
- Uses `debian:trixie-slim` as the final base to satisfy the glibc requirements of
  dynamically-linked tools (`gh`, `glab`, `communique`).
```

Replace with:

```markdown
- Uses `debian:trixie-slim` as the final base — `git`/`ca-certificates` come from `apt`,
  which requires a glibc-based distro (see "Base image choice" below).
```

- [ ] **Step 4: Update the bundled tool versions table**

Current:

```markdown
### Bundled tool versions (pinned as Dockerfile ARGs)

| Tool          | Version     | Source                        |
|---------------|-------------|-------------------------------|
| git-cliff     | `2.13.1`    | `.config/mise/config.toml`    |
| glab          | `1.99.0`    | chosen for image              |
| gh            | `2.92.0`    | chosen for image              |
| communique    | `1.1.3`     | chosen for image              |

`git` and `ca-certificates` are installed via `apt` from the Debian package index (no
version pin; they track the Debian stable release for the chosen base).
```

Replace with:

```markdown
### Bundled tool versions (pinned as Dockerfile ARGs)

| Tool          | Version     | Source                        |
|---------------|-------------|-------------------------------|
| glab          | `1.99.0`    | chosen for image              |
| gh            | `2.92.0`    | chosen for image              |

`git` and `ca-certificates` are installed via `apt` from the Debian package index (no
version pin; they track the Debian stable release for the chosen base).
```

- [ ] **Step 5: Rewrite the "Base image choice" section**

Current:

```markdown
### Base image choice: debian:trixie-slim over alpine

`gh`, `glab`, and `communique` ship dynamically-linked glibc binaries; they do not
provide musl builds. Alpine (musl libc) would require either static builds (not
available for all tools) or a compatibility shim (added complexity, fragile). Debian
trixie-slim (glibc 2.40+) is the simplest compatible base. The `golang:trixie` builder
stage is kept consistent with the final image to avoid glibc version surprises.
```

Replace with (this is the corrected, verified answer to "did git-cliff also drive this
choice?" — record it here so the question never needs re-asking):

```markdown
### Base image choice: debian:trixie-slim over alpine

`git` and `ca-certificates` come from Debian's `apt` package index, which requires a
glibc-based distro — moving to Alpine (musl libc) would mean replacing `apt` with `apk`,
a larger change than this ADR's scope. The bundled CLI binaries impose no glibc
requirement of their own: `gh` and `glab` are statically-linked Go binaries, confirmed by
running `file` against the tools stage's output. `communique`, removed along with
`git-cliff` in [ADR-0045](0045-native-sole-generator.md), was the one dynamically-linked,
glibc-only binary in the original lineup — `git-cliff` was a static musl build, so it
never actually drove this choice. The `golang:trixie` builder stage is kept consistent
with this base to avoid glibc version surprises.
```

- [ ] **Step 6: Update the Consequences section's "Version isolation" example**

Current:

```markdown
**Version isolation.**  
Tool versions are pinned as Dockerfile ARGs. Upgrading git-cliff means bumping one
line; the change is reviewable and trackable in git history.
```

Replace with:

```markdown
**Version isolation.**  
Tool versions are pinned as Dockerfile ARGs. Upgrading gh or glab means bumping one
line; the change is reviewable and trackable in git history.
```

- [ ] **Step 7: Update the Consequences section's "Image size" paragraph**

Current:

```markdown
**Image size.**  
The image is larger than the previous thin image. This is intentional and unavoidable —
bundling five CLIs costs ~200–400 MB depending on compression. Users who only want the
binary use the raw binary (ADR-0013).
```

Replace with (citing the real measured numbers from T205 — do not estimate, these are
measured):

```markdown
**Image size.**  
The image is larger than the previous thin image. Dropping `git-cliff` and `communique`
(ADR-0045) shrank it somewhat: a local `arm64` build went from 334MB to 307MB (`docker
images`, uncompressed layer size) after this cleanup. The remaining weight is the Debian
base plus `gh`/`glab` themselves, both kept for publishing (ADR-0044). Users who only
want the binary use the raw binary (ADR-0013).
```

- [ ] **Step 8: Confirm no other `git-cliff`/`communique` mentions remain in the file**

```bash
grep -n "git-cliff\|communique" docs/adr/0016-bundled-docker-image.md
```

Expected: no output.

- [ ] **Step 9: Run `hk check` against the file**

```bash
hk check --all docs/adr/0016-bundled-docker-image.md
```

Expected: clean (Markdown/prose linters only; no hadolint applies to this file).

- [ ] **Step 10: Flip T207 to done in the roadmap and add a completion note**

- [ ] **Step 11: Commit**

```bash
git add docs/adr/0016-bundled-docker-image.md docs/tasks/native-generator-roadmap.md
git commit -m "docs(adr): drop git-cliff/communique from ADR-0016's bundled-CLI inventory

ADR-0016's bundled-CLI table is a documented living-inventory exception (ADR-0028
precedent), updated here to match the Docker image after T205. Also corrects the
base-image-choice rationale: a real docker build + file(1) check on the tools stage
shows only communique was ever dynamically linked against glibc — git-cliff, gh, and
glab are all static binaries. The real reason to stay off Alpine is apt-installed
git/ca-certificates, not the bundled CLI tools. Image-size consequence updated with
a measured before/after (334MB -> 307MB), not an estimate."
```

---

## Task 4 — T208: Final sweep, verification, and phase close-out

**Files:**
- Modify: `docs/tasks/native-generator-roadmap.md`
- Read-only check: repo-wide grep, `go test ./...`, `hk check`, `docker build`

**Depends on:** T205, T206, T207 all landed.

- [ ] **Step 1: Confirm T208 is `[ ]` in `docs/tasks/native-generator-roadmap.md` and T205–T207 are `[x]`**

- [ ] **Step 2: Repo-wide sweep for anything Phase D should have caught but didn't**

```bash
grep -rniE "git-cliff|git_cliff|communique" --include="*" . \
  | grep -v "\.git/" \
  | grep -v "docs/adr/0010-embedded-cliff-toml-default.md" \
  | grep -v "docs/adr/0028-" \
  | grep -v "docs/adr/0045-" \
  | grep -v "docs/tasks/native-generator-roadmap.md" \
  | grep -v "docs/superpowers/specs/2026-08-08-native-only-generator-design.md" \
  | grep -v "docs/superpowers/plans/2026-08-22-native-only-generator-phase-d.md" \
  | grep -v "CHANGELOG.md" \
  | grep -v "\.claude/plans/"
```

The excluded paths are: historical ADRs that correctly still mention these tools in past
tense (expected, not a finding), this epic's own roadmap/spec/plan docs (expected — they
describe the removal itself), the auto-generated changelog (historical record, never edited
by hand), and this repo's `.claude/plans/` scratch directory (old exploratory planning docs
predating this epic's current shape — not shipped documentation, out of scope for a cleanup
sweep).

Expected remaining hits after those exclusions, both **out of scope for this phase** — confirm
they're still exactly these two and do not fix them here:

- `schema.json` (one line, a JSON Schema `description` field comparing heraut's
  `azure_devops` owner-path shape to git-cliff's own — accurate historical documentation of a
  naming-convention choice, not a claim that git-cliff is live).
- `.github/workflows/release.yml` (two comments, ~lines 86 and 96, explaining why
  `GITHUB_TOKEN` is set for git-cliff's PR-metadata API auth — now stale, since native's own
  enrichment handles this and never shells out to git-cliff. This is a CI/CD workflow file;
  `.claude/rules/claude.md` requires explicit user confirmation before modifying CI/CD
  pipelines, and it's outside this phase's `docs/superpowers/specs/2026-08-08-…§5` scope. File
  it as a new task instead of fixing it — see Step 5.)

If the sweep surfaces anything else unexpected, stop and investigate before continuing —
don't fold an unplanned fix into this task silently.

- [ ] **Step 3: Full regression check**

```bash
go build ./...
go test ./...
hk check
```

Expected: all green. No production Go code was touched by T205–T207, so this is a pure
regression guard, not expected to find anything — but per this project's "Verify, Don't
Assume" standard, run it rather than assume it.

- [ ] **Step 4: Final full Docker build + smoke test**

```bash
docker build -t heraut-phase-d-final -f Dockerfile .
docker run --rm heraut-phase-d-final --version
docker run --rm --entrypoint /bin/sh heraut-phase-d-final -c 'gh --version | head -1 && glab --version | head -1 && git --version && ls /usr/local/bin'
docker rmi heraut-phase-d-final
```

Expected: version banner prints; `gh`/`glab`/`git` all report versions; `/usr/local/bin`
contains exactly `heraut`, `gh`, `glab` (no `git-cliff`, no `communique`).

- [ ] **Step 5: File the discovered `release.yml` staleness as a new, unimplemented follow-up task**

In `docs/tasks/native-generator-roadmap.md`'s "Follow-ups" section (after T204, same section
T203/T204 live in), add:

```markdown
#### `[ ]` T209: stale git-cliff comments in `.github/workflows/release.yml`

Two comments (~lines 86, 96) explain why `GITHUB_TOKEN` is set for "git-cliff's PR-metadata
API auth" — stale since native (heraut's sole generator as of ADR-0045) never shells out to
git-cliff; its own enrichment (`internal/generators/native/enrich_github.go`) reads the same
token directly. `GITHUB_TOKEN` itself likely still needs to stay set (native's enrichment
almost certainly still wants it) — this is a comment-accuracy fix, not a behavior change, but
touching `.github/workflows/release.yml` needs explicit user confirmation per
`.claude/rules/claude.md`'s CI/CD-modification rule, so it wasn't done inline during Phase D
(discovered by T208's repo-wide sweep, out of that task's own scope). **Scope:** S.
```

- [ ] **Step 6: Update the "Progress at a glance" table**

Change:

```markdown
| Phase 3 — raw-HTTP clients (drop `gh` / `glab`)       | —                      | Deferred    |
```

to (inserting the new row above it):

```markdown
| Phase D — infra housekeeping (Dockerfile / mise / ADR-0016) | T205–T208       | Done        |
| Phase 3 — raw-HTTP clients (drop `gh` / `glab`)       | —                      | Deferred    |
```

- [ ] **Step 7: Add the "Phase D" section heading and closing summary**

Insert a new `## Phase D — Infra housekeeping (Dockerfile / mise / ADR-0016)` section between
the end of the "Follow-ups" section (after T209, added in Step 5) and the
`## Phase 3 — Raw-HTTP platform clients (deferred)` heading, following this exact shape (the
per-task `#### `[x]` T205: …` etc. entries with their completion notes were already added by
each task's own commit — T208 adds only the section's opening blockquote and, at the very end,
after all four tasks, this closing summary paragraph):

```markdown
## Phase D — Infra housekeeping (Dockerfile / mise / ADR-0016)

> See `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §5 ("Infra
> housekeeping (Phase D)") for the full scope — already a complete, unambiguous spec, so no
> separate design doc was written. Plan:
> `docs/superpowers/plans/2026-08-22-native-only-generator-phase-d.md`.

[T205–T208 task entries land here, one per task, each added by that task's own commit]

---

**Phase D is done — this closes out the native-only-generator epic's last remaining phase.**
All 4 tasks (T205–T208) landed on `main`. `git-cliff` and `communique` are now gone from
every layer: no package (Phase A/B), no wizard option (Phase C), no config field (Phase A),
and now no Docker image bundling and no dev-toolchain pin (Phase D). A real `docker build`
verified the final image works end-to-end with just `heraut`, `gh`, and `glab` — down from
334MB to 307MB locally. One discovered-but-deferred item was filed rather than fixed inline:
T209, a stale git-cliff comment pair in `.github/workflows/release.yml` (CI/CD file, needs
explicit user confirmation to touch, outside this phase's scope). The only remaining item in
this epic's roadmap is Phase 3 (raw-HTTP platform clients, replacing `gh`/`glab` — explicitly
deferred behind its own future ADR, not scheduled).
```

- [ ] **Step 8: Commit**

```bash
git add docs/tasks/native-generator-roadmap.md
git commit -m "docs(roadmap): close out Phase D and the native-only-generator epic

Repo-wide sweep after T205-T207 found nothing unaccounted for beyond two already-
known-acceptable historical mentions and one genuinely stale CI comment pair
(filed as T209, not fixed inline — touching .github/workflows/ needs explicit
confirmation and is outside Phase D's scope). Full go build/test/hk check green;
a final real docker build + gh/glab/git smoke test passed. Phase D marked Done;
only Phase 3 (deferred, no ADR yet) remains on this epic's roadmap."
```

---

## Roadmap registration (do this before Task 1)

Before dispatching Task 1, add the Phase D section skeleton (heading + blockquote + four `[ ]`
task stub entries, no completion notes yet) to `docs/tasks/native-generator-roadmap.md` and the
Progress-at-a-glance row (Status: `Active`), so each task has something to flip from `[ ]` to
`[x]`. Use the same task-entry format as any existing `[x]` entry in the file (heading, one
paragraph of scope description, **Files:**/**Scope:**/**Dependencies:** lines) — the full
per-task descriptions are already written out in Tasks 1–4 above; use those as the stub text.

---

## Self-review notes (for whoever executes this plan)

- **Spec coverage:** all three of design-doc §5's literal bullets are covered (Dockerfile ARGs/
  install/cp removal + comment update + glibc investigation → T205; mise pin → T206; ADR-0016
  table + prose → T207). The "check whether git-cliff also drove the glibc choice" question is
  answered with real evidence (Verified findings section) rather than left as an open TODO.
- **No placeholders:** every edit step above shows exact current text and exact replacement
  text, copied from the real files as read during planning (2026-08-22) — nothing is described
  without being shown.
- **Scope discipline:** two adjacent staleness items were found during planning
  (`CLAUDE.md`'s mise-tooling line, `.github/workflows/release.yml`'s git-cliff comments). The
  first is folded into T206 as a direct, same-fact consequence of that task's own edit
  (mirroring T189's established precedent in this exact epic). The second is deliberately
  **not** fixed — CI/CD files need explicit user confirmation per this repo's rules, and it's
  outside §5's scope — so T208 files it as new task T209 instead of implementing it, per
  `.claude/rules/coding.md`'s "don't expand scope" rule.
