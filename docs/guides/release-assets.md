# Attach files to a release

Binaries, archives, checksums: list them with glob patterns and heraut uploads them with the
release.

```yaml
release:
  assets:
    - dist/*.tar.gz
    - dist/checksums.txt
```

Every target gets these files. To give one target its own list, set `assets` on it; that list
replaces `release.assets` for that target:

```yaml
release:
  assets:
    - dist/*.tar.gz
  targets:
    - forge: github
    - forge: gitlab
      assets:
        - dist/*_linux_*.tar.gz   # GitLab gets the Linux archives only
```

## Build the files first

heraut checks the patterns early, right after the `post_bump` hooks and before it commits, tags or
pushes anything. Each pattern must match at least one file by then, or the release stops with
nothing changed:

```text
asset pattern "dist/*.tar.gz" matches no file: build the files in a post_bump hook (or earlier),
fix the pattern, or set release.allow_missing_assets: true if a later hook builds them
```

So build before `heraut release`, in an earlier CI step, or in a `post_bump` hook, which runs once
the version is known:

```yaml
hooks:
  post_bump:
    - run: make dist VERSION={{ .Version }}
```

If the files are built later, in a `pre_tag`, `post_tag` or `pre_release` hook, set
`release.allow_missing_assets: true`. A pattern that still matches nothing at publish time is then
skipped with a warning instead of stopping the release.

`--dry-run` runs the same check, and only warns, since hooks don't run in a dry run.

## Files that are refused

Before anything is pushed, heraut also refuses:

- **a file outside the repository**, including through a symbolic link:
  `asset "dist/evil.tar.gz" resolves outside the repository (…): only files under the repository
  root can be published`;
- **two files with the same name**, from different folders;
- **on GitHub, two names GitHub would publish as one.** GitHub replaces special characters with `.`
  and ignores case, so `meta#1.tar.gz` and `meta.1.tar.gz` would collide:
  `assets "dist/meta#1.tar.gz" and "dist/meta.1.tar.gz" would both be published on GitHub as
  "meta.1.tar.gz"`;
- **on GitLab, a name the package registry rejects.** GitLab accepts only letters, digits and
  `. _ - + ~ @`, not starting or ending with `~` or `@`, so a space, a `#` or an accented letter is
  refused: `asset "dist/my app.zip" cannot be published on GitLab: GitLab package file names allow
  only letters, digits and . _ - + ~ @, and cannot start or end with ~ or @: rename the file`;
- anything that isn't a regular file, such as a named pipe. A directory a pattern matches is simply
  skipped.

## How each forge stores them

**GitHub** attaches the files to the release. A name with special characters is published under
GitHub's version of it (`meta#1.json` becomes `meta.1.json`) and heraut warns when that happens.
While uploading, the release is a draft; it is published once every file is there, and deleted if an
upload fails, so a run never leaves a release with half its files.

**GitLab** has no attachments: heraut uploads each file to the project's generic package registry
(`release-assets`, version = the tag), then creates the release with a link to each one. The files
download from `…/-/releases/<tag>/downloads/<file>`, with their names unchanged. The package
registry must be enabled on the project. The uploads come first, so a failed upload creates no
release.

## Draft releases

On GitHub, `draft: true` on a target keeps the release as a draft, files included, for you to
publish by hand:

```yaml
release:
  targets:
    - forge: github
      draft: true
```
