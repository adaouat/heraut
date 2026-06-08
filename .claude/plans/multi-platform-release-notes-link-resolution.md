# Multi-Platform Release Notes: Link Resolution — Design Spike

## Status

Draft for review. Not a roadmap commitment — captures the problem, the options discussed,
and a recommended direction with a draft task breakdown to refine once agreed.

## Problem statement

heraut can publish a single release to multiple platforms (`release.platforms`: GitHub +
GitLab) from one pipeline run. Release notes are generated **once**:

```go
// internal/pipeline/release.go — Step 6
notes, genErr = p.cfg.Notes.Generate(result.Tag)
```

…and the resulting string is reused **verbatim** for every platform's `CreateRelease` in
the loop that follows (Step 7). But the generators resolve commit/PR/MR links from
*ambient CI environment variables* — see the `remote_url()` / `pr_link()` Tera macros in
`internal/generators/gitcliff/cliff.release-notes.toml`:

```
{{ get_env(name="CI_PROJECT_URL", default=get_env(name="GITHUB_SERVER_URL", default="") ~ "/" ~ get_env(name="GITHUB_REPOSITORY", default="")) }}
```

Whichever CI the pipeline happens to run in "wins" the link flavor (host **and** path
shape — `/pulls/N` vs `/-/merge_requests/N`, `/issues/N` vs `/-/issues/N`, `/commit/<sha>`
host). Every other configured platform then gets a release whose notes point to the wrong
host with the wrong link paths.

This is **distinct** from the tag-sync race discussed earlier (mirrored repo, secondary
platform may not have the tag yet when `gh release create` runs — a timing problem). This
spike is scoped to the *link-flavor* problem, which exists even with perfect sync, e.g.
running `heraut release` from GitLab CI while also publishing to GitHub: GitLab-flavored
links land in the GitHub release notes.

## Constraints that shape the solution space

- **The committed `CHANGELOG.md` must stay singular** — one canonical generation, one
  link flavor. You cannot commit two flavors of the same file. Whatever we do for
  *release notes* (ephemeral, per-platform-release-body) must not be confused with the
  *changelog* (committed, singular, generated once in Step 2).
- **Layer rules**: `internal/generators/*` only import `internal/{port,config}`. Any new
  per-platform context has to flow through `port.Generator`'s surface (an interface
  change — must update all three implementations in one commit) or through config the
  generator already owns.
- **Three generators, not one**: git-cliff (embedded TOML, heraut-controlled), cocogitto
  (embedded `cog.toml` + Tera, heraut-controlled), communique (fully external — see
  below). A fix that only touches git-cliff leaves the other two with the same bug.
- **communique is opaque to heraut.** `communique.Generate` (`internal/generators/
  communique/generator.go`) just runs `communique generate --config <user-file> <tag>`
  and returns stdout. heraut has **no** template surface here — link resolution is
  entirely owned by the user's communique config. This means "inject per-platform
  context" is *not achievable* for communique users without a communique-side feature.
  Any heraut-side fix has to either (a) accept that communique users are out of scope,
  or (b) find some indirect lever (env vars passed to the subprocess — communique would
  need to read them, which is outside heraut's control).
- **ADR-0010** treats embedded TOML/Tera as user-facing — any byte change to the
  defaults changes the effective config for everyone relying on them. Template changes
  need care and probably their own ADR note.

## Options considered (responding to the four raised)

### A. Designate a "main platform"; treat others as synced repos

Doesn't close the gap — it just narrows *which* platform is affected. The non-canonical
platform(s) still get wrong-flavored notes. Where this idea **does** land correctly:
deciding which platform's link flavor appears in the *committed changelog* (which must be
singular anyway) — that's "main platform" thinking applied to the one artifact that
actually requires a single answer. See Recommendation, point 4.

### B. Ensure synchronization before publishing to secondary platforms

This is the tag-existence race, not the link-flavor problem — orthogonal. Worth its own
design note if pursued (heraut would need to poll a secondary platform's API for the tag,
a new category of side effect for a tool that's deliberately thin orchestration over
CLIs). Not further explored here.

### C. Explicit per-platform base URL (self-hosted instances)

**Necessary regardless of direction** — heraut cannot correctly resolve a self-hosted
GitLab/GitHub Enterprise host by sniffing ambient CI env vars (those describe *where CI
is running*, not *where each configured target platform lives*). A `base_url` field
alongside the existing `repository` / `project` fields in `config.Platform` is the
natural extension, following the field-change checklist in `coding.md` (struct + 
`schema.json` + `docs/heraut.sample.yml`, plus an ADR since it's a new wire-compatible
field).

### D. Smarter templates fed by a heraut-supplied value

The piece that actually closes the loop — but **only** in combination with regenerating
notes once per platform (see below). Smarter templates alone are inert if the same
rendered string keeps getting reused for every platform.

