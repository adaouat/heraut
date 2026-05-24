# Spec 05 — Generators and Platforms

Generators produce changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.platforms`.

## Generators

Three generators are supported: `git-cliff`, `communique`, `cocogitto`. A project can
use different generators for `changelog` and `release.notes`.

| Generator   | Strengths                                                        | Limits                                                       |
|-------------|------------------------------------------------------------------|--------------------------------------------------------------|
| `git-cliff` | Embedded opinionated default; deep-merged TOML overrides; labels new commits with `--tag <version>` | TOML config only                                            |
| `communique`| AI-assisted release notes from commit history                    | Requires a full config file; no embedded default              |
| `cocogitto` | Native conventional-commit grouping; rich Tera templating         | Cannot label unreleased commits with a target version        |

### git-cliff

```yaml
changelog:
  generator: git-cliff
  config: .config/cliff/cliff.toml   # optional partial override
  output: CHANGELOG.md               # optional, defaults to CHANGELOG.md
  tag_pattern: "dev/*"               # required for prefixed-tag strategies
```

heraut ships two embedded `cliff.toml` defaults (see
[ADR-0010](../adr/0010-embedded-cliff-toml-default.md)):

- **Changelog variant** — includes a stats block, version header, and the full Conventional
  Commits taxonomy
- **Release-notes variant** — no header, no stats; just the body suitable for the
  release page

When `config:` is set in `.heraut.yml`, heraut deep-merges it with the embedded default at
runtime — you only need to override what differs. Inspect the effective merged TOML with:

```
heraut cliff changelog
heraut cliff release-notes
```

**Invocation**:

```
git-cliff --config <merged-tmp.toml> [--tag-pattern <pattern>] [--tag <version>] [--output <file>] [--unreleased]
```

- `--config` points at a temp file containing the merged TOML (cleaned up after run)
- `--tag-pattern` is set from `tag_pattern:` when configured
- `--tag` is set to the resolved version, so unreleased commits get the new version's
  heading in `CHANGELOG.md`
- `--output` is set for changelog mode (writes to `output:`); omitted for release-notes
  mode (stdout is captured)

### communique

```yaml
release:
  notes:
    generator: communique
    config: communique.toml   # required for communique
```

Simple wrapper. heraut does not embed any default — `config:` must point at a file the
user supplies.

**Invocation**:

```
communique generate --config <file> <tag>
```

stdout is captured and used as the release notes content.

### cocogitto

```yaml
release:
  notes:
    generator: cocogitto
    config: cog.toml        # optional
    output: CHANGELOG.md    # optional (changelog mode only; written by heraut from stdout)
    template: my.tera       # optional custom Tera template
