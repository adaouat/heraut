# ADR-0002: Tool Name — Héraut / heraut

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

The CLI tool needed a name. Requirements were: memorable, meaningful relative to the
tool's purpose (announcing releases), and pronounceable.

## Decision

**Brand name**: Héraut  
**Technical name (binary, image, module path, config file)**: `heraut`

"Héraut" is the French word for *herald* — one who announces important news. In Marvel
lore, the Silver Surfer is the herald of Galactus, traveling ahead to announce his
arrival. The metaphor fits: `heraut` announces the arrival of a new version.

The split between accent (brand) and no-accent (technical) follows the precedent of
`communique` / communiqué, a tool already used in this stack.

### Usage examples

```bash
heraut release --env prod
heraut changelog
heraut check config
heraut version next --dry-run
```

### Artifact names

| Artifact                  | Name                                                |
|---------------------------|-----------------------------------------------------|
| Binary                    | `heraut`                                            |
| Go module                 | `github.com/adaouat/heraut`                         |
| Container image           | `ghcr.io/adaouat/heraut:<version>`                  |
| GitHub binary (per OS/arch) | `heraut_<version>_<os>_<arch>[.exe]`              |
| Config file               | `.heraut.yml` (or `.config/heraut.yml`)             |
| Schema URL                | `https://raw.githubusercontent.com/adaouat/heraut/main/schema.json` |

## Consequences

**Positive**
- Memorable and meaningful — the French word reinforces the project's identity
- ASCII-safe technical name avoids encoding issues in Docker image names, runner
  environments, and CI paths
- Consistent pattern with `communique` / communiqué, already familiar to French-speaking
  contributors
- The "héraut announces a release" mental model maps directly to what the tool does

**Negative / trade-offs**
- Non-French speakers may initially be unsure how to pronounce "Héraut" (ay-roh). Not a
  blocker for a CLI tool — they type `heraut` regardless.
