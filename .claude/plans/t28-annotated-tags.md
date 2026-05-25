# T28 — Configurable Git Tag Type (Annotated vs Lightweight)

## Context

Both pipelines create lightweight tags (`git tag <tag>`). Annotated tags carry a tagger
identity, timestamp, and message — what `git describe`, release dashboards, and most
tooling expect. The default switches to annotated for v1.0, but users can opt back to
lightweight via a new `versioning.tag_type` config field.

Annotation message = resolved `commitMessage(result.Version)` (defaults to
`"chore(release): 1.2.3"`) — reuses the existing `commit_message` template with no new
config field for the message itself.

## New config field

```yaml
versioning:
  tag_type: annotated   # annotated (default) | lightweight
```

Lives in `Versioning` struct alongside `tag_format` (same concern: how tags are shaped/created).
Default behaviour when unset: annotated.

## Changes (TDD order)

### 1. `internal/config/config.go`

Add to `Versioning` struct:
```go
TagType string `yaml:"tag_type"`
```

### 2. `internal/config/validator.go`

Add enum validation for `tag_type`: accepts `""`, `"annotated"`, `"lightweight"`.
Error path: `"versioning.tag_type"`, message `"must be one of: annotated, lightweight"`.

### 3. `internal/config/validator_test.go`

Add test row for invalid `tag_type` (e.g. `"signed"`) → error.

### 4. `testdata/config/invalid/invalid_tag_type.yml`

Fixture with `tag_type: signed` for the schema + validator tests.

### 5. `schema.json`

Add to versioning object:
```json
"tag_type": { "type": "string", "enum": ["annotated", "lightweight"] }
```

### 6. `internal/pipeline/config.go`

Add to both `Config` and `ChangelogConfig`:
```go
AnnotatedTags bool
```

### 7. `internal/pipeline/release.go` and `changelog.go`

Replace the bare `p.run("git", "tag", result.Tag)` with a private helper in each pipeline:

```go
func (p *Pipeline) gitTag(tag, version string) error {
    if p.cfg.AnnotatedTags {
        return p.run("git", "tag", "-a", tag, "-m", p.commitMessage(version))
    }
    return p.run("git", "tag", tag)
}
```

Call it as `p.gitTag(result.Tag, result.Version)`.
Same pattern for `ChangelogPipeline`.

### 8. `internal/app/pipeline.go`

In `buildReleasePipelineConfig` and `buildChangelogPipelineConfig`, set:
```go
AnnotatedTags: cfg.Versioning.TagType != "lightweight",
```
(empty or `"annotated"` → true; only `"lightweight"` → false)

### 9. `internal/pipeline/release_test.go`

- Update existing `["tag", "v1.2.3"]` assertions (lines 87 and 130) to
  `["tag", "-a", "v1.2.3", "-m", "chore(release): 1.2.3"]` (default annotated, using
  `AnnotatedTags: true` in the test cfg).
- Add `TestRun_LightweightTag`: `AnnotatedTags: false` → args stay `["tag", "v1.2.3"]`.
- Add assertion to `TestRun_CustomCommitMessage` that the tag annotation (Calls[3])
  also carries the custom message `"release: v1.2.3"`.

### 10. `internal/pipeline/changelog_test.go`

- Update existing `["tag", "v1.2.3"]` assertions (lines 84 and 102) → annotated form.
- Add `TestChangelogRun_LightweightTag`: `AnnotatedTags: false` → `["tag", "v1.2.3"]`.
- Add `TestChangelogRun_TagAnnotationUsesCustomMessage`.

### 11. `docs/specs/02-configuration.md`

Add `tag_type` row to the `versioning:` field table.

### 12. `docs/specs/03-commands.md`

Update the two "Create git tag (lightweight)" steps:
- `heraut release` step 5 → "Create git tag (annotated by default; `tag_type: lightweight` to override)"
- `heraut changelog --tag` step 4 → same note

### 13. `docs/tasks/roadmap.md`

Flip T28 `[ ]` → `[x]`, add implementation note.

## Scope note

Grows from S to S-M due to the config/schema/validator surface. No new ADR needed —
the decision is: annotated default, user-overridable. Not architectural enough to warrant
a full ADR.

## Verification

```bash
go test ./internal/config/... -v -run TestValidate
go test ./internal/pipeline/... -v
go test ./...  # full suite must stay green
```
