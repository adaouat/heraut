# Release notes for an app store

An app store submission needs the release text on its own, without the decoration a forge
release body carries. `heraut notes <tag>` prints it.

## The flow

```bash
heraut release                                   # cut and publish v7.8.0
heraut notes v7.8.0 --format text --output store.txt
```

`--output` refuses to replace a file that already exists; add `--overwrite` when `store.txt` is left
from an earlier run (a fresh CI checkout never has one).

`heraut notes` only reads git. Run it after the tag exists, from any checkout that has the tag
and its history, including a shallow-free CI checkout of the tag itself.

By default it prints the groups of the range from the tag's previous release, one line per commit,
with no release heading and none of the hash, author or PR reference the changelog shows:

```markdown
### 🚀 Features

- *(auth)* Add biometric login

- [**breaking**] Drop the v1 sync API ([PROJ-12](https://example.com/PROJ-12))


### 🐛 Bug Fixes

- *(ui)* Fix crash when rotating the settings screen
```

The group headings are the `commits.types` labels, emoji included; set `render` on a type to change one
(see the [`commits.types`](../specs/02-configuration.md#commitstypes) reference). The blank lines are the
changelog's own spacing.

`--format text` converts the same notes to plain text:

```text
🚀 Features

- (auth) Add biometric login
- [breaking] Drop the v1 sync API (PROJ-12)

🐛 Bug Fixes

- (ui) Fix crash when rotating the settings screen
```

Text mode keeps text that is not formatting, such as `Option<T>` or `snake_case`.

## Shaping the output

The `notes:` block uses the template engine of the changelog, with the same blocks and data. This
one adds a heading and the short hash back:

```yaml
notes:
  rendering:
    templates:
      release:
        section: "## Version {{ .Version }}"
      commit:
        message: "- {{ .Description }} ({{ .ShortHash }})"
```

A full template file works too (`notes.template`, or `--template <file>` for one run). Layering, lowest
to highest: heraut's built-in notes blocks, the global `rendering.templates`, `notes.rendering.templates`,
then the template file. See [Template customization](template-customization.md) for the blocks and
fields.

Since the text conversion reads the Markdown your templates produce, a template that prints plain
lines needs no special case.

## Release candidates

A release candidate is bounded by the previous tag of any kind, so its notes hold only what changed
since the last candidate. The final is bounded by the previous *final*, so its notes span every
candidate. To get one text for a whole stabilisation cycle without cutting the final first, widen
the bound:

```bash
heraut notes v7.8.0-rc.3 --from v7.7.0 --format text
```

## Exit codes

| Situation | Code |
|---|---|
| unknown tag, a tag that is not a release tag, unknown `--from`, `--from` outside the tag's history, unwritable `--output`, git failing | 3 (runtime) |
| `--format` other than `markdown` or `text`, `--output` naming an existing file without `--overwrite` | 1 (usage) |
| per-env strategy without `--env` | 2 (config) |

A range with no notable commit prints nothing on stdout, exits 0 and says so on stderr.
