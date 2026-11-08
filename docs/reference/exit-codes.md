# Exit codes

Every heraut command exits with one of these codes, so a script or a CI job can tell what kind of
problem stopped it. [Troubleshooting and FAQ](../guides/troubleshooting.md) lists the common messages. The code is the same with and without `--json`; in `--json` output,
`error.code` names the same class.

| Code | `error.code` | Meaning |
|---|---|---|
| 0 | | Success. |
| 1 | `usage` | The command line is wrong, or a commit message is invalid. |
| 2 | `config` | The configuration is wrong, or doesn't fit this run. |
| 3 | `runtime` | Something outside heraut failed: git, the network, a forge's API, a hook, the checkout. |
| 4 | `promotion`, `E001`–`E003` | A promotion guard stopped a per-environment release. |
| 5 | `nothing_to_release` | There is nothing to release: no new commit, or none that releases. |
| 70 | `internal` | A bug in heraut. |

## 1: usage

- An unknown flag or command, a missing argument, or flags that can't be combined (`--json` on
  `heraut release` without `--dry-run`).
- `heraut commit verify` or `heraut commit check` found an invalid commit message.
- `heraut init` or `heraut commit create` ran without an interactive terminal.
- `heraut skill install` would replace an installed skill that differs: pass `--force`.

Read the message, fix the command line, run it again. `heraut <command> --help` lists the flags.

## 2: config

- `--set-build-id` without `--set-version`.
- `.heraut.yml` doesn't parse, has an unknown key or a missing field, or has an invalid value. The
  message gives the key's path and a hint; `heraut check config` reports every finding at once.
- `--env` is missing or names an unknown environment, on a per-environment strategy.
- The current branch is not the one the environment is linked to (`environments.<name>.branch`).
- The tag format contains `{build}` and no `--set-build-id` was given.
- A `--set-version` value doesn't fit the strategy (not SemVer, or not the CalVer format).
- The next release is a major one and `versioning.bump.confirm_major` asks for `--allow-major`.
- `heraut release` has a `release:` block but nowhere to publish: no `release.targets`, and no GitHub
  or GitLab forge found from CI or the git origin. (Without a `release:` block it tags only.)

Fix `.heraut.yml` or the flags. `heraut check config` runs offline and catches the file's errors
before a release does.

## 3: runtime

- git is missing, or `git config user.name` / `user.email` is unset.
- A git command failed: a push refused by a protected branch, a tag that already exists on the
  remote, new commits on the branch since the job started.
- The checkout can't be released from: a shallow clone (fetch the full history with
  `git fetch --unshallow --tags`, `fetch-depth: 0` or `GIT_DEPTH: "0"`), or a detached `HEAD` with no
  CI branch to push the changelog commit to.
- A token is missing or can't reach the forge's API, or a forge API call failed while publishing.
- A hook command exited non-zero.
- The run was interrupted (Ctrl-C).

Fix the cause and run again. `heraut check runtime` checks git, the history and every target's token
up front. If a release stopped halfway, see
[Recover from a failed release](../guides/failure-recovery.md).

## 4: promotion guards

Only for environments with `bump: promote`, which copy a version from a source environment. The
message names both environments, the candidate tag and the tags in the way.

| Guard | What happened | What to do | `--force` |
|---|---|---|---|
| `E001` | The tag the promotion would create already exists. | Nothing, if that version was already promoted. To release it again, delete the tag first. | No effect: the run stops before writing anything |
| `E002` | The destination environment is already at a higher version than the candidate. | Usually a fix went straight to the destination: tag the source environment for it first, or wait for a newer source version. | Bypasses it |
| `E003` | The source environment has no release yet, so there is nothing to promote. | Release the source environment first (`heraut release --env <source>`). | No effect |

## 5: nothing to release

- No commit since the last release tag: `no commits since v1.4.0`.
- Commits since then, none of which releases (`Update readme`, or types `versioning.bump.overrides`
  sets to `none`): `no releasable commits since v1.4.0`, with the list.
- `heraut version current` before the first release: `no tags found`.

Nothing went wrong, but nothing was released either. A job that runs `heraut release` on every push
to the main branch gets this after each release commit; accept it so the job stays green:

```bash
heraut release || [ $? -eq 5 ]
```

## 70: internal

heraut hit a condition it doesn't handle. Please
[open an issue](https://github.com/adaouat/heraut/issues) with the command, the output, and
`heraut --version`.
