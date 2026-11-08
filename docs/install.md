# Install heraut

The [README](../README.md#install) covers the two quickest ways, Homebrew and mise with
verification. This page covers the others, how to check what you downloaded, and how to upgrade.

heraut needs `git` on your `PATH`. Nothing else: publishing and changelog generation are built in.
The Docker image ships with `git`.

## mise without verification

The `packslip:` backend checks the download's signature before installing; it needs mise 2026.9.2
or newer. On an older mise, or to skip the check, use the plain GitHub releases backend:

```bash
mise use github:adaouat/heraut
```

## Prebuilt binary

Each [release](https://github.com/adaouat/heraut/releases) has one raw binary per platform, with no
archive to unpack:

| Platform | File |
|---|---|
| Linux x86-64 | `heraut_<version>_linux_amd64` |
| Linux ARM64 | `heraut_<version>_linux_arm64` |
| macOS Intel | `heraut_<version>_darwin_amd64` |
| macOS Apple silicon | `heraut_<version>_darwin_arm64` |
| Windows x86-64 | `heraut_<version>_windows_amd64.exe` |

The release tag has a `v` and the file name doesn't: version `1.2.3` is tag `v1.2.3`, file
`heraut_1.2.3_linux_amd64`.

```bash
VERSION=1.2.3   # the release you want
curl -fsSLo heraut "https://github.com/adaouat/heraut/releases/download/v${VERSION}/heraut_${VERSION}_linux_amd64"
chmod +x heraut
sudo mv heraut /usr/local/bin/
heraut --version
```

**macOS:** the binaries aren't notarized by Apple yet, so a binary downloaded with a browser is
quarantined and macOS refuses to run it. Clear the flag once:

```bash
xattr -d com.apple.quarantine heraut
```

### Check the download

Every release has a `checksums.txt`. Download it next to the binary, then:

```bash
sha256sum --ignore-missing -c checksums.txt        # Linux
shasum -a 256 --ignore-missing -c checksums.txt    # macOS
```

The line for your file must say `OK`.

The binaries are also signed in CI with Sigstore: the release carries a
[packslip](https://packslip.dev) manifest (`packslip.sigstore.json`), which is what
`mise use packslip:…` checks, and a GitHub build attestation you can check with the GitHub CLI:

```bash
gh attestation verify heraut_1.2.3_linux_amd64 --repo adaouat/heraut
```

Each binary also has a software bill of materials next to it (`….sbom.json`).

## Docker

The image bundles heraut and `git`:

```bash
docker run --rm ghcr.io/adaouat/heraut:latest --version

# run against the repository in the current directory
docker run --rm -v "$PWD":/repo -w /repo ghcr.io/adaouat/heraut:latest release --dry-run
```

Image tags have no `v`:

| Tag | Meaning |
|---|---|
| `latest` | The latest release |
| `1.2.3` | That exact version |
| `1.2` | The latest patch of 1.2 |
| `1` | The latest release of 1.x |

Pass the tokens heraut needs with `-e`, for example `-e GITHUB_TOKEN`.

## `go install`

With Go installed:

```bash
go install github.com/adaouat/heraut/cmd/heraut@latest
```

## Check your setup

In a repository with a heraut config, `heraut check runtime` checks that `git` is installed, your git
user is set, and the tokens your config needs are present and valid.

## Upgrade

Once a day, after a command, heraut checks for a newer release and prints a one-line hint with the
upgrade command for your install method. It never replaces itself. Upgrade the way you installed:

| Installed with | Upgrade with |
|---|---|
| Homebrew | `brew upgrade heraut` |
| mise | `mise upgrade heraut` |
| Prebuilt binary | download the new one |
| Docker | pull the new tag |
| `go install` | run `go install …@latest` again |

It doesn't check after `--dry-run`, `--offline` or `--json` runs, or after `heraut check config`. To
turn the check off everywhere, set `HERAUT_CHECK_UPDATE=false`.
