# ADR-0013: Raw Binary GoReleaser Format (No Archives)

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

> **Note (2026-06-04).** `builds.binary` is now plain `heraut`, not the versioned
> `heraut_<version>_<os>_<arch>` shown below, so the Homebrew cask installs the binary as
> `heraut`. The versioned **asset** name is unchanged — it comes from `archives.name_template`,
> and the release workflow maps each build output to its versioned name via goreleaser's
> `artifacts.json`. The raw-binary decision and the asset/checksum naming below still hold;
> only where the version lives in the config moved (from `builds.binary` to `name_template`).

> **Note (2026-09-26, T323).** Added a second `archives:` entry (`id: homebrew`,
> `formats: tar.gz`) alongside the raw-binary one, used only by `homebrew_casks:` via
> `ids: [homebrew]`. `before.hooks` now pre-generates `completions/*` and a gzipped
> `manpages/heraut.1.gz`, bundled into that archive so the cask's static `completions:`
> and `manpages:` fields can install them — GoReleaser's `generate_completions_from_executable`
> was tried instead (T321) and hangs under Gatekeeper on heraut's unsigned/non-notarized
> binaries; the static fields never execute the installed binary, so that gap doesn't apply.
> Verified locally: a real `brew install --cask` against a scratch tap linked the binary,
> man page, and all three completions in well under a second, with no hang. Confirmed the
> quarantine attribute is still set on the installed binary itself (`xattr -l` shows
> `com.apple.quarantine`) and that *running* it hangs until the flag is removed — that is
> the pre-existing, unrelated Gatekeeper gap already called out in the README's "Prebuilt
> binary" caveat, not something this change touches. The direct-download raw binary
> (`id: default`) is untouched: this decision's core rationale for it — no tar/unzip step
> for `curl`+`chmod` installs — still holds. Its other original rationale, avoiding
> extraction code in heraut's own self-update, is now moot regardless of archives, since
> self-update was removed entirely ([ADR-0014](0014-self-update-architecture.md),
> superseded by forge ADR-0005) — `forge/updatecheck` only prints a hint and never
> downloads or extracts anything.

## Context

GoReleaser is the build tool ([Spec 06 — CI](../specs/06-dx-and-testing.md#ci)). The
two natural archive formats it supports are `tar.gz` (Linux / macOS) and `zip`
(Windows). Wrapping the heraut binary in an archive would require:

1. **Extraction code** in `heraut self-update`: `archive/tar`, `archive/zip`,
   `compress/gzip` stdlib imports and ~70 lines of extraction logic with two separate
   code paths, including a path-traversal guard for zip-slip.
2. **Archive-aware asset naming**: the self-update code would need to know which format
   to expect per platform.
3. **User friction**: initial install would require
   `tar -xzf heraut_<ver>_<os>_<arch>.tar.gz heraut` instead of a simple `chmod +x`.

heraut bundles no extra files alongside the binary — no README, no man pages, no shell
completions (completions are produced by a subcommand at runtime). The archive wrapper
would be pure overhead.

## Decision

Use GoReleaser's `format: binary` (no archive wrapper).

GoReleaser produces a single executable per target platform, named per `name_template`:

| Target          | Binary                                |
|-----------------|---------------------------------------|
| linux/amd64     | `heraut_<version>_linux_amd64`        |
| linux/arm64     | `heraut_<version>_linux_arm64`        |
| darwin/amd64    | `heraut_<version>_darwin_amd64`       |
| darwin/arm64    | `heraut_<version>_darwin_arm64`       |
| windows/amd64   | `heraut_<version>_windows_amd64.exe`  |

The SHA-256 checksum file (`heraut_<version>_checksums.txt`) covers the binaries
directly — consumers verify what they actually run, not a wrapper.

GitHub Releases hosts the binaries + the checksum file. GoReleaser creates the release
directly (`release: disable: false`).

## Consequences

**Self-update simplification.**  
`internal/selfupdate/updater.go::AssetName()` returns the bare binary name (no
extension, `.exe` on Windows). `Run()` downloads the binary, verifies its checksum,
`chmod 0o755`, and atomically replaces the existing executable. The `archive/tar`,
`archive/zip`, and `compress/gzip` stdlib imports are absent. The extraction layer
(~70 lines, two code paths) does not exist.

**Security surface reduction.**  
Tar extraction is a historic source of path-traversal vulnerabilities (zip-slip).
Removing extraction removes that entire class of risk.

**Simpler install UX.**  
Initial install is:

```bash
curl -L -o heraut https://github.com/adaouat/heraut/releases/latest/download/heraut_<version>_<os>_<arch>
chmod +x heraut
sudo mv heraut /usr/local/bin/
```

No `tar` or `unzip` required.

**Checksum semantics.**  
The checksum is over the binary users execute rather than a wrapper. If the binary is
tampered with after extraction, the archive-based model would not catch it; the raw
model does.

**Future bundling foreclosed for the direct-download binary.**  
Resolved for Homebrew by the 2026-09-26 note above: a second, cask-only `tar.gz` archive
now carries completions and the man page without touching the raw-binary asset. The
direct `curl`+`chmod` download path still has no bundled completions/man page — those
users still reach for `heraut completion bash`/`heraut man` directly, which fang provides
out of the box.

**CI artifact glob.**  
The release workflow uploads `dist/heraut_*` to match binaries and the checksum file
uniformly.
