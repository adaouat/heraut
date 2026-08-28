# Spec 03 — Commands

This spec covers every heraut subcommand, its flags, and its behaviour. Configuration
fields referenced here are defined in [Spec 02 — Configuration](02-configuration.md).

## Global flags

Present on every subcommand. Defined on the root command in `internal/cmd/root.go`.

| Flag                 | Default     | Description                                                                                                                |
|----------------------|-------------|----------------------------------------------------------------------------------------------------------------------------|
| `--config <path>`    | _(auto)_    | Path to `.heraut.yml`. Defaults to `.config/heraut.yml` if present, else `.heraut.yml`. See [ADR-0005](../adr/0005-config-file-discovery.md). |
| `--dry-run`          | `false`     | Print actions without executing them. No git writes, no network calls, no file writes outside `/tmp`. Read-only git calls (tag list, log) still execute so the resolved version is accurate. |
| `--verbose`          | `false`     | Log each external command (`[exec] <cmd> <args>`) before running it, then echo its captured output (indented). Also raises the pipeline's structured logger to debug level. |
| `--env <name>`       | `""`        | Active environment override. Required for per-env strategies; ignored by single-env strategies. `auto` resolves the active environment from the current git branch against each environment's `branch:` instead of naming one explicitly. |
| `--force`            | `false`     | Bypass promotion guards E001 and E002 (per [ADR-0007](../adr/0007-version-promotion-error-handling.md)). E003 is not bypassed. Also downgrades `commits.enrichment_policy: required` to `optional` for the run (degrade instead of failing when metadata is unavailable), and required to overwrite an existing config with `heraut init --defaults` (see § `heraut init` below). |
| `--offline`          | `false`     | Forces `commits.enrichment_policy: disabled` for the run regardless of what `.heraut.yml` sets, skipping PR/MR enrichment in changelog and release-notes generation. |
| `--version` / `-v`   | —           | Print the heraut version (see § `heraut --version` below).                                                                 |
| `--help` / `-h`      | —           | Print usage and exit.                                                                                                      |

## `heraut --version`

Prints the heraut version banner (logo + tagline + `heraut <version>`), with the version
string injected from ldflags. To check the external CLIs heraut orchestrates (`git`,
`glab`, `gh`) — whether they are on `PATH` and which token env vars are set —
use `heraut check runtime` instead.

## `heraut init`

Generate a new `.heraut.yml` interactively or non-interactively.

```
heraut init                # interactive wizard (huh-based prompts)
heraut init --defaults     # write an opinionated default config, no prompts
heraut init --force        # overwrite an existing config without prompting
```

| Flag         | Default | Description                                                                                                  |
|--------------|---------|--------------------------------------------------------------------------------------------------------------|
| `--defaults` | `false` | Write a non-interactive default config (semver, prefix `"v"`, GitLab). Skip the wizard. Requires `--force` when a config already exists at the destination — see `--force` below. |
| `--force`    | `false` | Interactive mode: overwrite an existing config file without prompting. `--defaults` mode: **required** to overwrite an existing config at all — without it, `heraut init --defaults` errors rather than silently replacing the file. |

**Wizard flow**: strategy → version prefix (or CalVer format, with a custom-format option)
→ common tag format (per-env strategies only) → generate a changelog? (if yes, changelog
output file) → sprint number (only when the CalVer format uses the `SPRINT` token) →
create a release (generate notes and publish) on your forge? (if yes, platform setup —
platform type, repository/project, GitLab CI/CD advisory, token env, API mode) → PR/MR
enrichment policy (+ enrichment forge, only when 2+ forges are configured) →
environments (for per-env strategies, loops to add N envs). Existing config (if any)
pre-populates the answers, so re-running `heraut init` updates instead of replacing.

**Fields the wizard cannot preserve.** The wizard has no prompt for `commits.tickets`,
`release.assets`, `commits.types`/`scopes`/`scopes_restricted`/`types_heading_level`,
`versioning.initial_version`, `versioning.tag_type`, `changelog.tag_pattern`,
`changelog.template`, `rendering.*` (global or per-driver), `release.notes` rendering
overrides, a forge's `api_url`, or per-environment `changelog`/`release` overrides. On a
config that already has any of these set, re-running `heraut init` silently drops them
from the rewritten file — there is currently no pre-wizard warning naming them. The only
warnings `heraut init` prints are narrower, post-wizard ones: if the platform list or the
environment list itself was edited (an entry added, removed, reordered, or its type
changed) such that the rebuilt list can no longer be positionally matched to the original,
it lists exactly which per-platform or per-environment fields couldn't carry through.

