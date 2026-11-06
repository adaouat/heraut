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
It drives `git` and the GitHub and GitLab APIs and handles what a hand-rolled
script usually gets wrong: version resolution for prefixed-tag strategies, multi-forge
publishing, and config that's validated before it runs, not discovered broken in CI.
Config-driven, not script-driven — change behavior by editing `.heraut.yml`, not by
reading bash.

*The name's a French pun — `héraut` (herald) sounds
like `héros` (hero), which is the idea.*

## Install

```bash
brew install --cask adaouat/tap/heraut       # Homebrew, with shell completions and a man page
mise use packslip:github.com/adaouat/heraut  # mise, checks the release signature first (mise 2026.9.2+)
```

heraut needs `git` on your `PATH`, nothing else. A prebuilt binary, Docker, `go install`, checking
a download and upgrading: see [Install](docs/install.md).

## A first look

```bash
heraut init                # answer a few questions, get a .heraut.yml
heraut check config        # check the config, offline
heraut release --dry-run   # see what would happen, change nothing
heraut release             # version, changelog, commit, tag, publish
```

Step by step, with what each command prints: [Your first release](docs/getting-started.md).

A small `.heraut.yml` for a project released on GitHub:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

changelog:
  output: CHANGELOG.md

release:
  notes: {}
```

heraut finds the GitHub or GitLab project from your CI or your git remote. The schema line gives
autocomplete and validation in editors that support the YAML language server.

## What it handles

- **Versions** from your commit messages ([Conventional Commits](https://www.conventionalcommits.org/)):
  SemVer or CalVer, and one version line per environment (`dev`, `staging`, `prod`) when you need it.
- **Changelog and release notes**, built in, with links to commits and pull requests, and templates you
  can override.
- **Publishing** to GitHub and GitLab, one or both, with files attached. Azure DevOps works for pull
  request data in the changelog; it has no releases to publish to.
- **Your own steps** before and after the tag or the release, from a single config file, checked before
  anything runs.

## Documentation

[Read the docs](docs/README.md).

Working on heraut itself? See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)
