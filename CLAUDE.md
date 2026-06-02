@.claude/rules/workflow.md
@.claude/rules/testing.md
@.claude/rules/coding.md
@.claude/rules/claude.md

# CLAUDE.md — Héraut

Héraut (`heraut`) is a Go CLI that orchestrates release management for git-based projects:
it resolves the next version, generates changelogs and release notes, creates a tag, and
publishes the release to GitHub or GitLab. The name is a French pun — "héraut" (a herald
announcing the release) sounds like "hero", which is what good release automation should
feel like (see [ADR-0002](docs/adr/0002-tool-name-heraut.md)).

## What this tool does

```
heraut release          # resolve next version → changelog → commit → tag → publish → notes
heraut changelog        # changelog only (optionally commit + tag)
heraut version next     # print the next version without side effects
heraut version current  # print the latest released tag (per active strategy / env)
heraut version sprint bump  # increment sprint counter in .heraut.yml (CalVer)
heraut check            # preflight: config + runtime + cliff (effective config)
heraut cliff <mode>     # show the effective merged git-cliff TOML
heraut init             # interactive wizard to generate .heraut.yml
heraut self-update      # download + verify + atomically replace the binary
```

Four versioning strategies are supported (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), three content generators (`git-cliff`, `communique`, `cocogitto`),
and two platforms (`github`, `gitlab`). See [`docs/specs/`](docs/specs/) for the full
behavioural spec.

## Docs

- [`docs/specs/`](docs/specs/) — behavioural specification (read before changing CLI surface or config schema)
- [`docs/adr/`](docs/adr/) — architecture decision records (19 ADRs, numbered consecutively)
- [`docs/tasks/`](docs/tasks/) — build roadmap with inline task checklist (`roadmap.md`)

## Tech stack

| Tool                          | Role                                                                |
|-------------------------------|---------------------------------------------------------------------|
| **Go** (via mise)             | Implementation language (see [ADR-0001](docs/adr/0001-language-go.md)) |
| **cobra**                     | CLI subcommand structure                                            |
| **fang**                      | cobra wrapper: styled help/errors, `--version`, completions (see [ADR-0003](docs/adr/0003-cli-framework-cobra-fang.md)) |
| **huh**                       | Interactive forms used by `heraut init`                             |
| **bubbles/spinner**           | Spinner for long-running pipeline steps                             |
| **lipgloss v2**               | Terminal styling                                                    |
| **yaml.v3**                   | `.heraut.yml` parsing (see [ADR-0004](docs/adr/0004-config-format-yaml.md)) |
| **JSON Schema**               | `schema.json` for IDE validation of `.heraut.yml`                   |
| **`//go:embed`**              | Embedded git-cliff / cocogitto defaults                             |
| **goreleaser**                | Cross-platform release builds, raw binaries (see [ADR-0013](docs/adr/0013-raw-binary-goreleaser-format.md)) |
| **git-cliff / glab / gh / cog / communique** | External CLIs orchestrated by heraut (not bundled)       |

## Project layout

