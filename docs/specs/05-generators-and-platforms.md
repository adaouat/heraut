# Spec 05 — Generators and Platforms

The `native` generator produces changelog and release-notes text. Platforms publish releases on a
hosting service. They are independent concerns and combined in `.heraut.yml` under
`changelog`, `release.notes`, and `release.targets` (each target referencing a `forges[].name`).

## Generator

`native` is heraut's sole content generator (ADR-0032 / ADR-0033 / ADR-0045) — a built-in,
zero-external-dependency renderer driven by `commits` / `rendering` config, with a
user-customizable template API (ADR-0037). It walks git history, classifies commits per the
`commits.types` taxonomy and `rendering.excludes`, and renders Markdown with internal templates —
no `git-cliff` binary required.

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

### Commit classification (ADR-0033, ADR-0075)

Each commit is grouped by its parsed conventional-commit type, never by its body. A type that is in the
effective `commits.types` set (the built-in defaults merged with the user's entries) gets its own section,
in `order`; the defaults are Features, Bug Fixes, Security, Refactor, Documentation, Performance, Styling,
Testing and Miscellaneous Tasks. A type without a `render` label, an unconfigured type and a
non-conventional message all join `💼 Other`; `revert` commits get `◀️ Revert` unless `revert` is a
configured type. Commits matching `rendering.excludes`, merge commits and `fixup!`/`squash!` commits are
dropped.

### User-customizable templates (ADR-0037, ADR-0048)

The native generator exposes a public template API with **two entry points**:

- **Inline block overrides** — short Go `text/template` snippets under `rendering.templates.<block>`
  (global), `<driver>.rendering.templates` (per-driver), or per-env. Reformat one block in a
  single line; everything else stays built-in.
- **A full template file** — `<driver>.template: <path>` points at a `.tmpl` file parsed on top of
  the built-ins; it may redefine the document root and/or any block (whole-document control).

```yaml
rendering:
  templates:
    commit:
      message: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
      contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  template: .config/heraut/changelog.tmpl   # optional full template file
  rendering:
    templates:
      title: "# Changelog"
      subtitle: "All notable changes."
```

**Overridable blocks:** the document-level trio `title`, `subtitle`, `footer` (no nesting); the
release-cadence blocks nested under `release:` — `release.section`, `release.group`,
`release.contributors`, `release.stats`, `release.footer`; the commit-cadence blocks nested
under `commit:` — `commit.message`, `commit.ticket`, `commit.contributor`; and the roots
`changelog` / `release_notes` (ADR-0059 — release/commit-cadence blocks are namespaced in YAML,
flattening to the dotted names above; the document-level trio and the two roots have no
release/commit-scoped counterpart to disambiguate from, so they stay flat).
`title`/`subtitle`/`footer` fire exactly **once per document**, unlike every other block (which
fires once per rendered release); they execute against `.Heraut`'s own fields directly
(`.Version` `.URL` `.GeneratedAt`), **not** `.Heraut.Version` like `release.footer`/
`release.section` do, since their root isn't a `Release`. `footer` defaults to a heraut credit
line (version + timestamp); `release.footer` — the per-release trailing block, unlike `footer` —
defaults to empty. The changelog renders a one-line commit; the release-notes root wraps the
shared `commit.message` block with indented body/footers.
`commit.ticket` renders one matched ticket link (`commits.tickets`) within a commit line —
`commit.message` calls it once per match, so overriding just `commit.ticket` customizes ticket
rendering without restating the whole commit-line template. Any other key under
`rendering.templates` — including any pre-ADR-0059 flat name (`release_header`, `group`,
`commit`, `ticket`, `contributor`, `contributors`, `stats`, `release_footer`) — is a
**config error** (a misspelled block would otherwise be silently ignored); `schema.json`
enumerates the same set for editor autocompletion.

**Template funcs** (safe set, no OS/file/network): `upperFirst`, `date`, `join`, `list`, `indent`,
`trim`.

**Data model.** The root is a `Release` exposing `.Version` `.Tag` `.PreviousTag` `.CompareURL`
`.Date` `.Groups` `.Contributors` `.Stats` `.Heraut` `.HeadingPrefix` (leading `#`s for the
contributors/stats headings); a `Group` exposes `.Name` `.Commits` `.HeadingPrefix`; a
`Commit` exposes `.Type` `.Scope` `.Breaking` `.Description` `.Subject` (the raw commit
subject line, distinct from `.Description`) `.Body` `.Hash` `.ShortHash`
`.CommitURL` `.Date` `.Author` `.PR` `.Tickets` `.Footers`; each entry of `.Tickets` (what the
`commit.ticket` block receives) exposes `.Text` (the matched ticket text) `.Href` (the resolved
URL); each entry of `.Footers` exposes `.Token` `.Value` (as parsed) and `.Line` — the trailer's
fully-resolved display line, with any matching `rendering.templates.commit.trailers` rule already
applied (a matching `renderer` executed, a matching `hide` entry already dropped from the list);
print `.Line` rather than composing `.Token`/`.Value` yourself, so your template stays correct
however a project has configured `rendering.templates.commit.trailers` — see [Spec 02 §
`rendering.trailers`](02-configuration.md#renderingtrailers-adr-0057) (ADR-0057, ADR-0060);
`.PR` (nil when absent) exposes
`.Number` `.URL` `.Title` `.Ref` `.Labels` `.Author` `.CreatedAt` `.MergedAt` `.MergedBy`
`.Approvers` (approvers best-effort: GitHub + Azure, empty on GitLab); `.Heraut` exposes
`.Version` `.URL` `.GeneratedAt`. All `.PR.*` fields are remote-only (empty offline). Field names
are the **experimental-in-v1** public API — additive changes are free.

`release.contributors` and `release.stats` render **only from the `release_notes` root
template** — a `changelog`-level override of either block has no effect, since the changelog's
own root never invokes them.

**Precedence** (lowest → highest): built-in → global `rendering.templates` →
`<driver>.rendering.templates` → per-env → `<driver>.template` file. `rendering.templates` and
`template` are native's own template API (heraut's sole generator — no `generator:` key to set);
each snippet and the file are parse-validated at config load.

`heraut notes` ([Spec 03](03-commands.md#heraut-notes)) is a third generator mode on the same layers,
with `notes` as the driver: its built-in `release.section` (empty) and `commit.message` (no hash,
author or PR reference) sit below the global `rendering.templates`, then `notes.rendering.templates`,
then `notes.template`. It renders the `changelog` root for one release and writes no file.

### Changelog structure & incremental generation (ADR-0038)

For `semver`/`semver-per-env`, the app layer injects a tag-ordering function (`native.
WithTagOrder`, T334/ADR-0064) that resolves the newest-first release list, release-notes previous
tag, and changelog rotation's own previous-tag lookup by SemVer §11 precedence rather than git's
`version:refname` sort — so in a *regenerated* changelog a pre-release tag (`v1.4.0-rc.1`) never
gets a `CHANGELOG.md` section of its own and is never a range boundary; its commits fold into the
next release's section instead, and a section/notes/rotation lookup never walks back to a
pre-release `git describe` topology would otherwise pick. `calver`/`calver-per-env` get an order
too (T356): newest first by the lenient dotted-integer comparison on the version read through
`tag_prefix` / the effective `tag_format` (zero padding is just a number), keeping every tag since
CalVer has no pre-release and placing an unreadable tag last. Per-env `TagGlob` and
an explicit `tag_pattern` apply before this ordering, not instead of it.

This §11-ordered walk is also the candidate pool for native's oldest-in-scope fallback (T334):
when the oldest tag within an active scope (a per-env `TagGlob`, an explicit `tag_pattern`, or a
rotating `changelog.output` bucket) needs its true, unscoped previous tag, the fallback only
considers ancestor tags (`git tag -l --merged`, native's `listMergedTags`) that additionally parse
as a SemVer release under the active extractor. A tag that doesn't — another env's
differently-shaped `tag_format`, or a legacy CalVer tag left over from before a strategy switch —
is skipped, so the section the fallback resolves may reach further back into history than
`git describe`'s own topology-only walk would.

**History-aware bounds (T352, ADR-0065).** Whenever a tag order is injected (`semver`,
`semver-per-env`, with or without a `versioning.branches` block), the changelog walk, the release
notes and changelog rotation's previous-tag lookup see only tags reachable from `HEAD`
(`git tag -l [glob] --merged HEAD`): a tag cut on a maintenance line never merged into the current
branch gets no section and never bounds one. On a `--regenerate` walk, each existing section's
lower bound comes from its own ancestry: the bound of tag `t` is the highest-precedence tag that is
*both* in the scoped list and an ancestor of `t` (`git tag -l --merged <t>`, `t` itself dropped, one
call per section that has an older tag in the list). Tags on `t`'s own commit count as ancestors
here, so of two releases tagged on one commit the newer is bounded by the older (an empty section),
as in linear history. The rule is scope-preserving — under `semver-per-env`, a `uat/1.3.0` ancestor
never bounds `prod/1.3.0`'s section while `prod/1.2.0` is an ancestor too. Only when no in-scope
ancestor exists does the oldest-in-scope fallback above apply (scoped: the ordered ancestor pool's
first entry, from `git tag -l --merged <t> --no-contains <t>`, which leaves out tags on `t`'s own
commit so another env's promotion tag never bounds it; unscoped: the start of history). So after `release/1.3` is merged forward into `main`,
`v1.3.2` gets its own `v1.3.1..v1.3.2` section and `v1.4.0` stays bounded at `v1.3.1`. The tag being
cut is not in git yet, so its bound keeps T341's ordering-in rule over the reachable list. A
repository whose tags all lie on one line of history (linear history, the common case) renders
byte-identical output. `calver`/`calver-per-env` follow the same rules (T356): reachable tags only and per-section ancestry,
with the dotted-integer ordering above in place of SemVer §11.

**Release-notes ranges (T341, ADR-0064).** For a final release, notes span back to the previous
*final* reachable from `HEAD` (pre-releases are dropped from the order; reachability since
T352). For a pre-release run (`--pre-release`, or a pre-release `--set-version`), notes span back to the previous tag of *any* kind that is reachable
from `HEAD` (`git tag -l [glob] --merged HEAD`), so `v1.4.0-rc.2` covers only the commits since
`v1.4.0-rc.1` and a tag cut on another, unmerged branch never bounds the range. The tag being
released is ordered in with the existing tags even when git does not have it yet, so its
predecessor is the next-lower tag by precedence: `v1.3.1-rc.1` with `v2.0.0` and `v1.3.0` present
spans back to `v1.3.0`. This ordering-in applies to finals too: a hotfix `v1.3.1` cut while
`v2.0.0` exists also resolves `v1.3.0`.

A pre-release run never writes a `CHANGELOG.md` section at cut time. The run is detected before
resolution — `--pre-release <label>`, or a `--set-version` whose value parses as a SemVer
pre-release under `semver`/`semver-per-env` (CalVer values never count) — and the changelog step
is skipped exactly like a per-env `disable_changelog: true`: no generator call, no `pre_changelog`
hooks, no commit, and the step count shrinks to match. `heraut changelog --set-version X-pre`
prints `pre-release <tag>: CHANGELOG.md not updated` (with the styled progress reporter, the
tag-less `pre-release: CHANGELOG.md not updated` warning), still tags when `--tag` is set — the
closing summary then reports only the tag, never a changelog update or commit — and exits 0
otherwise. A `[X-pre]` section already on disk is not migrated or removed.

A `native`-managed `CHANGELOG.md` is a **preamble** (free-form content before the first section,
e.g. the `# Changelog` title) followed by **anchored sections**, newest first. Each section is
preceded by a structural HTML comment on its own line:

```
<!-- heraut-release: v0.49.0 -->
```

The anchor carries the release **tag**, is invisible in every Markdown renderer, and is emitted by
the assembly layer — never by a template block — so it is non-overridable and independent of the
customizable `release.section` block (ADR-0037, ADR-0048, ADR-0059): reformatting the header can
neither remove the anchor nor change its shape.

**Incremental (default).** Each run renders and enriches only the new release's section (O(1) API
calls) and splices it into the existing file's sections, leaving every other section untouched:

- **Missing or empty file** → bootstrap: build every section from all tags (anchored), enriching
  only the newest. No warning.
- **File with ≥1 anchor** → splice: the new section replaces the top one if its tag matches
  (idempotent re-run), otherwise it is inserted above it. All other **sections** are preserved
  verbatim, including their historical PR/MR attribution.
- **Non-empty file with no anchors** (produced by another tool, e.g. `git-cliff`, or predating this
  feature) → the run **stops with an error** directing the operator to `--regenerate`; the file is
  left byte-for-byte unchanged.

`title`/`subtitle`/`footer` (the preamble and postamble, ADR-0048/ADR-0049) are **not** part of
what "incremental" preserves: they render fresh from current config on every run, incremental or
full-regeneration alike, discarding whatever was previously on disk — including a hand-edited
title (ADR-0050). Only the sections themselves are ever preserved verbatim.

**Full regeneration (`--regenerate` / `--regenerate-changelog`).** Ignores the existing file,
rebuilds every section from all tags, and **re-enriches all of them** — this re-enrichment (an
API call per historical release rather than just the newest) is the actual distinguishing cost
`--regenerate` controls; preamble/postamble freshness no longer depends on it at all (ADR-0050).
Each section is enriched independently, so GitHub (batched GraphQL, 50 SHAs/query), GitLab in
`api_mode: graphql` (two
batched connection queries — commit authors via `commits`, MR refs via inverted `mergeRequests`,
[ADR-0042](../adr/0042-gitlab-graphql-enrichment.md)), and Azure (one `pullrequestquery` POST) each
batch *within* a release, costing roughly one API call per release (O(releases)). All of these run
over heraut's own `net/http` forge clients — no external process is spawned for enrichment
([ADR-0043](../adr/0043-forge-abstraction.md)). The one exception is GitLab in `api_mode: rest`
(the **default**), which resolves MRs with one
`GET /projects/:id/repository/commits/:sha/merge_requests` **per commit**: a full regeneration
there costs O(commits), so prefer `api_mode: graphql` (or an incremental run) on a long history.
The changelog pipeline step itself no longer carries a dedicated GitLab full-regeneration warning;
the only warning it emits is the degraded note raised when an enrichment fetch actually fails.
This is the required one-time step when migrating an existing changelog onto heraut (or repairing a
previously-anchorless file) — see [ADR-0038](../adr/0038-incremental-changelog.md) for the full
migration story, including the `regenerate_changelog` `workflow_dispatch` input heraut's own CI
uses for its own migration.

**Atomic write.** The changelog is written to a temp file in the same directory (`.<name>.tmp-*`) and
renamed over the target, so a crash or a full disk leaves the previous file whole. An existing file keeps
its permission bits; a new one gets `0644`. A symlinked changelog path is resolved first: the link is kept
and its target replaced. A temp file left by a write that was killed before its rename is removed by the
next write to the same target, so it cannot linger until a hook that stages everything commits it. The
temp file stays in the target's directory because the rename is atomic only within one filesystem.

**Rotating `changelog.output`** (see [Spec 02 § Rotating changelog output](02-configuration.md#rotating-changelog-output))
reuses this same bootstrap path unchanged for each new file: the first release in a new period/
release-line bucket hits "missing file → bootstrap" exactly as above. The one addition is bounding
that bootstrap's new section correctly — with the tag-scoping a rotation bucket implies, the scoped
tag list is empty for a brand-new bucket, which would otherwise default this step's range to "since
the beginning of history" and duplicate every prior bucket's entries into the new file. See
[ADR-0047](../adr/0047-changelog-output-resolves-after-version.md) for how this is bounded instead.

### forges — explicit metadata forge (ADR-0043)

```yaml
forges:
  - name: azure-devops
    platform: azure_devops           # github | gitlab | azure_devops
    project: my-org/my-project       # azure_devops (required: "organization/project") / gitlab
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
such anchor, so it resolves the enrichment forge the same way any zero-config forge
resolves: a **non-empty `forges:` block always wins** — each entry's still-unset fields
(host, project, token) are filled from CI env or git origin, but the block's own presence
and its entries are never bypassed. Only an **empty `forges:`** (or no block at all) falls
through to auto-detection: ambient CI markers, then `git remote get-url origin`, then a
single unambiguous default token env var; `nil` (bare hashes, no enrichment) if none of
those resolve anything. With multiple configured `forges:` entries and no explicit
`commits.enrichment_forge`, the **first** entry is used, not an error — name one explicitly
once you have more than one and enrichment shouldn't default to the first. `forges` entries
are connection/identity only — they never grant publish capability on their own; heraut
never publishes a release through a `forges` entry, it only tells heraut where to source
PR/MR metadata and commit/PR link shapes. Publishing is a separate concern (`release.targets`,
each referencing a forge by name). Enrichment source config was originally introduced for
`changelog.remote` by ADR-0026/ADR-0040; unified into the top-level `forges:` list by
ADR-0043.

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
- **Two or more set** — ambiguous; `Resolve` returns `forge.ErrAmbiguousForge`. Whether that
  aborts the run depends on `commits.enrichment_policy`, same as any other resolution failure
  below — it is not automatically fatal.
- **None set** — auto-detection resolves **no forge** for that project, same as if the token-env
  step didn't exist.

`commits.enrichment_policy` (`disabled` / `optional`, the default / `required`) governs what
happens with whichever outcome above occurred:

- **`disabled`** — forge resolution for enrichment is skipped entirely; generation proceeds
  with no PR/MR metadata, unconditionally, regardless of which of the outcomes above would
  otherwise apply.
- **`optional` / `required`, single-token outcome** — this does *not* short-circuit to "no
  forge": a real `port.Forge` is constructed against the public host with an empty project, so
  enrichment is attempted and fails. GitLab's `enrichREST` (`internal/forge/gitlab/rest.go`),
  for example, builds `https://gitlab.com/api/v4/projects//repository/commits/<sha>/merge_requests`
  (empty project segment) and gets a 404. Under `optional`, `enrichForRelease`
  (`internal/generators/native/enrich.go`) catches that error, drops enrichment, and sets
  `Degraded()` with the wrapped HTTP error as the reason — commit lines render with no `by @`
  handle and no `in [#N]` reference, but the degraded warning names an HTTP failure, not a
  missing forge. Under `required`, the run **fails outright** with that same wrapped 404 (not
  the "no forge resolved" message below). `--force` downgrades `required` to the `optional`
  behavior above ([ADR-0041](../adr/0041-remote-metadata-required-enforcement-and-force.md));
  this is also what downgrades the ambiguous-token-env case above from a hard failure.
- **`optional` / `required`, zero-token (or ambiguous, under `--force`) outcome** — nothing
  resolves at all: no CI marker, no recognised origin, no ambient token (or an ambiguity that
  got downgraded). Under `optional`, generation proceeds with no PR/MR enrichment, silently,
  with no warning and no `Degraded()` signal (that signal is reserved for a *configured* forge
  whose fetch fails, not for the absence of one); under `required` (without `--force`), the run
  fails with an error explaining that no forge was resolved and naming the three ways to supply
  one — a `forges:` entry, a supported CI environment, or a recognised git origin.

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

`Validate()` has no production call site today — `heraut check config` validates the parsed
`Config` struct directly (`config.Validate`), and `Pipeline.Check` calls `Generator.Check()`,
not `Validate()`. It exists on the interface for a generator implementation that has its own
config to validate independently of `.heraut.yml`; `native` correctly no-ops it since it has
none.

**Per-platform link resolution**: when a release targets **more than one** platform,
heraut regenerates the release notes once per platform and passes that platform's
`link` context (host, owner, repo, type) so commit/PR/MR links resolve to the correct
host and path shape (see [ADR-0021](../adr/0021-per-platform-release-notes.md)). A
single-platform release passes `nil`, and the generator falls through to ambient-CI
link detection.

## Platforms

Two platforms are supported: `github` (via the GitHub REST API, ADR-0077) and `gitlab` (via the GitLab REST API, ADR-0069). A release
can be published to one or both — `release.targets` is a list, each entry referencing a
`forges[].name` ([ADR-0044](../adr/0044-publishing-config-unification.md)). Connection
fields (`repository`/`project`, `token_env`, `base_url`) live on the `forges:` entry;
publish behavior (`draft`, `assets`) lives on the `release.targets` entry. GitHub's
pre-release flag is not configured at all — it is derived from the resolved version (§ GitHub
below, [ADR-0064](../adr/0064-semver-v2-compliance.md)).

`azure_devops` is a valid `forges:` platform (§ forges above) but has no publish driver and
never will — Azure DevOps has no equivalent of a GitHub/GitLab Release. A `release.targets[]`
entry naming an `azure_devops` forge is rejected with an actionable error; an auto-detected or
zero-config forge that resolves only to `azure_devops` is treated the same as no forge resolving
at all, so `heraut release` reports no resolvable publish destination rather than attempting one.

### GitHub

```yaml
forges:
  - name: github
    platform: github
    repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
    # token_env: GH_TOKEN       # optional; omit it to keep the default chain, see Token below
    base_url: https://github.com  # optional, defaults to https://github.com

release:
  targets:
    - forge: github              # optional when exactly one forge is configured
      draft: false
      assets:
        - dist/myapp_*
```

**Requests** ([ADR-0077](../adr/0077-github-publish-transport.md)): the driver calls the REST API in
process through `go-github`; no `gh` binary is involved.

| Step | Request |
|---|---|
| `Check()` | `GET /repos/{owner}/{repo}/releases?per_page=1` |
| create | `POST /repos/{owner}/{repo}/releases` (`tag_name`, `name` = the tag, `body` = the notes, `draft`, `prerelease`) |
| upload | `POST {upload_url}?name=<basename>`, the file as the raw body |
| publish | `PATCH /repos/{owner}/{repo}/releases/{id}` (`draft: false`) |
| cleanup | `DELETE /repos/{owner}/{repo}/releases/{id}` |

Without assets the release is one create call. With assets it is created as a draft, each file is
uploaded, then the draft is published, unless the target sets `draft: true` (uploading to a
published release fails when immutable releases are on). If an upload or the publish fails, the
draft is deleted and the original error is returned, with any delete failure appended. The upload
goes only to an expected origin: `uploads.github.com` for github.com, else `base_url`'s own origin
or `uploads.<host>`; any other `upload_url` is refused before a byte is sent.

- **API root**: `https://api.github.com` for github.com, `https://api.<host>` for a `*.ghe.com`
  tenant, `<base_url>/api/v3` for GitHub Enterprise Server. In GitHub Actions, `GITHUB_API_URL` is
  used instead when `GITHUB_SERVER_URL` has the same origin as `base_url`.
- **Repository** resolution: `cfg.Repository` → `$GITHUB_REPOSITORY` → error
- **Token**: when `token_env` is set, only that variable is read. Otherwise github.com and
  `*.ghe.com` read `GH_TOKEN`, then `GITHUB_TOKEN`; GitHub Enterprise Server reads
  `GH_ENTERPRISE_TOKEN`, then `GITHUB_ENTERPRISE_TOKEN`, then `GH_TOKEN`. `Check()` and publishing
  use the same token, sent as `Authorization: Bearer`.
- **Asset names**: each file is published under its basename; GitHub rewrites special characters
  (`meta#1.json` becomes `meta.1.json`), and heraut warns when the published name differs. GitHub
  also compares asset names without case, so `App.zip` and `app.zip` cannot both be published. The
  pre-release check therefore refuses two basenames that GitHub would publish as one, before
  anything is committed, using the renaming observed on github.com (ADR-0077 addendum): accents are
  dropped and a few letters spelled out (`ß` → `ss`, `ø` → `o`, `æ` → `ae`, `ł` → `l`), any other
  character outside ASCII letters, digits and `._-+@` becomes `.`, runs of `.` collapse, a
  trailing `.` is dropped and a leading `.` gets `default` in front. GitLab keeps names as given.
- **Pre-release**: the release is created with `prerelease: true` iff the active strategy is SemVer-based
  (`semver`/`semver-per-env`) *and* the resolved version carries SemVer pre-release identifiers
  (e.g. `1.4.0-rc.1`) — never for a final, and never under a CalVer strategy, regardless of
  whether the CalVer version happens to parse as a SemVer pre-release (a `format:
  YYYY.MM.SS-PATCH` can resolve something like `2026.10.2-0`, which does). There is no config key
  for this: a static per-target bool could contradict the version actually being published, so
  the pipeline derives the flag from the active strategy and `versioning.Result.Version` and
  passes it to `CreateRelease` ([ADR-0064](../adr/0064-semver-v2-compliance.md)).
- **Release URL**: `https://github.com/<repo>/releases/tag/<tag>` — used in the
  post-release log line

### GitLab

```yaml
forges:
  - name: gitlab
    platform: gitlab
    project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
    token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
    base_url: https://gitlab.com  # optional — see the resolution chain below

release:
  targets:
    - forge: gitlab               # optional when exactly one forge is configured
      assets:
        - dist/myapp_*
```

**`base_url` resolution**, when omitted: `CI_SERVER_URL` (when `GITLAB_CI=true`), else the
scheme+host of `CI_PROJECT_URL`, else `https://gitlab.com`. On a self-hosted GitLab CI
runner this resolves to the instance's own host with no config at all — `base_url:
https://gitlab.com` is only the *outermost* fallback, not a blanket default.

**Transport** ([ADR-0069](../adr/0069-gitlab-publish-transport.md)): heraut calls the GitLab REST
API through `gitlab.com/gitlab-org/api/client-go`; no `glab` binary is involved.

1. Each resolved asset is uploaded with `PUT /projects/:id/packages/generic/release-assets/<version>/<file>`
   (`<version>` is the tag with `/` replaced by `-`, since GitLab rejects `/` in a package version).
2. The release is created with `POST /projects/:id/releases` (`tag_name` and `name` = the tag, the
   notes as `description`), carrying one `package` asset link per uploaded file whose
   `direct_asset_path` is `/<file>`, so the public URL stays `…/-/releases/<tag>/downloads/<file>`.

Uploads happen first, so a failed upload leaves no release without its assets. The tag must
already exist (heraut tags and pushes before publishing).

- **Project** resolution: `cfg.Project` → `$CI_PROJECT_PATH` → error
- **Token**: in GitLab CI on the instance named by `CI_SERVER_URL`, `CI_JOB_TOKEN` is sent as
  `JOB-TOKEN` and any configured token is ignored; otherwise `$<TokenEnv>` (default
  `GITLAB_TOKEN`) is sent as `PRIVATE-TOKEN`. `heraut check runtime` validates a personal token
  with `GET /user` and a job token by listing the project's releases (a job token cannot call
  `GET /user`)
- **Pre-release**: not supported — GitLab releases have no pre-release concept. The shared
  `port.Platform.CreateRelease` contract still passes the derived flag (§ GitHub above), but
  this driver accepts and ignores it.
- **Catalog**: GitLab automatically publishes to the CI/CD Catalog when the project is a
  registered catalog resource — heraut has no separate config field or flag for this
- **Release URL**: `<gitlab-base>/<project>/-/releases/<tag>` — GitLab takes the tag as a single
  path segment, so `/` is escaped as `%2F` (`uat/7.4.1` → `…/-/releases/uat%2F7.4.1`) on top of
  `+` → `%2B`

### Asset resolution

`release.assets` and `release.targets[].assets` (see [Spec 02](02-configuration.md#releaseassets))
both resolve in two steps. Before anything is committed or pushed, right after the `post_bump`
hooks, each glob is expanded against the working directory, and the run aborts on a match outside the
repository, a match that is not a regular file (directories are skipped), two matches with the same
basename (they would be published under one name, so this applies to a dry run too; on GitHub,
also two basenames it would rename into one, § GitHub), or a pattern
matching no file ([ADR-0076](../adr/0076-a-release-asset-pattern-must-match-a-file.md); `release.allow_missing_assets: true`
turns that off, and a dry run only warns). (A path containing `#` is accepted since ADR-0077, which supersedes the ADR-0076 addendum.) At publish time resolution stays **lenient**: a pattern
matching nothing emits a warning and is skipped rather than failing the release — by then the tag has
already been created and pushed, so a strict failure here would leave the repository in a
partially-completed state (tag exists, no platform release). On both forges each
resolved file is opened once, after its real path is checked to be inside the repository, and
uploaded from that open file, so a symlink swapped in after the check cannot leak a file from
outside (NEW-002); the rules of the first step apply again, so a duplicate basename or a file that is
not a regular file still fails the release. On GitHub the uploads go to a draft that is
published afterwards (§ GitHub above): GitHub rejects an upload to an already-published immutable
release with HTTP 422 ("Cannot upload assets to an immutable release"). On GitLab they are uploaded to the Generic Packages
registry and linked on the release in the same create request (§ GitLab above). Deleting a GitLab
release does not delete its package files.

### Self-hosted instances and multiple entries of the same type (ADR-0025)

`base_url` may be set to any absolute `https://` URL — including a host other than the
platform's default (`github.com` / `gitlab.com`). Plain `http://` is refused, for `base_url`,
`api_url` and the CI-provided `CI_SERVER_URL` / `CI_API_V4_URL`, because the forge token is sent to
that host; the only exception is a loopback host (`localhost`, `127.0.0.0/8`, `::1`), which exists
for local servers and tests. When `base_url` is self-hosted, heraut:

- Points the driver at that host: the GitLab client is built on `base_url`; the GitHub client on
  `<base_url>/api/v3` (GitHub Enterprise Server) or `https://api.<host>` (`*.ghe.com`), with the
  token chain above. GitHub PR enrichment uses the same API root, and its GraphQL endpoint is
  `<base_url>/api/graphql` on GitHub Enterprise Server and `https://api.<host>/graphql` on
  `*.ghe.com`.
- On GitLab, skips CI autologin (`GITLAB_CI`) for that entry unless `base_url` is the CI job's own
  instance — the job token cannot reach another host — and validates the configured `token_env`.
  GitHub has no autologin: the token comes from the chain in § GitHub above, the same for `Check()`
  and publishing.
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
    ReleaseURLFromContext(tag string, lc *port.LinkContext) string // URL consistent with a given link context (ADR-0022); falls back to ReleaseURL when lc is nil
    LinkContext() port.LinkContext                 // this platform's link-resolution coordinates (host, owner, repo, type)
    Check() error                                  // token + project/repo resolved, token accepted by the API
    CreateRelease(tag, notes string, prerelease bool) error // create the release and attach its assets — see § Asset resolution above
    HasAssets() bool                               // true if cfg.Assets is non-empty
    CheckAssets() error                            // asset path rules, applied before anything is pushed (no request, no CLI call)
}
```

`ReleaseURLFromContext`/`LinkContext`, not the plain `ReleaseURL`, are what the pipeline
actually calls to produce the post-release URL and to keep it consistent with the link
context passed to the release-notes generator in the multi-platform case (§ Generator
interface above).

Both implementations are contract-tested: their requests (hosts, paths, headers, bodies) are
asserted against an `httptest.Server`. Adding a third
platform later means: implement `port.Platform`, add contract tests, register in
`app.BuildPipeline`.

## Extensibility

Adding custom platforms or generators requires modifying heraut's source. A formal
plugin system is out of scope (see [Spec 01 — Overview § Boundaries](01-overview.md#boundaries)).