```

**Four config-path combinations**:

| `config:`  | `template:` | Effective behaviour                                                                          |
|------------|-------------|----------------------------------------------------------------------------------------------|
| _(none)_   | _(none)_    | embedded `cog.toml` + embedded Tera template (full opinionated defaults)                     |
| _(none)_   | `my.tera`   | embedded `cog.toml` + user's Tera template                                                   |
| `cog.toml` | _(none)_    | user's `cog.toml`, no `-t` flag (cog uses the template referenced in `cog.toml` or its own default) |
| `cog.toml` | `my.tera`   | user's `cog.toml` + user's Tera template                                                     |

The embedded `cog.toml` sets `tag_prefix = "v"`, `from_latest_tag = false`,
`ignore_merge_commits = true`, and maps conventional commit types (feat, fix, refactor,
docs, perf) to clean group titles while silencing chore/ci/build/test/style. The
embedded Tera templates produce clean markdown without commit hashes or author names,
with breaking changes marked as `**[BREAKING]**`.

**Invocation**:

- Changelog mode (full history): `cog [--config <path>] changelog [-t <template.tera>]`
- Release notes mode (single release): `cog [--config <path>] changelog [-t <template.tera>] --at <tag>`

`--config` is a **global** flag for the `cog` binary (must precede the subcommand);
`-t` is a `changelog` subcommand flag.

cocogitto always writes to stdout; there is no `--output` flag. When `output:` is set,
heraut captures stdout and writes the file itself.

**Differences from git-cliff**:

| Aspect                                  | git-cliff                                      | cocogitto                                  |
|-----------------------------------------|------------------------------------------------|--------------------------------------------|
| Output file                             | `--output` flag (written by git-cliff)         | stdout redirect (written by heraut)        |
| Embedded config                         | TOML partial-override, deep-merged at runtime  | TOML + Tera, written as temp files         |
| Version label for unreleased commits    | `--tag <version>`                              | not supported                              |
| Tag pattern                             | `--tag-pattern <regex>`                        | not supported                              |

**Known limitation — changelog mode**: heraut creates the git tag *after* committing the
changelog, so when cocogitto generates the full `CHANGELOG.md`, the new version's tag
does not yet exist in the repository. As a result, the new commits appear under an
"Unreleased" section rather than under the version heading. Teams that require a
correctly versioned heading in `CHANGELOG.md` should use `git-cliff`, which supports
`--tag <version>` to label unreleased commits with the target version.

**`tag_pattern:` field**: not used by cocogitto. Setting `tag_pattern` when
`generator: cocogitto` has no effect. For prefixed tag strategies, heraut passes the
exact resolved tag via `--at` in release notes mode. Full changelog mode scans all tags.

### No generator

Omitting `changelog` or `release.notes` skips that output. The release is still
created on the platforms (if `release.platforms` is configured).

## Generator interface

All generators implement `port.Generator`:

```go
type Generator interface {
    Check() error                                  // binary in PATH
    Validate() error                               // user config files exist if specified
    Generate(tag string) (string, error)           // run the binary, return stdout
}
```

`Validate()` is called by `heraut check config` and `heraut check cliff` and before
the pipeline runs. For generators with no config-file dependency (e.g. git-cliff with
only embedded defaults), `Validate()` returns `nil`.

## Platforms

Two platforms are supported: `github` (via `gh`) and `gitlab` (via `glab`). A release
can be published to one or both — `release.platforms` is a list.

### GitHub

```yaml
release:
  platforms:
    - platform: github
      repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
      token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

**Invocation**:

```
gh release create <tag> --notes <notes> [--draft] [--prerelease] --repo <repository>
gh release upload <tag> <file> --repo <repository>     # per asset, after release is created
```

- **Repository** resolution: `cfg.Repository` → `$GITHUB_REPOSITORY` → error
- **Token** is read from `$<TokenEnv>` (default `GH_TOKEN`); `gh` picks it up
  automatically from the environment
- **Asset upload** resolves each glob against the working directory; non-matching globs
  fail the run with an actionable error
- **Release URL**: `https://github.com/<repo>/releases/tag/<tag>` — used in the
  post-release log line

### GitLab

```yaml
release:
  platforms:
    - platform: gitlab
      project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
      token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
      catalog: false              # optional, set true for a CI/CD Catalog release
      assets:
        - dist/myapp_*
```

**Invocation**:

```
glab release create <tag> --notes <notes> [--publish-to-catalog] -R <project>
glab release upload-asset <tag> <file> -R <project>    # per asset, after release is created
```

- **Project** resolution: `cfg.Project` → `$CI_PROJECT_PATH` → error
- **Token** is read from `$<TokenEnv>` (default `GITLAB_TOKEN`); `glab` picks it up
  automatically from the environment
- **Catalog**: `catalog: true` adds `--publish-to-catalog` to enable a GitLab CI/CD
  Catalog release
- **Release URL**: `<gitlab-base>/<project>/-/releases/<tag>`

### Platform interface

All platforms implement `port.Platform`:

```go
type Platform interface {
    Name() string                                  // "github" or "gitlab"
    ReleaseURL(tag string) string                  // canonical URL
    Check() error                                  // binary + token + project/repo resolved
    CreateRelease(tag, notes string) error         // create the release
    HasAssets() bool                               // true if cfg.Assets is non-empty
    UploadAssets(tag string) error                 // resolve globs + upload each match
}
```

Both implementations are contract-tested with `MockRunner` — every CLI argument
heraut passes to `gh` and `glab` is asserted in the test suite. Adding a third
platform later means: implement `port.Platform`, add contract tests, register in
`app.BuildPipeline`.

## Generator/platform combinations

heraut does not constrain combinations. Any generator can produce text for any
platform. A common pattern: `cocogitto` for `release.notes` (rich Tera template for
the release page) and `git-cliff` for `changelog` (versioned `CHANGELOG.md` in the
repo, thanks to `--tag <version>`).

## Extensibility

Adding custom platforms or generators requires modifying heraut's source. A formal
plugin system is out of scope (see [Spec 01 — Overview § Boundaries](01-overview.md#boundaries)).
