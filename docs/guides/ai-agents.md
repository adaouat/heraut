# Let an AI agent drive heraut

A coding agent (Claude Code, or any agent that runs commands) can prepare a release for you: check
the setup, tell you the next version, show you the plan, and release once you agree. heraut ships
instructions written for agents, and machine-readable output for them to read.

## Give the agent the skill

The skill is a short Markdown file that tells an agent which commands are safe, in what order to run
them, how to read their output and what each exit code means. It is built into heraut, so it always
matches the version you have.

For Claude Code, install it in the project:

```console
$ heraut skill install
skill written to .claude/skills/heraut/SKILL.md
```

`--dir ~/.claude/skills` installs it for all your projects instead. Run the command again after
upgrading heraut: an unchanged file is left alone, and a file you edited is only replaced with
`--force`.

For another agent, print it and put it where that agent reads its instructions:

```bash
heraut skill print > docs/agents/heraut.md
```

## What the agent does

The skill tells the agent to work in this order, and to stop before anything that can't be undone:

1. `heraut check --json`: the configuration is valid, and git and the tokens are in place.
2. `heraut version next --json`: the version a release would create, and why.
3. `heraut release --dry-run --json`: every step the release would take. The agent shows you this
   plan.
4. `heraut release`, only after you approve it in the conversation.

The skill also tells the agent never to pass `--allow-major` or `--force`, nor to replace a file, unless
you asked, and to use `--offline` when it has no forge token.

## The JSON output

Commands that an agent reads take `--json`: `check` (and `check config`, `check runtime`),
`version next`, `version current`, `commit verify`, and `release --dry-run`. Each prints exactly one
line on stdout:

```console
$ heraut version next --json
{"ok":true,"data":{"tag":"v1.4.0","version":"1.4.0","current_tag":"v1.3.0","bump":"minor"},"warnings":[]}
```

The release plan lists the steps in the order they run:

```json
{"ok":true,"data":{"tag":"v1.4.0","version":"1.4.0","current_tag":"v1.3.0","pre_release":false,"steps":[
  {"action":"changelog","target":"CHANGELOG.md"},
  {"action":"commit_changelog","target":"CHANGELOG.md"},
  {"action":"tag","target":"v1.4.0"},
  {"action":"push_tag","target":"v1.4.0"},
  {"action":"notes"},
  {"action":"publish","target":"github","notes":true}
]},"warnings":[]}
```

A failure has `"ok":false`, no `data` (except `heraut check`, which keeps its findings), and an `error`
with a `code` (`usage`, `config`, `runtime`, `promotion`, or `E001`–`E003`), a `message` and sometimes
a `hint`. The exit code is the same as without
`--json`; [Exit codes](../reference/exit-codes.md) explains each one.

## Permissions

If your agent asks before running commands, it is safe to allow the read-only ones without asking:
`heraut check`, `heraut version next`, `heraut version current`, `heraut commit verify`,
`heraut commit check` and `heraut release --dry-run`. Keep `heraut release` and
`heraut changelog --commit` / `--tag` behind a confirmation: they push and publish.
