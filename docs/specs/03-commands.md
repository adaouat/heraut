# Spec 03 — Commands

This spec covers every heraut subcommand, its flags, and its behaviour. Configuration
fields referenced here are defined in [Spec 02 — Configuration](02-configuration.md).

## Global flags

Present on every subcommand. Defined on the root command in `internal/cmd/root.go`.

| Flag                 | Default     | Description                                                                                                                |
|----------------------|-------------|----------------------------------------------------------------------------------------------------------------------------|
| `--config <path>`    | _(auto)_    | Path to `.heraut.yml`. Defaults to `.config/heraut.yml` if present, else `.heraut.yml`. See [ADR-0005](../adr/0005-config-file-discovery.md). |
| `--dry-run`          | `false`     | Print actions without executing them. No git writes, no network calls, no file writes outside `/tmp`. Read-only git calls (tag list, log) still execute so the resolved version is accurate. |
| `--verbose`          | `false`     | Log each external command (`[exec] <cmd> <args>`) before running it, then echo its captured output (indented).              |
| `--env <name>`       | `""`        | Active environment override. Required for per-env strategies; ignored by single-env strategies.                            |
| `--force`            | `false`     | Bypass promotion guards E001 and E002 (per [ADR-0007](../adr/0007-version-promotion-error-handling.md)). E003 is not bypassed. |
| `--version` / `-v`   | —           | Print the heraut version (see § `heraut --version` below).                                                                 |
| `--help` / `-h`      | —           | Print usage and exit.                                                                                                      |

## `heraut --version`

Prints the heraut version banner (logo + tagline + `heraut <version>`), with the version
string injected from ldflags. To check the external CLIs heraut orchestrates (`git`,
`git-cliff`, `glab`, `gh`, `cog`, `communique`) — whether they are on `PATH` and which
token env vars are set — use `heraut check runtime` instead.

## `heraut init`

Generate a new `.heraut.yml` interactively or non-interactively.

```
heraut init                # interactive wizard (huh-based prompts)
heraut init --defaults     # write an opinionated default config, no prompts
heraut init --force        # overwrite an existing config without prompting
```

| Flag         | Default | Description                                                                                                  |
|--------------|---------|--------------------------------------------------------------------------------------------------------------|
| `--defaults` | `false` | Write a non-interactive default config (semver, prefix `"v"`, git-cliff, GitLab). Skip the wizard.           |
| `--force`    | `false` | Overwrite an existing config file. Without it, heraut prompts before overwriting.                            |

**Wizard flow**: strategy → prefix / format → sprint (if format uses `SPRINT`) → generator(s) → platform(s) → environments
(for per-env strategies, loops to add N envs). Existing config (if any) pre-populates
the answers, so re-running `heraut init` updates instead of replacing.

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

**Output**: writes to `.config/heraut.yml` if the `.config/` directory already exists,
else to `.heraut.yml`. Pass `--config <path>` to write to an explicit location instead.
The file starts with a `# yaml-language-server: $schema=…` header so IDEs pick up the
schema automatically.

## `heraut release`

Run the full release pipeline.

```
heraut release [--version X.Y.Z] [--dry-run] [--env <name>] [--force]
```

| Flag         | Description                                                                          |
|--------------|--------------------------------------------------------------------------------------|
| `--version`  | Override the auto-computed version. Bypasses bump resolution.                        |
| `--dry-run`  | Print the action plan; execute nothing.                                              |
| `--env`      | Active environment (required for per-env strategies).                                |
| `--force`    | Bypass E001 (target tag exists) and E002 (destination ahead).                        |

