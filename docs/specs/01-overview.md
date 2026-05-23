# Spec 01 — Overview

## What Héraut does

Héraut is a Go CLI that orchestrates a release: it resolves the next version, generates
the changelog and release notes, creates the git tag, and publishes the release to
GitHub and/or GitLab. It wraps existing tools (`git-cliff`, `glab`, `gh`, `cog`,
`communique`) and handles the custom logic those tools cannot handle natively — version
resolution for prefixed tag strategies, generator/platform composition, strict config
validation.

**Primary users**: developers and release engineers who want a single command to drive
the full release pipeline of a Go / Node / Python / … project, without writing custom
shell scripts to glue the underlying tools together.

The name is a French pun. *Héraut* means herald — the medieval messenger who announces
news. It sounds like *hero*, which is what good release automation should feel like.
See [ADR-0002](../adr/0002-tool-name-heraut.md).

## Architecture overview

```
Project repo (consumer)
└── .heraut.yml                     per-project config (versioning + generators + platforms)

heraut binary
├── reads .heraut.yml from project root
├── invokes git, git-cliff, glab, gh, cog, communique via port.Runner
└── publishes release(s) on configured platforms
```

Héraut is distributed as a raw cross-platform binary via GitHub Releases and as a
container image at `ghcr.io/adaouat/heraut`. There is no daemon, no server, no plugin
system. Users install it with `go install`, download the binary from GitHub Releases,
or run the Docker image. See [ADR-0013](../adr/0013-raw-binary-goreleaser-format.md).

### Internal architecture (high level)

```
cmd/heraut/         thin CLI: parses flags, calls app.*, displays UI
   ↓
internal/app/       wiring layer — NewResolver(), BuildPipeline(), BuildChangelogPipeline()
   ↓
internal/pipeline/  release + changelog flows
   ↓
internal/{versioning, generators, platforms}/      ports & adapters
   ↓
internal/adapter/exec/                              shell runner (port.Runner)
```

The layout is hexagonal: `internal/port/` defines the contracts (`Runner`, `Generator`,
`Platform`); `internal/adapter/`, `internal/generators/`, and `internal/platforms/`
implement them. Domain wiring lives only in `internal/app/` — `cmd/heraut/` never
constructs concrete implementations directly.

## Key concepts

**Strategy** — one of four versioning approaches: `semver`, `calver`, `semver-per-env`,
`calver-per-env`. The strategy is selected in `.heraut.yml` and determines how the next
version is computed. See [Spec 04 — Versioning](04-versioning.md).

**Generator** — produces text. `git-cliff`, `communique`, or `cocogitto`. Used
independently for `changelog` (writes a `CHANGELOG.md` to the repo) and `release.notes`
(text attached to the platform release). See
[Spec 05 — Generators and Platforms](05-generators-and-platforms.md).

**Platform** — creates a release on a hosting service. `github` (via `gh`) or `gitlab`
(via `glab`). A project can publish to multiple platforms in the same release.

**Environment** — a deployment target (e.g. `dev`, `staging`, `prod`) with its own tag
namespace and bump policy. Only used by the per-env strategies.

**Promotion** — copying a version from one environment to another (e.g. `dev/1.2.3` →
`prod/1.2.3`). The destination's tag format is rendered against the source's bare
version. Guards: E001 (target exists), E002 (destination ahead), E003 (no source tags).
See [ADR-0007](../adr/0007-version-promotion-error-handling.md) and
[ADR-0008](../adr/0008-promote-source-env.md).

**Dry-run** (`--dry-run`) — every command produces a human-readable plan of what it
would do, without side effects. No git operations, no network calls, no file writes
outside `/tmp`.

## Exit codes

| Code | Meaning                                                              |
|------|----------------------------------------------------------------------|
| 0    | Success                                                              |
| 1    | Usage error (bad flags or arguments)                                 |
| 2    | Configuration error (invalid YAML, missing required fields, semantic validation failure) |
| 3    | Runtime error (binary missing from PATH, token env var unset, network failure, git operation failed) |
| 4    | Promotion guard tripped (E001 / E002 / E003) — see [ADR-0007](../adr/0007-version-promotion-error-handling.md) |
| 70   | Internal software error (unexpected panic or unhandled condition)    |

## Boundaries

### In scope

- Single-repo projects (one `.heraut.yml`, one version per project)
- Four versioning strategies: SemVer, CalVer, SemVer per env, CalVer per env
- Two platforms: GitLab, GitHub
- Three content generators: git-cliff, communique, cocogitto
- JSON Schema for IDE validation of `.heraut.yml`
- Raw binary distribution via GitHub Releases + GHCR Docker image
- Self-update via the GitHub Releases API
  ([ADR-0014](../adr/0014-self-update-architecture.md))

### Out of scope (revisit later)

- **Monorepo support** — multiple packages with independent versions in one repo
- **Artifact publishing** — npm publish, PyPI, Docker Hub, etc. heraut creates the
  release and uploads pre-built assets via `release.platforms[*].assets`, but it does
  not build or publish the assets themselves
- **Rollback** — no command to undo a release
- **Notification integrations** — Slack, email, etc. on release
- **Plugin system** — custom platforms / generators require modifying the heraut source

## What heraut does not do

- It does not bundle the external CLIs (`git-cliff`, `glab`, `gh`, `cog`, `communique`).
  Users install them separately. `heraut check runtime` verifies they are on `PATH`.
- It does not commit your code. The user is responsible for the commits being released
  before `heraut release` runs. The one exception is the `CHANGELOG.md` commit that
  heraut creates as part of the release flow
  ([ADR-0012](../adr/0012-changelog-commit-ownership.md)).
- It does not push to remote outside the explicit operations documented in
  [Spec 03 — Commands](03-commands.md).
