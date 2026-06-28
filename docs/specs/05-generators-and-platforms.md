# Spec 05 — Generators and Platforms

Generators produce changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.platforms`.

## Generators

Two generators are supported: `git-cliff`, `communique`. A project can
use different generators for `changelog` and `release.notes`.

| Generator   | Strengths                                                        | Limits                                                       |
|-------------|------------------------------------------------------------------|--------------------------------------------------------------|
| `git-cliff` | Embedded opinionated default; deep-merged TOML overrides; labels new commits with `--tag <version>` | TOML config only                                            |
| `communique`| AI-assisted release notes from commit history                    | Requires a full config file; no embedded default              |

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

**Invocation** (changelog mode):

```
git-cliff --config <merged-tmp.toml> --tag <version> [--tag-pattern <pattern>] --output <file>
```

**Invocation** (release-notes mode):

```
git-cliff --config <merged-tmp.toml> --tag <version> --latest [--tag-pattern <pattern>]
```

- `--config` points at a temp file containing the merged TOML (cleaned up after run)
- `--tag` is always set to the resolved version, so unreleased commits get the new
  version's heading in `CHANGELOG.md`
- No range flag in changelog mode — git-cliff processes the full commit history so that
  `CHANGELOG.md` always contains every release, not just the current one
- `--latest` in release-notes mode — the tag is already pushed by the time release notes
  are generated (step 6 of the release pipeline), so `--unreleased` would return nothing;
  `--latest` returns the commits in the tag just created
- `--tag-pattern` is set from `tag_pattern:` when configured
- `--output` is set for changelog mode (writes to `output:`); omitted for release-notes
  mode (stdout is captured)

#### Remote metadata (PR/author enrichment)

git-cliff's `[remote.*]` config sections enable PR/MR metadata fetching (author handle,
PR number) via the platform API. heraut auto-injects this for both changelog and
release-notes generation whenever owner/repo are known: `[remote.github]` /
`[remote.gitlab]` is appended to the effective merged TOML, and `GITHUB_REPO` /
`GITHUB_TOKEN` (or `GITLAB_REPO` / `GITLAB_TOKEN`) are set on the git-cliff subprocess
environment when not already present — no manual git-cliff config is needed. Owner/repo
are derived from whichever `release.platforms` entry the generation step is resolving
against (see [`changelogLinkContext`/`platformLinkContext`](../adr/0022-fat-injection-thin-templates.md)
for the resolution order). Whether the fetch itself is attempted at all (vs. skipped or
gracefully degraded) is governed separately by [`commits.remote_metadata`](../adr/0023-remote-metadata-policy.md).

##### `changelog.remote` — explicit metadata remote (ADR-0026)

```yaml
changelog:
  generator: git-cliff
  remote:
    type: azure_devops              # github | gitlab | azure_devops
    project: my-org/my-project      # azure_devops (required: "organization/project", matching
                                     # git-cliff's own azure_devops "owner" shape) / gitlab
                                     # (required: namespace[/subgroup]/repo)
    repository: my-repo             # azure_devops (required) / github (required: owner/repo)
    token_env: AZURE_DEVOPS_TOKEN    # optional override
    api_url: https://dev.azure.com  # optional, Azure DevOps Server (on-prem) only
