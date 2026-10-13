---
name: heraut
description: Use when releasing, versioning, tagging, changelogging or validating commit messages in a git project that has a .heraut.yml, or when asked to run the heraut CLI. Covers the safe order of commands, JSON output and exit codes.
---

# heraut

`heraut` resolves the next version of a git project, writes the changelog, tags, and publishes the
release to GitHub or GitLab. Configuration lives in `.heraut.yml` (or `.config/heraut.yml`, or the
file named by `--config` / `HERAUT_FILE`). The project's `schema.json` describes every key.

## Rules for an agent

- **Never run `heraut release` or `heraut changelog --commit/--tag` without the user's explicit
  approval in this conversation.** They push commits and tags and publish releases, which cannot be
  taken back. The other commands read, or write one local file (noted in the table).
- Always put `--json` on commands that support it and read stdout only. Stderr carries human text.
- Use `--offline` when you have no forge token; it skips PR/MR enrichment.
- Do not run `heraut init` or `heraut commit create` interactively. Use `heraut init --defaults`, or
  write `.heraut.yml` yourself; check a message with `heraut commit verify`, then use `git commit -m`.

## Safe order for a release

1. `heraut check --json` — the config is valid and the tools (git, gh, tokens) are present.
2. `heraut version next --json` — the version a release would create, and why (`bump`).
3. `heraut release --dry-run --json` — the ordered plan: changelog, hooks, tag, push, publish.
   Show it to the user.
4. Only after approval: `heraut release`.

Per-env strategies need `--env <name>` on every command. A tag format containing `{build}` also needs
`--set-version <v> --set-build-id <id>`.

## Commands

| Command | Does | `--json` | Changes things |
|---|---|---|---|
| `heraut check` | config + runtime preflight (`heraut check config`, `heraut check runtime` run one half) | yes | no |
| `heraut version next` | prints the next tag | yes | no |
| `heraut version current` | prints the latest released tag (`--bare`, `--include-pre-release`) | yes | no |
| `heraut commit verify "<message>"` | validates one conventional-commit message | yes | no |
| `heraut commit check [range]` | validates every commit of a range | no | no |
| `heraut commit tickets [range]` | validates the `commits.tickets` patterns | no | no |
| `heraut notes <tag>` | prints the release notes of a tag (`--format markdown\|text`, `--from <tag>`, `--output <file>`) | no | no (writes only `--output`) |
| `heraut whatsnew` | release notes newer than the running heraut build | no | no |
| `heraut release --dry-run` | the release plan | yes (with `--dry-run`) | no |
| `heraut release` | version, changelog, commit, tag, push, publish | no | **yes** |
| `heraut changelog` | writes `CHANGELOG.md`; `--commit`, `--tag`, `--no-push` | no | file; with flags commits and tags |
| `heraut version sprint bump` | increments the CalVer sprint counter (`--dry-run` previews) | no | edits `.heraut.yml` |
| `heraut init --defaults` | writes a default `.heraut.yml` (`--overwrite` to replace) | no | writes the config |
| `heraut skill install` | writes this skill to `.claude/skills/heraut/SKILL.md` (`--dir`, `--force`) | no | writes the skill |
| `heraut skill print` | prints this skill | no | no |

`--dry-run` on `release` and `changelog` executes nothing: no git writes, no network, no hooks run.

## JSON output

One line on stdout:

```json
{"ok":true,"data":{"tag":"v1.4.0","version":"1.4.0","current_tag":"v1.3.0","bump":"minor"},"warnings":[]}
{"ok":false,"warnings":[],"error":{"code":"config","message":"…","hint":"…"}}
```

`error.code` is `usage`, `config`, `runtime`, `promotion` or `internal`; a promotion guard reports
`E001` (target tag exists), `E002` (destination ahead of source) or `E003` (no source tags).
`check` failures keep their findings in `data` next to `ok:false`. The exit code is the same with and
without `--json`. A flag or subcommand typo is rejected by the argument parser before heraut runs:
stdout stays empty, the message is on stderr, exit 1.

## Exit codes

| Code | Meaning | What to do |
|---|---|---|
| 0 | success | — |
| 1 | usage error (bad flag, no terminal for an interactive command, invalid commit message) | fix the call |
| 2 | configuration error (invalid YAML, missing `--env`, branch guard, `{build}` without an ID) | fix `.heraut.yml` or the flags; `heraut check config --json` lists the findings |
| 3 | runtime error (git, token, network, hook, publish failed) | read `error.message`; retry only if transient |
| 4 | promotion guard (E001–E003) | do not force: report to the user, `--force` overrides a safety check |
| 70 | internal error | report it |

## Reading a plan

`release --dry-run --json` returns `data.steps` in execution order. Each step has an `action`:
`hook` (with `point` and the rendered `commands`), `changelog`, `commit_changelog`, `tag`, `push_tag`,
`notes`, `publish` (with `target`, `notes`, `assets`, `pre_release`). `data.pre_release` is true for a
SemVer pre-release, which publishes as a pre-release and never writes `CHANGELOG.md`.

## Release notes for a store

`heraut notes <tag> --format text` prints the notes of the range from the previous release to `<tag>`
as plain text, with no Markdown characters. A `notes:` block in `.heraut.yml` changes its template.
