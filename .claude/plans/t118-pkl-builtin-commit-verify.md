# T118: Publish a Pkl Builtin for `heraut commit verify` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `heraut commit verify`'s hk hook definition as a real, versioned Pkl package attached to heraut's own GitHub releases, so sibling repos (`bifrost`, `forge`, `hermes`) can later import it the same way they already import hk's own `Builtins.pkl`.

**Architecture:** A new top-level `pkl/` directory holds exactly two source files — `PklProject` (package metadata) and `Builtins.pkl` (the published `commit_verify` Step) — plus a generated, checked-in lockfile (`PklProject.deps.json`). `pkl/Builtins.pkl` depends on hk's own published `Config.pkl` (pinned to v1.46.0, the same version heraut's own `.config/hk/config.pkl` already imports) to construct a real `Config.Step` instance — **this is a hard requirement, verified empirically below**: a plain untyped Pkl object (`new { check = "..." }`) is rejected wherever a consumer's `Mapping<String, Config.Step>` expects a typed `Step`. `.github/workflows/release.yml` gets one new step that rewrites `pkl/PklProject`'s `version` field to the resolved release version and runs `pkl project package`, writing two new files into `dist/`; `.config/heraut.yml`'s `release.assets` gains one glob so the *existing*, already-contract-tested `heraut release` asset-upload path picks them up. No new heraut application code is written.

**Tech Stack:** Pkl 0.31 (`pkl` CLI, already on `PATH` via mise), Go 1.26 (one smoke test only), GitHub Actions, GoReleaser (unchanged).

## Verified Pkl mechanics (do not re-derive — these were confirmed by hands-on spike against the real `pkl` 0.31.1 binary before this plan was written)

- `pkl project package <dir> --output-path <out> --skip-publish-check` packages **only** the files inside `<dir>` (confirmed: files placed *outside* the target directory, including in its parent, are never included) — this validates the design's choice of a dedicated `pkl/` directory rather than repo root.
- `pkl project package` writes exactly four files per package: `<name>@<version>` (dependency metadata, JSON, no extension), `<name>@<version>.sha256`, `<name>@<version>.zip` (the package archive), `<name>@<version>.zip.sha256`.
- `package.baseUri` and `package.packageZipUrl` are **both required** fields (no defaults).
- `baseUri`'s final path segment must be the bare package **name**, with **no** `@version` suffix — `pkl project package` appends `@\(version)` itself when computing the canonical `packageUri`. Writing `@\(version)` into `baseUri` yourself produces a doubled `name@version@version` output filename (reproduced and confirmed during the spike — this is the one mistake most likely to recur if this file is ever hand-edited without re-running the verification step below).
- `packageZipUrl` must be an `https:` URL (a `package://` value fails Pkl's own type constraint) and is independent of `baseUri` — it must spell out the full GitHub-releases download path itself.
- A bare `new { check = "..." }` value (an untyped Pkl `Dynamic`) is **rejected** with `Expected value of type 'hk.Config#Step', but got type 'Dynamic'` when a consumer assigns it into a `Mapping<String, Config.Step>`. The published value must be a real `Config.Step` instance — `commit_verify: Config.Step = new { check = "..." }` — which requires importing hk's `Config.pkl` as a declared package dependency, which in turn requires `pkl project resolve` to generate `PklProject.deps.json` before packaging.
- With the above two fixes in place, an end-to-end consumer simulation (`import "@hk/Config.pkl"`, `import "Builtins.pkl" as HerautBuiltins`, `steps: Mapping<String, Config.Step> = new { ["heraut-commit-lint"] = HerautBuiltins.commit_verify }`) evaluates cleanly and renders a fully-populated `Config.Step` with every hk default field present and `check` set correctly.

## Global Constraints

- No new heraut Go application code — `heraut release`'s asset-upload path (`internal/platforms/github`) is reused unchanged; this plan only adds data files, one smoke test, and CI/config wiring.
- `pkl/PklProject`'s `version` field is meaningless outside a real release build — it is rewritten to the resolved `$VERSION` only momentarily, mid-CI-run, immediately before packaging (ADR-0029's documented consequence). Never hand-edit it to a "real-looking" value on `main`.
- Always pass `--skip-publish-check` to `pkl project package` — both in the smoke test and in the real release workflow. Per `testing.md`'s "No network calls" determinism rule the test must never depend on a live GitHub HTTP round-trip; in the release workflow, every version is unique and never re-released, so the duplicate-publish protection the flag skips has no use case here, and skipping it removes a CI flake source.
- `pkl/PklProject`'s `dependencies.hk` pin (`v1.46.0`) should track whatever hk version heraut's own `.config/hk/config.pkl` imports — bump both together if one changes, don't let them drift.
- Conventional-commit subject lines for this work's own commits, per `workflow.md`'s type table (`feat`, `chore`, `ci`, `docs`, ...).
- Reference docs: [ADR-0029](../../docs/adr/0029-pkl-builtin-commit-verify.md), [T118 in the roadmap](../../docs/tasks/roadmap.md).

