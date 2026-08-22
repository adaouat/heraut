# Spec 05 — Generators and Platforms

The `native` generator produces changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.targets` (each target referencing a `forges[].name`).

## Generator

`native` is heraut's sole content generator (ADR-0045) — a built-in, zero-external-dependency
renderer driven by `commits` / `rendering` config, with a user-customizable template API
(ADR-0037).

## native

heraut's built-in, zero-external-dependency renderer (ADR-0032 / ADR-0033). It walks git
history, classifies commits per the `commits.types` taxonomy and `rendering.excludes`, and
renders Markdown with internal templates — no `git-cliff` binary required.

```yaml
changelog:
  output: CHANGELOG.md
```

Section labels, order, and heading depth come from `commits.types` and
`commits.types_heading_level`; `rendering.excludes` drops matched commits from the output.
Each commit line credits the **commit author** — `by @<handle>` — resolved from the platform,
independent of any associated pull request; the PR/MR, when present, contributes only its
`in [#N](url)` reference link. When the committer differs from the PR/MR author, the committer
is credited (matching git-cliff). GitHub resolves the handle at no extra cost, riding its existing
batched PR-fetch query (ADR-0039). **GitLab** in `api_mode: graphql` (opt-in) resolves it via a
batched `commits` GraphQL query, and separately inverts a batched `mergeRequests` query into a
`commitSha → MR` map for the `in [!N]` reference plus MR review-metadata; a commit attributable to
no MR (e.g. a squash-with-fast-forward merge, for which GitLab's GraphQL schema exposes no
squashed-commit SHA) renders no ref — a graceful omission, not an error (ADR-0042). GitLab in
`api_mode: rest` (the default) exposes no linked username on its commit payloads, so its `by
@<author>` is rendered from the git author email's local-part instead, falling back to the git
author name when no email is present — the same local rendering Azure DevOps uses, and for the same
reason: an `@handle` should not contain spaces. **Azure DevOps** resolves no identity from a
commit's git author email (no API can map it — confirmed by a live spike, T151), so its
`by @<author>` is rendered from the local git author email's local-part instead — a text
attribution, not a clickable Azure @mention. See
[ADR-0039](../adr/0039-commit-author-attribution.md) and
[ADR-0042](../adr/0042-gitlab-graphql-enrichment.md).

PR/MR number attribution and the "New Contributors" block derive from a unified,
platform-agnostic model (`Author`/`PullRequest`/`Contributor`): a **local tier** always
computes contributors and `first_time` from git author history (one `git log`, available
offline, identical across GitHub/GitLab/Azure DevOps), while a **remote tier** — gated by the
`commits.enrichment_policy` policy — fetches PR/MR metadata (number, URL, title, labels, author
handle) and overlays it onto that local model. Because `first_time` no longer depends on
platform-specific data, the "New Contributors" block is available for all three platforms,
including Azure DevOps. See [ADR-0033](../adr/0033-native-config-model.md),
[ADR-0034](../adr/0034-native-remote-enrichment.md), and
[ADR-0036](../adr/0036-unified-enrichment-model.md).

### User-customizable templates (ADR-0037)

The native generator exposes a public template API with **two entry points**:

- **Inline block overrides** — short Go `text/template` snippets under `rendering.templates.<block>`
  (global), `<driver>.rendering.templates` (per-driver), or per-env. Reformat one block in a
  single line; everything else stays built-in.
- **A full template file** — `<driver>.template: <path>` points at a `.tmpl` file parsed on top of
  the built-ins; it may redefine the document root and/or any block (whole-document control).

```yaml
rendering:
  templates:
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  template: .config/heraut/changelog.tmpl   # optional full template file
  rendering:
    templates:
      header: "# Changelog\n\nAll notable changes.\n"
```

**Overridable blocks:** `header`, `group`, `commit`, `contributor`, `contributors`, `stats`,
`footer`, and the roots `changelog` / `release-notes`. The changelog renders a one-line commit;
the release-notes root wraps the shared `commit` block with indented body/footers. Any other key
under `rendering.templates` is a **config error** (a misspelled block would otherwise be silently
ignored); `schema.json` enumerates the same set for editor autocompletion.

**Template funcs** (safe set, no OS/file/network): `upperFirst`, `date`, `join`, `list`, `indent`,
`trim`.

**Data model.** The root is a `Release` exposing `.Version` `.Tag` `.PreviousTag` `.CompareURL`
`.Date` `.Groups` `.Contributors` `.Stats` `.Heraut`; a `Group` exposes `.Name` `.Commits`; a
`Commit` exposes `.Type` `.Scope` `.Breaking` `.Description` `.Body` `.Hash` `.ShortHash`
`.CommitURL` `.Date` `.Author` `.PR` `.Tickets` `.Footers`; `.PR` (nil when absent) exposes
`.Number` `.URL` `.Title` `.Ref` `.Labels` `.Author` `.CreatedAt` `.MergedAt` `.MergedBy`
`.Approvers` (approvers best-effort: GitHub + Azure, empty on GitLab); `.Heraut` exposes
`.Version` `.URL` `.GeneratedAt`. All `.PR.*` fields are remote-only (empty offline). Field names
are the **experimental-in-v1** public API — additive changes are free.

**Precedence** (lowest → highest): built-in → global `rendering.templates` →
`<driver>.rendering.templates` → per-env → `<driver>.template` file. `rendering.templates` and
`template` are native's own template API (heraut's sole generator — no `generator:` key to set);
each snippet and the file are parse-validated at config load.

