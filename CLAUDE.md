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
heraut check            # preflight: config + runtime
heraut init             # interactive wizard to generate .heraut.yml
heraut commit verify/check/create  # conventional-commit validation + interactive authoring
heraut whatsnew         # release notes for versions newer than the running build
```

Four versioning strategies are supported (`semver`, `calver`, `semver-per-env`,
`calver-per-env`). Forges (`github`, `gitlab`, `azure_devops`) supply PR/MR commit
enrichment; only `github` and `gitlab` also have a publish driver — Azure DevOps has no
equivalent of a GitHub/GitLab Release. See [`docs/specs/`](docs/specs/) for the full
behavioural spec.

## Docs

- [`docs/specs/`](docs/specs/) — behavioural specification (read before changing CLI surface or config schema)
- [`docs/adr/`](docs/adr/) — architecture decision records (51 ADRs, numbered consecutively)
- [`docs/tasks/`](docs/tasks/) — build roadmap (`roadmap.md`) plus dedicated per-epic roadmaps
  (`forge-abstraction-roadmap.md`, `native-generator-roadmap.md`, `release-config-roadmap.md`,
  `docs-audit-roadmap.md`) that `roadmap.md` points to for large, multi-task epics

## Tech stack

| Tool                          | Role                                                                |
|-------------------------------|---------------------------------------------------------------------|
| **Go** (via mise)             | Implementation language (see [ADR-0001](docs/adr/0001-language-go.md)) |
| **cobra**                     | CLI subcommand structure                                            |
| **github.com/adaouat/forge**  | Shared library: CLI execution wrapping cobra + fang (`forge/cli`, see [ADR-0003](docs/adr/0003-cli-framework-cobra-fang.md)), exec `Runner` + test doubles (`forge/exec`), config loading (`forge/config`), exit codes (`forge/exitcode`), UI theming/spinner (`forge/ui`), update check + `whatsnew` (`forge/updatecheck`), structured logging (`forge/log`) |
| **huh**                       | Interactive forms used by `heraut init`                             |
| **lipgloss v2**               | Terminal styling — heraut's gold accent, layered over `forge/ui`    |
| **yaml.v3**                   | `.heraut.yml` parsing (see [ADR-0004](docs/adr/0004-config-format-yaml.md)) |
| **JSON Schema**               | `schema.json` for IDE validation of `.heraut.yml`                   |
| **`//go:embed`**              | Embedded native generator templates (`internal/generators/native/`) + `CHANGELOG.md` (offline fallback for `whatsnew`) |
| **goreleaser**                | Cross-platform release builds, raw binaries (see [ADR-0013](docs/adr/0013-raw-binary-goreleaser-format.md)) |
| **glab / gh**                 | External CLIs orchestrated by heraut (not bundled)                  |

## Project layout

```
cmd/heraut/main.go              entry point — forge/cli.Run(cmd.NewRootCmd(version), …)
changelog.go                    //go:embed CHANGELOG.md — offline fallback for `whatsnew`

internal/
   cmd/                         cobra command definitions (package cmd)
      root.go                   root command, persistent flags, whatsnew + update-hint wiring
      release.go                heraut release
      changelog.go              heraut changelog
      version.go                heraut version next / current
      version_sprint.go         heraut version sprint bump
      check.go                  heraut check config / runtime
      init.go                   heraut init
      offline.go                --offline flag → forces commits.enrichment_policy: disabled
      exit.go                   maps pipeline errors to internal/exitcode codes
      commit.go                 heraut commit verify / check / create
   port/                        interfaces — Runner (alias to forge/exec.Runner), Generator, Platform, Forge
   exitcode/                    re-exports forge/exitcode + heraut's own Promotion (E001/E002/E003) code
   testutil/                    MockGenerator, MockPlatform, RealGitRepo, CI-env-clearing helpers
   ui/                          gold accent + huh theme (wrapping forge/ui), version banner, StepFn
   config/                      structs, loader, path resolution, validator, errors
   forge/                       forge identity resolution (config/CI/git-origin) + direct net/http
                                 PR/MR-enrichment clients — github/, gitlab/, azure/ (ADR-0043)
   commitwizard/                interactive Conventional Commits authoring, backs `heraut commit create`
   conventionalcommit/          pure commit-message parser (type/scope/breaking/footers), no heraut imports
   versioning/
      result.go                 shared Result type
      resolver.go, static.go    Resolver interface + the --version/--build override path
      tagfmt/                   {version}/{env}/{build} substitution + glob patterns
      semver/                   conventional-commit bump + version arithmetic
      calver/                   token parser (YYYY/MM/DD/WW/QQ/SS/SPRINT/PATCH)
      perenv/                   generic per-env wrapper over a VersionCalculator
   generators/
      native/                   built-in, zero-external-dependency generator — commit walk + classification + embedded template rendering (ADR-0032, sole generator since ADR-0045)
   platforms/
      github/                   `gh release create` + asset upload (with contract tests)
      gitlab/                   `glab release create` + asset upload (with contract tests)
   pipeline/
      release.go                full release flow (resolve → changelog → tag → publish)
      changelog.go              changelog-only flow
      config.go                 Pipeline.Config struct
      git.go, linkctx.go, warn.go   git plumbing, link-context derivation, degraded-run warnings
   app/                         wiring layer — NewResolver(), BuildPipeline(), BuildChangelogPipeline()
   scaffold/                    heraut init wizard + YAML generation

pkl/                            Pkl builtin used by `heraut commit verify`'s own commit-msg hook (ADR-0029)
testdata/                       repo-wide read-only test fixtures (.heraut.yml samples, …)

docs/specs/                     6 numbered specs (behavioural authority)
docs/adr/                       51 ADRs (architectural decisions)
docs/tasks/                     roadmap.md + dedicated per-epic roadmaps (see § Docs above)
docs/guides/                    task-oriented how-tos, distinct from the behavioural specs

LICENSE.md                      project license
schema.json                     published JSON Schema for .heraut.yml IDE validation
.goreleaser.yml                 raw-binary release config (no archives — see ADR-0013)
Dockerfile                      bundled image: heraut + all external CLIs (see ADR-0016)
.github/workflows/              ci.yml (PR test/lint/build), release.yml (workflow_dispatch —
                                 builds via GoReleaser, then the fresh heraut binary creates its
                                 own tag/release — ADR-0018), osv-scan.yml (dependency scanning)
```

