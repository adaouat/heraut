<p align="center">
  <img src="docs/images/lockup-horizontal.png" alt="héraut" width="420">
</p>

<p align="center"><em>Every release deserves a héraut.</em></p>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT">
  <img src="https://img.shields.io/badge/go-1.27%2B-00ADD8.svg" alt="Go 1.27+">
</p>

---

Every team that tags releases ends up with a script. It bumps a version, writes a
changelog entry, creates the tag, pushes a release to GitHub. It works — until someone
adds a second environment, or the changelog format needs a tweak, or a new teammate has
to figure out what it does. That script is now yours to maintain.

**Héraut replaces it.** One command — `heraut release` — resolves the next version,
generates the changelog and release notes, tags, and publishes to GitHub and/or GitLab.
It wraps the tools you already use (`git`, `gh`, `glab`) and handles what a hand-rolled
script usually gets wrong: version resolution for prefixed-tag strategies, multi-forge
publishing, and config that's validated before it runs, not discovered broken in CI.
Config-driven, not script-driven — change behavior by editing `.heraut.yml`, not by
reading bash.

*The name's a French pun — [`héraut`](docs/adr/0002-tool-name-heraut.md) (herald) sounds
like `héros` (hero), which is the idea.*

## Install

### Homebrew

```bash
brew install --cask adaouat/tap/heraut
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
`heraut_<version>_<os>_<arch>` — no `.tar.gz`/`.zip` wrapper, just the binary — alongside
a `checksums.txt` for verification.

```bash
# example: macOS arm64 — replace <version> with the release tag
curl -L -o heraut "https://github.com/adaouat/heraut/releases/download/<version>/heraut_<version>_darwin_arm64"
chmod +x heraut
./heraut --version
```

Once installed, heraut prints a one-line upgrade hint when a newer release exists; re-run
your install method (`mise upgrade heraut`, `go install …@latest`, or the curl command) to
upgrade.

> **macOS / Gatekeeper:** heraut's binaries aren't notarized by Apple (yet), so macOS
> quarantines them on download and refuses to run them. Until that changes, clear the
> quarantine flag yourself before running the binary:
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
| `X.Y.Z` | Exact version, e.g. `0.58.0` |
| `X.Y` | Latest patch of that minor, e.g. `0.58` |
| `X` | Latest release of that major, e.g. `0` |

### `go install`

```bash
go install github.com/adaouat/heraut/cmd/heraut@latest
```

## Prerequisites

When running via **binary or `go install`**, heraut does **not** bundle the external CLIs it
orchestrates — install the ones your config uses and make sure they are on `PATH`.
The **Docker image** bundles all of them at pinned versions; no extra setup needed.

| Tool | Needed for |
|------|------------|
| `git` | always |
| `gh` | publishing to a GitHub `release.targets[]` entry |
| `glab` | publishing to a GitLab `release.targets[]` entry |

Neither is needed just to *enrich* a changelog with PR/MR data from a `github`/`gitlab`
forge — that talks to each platform's API directly over HTTP, no CLI involved. Changelog
and release-notes generation itself needs no external binary either — `native` (heraut's
built-in generator) ships in the `heraut` binary.

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

## What it handles

- **Versioning** — SemVer or CalVer, plus a per-environment variant of each for projects
  with independently-versioned lines (e.g. `staging` vs. `prod`). See
  [Spec 04 — Versioning](docs/specs/04-versioning.md).
- **Changelog & release notes** — built in (`native`), no external binary to install or
  pin a version of. See [Spec 05](docs/specs/05-generators-and-platforms.md).
- **Publishing** — GitHub and GitLab releases; Azure DevOps is supported for commit
  enrichment (PR/MR data in the changelog) but has no release API of its own to publish
  to. See [Spec 05](docs/specs/05-generators-and-platforms.md).

## Commands

The [Quickstart](#quickstart) above covers the core loop. Beyond that: `heraut changelog`
(changelog only, optionally `--commit`/`--tag`), `heraut version next`/`current` (print
without side effects), `heraut commit verify`/`create` (Conventional Commits tooling).
Every command takes `--dry-run` and `--help`; see
[Spec 03 — Commands](docs/specs/03-commands.md) for the full reference including every
global flag.

> **Gotcha:** `--version`/`-v` is root-only and prints the heraut binary's own version.
> `heraut release --version <value>` is a different, subcommand-local flag that
> *overrides the resolved release version* — same name, unrelated meaning.

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
  output: CHANGELOG.md

forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN

release:
  notes: {}
  targets:
    - forge: github
```

| Block | Purpose |
|-------|---------|
| `versioning` | Which strategy to use and how it computes the next version |
| `forges` | Code-hosting connections heraut talks to — often not needed at all, since it auto-detects from CI or your git origin |
| `release` | What to publish and where; `release.targets` references a `forges[].name` |

That's the shape, not the whole schema — `changelog` output, per-environment overrides
via `environments`, and every other field are in
[Spec 02 — Configuration](docs/specs/02-configuration.md). For a fully annotated example
covering every field, see [`docs/heraut.sample.yml`](docs/heraut.sample.yml).

## Updates

After any command, heraut runs a non-blocking, once-per-day check against the GitHub
Releases API and prints a one-line hint — with the matching upgrade command — when a newer
version exists. Disable the check with `HERAUT_CHECK_UPDATE=false`.

heraut does not self-replace its binary: upgrades go through your install method
(`mise upgrade heraut`, `go install …@latest`, Homebrew, or re-running the curl command).

## Documentation

- [`docs/specs/`](docs/specs/) — behavioural specification (the authority for users)
- [`docs/adr/`](docs/adr/) — architecture decision records
- [`docs/guides/`](docs/guides/) — task-oriented how-tos

## License

[MIT](LICENSE.md)