### Changelog structure & incremental generation (ADR-0038)

A `native`-managed `CHANGELOG.md` is a **preamble** (free-form content before the first section,
e.g. the `# Changelog` title) followed by **anchored sections**, newest first. Each section is
preceded by a structural HTML comment on its own line:

```
<!-- heraut-release: v0.49.0 -->
```

The anchor carries the release **tag**, is invisible in every Markdown renderer, and is emitted by
the assembly layer — never by a template block — so it is non-overridable and independent of the
customizable `header` block (ADR-0037): reformatting the header can neither remove the anchor nor
change its shape.

**Incremental (default).** Each run renders and enriches only the new release's section (O(1) API
calls) and splices it into the existing file, leaving every other section untouched:

- **Missing or empty file** → bootstrap: build every section from all tags (anchored), enriching
  only the newest. No warning.
- **File with ≥1 anchor** → splice: the new section replaces the top one if its tag matches
  (idempotent re-run), otherwise it is inserted above it. All other sections are preserved
  verbatim, including their historical PR/MR attribution.
- **Non-empty file with no anchors** (produced by another tool, e.g. `git-cliff`, or predating this
  feature) → the run **stops with an error** directing the operator to `--regenerate`; the file is
  left byte-for-byte unchanged.

**Full regeneration (`--regenerate` / `--regenerate-changelog`).** Ignores the existing file,
rebuilds every section from all tags, and **re-enriches all of them** — each section is enriched
independently, so GitHub (batched GraphQL, 50 SHAs/query), GitLab in `api_mode: graphql` (two
batched connection queries — commit authors via `commits`, MR refs via inverted `mergeRequests`,
[ADR-0042](../adr/0042-gitlab-graphql-enrichment.md)), and Azure (one `pullrequestquery` POST) each
batch *within* a release, costing roughly one API call per release (O(releases)). All of these run
over heraut's own `net/http` forge clients — no `gh` or `glab` process is spawned for enrichment
([ADR-0043](../adr/0043-forge-abstraction.md)). The one exception is GitLab in `api_mode: rest`
(the **default**), which resolves MRs with one
`GET /projects/:id/repository/commits/:sha/merge_requests` **per commit**: a full regeneration
there costs O(commits), so prefer `api_mode: graphql` (or an incremental run) on a long history.
The changelog pipeline step itself no longer carries a dedicated GitLab full-regeneration warning;
the only warning it emits is the degraded note raised when an enrichment fetch actually fails.
This is the required one-time step when migrating a changelog onto `native` (or repairing a
previously-anchorless file) — see [ADR-0038](../adr/0038-incremental-changelog.md) for the full
migration story, including the `regenerate_changelog` `workflow_dispatch` input heraut's own CI
uses for its own migration.

### forges — explicit metadata forge (ADR-0043)

