# Plan: Versioned `schema.json` URL per heraut release

## Context

`heraut init` emits a `# yaml-language-server: $schema=<url>` header in every generated
`.heraut.yml`. Today that URL is the hardcoded constant
`https://raw.githubusercontent.com/adaouat/heraut/main/schema.json` (in
`internal/scaffold/generate.go:12`). As the config schema evolves, a user whose config
was written with heraut v1.0.0 will have their IDE validate against whatever the `main`
schema says today — potentially flagging fields that were valid when the config was
generated, or silently accepting fields that no longer exist.

**Goal:** each released binary writes a pinned, version-specific schema URL so configs
validate against the schema that was current when they were generated, matching the
Biome-style pattern.

---

## Approach: Thread version through constructors + git tags as hosting

### Why git tags?

`https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json` is served by
GitHub for free, is immutable once the tag is pushed, requires zero extra infrastructure,
and works with `yaml-language-server` without HTTP redirects. No GitHub Pages, no release
asset upload, no `schemas/` accumulation directory needed.

### Why thread through constructors?

The build-time `Version` variable lives in `cmd/heraut/main.go`. It needs to reach
`scaffold.GenerateYAML`. The clean path is explicit parameter threading:

```
main.Version  →  cmd.NewRootCmd(version)  →  cmd.NewInitCmd(version)
             →  scaffold.GenerateYAML(answers, version)
             →  buildSchemaURL(version)
```

This keeps all state explicit, makes the behavior trivially testable (pass any version
string), and avoids package-level mutable globals.

---

## Pros / Cons

### Pros
- **Zero hosting cost** — schema lives in the repo, pinned by the git tag
- **Correct semantics** — user's `.heraut.yml` always validates against the schema it
  was written for, not whatever `main` is today
- **Dev builds stay sane** — `version == "dev"` falls back to `main`, so CI / local
  builds always get the latest schema
- **Explicit dependency** — version flows through function parameters, easy to test and
  reason about
- **Minimal blast radius** — only 4 files change: `main.go`, `root.go`, `init.go`,
  `generate.go`; test fixes are mechanical

### Cons
- **`NewRootCmd` signature change** — any external code (none today, but worth noting)
  calling `NewRootCmd()` with no args will break; all internal tests need a trivial
  `"dev"` argument update
- **Tag must exist before yaml-language-server fetches** — if someone runs `heraut init`
  on a pre-release binary before the tag is pushed, the URL 404s. Mitigated by always
  pushing the tag before distributing binaries (the normal GoReleaser flow)
- **`$id` in `schema.json` stays at `main`** — technically the `$id` should match the
  URL where the file lives; leaving it as `main` is a minor spec violation. Low-risk in
  practice (most tooling ignores `$id` for fetching), but worth a follow-up task
- **Schema accumulates at every tag** — this is not a storage cost (GitHub serves from
  the commit tree), but there's no "delete old schema" mechanism. Intentional: old configs
  should keep validating

---

## Implementation

### 1. `internal/scaffold/generate.go`

Replace the constant with a function; add a `version string` parameter to `GenerateYAML`:

```go
// Before:
const schemaURL = "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"
func GenerateYAML(a Answers) (string, error) { ... }

// After:
const schemaBase = "https://raw.githubusercontent.com/adaouat/heraut/"
const schemaFile = "/schema.json"

func buildSchemaURL(version string) string {
    if version == "" || version == "dev" {
        return schemaBase + "main" + schemaFile
    }
    return schemaBase + version + schemaFile
}

func GenerateYAML(a Answers, version string) (string, error) {
    header := "# yaml-language-server: $schema=" + buildSchemaURL(version) + "\n\n"
    ...
}
```

### 2. `internal/cmd/init.go`

Change `NewInitCmd()` to accept a version string; thread it to `GenerateYAML`:

```go
// Before:
func NewInitCmd() *cobra.Command {
    ...
    content, err := scaffold.GenerateYAML(answers)
    ...
}

// After:
func NewInitCmd(version string) *cobra.Command {
    ...
    content, err := scaffold.GenerateYAML(answers, version)
    ...
}
```

### 3. `internal/cmd/root.go`

Pass version into `NewRootCmd` and forward to `NewInitCmd`:

```go
// Before:
func NewRootCmd() *cobra.Command {
    ...
    root.AddCommand(NewInitCmd())
    ...
}

// After:
func NewRootCmd(version string) *cobra.Command {
    ...
    root.AddCommand(NewInitCmd(version))
    ...
}
```

### 4. `cmd/heraut/main.go`

Pass `Version` to `NewRootCmd`:

```go
// Before:
root := cmd.NewRootCmd()

// After:
root := cmd.NewRootCmd(Version)
```

---

## Test updates (mechanical)

| File | Change |
|------|--------|
| `internal/cmd/*_test.go` | `executeRoot(...)` helper calls `NewRootCmd("dev")` |
| `internal/scaffold/generate_test.go` | `scaffold.GenerateYAML(a, "dev")` everywhere; add `TestBuildSchemaURL` table test covering `"dev"`, `""`, `"v1.2.3"`, `"v2.0.0"` |
| `internal/scaffold/wizard_test.go` | No change (RunWizard signature unchanged) |

**`TestBuildSchemaURL` cases (new):**

| version | expected URL |
|---------|--------------|
| `"dev"` | `…/main/schema.json` |
| `""` | `…/main/schema.json` |
| `"v1.2.3"` | `…/v1.2.3/schema.json` |
| `"v2.0.0"` | `…/v2.0.0/schema.json` |

The existing `TestGenerateYAML_SchemaHeader` test should be updated to assert the full
`main` URL (since tests use `"dev"` as the version), making it a stricter check.

---

## Deferred / out of scope

- **`schema.json` `$id` update per release** — The `$id` field should ideally match the
  versioned URL. Options: (a) update it in the GoReleaser config via a `before:hooks`
  step; (b) accept the minor spec deviation. Left for a follow-up task.
- **`schemas/` directory** — Biome also maintains a `schemas/` dir with copies for
  discoverability. Not needed for functional correctness; raw git tags suffice.
- **CDN / custom domain** — If `raw.githubusercontent.com` ever becomes unreliable,
  a redirect layer could be added. Out of scope.

---

## Verification

```bash
# Unit tests
go test ./internal/scaffold/... -run TestBuildSchemaURL -v
go test ./internal/scaffold/... -run TestGenerateYAML -v
go test ./internal/cmd/...      # init tests including schema header check
go test ./...                   # full suite

# Manual check
mise run build
./heraut init --defaults --config /tmp/test.yml
head -3 /tmp/test.yml
# → # yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/dev/schema.json
# (or main since "dev" maps to main)

# Simulate a release build
go build -ldflags="-X main.Version=v1.2.3" ./cmd/heraut/
./heraut init --defaults --config /tmp/test-v1.yml
head -3 /tmp/test-v1.yml
# → # yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json
```

---

## Roadmap task

This work should be added to `docs/tasks/roadmap.md` as a new task (T26 or similar),
placed after T25, marked `[ ]`, with this plan as the design reference.