For each platform step, the project/repository field is pre-populated from
`git remote get-url origin` when the current directory is a git repo (SSH and HTTPS
remotes are both supported; falls back silently if detection fails). When GitLab is
selected, the wizard shows an advisory page reminding users running `heraut release`
in a GitLab CI/CD pipeline to use `CI_JOB_TOKEN` instead of `GITLAB_TOKEN` and to
enable "Allow Git push requests to the repository" (Settings › CI/CD › Job token
permissions).

When `SPRINT` is chosen as part of the CalVer format, the wizard adds an extra step
asking for the current sprint number. This value is written to `versioning.sprint` and
can be advanced later with `heraut version sprint bump`.

**Output destination**, in priority order: `--config <path>` if passed, else the
`HERAUT_FILE` environment variable if set, else `.config/heraut.yml` if that directory
already exists, else `.heraut.yml`. The file starts with a
`# yaml-language-server: $schema=…` header so IDEs pick up the schema automatically.

## `heraut release`

Run the full release pipeline.

```
heraut release [--version <version>] [--build <id>] [--regenerate-changelog] [--dry-run] [--env <name>] [--force]
```

| Flag                     | Description                                                                          |
|--------------------------|--------------------------------------------------------------------------------------|
| `--version`              | Override the auto-computed version for **any** strategy. Bypasses bump resolution entirely — no git calls are made to resolve it. Accepts any non-empty value with no whitespace; an optional leading `v` is stripped, then the result is rendered through the active strategy's tag shape exactly like an auto-resolved version would be: through the effective `tag_format` when one applies (per-env strategies, or a top-level `tag_format`), otherwise through `versioning.tag_prefix` (default `"v"` for SemVer strategies, `""` for CalVer). A full tag already carrying the right prefix round-trips unchanged. |
| `--build`                | CI build ID appended to the tag via the `{build}` token in `tag_format`. Requires `--version`. |
| `--regenerate-changelog` | Native generator only: rebuild the entire changelog and re-enrich every section (batched per platform; one API call per commit on GitLab) instead of incrementally splicing just the new section. See [ADR-0038](../adr/0038-incremental-changelog.md). |
| `--dry-run`              | Print the action plan; execute nothing.                                              |
| `--env`                  | Active environment (required for per-env strategies).                                |
| `--force`                | Bypass E001 (target tag exists) and E002 (destination ahead).                        |

