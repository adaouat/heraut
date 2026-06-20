# T113: Auto-inject `[remote.github]` / `[remote.gitlab]` into effective git-cliff config

## Context

git-cliff's `[remote.github]` and `[remote.gitlab]` sections enable PR/MR metadata
fetching (authors, PR numbers). Today the user must manually add and fill in that section
in a custom git-cliff override config. Since heraut already holds all the needed info in
the `LinkContext` (owner, repo, token), it can inject the section automatically into the
effective TOML it writes to the temp file — eliminating the need for any custom git-cliff
config to get PR metadata.

What heraut already injects for git-cliff (via `linkEnv()`):
- `HERAUT_REMOTE_URL`, `HERAUT_COMMIT_URL`, etc. — link template vars
- `GITHUB_TOKEN` / `GITLAB_TOKEN` — API auth

What's missing:
- `[remote.github]` or `[remote.gitlab]` TOML section with `owner` and `repo`
- `GITHUB_REPO` / `GITLAB_REPO` env vars (belt-and-suspenders, for users with custom configs)

## Implementation

### 1. New `injectRemote` function in `generator.go`

Follows the same pattern as `injectLinkParsers` and `injectHeadingPostprocessor`:
parse the merged TOML, add the remote section if not already present, marshal back.

```go
// injectRemote appends [remote.github] or [remote.gitlab] to the merged TOML when the
// link context carries owner and repo, so git-cliff can fetch PR/MR metadata without
// requiring a custom config. Skipped when:
//   - lc is nil or owner/repo are empty (ambient context or no platform info)
//   - the user's override config already declares [remote.<platform>]
func injectRemote(merged string, lc *port.LinkContext) (string, error) {
    if lc == nil || lc.Owner == "" || lc.Repo == "" {
        return merged, nil
    }
    var doc map[string]any
    if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
        return "", fmt.Errorf("parsing merged TOML for remote injection: %w", err)
    }
    remote, _ := doc["remote"].(map[string]any)
    if remote != nil {
        if _, exists := remote[lc.Platform]; exists {
            return merged, nil  // user already declared [remote.<platform>]
        }
    }
    if remote == nil {
        remote = make(map[string]any)
        doc["remote"] = remote
    }
    remote[lc.Platform] = map[string]any{"owner": lc.Owner, "repo": lc.Repo}
    out, err := toml.Marshal(doc)
    if err != nil {
        return "", fmt.Errorf("marshalling TOML after remote injection: %w", err)
    }
    return string(out), nil
}
```

### 2. Thread `lc` into `prepareConfig`

`prepareConfig()` → `prepareConfig(lc *port.LinkContext)`.

Inside, after `effectiveConfig()` builds the merged TOML, call `injectRemote`:

```go
merged, err = injectRemote(merged, lc)
```

This is the last post-processing step, after link parsers and heading postprocessor.

Callers:
- `Generate(tag, lc)` → `prepareConfig(lc)` — passes the platform link context
- `CheckCliff()` → `prepareConfig(nil)` — no context, injectRemote is a no-op

`EffectiveChangelogConfig()` / `EffectiveReleaseNotesConfig()` call `effectiveConfig()`
directly (not `prepareConfig()`), so they do NOT show the injected remote section — this
is acceptable; the `heraut cliff` diagnostic shows the base effective config.

### 3. Also inject `GITHUB_REPO` / `GITLAB_REPO` in `linkEnv()`

Belt-and-suspenders: helps users with custom git-cliff configs that have `[remote.*]`
but rely on env vars for `owner`/`repo`. Same guard pattern as the token injection.

```go
// After the existing token injection block:
if lc.Owner != "" && lc.Repo != "" {
    repoVar := "GITHUB_REPO"
    if lc.Platform == "gitlab" {
        repoVar = "GITLAB_REPO"
    }
    if os.Getenv(repoVar) == "" {
        env = append(env, repoVar+"="+lc.Owner+"/"+lc.Repo)
    }
}
```

Only when `Owner` and `Repo` are both non-empty (platform context). Ambient contexts
(Owner/Repo empty, BaseURL is full URL) are skipped.

### 4. Update embedded TOML comments

Both `cliff.changelog.toml` and `cliff.release-notes.toml` — replace the "Uncomment
and fill in" block with a note that heraut handles this automatically:

```toml
# PR/MR metadata fetching (authors, PR numbers) is enabled automatically by heraut
# when owner/repo are known from your release.platforms config. No manual config needed.
# To override or extend, add [remote.github] or [remote.gitlab] to your cliff config.
```

### 5. Tests

**`linkenv_internal_test.go`** — update `TestLinkEnv` expected slices to include
`GITHUB_REPO=acme/widget` / `GITLAB_REPO=group/sub/proj` (and clear those env vars in
test setup alongside the existing `GITHUB_TOKEN`/`GITLAB_TOKEN` clearing). Add cases
verifying ambient context (Owner/Repo empty) produces no `*_REPO` entry.

Add `TestLinkEnv_RepoInjection` following the same table structure as
`TestLinkEnv_TokenInjection` to cover: injected when not set, not injected when already
set, not injected when Owner or Repo is empty.

**New `remote_internal_test.go`** (package `gitcliff`) — table-driven tests for
`injectRemote`:
- GitHub platform with owner/repo → `[remote.github]` section appears
- GitLab with nested group → `[remote.gitlab]` with `owner = "group/sub"`, `repo = "proj"`
- `lc == nil` → TOML unchanged
- Owner or Repo empty → TOML unchanged
- User already has `[remote.github]` in TOML → not overridden

### 6. Roadmap

Add `[x] T113` to `docs/tasks/roadmap.md`.

## Files modified

| File | Change |
|---|---|
| `internal/generators/gitcliff/generator.go` | `injectRemote`, `prepareConfig(lc)`, `Generate` → `prepareConfig(lc)`, `CheckCliff` → `prepareConfig(nil)`, `linkEnv` repo injection |
| `internal/generators/gitcliff/cliff.changelog.toml` | comment update |
| `internal/generators/gitcliff/cliff.release-notes.toml` | comment update |
| `internal/generators/gitcliff/linkenv_internal_test.go` | update existing + new repo injection cases |
| `internal/generators/gitcliff/remote_internal_test.go` | new file — `injectRemote` unit tests |
| `docs/tasks/roadmap.md` | T113 entry |

## Verification

```bash
go test ./internal/generators/gitcliff/...   # all new cases pass
go test ./...                                 # full suite green
```

Manual: set `remote_metadata: optional`, run `heraut release --dry-run` with debug
logging — confirm git-cliff's temp config contains `[remote.github]` or `[remote.gitlab]`
with the correct `owner`/`repo` values.
