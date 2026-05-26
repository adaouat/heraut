# Remove Platform and Pipeline Duplication

## Context

Two independent duplication sites were identified during a code audit:

1. `resolveGlobs()` is copy-pasted identically between `github` and `gitlab` platform packages. With more platforms planned, this will keep spreading.
2. Four helper methods — `run`, `gitCommitChangelog`, `commitMessage`, `gitTag` — are byte-for-byte identical across `Pipeline` and `ChangelogPipeline`.

This plan eliminates both sites in two isolated commits.

---

## Part 1 — Shared glob resolver for platforms

### New file: `internal/platforms/globs.go` (package `platforms`)

Extract an exported `ResolveGlobs(patterns []string) ([]string, error)` into the parent `internal/platforms/` package. Both `github` and `gitlab` (and any future platform) import `github.com/adaouat/heraut/internal/platforms` and call `platforms.ResolveGlobs(p.cfg.Assets)`.

The body is identical to both current private copies:
- iterate patterns, call `filepath.Glob`
- error on zero matches
- filter out directories via `os.Stat`

### New file: `internal/platforms/globs_test.go` (written first — TDD)

Tests to write before the implementation (matching existing pattern in the repo):
- happy path: single glob matches multiple files, dirs filtered out
- error: no matches for a pattern
- error: invalid glob syntax
- multiple patterns: all resolved and merged

### Changes to existing files

- `internal/platforms/github/platform.go`: delete private `resolveGlobs`, replace call on line 124 with `platforms.ResolveGlobs(p.cfg.Assets)`
- `internal/platforms/gitlab/platform.go`: delete private `resolveGlobs`, replace call on line 79 with `platforms.ResolveGlobs(p.cfg.Assets)`

### Commit

```
refactor(platforms): extract resolveGlobs to shared platforms package
```

---

## Part 2 — Shared git helpers for pipelines

### New file: `internal/pipeline/git.go` (same package)

Both pipeline types are in `package pipeline`, so extraction stays within the package. Create a `gitHelper` struct that wraps a `port.Runner`:

```go
type gitHelper struct {
    runner port.Runner
}

// run wraps runner.Run, discarding stdout/stderr.
func (g *gitHelper) run(name string, args ...string) error { ... }

// commitChangelog stages file, commits with msg, and pushes.
func (g *gitHelper) commitChangelog(file, msg string) error { ... }

// tag creates an annotated or lightweight git tag.
func (g *gitHelper) tag(tag, msg string, annotated bool) error { ... }

// commitMessage substitutes ${version} in template, falling back to the default.
func commitMessage(template, version string) string { ... }

const defaultCommitMessage = "chore(release): ${version}"
```

Note: `commitMessage` becomes a package-level function (not a method) because it has no dependency on `gitHelper`.

### Changes to `internal/pipeline/release.go`

- Embed `gitHelper` in `Pipeline` struct: `git gitHelper`
- Initialize it in `New()`: `git: gitHelper{runner: runner}`
- Delete methods: `run`, `gitCommitChangelog`, `commitMessage`, `gitTag`
- Delete constant `defaultCommitMessage`
- Update call sites:
  - `p.gitCommitChangelog(result)` → `p.git.commitChangelog(file, commitMessage(p.cfg.CommitMessage, result.Version))`
  - `p.gitTag(result.Tag, result.Version)` → `p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags)`
  - `p.run(...)` → `p.git.run(...)`

### Changes to `internal/pipeline/changelog.go`

Same pattern as release.go:
- Embed `gitHelper` in `ChangelogPipeline`
- Delete the four duplicate methods
- Update call sites identically

### No new tests needed

The existing `release_test.go` (579 lines) and `changelog_test.go` (321 lines) already exercise all four extracted methods end-to-end through `MockRunner`. They must pass unchanged after the refactor — that is the verification.

### Commit

```
refactor(pipeline): extract shared git helpers to gitHelper struct
```

---

## Verification

```bash
mise run test        # all packages must pass, no new failures
mise run lint:check  # no new lint warnings
```

The two existing test suites cover all extracted behavior. No new test infrastructure is needed for Part 2.
