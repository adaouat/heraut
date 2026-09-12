# Guide: Release pipeline and hook positions

[Spec 02 § `hooks`](../specs/02-configuration.md#hooks) documents each of the six hook
points in isolation — when it fires, what template variables it sees, how failures are
handled. This guide instead shows the two pipelines end to end, so it's visible at a
glance where each hook point actually sits relative to every other step. It does not
replace Spec 02 or [ADR-0053](../adr/0053-release-lifecycle-hooks.md) as the source of
truth for hook behavior — treat this page as a map, those as the reference.

Hook nodes are drawn as hexagons in both diagrams below.

## `heraut release`

```mermaid
flowchart TD
    A["Preflight<br/>config validate + branch guard + runtime check"] --> B["Resolve version"]
    B --> H1{{"hook: post_bump"}}
    H1 --> C{"--dry-run?"}
    C -- yes --> DR["Render remaining steps as a plan<br/>(nothing executes)"]
    C -- no --> D{"changelog configured and<br/>not disabled for this env?"}
    D -- yes --> H2{{"hook: pre_changelog"}}
    H2 --> E["Generate changelog"]
    E --> F["Commit changelog + push<br/>(skipped if byte-identical to last commit)"]
    F --> H3{{"hook: pre_tag"}}
    D -- no --> H3
    H3 --> G["Create tag"]
    G --> I["Push tag"]
    I --> H4{{"hook: post_tag"}}
    H4 --> J["Generate release notes<br/>(single-target runs only, upfront)"]
    J --> T
    subgraph T["For each release.targets entry, in declared order"]
        direction TB
        H5{{"hook: pre_release<br/>failure skips this target only"}} --> K["Regenerate notes<br/>(multi-target runs only)"]
        K --> L["Create release via gh / glab"]
        L --> M["Upload assets"]
        M --> H6{{"hook: post_release<br/>failure warns — target already published"}}
    end
    T --> N["Print summary"]
```

Things the diagram compresses that are worth stating explicitly:

- **`post_bump` fires unconditionally**, before the `--dry-run` check — it runs (or, under
  `--dry-run`, renders) even on a run that will otherwise change nothing, e.g.
  `disable_changelog: true` with no other flags.
- **`pre_tag` fires whichever branch the changelog decision took.** A skipped or disabled
  changelog step doesn't skip `pre_tag` — it's a separate gate, not chained to changelog
  generation.
- **`pre_release`/`post_release` are the one exception to "a failing hook aborts the run."**
  They're scoped per `release.targets` entry: a failing hook skips (`pre_release`) or warns
  (`post_release`) for that target only, and the loop still attempts the rest. A real
  publish failure (not a hook) still aborts the whole loop as every other step does. See
  [Spec 02 § Failure semantics](../specs/02-configuration.md#failure-semantics) for the
  full rationale.
- **`--dry-run` never executes a hook.** Every hook node above still "fires" during a
  dry run, but only to render the substituted command as a plan line — the shell it would
  otherwise run through ([ADR-0054](../adr/0054-windows-hook-execution.md)) never starts.

## `heraut changelog`

```mermaid
flowchart TD
    A["Resolve version"] --> H1{{"hook: post_bump<br/>always fires, even in the disabled-and-no-tag case below"}}
    H1 --> B{"disable_changelog<br/>for this env?"}
    B -- yes --> W["Warn: changelog disabled"]
    W --> C2{"--tag?"}
    C2 -- no --> END1["Exit"]
    C2 -- yes --> R
    B -- no --> R{"--dry-run?"}
    R -- yes --> DR["Render remaining steps as a plan<br/>(nothing executes)"]
    R -- no --> S{"changelog configured and<br/>not disabled?"}
    S -- yes --> H2{{"hook: pre_changelog"}}
    H2 --> Gen["Generate changelog"]
    Gen --> Cd{"--commit or --tag?"}
    Cd -- yes --> Com["Commit changelog<br/>+ push unless --no-push"]
    Cd -- no --> Tg
    Com --> Tg{"--tag?"}
    S -- no --> Tg
    Tg -- no --> SUM["Print summary"]
    Tg -- yes --> H3{{"hook: pre_tag"}}
    H3 --> Tag["Create tag"]
    Tag --> Np{"--no-push?"}
    Np -- no --> Push["Push tag"]
    Np -- yes --> H4
    Push --> H4{{"hook: post_tag<br/>fires regardless of --no-push"}}
    H4 --> SUM
```

`pre_release`/`post_release` never appear here — `heraut changelog` never publishes to a
platform, so those two points simply don't exist in this pipeline (no flag needed to turn
them off). Note also that `--tag` with `disable_changelog: true` skips straight from the
disabled-changelog warning to the tag section: `pre_changelog` and `post_bump`'s sibling
changelog-generation step never run, but `pre_tag`/`post_tag` still do — see [Spec 03 §
Tag-only workflow](../specs/03-commands.md#tag-only-workflow-no-release-block-required).

## Hook point quick reference

| Point            | Fires (both pipelines, unless noted)                          |
|-------------------|----------------------------------------------------------------|
| `post_bump`       | Immediately after version resolution — unconditionally.        |
| `pre_changelog`   | Before changelog generation — only when a changelog actually generates this run. |
| `pre_tag`         | Before the local git tag is created — only when a tag is actually created this run. |
| `post_tag`        | After the tag push attempt — fires even with `--no-push`.       |
| `pre_release`     | `heraut release` only, once per `release.targets` entry, before publishing. |
| `post_release`    | `heraut release` only, once per `release.targets` entry, after publishing (and any asset upload). |

Full detail — command list semantics, template variables (`{{ .Version }}`, `{{ .Tag }}`,
`{{ .PreviousTag }}`, `{{ .Platform }}`), failure semantics, and `--no-hooks` — lives in
[Spec 02 § `hooks`](../specs/02-configuration.md#hooks).

## See also

- [ADR-0053: Release lifecycle hooks](../adr/0053-release-lifecycle-hooks.md) — why hooks
  are shaped this way, including the `pre_release`/`post_release` isolation decision.
- [Spec 02 § `hooks`](../specs/02-configuration.md#hooks) — configuration reference.
- [Spec 03 § `heraut release`](../specs/03-commands.md#heraut-release) and
  [§ `heraut changelog`](../specs/03-commands.md#heraut-changelog) — full command/flag
  reference, including the non-hook action sequence this guide diagrams.