## Recommended direction: C + D, applied per platform, scoped to git-cliff/cocogitto

1. **Add `base_url` to `config.Platform`** (optional; default `https://github.com` /
   `https://gitlab.com`). Closes C. Needs an ADR (new wire-compatible field) plus the
   usual schema/sample sync.

2. **Move release-notes generation inside the per-platform loop** in `pipeline.Run()`.
   Regenerate once per platform — not once globally — passing each platform's
   link-resolution context (derived from *its* `base_url` + `repository`/`project`,
   never from ambient CI vars). This is a real behavioral/architectural shift (notes stop
   being a single global artifact) — warrants its own ADR.

3. **Update the embedded git-cliff and cocogitto templates** so `remote_url()` /
   `pr_link()` prefer heraut-injected values, falling back to the existing
   `CI_PROJECT_URL` / `GITHUB_SERVER_URL` detection when heraut hasn't supplied anything
   — preserving today's behavior for single-platform setups and anyone who copied the
   defaults into an override. Communique stays out of scope per the constraint above;
   document that explicitly (e.g. in the spec) so communique users aren't surprised.

4. **The committed `CHANGELOG.md` keeps a single canonical generation** (Step 2,
   unchanged) — tied to wherever `origin` is. This is "main platform" thinking (option A)
   correctly scoped to the one artifact that genuinely needs a single answer.

### Open sub-decision: how does heraut inject per-platform context?

**RESOLVED (T68) — see [ADR-0021](../../docs/adr/0021-per-platform-release-notes.md)
"Context-injection shape".** PoC against `cog 7.0.0` / `git-cliff 2.13.1` showed the two
generators resolve links through different surfaces, so neither shape below applies
uniformly: chose **option (b), heraut-owned context** via a generator-agnostic
`LinkContext` at the `port.Generator` boundary, each adapter translating to its native
mechanism (git-cliff = heraut-owned env vars via `get_env` with the CI-var chain as
fallback; cocogitto = `--remote/--owner/--repository`; communique = ignored). The two
shapes originally considered:

Two shapes, each with trade-offs — this needs its own mini-spike once the direction above
is agreed, because it's the crux of "three generators must stay consistent":

- **Reuse env vars** (`CI_PROJECT_URL`, `GITHUB_SERVER_URL` + `GITHUB_REPOSITORY`) —
  smallest template diff, reuses macros that already exist. But it conflates "the CI
  context heraut is actually running in" with "the target platform heraut wants notes
  for" — e.g. a GitHub Actions run publishing to a GitLab platform would have *both* sets
  of vars present, with the wrong one taking precedence by the macro's `default()` chain.
