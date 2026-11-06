# Contributing to heraut

This page is for people (and coding agents) who work on heraut itself. To use heraut, start with the
[README](README.md) and the [user docs](docs/README.md).

## Set up

heraut's tooling is managed by [mise](https://mise.jdx.dev): Go, golangci-lint, hk and the linters.

```bash
mise install          # installs the tools, then installs the git hooks (hk)
mise run build        # builds ./heraut
mise run test         # go test ./...
mise run lint:check   # every linter (hk check)
mise run lint:fix     # fixes what can be fixed (hk fix)
mise run run -- release --dry-run   # runs the CLI from source
mise run docs:gen     # regenerates docs/reference/commands from the commands' help text
```

The hooks run on every commit: linters on the staged files, a typo check, and `heraut commit verify`
on the commit message. Don't bypass them (`--no-verify`); fix what they report.

## How work is done

- **Read before changing behaviour.** [`docs/specs/`](docs/specs/) is the behavioural specification,
  [`docs/adr/`](docs/adr/) records why things are the way they are. If both are silent, ask.
- **Roadmap first.** Work is planned in [`docs/tasks/roadmap.md`](docs/tasks/roadmap.md) (and the
  dedicated roadmaps it points to). A task is done when its marker is flipped to `[x]` and a short note
  says what was decided.
- **Tests first.** Write the failing test, then the code. The test layers (unit, contract, integration,
  schema, e2e) and their rules are in [`.claude/rules/testing.md`](.claude/rules/testing.md).
- **Conventional commits**, subject at most 72 characters: `feat(cmd): …`, `fix(config): …`,
  `docs(guides): …`. Types and scopes: [`.claude/rules/workflow.md`](.claude/rules/workflow.md).
- **Architecture.** heraut is hexagonal (ports and adapters); which package may import which is in
  [`.claude/rules/coding.md`](.claude/rules/coding.md), with the rules on errors, config and comments.
- **Config changes** update `internal/config`, [`schema.json`](schema.json) and
  [`docs/heraut.sample.yml`](docs/heraut.sample.yml) together.
- **Command help is user documentation.** A command's `Short:`, `Long:`, `Example:` and flag text
  become [`docs/reference/commands/`](docs/reference/commands/). After changing them, run
  `mise run docs:gen`; a test fails while the pages are stale.

The files in [`.claude/rules/`](.claude/rules/) are the full rules; [`CLAUDE.md`](CLAUDE.md) is the
project overview for coding agents.

## End-to-end tests

The `e2e/` tree runs the built binary. The hermetic lane runs with `go test ./...`; the forge-sandbox
lane runs against real GitHub and GitLab repositories, on demand. Setup and scenarios:
[End-to-end tests](docs/contributing/e2e-tests.md).

## Releases

Releases are cut by running the `Release` workflow by hand. GoReleaser builds the binaries, then the
fresh heraut binary releases itself (tag, changelog, GitHub release). See
[ADR-0018](docs/adr/0018-ci-build-then-release-pipeline.md).

## Where things are

| Path | Contents |
|---|---|
| [`docs/specs/`](docs/specs/) | Behavioural specification |
| [`docs/adr/`](docs/adr/) | Architecture decision records |
| [`docs/tasks/`](docs/tasks/) | Roadmaps and task status |
| [`docs/contributing/`](docs/contributing/) | Contributor guides (e2e tests) |
| [`docs/ideas.md`](docs/ideas.md) | Post-v1.0 ideas, not scheduled yet |
| [`internal/`](internal/) | The code, one package per concern (see `CLAUDE.md` for the layout) |
| [`e2e/`](e2e/) | Tests against the built binary |