```yaml
forges:
  - name: azure-devops
    platform: azure_devops           # github | gitlab | azure_devops
    project: my-org/my-project       # azure_devops (required: "organization/project", matching
                                      # git-cliff's own azure_devops "owner" shape) / gitlab
                                      # (required: namespace[/subgroup]/repo)
    repository: my-repo              # azure_devops (required) / github (required: owner/repo)
    token_env: AZURE_DEVOPS_TOKEN     # optional override
    base_url: https://git.example.com  # optional host override; all types (GHES /
                                        # self-managed GitLab / on-prem Azure). Absolute http(s) URL.

commits:
  enrichment_forge: azure-devops     # names the forges[] entry above as the metadata source
```

Release notes always has a deterministic forge — it is generated per publish target being
published to (`release.targets`, each referencing a `forges[].name`). The changelog has no
such anchor: it falls back through ambient CI detection, then `git remote get-url origin`,
then the sole configured `forges` entry (if exactly one), then `nil` (bare hashes). An
explicit `forges` entry named by `commits.enrichment_forge` is consumed ahead of that
fallback chain. `forges` entries are connection/identity only — they never grant publish
capability on their own; heraut never publishes a release through a `forges` entry, it only
tells the active generator where to source PR/MR metadata and commit/PR link shapes.
Publishing is a separate concern (`release.targets`, each referencing a forge by name).
Valid as the enrichment source for `native` (originally
introduced for `changelog.remote` by ADR-0026/ADR-0040; unified into the top-level `forges:`
list by ADR-0043).

### Auto-detection and self-hosted hosts

When no explicit `forges:` entry supplies a field, the fallback chain above resolves it from
three sources, in order (`internal/forge/resolve.go`'s `resolveAuto`): the ambient CI environment
(`GITLAB_CI` / `GITHUB_ACTIONS` / `TF_BUILD` markers pin the type unambiguously), then
`git remote get-url origin`, then — when neither pins a type — the ambient *default* token env
vars (`GITLAB_TOKEN`, `GITHUB_TOKEN`, `AZURE_DEVOPS_TOKEN`). Origin-based detection recognises
only the **public** hosts — `github.com`, `gitlab.com`, `dev.azure.com` — parsed from both the
SSH and HTTPS remote forms. A **self-hosted** GitHub Enterprise or GitLab host (or any other host
outside that list) does not match, so outside CI a self-hosted project falls through to the
token-env step. That step's outcome depends on how many of the three token vars are set:

- **Exactly one set** — auto-detection resolves *a* forge of that token's type, but with
  `Host` set to the type's **public** default host (`https://gitlab.com` for `GITLAB_TOKEN`,
  and so on) and an **empty** `Project`, since a bare token env var carries no host or project
  path. For a self-hosted user this is worse than no forge: requests go to the public host with
  an empty project segment, and a token minted for the user's own instance is sent to
  `gitlab.com`/`github.com` instead. This is pre-existing behaviour of the token-env fallback,
  not something this phase introduced — it is exactly why the explicit `forges:` remedy below
  matters for a self-hosted setup.
- **Two or more set** — ambiguous; `Resolve` returns `forge.ErrAmbiguousForge` and the run fails.
- **None set** — auto-detection resolves **no forge** for that project, same as if the token-env
  step didn't exist.

What happens next depends on `commits.enrichment_policy` and which of the three outcomes above
occurred:

- **`native`** — the single-token outcome does *not* short-circuit to "no forge": a real
  `port.Forge` is constructed against the public host with an empty project, so enrichment is
  attempted and fails. GitLab's `enrichREST` (`internal/forge/gitlab/rest.go`), for example,
  builds `https://gitlab.com/api/v4/projects//repository/commits/<sha>/merge_requests` (empty
  project segment) and gets a 404. Under `optional` (the default), `enrichForRelease`
  (`internal/generators/native/enrich.go`) catches that error, drops enrichment, and sets
  `Degraded()` with the wrapped HTTP error as the reason — commit lines render with no `by @`
  handle and no `in [#N]` reference, but the degraded warning names an HTTP failure, not a
  missing forge. Under `required`, the run **fails outright** with that same wrapped 404 (not
  the "no forge resolved" message below). `--force` downgrades `required` to the `optional`
  behavior above ([ADR-0041](../adr/0041-remote-metadata-required-enforcement-and-force.md)).
  Only the **zero-token** outcome (nothing resolves at all — no CI marker, no recognised origin,
  no ambient token) reaches the "no forge" behavior: under `optional`, generation proceeds with
  no PR/MR enrichment, silently, with no warning and no `Degraded()` signal (that signal is
  reserved for a *configured* forge whose fetch fails, not for the absence of one); under
  `required`, the run fails with an error explaining that no forge was resolved and naming the
  three ways to supply one — a `forges:` entry, a supported CI environment, or a recognised git
  origin.

