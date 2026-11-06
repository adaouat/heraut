# Mobile apps: build numbers in tags

This guide covers projects that create multiple tagged builds per semantic
version — the typical mobile CI pattern:

```
main/7.4.0+155398    ← production release
uat/7.4.0+154392     ← first UAT build
uat/7.4.0+154572     ← second UAT build (same version, new build)
uat/7.4.0+155391
uat/7.4.1+158404
uat/7.4.1+158565
```

The build ID comes from the CI system (e.g. `$CI_PIPELINE_ID` on GitLab,
`$GITHUB_RUN_NUMBER` on GitHub Actions). The version (`7.4.1`) is managed
by the team and declared explicitly at pipeline time.

---

## Configuration

```yaml
# .config/heraut.yml
version: "1"

versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}+{build}"  # {build} must follow "+": it's SemVer build metadata, not a pre-release

changelog:
  # tag_pattern scopes changelog generation to the production env only.
  # UAT builds skip changelog entirely (disable_changelog: true).
  tag_pattern: "main/[0-9]+\\.[0-9]+\\.[0-9]+\\+[0-9]+"

environments:
  uat:
    bump: auto
    disable_changelog: true      # tag only — no CHANGELOG.md update
  main:
    bump: promote
    source: uat
```

**What `disable_changelog: true` does:** UAT builds create the git tag and
push it, but skip the changelog generation step and the changelog commit
entirely. The CHANGELOG.md is only updated on production (`main`) releases.

---

## CI commands

### UAT build (every pipeline run)

```bash
# GitLab CI example
heraut changelog \
  --tag \
  --env uat \
  --set-version "$APP_VERSION" \
  --set-build-id "$CI_PIPELINE_ID"
```

`APP_VERSION` is the semantic version your team maintains (e.g. `7.4.1`).
It can come from a file (`VERSION`, `package.json`, `pubspec.yaml`), a
pipeline variable set by the release manager, or any other source. heraut
does not derive it from commits — you own the version, heraut owns the tag.

This produces tag `uat/7.4.1+158404` and pushes it. No CHANGELOG.md
update occurs.

### Production release

```bash
# Tag + CHANGELOG.md update for the promoted production build
heraut changelog \
  --tag \
  --env main \
  --set-version "$APP_VERSION" \
  --set-build-id "$CI_PIPELINE_ID"
```

This produces tag `main/7.4.1+158404`, updates CHANGELOG.md with all
commits since `main/7.4.0+155398` (the previous production tag), commits
and pushes the changelog, then pushes the tag.

---

## Version display in the changelog

The changelog's version headings show the version alone: heraut drops the
environment and the build number it reads from `tag_format`, so a heading
reads `7.4.1`, not `main/7.4.1+159001`:

| Tag | Displayed as |
|---|---|
| `main/7.4.1+159001` | `7.4.1` |
| `main/7.4.1-rc.1+159001` | `7.4.1-rc.1` |
| `7.4.1+159001` | `7.4.1` |
| `v1.2.3` | `1.2.3` (unchanged — no build suffix) |

Only a trailing `+<digits>` build number is dropped, so ordinary tags are
unaffected. Nothing else to configure: `changelog.tag_pattern`, as above,
keeps the changelog to the production tags.

---

## Resulting CHANGELOG.md (production)

```markdown
# Changelog

<!-- heraut-release: main/7.4.1+159001 -->
## [7.4.1] - 2026-06-01

### 🚀 Features

- *(auth)* Add biometric login support - 6881760


### 🐛 Bug Fixes

- *(checkout)* Correct total when applying discount codes - 68d598e

<!-- heraut-release: main/7.4.0+155398 -->
## [7.4.0] - 2026-05-12

### 🚀 Features

- Add dark mode toggle - 05fd288
```

The two UAT builds of `7.4.1` added nothing to it: the `7.4.1` section spans
every commit since the previous production tag. In CI, each hash links to its
commit.

---

## Tag structure overview

```
Env     Version  Build     Tag
------  -------  --------  -----------------------
uat     7.4.0    154392    uat/7.4.0+154392
uat     7.4.0    154572    uat/7.4.0+154572   ← no CHANGELOG update
uat     7.4.0    155391    uat/7.4.0+155391   ← no CHANGELOG update
main    7.4.0    155398    main/7.4.0+155398  ← CHANGELOG: commits since main/7.3.x+*
uat     7.4.1    158404    uat/7.4.1+158404
uat     7.4.1    158565    uat/7.4.1+158565   ← no CHANGELOG update
main    7.4.1    159001    main/7.4.1+159001  ← CHANGELOG: commits since main/7.4.0+155398
```

---

## Version source

heraut does not derive the version from commits when `--set-build-id` is used —
`--set-version` is required. Common ways to provide it in a CI pipeline:

```bash
# From a VERSION file
APP_VERSION=$(cat VERSION)

# From pubspec.yaml (Flutter)
APP_VERSION=$(grep '^version:' pubspec.yaml | cut -d' ' -f2 | cut -d'+' -f1)

# From package.json
APP_VERSION=$(node -p "require('./package.json').version")

# From a pipeline variable set by the release manager
# (GitLab: Settings > CI/CD > Variables > APP_VERSION)
```

---

## Querying the current version after tagging

`heraut version current --env uat` prints the latest **raw tag**
(`uat/7.4.1+158404`). Add `--bare` to get just the semantic version — the env
prefix and build ID are stripped via the effective `tag_format`:

```bash
heraut version current --env uat           # uat/7.4.1+158404
heraut version current --env uat --bare    # 7.4.1

CURRENT=$(heraut version current --env uat --bare)
echo "Current UAT version: $CURRENT"
```