> **`{build}` tag formats:** with a `tag_format` containing `{build}`, pass `--build <id>`
> (requires `--version`) to render and publish a release per build — this creates one
> platform release per build, by design. See
> [Spec 02 § `{build}` token](02-configuration.md#build-token--ci-build-ids).

`heraut release` requires at least one **resolvable** publish destination — an explicit
`release.targets` entry, or a forge that auto-detects from CI/git origin and has a publish
driver (`github`/`gitlab`; `azure_devops` never counts, see
[Spec 02 § Platform drivers](02-configuration.md#platform-drivers)). Zero resolvable
destinations is a configuration error for this command specifically — `heraut changelog`
has no such requirement (see § Tag-only workflow below).

**Action sequence** ([ADR-0011](../adr/0011-single-pipeline-release-via-pre-computation.md), [ADR-0012](../adr/0012-changelog-commit-ownership.md)):

1. **Preflight** — always: `config.Validate`. Unless `--dry-run`: also the branch guard
   (§ Per-environment fields → `branch` in [Spec 02](02-configuration.md)) and a runtime
   check (`git` on `PATH`, `git config user.name`/`user.email` set). There is no
   working-tree cleanliness check.
2. **Resolve next version** — strategy-specific (see [Spec 04](04-versioning.md))
3. **Generate changelog** (if `changelog` is configured and not disabled for the env)
   — writes to `changelog.output` (default `CHANGELOG.md`)
4. **Commit changelog + push** — `chore(release): <version>`, then `git push`. If the
   regenerated changelog is byte-identical to the last commit (`git add` stages nothing —
   a re-run after a partial release, or a release with no changelog-worthy commits), the
   commit and push are **skipped** with a warning naming the file, and the pipeline
   continues to tag and publish rather than failing on git's "nothing to commit" exit.
5. **Create git tag** (annotated by default; set `versioning.tag_type: lightweight` to use a bare ref tag) on the changelog commit, then `git push origin <tag>`
6. **For each target** in `release.targets` (in declared order, or the single resolved
   forge with default options when `release.targets` is omitted):
   1. **Generate release notes** — regenerated independently for *this* target, using
      *its* link context, since a multi-target release can point at different forges
      with different URLs. `release.notes` (defaulted per [ADR-0046](../adr/0046-release-block-atomicity.md)
      when `release:` is present at all — see [Spec 02 § `release`](02-configuration.md#release))
      supplies rendering overrides.
   2. Create the release via `gh release create` / `glab release create`, writing the
      notes to a temp file and passing `--notes-file`/`-F` — not `--notes` — so a large
      changelog can't exceed `ARG_MAX`.
   3. Upload assets matching glob patterns (or, when the assets came from a lenient
      source, attach them directly to the create call instead of a separate upload —
      see [Spec 05](05-generators-and-platforms.md)).

The version is pre-computed in step 2 and re-used in every subsequent step
([ADR-0011](../adr/0011-single-pipeline-release-via-pre-computation.md)) — no driver
re-resolves it.

### Pre-commit hooks and the changelog commit

heraut runs `git commit` without `--no-verify` — hooks always fire. If your pre-commit
hooks run linters (e.g. `typos`, `markdownlint`) against staged files, they will also
run against `CHANGELOG.md`. Because changelog content comes from commit messages
verbatim, linters frequently produce false positives on it (unusual words, non-standard
casing, etc.).

The correct fix is to **exclude the changelog file from those linters** in your project's
tool configuration — not to bypass hooks. Examples:

```toml
# .config/typos/config.toml (or typos.toml)
extend-exclude = ["CHANGELOG.md"]
```

```yaml
# .markdownlint.yml
# or add to .markdownlintignore
```

heraut will never pass `--no-verify` to `git commit`. Bypassing hooks removes safety
checks (commit-message validation, signing, etc.) that are unrelated to the linting
false-positive problem.

## `heraut changelog`

Resolve the next version, optionally generate a changelog, optionally commit and tag —
without publishing to any release platform.

```
heraut changelog [--commit] [--tag] [--no-push] [--version <version>] [--regenerate] [--dry-run] [--env <name>]
```

| Flag           | Description                                                                                              |
|----------------|----------------------------------------------------------------------------------------------------------|
| `--commit`     | After generating, commit `CHANGELOG.md` and push.                                                        |
| `--tag`        | After committing, create and push a git tag on that commit. Implies `--commit`.                          |
| `--no-push`    | Commit and tag locally without pushing. Skips both `git push origin HEAD` and `git push origin <tag>`. Only meaningful with `--commit`/`--tag`. |
| `--version`    | Override the auto-computed version. Bypasses bump resolution. Same validation as `heraut release --version` — non-empty, no whitespace, format-agnostic. |
| `--build`      | CI build ID appended to the tag via the `{build}` token in `tag_format`. Requires `--version`.           |
| `--regenerate` | Native generator only: rebuild the entire changelog and re-enrich every section (batched per platform; one API call per commit on GitLab) instead of incrementally splicing just the new section. See [ADR-0038](../adr/0038-incremental-changelog.md). |
| `--dry-run`    | Print the action plan; execute nothing.                                                                  |
| `--env`        | Active environment.                                                                                      |

**Action sequence** (with `--tag`, mirrors `cog bump`):

1. Resolve next version (or use `--version`)
2. Generate and update `CHANGELOG.md` (only if `changelog` is configured)
3. Commit and push — `chore(release): <version>` (push skipped with `--no-push`). If
   `git add` stages nothing (the changelog is byte-identical to the last commit), the
   commit and its push are skipped with a warning naming the file; with `--tag` the tag
   is still created on the current `HEAD`.
4. Create a git tag (annotated by default; set `versioning.tag_type: lightweight` for a bare ref tag) on that commit
5. Push tag (`git push origin <tag>`) — skipped with `--no-push`

With `--no-push`, the commit and tag are created locally only; push them yourself (or
let a CI job own pushing) afterwards. The flag is a no-op without `--commit`/`--tag`,
since nothing is committed or tagged to push.

To then publish the platform release without re-generating the changelog, run
`heraut release --version <tag>`.

If the active environment has `disable_changelog: true`, changelog generation and the
git commit are skipped. If `--tag` was also passed, the tag is still created and pushed
— only the changelog step is suppressed. If neither `--tag` nor any other work remains,
the command exits 0 with an info message.

### Tag-only workflow (no `release` block required)

`heraut changelog --tag` is valid even when no `changelog` generator is configured.
It resolves the next version, creates an annotated tag, and pushes — nothing else.
This is useful when:

- The changelog is maintained manually or by another tool.
- A CI pipeline needs a versioned tag to trigger downstream jobs, but the GitHub/GitLab
  release will be created separately.
- You want to use heraut purely for version resolution and tagging, without committing
  to any platform integration.

```yaml
# Minimal .heraut.yml for tag-only use
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
# No changelog block, no release block — heraut changelog --tag still works.
```

Unlike `heraut release`, this command does **not** require a resolvable `release.targets`
entry.

## `heraut version next`

Compute and print the next version without side effects. Useful in CI to capture the
version before invoking other tools.

```
heraut version next [--env <name>] [--force]
```

Before resolving, runs the same semantic validation as `heraut check config`. A config
error prints the same path/hint output and exits with the Config code (2) without
attempting resolution.

Exits non-zero if a promotion guard trips (E001/E002/E003).

> **`{build}` tag formats:** `version next` cannot render a tag that requires a build ID
> and will error — it infers the tag from git history with no `--build` flag to supply one.
> `heraut changelog --build` and `heraut release --build` are the two commands that can
> render one (see [Spec 02 § `{build}` token](02-configuration.md#build-token--ci-build-ids)).

## `heraut version current`

Print the latest released tag for the active strategy / environment.

```
heraut version current [--env <name>] [--bare] [--force]
```

For single-env strategies, prints the latest tag overall. For per-env strategies,
prints the latest tag in the active environment's tag namespace (e.g. the latest
`prod/*` tag when `--env prod`). The common top-level `tag_format` is honoured (no
per-environment override required).

By default prints the **raw tag** (including any `{build}` suffix). `--bare` prints the
bare semantic version instead: single-env strips the tag prefix; per-env parses the tag
through the effective `tag_format`, so `main/7.4.1-158404` → `7.4.1` (and
`main/7.4.1-rc.1-158404` → `7.4.1-rc.1`).

Before resolving, runs the same semantic validation as `heraut check config`. A config
error prints the same path/hint output and exits with the Config code (2) without
attempting resolution.

Exits non-zero if no tags exist.

## `heraut version sprint bump`

For `calver` / `calver-per-env` strategies whose `format` includes the `SPRINT` token:
increments `versioning.sprint` in `.heraut.yml` and writes the file back.

```
heraut version sprint bump
```

Run this at the start of each sprint. The next `heraut release` will use the new sprint
number and reset `PATCH` to `0`.

`--dry-run` has no effect — the file is written immediately, with no confirmation prompt.

## `heraut commit verify`

Validate a single commit message against the Conventional Commits grammar
(`type(scope)!: description`, with structural body/footer parsing — see
[ADR-0027](../adr/0027-builtin-conventional-commit-checker.md)).

```
heraut commit verify [message] [--file <path>]
```

| Flag     | Description                                                                          |
|----------|---------------------------------------------------------------------------------------|
| `--file` | Read the commit message from a file instead of the positional argument. `--file -` reads from stdin. |

Exactly one of a positional `message` argument or `--file` must be given — both or
neither is a usage error.

Validates grammar, then checks the parsed type against `commits.types` (or the
default 10-type list — see [Spec 02 § `commits`](02-configuration.md#commits))
— unless the message is a git-generated merge commit or a `fixup!`/`squash!` commit,
which are always skipped. An invalid message exits with the Usage code (1); an invalid
`.heraut.yml` (if one is present) exits with the Config code (2) — same semantic
validation `heraut check config` runs.

heraut's own `.config/hk/config.pkl` `commit-msg` hook runs this command via
`go run ./cmd/heraut commit verify --file {{ commit_msg_file }}` instead of `cog verify`.

## `heraut commit check`

Validate every commit in a range — or the full history reachable from `HEAD` when no
range is given — against the same grammar and type allow-list `heraut commit verify`
checks for a single message (see [ADR-0030](../adr/0030-commit-check-rev-range-validation.md)).

```
heraut commit check [rev-range] [--from-latest-tag]
```

`rev-range` is passed straight through to `git log` — `A..B`, `A...B`, a single ref, or
omitted entirely for every commit reachable from `HEAD`. No heraut-specific range syntax;
git's own range syntax and its own errors on a malformed range are reused as-is.

`--from-latest-tag` checks only commits since the latest tag (for the active `--env`, on
per-env strategies) instead — mutually exclusive with a positional `rev-range` (passing
both is a Usage error). When no tags exist yet, it warns "no tags found — checking full
history" and falls back to the same full-history scan as no range at all.

Every commit in the range is evaluated — an invalid commit does not stop the scan. Merge
and fixup commits are skipped (the same unconditional skip `heraut commit verify` already
applies). By default only invalid commits are printed (short SHA, subject, reason) plus a
summary line (`N of M commits invalid`); the global `--verbose` flag additionally prints
every commit, valid ones included. Exits with the Usage code (1) if any commit is invalid,
or if the range itself cannot be resolved (e.g. malformed `rev-range`, git not on `PATH`)
— same classification as `heraut commit verify`'s single-message case.

## `heraut commit create`

Interactively author a Conventional Commits message and run `git commit`. Requires an
interactive terminal (TTY); invoking it from a script or CI pipeline where stdout is not a
TTY is a Usage error (exit code 1).

```
heraut commit create [--all/-a] [--dry-run] [--config <path>]
```

| Flag         | Description                                                                      |
|--------------|----------------------------------------------------------------------------------|
| `--all`/`-a` | Stage all tracked modifications before committing (`git commit -a`).             |
| `--dry-run`  | Print the assembled commit message without staging or committing.                |
| `--config`   | Path to `.heraut.yml` (inherits global default discovery).                       |

**Interactive flow:**

1. **Staging check** — if neither `--all` nor `--dry-run` is set and nothing is staged,
   the wizard first inspects the working tree: a clean tree (nothing to commit) exits
   cleanly with `nothing to commit — working tree clean`; otherwise it asks to confirm a
   `git add -A` before proceeding. Declining cancels cleanly. All these no-ops exit with
   code 0.
2. **Type** — select from `commits.types` in `.heraut.yml` (or the default 10-type
   list when absent): `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `style`,
   `perf`, `ci`, `build`. Each built-in type shows a one-line description in the menu.
3. **Scope** — if `commits.scopes` is configured, a select from the list; otherwise
   a free-text field (optional, press Enter to skip).
4. **Subject** — short, imperative description (the commit header after `type(scope): `).
5. **Breaking change** — yes/no; if yes, an optional description populates the
   `BREAKING CHANGE` footer and appends `!` to the header.
6. **Body** — optional multi-line commit body (press Enter twice to finish).
7. **Footers** — optional trailer lines in `Token: value` format (e.g.
   `Closes: #123`); one per line, blank line to stop.
8. **Preview + confirm** — the assembled message is shown in full; confirm to commit or
   cancel (exit code 0, no commit made).

The assembled message is validated with the same logic as `heraut commit verify` before
the confirmation prompt. If validation fails (assembled message violates grammar or type
allow-list), the wizard exits with the Usage code (1) and prints the error — this
situation indicates a bug in the wizard rather than user error.

**`--dry-run`** prints the assembled commit message and a `[dry-run] would run: git commit`
line, then exits with code 0. Staging checks and the interactive confirmation prompt are
both skipped.

**Config** (optional — wizard works without `.heraut.yml`):

- `commits.types` customizes the type menu — merged over the built-in defaults (`remove:` drops one).
- `commits.scopes` switches the scope field from free text to a select menu (and, with `scopes_restricted: true`, is enforced by `heraut commit verify`).

**Exit codes:**

| Code | Meaning                                                    |
|------|------------------------------------------------------------|
| 0    | Commit created, or cancelled/declined by user, or dry-run. |
| 1    | Usage error: non-TTY, guard failure, invalid footer syntax. |
| 2    | Config error: `.heraut.yml` fails to parse or validate.    |

## `heraut check`

Run preflight validations. Both `heraut release` and `heraut changelog` run these
automatically before doing any work; running them directly is useful in CI as a
separate validation step.

```
heraut check                       # runs config + runtime
heraut check config                # offline only: parse + semantic validation
heraut check runtime               # online: binaries + tokens + git user
```

Runs both sections in sequence (Config, then Runtime) and reports a combined summary.

**No config file found**: like `heraut check runtime`, bare `check` degrades instead of
hard-failing — the Config section prints a warning and is skipped, and the Runtime
section falls back to the all-tools-required probe (see § `heraut check runtime` below).
**`heraut check config` run standalone does not degrade** — with no config file, it
hard-fails with the Config exit code (2), since running the config subcommand at all is an
explicit request to validate a specific file; there is nothing to skip.

**Exit code**: if the Config section reports any errors, `heraut check` exits with the
Config code (2) — even if Runtime also failed, since a broken config makes that result
unreliable. Otherwise, if Runtime failed, it exits with the Runtime code (3).

### `heraut check config`

Offline. Parses `.heraut.yml` and runs the full semantic validator. No token or
network access required. Errors include the key path, the invalid value, and a hint.

After a successful load, a supplementary line shows the resolved path and the source
that determined it:

```
  .heraut.yml  (from .heraut.yml)
✓ config: ok
```

The source label is one of `--config`, `HERAUT_FILE`, `.config/heraut.yml`, or
`.heraut.yml`.

Example output (with errors):
```
✗ versioning.strategy: invalid value "semvr"
  hint: must be one of: semver, calver, semver-per-env, calver-per-env

✗ versioning.environments.prod.source: cycle detected (prod → staging → prod)
  hint: each promotion source must trace back to an auto env without revisiting envs

2 errors
```

### `heraut check runtime`

Online checks:

- Each external CLI in use (`git`, plus the configured platforms) is on `PATH`
  (the `native` generator has no external binary of its own)
- The token env var for each configured platform is set
- `git config user.name` and `git config user.email` are set (required for the
  changelog commit)
- **Working tree** (advisory) — `git status --porcelain`; a dirty tree prints the count
  of uncommitted changes as a warning, never a hard failure.
- **Forge resolution** — hard failure if it fails *and* a publish destination was
  actually requested (an explicit `forges:` block, or a non-empty effective
  `release.targets`); otherwise (zero-config auto-detection with nothing configured)
  the same failure is only an advisory warning, since a changelog-only user may never
  publish.

When no config file is found, `heraut check runtime` proceeds rather than failing. All
supported tools (git, gh, glab) are treated as required — hard error if any binary
is missing. The full platform check (token + API auth) is skipped in this case since
there is no config to source token names from; only binary presence is verified.

**`--env`**: the Platforms section checks the *effective* `release.targets` list for
the given environment — the env's `release.targets` when non-empty, replacing the root
list entirely; otherwise the root list (same replace semantics as the release pipeline,
see [ADR-0025](../adr/0025-multi-instance-platforms.md)/[ADR-0044](../adr/0044-publishing-config-unification.md)).
Without `--env`, or for an env with no `release.targets` override, the root list is
checked (or the single resolved forge with default options when `release.targets` is
empty). `heraut check` (bare) applies the same `--env`-aware resolution.

## Background update hint

After any successful command, heraut performs a non-blocking check against the GitHub
Releases API (timeout 500ms). If a newer version is available, it prints a single line to
stderr after the command's normal output, naming the upgrade command for however the binary
was installed:

```
heraut 1.2.3 available — run: mise upgrade heraut
```

The check is delegated to forge's shared `updatecheck` package and runs at most once per 24
hours (cached under `$XDG_CACHE_HOME/heraut/`). heraut does **not** self-replace its binary —
upgrades go through the package manager (see [ADR-0014](../adr/0014-self-update-architecture.md),
superseded). Disabled in `dev` builds and when `HERAUT_CHECK_UPDATE=false` is set.

## `heraut whatsnew`

Show release notes for every version newer than the running build, rendered as
formatted markdown.

```
heraut whatsnew
```

Fetches release notes from the GitHub Releases API for `adaouat/heraut` and renders
every version newer than the current `--version`. If the API is unreachable, falls back
to a local cache (`$XDG_CACHE_HOME/heraut/`), and finally to the changelog embedded in
the binary at build time — so the command still produces useful output offline.

Delegated to forge's shared `updatecheck.WhatsNewCommand` (see
[ADR-0014](../adr/0014-self-update-architecture.md), superseded) — heraut supplies its
repo, current version, cache file, and embedded changelog; forge handles fetching,
caching, version comparison, and rendering.