The remedy is an explicit `forges:` entry naming the self-hosted `base_url` and the
`project`/`repository` path, since nothing else can supply them:

```yaml
forges:
  - name: gitlab-internal
    platform: gitlab
    base_url: https://gitlab.example.com
    project: group/subgroup/project

commits:
  enrichment_forge: gitlab-internal
```

Once declared, that entry's still-unset fields (token, API URL) continue to fill from CI or a
type default as usual — only the fields self-hosted detection cannot infer (`base_url`,
`project`/`repository`) must be given explicitly.

Azure DevOps repository URLs are structurally different from GitHub/GitLab: the
repository root inserts `/_git/` between the project and repository segments
(`https://dev.azure.com/{organization}/{project}/_git/{repository}`). Commit, PR, and
compare links are all supported. The compare link is query-string based with two
separately-prefixed refs (`?baseVersion=GT{old}&targetVersion=GT{new}`), which doesn't fit
the single-prefix `{prefix}{old}..{new}` shape GitHub/GitLab use — the embedded changelog
template substitutes an additional `HERAUT_COMPARE_URL_MIDDLE` var between `{old}` and
`{new}` to support it (see [ADR-0022's update](../adr/0022-fat-injection-thin-templates.md)).
GitHub/GitLab never set it, so their output is unchanged.

### Omitting changelog or release notes

Omitting `changelog` or `release.notes` from `.heraut.yml` skips that output. The release is
still created on the configured platforms (an explicit `release.targets` entry, or the single
resolved forge with default options when `release.targets` is omitted).

## Generator interface

The generator implements `port.Generator`:

```go
type Generator interface {
    Check() error                                            // no-op: no external binary to verify
    Validate() error                                         // no-op: no external config file to verify
    Generate(tag string, link *port.LinkContext) (string, error) // render in-process, return the rendered string
}
```

`Validate()` is called by `heraut check config` and before the pipeline runs.

**Per-platform link resolution**: when a release targets **more than one** platform,
heraut regenerates the release notes once per platform and passes that platform's
`link` context (host, owner, repo, type) so commit/PR/MR links resolve to the correct
host and path shape (see [ADR-0021](../adr/0021-per-platform-release-notes.md)). A
single-platform release passes `nil`, and the generator falls through to ambient-CI
link detection.

## Platforms

Two platforms are supported: `github` (via `gh`) and `gitlab` (via `glab`). A release
can be published to one or both — `release.targets` is a list, each entry referencing a
`forges[].name` ([ADR-0044](../adr/0044-publishing-config-unification.md)). Connection
fields (`repository`/`project`, `token_env`, `base_url`) live on the `forges:` entry;
publish behavior (`draft`, `prerelease`, `assets`) lives on the `release.targets` entry.

### GitHub

```yaml
forges:
  - name: github
    platform: github
    repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
    token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
    base_url: https://github.com  # optional, defaults to https://github.com

release:
  targets:
    - forge: github              # optional when exactly one forge is configured
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
forges:
  - name: gitlab
    platform: gitlab
    project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
    token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
    base_url: https://gitlab.com  # optional, defaults to https://gitlab.com

release:
  targets:
    - forge: gitlab               # optional when exactly one forge is configured
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

Because `forges` is a list, multiple entries of the *same* platform type are
supported — e.g. publishing to both `gitlab.com` and a self-hosted
`gitlab.example.com`:

```yaml
forges:
  - name: gitlab-com
    platform: gitlab
    project: acme/widget-catalog
  - name: gitlab-internal
    platform: gitlab
    project: acme/widget
    base_url: https://gitlab.example.com

release:
  targets:
    - forge: gitlab-com
    - forge: gitlab-internal
```

Each `forges[]` entry's `name` must be unique and is referenced by its `release.targets`
entry; the name is used to label that entry's row in `heraut check runtime`'s Platforms
section and in any per-entry error message.

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

## Extensibility

Adding custom platforms or generators requires modifying heraut's source. A formal
plugin system is out of scope (see [Spec 01 — Overview § Boundaries](01-overview.md#boundaries)).
