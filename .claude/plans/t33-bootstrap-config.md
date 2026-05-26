# T33 — Bootstrap heraut's own `.heraut.yml`

## Context

Heraut currently has no `.heraut.yml` at its own repo root, so `heraut release` cannot be
run against the heraut repo itself. T33 closes that gap by adding the config file and
generating the initial `CHANGELOG.md`. The pipeline intentionally stops after tagging:
GoReleaser + the existing `release.yml` workflow own binary builds and GitHub Release
creation. Heraut's role is version resolution → changelog generation → commit → tag.

This is **Option A**: heraut owns version/changelog/tag; GoReleaser owns binaries + GH
Release. Option B (heraut fully drives GitHub Release) is deferred and tracked in
`docs/ideas.md`.

---

## What to create / modify

### 1. `.config/heraut.yml` (new file)

`.config/heraut.yml` is the highest-priority auto-discovery path (ahead of `.heraut.yml`)
and keeps all tool configs together under `.config/`.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
version: "1"

versioning:
  strategy: semver
  prefix: "v"
  initial_version: "0.1.0"
  bump: auto

changelog:
  generator: git-cliff
  config: .config/cliff.toml
  output: CHANGELOG.md
```

Key decisions:
- **No `release` block** — pipeline stops after tagging; no platforms configured.
- **`config: .config/cliff.toml`** — the hand-crafted cliff config already in the repo (3.7 KB,
  includes GitHub remote, PR links, proper commit taxonomy). Using it instead of the embedded
  default ensures the changelog style is consistent with what the existing `release.yml`
  workflow already produces.
- **`bump: auto`** — conventional commits drive the bump automatically.
- **`tag_type` omitted** — defaults to annotated (T28 confirmed annotated is the standard).
- **`initial_version: "0.1.0"`** — matches the earliest existing tag; only used when no tags
  exist (not the case here, but required semantically).

### 2. `CHANGELOG.md` (new file, generated)

Generate the initial changelog by running git-cliff against all existing tags (`v0.1.0`
through `v0.8.0` and the current `main` commits):

```bash
git cliff --config .config/cliff.toml --output CHANGELOG.md
```

This file is committed and tracked (not gitignored). The roadmap explicitly states
"CHANGELOG.md added to the repo (generated, not hand-maintained)". Future `heraut release`
runs will regenerate and re-commit it on each release.

### 3. `docs/tasks/roadmap.md`

Mark T33 `[x]` and add the completion note.

---

## Validation steps

```bash
# 1. Validate the config parses and passes semantic validation
heraut check config

# 2. Verify runtime tools are available
heraut check runtime

# 3. Confirm next version resolves correctly against v0.8.0
heraut version next

# 4. (Dry-run the full pipeline to see what would happen)
heraut release --dry-run
```

Expected outcomes:
- `heraut check config` → no errors
- `heraut version next` → `v0.9.0` (or appropriate next version based on commits since v0.8.0)
- `heraut release --dry-run` → resolves version, generates CHANGELOG.md output, shows commit + tag steps, stops (no platform publish)

---

## Files touched

| File | Action |
|------|--------|
| `.config/heraut.yml` | Create |
| `CHANGELOG.md` | Create (generated via `git cliff`) |
| `docs/tasks/roadmap.md` | Mark T33 `[x]`, add note |

`.gitignore` does **not** need changes — `CHANGELOG.md` is not currently listed there and
should be committed/tracked.

---

## Commit

One conventional commit:

```
chore(config): bootstrap heraut's own .heraut.yml and initial CHANGELOG.md
```

Body: explain Option A split (heraut owns version/changelog/tag, GoReleaser owns
binaries/GH Release), and note the pipeline stops after tagging.