- **New heraut-owned template variables** — cleaner separation (e.g. `HERAUT_PLATFORM_*`
  env vars, or git-cliff's own `--config`/context-injection surface, or a postprocessor).
  Real template-surface change across two generators; needs to confirm what cocogitto's
  Tera context can actually accept before committing to a shape.

## Related (but distinct) gap surfaced during this spike: multiple platforms of the same type

While discussing per-platform `base_url`, the question came up: *can heraut already publish
to two different GitLab targets (e.g. two instances, or two projects on the same instance)?*
Tracing the code: the config technically allows it —

```yaml
release:
  platforms:
    - { platform: gitlab, project: "group-a/project" }
    - { platform: gitlab, project: "group-b/project" }
```

`internal/config/validator.go` only checks `plat.Type` is `github`/`gitlab` (no uniqueness
constraint), and `app.BuildPipeline` loops over every entry and constructs a `port.Platform`
for each. But several places silently assume **at most one platform per type**, so this
would half-work at best:

1. **`heraut check` only validates the first match.** `app/check.go:200-207` —
   `findPlatformCfg` linearly scans and returns on the *first* config whose
   `Type == "gitlab"`. A second GitLab entry is never checked: no `glab`/token/project
   validation runs for it at all. Same for `configuredPlatforms`, which collapses both
   entries into a single `map[string]bool{"gitlab": true}`.

2. **The release URL is hardcoded to gitlab.com.** `internal/platforms/gitlab/platform.go:17`:
   ```go
   const gitlabBaseURL = "https://gitlab.com"
   ```
   `ReleaseURL` always builds against this constant, and heraut never tells `glab` which
   host to talk to (only `-R <project>`, never a host). Any self-hosted target — whether
   it's the only GitLab platform or one of several — gets a wrong URL in the reporter
   summary, and `glab` itself has no host info to act on.

3. **The CI-auth probe assumes one ambient GitLab project.** `checkAPIAuth`
   (`gitlab/platform.go:72-93`) branches on `GITLAB_CI`/`CI_PROJECT_ID` — env vars that
   describe *the CI runner's own project*, not "whichever of the N configured platforms
   is being checked right now." A second platform would get probed against the wrong
   project, or skipped.

4. **Reporter output is ambiguous.** `Name()` returns the static string `"gitlab"`
   (`platform.go:33`), so the per-platform pipeline step would render as two
   indistinguishable `Publish to gitlab` lines (`internal/pipeline/release.go:166`).

**Why this belongs in this note rather than as a footnote:** it independently confirms
that `base_url` (option C / Recommendation point 1, above) isn't merely a self-hosted
nicety — it's a load-bearing prerequisite for *any* multi-instance scenario, because
without a per-platform host, `ReleaseURL`, the auth probe, and the actual `glab -R
<host>/<project>` invocation can't be made instance-aware.

**What closing this gap would actually take** (beyond `base_url` itself):
- Fix the type-keyed lookups in `app/check.go` (`findPlatformCfg`, `configuredPlatforms`)
  to iterate *all* matching entries, not the first.
- Derive `ReleaseURL` from the per-platform host instead of the `gitlabBaseURL` constant
  (and the GitHub equivalent, which currently hardcodes `github.com` in
  `github/platform.go:35` too — same class of bug, just not yet visible because GitHub
  Enterprise base-URL support hasn't come up).
- Disambiguate reporter labels — probably derive them from `project`/`repository` (or
  host) rather than the bare platform type, once more than one instance of a type exists.
- Rework `checkAPIAuth`'s CI-context assumptions so the ambient `CI_PROJECT_ID` /
  `GITHUB_REPOSITORY` checks don't get applied to the *wrong* configured platform when
  there's more than one of the same type.

**Scoping note:** this is a **third, distinct thread** — separate from both the
link-flavor problem (this note's main subject) and the tag-sync/target-pinning sketch
discussed earlier. It should **not** be folded into the T65-T73 breakdown below; it would
need its own task numbering and likely its own ADR (multi-instance support is itself an
architectural capability, not a bugfix). Flagging it here so it isn't lost — and because
landing `base_url` (T65/T66) is now doing double duty: it's required for both the
link-flavor fix *and* this multi-instance gap, which may argue for sequencing it earlier
/ scoping it slightly more generously than "just enough for link resolution."

## Impact / sizing

Cross-cutting — touches:

- `config.Platform` + `schema.json` + `docs/heraut.sample.yml` + new ADR (per-platform
  `base_url`)
- New ADR (release notes regenerated per-platform — architectural shift)
- `port.Generator` — likely an interface change (`Generate(tag string, ctx ...)`),
  meaning all three implementations + every test double move together in one commit
- `internal/pipeline/release.go` — restructure: notes generation moves into the platform
  loop; interacts with the reporter step semantics from ADR-0017 (step count / naming)
- Embedded git-cliff (×2 TOML) and cocogitto (Tera) templates
- `docs/specs/05-generators-and-platforms.md` — document the communique exclusion
- New contract + integration coverage: "N platforms configured → N distinctly-flavored
  notes, each pointing at its own host/path shape"

Rough sizing: at least 6-7 roadmap tasks plus two ADRs. Bigger than a single-session
task — needs the breakdown-and-propose treatment per `claude.md`'s task-discipline rule.

## Draft task breakdown (numbering illustrative — next free is T65)

| # | Task | Notes |
|---|------|-------|
| T65 | ADR — per-platform `base_url` for self-hosted instances | Decision record before the field lands |
| T66 | `base_url` config field (config + schema + sample) | Depends on T65 |
| T67 | ADR — release notes regenerated per platform (architectural shift) | Decision record before the pipeline restructure |
| T68 | Resolve the context-injection shape (env vars vs new template vars) | Mini-spike; unblocks T69-T71 |
| T69 | `port.Generator` interface change + update all 3 implementations | Mechanical once T68 decided |
| T70 | Restructure `pipeline.Run()` — notes generation moves into the per-platform loop | Touches ADR-0017 step semantics |
| T71 | Update embedded git-cliff + cocogitto templates to prefer heraut-injected context | Two generators; communique explicitly excluded |
| T72 | Integration test — multi-platform release, distinctly-flavored notes per platform | Closes the loop |
| T73 | Spec update — document communique's link-resolution exclusion | `docs/specs/05-generators-and-platforms.md` |

## Open questions to resolve before turning this into roadmap entries

1. ~~Env-var reuse vs new heraut-owned template variables (T68)~~ — **RESOLVED (T68):**
   heraut-owned `LinkContext`, per-generator native translation. See
   [ADR-0021](../../docs/adr/0021-per-platform-release-notes.md).
2. ~~Should `base_url` support per-environment overrides?~~ — **RESOLVED (T65/ADR-0020):**
   no special merge; `release.platforms` is replaced wholesale per env (ADR-0019), so a
   per-env platform block already carries its own `base_url`. Platform-level granularity is
   enough.
3. Is the communique exclusion acceptable as a documented limitation, or does it push
   some users toward git-cliff/cocogitto for multi-platform setups specifically? Worth
   surfacing in the spec either way so it's a documented trade-off, not a surprise.
