# T57 — `heraut release --build` for build-id release flows

> Implementation step 0: rename this plan file to `t57-release-build-flag.md` before the
> first commit (auto-generated name; per saved naming preference).

## Context

heraut supports a `{build}` token in `tag_format` (e.g. `{env}/{version}-{build}` →
`uat/7.4.1-158404`) for teams that tag **per CI build** (typically mobile). Because a build
ID isn't inferable from git history, build-id flows require the value to be passed
explicitly. `heraut changelog --build <id> --version <ver>` already supports this, but
`heraut release` does **not** (it calls `NewResolver(..., "", ...)` at
`internal/cmd/release.go:67`), so build-id teams can tag per build but cannot run a full
release (changelog → tag → publish) per build. T57 closes that gap.

The resolver already does the real work: `app.NewResolver(..., buildID, ...)`
(`internal/app/resolver.go:23-43`) renders `{build}` into the tag via
`tagfmt.Render(tf, env, version, build)` and returns a `StaticResolver`. The release
pipeline consumes `result.Tag` unchanged, so **the change is localized to the `release`
command + docs + tests** — not the multi-file effort the roadmap's `Files:` line guessed
(`internal/app/pipeline.go` needs no change).

**Product decision (was the reason T57 was held):** release-per-build is **allowed freely** —
no guard, no warn. Passing both `--version` and `--build` is already a deliberate, scripted
opt-in; a per-build warning would just be CI log noise. This matches `changelog --build`
(no guard there either). Documented as a caveat in the spec.

## Approach

Mirror the `changelog --build` wiring (`internal/cmd/changelog.go:20, 33-40, 59, 95`) into
`internal/cmd/release.go`. Reuse — do not duplicate — `app.NewResolver`'s build path and
`app.ValidateBuildID`.

## Changes

**1. `internal/cmd/release.go`** (the only production change)
- Add a `buildID` var and a `--build` string flag (mirror `changelog.go:20, 95`):
  `"build ID appended to the tag via the {build} token in tag_format (requires --version)"`.
- After the existing `versionPattern` check (`release.go:26-31`), add the validation block
  mirroring `changelog.go:33-40`: if `buildID != ""` → require `versionOverride != ""`
  (else `exitcode.Config` error "--build requires --version…") and run
  `app.ValidateBuildID(buildID)`.
- Pass `buildID` (not `""`) to `app.NewResolver` at `release.go:67`.
- Nothing else: resolver renders the tag; the pipeline already flows `result.Tag` to the
  tag step, notes, and every platform's `CreateRelease`.

**2. Docs**
- `docs/specs/02-configuration.md`: flip the support-table row (~line 316) from
  `heraut release | ❌ no --build flag (planned — roadmap T57)` → `✅ supported`, update the
  "Scope — changelog-only" note (~line 309) to include release, and add the release example
  + the "allowed freely (one release per build is intentional)" caveat.
- `docs/specs/03-commands.md`: add `--build` to the `heraut release` flag list.

**3. Roadmap** — flip T57 `[ ]`→`[x]` with a Done note recording: allow-freely decision,
that `internal/app/pipeline.go` was untouched (resolver already handles build), and the
spec update.

## Tests (TDD — red → green, mirroring `internal/cmd/changelog_test.go:19-44`)

In `internal/cmd/release_test.go` (uses the `executeRoot(...)` harness):
- **Flag registered:** `--build` present on `NewReleaseCmd()` (extend the structural flag
  list at `release_test.go:13-19`).
- **`TestRelease_BuildRequiresVersion`:** `release --build 12345` (no `--version`) → error
  mentioning `--build` / `--version` (mirror `TestChangelog_BuildRequiresVersion`).
- **`TestRelease_BuildRejectsInvalidValue`:** `release --version 7.4.1 --build bad/value`
  with a `{build}` `tag_format` → error containing "build".
- **`TestRelease_Build_DryRun_RendersTag` (integration):**
  `release --version 7.4.1 --build 158404 --env uat --dry-run` against a fixture with
  `tag_format: "{env}/{version}-{build}"` and one platform → dry-run output contains
  `uat/7.4.1-158404`. This exercises the full new path cmd → `NewResolver` → pipeline.

Platform contract coverage (acceptance: "release pipeline with a build ID"): in
`internal/pipeline/release_test.go`, add/extend a test using the existing `fakeResolver`
returning a build-rendered tag (e.g. `uat/7.4.1-158404`) + `MockPlatform`, asserting
`CreateRelease` receives that exact tag (reuses the established harness; confirms the build
tag propagates to the platform).

## Verification

- `go test ./internal/cmd/ ./internal/app/ ./internal/pipeline/` green.
- Manual: `mise run build` then
  `./heraut release --version 7.4.1 --build 158404 --env uat --dry-run` against a temp
  config with `{env}/{version}-{build}` tag_format + a github platform → prints the
  `uat/7.4.1-158404` tag and the dry-run publish line.
- Full suite green + `hk check` (golangci-lint) clean.
- Commit in logical pieces (`feat(cmd): …`, `docs(specs): …`, `docs(roadmap): …`).

## Out of scope

- No guard/warn (decided: allow freely).
- No `internal/app/pipeline.go` / resolver / `tagfmt` changes — the build path already
  exists and is tested (`resolver_test.go`, `tagfmt` tests).
- Duplicate-tag on re-running the same build = existing `git tag` failure behavior,
  unchanged.