```

Release notes always has a deterministic remote — it is generated per platform being
published to. The changelog has no such anchor: it falls back through ambient CI
detection, then the sole configured platform (if exactly one), then `nil` (bare hashes).
`changelog.remote` fills that gap with an explicit, type-discriminated, metadata-only
block, consumed ahead of that fallback chain. Unlike `release.platforms`, it never grants
publish capability — heraut never publishes a release through this block, it only tells
git-cliff/heraut where to source PR/MR metadata and commit/PR link shapes. `git-cliff`
only; setting it on `release.notes` (which already resolves this from
`release.platforms`) is a config error.

Azure DevOps repository URLs are structurally different from GitHub/GitLab: the
repository root inserts `/_git/` between the project and repository segments
(`https://dev.azure.com/{organization}/{project}/_git/{repository}`). Commit, PR, and
compare links are all supported. The compare link is query-string based with two
separately-prefixed refs (`?baseVersion=GT{old}&targetVersion=GT{new}`), which doesn't fit
the single-prefix `{prefix}{old}..{new}` shape GitHub/GitLab use — the embedded changelog
template substitutes an additional `HERAUT_COMPARE_URL_MIDDLE` var between `{old}` and
`{new}` to support it (see [ADR-0022's update](../adr/0022-fat-injection-thin-templates.md)).
GitHub/GitLab never set it, so their output is unchanged.

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

**Known limitation — multi-platform links**: communique is opaque to heraut. Link
resolution lives entirely inside the user's `communique.toml`; heraut has no template
surface to inject per-platform context, so the link context heraut passes to `Generate`
(see [Generator interface](#generator-interface)) is **ignored** by the communique
generator. Consequence: a release published to **more than one** platform (e.g. GitHub +
GitLab) gets **identical** release notes — and identical links — on every platform.
communique cannot tailor links to each platform's host or path shape. This is a known,
accepted scope boundary, **not a bug**. Teams that need per-platform-flavored links across
multiple platforms should use `git-cliff`
(see [ADR-0021](../adr/0021-per-platform-release-notes.md)).

### No generator

Omitting `changelog` or `release.notes` skips that output. The release is still
created on the platforms (if `release.platforms` is configured).

## Generator interface

All generators implement `port.Generator`:

```go
type Generator interface {
    Check() error                                            // binary in PATH
    Validate() error                                         // user config files exist if specified
    Generate(tag string, link *port.LinkContext) (string, error) // run the binary, return stdout
}
```

`Validate()` is called by `heraut check config` and `heraut check cliff` and before
the pipeline runs. For generators with no config-file dependency (e.g. git-cliff with
only embedded defaults), `Validate()` returns `nil`.

**Per-platform link resolution**: when a release targets **more than one** platform,
heraut regenerates the release notes once per platform and passes that platform's
`link` context (host, owner, repo, type) so commit/PR/MR links resolve to the correct
host and path shape (see [ADR-0021](../adr/0021-per-platform-release-notes.md)).
`git-cliff` consumes this context. A single-platform release passes
`nil`, and the generator falls through to ambient-CI link detection — today's unchanged
behaviour. **communique does not consume the context** (see its section above).

## Platforms

Two platforms are supported: `github` (via `gh`) and `gitlab` (via `glab`). A release
can be published to one or both — `release.platforms` is a list.

### GitHub

```yaml
release:
  platforms:
    - platform: github
      name: github               # required, must be unique within this platforms list
      repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
      token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
      base_url: https://github.com  # optional, defaults to https://github.com
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

**Invocation**:

```
gh release create <tag> --notes <notes> --repo <repository> [--draft] [--prerelease]
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
      name: gitlab               # required, must be unique within this platforms list
      project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
      token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
      base_url: https://gitlab.com  # optional, defaults to https://gitlab.com
      assets:
        - dist/myapp_*
```

**Invocation**:

```
glab release create <tag> --notes <notes> -R <project>
glab release upload <tag> --use-package-registry -R <project> <file>...   # all assets, after release is created
```

- **Project** resolution: `cfg.Project` → `$CI_PROJECT_PATH` → error
- **Token** is read from `$<TokenEnv>` (default `GITLAB_TOKEN`); `glab` picks it up
  automatically from the environment
- **Catalog**: GitLab automatically publishes to the CI/CD Catalog when the project is a
  registered catalog resource — heraut has no separate config field or flag for this
- **Release URL**: `<gitlab-base>/<project>/-/releases/<tag>`

### Self-hosted instances and multiple entries of the same type (ADR-0025)

`base_url` may be set to any absolute `http(s)://` URL — including a host other than the
platform's default (`github.com` / `gitlab.com`). When `base_url` is self-hosted, heraut:

- Points `gh`/`glab` at that host: `GITLAB_HOST=<host>` for GitLab, `GH_HOST=<host>` +
  `GH_ENTERPRISE_TOKEN=<token>` for GitHub Enterprise Server.
- Skips CI autologin (`GITHUB_ACTIONS`/`GITLAB_CI`) for that entry — autologin always
  targets the CI runner's own (public) host, never a separately-configured self-hosted
  target — and instead always validates the configured `token_env`.
- Resolves `ReleaseURL`/`LinkContext` against `base_url` instead of the type default.

Because `release.platforms` is a list, multiple entries of the *same* platform type are
supported — e.g. publishing to both `gitlab.com` and a self-hosted
`gitlab.example.com`:

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab-com
      project: acme/widget-catalog
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
```

Each entry's `name` must be unique within its `release.platforms` list and is used to
label that entry's row in `heraut check runtime`'s Platforms section and in any
per-entry error message.

### Platform interface

All platforms implement `port.Platform`:

```go
type Platform interface {
    Name() string                                  // the configured platform entry's name
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
platform. A common pattern: `communique` for `release.notes` (AI-assisted summary for
the release page) and `git-cliff` for `changelog` (versioned `CHANGELOG.md` in the
repo, thanks to `--tag <version>`).

## Extensibility

Adding custom platforms or generators requires modifying heraut's source. A formal
plugin system is out of scope (see [Spec 01 — Overview § Boundaries](01-overview.md#boundaries)).
