# ADR-0001: Implementation Language — Go

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

`heraut` is a CLI tool that orchestrates external binaries (`glab`, `gh`, `git-cliff`,
`cog`, `communique`) and implements custom version resolution logic (SemVer, CalVer,
SemVer-per-env with prefixed tags, CalVer-per-env). It is distributed as raw cross-platform
binaries via GitHub Releases and as a container image at `ghcr.io/adaouat/heraut`
(see [ADR-0013](0013-raw-binary-goreleaser-format.md)).

The following constraints shape the language choice:

- **Single binary required**: no runtime (interpreter, VM) — users download one file and
  run it.
- **Cross-compilation required**: builds must target Linux / macOS / Windows on both amd64
  and arm64 from any developer machine and any CI runner.
- **Startup time matters**: `heraut` is invoked in every release pipeline job; 200+ ms
  cold-start is a regression.
- **Core work is orchestration**: the tool shells out, parses YAML, validates a JSON
  Schema, and resolves version numbers — no performance-critical computation.
- **JSON Schema validation is a hard requirement**: `heraut check config` validates
  `.heraut.yml` against a published schema.
- **TDD is required**: every layer must be unit-testable; external CLI interactions must
  be contract-testable without invoking real binaries.

Five languages were evaluated: Go, Rust, PHP + Symfony, Node.js / TypeScript, and Dart.

## Evaluation

| | Go | Rust | PHP + Symfony | Node.js / TS | Dart |
|---|:---:|:---:|:---:|:---:|:---:|
| Single binary | ✅ | ✅ | ❌ (.phar needs PHP runtime) | ⚠️ (needs pkg/Bun/Deno) | ✅ (`dart compile exe`) |
| Cross-compilation | ✅ (`GOOS`/`GOARCH`) | ⚠️ (`cross`/cargo targets) | ❌ | ⚠️ (third-party bundlers) | ✅ |
| Binary size impact | ~10–15 MB | ~5–10 MB | +100 MB (PHP runtime) | +50–80 MB | ~10–20 MB |
| Cold-start | <10 ms | <5 ms | 200–300 ms | 100–400 ms | 20–50 ms |
| CLI framework | ✅ cobra + fang | ✅ clap | ✅ Symfony Console | ✅ oclif / commander | ⚠️ args (thin) |
| Subprocess DX | ✅ | ✅ | ✅ symfony/process | ✅ execa | ✅ dart:io |
| YAML + Schema | ✅ mature | ✅ serde_yaml | ✅ symfony/yaml | ✅ ajv | ⚠️ lightly maintained |
| Test ecosystem | ✅ | ✅ | ✅ PHPUnit | ✅ vitest / jest | ✅ package:test |

### Per-language notes

**Go** — `glab` and `gh` (the primary tools heraut wraps) are both written in Go. cobra
+ fang is the proven choice for tools of this shape. Static binary, sub-10 ms startup,
trivial cross-compilation. Verbose error handling is the main friction point but is
manageable for a tool of this scope.

**Rust** — Technically superior on binary size and startup. `clap` is excellent. The cost
is a steeper learning curve, slow clean builds (~30–60 s), and lifetime complexity for
string-heavy work (version parsing, tag manipulation) with no payoff — heraut does no
performance-critical computation.

**PHP + Symfony Console** — Disqualified by the single-binary requirement: a PHP runtime
adds ~100 MB and 200+ ms startup.

**Node.js / TypeScript** — Best subprocess ergonomics, best JSON Schema implementation
(`ajv`). Native binary output requires a third-party bundler, adding build complexity.
Cold-start without compilation is a CI regression.

**Dart** — `dart compile exe` produces an acceptable AOT binary. Disqualifying factor:
ecosystem alignment. JSON Schema and CLI framework libraries are either lightly
maintained or absent.

## Decision

**Go.**

Primary drivers:

1. **Ecosystem alignment**: `glab` and `gh` are Go programs; integration patterns are
   well-documented for Go consumers.
2. **Binary story is the simplest**: static binary, `FROM scratch`-compatible Docker
   layer, no runtime decision.
3. **cobra + fang** ([ADR-0003](0003-cli-framework-cobra-fang.md)) handles subcommands,
   global flags, styled help/errors with minimal boilerplate.
4. **Startup and image size** meet the CI constraints without trade-offs.
5. **TDD support**: table-driven tests in the stdlib plus `testify`; fake binaries in
   `PATH` for contract tests.

Rust was the only real alternative and was rejected on maintenance grounds: complexity
not justified by the performance requirements of an orchestration tool.

## Consequences

**Positive**
- Single static binary; container image stays lean
- cobra + fang handle all CLI scaffolding
- Consistent ecosystem with the tools being wrapped (`glab`, `gh`)
- Cross-compilation via `GOOS`/`GOARCH` and GoReleaser (see
  [ADR-0013](0013-raw-binary-goreleaser-format.md))
- Strong concurrency primitives available if parallel platform / generator execution is
  added later

**Negative / trade-offs**
- Go's error handling is verbose; discipline is needed to keep error messages actionable
  (see `.claude/rules/coding.md`)
- Team members unfamiliar with Go will need onboarding; the language is not complex but
  the idioms (error wrapping, interface composition, table tests) take time to
  internalize
