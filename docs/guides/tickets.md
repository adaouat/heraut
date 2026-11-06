# Link tickets

When commit messages mention tickets (`APP-12`, `#34`), heraut can turn each mention into a link in
the changelog and the release notes, and check that commits mention one.

## Patterns and links

```yaml
commits:
  tickets:
    - pattern: "[A-Z]+-[0-9]+"                         # Jira: APP-12
      url: https://jira.example.com/browse/{ticket}
    - pattern: "#([0-9]+)"                             # GitHub issues: #34
      url: https://github.com/acme/app/issues/{ticket}
```

`pattern` is a regular expression searched in the commit's subject and body. `{ticket}` in the `url`
is the first capture group, or the whole match without one: `#34` gives `34`, `APP-12` gives
`APP-12`.

The changelog then reads:

```markdown
- Search by name (APP-12) - d0c75ef ([APP-12](https://jira.example.com/browse/APP-12))
- Crash on empty query - 21fe179 ([#34](https://github.com/acme/app/issues/34))
```

The second commit's ticket is in its body (`Fixes #34`); it is linked all the same.

## Check your patterns

`heraut commit tickets` shows what the patterns find, commit by commit, and the link each mention
becomes:

```console
$ heraut commit tickets
✓ 21fe179  fix: crash on empty query
  → #34 (https://github.com/acme/app/issues/34)
✓ d0c75ef  feat: search by name (APP-12)
  → APP-12 (https://jira.example.com/browse/APP-12)
2 ticket reference(s) found across 3 commit(s) analysed.
```

It reads the whole history by default; give it a range (`v1.2.0..HEAD`) or `--from-latest-tag` to
look at less. It never fails, even when nothing matches, since a range without tickets can be
normal; it warns instead.

## Require a ticket

A rule in `commits.rules` makes `heraut commit verify` and `heraut commit check` refuse commits
without a ticket:

```yaml
commits:
  rules:
    - name: ticket
      require_ticket: true
      types: [feat, fix]          # only these types; omit to require it everywhere
```

```console
$ heraut commit verify "feat(api): add pagination"
Commit message violates rule(s): commit must reference a ticket matching one of the configured
commits.tickets patterns (rule "ticket").
```

The exit code is then 1. To run the check on every commit as you write it, and on every pull
request, see [Check commit messages](commit-conventions.md).

## In the release statistics

The release notes' **Commit Statistics** block counts the tickets the release mentions and links
each one with how many commits referenced it.