## Tooling (mise)

All tooling is managed by [mise](https://mise.jdx.dev). Config lives in `.config/mise/`.

```bash
mise install              # install all tools + run hk install automatically
mise run build            # compile to ./heraut
mise run test             # go test ./...
mise run lint:check       # hk check — all linters
mise run lint:fix         # hk fix   — auto-fix all linters
mise run lint:go:check    # hk check -S golangci_lint
mise run lint:go:fix      # hk fix -S golangci_lint
mise run run -- <args>    # run the CLI in dev mode (e.g. mise run run -- release --dry-run)
```

For targeted lint fixes: `hk fix -S <linter>` (e.g. `hk fix -S golangci_lint`,
`hk fix -S yamlfmt`) — note the underscore in `golangci_lint`, the actual `.config/hk/config.pkl`
step id; a hyphenated `golangci-lint` matches no step.

Go, golangci-lint, and goreleaser are installed via mise (see
`.config/mise/config.toml`).

## ldflags invariant

`Dockerfile` and `.goreleaser.yml` both inject the build-time version via `-ldflags`.
**They must stay identical.** GoReleaser is the source of truth; the Dockerfile carries
a comment pointing there. The Dockerfile receives the version via `--build-arg
HERAUT_VERSION=${{ needs.release.outputs.tag }}` in the release workflow (`workflow_dispatch`-only
— see [ADR-0018](docs/adr/0018-ci-build-then-release-pipeline.md); there is no `v*` tag trigger).

| ldflag         | Purpose                                              |
|----------------|------------------------------------------------------|
| `main.Version` | Running binary's version string (`heraut --version`) |

`main.Version` is the only ldflag. heraut's GitHub repo (`adaouat/heraut`) is a hardcoded
string literal passed to `forge/updatecheck.WhatsNewCommand` and `updatecheck.Hinter` in
`internal/cmd/root.go`, not a build-time constant — heraut targets a single fixed public
repo and no longer self-replaces its binary (see
[ADR-0014](docs/adr/0014-self-update-architecture.md), superseded by forge ADR-0005).

## Config file discovery

`config.ResolvePath` checks in order: `--config` flag → `HERAUT_FILE` env var →
`.config/heraut.yml` → `.heraut.yml`. `HERAUT_FILE` is the easiest way to inject a
non-default path in CI pipelines without touching command invocations.

## Bundled external CLIs (not installed by heraut)

heraut invokes `git`, `glab`, and `gh` via the `port.Runner` abstraction for **publishing**
and git plumbing. None of these are bundled with the heraut binary — users install them
separately. `heraut check runtime` verifies they are on `PATH`. PR/MR commit **enrichment**
is a separate concern and does *not* shell out to `gh`/`glab`: `internal/forge/{github,
gitlab,azure}` talk to each platform's API directly over `net/http` (ADR-0043).

## Non-obvious constraints

- `heraut release` requires at least one **resolvable** publish destination — an explicit
  `release.targets` entry, or a forge that auto-detects from CI/git origin *and* has a
  publish driver (`github`/`gitlab`; an auto-detected `azure_devops`-only forge does not
  count, since Azure DevOps has no publish driver at all). Omitting the `release` block
  with zero resolvable destinations is a config error, not a silent no-op.
- `disable_changelog: true` per-env skips changelog generation and the commit, but **not**
  the tag when `--tag` is also passed. Use `heraut changelog --tag --env <env>` for
  tag-only flows on environments where changelog is disabled.
- The global `--offline` flag overrides config: it forces `commits.enrichment_policy:
  disabled` for the run regardless of what `.heraut.yml` sets, skipping PR/MR enrichment in
  changelog and release-notes generation (`internal/cmd/offline.go`).

## When in doubt

1. Read the relevant `docs/specs/` section
2. If the spec is silent, read the relevant `docs/adr/`
3. If both are silent, ask the user before assuming a behaviour
