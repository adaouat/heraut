# ADR-0014: Self-Update Architecture

- **Status**: Superseded by forge ADR-0005
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

> **Superseded (2026-06-04).** heraut no longer self-replaces its binary: the
> `internal/selfupdate` package and the `heraut self-update` command were removed. Upgrades are
> delegated to whatever installed the binary (mise / Homebrew / `go install` / curl). The
> once-per-24h update **check** described below survives, but now comes from forge's shared
> `updatecheck` package, which prints a hint with the detected upgrade command — it never
> downloads or replaces the binary. See **forge ADR-0005 — Updates via package managers, not
> binary self-replacement**. The design below is kept as a historical record.

## Context

heraut is distributed as a downloadable raw binary attached to each GitHub Release
([ADR-0013](0013-raw-binary-goreleaser-format.md)). Once a user installs it locally,
there is no mechanism to receive updates — they would otherwise have to manually
revisit the release page and re-download.

This ADR designs the self-update feature against the **public GitHub Releases API**,
which requires no authentication for either the version check or the binary download.

Two constraints shape the design:

1. **The repository is public.** Anyone can read the Releases API without a token —
   anonymous requests are subject to GitHub's IP-based rate limit (60 req/hr), which is
   ample for a self-update tool.
2. **No separate manifest hosting is needed.** The GitHub Releases API exposes
   `/repos/<owner>/<repo>/releases/latest` directly — heraut does not publish a
   `latest.json` of its own.

## Decision

Self-update uses the GitHub Releases API directly. The same endpoint serves both the
version check and the download URL resolution.

### Endpoint

```
GET https://api.github.com/repos/adaouat/heraut/releases/latest
```

The response includes:

- `tag_name` — the latest version (e.g. `v1.2.3`)
- `assets[]` — the uploaded binaries + `checksums.txt`

No authentication. heraut does not pass an `Authorization` header. The rate limit
applies; if heraut is invoked from a CI runner that exceeds it, the background hint
fails silently and `heraut self-update --check` reports the rate-limit error verbatim.

### Background hint

After every successful command (except `heraut self-update` itself), heraut performs a
non-blocking HTTP GET against the latest-release endpoint with a 500 ms timeout. If a
newer version is available, it prints a single hint line to stderr after the command's
normal output:

```
hint: heraut v1.2.3 available — run: heraut self-update
```

On timeout, network failure, or rate-limit: silently skipped. The hint must never block
or error a real command invocation.

The hint is disabled when:

- The command being run is `heraut self-update` (avoids confusing output during update)
  — gated in `internal/cmd/root.go`'s `PersistentPostRunE`
- `HERAUT_CHECK_UPDATE=false` is set
- The binary is a `dev` build (no `main.Version` ldflag) — can't reliably compare
- The Updater's `latestURL` is empty — in practice only when a test overrides it via
  `WithLatestURL("")`, since production builds use the compiled-in constant

A short-lived cache at `$XDG_CACHE_HOME/heraut/update-check.json` (fallback
`~/.cache/heraut/update-check.json`) holds the last check timestamp and result; the
hint runs at most once per 24 hours.

### `heraut self-update --check`

Same endpoint, explicit, bypasses the cache. Exits 0 when up to date, exits 1 when an
update is available. Prints both the current and latest version. No download.

### `heraut self-update` (perform the update)

1. Fetch `/releases/latest`. On non-2xx: surface the status code + body with a hint
   (rate-limit message, transient failure, etc.).
2. From the response, find the asset whose name matches the current OS / arch + binary
   suffix (`heraut_<version>_<os>_<arch>[.exe]`).
3. Download the matching binary to a `.new` file beside the running binary (same
   directory, same filesystem — required for atomic rename).
4. Download `checksums.txt` from the same release.
5. Verify the binary's SHA-256 against the checksum entry. Mismatch → delete the temp
   file, abort, leave the existing binary untouched.
6. Set executable bit (`chmod 0o755`) on the temp file.
7. Replace the existing binary atomically (see § Binary replacement).
8. On macOS only: remove the `com.apple.quarantine` extended attribute via
   `xattr -d com.apple.quarantine <new-path>` so Gatekeeper does not block execution.

### Binary replacement

A running process cannot overwrite its own executable safely:

- On POSIX, the inode is in use; a direct write would corrupt a concurrently running
  copy.
- On Windows, the file is exclusively locked.

The standard write-beside + atomic rename pattern is used:

```
1. Resolve os.Executable()         → /usr/local/bin/heraut  (resolves symlinks)
2. Download new binary             → /usr/local/bin/heraut.new  (same dir)
3. chmod 0o755 heraut.new
4. os.Rename(.new, heraut)         → atomic on POSIX (single syscall, same filesystem)
```

On **POSIX**, `rename(2)` is atomic. The running process holds the old inode through
its open file descriptor; renaming the directory entry does not affect it. The next
invocation reads the new binary from the replaced path.

On **Windows**, `os.Rename` fails while the target is in use. The fallback is to
rename the *old* binary to `heraut.old` first (which succeeds even while running), then
rename the new binary into place. `heraut.old` is cleaned up on the next successful
invocation.

**Same-filesystem requirement**: the temp file is written beside the target (not in
`/tmp`) so `rename` stays within one filesystem and remains atomic. Cross-mount renames
silently degrade to a non-atomic copy.

**Permission failure**: if `os.Rename` fails with permission denied:

```
error: permission denied replacing /usr/local/bin/heraut
  hint: run with elevated privileges: sudo heraut self-update
```

### Project and latest-release URLs

The project URL and the latest-release API endpoint are **compiled-in constants** in
`internal/selfupdate/updater.go`, not build-time ldflags:

- `defaultProjectURL` — `https://github.com/adaouat/heraut`. Used for the install hint
  in error messages.
- `defaultLatestURL` — `https://api.github.com/repos/adaouat/heraut/releases/latest`.
  Used directly by the self-update code to fetch the latest release manifest.

Because heraut targets a single, fixed public repository, these URLs never vary per
build, so hardcoding them as constants is simpler than threading two extra ldflags
through `main` → `cmd` → `selfupdate`. Tests override the endpoint with the
`WithLatestURL(...)` option (backed by an `httptest.Server`).

The only build-time ldflag is `main.Version` (`-X main.Version={{.Tag}}`), injected by
both `.goreleaser.yml` and `Dockerfile`; GoReleaser is the source of truth (see
`CLAUDE.md` § ldflags invariant).

## Alternatives considered

**Publish a separate manifest (e.g. via GitHub Pages)** — adds a hosting layer with no
benefit on a public GitHub repo, where the Releases API endpoint already serves the
same purpose without authentication.

**Use `creativeprojects/go-selfupdate`** — the library would replace ~100 lines of
download / extract / replace logic but the remaining custom code (XDG cache, background
hint, actionable errors, macOS quarantine) would still need to be written. The custom
implementation using `net/http` + `os.Chmod` + `os.Rename` adds no dependencies and is
fully testable with `httptest.NewServer`.

## Consequences

- No separate manifest hosting; the GitHub Releases API is the single source of truth.
- The background hint adds at most one HTTP GET (500 ms timeout) per 24 hours.
- Anonymous requests are rate-limited to 60/hr/IP — heraut's check happens infrequently
  enough that this is not a practical limit for a single user, but a shared CI runner
  could occasionally hit it. The graceful skip ensures this is not a user-visible
  failure.
- Windows self-update requires the `.old` swap pattern. This is the only
  platform-specific branch in the implementation.
- macOS users do not need to manually remove the quarantine xattr after self-update.
