# Guide: Mobile / CI multi-build tagging

This guide covers projects that create multiple tagged builds per semantic
version — the typical mobile CI pattern:

```
main/7.4.0-155398    ← production release
uat/7.4.0-154392     ← first UAT build
uat/7.4.0-154572     ← second UAT build (same version, new build)
uat/7.4.0-155391
uat/7.4.1-158404
uat/7.4.1-158565
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
  tag_format: "{env}/{version}-{build}"

changelog:
  generator: git-cliff
  # Point to a custom cliff config scoped to production (see below).
  # UAT builds skip changelog entirely (disable_changelog: true).
  config: .config/cliff.prod.toml
  tag_pattern: "main/[0-9]+\\.[0-9]+\\.[0-9]+-[0-9]+"

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
  --version "$APP_VERSION" \
  --build "$CI_PIPELINE_ID"
```

`APP_VERSION` is the semantic version your team maintains (e.g. `7.4.1`).
It can come from a file (`VERSION`, `package.json`, `pubspec.yaml`), a
pipeline variable set by the release manager, or any other source. heraut
does not derive it from commits — you own the version, heraut owns the tag.

This produces tag `uat/7.4.1-158404` and pushes it. No CHANGELOG.md
update occurs.

### Production release

```bash
# Tag + CHANGELOG.md update for the promoted production build
heraut changelog \
  --tag \
  --env main \
  --version "$APP_VERSION" \
  --build "$CI_PIPELINE_ID"
```

This produces tag `main/7.4.1-158404`, updates CHANGELOG.md with all
commits since `main/7.4.0-155398` (the previous production tag), commits
and pushes the changelog, then pushes the tag.

---

## Custom git-cliff config for production

Without a `tag_pattern`, git-cliff won't recognise the `{env}/{version}-{build}`
format and will treat all commits as unreleased. The following config scopes
git-cliff to production tags only and displays `7.4.1` instead of `main/7.4.1-158404`
in the changelog headings.

```toml
# .config/cliff.prod.toml
[changelog]
header = "# Changelog\n\n"
body = """
{%- macro remote_url() -%}
  {{ get_env(name="CI_PROJECT_URL", default=get_env(name="GITHUB_SERVER_URL", default="") ~ "/" ~ get_env(name="GITHUB_REPOSITORY", default="")) }}
{%- endmacro -%}

{% if version %}\
    {% set display_version = version | split(pat="/") | last | split(pat="-") | first %}\
    {% if previous.version %}\
        ## [{{ display_version }}]\
          ({{ self::remote_url() }}/compare/{{ previous.version }}..{{ version }}) - {{ timestamp | date(format="%Y-%m-%d") }}
    {% else %}\
        ## [{{ display_version }}] - {{ timestamp | date(format="%Y-%m-%d") }}
    {% endif %}\
{% else %}\
    ## [unreleased]
{% endif %}\

{% for group, commits in commits | group_by(attribute="group") %}
    ### {{ group | striptags | trim | upper_first }}
    {% for commit in commits | sort(attribute="message") %}
        - {% if commit.scope %}*({{ commit.scope }})* {% endif %}\
            {% if commit.breaking %}[**breaking**] {% endif %}\
            {{ commit.message | upper_first }}
    {%- endfor %}
{% endfor -%}
"""
trim = true

[git]
conventional_commits = true
filter_unconventional = false
commit_parsers = [
  { message = "^feat", group = "<!-- 0 -->🚀 Features" },
  { message = "^fix", group = "<!-- 1 -->🐛 Bug Fixes" },
  { message = "^doc", group = "<!-- 3 -->📚 Documentation" },
  { message = "^perf", group = "<!-- 4 -->⚡ Performance" },
  { message = "^refactor", group = "<!-- 2 -->🚜 Refactor" },
  { message = "^chore\\(release\\):", skip = true },
  { message = "^chore|^ci|^build", skip = true },
  { message = "^test|^style", skip = true },
]
filter_commits = true
```

**Key differences from the default embedded config:**

| | Default (embedded) | Production cliff config |
|---|---|---|
| `tag_pattern` | not set (relies on `--tag-pattern` flag) | passed via `changelog.tag_pattern` |
| Version heading | `{{ version \| trim_start_matches(pat="v") }}` → `main/7.4.1-158404` | extracts `7.4.1` via split |
| Chore/CI commits | shown | skipped (`filter_commits = true`) |

---

## Resulting CHANGELOG.md (production)

```markdown
# Changelog

## [7.4.1] - 2024-06-01

### 🚀 Features
- *(auth)* Add biometric login support
- *(cart)* Persist cart across sessions

### 🐛 Bug Fixes
- Fix crash on empty product list
- *(checkout)* Correct total when applying discount codes

## [7.4.0] - 2024-05-12

### 🚀 Features
- Add dark mode toggle
...
```

---

## Tag structure overview

```
Env     Version  Build     Tag
------  -------  --------  -----------------------
uat     7.4.0    154392    uat/7.4.0-154392
uat     7.4.0    154572    uat/7.4.0-154572   ← no CHANGELOG update
uat     7.4.0    155391    uat/7.4.0-155391   ← no CHANGELOG update
main    7.4.0    155398    main/7.4.0-155398  ← CHANGELOG: commits since main/7.3.x-*
uat     7.4.1    158404    uat/7.4.1-158404
uat     7.4.1    158565    uat/7.4.1-158565   ← no CHANGELOG update
main    7.4.1    159001    main/7.4.1-159001  ← CHANGELOG: commits since main/7.4.0-155398
```

---

## Version source

heraut does not derive the version from commits when `--build` is used —
`--version` is required. Common ways to provide it in a CI pipeline:

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

## Using `heraut version current` after tagging

`heraut version current --env uat` reads the latest tag matching
`uat/*-*` and strips the build suffix, returning `7.4.1`. This lets
downstream jobs query the current semantic version without re-parsing
the tag manually:

```bash
CURRENT=$(heraut version current --env uat)
echo "Current UAT version: $CURRENT"   # → 7.4.1
```
