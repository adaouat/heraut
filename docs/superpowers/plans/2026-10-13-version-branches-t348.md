# Version branches (T348) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On a branch named like a version (`release/7.8.0`), `heraut release` cuts exactly that version, commits never bump it, repeat builds go through `--pre-release rc` / `--set-version … --set-build-id`, and a re-release refuses.

**Architecture:** A glob `versioning.branches` entry (`release/*`) whose branch's last segment is `N.N.N` yields a new match kind `BranchVersion`. `MatchBranchRule` produces it; `applyBranchRule` (renamed from `applyMaintenanceRange`) hands the version to the semver resolver through `SetFixedVersion`, which skips bump computation. No new config key.

**Tech Stack:** Go, testify, `exectest.MockRunner` (unit), real git repos via `historyRepo` (`internal/app`), the e2e scenario DSL (`e2e/maintenance_test.go`).

**Spec:** `docs/superpowers/specs/2026-10-13-version-branches-design.md`

## Global Constraints

- `versioning.strategy: semver` only (like `versioning.branches`).
- Platforms/layering rules unchanged: derivation in `internal/config`, matching in `internal/app`, resolution in `internal/versioning/semver`.
- Ordering check is a **warning only**, and only for a SemVer release *merged into HEAD* that outranks the branch version. A higher tag outside the branch's history is silent.
- A plain re-release on an already released branch version is an error (`semver.ErrTagExists`); a build-metadata tag (`v7.8.0+3`) counts as released.
- `--set-version` stays an explicit override and may differ from the branch version (warning, not error).
- Commits: conventional, subject ≤ 72 chars, trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `--no-verify`, run git/hk under `mise exec --`, formatting via `mise exec -- hk fix -S go_fmt` / `-S golangci_lint`.
- TDD: failing test first, watch it fail, then implement. Comments only where the *why* is non-obvious.

## Review Focus

- `release/v7.8.0` (leading `v`) and a custom `tag_prefix` both resolve to the right tag (Task 1 table row, Task 3 prefix test).
- A `07.8.0` / `7.8.0.1` / `7.8` / `7.8.0-rc.1` last segment is **not** a version branch (Task 1 table; `7.8` still derives a maintenance line, Task 2).
- A `feat!:` breaking commit on the version branch never changes the cut version (Task 4 real-git test).
- A build-metadata-only tag of the version (`v7.8.0+3`) counts as the final and blocks a plain release (Task 3).
- The ordering warning appears only after the higher tag is merged into the branch (Task 4 real-git test).

---

### Task 1: Derive a version from a branch name

**Files:**
- Modify: `internal/config/branches.go`
- Test: `internal/config/branches_test.go`

**Interfaces:**
- Produces: `config.DeriveBranchVersion(branch string) (version string, ok bool)` — the bare `N.N.N` carried by the branch's last `/` segment (one optional leading `v`/`V` stripped).

- [ ] **Step 1: Write the failing test** — append to `internal/config/branches_test.go`:

```go
func TestDeriveBranchVersion(t *testing.T) {
	tests := []struct {
		branch string
		want   string
		ok     bool
	}{
		{"release/7.8.0", "7.8.0", true},
		{"release/v7.8.0", "7.8.0", true},
		{"release/V7.8.0", "7.8.0", true},
		{"7.8.0", "7.8.0", true},
		{"client/acme/release/0.10.2", "0.10.2", true},
		{"release/7.8", "", false},
		{"release/7.8.x", "", false},
		{"release/07.8.0", "", false},
		{"release/7.08.0", "", false},
		{"release/7.8.0.1", "", false},
		{"release/7.8.0-rc.1", "", false},
		{"release/7.8.0+3", "", false},
		{"release/vv7.8.0", "", false},
		{"release/legacy", "", false},
		{"release/", "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.branch, func(t *testing.T) {
			got, ok := config.DeriveBranchVersion(tc.branch)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `mise exec -- go test ./internal/config -run TestDeriveBranchVersion`
Expected: FAIL (compile error: `config.DeriveBranchVersion` undefined).

- [ ] **Step 3: Implement** — in `internal/config/branches.go`, add next to `branchDerivePattern`:

```go
var branchVersionPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// DeriveBranchVersion reads the version a branch name carries: its last "/" segment as N.N.N,
// with one optional leading v/V (release/7.8.0, release/v7.8.0). It is the branch-is-the-version
// counterpart of DeriveBranchRange, which deliberately refuses N.N.N because it names a release,
// not a line.
func DeriveBranchVersion(branch string) (string, bool) {
	seg := branch[strings.LastIndex(branch, "/")+1:]
	if len(seg) > 0 && (seg[0] == 'v' || seg[0] == 'V') {
		seg = seg[1:]
	}
	if !branchVersionPattern.MatchString(seg) {
		return "", false
	}
	return seg, true
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `mise exec -- go test ./internal/config -run TestDeriveBranchVersion`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add internal/config && mise exec -- git commit -m "feat(config): derive a version from a branch name

DeriveBranchVersion reads N.N.N from a branch's last segment, the
counterpart of DeriveBranchRange, for version branches (T348).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Match version branches

**Files:**
- Modify: `internal/app/branchrules.go:20-35,94-112`
- Test: `internal/app/branchrules_test.go:76-125`

**Interfaces:**
- Consumes: `config.DeriveBranchVersion` (Task 1).
- Produces: `app.BranchVersion` (a new `BranchKind`, appended after `BranchUnlisted` so existing values do not shift) and `BranchMatch.Version string` (bare `N.N.N`, set only for `BranchVersion`). Matching never errors for a version branch, with or without `needRange`.

- [ ] **Step 1: Write the failing tests** — in `TestMatchBranchRule` (`internal/app/branchrules_test.go`):
  1. add `wantVersion string` to the table struct, and assert it after `assert.Equal(t, tc.wantRule, m.Rule)`:
     `assert.Equal(t, tc.wantVersion, m.Version)`;
  2. change the existing `underivable, set-version` row (it used `release/7.8.0`, which is now a version branch) to `release/legacy`, and update the `underivable` row's message;
  3. add the new rows.

```go
		{name: "underivable", branch: "release/legacy", needRange: true, wantErr: app.ErrUnderivableRange,
			wantMsg: `branch "release/legacy" matches versioning.branches[1] ("release/*") but its name carries no N.x / N.N.x / N.N / N.N.N version — add range: to the entry`},
		{name: "underivable, set-version", branch: "release/legacy", needRange: false, wantKind: app.BranchMaintenance, wantRule: 1},
		{name: "version branch", branch: "release/7.8.0", needRange: true, wantKind: app.BranchVersion, wantRule: 1, wantVersion: "7.8.0"},
		{name: "version branch, v prefix", branch: "release/v7.8.0", needRange: true, wantKind: app.BranchVersion, wantRule: 1, wantVersion: "7.8.0"},
		{name: "version branch, set-version path", branch: "release/7.8.0", needRange: false, wantKind: app.BranchVersion, wantRule: 1, wantVersion: "7.8.0"},
		{name: "N.N still derives a line", branch: "release/7.8", needRange: true, wantKind: app.BranchMaintenance, wantRule: 1, wantRange: config.BranchRange{Major: 7, Minor: ptrU64(8)}},
		{name: "leading zero is not a version", branch: "release/07.8.0", needRange: true, wantErr: app.ErrUnderivableRange,
			wantMsg: `branch "release/07.8.0" matches versioning.branches[1]`},
		{name: "explicit range wins over name shape", rules: []config.BranchRule{{Name: "release/*", Range: "7.x"}}, branch: "release/7.8.0", needRange: true,
			wantKind: app.BranchMaintenance, wantRange: config.BranchRange{Major: 7}, wantRule: 0},
		{name: "exact name is never a version branch", rules: []config.BranchRule{{Name: "release/7.8.0"}}, branch: "release/7.8.0", needRange: true, wantKind: app.BranchRelease, wantRule: 0},
```

  Add the helper at the bottom of the file: `func ptrU64(n uint64) *uint64 { return &n }`. In `TestCheckReleaseBranch`, add inside the function:

```go
	t.Run("version branch is listed", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("release/7.8.0\n", "", nil)
		require.NoError(t, app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), false))
	})
```

- [ ] **Step 2: Run to verify failure**

Run: `mise exec -- go test ./internal/app -run 'TestMatchBranchRule|TestCheckReleaseBranch'`
Expected: FAIL (compile error: `app.BranchVersion`, `wantVersion`).

- [ ] **Step 3: Implement** — in `internal/app/branchrules.go`:

```go
const (
	BranchRelease     BranchKind = iota // exact name, no range: today's behaviour
	BranchMaintenance                   // has a Range
	BranchUnlisted                      // no entry matches, or the branch is unknown
	BranchVersion                       // the branch name is the version (Version)
)

type BranchMatch struct {
	Branch  string // "" when unknown
	Kind    BranchKind
	Range   config.BranchRange // set when Kind == BranchMaintenance
	Version string             // bare N.N.N, set when Kind == BranchVersion
	Rule    int                // index into cfg.Versioning.Branches; -1 when unlisted
}
```

  and replace the `case rule.IsGlob():` arm:

```go
	case rule.IsGlob():
		if r, ok := config.DeriveBranchRange(branch); ok {
			match.Range = r
		} else if v, ok := config.DeriveBranchVersion(branch); ok {
			match.Kind, match.Version = BranchVersion, v
		} else if needRange {
			return BranchMatch{}, fmt.Errorf("%w: branch %q matches versioning.branches[%d] (%q) but its name carries no N.x / N.N.x / N.N / N.N.N version — add range: to the entry",
				ErrUnderivableRange, branch, idx, rule.Name)
		}
```

  Update the `MatchBranchRule` doc comment: a version branch needs no range, so `needRange == false` only affects names carrying neither.

- [ ] **Step 4: Run to verify pass**

Run: `mise exec -- go test ./internal/app ./internal/cmd`
Expected: PASS (`internal/cmd` exercises the error text; if a test pins the old "N.N version" wording, update it to the new message in the same commit).

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add internal && mise exec -- git commit -m "feat(app): match version branches in versioning.branches

A glob entry whose branch ends in N.N.N now yields BranchVersion
instead of ErrUnderivableRange (T348).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Fixed-version resolution in the semver resolver

**Files:**
- Create: `internal/versioning/semver/fixed.go`
- Create: `internal/versioning/semver/fixed_test.go`
- Modify: `internal/versioning/semver/resolver.go` (struct field `fixedCore string`; `Resolve` dispatch)

**Interfaces:**
- Consumes: package-internal `SortTags`, `Latest`, `Parse`, `coreVersion`, `nextCounter`, `parseTags`, `sentinelError`, `ErrTagExists`, `ErrPreReleaseRegression`.
- Produces: `(*Resolver).SetFixedVersion(core string)` — `""` restores normal resolution. With a core set, `Resolve` (after the `--set-version` / manual-mode short-circuit) returns the tag for `core` (final, or `core-<label>.N` with `SetPreRelease`), `Bump: BumpNone`, `CurrentTag` = highest tag merged into HEAD that sorts below the result, and records the ordering warning in `Warnings()`.

- [ ] **Step 1: Write the failing tests** — `internal/versioning/semver/fixed_test.go`:

```go
package semver_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedResolver(t *testing.T, cfg *config.Config, core, label string, all, merged string) (*semver.Resolver, *exectest.MockRunner) {
	t.Helper()
	mr := exectest.NewMockRunner()
	mr.QueueResponse(all, "", nil)
	mr.QueueResponse(merged, "", nil)
	r := semver.New(mr, cfg)
	r.SetFixedVersion(core)
	r.SetPreRelease(label)
	return r, mr
}

func semverCfg() *config.Config {
	return &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
}

func TestResolveFixed(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		all      string
		merged   string
		wantTag  string
		wantVer  string
		wantCur  string
		wantErr  error
		wantSub  string
		wantWarn string
	}{
		{name: "first release", all: "v7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
		{name: "no tags at all", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: ""},
		{name: "rc never bumps anything but the counter", label: "rc", all: "v7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0-rc.1", wantVer: "7.8.0-rc.1", wantCur: "v7.7.0"},
		{name: "second rc", label: "rc", all: "v7.8.0-rc.1\nv7.7.0\n", merged: "v7.8.0-rc.1\nv7.7.0\n", wantTag: "v7.8.0-rc.2", wantVer: "7.8.0-rc.2", wantCur: "v7.8.0-rc.1"},
		{name: "final after rcs", all: "v7.8.0-rc.2\nv7.7.0\n", merged: "v7.8.0-rc.2\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.8.0-rc.2"},
		{name: "already released", all: "v7.8.0\nv7.7.0\n", merged: "v7.8.0\nv7.7.0\n", wantErr: semver.ErrTagExists, wantSub: "--pre-release"},
		{name: "build-metadata tag is the release", all: "v7.8.0+3\nv7.7.0\n", merged: "v7.8.0+3\nv7.7.0\n", wantErr: semver.ErrTagExists, wantSub: "--set-version 7.8.0 --set-build-id"},
		{name: "released elsewhere still counts", all: "v7.8.0\nv7.7.0\n", merged: "v7.7.0\n", wantErr: semver.ErrTagExists},
		{name: "rc after the final", label: "rc", all: "v7.8.0\nv7.8.0-rc.1\n", merged: "v7.8.0\nv7.8.0-rc.1\n", wantErr: semver.ErrPreReleaseRegression, wantSub: "already released"},
		{name: "lower label than an existing one", label: "beta", all: "v7.8.0-rc.1\n", merged: "v7.8.0-rc.1\n", wantErr: semver.ErrPreReleaseRegression},
		{name: "higher tag outside the branch history is silent", all: "v7.9.0\nv7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
		{name: "higher release in the branch history warns", all: "v7.9.0\nv7.7.0\n", merged: "v7.9.0\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0",
			wantWarn: "branch version 7.8.0 is lower than v7.9.0, already in this branch's history"},
		{name: "a merged pre-release does not warn", all: "v7.9.0-rc.1\nv7.7.0\n", merged: "v7.9.0-rc.1\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.9.0-rc.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, mr := fixedResolver(t, semverCfg(), "7.8.0", tc.label, tc.all, tc.merged)

			res, err := r.Resolve()

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				if tc.wantSub != "" {
					assert.Contains(t, err.Error(), tc.wantSub)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantVer, res.Version)
			assert.Equal(t, tc.wantCur, res.CurrentTag)
			assert.Equal(t, versioning.BumpNone, res.Bump)
			if tc.wantWarn == "" {
				assert.Empty(t, r.Warnings())
			} else {
				assert.Equal(t, []string{tc.wantWarn}, r.Warnings())
			}
			require.Len(t, mr.Calls, 2, "tags and merged tags only — commits are never read")
			assert.Equal(t, tagsCall("v"), mr.Calls[0].Args)
			assert.Equal(t, mergedCall("v"), mr.Calls[1].Args)
		})
	}
}

func TestResolveFixed_CustomPrefix(t *testing.T) {
	prefix := "rel-"
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix}}
	r, _ := fixedResolver(t, cfg, "7.8.0", "", "rel-7.7.0\n", "rel-7.7.0\n")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "rel-7.8.0", res.Tag)
	assert.Equal(t, "7.8.0", res.Version)
}

func TestResolveFixed_SetVersionStillWins(t *testing.T) {
	mr := exectest.NewMockRunner()
	r := semver.New(mr, semverCfg())
	r.SetFixedVersion("7.8.0")
	r.SetVersionOverride("9.9.9")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", res.Tag)
	assert.Empty(t, mr.Calls)
}

func TestResolveFixed_EmptyRestoresNormalResolution(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // no tags: initial version
	r := semver.New(mr, semverCfg())
	r.SetFixedVersion("7.8.0")
	r.SetFixedVersion("")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", res.Tag)
}
```

  (`tagsCall` / `mergedCall` already exist in `prerelease_test.go`, same package.)

- [ ] **Step 2: Run to verify failure**

Run: `mise exec -- go test ./internal/versioning/semver -run 'TestResolveFixed'`
Expected: FAIL (compile error: `SetFixedVersion` undefined).

- [ ] **Step 3: Implement** — `internal/versioning/semver/resolver.go`: add the field `fixedCore string` to `Resolver`, and in `Resolve` after the manual-mode `if` and before `preReleaseLabel`:

```go
	if r.fixedCore != "" {
		return r.resolveFixed()
	}
```

  `internal/versioning/semver/fixed.go`:

```go
package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

// SetFixedVersion pins resolution to core (MAJOR.MINOR.PATCH), the version a version branch
// carries in its name (ADR-0065): commits never bump it, only the pre-release label or the
// final decides the tag. "" restores normal resolution. --set-version and manual bump mode still
// win; Resolve never reaches this path for them.
func (r *Resolver) SetFixedVersion(core string) {
	r.fixedCore = core
}

func (r *Resolver) resolveFixed() (versioning.Result, error) {
	core, err := Parse(r.fixedCore)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing version branch version %q: %w", r.fixedCore, err)
	}
	prefix, label := r.prefix(), r.preReleaseLabel
	extract := func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}
	all := SortTags(parseTags(stdout), extract)
	stdout, _, err = r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing merged git tags: %w", err)
	}
	merged := SortTags(parseTags(stdout), extract)

	version := core.Core()
	if label == "" {
		for _, t := range all {
			// A build-metadata-only tag is the release of its core (ADR-0064).
			if Compare(coreVersion(t.Version), core) == 0 && !t.Version.IsPreRelease() {
				return versioning.Result{}, fmt.Errorf("%w: %s — branch version %s is already released; pass --pre-release only before the final, or --set-version %s --set-build-id <id> for a repeat build",
					ErrTagExists, t.Tag, version, version)
			}
		}
	} else {
		version = fmt.Sprintf("%s-%s.%d", version, label, nextCounter(all, core, label))
	}
	candidate, err := Parse(version)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building version %q: %w", version, err)
	}
	if label != "" {
		for _, t := range all {
			if Compare(coreVersion(t.Version), core) == 0 && Compare(candidate, t.Version) <= 0 {
				return versioning.Result{}, &sentinelError{
					sentinel: ErrPreReleaseRegression,
					msg: fmt.Sprintf("%s%s would sort below existing %s — a pre-release of branch version %s cannot follow it (already released or a higher label exists)",
						prefix, version, t.Tag, core.Core()),
				}
			}
		}
	}

	var previous string
	for _, t := range merged {
		if Compare(t.Version, candidate) < 0 {
			previous = t.Tag
			break
		}
	}
	if latest, ok := Latest(merged, false); ok && Compare(coreVersion(latest.Version), core) > 0 {
		r.warnings = append(r.warnings, fmt.Sprintf("branch version %s is lower than %s, already in this branch's history", core.Core(), latest.Tag))
	}
	return versioning.Result{
		Version:    version,
		Tag:        prefix + version,
		CurrentTag: previous,
		Bump:       versioning.BumpNone,
	}, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `mise exec -- go test ./internal/versioning/...`
Expected: PASS (the `all`/`merged` queue order matches the two `git tag` calls).

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add internal/versioning && mise exec -- git commit -m "feat(versioning/semver): resolve a fixed branch version

SetFixedVersion pins the core: the final, or -label.N, with no bump
from commits, a refusal when released, and a warning when a higher
release is already in HEAD's history (T348).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Wire version branches into the app layer

**Files:**
- Modify: `internal/app/resolver.go` (rename `applyMaintenanceRange` → `applyBranchRule` at its call site and definition; extend `collisionGuardResolver`)
- Modify: `internal/app/current.go` (`CurrentTag`)
- Test: `internal/app/versionbranch_realrepo_internal_test.go` (new)

**Interfaces:**
- Consumes: `BranchVersion` / `BranchMatch.Version` (Task 2), `(*semver.Resolver).SetFixedVersion` (Task 3), `historyRepo`, `NewResolver`, `WithPreRelease`, `CurrentTag`, `errNoTagsFound`.
- Produces: `NewResolver` on a `BranchVersion` branch returns the fixed-version resolver; `NewResolver(..., versionOverride, ...)` adds a warning to `Result.Warnings` when the override's core differs from the branch version; `CurrentTag` on a version branch returns the highest tag reachable from HEAD whose core equals the branch version.

- [ ] **Step 1: Write the failing tests** — `internal/app/versionbranch_realrepo_internal_test.go`:

```go
package app

import (
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// versionBranchFixture: main has v7.7.0; release/7.8.0 is cut from it and carries one commit.
// release/7.8.0 is left checked out.
func versionBranchFixture(t *testing.T) (git func(args ...string), commit func(msg string), tag func(name string), cfg *config.Config) {
	t.Helper()
	git = historyRepo(t)
	commit = func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag = func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v7.7.0")
	git("checkout", "-b", "release/7.8.0")
	commit("feat: client scope")

	cfg = &config.Config{Version: "1", Versioning: config.Versioning{
		Strategy: "semver",
		Branches: []config.BranchRule{{Name: "main"}, {Name: "release/*"}},
	}}
	return git, commit, tag, cfg
}

func TestVersionBranch_RealRepo_ReleaseLifecycle(t *testing.T) {
	_, commit, tag, cfg := versionBranchFixture(t)
	runner := execadapter.New(false, false)
	resolve := func(label string) (string, error) {
		var opts []ResolverOption
		if label != "" {
			opts = append(opts, WithPreRelease(label))
		}
		r, err := NewResolver(cfg, "", false, "", "", runner, opts...)
		require.NoError(t, err)
		res, err := r.Resolve()
		return res.Tag, err
	}

	commit("feat!: breaking, but the branch is the version")
	got, err := resolve("")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", got, "a breaking commit never bumps a version branch")

	got, err = resolve("rc")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.1", got)
	tag(got)

	commit("fix: stabilise")
	got, err = resolve("rc")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.2", got)
	tag(got)

	got, err = resolve("")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", got, "the final follows the release candidates")
	tag(got)

	_, err = resolve("")
	require.ErrorIs(t, err, semver.ErrTagExists)

	_, err = resolve("rc")
	require.ErrorIs(t, err, semver.ErrPreReleaseRegression, "no pre-release after the final")

	r, err := NewResolver(cfg, "", false, "7.8.0", "158404", runner)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0+158404", res.Tag, "repeat builds go through --set-version/--set-build-id")
	assert.Empty(t, res.Warnings, "the override equals the branch version")
}

func TestVersionBranch_RealRepo_BuildMetadataTagIsTheRelease(t *testing.T) {
	_, _, tag, cfg := versionBranchFixture(t)
	tag("v7.8.0+3")

	r, err := NewResolver(cfg, "", false, "", "", execadapter.New(false, false))
	require.NoError(t, err)
	_, err = r.Resolve()

	require.ErrorIs(t, err, semver.ErrTagExists)
}

func TestVersionBranch_RealRepo_OrderingWarningOnlyWhenMerged(t *testing.T) {
	git, commit, tag, cfg := versionBranchFixture(t)
	git("checkout", "main")
	commit("feat: later")
	tag("v7.9.0")
	git("checkout", "release/7.8.0")
	runner := execadapter.New(false, false)
	resolve := func() []string {
		r, err := NewResolver(cfg, "", false, "", "", runner)
		require.NoError(t, err)
		res, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "v7.8.0", res.Tag)
		return res.Warnings
	}

	assert.Empty(t, resolve(), "a higher tag on another line is legitimate")

	git("merge", "--no-ff", "-m", "chore: merge main", "main")
	assert.Equal(t, []string{"branch version 7.8.0 is lower than v7.9.0, already in this branch's history"}, resolve())
}

func TestVersionBranch_RealRepo_SetVersionMismatchWarns(t *testing.T) {
	_, _, _, cfg := versionBranchFixture(t)

	r, err := NewResolver(cfg, "", false, "7.9.1", "", execadapter.New(false, false))
	require.NoError(t, err)
	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v7.9.1", res.Tag, "--set-version stays an explicit override")
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "--set-version 7.9.1 differs from the version of branch release/7.8.0 (7.8.0)")
}

