# Proposal: `{build}` token + `--build` flag for mobile/CI tagging flows

## Motivation

Mobile CI pipelines create tags with the pattern `<env>/<version>-<build_id>`:

```
main/7.4.0-155398
uat/7.4.0-154392
uat/7.4.0-154572   ← same version, different build
uat/7.4.1-158404
```

The build ID comes from the CI system (e.g. `$CI_PIPELINE_ID`) and is
not derived from commits. Multiple builds can carry the same semantic
version. The goal is to generate a changelog and create the tag via
`heraut changelog --tag` — without triggering a full platform release.

---

## What changes

### 1. New `{build}` token in `versioning.tag_format`

`tagfmt` already has `{version}` and `{env}`. Adding `{build}`:

| Token       | Render behaviour                       | ParseVersion behaviour       | GlobPattern  |
|-------------|----------------------------------------|------------------------------|--------------|
| `{version}` | substituted with the resolved version  | named capture group `version`| `*`          |
| `{env}`     | substituted with the active env        | non-capturing wildcard       | env literal  |
| `{build}`   | substituted with the `--build` value   | non-capturing wildcard       | `*`          |

**`tagfmt` changes** (minimal):

- `Render(template, env, version, build string)` — replaces `{build}`;
  returns error if `{build}` is in template but `build` is empty.
- `ParseVersion` — replaces `{build}` with `[^/]+` (non-capturing), so
  `uat/7.4.0-154392` parsed against `{env}/{version}-{build}` yields
  `7.4.0`. This is the key fix: existing tags are correctly versioned
  even though they carry a build suffix.
- `GlobPattern` — replaces `{build}` with `*`, so `uat/*` glob still
  matches all builds.

### 2. `--build <id>` flag on `heraut changelog`

```
heraut changelog --tag --env uat --version 7.4.1 --build $CI_PIPELINE_ID
```

**Constraint: `--build` requires `--version`.**

When a build ID is provided, version resolution from commits is skipped
entirely. The caller owns the version (they know `7.4.1` is being
built); heraut owns the tag creation and changelog generation.

Attempting `heraut changelog --build 12345` without `--version` → error
with a clear message.

### 3. Config

No new config fields required. The `{build}` token is opt-in via
`versioning.tag_format`. A project that does not use `{build}` is
completely unaffected.

Example `.heraut.yml` for the mobile use case:

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"

changelog:
  generator: git-cliff
```

`heraut check config` validates: if `{build}` is present in
`tag_format`, warn (not error) when `--build` is absent at runtime —
the check command cannot know the build ID, so it cannot fail hard.

---

## Changelog range

The "from" tag is the previous tag in the active env (existing
per-env behaviour). The build ID is transparent: `ParseVersion` strips
it, so `uat/7.4.0-154392` and `uat/7.4.0-154572` both yield version
`7.4.0`, and the "current version in UAT" lookup works as before.

Two natural scenarios fall out:

| Situation                                  | Range                                  | What changelog shows        |
|--------------------------------------------|----------------------------------------|-----------------------------|
| Same version, new build (`7.4.0-154572`)   | since `uat/7.4.0-154392`              | build diff (incremental)    |
| New version, first build (`7.4.1-158404`)  | since `uat/7.4.0-155391` (last 7.4.0) | version changelog           |

Both are handled automatically by the existing tag-range logic once
`ParseVersion` correctly ignores the build suffix.

---

## Impact on other commands

| Command                | `--build` support | Notes                                                    |
|------------------------|-------------------|----------------------------------------------------------|
| `heraut changelog`     | ✅ add now         | Core use case: tag + changelog, no platform publish      |
| `heraut release`       | ❌ defer           | A release per CI build creates too many GitHub releases  |
| `heraut version next`  | ❌ not applicable  | Build ID is not part of version resolution               |
| `heraut version current` | no change       | Returns the version extracted from the latest tag (build stripped) |
| `heraut check runtime` | no change         | No build-ID-specific runtime deps                        |

---

## Implementation scope

Touches only:

- `internal/versioning/tagfmt/` — add `{build}` token (~20 lines)
- `internal/versioning/perenv/` — pass build through to tagfmt render
- `internal/pipeline/changelog.go` + `internal/pipeline/config.go` — carry `Build string` field
- `internal/app/` — propagate from opts to pipeline config
- `internal/cmd/changelog.go` — add `--build` flag, validate `--version` required when set
- `schema.json` — no change (tag_format already accepts arbitrary strings)

---

## Decisions

1. **`{build}` without per-env** — allowed. `{version}-{build}` works for
   single-env projects. No restriction to per-env strategies.

2. **Free pattern** — `{build}` can appear anywhere in `tag_format`, exactly
   like `{version}` and `{env}` today.

3. **`heraut release --build`** — deferred. Focus is `heraut changelog --tag`
   first. Release addition is a follow-up task once changelog is validated
   in production.

4. **Build ID validation** — non-empty, no `/`, no whitespace (git tag
   constraints). `ParseVersion` uses `[^/]+` for `{build}` — same as `{env}`.
   Note: using `-` as the separator between version and build ID while also
   having `-` inside the build ID would create parsing ambiguity; document
   this constraint.