---

### Task 1: `pkl/` — the package source

**Files:**
- Create: `pkl/PklProject`
- Create: `pkl/Builtins.pkl`
- Create (generated): `pkl/PklProject.deps.json`

**Interfaces:**
- Produces: a Pkl module `heraut.Builtins` exporting `commit_verify: Config.Step`. Task 2's smoke test packages this directory; Task 3's release step packages the same directory with `version` rewritten.

- [ ] **Step 1: Create the `pkl/` directory and write `PklProject`**

```
amends "pkl:Project"

dependencies {
  ["hk"] {
    uri = "package://github.com/jdx/hk/releases/download/v1.46.0/hk@1.46.0"
  }
}

package {
  name = "heraut"
  version = "0.0.0-dev"
  baseUri = "package://github.com/adaouat/heraut/releases/download/v\(version)/\(name)"
  packageZipUrl = "https://github.com/adaouat/heraut/releases/download/v\(version)/\(name)@\(version).zip"
  description = "Pkl builtins for heraut (https://github.com/adaouat/heraut)"
}
```

`version = "0.0.0-dev"` is the checked-in placeholder; Task 3's CI step is the only thing that ever changes it, and only transiently.

- [ ] **Step 2: Write `Builtins.pkl`**

```
module heraut.Builtins

import "@hk/Config.pkl"

/// Runs `heraut commit verify` against the commit message being authored.
/// Mirrors heraut's own `.config/hk/config.pkl` `commit-msg` hook step.
commit_verify: Config.Step = new {
  check = "heraut commit verify --file {{ commit_msg_file }}"
}
```

This calls the `heraut` binary directly (assumes it is on `PATH`) — never `go run ./cmd/heraut`, which only makes sense inside heraut's own source checkout. The doc comment exists because the *why* (mirrors an existing hook, not a generic example) isn't obvious from the code alone.

- [ ] **Step 3: Resolve the hk dependency**

Run: `pkl project resolve pkl/`
Expected: creates `pkl/PklProject.deps.json` and prints its path. This file is a lockfile (pins the exact resolved checksum for the hk dependency) and must be committed, the same way `go.sum` is committed.

- [ ] **Step 4: Manually verify packaging succeeds**

Run:
```bash
rm -rf /tmp/heraut-pkl-verify
pkl project package pkl/ --output-path /tmp/heraut-pkl-verify/ --skip-publish-check
ls /tmp/heraut-pkl-verify/
```
Expected: exit 0, and the directory listing shows exactly four files: `heraut@0.0.0-dev`, `heraut@0.0.0-dev.sha256`, `heraut@0.0.0-dev.zip`, `heraut@0.0.0-dev.zip.sha256`.

- [ ] **Step 5: Confirm `hk check` has no complaints about the new files**

Run: `mise run lint:check`
Expected: passes. If `pkl/PklProject` (no extension) is *not* picked up by hk's own `pkl`/`pkl_format` builtin steps, that's fine — it's not a requirement, just worth knowing. If it *is* picked up and reformats `pkl/Builtins.pkl`, accept hk's formatting as canonical and re-run Step 4 to confirm packaging still succeeds against the reformatted file.

- [ ] **Step 6: Commit**

```bash
git add pkl/PklProject pkl/PklProject.deps.json pkl/Builtins.pkl
git commit -m "feat(pkl): add heraut.Builtins Pkl package source"
```

---

### Task 2: Real-CLI smoke test

**Files:**
- Create: `pkl_test.go` (repo root, package `heraut` — matches `changelog.go`'s existing root-level package)

**Interfaces:**
- Consumes: `pkl/` from Task 1 (the directory being packaged).
- Produces: nothing consumed by later tasks — this is a leaf test.

This follows the same real-CLI smoke-test convention as `internal/generators/gitcliff/generator_test.go`'s `TestEmbeddedConfig_RealGitCliff` (per `testing.md`'s documented exception): skip if the real binary is absent, otherwise actually run it, no mocking.

- [ ] **Step 1: Write the failing test**

```go
package heraut_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPklBuiltinPackages asserts the real `pkl` CLI accepts and can package
// pkl/Builtins.pkl — MockRunner-style contract tests can't catch a Pkl syntax
// or type error the real tool would reject. Skips when pkl is not on PATH;
// runs in CI where mise installs it. See ADR-0029.
func TestPklBuiltinPackages(t *testing.T) {
	if _, err := exec.LookPath("pkl"); err != nil {
		t.Skip("pkl not on PATH")
	}

	outDir := t.TempDir()
	cmd := exec.Command("pkl", "project", "package", "pkl/",
		"--output-path", outDir+"/",
		"--skip-publish-check",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "real pkl must accept pkl/Builtins.pkl: %s", out)

	zipPath := filepath.Join(outDir, "heraut@0.0.0-dev.zip")
	require.FileExists(t, zipPath)
}
```