func TestVersionBranch_RealRepo_VersionCurrent(t *testing.T) {
	_, _, tag, cfg := versionBranchFixture(t)
	runner := execadapter.New(false, false)

	_, err := CurrentTag(runner, cfg, "", false)
	require.ErrorIs(t, err, errNoTagsFound, "nothing is tagged on this version yet")

	tag("v7.8.0-rc.1")
	_, err = CurrentTag(runner, cfg, "", false)
	require.ErrorIs(t, err, errNoTagsFound)
	assert.Contains(t, err.Error(), "--include-pre-release")
	cur, err := CurrentTag(runner, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.1", cur)

	tag("v7.8.0")
	cur, err = CurrentTag(runner, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", cur)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `mise exec -- go test ./internal/app -run TestVersionBranch_RealRepo`
Expected: FAIL (the resolver still treats `release/7.8.0` as a maintenance branch with a zero range; assertions on tags/warnings fail).

- [ ] **Step 3: Implement**

  1. `internal/app/resolver.go`: rename `applyMaintenanceRange` to `applyBranchRule` (definition and the one call site in `NewResolver`), update its doc comment to mention version branches, and extend the tail:

```go
	switch m.Kind {
	case BranchMaintenance:
		rg := semver.RangeFrom(m.Range, m.Branch)
		r.SetMaintenanceRange(&rg)
	case BranchVersion:
		r.SetFixedVersion(m.Version)
	}
	return nil
```

  2. `collisionGuardResolver` gains `cfg *config.Config`; `guardSetVersion` passes it (`collisionGuardResolver{inner: r, runner: runner, cfg: cfg}`). At the end of its `Resolve`, replace `return res, nil` with:

```go
	warn, err := versionBranchMismatch(g.runner, g.cfg, res.Version)
	if err != nil {
		return versioning.Result{}, err
	}
	if warn != "" {
		res.Warnings = append(res.Warnings, warn)
	}
	return res, nil
```

  and add:

```go
// versionBranchMismatch warns when an explicit --set-version names a different core than the
// version branch it runs on. An unmatched or ambiguous branch is not this check's business
// (CheckReleaseBranch reports those), so a match error is ignored.
func versionBranchMismatch(runner port.Runner, cfg *config.Config, version string) (string, error) {
	branch, known, err := CurrentBranch(runner)
	if err != nil {
		return "", err
	}
	m, err := MatchBranchRule(cfg, branch, known, false)
	if err != nil || m.Kind != BranchVersion {
		return "", nil
	}
	v, perr := semver.Parse(version)
	if perr != nil || v.Core() == m.Version {
		return "", nil
	}
	return fmt.Sprintf("--set-version %s differs from the version of branch %s (%s)", v.Core(), m.Branch, m.Version), nil
}
```

  3. `internal/app/current.go`, in `CurrentTag` after the `BranchMaintenance` block:

```go
		if m.Kind == BranchVersion {
			return currentVersionBranchTag(runner, cfg, glob, m, includePreRelease)
		}
```

  and the helper next to `currentMaintenanceTag`:

```go
// currentVersionBranchTag is CurrentTag on a version branch (ADR-0065): the highest tag reachable
// from HEAD whose core is the branch's version, never a higher tag cut on another line.
func currentVersionBranchTag(runner port.Runner, cfg *config.Config, glob string, m BranchMatch, includePreRelease bool) (string, error) {
	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("listing git tags merged into HEAD: %w", err)
	}
	core, err := semver.Parse(m.Version)
	if err != nil {
		return "", fmt.Errorf("parsing version branch version %q: %w", m.Version, err)
	}
	var onVersion []semver.TagVersion
	for _, tv := range semver.SortTags(strings.Fields(stdout), semverExtractor(cfg, "")) {
		if semver.Compare(semver.CoreOf(tv.Version), core) == 0 {
			onVersion = append(onVersion, tv)
		}
	}
	if tv, ok := semver.Latest(onVersion, includePreRelease); ok {
		return tv.Tag, nil
	}
	if len(onVersion) > 0 {
		return "", fmt.Errorf("%w for version %s reachable from %s: only pre-release tags exist (pass --include-pre-release to show them)", errNoTagsFound, m.Version, m.Branch)
	}
	return "", fmt.Errorf("%w for version %s reachable from %s", errNoTagsFound, m.Version, m.Branch)
}
```

  `Version.Core()` returns a string, so the comparison above needs a `Version`: add the exported helper `func CoreOf(v Version) Version { return coreVersion(v) }` to `internal/versioning/semver/prerelease.go` next to `coreVersion`.

- [ ] **Step 4: Run to verify pass**

Run: `mise exec -- go test ./internal/app/... ./internal/versioning/...`
Expected: PASS. Existing `--set-version` + `branches` tests built on `MockRunner` may now see one extra `git rev-parse` call (FIFO queue); for each failing one append `mr.QueueResponse("main\n", "", nil)` after its collision-probe response. Do not loosen any assertion.

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add internal && mise exec -- git commit -m "feat(app): resolve version branches end to end

NewResolver pins the branch version, --set-version warns when it
differs, and version current reports the branch version's own tag
(T348).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: End-to-end scenarios

**Files:**
- Modify: `e2e/maintenance_test.go` (append a new test function)

**Interfaces:**
- Consumes: the existing scenario DSL (`runScenarios`, `scenario`, `commit`, `tag`, `checkout`, `versionNext`, `branchesCfg`, `exitRuntime`) already defined in the `e2e_test` package.

- [ ] **Step 1: Write the failing test** — append to `e2e/maintenance_test.go`:

```go
// versionBranchHistory: main has v7.7.0, release/7.8.0 is cut from it with extra applied.
func versionBranchHistory(extra ...step) []step {
	base := []step{commit("feat: a"), tag("v7.7.0"), checkout("release/7.8.0")}
	return append(base, extra...)
}

func TestVersionBranch_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "the first release is the branch version whatever the commits", config: branchesCfg,
			history: versionBranchHistory(commit("feat!: breaking")), args: versionNext, wantOut: "v7.8.0"},
		{name: "a v-prefixed branch name gives the same version", config: branchesCfg,
			history: []step{commit("feat: a"), tag("v7.7.0"), checkout("release/v7.8.0"), commit("fix: x")},
			args:    versionNext, wantOut: "v7.8.0"},
		{name: "a release candidate numbers itself", config: branchesCfg,
			history: versionBranchHistory(commit("fix: x"), tag("v7.8.0-rc.1"), commit("fix: y")),
			args:    []string{"version", "next", "--pre-release", "rc"}, wantOut: "v7.8.0-rc.2"},
		{name: "a second release on the same version refuses with the way out", config: branchesCfg,
			history:  versionBranchHistory(commit("fix: x"), tag("v7.8.0"), commit("fix: y")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"tag already exists: v7.8.0", "--pre-release", "--set-build-id"}},
		{name: "a release candidate after the final is refused", config: branchesCfg,
			history:  versionBranchHistory(commit("fix: x"), tag("v7.8.0"), commit("fix: y")),
			args:     []string{"version", "next", "--pre-release", "rc"},
			wantExit: exitRuntime, wantText: []string{"would sort below existing v7.8.0"}},
		{name: "a repeat build is an explicit build ID", config: branchesCfg,
			history: versionBranchHistory(commit("fix: x"), tag("v7.8.0"), commit("fix: y")),
			args:    []string{"version", "next", "--set-version", "7.8.0", "--set-build-id", "158404"}, wantOut: "v7.8.0+158404"},
		{name: "version current is the branch version's own tag", config: branchesCfg,
			history: versionBranchHistory(commit("fix: x"), tag("v7.8.0-rc.1")),
			args:    []string{"version", "current", "--include-pre-release"}, wantOut: "v7.8.0-rc.1"},
		{name: "a maintenance line in the same namespace still derives its range", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")), args: versionNext, wantOut: "v1.3.1"},
	})
}
```

  If a helper name differs (`checkout`, `tag`, `commit`, `step`), use the exact identifiers `lineHistory` in this file uses.

- [ ] **Step 2: Run to verify failure, then pass**

Run (before Tasks 2-4 are merged it fails; with them present it should pass): `mise exec -- go test ./e2e -run 'TestVersionBranch_Resolution'`
Expected after Tasks 1-4: PASS. If a scenario fails because of output wording (e.g. the exact "tag already exists" text), fix the *test's* `wantText` to the real message only after reading it — never loosen the version assertions.

- [ ] **Step 3: Run the whole e2e lane**

Run: `mise exec -- go test ./e2e/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add e2e && mise exec -- git commit -m "test(e2e): version branch scenarios on the built binary

First release, rc numbering, the refused re-release, repeat builds
and version current on release/7.8.0 (T348).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Schema, docs, ADR and roadmap

**Files:**
- Modify: `schema.json` (the `branches` description, near line 547)
- Modify: `docs/heraut.sample.yml` (the `branches` comment block, lines 94-105)
- Modify: `docs/specs/02-configuration.md` (§ `versioning.branches`, lines 124-160)
- Modify: `docs/adr/0065-branch-aware-semver-resolution.md` (entry-type table and Derivation paragraph)
- Modify: `docs/guides/maintenance-branches.md` (§ "When the branch is the version", lines 96-111)
- Modify: `docs/superpowers/specs/2026-10-13-version-branches-design.md` (behaviour table: fix the `--set-build-id` row, status → Implemented)
- Modify: `docs/tasks/roadmap.md` (T348 → `[x]` with note, Phase 62 row)

- [ ] **Step 1: Edit `schema.json`** — replace the `branches` description string with:

  `"Release, maintenance and version branches (semver strategy only). An entry with a range, or a glob name whose matched branch ends in N.x / N.N.x / N.N, is a maintenance branch; a glob name whose matched branch ends in N.N.N (release/7.8.0) is a version branch whose name is its version; an exact name without a range is a release branch. See docs/specs/02-configuration.md § versioning.branches."`

- [ ] **Step 2: Edit `docs/heraut.sample.yml`** — in the comment list under `branches`, after the `glob name, no range` lines add:

```yaml
  #   glob name, matched branch ends in N.N.N   version branch: release/7.8.0 releases
  #                          exactly v7.8.0, commits never bump it (re-releases use
  #                          --pre-release rc, or --set-version 7.8.0 --set-build-id <id>)
```

- [ ] **Step 3: Edit Spec 02** — in the entry-type table add the row `| glob \`name\`, matched branch ends in \`N.N.N\` | version branch: the name is the version |`; in the paragraph after the table append: ``A last segment of the form `N.N.N` (`release/7.8.0`, `release/v7.8.0`; no leading zeros) is a **version branch**: heraut cuts exactly that version (`v7.8.0`), commits never bump it, `--pre-release rc` numbers `v7.8.0-rc.1`, `-rc.2`, …, and once the final exists a plain release or a further pre-release is refused (a repeat build is ``--set-version 7.8.0 --set-build-id <id>``). `--set-version` with another version is allowed and warns. A warning is also printed when a higher SemVer release is already merged into the branch. Exact-name entries are never version branches.``

- [ ] **Step 4: Edit ADR-0065** — add a row `| glob \`name\`, matched branch ends in \`N.N.N\` | version branch: the branch name is the version (T348) |` to the Model table, and a closing paragraph under **Resolution by branch type**: `A **version branch** (T348) resolves to the version in its name: no bump, \`--pre-release\` numbering over that core, refusal once the final exists, and a warning (never an error) when a higher release is already in HEAD's history. It is recognised by name shape under a glob entry, so one \`release/*\` entry serves both lines and version branches.`

- [ ] **Step 5: Rewrite the guide section** — replace the body of "## When the branch is the version" with the automated flow (`heraut release` on `release/7.8.0` → `v7.8.0`; `--pre-release rc` before the final; `heraut release --set-version 7.8.0 --set-build-id 158404` for repeat builds; the refusal after the final; the ordering warning; mixed namespace example) and delete the sentence "Automating this as its own branch type is tracked as T348".

- [ ] **Step 6: Fix the design spec** — in the behaviour table replace the `--set-build-id N` row with `| \`--set-version 7.8.0 --set-build-id N\` | \`v7.8.0+N\` (SemVer build metadata; \`--set-build-id\` requires \`--set-version\`) |`, and change `Status` to `Implemented`.

- [ ] **Step 7: Roadmap** — flip T348 `[ ]` → `[x]` (drop the ✦ "needs design" wording from the heading) and add the note: version branches recognised by name shape under glob entries (`DeriveBranchVersion`, `BranchVersion`), `SetFixedVersion` in the semver resolver, ordering and `--set-version` mismatch warnings, `version current` on the branch's own tag, ADR-0065 amended instead of a new ADR, deviations: `--set-build-id` still requires `--set-version` (spec table corrected), no check that commits imply a larger bump, no commit-presence check for repeat `-rc.N`. Set the Phase 62 row in the status table to `Done — T348`.

- [ ] **Step 8: Verify and commit**

Run: `mise exec -- go test ./... && mise exec -- hk check`
Expected: PASS (schema fixtures under `testdata/config/valid/` still validate; add `testdata/config/valid/semver_version_branches.yml` only if the schema test enumerates strategies per branch type — it does not, so none is needed).

```bash
git add schema.json docs && mise exec -- git commit -m "docs: document version branches and close T348

Schema description, sample, Spec 02, ADR-0065 amendment, guide and
roadmap note for release/N.N.N branches.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

## Self-review (done while writing)

- **Spec coverage:** declaration by name shape (T1, T2); plain re-release error with hint (T3); `--pre-release` numbering and rc-after-final refusal (T3); `+build` via `--set-version` (T4, T5); `--set-version` mismatch warning (T4); ordering warning only when merged (T3, T4); `version next`/`version current` (T4, T5); `semver` only, exact names excluded (T2); schema/sample/specs/ADR/guide (T6); tests at unit, resolver, real-git, e2e (T1-T5). The spec's `--set-build-id N` row is corrected in T6 (the CLI requires `--set-version` with it).
- **Placeholders:** none; the only conditional wording is the explicit fallback to existing helper names in Task 5 and the queue fix-up in Task 4 Step 4, both naming the exact action.
- **Type consistency:** `BranchVersion`, `BranchMatch.Version`, `DeriveBranchVersion`, `SetFixedVersion`, `fixedCore`, `applyBranchRule`, `versionBranchMismatch`, `currentVersionBranchTag`, `semver.CoreOf` are used with identical spellings across tasks.
