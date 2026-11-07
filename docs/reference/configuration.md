# Configuration

heraut reads one YAML file. This page lists every key it accepts; for the same keys in context, with
comments, see [`heraut.sample.yml`](../heraut.sample.yml).

## Where the file lives

heraut uses the first of:

1. the `--config` flag: `heraut release --config ci/heraut.yml`;
2. the `HERAUT_FILE` environment variable;
3. `.config/heraut.yml`;
4. `.heraut.yml`, at the root of the repository.

`heraut init` writes `.config/heraut.yml` when a `.config/` directory exists, `.heraut.yml`
otherwise.

## A minimal file

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

changelog:
  output: CHANGELOG.md

release:
  notes: {}
```

The first line points editors that use the YAML language server (VS Code, Zed, Neovim, JetBrains
with a plugin) at heraut's schema: they then complete keys, flag mistakes, and show the
descriptions below on hover. Replace `main` with your heraut version to pin it.

## How the keys are organised

- `version`, `versioning`: how versions are computed and tagged.
- `changelog`, `release`, `notes`: what heraut writes and publishes.
- `commits`, `rendering`: how commit messages are read and how the documents look.
- `forges`: which GitHub, GitLab or Azure DevOps it talks to, when detection from CI or the git
  origin isn't enough.
- `environments`: per-environment settings, for the `-per-env` strategies.
- `hooks`, `git`: your own commands during a release, and options for heraut's git pushes.

`heraut check config` validates the file offline, with the path and a hint for every finding.

In the tables, `forges[].name` is the `name` of each `forges` entry, and `environments.<name>.bump`
is the `bump` of each environment.

<!-- BEGIN GENERATED from schema.json by `mise run docs:gen`. Edit schema.json, not this part. -->

## `version`

| Key | Type | Default | Description |
|---|---|---|---|
| `version` | string |  | Schema version. Must be "1". |

## `versioning`

How versions are computed and written as tags: the strategy (SemVer or CalVer, optionally one line per environment), the tag prefix or format, and how commits bump the version. Example: strategy: semver with tag_prefix: "v" gives tags like v1.4.0.

| Key | Type | Default | Description |
|---|---|---|---|
| `versioning.strategy` | `semver` \| `calver` \| `semver-per-env` \| `calver-per-env` |  | How versions are computed: semver from commit messages (v1.4.0), calver from the date (2026.11.0), or semver-per-env / calver-per-env for one version line per environment (dev, staging, prod). |
| `versioning.tag_prefix` | string |  | Prepended to every version tag. Defaults to "v" for semver and semver-per-env, "" for calver and calver-per-env. Set "" for bare version tags (1.2.3). Only tags with this prefix count as releases. |
| `versioning.initial_version` | string |  | Version used when no tags exist yet. SemVer strategies only. |
| `versioning.bump` | object |  | SemVer auto-bump resolution: mode and, for auto mode, per-commit bump-level overrides. SemVer strategies only. |
| `versioning.bump.mode` | `auto` \| `manual` |  | "auto" infers from conventional commits. "manual" requires --set-version. Defaults to "auto" when omitted. |
| `versioning.bump.overrides` | list of objects |  | Bump-level rules evaluated in order; the first match wins over the built-in defaults (breaking commits -\> major, feat -\> minor, any other conventional commit -\> patch). |
| `versioning.bump.overrides[].type` | string |  | Match commits of this conventional-commit type. |
| `versioning.bump.overrides[].regex` | string |  | Match commits whose subject matches this regex. |
| `versioning.bump.overrides[].breaking` | boolean |  | Further scope the match to commits with (true) or without (false) the breaking-change flag/footer. |
| `versioning.bump.overrides[].bump` | `major` \| `minor` \| `patch` \| `none` |  | Bump level to assign to matching commits. "none" excludes them from the release's bump entirely. |
| `versioning.bump.stay_at_v0` | boolean |  | While the current major version is 0, hold a major bump back to minor (with a warning) instead of releasing 1.0.0. Lift it for one run with --allow-major. Defaults to false. |
| `versioning.bump.confirm_major` | boolean |  | Refuse an automatic major bump, at any version, until the run is repeated with --allow-major, so a major release is always deliberate. version next still prints it, with a warning. Ignored with --set-version or mode: manual. Default false. |
| `versioning.format` | string |  | CalVer format string (e.g. "YYYY.MM.PATCH"). Required for calver strategies. |
| `versioning.sprint` | integer |  | Current sprint counter. Required when format contains SPRINT. Advance with heraut version sprint bump. |
| `versioning.tag_format` | string |  | Common tag format template for per-env strategies (e.g. "{env}/{version}"). {build} (CI build ID) must directly follow "+", e.g. "{env}/{version}+{build}" — SemVer build metadata. |
| `versioning.tag_type` | `annotated` \| `lightweight` |  | Git tag type. "annotated" (default) creates tags with a tagger, timestamp, and message. "lightweight" creates bare ref tags. |
| `versioning.commit_message` | string |  | Commit message template for the changelog commit, and (when tag_type is "annotated") the tag's own annotation message. "${version}" is substituted with the resolved version. Defaults to "chore(release): ${version}". |
| `versioning.branches` | list of objects |  | Branches heraut may release from (semver only). An exact name is a release branch (main). A range, or a glob whose matched name ends in N.x or N.N.x, makes a maintenance branch limited to that line (release/1.x). |
| `versioning.branches[].name` | string |  | Exact branch name, or a glob in path.Match syntax ("*" does not cross "/"). |
| `versioning.branches[].range` | string |  | Version range this branch may release: N.x (patch and minor bumps) or N.N.x (patch bumps only), e.g. 1.x or 1.3.x. Omit on a glob name to derive it from the matched branch name. |

## `changelog`

A generated document: the changelog file (changelog:) or the notes published with each release (release.notes:). Sets where the changelog is written, which tags it covers, and its template and rendering overrides.

| Key | Type | Default | Description |
|---|---|---|---|
| `changelog.output` | string |  | Where the changelog is written, e.g. CHANGELOG.md. Only under changelog:, since release notes are never written to a file. Tokens start a new file per period: CHANGELOG_{YYYY}.md (CalVer) or CHANGELOG_{MAJOR}.md (SemVer). |
| `changelog.tag_pattern` | string |  | Regex selecting this scope's release tags, applied in-process (Go regex). E.g. "^v\[0-9]" for bare semver or "^prod/" for a per-env prefix. |
| `changelog.template` | string |  | Path to a full Go text/template file layered over the built-in blocks. It can redefine the whole document or single blocks, and wins over rendering.templates for what it defines. |
| `changelog.rendering` | object |  | Rendering overrides for this document only (template snippets, excludes), merged over the top-level rendering block. Same keys as `rendering`. |

## `notes`

Configuration of 'heraut notes': the template layered over the built-in notes blocks (no release heading; commit lines without hash, author or PR reference).

| Key | Type | Default | Description |
|---|---|---|---|
| `notes.template` | string |  | Path to a full template file (Go text/template) parsed on top of the built-in blocks; it wins over inline rendering.templates for whatever it defines. 'heraut notes --template' replaces it for one run. |
| `notes.rendering` | object |  | Template snippets and excludes for the notes, deep-merged over the global rendering block. Same keys as `rendering`. |

## `release`

Publishing: create a release with its notes on each target forge. Present, even as release: {}, means heraut release publishes; omit it for a project that only tags. Holds the targets, the files to attach and the notes settings.

| Key | Type | Default | Description |
|---|---|---|---|
| `release.notes` | object |  | A generated document: the changelog file (changelog:) or the notes published with each release (release.notes:). Sets where the changelog is written, which tags it covers, and its template and rendering overrides. Same keys as `changelog`. |
| `release.assets` | list of string |  | Files to attach to every target's release, as glob patterns (e.g. dist/*.tar.gz). Each must match a file in the repository once post_bump hooks have run, or the release stops before committing. A target's own assets replace this list. |
| `release.allow_missing_assets` | boolean |  | Let an asset pattern match nothing: it is skipped with a warning instead of stopping the release. Set it when a pre_tag, post_tag or pre_release hook builds the files. Default false. |
| `release.targets` | list of objects |  | Where to publish, each entry naming a forges\[].name. Omit it to publish to the one forge found from the CI job or the git origin. |
| `release.targets[].forge` | string |  | References a forges\[].name. Optional when exactly one forge is configured. |
| `release.targets[].draft` | boolean |  | Create the release as a draft, to publish by hand. GitHub only. Default false. |
| `release.targets[].assets` | list of string |  | Glob patterns of files to attach to this target's release only, replacing release.assets for it. The same rules apply: files in the repository, and each pattern must match unless allow_missing_assets is set. |

## `environments`

Per-environment configuration. Required for semver-per-env and calver-per-env strategies.

| Key | Type | Default | Description |
|---|---|---|---|
| `environments.<name>.bump` | `auto` \| `promote` |  | auto computes this environment's next version from its commits or the date; promote copies the latest version of the source environment (e.g. staging to prod). |
| `environments.<name>.tag_format` | string |  | Per-environment tag format (e.g. "dev/{version}"). Must contain {version}. {build} (CI build ID) must directly follow "+", e.g. "{env}/{version}+{build}" — SemVer build metadata. |
| `environments.<name>.branch` | string |  | Branch this environment is operated from. heraut refuses any --env command for this environment unless the current git branch matches (override with --force). |
| `environments.<name>.source` | string |  | The environment a promote environment copies its version from, e.g. staging. |
| `environments.<name>.disable_changelog` | boolean |  | Disable changelog generation for this environment. |
| `environments.<name>.disable_release` | boolean |  | Disable the entire release: behavior (notes generation and publishing, together) for this environment. |
| `environments.<name>.changelog` | object |  | Changelog settings for this environment, merged over the top-level changelog block. Same keys as `changelog`. |
| `environments.<name>.release` | object |  | Override release settings for this environment. |
| `environments.<name>.release.notes` | object |  | Release notes settings for this environment, merged over release.notes. Same keys as `changelog`. |
| `environments.<name>.release.targets` | list of objects |  | Where to publish for this environment, replacing the top-level release.targets list entirely. |
| `environments.<name>.release.targets[].forge` | string |  | References a forges\[].name. Optional when exactly one forge is configured. |
| `environments.<name>.release.targets[].draft` | boolean |  | Create the release as a draft, to publish by hand. GitHub only. Default false. |
| `environments.<name>.release.targets[].assets` | list of string |  | Glob patterns of files to attach to this target's release only, replacing release.assets for it. The same rules apply: files in the repository, and each pattern must match unless allow_missing_assets is set. |
| `environments.<name>.git` | object |  | Override git.push_options for this environment, list by list. Same keys as `git`. |

## `commits`

How commit messages are read: the allowed types and scopes, ticket links, extra message rules, and where pull request data for the changelog comes from. Used by heraut commit verify and by the changelog alike.

| Key | Type | Default | Description |
|---|---|---|---|
| `commits.types_heading_level` | integer |  | Heading depth (number of '#') of the type sections, such as ### 🚀 Features, in the changelog and release notes. Default 3. |
| `commits.types` | list of objects |  | Conventional-commit type rules, merged over the built-in defaults by name. The effective set is both the 'heraut commit verify' allow-list and the changelog section taxonomy. |
| `commits.types[].name` | string |  | The conventional-commit type word (e.g. feat, fix). |
| `commits.types[].order` | integer |  | Display sort position for the changelog section. Omit for unordered (sorts after ordered types). |
| `commits.types[].render` | string |  | Section heading label (e.g. "🚀 Features"). Omit to route the type into the catch-all "💼 Other" group (like the default build type). |
| `commits.types[].remove` | boolean |  | Drop this default type from the effective set (removes it from the 'heraut commit verify' allow-list). |
| `commits.types[].description` | string |  | One-line hint shown beside the type in the 'heraut commit create' wizard picker. |
| `commits.scopes` | list of objects |  | Allowed scopes. Offered by 'heraut commit create'; enforced by 'heraut commit verify' only when scopes_restricted is true. |
| `commits.scopes[].name` | string |  | The scope name. |
| `commits.scopes[].remove` | boolean |  | Drop this scope from the effective set — used to remove a built-in default (deps, deps-dev, release). |
| `commits.scopes[].description` | string |  | One-line hint shown beside the scope in the 'heraut commit create' wizard picker. |
| `commits.scopes_restricted` | boolean |  | When true, 'heraut commit verify' rejects scopes outside the scopes list. Defaults to false. |
| `commits.tickets` | list of object |  | Issue-tracker links. Each entry's regex is matched in commit messages and rendered as a link in the changelog and release notes. |
| `commits.tickets[].pattern` | string |  | Regex matching ticket IDs, e.g. \[A-Z]+-\[0-9]+. The first capture group (or full match) becomes {ticket}. |
| `commits.tickets[].url` | string |  | URL template containing {ticket}, e.g. https://jira.example.com/browse/{ticket}. |
| `commits.rules` | list of objects |  | Extra checks on commit messages, run by heraut commit verify and commit check: a regex a message must or must not match, or a required ticket reference, optionally limited to some types or scopes. |
| `commits.rules[].name` | string |  | Rule name, shown in violation messages. Must be unique within commits.rules. |
| `commits.rules[].deny` | string |  | Regex that must NOT match target. |
| `commits.rules[].require` | string |  | Regex that MUST match target. |
| `commits.rules[].require_ticket` | boolean |  | Shorthand: target must match one of the already-configured commits.tickets\[].pattern entries, instead of duplicating a ticket regex in require. Requires a non-empty commits.tickets. |
| `commits.rules[].target` | `header` \| `body` \| `footer` \| `message` | `message` | Part of the message matched against: the raw first line (header), the parsed body, the joined footer trailers (footer), or the full raw text (message, the default). |
| `commits.rules[].message` | string |  | Shown when the rule is violated. Required for deny/require; optional for require_ticket (gets a synthesized default naming the rule). |
| `commits.rules[].types` | list of string |  | Optionally scope the rule to commits of these conventional-commit types. Omitting both types and scopes applies the rule to every commit. |
| `commits.rules[].scopes` | list of string |  | Optionally scope the rule to commits with these scopes. Omitting both types and scopes applies the rule to every commit. |
| `commits.enrichment_forge` | string |  | The forges\[].name to read pull request or merge request data from (links, authors, new contributors) for every target's notes. Required with two or more forges. Example: github. |
| `commits.enrichment_policy` | `disabled` \| `optional` \| `required` | `optional` | Whether to fetch pull request data: optional (the default) fetches it and warns when it can't, required fails the run instead, disabled never fetches. --offline forces disabled for one run. |

## `rendering`

Shapes the changelog and release notes: template snippets that replace the built-in blocks, and filters that drop commits. Set at the top level, per document (changelog, release.notes) or per environment; the closest level wins, key by key.

| Key | Type | Default | Description |
|---|---|---|---|
| `rendering.excludes` | list of objects |  | Filters that drop matched commits from the rendered changelog/release-notes. |
| `rendering.excludes[].type` | string |  | Match commits of this conventional-commit type. |
| `rendering.excludes[].regex` | string |  | Match commits whose subject matches this regex (e.g. "^chore\\(deps"). |
| `rendering.templates` | object |  | Go text/template snippets that replace built-in blocks by name, e.g. release.group for the type headings. Unset blocks keep their default. Merged from the top level, then per document, then per environment. |
| `rendering.templates.title` | string |  | Document-level title, fired once per document (not once per release). Executes against .Heraut's own fields directly (.Version .URL .GeneratedAt), not the full release data — write {{ .Version }}, not {{ .Heraut.Version }}. Changelog default: '# Changelog'. Release-notes default: empty. |
| `rendering.templates.subtitle` | string |  | Document-level subtitle rendered under title, fired once per document. Same .Heraut-only context as title. Empty by default, in the changelog and the release notes. |
| `rendering.templates.footer` | string |  | Document-level footer, fired once per document (not once per release) — the trailing counterpart to title/subtitle. Same .Heraut-only context as title: write {{ .Version }}, not {{ .Heraut.Version }}. Default: '_Generated by \[heraut](url) version at HH:MM on YYYY-MM-DD._'. |
| `rendering.templates.changelog` | string |  | Root changelog-section template composing release.section, release.group, and commit.message blocks. |
| `rendering.templates.release_notes` | string |  | Root release-notes template composing release.section, release.group, commit.message, release.contributors, and release.stats blocks. |
| `rendering.templates.release` | object |  | Blocks used once per release in the document: its version heading, the type group headings, the new contributors and statistics blocks, and a footer after each release. |
| `rendering.templates.release.section` | string |  | One release's version heading, fired once per rendered release. Changelog default: '## \[version](compare-url) - date'. Release-notes default: empty. |
| `rendering.templates.release.group` | string |  | Commit-type group heading (e.g. '### 🚀 Features'). |
| `rendering.templates.release.contributors` | string |  | The 'New Contributors ❤️' block wrapping the contributor entries (release notes). |
| `rendering.templates.release.stats` | string |  | The Commit Statistics block (commit/issue counts, timespan) in release notes. |
| `rendering.templates.release.footer` | string |  | Trailing content appended after each rendered release's body. Fires once per release, not once per document — use the top-level footer instead for document-level content. Empty by default. |
| `rendering.templates.commit` | object |  | Blocks used once per commit: the commit line, its ticket links and the new-contributor entries, plus trailers, which sets how footer lines such as Co-authored-by render. |
| `rendering.templates.commit.message` | string |  | One commit line: description, scope, breaking marker, hash link, 'by @author', PR/MR reference, ticket links. |
| `rendering.templates.commit.ticket` | string |  | One ticket link within a commit line (called once per match from the commit.message block). Default: '(\[TICKET](url))'. |
| `rendering.templates.commit.contributor` | string |  | One entry in the New Contributors block for a first-time contributor. |
| `rendering.templates.commit.trailers` | list of objects |  | How commit footer trailers render, by token (e.g. Co-authored-by): a template for the line, or hide to drop it. Merged by token from the top level, then per document, then per environment. |
| `rendering.templates.commit.trailers[].token` | string |  | The trailer token to match, e.g. "Co-authored-by". Case doesn't matter. |
| `rendering.templates.commit.trailers[].renderer` | string |  | Go text/template snippet executed with {Token, Value} as its data context, replacing the built-in "Token: Value" line for matching footers. |
| `rendering.templates.commit.trailers[].hide` | boolean |  | Drops matching footers from rendered output entirely. |

## `hooks`

Shell commands run at fixed points of a release, e.g. a build once the version is known. run can use {{ .Version }}, {{ .Tag }}, {{ .PreviousTag }}, {{ .Env }}, and {{ .Platform }} in pre_release and post_release.

| Key | Type | Default | Description |
|---|---|---|---|
| `hooks.post_bump` | list of objects |  | Runs immediately after the next version is resolved. |
| `hooks.post_bump[].run` | string |  | The shell command to run. |
| `hooks.post_bump[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |
| `hooks.pre_changelog` | list of objects |  | Runs before changelog generation. |
| `hooks.pre_changelog[].run` | string |  | The shell command to run. |
| `hooks.pre_changelog[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |
| `hooks.pre_tag` | list of objects |  | Runs before the local git tag is created. |
| `hooks.pre_tag[].run` | string |  | The shell command to run. |
| `hooks.pre_tag[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |
| `hooks.post_tag` | list of objects |  | Runs after the tag is pushed to origin. |
| `hooks.post_tag[].run` | string |  | The shell command to run. |
| `hooks.post_tag[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |
| `hooks.pre_release` | list of objects |  | Runs before publishing to a platform, once per platform. |
| `hooks.pre_release[].run` | string |  | The shell command to run. |
| `hooks.pre_release[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |
| `hooks.post_release` | list of objects |  | Runs after publishing to a platform, once per platform. |
| `hooks.post_release[].run` | string |  | The shell command to run. |
| `hooks.post_release[].stage` | list of string |  | Paths to stage after run completes. Only valid under post_bump/pre_changelog — a config validation error at any other hook point. |

## `forges`

Code-hosting platforms heraut talks to. Usually not needed: heraut finds the forge from the CI job or the git origin. Add one for a self-hosted instance, a custom token variable, or to publish to several forges.

| Key | Type | Default | Description |
|---|---|---|---|
| `forges[].name` | string |  | Unique identifier for this forge entry, referenced by release.targets\[].forge and commits.enrichment_forge. |
| `forges[].platform` | `github` \| `gitlab` \| `azure_devops` |  | The forge's type: github, gitlab or azure_devops. Azure DevOps supplies pull request data only; it has no releases to publish to. |
| `forges[].project` | string |  | GitLab project path (namespace\[/subgroup]/repo). GitLab. For azure_devops, "organization/project". |
| `forges[].repository` | string |  | GitHub repository (owner/repo). GitHub. For azure_devops, the repository name. |
| `forges[].base_url` | string |  | Web base URL of the forge instance (e.g. https://gitlab.example.com for a self-hosted instance). Must be https (plain http is accepted only for loopback hosts). Defaults per type when unset. |
| `forges[].api_url` | string |  | API host override, when it differs from base_url. Must be https (plain http is accepted only for loopback hosts). Defaults per type when unset. |
| `forges[].api_mode` | `rest` \| `graphql` | `rest` | How heraut calls the forge's API: rest (the default) or graphql. |
| `forges[].token_env` | string |  | Environment variable holding the API token. Defaults per type when unset. |

## `git`

Options for the git commands heraut runs itself: the --push-option values of its two pushes.

| Key | Type | Default | Description |
|---|---|---|---|
| `git.push_options` | object |  | --push-option values for heraut's pushes, e.g. commit: \["ci.skip"] to skip the release commit's GitLab pipeline. Under an environment, a missing list keeps the top-level one and \[] clears it. |
| `git.push_options.commit` | list of string |  | Passed to the changelog commit push (git push origin HEAD), in order, e.g. \["ci.skip"] to skip that commit's GitLab pipeline. |
| `git.push_options.tag` | list of string |  | Passed to the tag push (git push origin \<tag\>), in order. Keep ci.skip out of it unless the tag pipeline should not run. |
<!-- END GENERATED -->