> **`{build}` tag formats:** `heraut release` has no `--build` flag, so a `tag_format`
> containing `{build}` cannot be rendered here and the command errors. The build-id flow
> is changelog-only today (see [Spec 02 § `{build}` token](02-configuration.md#build-token--ci-build-ids));
> `release --build` is planned (roadmap T57).

**Action sequence** ([ADR-0011](../adr/0011-single-pipeline-release-via-pre-computation.md), [ADR-0012](../adr/0012-changelog-commit-ownership.md)):

1. **Preflight** — run `heraut check config` + `heraut check runtime` checks
2. **Resolve next version** — strategy-specific (see [Spec 04](04-versioning.md))
3. **Generate changelog** (if `changelog` is configured and not disabled for the env)
   — writes to `changelog.output` (default `CHANGELOG.md`)
4. **Commit changelog + push** — `chore(release): <version>`, then `git push`
5. **Create git tag** (annotated by default; set `versioning.tag_type: lightweight` to use a bare ref tag) on the changelog commit, then `git push --tags`
6. **Generate release notes** (if `release.notes` is configured and not disabled for
   the env) — the notes are needed at release-creation time, so they are produced before
   any platform call
7. **For each platform** in `release.platforms` (in declared order):
   1. Create the release via `gh release create` / `glab release create`, passing the
      notes from step 6 (`--notes`)
   2. Upload assets matching glob patterns

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
heraut changelog [--commit] [--tag] [--version X.Y.Z] [--dry-run] [--env <name>]
```

| Flag         | Description                                                                                              |
|--------------|----------------------------------------------------------------------------------------------------------|
| `--commit`   | After generating, commit `CHANGELOG.md` and push.                                                        |
| `--tag`      | After committing, create and push a git tag on that commit. Implies `--commit`.                          |
| `--version`  | Override the auto-computed version.                                                                      |
| `--build`    | CI build ID appended to the tag via the `{build}` token in `tag_format`. Requires `--version`.           |
| `--dry-run`  | Print the action plan; execute nothing.                                                                  |
| `--env`      | Active environment.                                                                                      |

**Action sequence** (with `--tag`, mirrors `cog bump`):

1. Resolve next version (or use `--version`)
2. Generate and update `CHANGELOG.md` (only if `changelog` is configured)
3. Commit and push — `chore(release): <version>`
4. Create a git tag (annotated by default; set `versioning.tag_type: lightweight` for a bare ref tag) on that commit
5. Push tag (`git push --tags`)

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

Unlike `heraut release`, this command does **not** require a `release.platforms` entry.

## `heraut version next`

Compute and print the next version without side effects. Useful in CI to capture the
version before invoking other tools.

```
heraut version next [--env <name>] [--force]
```

Exits non-zero if a promotion guard trips (E001/E002/E003).

> **`{build}` tag formats:** `version next` cannot render a tag that requires a build ID
> and will error. The build-id flow is changelog-only (see
> [Spec 02 § `{build}` token](02-configuration.md#build-token--ci-build-ids)).

## `heraut version current`

Print the latest released tag for the active strategy / environment.

```
heraut version current [--env <name>]
```

For single-env strategies, prints the latest tag overall. For per-env strategies,
prints the latest tag in the active environment's tag namespace (e.g. the latest
`prod/*` tag when `--env prod`).

Prints the **raw tag** (including any `{build}` suffix), not the bare version. For per-env
strategies it currently requires a per-environment `tag_format` rather than the common
top-level one — both are tracked in roadmap T54 (fallback) and T58 (bare-version output).

Exits non-zero if no tags exist.

## `heraut version sprint bump`

For `calver` / `calver-per-env` strategies whose `format` includes the `SPRINT` token:
increments `versioning.sprint` in `.heraut.yml` and writes the file back.

```
heraut version sprint bump
```

Run this at the start of each sprint. The next `heraut release` will use the new sprint
number and reset `PATCH` to `0`.

`--dry-run` has no effect — this is a write-only command that requires confirmation.

## `heraut check`

Run preflight validations. Both `heraut release` and `heraut changelog` run these
automatically before doing any work; running them directly is useful in CI as a
separate validation step.

```
heraut check                       # runs config + runtime + cliff
heraut check config                # offline only: parse + semantic validation
heraut check runtime               # online: binaries + tokens + git user
heraut check cliff                 # validate the effective git-cliff config(s)
heraut check cliff changelog       # only the changelog config
heraut check cliff release-notes   # only the release-notes config
```

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

- Each external CLI in use (`git`, plus the configured generators and platforms) is on
  `PATH`
- The token env var for each configured platform is set
- `git config user.name` and `git config user.email` are set (required for the
  changelog commit)

When no config file is found, `heraut check runtime` proceeds rather than failing. All
supported tools (git, gh, glab, git-cliff, cog, communique) are treated as required —
hard error if any binary is missing. The full platform check (token + API auth) is
skipped in this case since there is no config to source token names from; only binary
presence is verified.

### `heraut check cliff`

Invokes `git-cliff --context --no-exec` against the effective merged config (embedded
default + user override). Exits non-zero if git-cliff rejects the config — catches
broken regex, malformed TOML, missing template references, etc.

`heraut check cliff changelog` scopes to the changelog generator only.
`heraut check cliff release-notes` scopes to the release-notes generator only.

## `heraut cliff`

Print the effective merged git-cliff TOML — what heraut actually feeds to `git-cliff`.

```
heraut cliff changelog        # effective config for `changelog`
heraut cliff release-notes    # effective config for `release.notes`
```

Output is valid TOML on stdout; pipe it to a file if you want to commit a frozen
version, or to `git-cliff` directly to reproduce heraut's invocation.

## `heraut self-update`

Replace the running binary with the latest GitHub release. See
[ADR-0014](../adr/0014-self-update-architecture.md).

```
heraut self-update            # download + verify + atomically replace
heraut self-update --check    # print current vs latest; exit 1 if update available
```

Skipped for `dev` builds (no `ldflags`-injected version) and during a running
`heraut self-update`. Disable the background check (printed at the end of any other
command) by setting `HERAUT_CHECK_UPDATE=false`.

## Background update hint

After any successful command, heraut performs a non-blocking check against the GitHub
Releases API (timeout 500ms). If a newer version is available, it prints a single hint
line to stderr after the command's normal output:

```
hint: heraut 1.2.3 available — run: heraut self-update
```

Disabled for `heraut self-update`, in `dev` builds, and when
`HERAUT_CHECK_UPDATE=false` is set.
