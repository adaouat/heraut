# ADR-0029: Publish a Pkl Builtin for `heraut commit verify`

- **Status**: Accepted
- **Date**: 2026-06-23
- **Deciders**: bchatard

---

## Context

[ADR-0027](0027-builtin-conventional-commit-checker.md) (T116) removed heraut's own
dev-tooling dependency on `cog verify` by adding `heraut commit verify` and switching
heraut's own `.config/hk/config.pkl` `commit-msg` hook to call it directly:

```pkl
["heraut-commit-lint"] {
  check = "go run ./cmd/heraut commit verify --file {{ commit_msg_file }}"
}
```

Three sibling repositories in the same GitHub organization — `bifrost`, `forge`, and
`hermes` — have the identical pre-T116 pattern in their own `.config/hk/config.pkl`:

```pkl
["cocogitto"] {
  check = "cog --config .config/cocogitto/config.toml verify --file {{ commit_msg_file }}"
}
```

All three already depend on heraut for their own release management (`.config/heraut.yml`
present in each, CI calling `heraut check`/`heraut release` directly) and already install
`heraut` itself via the org's Homebrew tap (`adaouat/homebrew-tap`) for that purpose. heraut
is already the org's standard release tool; this ADR makes it the org's standard
commit-lint tool too, by publishing the hook definition itself as a Pkl package those repos
can import — the same way every one of these repos already imports hk's own builtins:

```pkl
import "package://github.com/jdx/hk/releases/download/v1.46.0/hk@1.46.0#/Builtins.pkl"
```

hk publishes `Builtins.pkl` as a versioned Pkl package attached to each of its own GitHub
releases (`PklProject` package metadata + `pkl project package` + a release asset at a
`packageZipUrl`). Mirroring that exact mechanism — rather than a lighter-weight raw-URL
`import "https://raw.githubusercontent.com/.../Builtins.pkl"` — keeps heraut consistent with
the one concrete precedent already in every consuming repo's config, and gets Pkl's own
package-resolution tooling (`pkl project resolve`, `PklProject.deps.json`, IDE support) for
free instead of a bespoke shortcut.

## Decision

### `pkl/` — new top-level directory, published as a Pkl package

```
pkl/
  PklProject        package metadata: name "heraut", version (rewritten at release time),
                     packageZipUrl pointing at this release's GitHub asset
  Builtins.pkl       module heraut.Builtins — the published Step(s)
```

A dedicated directory, not repo root: it is not yet confirmed whether `pkl project package`
zips only the modules a `PklProject` declares or the whole containing directory tree.
Scoping the directory to exactly the files meant for publication makes that question moot
regardless of the answer, and avoids ever accidentally bundling heraut's Go source tree into
a "Pkl package" zip. The first implementation task validates the exact mechanics with a
local spike against the `pkl` CLI before anything is wired into CI.

```pkl
module heraut.Builtins

commit_verify: Step = new {
  check = "heraut commit verify --file {{ commit_msg_file }}"
}
```

This calls the `heraut` binary directly — **not** `go run ./cmd/heraut`, which only makes
sense inside heraut's own source checkout. Consuming repos already install `heraut` via the
Homebrew tap or mise for their own `heraut release`/`heraut check` calls, so assuming it's on
`PATH` matches how those repos already use heraut everywhere else.

One Step today. The module is structured so adding siblings later (e.g. if the
`heraut commit check`/wizard ideas from ADR-0027's "Related future work" ever ship) is an
additive top-level property, not a breaking change to `heraut.Builtins`.

### Versioning — one tag, one release, one package

The Pkl package is versioned 1:1 with heraut's own release tag, exactly like hk versions its
own `Builtins.pkl` against its own releases. There is no independent version number to
track, bump, or explain — consumers pin `heraut@X.Y.Z` the same way they already pin
`hk@X.Y.Z`.

`pkl/PklProject`'s `version` field cannot be hand-maintained in the committed file the way a
normal dependency version is, because heraut's own version is resolved dynamically at release
time (`heraut version next`), not manually bumped in a tracked file. The release workflow
rewrites that field to `$VERSION` immediately before packaging — functionally the same kind
of build-time injection GoReleaser's `-ldflags` already does for the Go binary's version
string, just via a template/sed step instead, since Pkl has no equivalent of `-ldflags`.

### Release pipeline integration — reuse the existing asset-upload path

`.github/workflows/release.yml` gains one new step, after "Collect release binaries" and
before "Attest build provenance":

```yaml
- name: Package Pkl builtin
  run: |
    sed -i "s/^version = .*/version = \"${VERSION#v}\"/" pkl/PklProject
    pkl project package pkl/ --output-path dist/
```

(Illustrative — exact `pkl project package` flags and output filename are confirmed by the
Task 1 spike, not asserted here as final.) `.config/heraut.yml`'s `release.assets` gains one
new glob entry alongside the existing binary/checksum patterns. From there, the existing
`heraut release` step uploads the Pkl package zip exactly like it already uploads the five
platform binaries and `checksums.txt` — **no new Go code, no new heraut feature is needed**;
this is pure reuse of the asset-upload path `internal/platforms/github` already has contract
tests for.

### Testing — one real-CLI smoke test, no new application code to unit-test

Per the `testing.md` "Real-CLI smoke tests" exception (the same one covering git-cliff's
embedded-config acceptance check): one skippable Go test shells out to the real `pkl`
binary, runs `pkl project package pkl/ --output-path <t.TempDir()>` with a fixed test
version, asserts success, and `t.Skip`s if `pkl` is absent from `PATH`. This rides the
existing `go test ./...` gate already wired into `ci.yml` — a broken `Builtins.pkl` fails CI
on the PR that breaks it, not at the next real release.

## Consequences

- heraut becomes the first package in the `adaouat` organization to publish its own Pkl
  package, rather than only consuming hk's.
- `bifrost`, `forge`, and `hermes` each gain the *option* to replace their `cocogitto` hk
  step with `["heraut-commit-lint"] = HerautBuiltins.commit_verify` and drop their
  `cocogitto`/`cog` mise tool and shell alias — but doing so is each repo's own follow-up
  work, tracked in that repo, not part of this task. This ADR and T118 cover publishing
  only.
- `pkl/PklProject`'s `version` field is meaningless outside a release build (it only becomes
  correct momentarily, mid-CI-run, right before packaging) — anyone inspecting it on `main`
  sees a stale or placeholder value. This mirrors how `cmd/heraut/main.go`'s `Version` build
  flag is also meaningless until GoReleaser injects it; the same caveat, just for a config
  file instead of a Go `-ldflags` value.
- The Pkl package zip is not covered by the existing build-provenance attestation
  (`actions/attest`, scoped to `checksums.txt`'s binary subjects), and there is no automated
  check that the published `package://` URL actually resolves for a consumer post-release —
  both are explicit, intentional deferrals (see below), not oversights.

## Deferred (explicitly, not silently dropped)

- **Provenance attestation for the Pkl zip.** Today's `actions/attest` step only covers the
  Go binaries via `checksums.txt`. Extending it to the Pkl package zip is straightforward
  (add it to the same checksums file before attestation) but isn't required for the builtin
  to function, so it's deferred rather than bundled into T118.
- **Automated post-release `package://` resolution check.** Whether
  `package://github.com/adaouat/heraut/releases/download/vX.Y.Z/heraut@X.Y.Z#/Builtins.pkl`
  actually resolves can only be verified *after* a real tagged release has published it —
  there's nothing to check before the first release ships. A follow-up task can add a
  scheduled/manual CI job that runs `pkl eval` against the latest published package; until
  then this is a manual verification step after the first release that includes T118.