```
cmd/heraut/main.go              entry point — fang.Execute(cmd.NewRootCmd())

internal/
   cmd/                         cobra command definitions (package cmd)
      root.go                   root command, persistent flags
      release.go                heraut release
      changelog.go              heraut changelog
      version.go                heraut version next / current
      version_sprint.go         heraut version sprint bump
      check.go                  heraut check config / runtime / cliff
      cliff.go                  heraut cliff changelog / release-notes
      init.go                   heraut init
      self_update.go            heraut self-update
   port/                        interfaces — Runner, Generator, Platform
   adapter/exec/                shell runner implementing port.Runner
   testutil/                    MockRunner, FakeBin, constants
   ui/                          lipgloss styles, step spinner, version banner
   config/                      structs, loader, path resolution, validator, errors
   versioning/
      result.go                 shared Result type
      tagfmt/                   {version}/{env} substitution + glob patterns
      semver/                   conventional-commit bump + version arithmetic
      calver/                   token parser (YYYY/MM/DD/WW/QQ/SS/SPRINT/PATCH)
      perenv/                   generic per-env wrapper over a VersionCalculator
   generators/
      gitcliff/                 embedded TOML defaults + user override merge
      communique/               wrapper around `communique generate`
      cocogitto/                4-path config resolution + embedded cog.toml + Tera
   platforms/
      github/                   `gh release create` + asset upload (with contract tests)
      gitlab/                   `glab release create` + asset upload (with contract tests)
   pipeline/
      release.go                full release flow (resolve → changelog → tag → publish)
      changelog.go              changelog-only flow
      config.go                 Pipeline.Config struct
   app/                         wiring layer — NewResolver(), BuildPipeline(), BuildChangelogPipeline()
   scaffold/                    heraut init wizard + YAML generation
   selfupdate/                  GitHub Releases API + atomic binary replace

testdata/                       repo-wide read-only test fixtures (.heraut.yml samples, …)

docs/specs/                     6 numbered specs (behavioural authority)
docs/adr/                       19 ADRs (architectural decisions)
docs/tasks/                     roadmap.md (build plan + inline task checklist)

schema.json                     published JSON Schema for .heraut.yml IDE validation
.goreleaser.yml                 raw-binary release config (no archives — see ADR-0013)
Dockerfile                      bundled image: heraut + all external CLIs (see ADR-0016)
.github/workflows/              ci.yml (PR build/test/lint), release.yml (tag → GoReleaser + Docker push)
```

## Tooling (mise)

All tooling is managed by [mise](https://mise.jdx.dev). Config lives in `.config/mise/`.

```bash
mise install              # install all tools + run hk install automatically
mise run build            # compile to ./heraut
mise run test             # go test ./...
mise run lint:check       # hk check — all linters
mise run lint:fix         # hk fix   — auto-fix all linters
mise run lint:go:check    # golangci-lint run
mise run lint:go:fix      # golangci-lint run --fix
mise run run -- <args>    # run the CLI in dev mode (e.g. mise run run -- release --dry-run)
```

For targeted lint fixes: `hk fix -S <linter>` (e.g. `hk fix -S golangci-lint`,
`hk fix -S yamlfmt`).

Go, golangci-lint, goreleaser, and git-cliff are installed via mise (see
`.config/mise/config.toml`).

## ldflags invariant

`Dockerfile` and `.goreleaser.yml` both inject the build-time version via `-ldflags`.
**They must stay identical.** GoReleaser is the source of truth; the Dockerfile carries
a comment pointing there. The Dockerfile receives the version via `--build-arg
HERAUT_VERSION=${{ github.ref_name }}` in the release workflow.

| ldflag         | Purpose                                              |
|----------------|------------------------------------------------------|
| `main.Version` | Running binary's version string (`heraut --version`) |

`main.Version` is the only ldflag. The project URL and the GitHub Releases API endpoint
used by `heraut self-update` are **compiled-in constants** (`defaultProjectURL`,
`defaultLatestURL`) in `internal/selfupdate/updater.go`, not ldflags — heraut targets a
single fixed public repo, so they never vary per build (see
[ADR-0014](docs/adr/0014-self-update-architecture.md)).

## Config file discovery

`config.ResolvePath` checks in order: `--config` flag → `HERAUT_FILE` env var →
`.config/heraut.yml` → `.heraut.yml`. `HERAUT_FILE` is the easiest way to inject a
non-default path in CI pipelines without touching command invocations.

## Bundled external CLIs (not installed by heraut)

heraut invokes `git`, `git-cliff`, `glab`, `gh`, `cog`, and `communique` via the
`port.Runner` abstraction. None of these are bundled with the heraut binary — users
install them separately. `heraut check runtime` verifies they are on `PATH`.

## Non-obvious constraints

- `heraut release` requires at least one entry in `release.platforms` — omitting the
  `release` block (or leaving `platforms` empty) is a config error, not a silent no-op.
- `disable_changelog: true` per-env skips changelog generation and the commit, but **not**
  the tag when `--tag` is also passed. Use `heraut changelog --tag --env <env>` for
  tag-only flows on environments where changelog is disabled.

## When in doubt

1. Read the relevant `docs/specs/` section
2. If the spec is silent, read the relevant `docs/adr/`
3. If both are silent, ask the user before assuming a behaviour
