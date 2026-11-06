# What goes into the changelog

heraut writes two documents from the same commits: `CHANGELOG.md`, which keeps every release in the
repository, and the release notes published with each release. This page says which commits appear,
where, and where their links come from.

## One section per release

Each release adds a section at the top of `CHANGELOG.md`, under the title:

```markdown
<!-- heraut-release: v2.0.0 -->
## [2.0.0](https://github.com/acme/app/compare/v1.0.0..v2.0.0) - 2026-11-06

### 🚀 Features

- *(api)* Add search - ([838b0e3](https://github.com/acme/app/commit/838b0e3…)) by @ada in [#12](https://github.com/acme/app/pull/12)
- [**breaking**] Drop the v1 sync endpoint - ([ee745a2](https://github.com/acme/app/commit/ee745a2…))

### 🐛 Bug Fixes

- Crash on empty query - ([e894217](https://github.com/acme/app/commit/e894217…))
```

The section lists the commits between the previous release and this one. Earlier sections are left as
they are: heraut inserts the new one and doesn't rewrite the rest. The `<!-- heraut-release: … -->`
comments mark where each section starts; keep them. `heraut changelog --regenerate` rebuilds the whole
file from the history, for instance after you change a template.

## Which commits, and in which section

Each commit goes to the section of its type:

| Type | Section |
|---|---|
| `feat` | 🚀 Features |
| `fix` | 🐛 Bug Fixes |
| `security` | 🛡️ Security |
| `refactor` | 🚜 Refactor |
| `docs` | 📚 Documentation |
| `perf` | ⚡ Performance |
| `style` | 🎨 Styling |
| `test` | 🧪 Testing |
| `chore`, `ci` | ⚙️ Miscellaneous Tasks |
| `revert` | ◀️ Revert |
| `build`, other types, messages that aren't conventional commits | 💼 Other |

Some commits never appear:

- release commits (`chore(release): …`) and dependency updates (`chore(deps): …`, `chore(deps-dev): …`),
  and the `chore(pr)` / `chore(pull)` commits some bots make;
- merge commits, and `fixup!` / `squash!` commits;
- anything matching your own `rendering.excludes`:

```yaml
rendering:
  excludes:
    - type: style
    - regex: '^docs\(typo\)'
```

Hidden commits still count for the version: a release of only `chore(deps)` commits is still a patch,
with an empty section. To stop them releasing, see
[How the next version is found](next-version.md#changing-the-levels).

Sections and their names come from `commits.types`: rename one with `render`, reorder them with
`order`, or give a new type its own section. See the configuration reference's
[`commits`](../reference/configuration.md#commits) keys.

## Breaking changes

A commit with `!` (`feat!: …`) or a `BREAKING CHANGE:` footer stays in its type's section, marked
`[**breaking**]`. It is what makes the release a major one.

## The release notes

The release notes of a version use the same sections and lines, with a few differences:

| | `CHANGELOG.md` | Release notes |
|---|---|---|
| Covers | every release, newest first | this release only |
| Lives | in the repository, committed | on the forge's release page |
| Commit bodies and footers | no | yes, under each line |
| New Contributors ❤️ | no | yes, people whose first contribution is in this release ([counted twice?](../guides/pr-data-and-tokens.md#someone-listed-as-new-twice)) |
| Commit Statistics | no | yes: commit counts, days covered, tickets |
| A pre-release | adds nothing | has its own notes, covering the changes since the previous candidate |

`heraut notes <tag>` prints a third, plainer form, for an app store: see
[Release notes for an app store](../guides/store-release-notes.md).

## Where the links come from

| Link | Comes from |
|---|---|
| The commit hash, the compare link in the heading | the forge the document is for: the CI job's forge, or each publish target's |
| The pull request (`#12`, `!12`) and `@handle` | the forge's API, when a token can read it; see [Pull request data in the changelog](../guides/pr-data-and-tokens.md) |
| Tickets (`APP-12`) | your `commits.tickets` patterns; see [Link tickets](../guides/tickets.md) |

Without a forge to link to (a plain `heraut changelog` on your machine, outside CI), the hashes are
plain text. When you publish to several forges, each one's notes link commits on that forge, while
the pull request links all come from the one forge named by `commits.enrichment_forge`.

To change any of this, the lines, headings, footer or whole layout, see
[Customize the changelog and release notes](../guides/template-customization.md).
