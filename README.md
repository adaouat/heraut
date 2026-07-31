<p align="center">
  <img src="docs/images/lockup-horizontal.png" alt="héraut" width="420">
</p>

<p align="center"><em>Every release deserves a héraut.</em></p>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT">
  <img src="https://img.shields.io/badge/go-1.26%2B-00ADD8.svg" alt="Go 1.26+">
</p>

---

**Héraut** (`heraut`) is a Go CLI that orchestrates release management for git-based
projects. One command resolves the next version, generates the changelog and release
notes, creates the git tag, and publishes the release to GitHub and/or GitLab. It wraps
the tools you already use — `git`, `git-cliff`, `gh`, `glab`, `communique` — and handles
the glue they can't: version resolution for prefixed-tag strategies, generator/platform
composition, and strict config validation.

The name is a French pun. *Héraut* means herald — the medieval messenger who announces
news — and it sounds like *hero*, which is what good release automation should feel like.

It supports **four versioning strategies** (`semver`, `calver`, `semver-per-env`,
`calver-per-env`), **two content generators** (`git-cliff`, `communique`),
and **two platforms** (`github`, `gitlab`).

## Install

### Homebrew

```bash
brew install --cask adaouat/tap/heraut
```

### `go install`

```bash
go install github.com/adaouat/heraut/cmd/heraut@latest
```

### mise

```bash
mise use github:adaouat/heraut
```

Or declare it in your `.mise.toml` / `mise.toml`:

```toml
[tools]
"github:adaouat/heraut" = "latest"
```

### Prebuilt binary

Download the raw binary for your platform from the
[releases page](https://github.com/adaouat/heraut/releases). Assets are named
`heraut_<version>_<os>_<arch>` (no archive wrapper — see
[ADR-0013](docs/adr/0013-raw-binary-goreleaser-format.md)), alongside a
`checksums.txt` for verification.

```bash
# example: macOS arm64 — replace <version> with the release tag
curl -L -o heraut "https://github.com/adaouat/heraut/releases/download/<version>/heraut_<version>_darwin_arm64"
chmod +x heraut
./heraut --version
```

Once installed, heraut prints a one-line upgrade hint when a newer release exists; re-run
your install method (`mise upgrade heraut`, `go install …@latest`, or the curl command) to
upgrade.

> **macOS / Gatekeeper:** if the binary is blocked after download, clear the quarantine flag:
> ```bash
> xattr -d com.apple.quarantine heraut
> ```

### Docker

```bash
docker run --rm ghcr.io/adaouat/heraut:latest --version

# run against the current repo
docker run --rm -v "$PWD":/repo -w /repo ghcr.io/adaouat/heraut:latest release --dry-run
```

Available tags (Docker images do not carry the `v` prefix that git tags use):

| Tag | Meaning |
|-----|---------|
| `latest` | Latest release |
| `0.9.0` | Exact version |
| `0.9` | Latest patch of 0.9.x |
| `0` | Latest release of major 0 |

## Prerequisites

When running via **binary or `go install`**, heraut does **not** bundle the external CLIs it
orchestrates — install the ones your config uses and make sure they are on `PATH`.
The **Docker image** bundles all of them at pinned versions; no extra setup needed.

| Tool | Needed for |
|------|------------|
| `git` | always |
| `git-cliff` | `generator: git-cliff` |
| `communique` | `generator: communique` |
| `gh` | `platform: github` |
| `glab` | `platform: gitlab` |

Run `heraut check runtime` to verify the tools and tokens for your config are available.

## Quickstart

```bash
# 1. Generate a .heraut.yml interactively (or `heraut init --defaults` for an opinionated default)
heraut init

# 2. Validate the config offline — parse + semantic checks, no network
heraut check config

# 3. Preview the full pipeline without side effects
heraut release --dry-run

# 4. Cut the release: resolve version → changelog → commit → tag → publish
heraut release
```

## Commands

| Command | Description |
|---------|-------------|
| `heraut release` | Resolve next version → changelog → commit → tag → publish → notes |
| `heraut changelog` | Generate `CHANGELOG.md` only (optionally `--commit` / `--tag`) |
| `heraut version next` | Print the next version without side effects |
| `heraut version current` | Print the latest released tag for the active strategy / env |
| `heraut version sprint bump` | Increment the CalVer sprint counter in `.heraut.yml` |
| `heraut check` | Preflight: config + runtime + cliff (`config` / `runtime` / `cliff` subcommands) |
| `heraut cliff <mode>` | Print the effective merged git-cliff TOML (`changelog` / `release-notes`) |
| `heraut init` | Interactive wizard to generate `.heraut.yml` |

Global flags (on every command): `--config`, `--dry-run`, `--verbose`, `--env`,
`--force`, `--version`/`-v`, `--help`/`-h`. See
[Spec 03 — Commands](docs/specs/03-commands.md) for the full reference.

## Configuration

Configuration lives in `.heraut.yml` (or `.config/heraut.yml`). Add the schema header for
IDE autocomplete and inline validation in any editor with YAML Language Server support:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump: auto

changelog:
  generator: git-cliff
  output: CHANGELOG.md

forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN

release:
  notes:
    generator: git-cliff
  targets:
    - forge: github
```

| Block | Purpose |
|-------|---------|
| `versioning` | Strategy and options (`semver` / `calver` / `*-per-env`) |
| `changelog` | Generator for `CHANGELOG.md` (committed during `release`) |
| `forges` | Code-hosting connections heraut talks to (`github` / `gitlab`, or both); optional when auto-detected from CI or git origin |
| `release.notes` | Generator for the release-page notes |
| `release.targets` | Publish destinations, each referencing a `forges[].name` |
| `environments` | Per-environment config for `*-per-env` strategies (bump mode, tag format, promotion source, changelog/release overrides) |

This is the short version. See [Spec 02 — Configuration](docs/specs/02-configuration.md)
for every field, [Spec 04 — Versioning](docs/specs/04-versioning.md) for how each
strategy computes the next version, and [`docs/heraut.sample.yml`](docs/heraut.sample.yml)
for a fully annotated config covering all fields with comments.

## Updates

After any command, heraut runs a non-blocking, once-per-day check against the GitHub
Releases API and prints a one-line hint — with the matching upgrade command — when a newer
version exists. Disable the check with `HERAUT_CHECK_UPDATE=false`.

heraut does not self-replace its binary: upgrades go through your install method
(`mise upgrade heraut`, `go install …@latest`, Homebrew, or re-running the curl command).

## Documentation

- [`docs/specs/`](docs/specs/) — behavioural specification (the authority for users)
- [`docs/adr/`](docs/adr/) — architecture decision records

## License

[MIT](LICENSE.md)