- [ ] **Step 2: Run it to verify it currently fails**

Run: `go test . -run TestPklBuiltinPackages -v`
Expected: FAIL — `pkl/` does not exist relative to the test binary's working directory yet if Task 1 wasn't completed first in this same checkout. (If Task 1 is already committed, as it will be when these tasks run in sequence, this step instead passes immediately — in that case, skip ahead to confirming it passes and note in the report that RED was skipped because Task 1's deliverable already satisfies it. Either outcome is fine; this test has no separate "implementation" step to write since Task 1 already produced everything it asserts against.)

- [ ] **Step 3: Run the full test to verify it passes**

Run: `go test . -run TestPklBuiltinPackages -v`
Expected: PASS (or SKIP, on a machine without `pkl` on `PATH` — both are acceptable outcomes for this command; only a genuine FAIL blocks the task).

- [ ] **Step 4: Run the full repo test suite to confirm no regressions**

Run: `go test ./...`
Expected: all packages pass.

- [ ] **Step 5: Commit**

```bash
git add pkl_test.go
git commit -m "test(pkl): add real-CLI smoke test for heraut.Builtins packaging"
```

---

### Task 3: Release pipeline wiring

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `.config/heraut.yml`

**Interfaces:**
- Consumes: `pkl/PklProject` from Task 1 (the file this task's new CI step rewrites in place before packaging).

- [ ] **Step 1: Add the packaging step to `.github/workflows/release.yml`**

Insert a new step immediately after the existing `"Collect release binaries"` step and before `"Attest build provenance"` (find the `"Collect release binaries"` step — it currently ends with `echo "FRESH_BIN=dist/heraut_${VERSION#v}_linux_amd64" >> "$GITHUB_ENV"` — insert this directly after it):

```yaml
      - name: Package Pkl builtin
        run: |
          sed -i "s/^  version = .*/  version = \"${VERSION#v}\"/" pkl/PklProject
          pkl project package pkl/ --output-path dist/ --skip-publish-check
```

`sed -i` (GNU sed, no backup-suffix argument) is correct here because this job runs on `ubuntu-latest`; do not add a `''` backup-suffix argument (that's the BSD/macOS form and would create a literal file named `''` on Linux). `--skip-publish-check` is required per this plan's Global Constraints — every release version is new, so the duplicate-publish check this flag skips has no use case and would otherwise add a live network call to the release pipeline.

- [ ] **Step 2: Add the new asset glob to `.config/heraut.yml`**

Find the existing `release.assets` list (currently ending in `- "dist/checksums.txt"`) and add one new entry:

```yaml
  assets:
    - "dist/heraut_*_linux_amd64"
    - "dist/heraut_*_linux_arm64"
    - "dist/heraut_*_darwin_amd64"
    - "dist/heraut_*_darwin_arm64"
    - "dist/heraut_*_windows_amd64.exe"
    - "dist/checksums.txt"
    - "dist/heraut@*"
```

`dist/heraut@*` matches all four files Task 1's verification step showed `pkl project package` produces (`heraut@X.Y.Z`, `.sha256`, `.zip`, `.zip.sha256`) — uploading all four as release assets mirrors how heraut already uploads `checksums.txt` alongside the binaries themselves, rather than only the binaries.

- [ ] **Step 3: Locally simulate the new CI step end-to-end**

This step cannot be "run" as a unit test (it's a workflow YAML change with no test harness in this repo), so verify it by hand, exactly as CI will execute it, then revert the local file mutation:

```bash
VERSION=v9.9.9-plan-verification
sed -i "s/^  version = .*/  version = \"${VERSION#v}\"/" pkl/PklProject
rm -rf /tmp/heraut-pkl-ci-verify
pkl project package pkl/ --output-path /tmp/heraut-pkl-ci-verify/ --skip-publish-check
ls /tmp/heraut-pkl-ci-verify/
git checkout -- pkl/PklProject
```

Expected: the `ls` output shows `heraut@9.9.9-plan-verification.zip` (and its three siblings) — confirming the `sed` pattern actually matches and rewrites the line CI will rewrite, with `dist/heraut@*` correctly matching the result. `git checkout -- pkl/PklProject` restores the file to its committed `0.0.0-dev` placeholder afterward — confirm with `git diff pkl/PklProject` that it shows no changes before continuing.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml .config/heraut.yml
git commit -m "ci(release): package and upload the heraut.Builtins Pkl artifact"
```

---

## Deferred (per ADR-0029 — do not add to this plan)

- Provenance attestation for the Pkl zip.
- An automated post-release check that the published `package://` URL resolves for a real consumer (only verifiable after the first release that includes this work actually ships).
- Switching `bifrost`/`forge`/`hermes` over to consume the published package — separate follow-up work in each of those repos.
