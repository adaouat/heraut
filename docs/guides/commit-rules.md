# Commit rules

`commits.rules` adds checks of your own to `heraut commit verify` and `heraut commit check`, on top of
the allowed types and scopes. A commit that breaks a rule is refused with the rule's message, and the
exit code is 1. To run the checks in a commit-msg hook and on every pull request, see
[Check commit messages](commit-conventions.md).

## What a rule is made of

Each rule has a `name` and exactly one check:

| Check | Refuses a commit when |
|---|---|
| `deny: <regex>` | the regular expression matches |
| `require: <regex>` | the regular expression doesn't match |
| `require_ticket: true` | no ticket is mentioned, by the patterns in `commits.tickets` (see [Link tickets](tickets.md)) |
| `deny_breaking: true` | the commit is a breaking change: `feat!:`, `feat(api)!:`, or a `BREAKING CHANGE:` or `BREAKING-CHANGE:` footer |

Then, optionally:

| Setting | Effect |
|---|---|
| `target` | The part of the message `deny`, `require` and `require_ticket` look at: `header` (the first line), `body`, `footer` (the trailers, each as `Token: value` on its own line) or `message` (everything, the default). `deny_breaking` reads the whole commit and takes no `target`. |
| `types`, `scopes` | Only check commits of these types, or these scopes. With both, a commit must match both. |
| `message` | What the author sees when the rule refuses a commit. Required for `deny` and `require`; `require_ticket` and `deny_breaking` have a default that names the rule. |

Every rule runs on every commit, and a commit that breaks several gets all their messages at once:

```console
$ heraut commit verify "fix: wip"
Commit message violates rule(s): commit message must not contain WIP; a fix names its ticket in a
footer, e.g. Refs: APP-12.
```

Merge and fixup commits are accepted as they are.

## Examples

Each example below is one entry under `commits.rules`. `heraut check config` reports a rule with a
mistake in it, such as an invalid regular expression or two checks in one rule.

### No work in progress

```yaml
commits:
  rules:
    - name: no-wip
      deny: "(?i)\\bwip\\b"
      message: commit message must not contain WIP
```

```console
$ heraut commit verify "chore: wip"
Commit message violates rule(s): commit message must not contain WIP.
```

### A short subject line

```yaml
    - name: short-subject
      deny: "^.{73,}"
      target: header
      message: the subject line must be at most 72 characters
```

`target: header` keeps a long line in the body from counting.

### A fix names its ticket in a footer

```yaml
commits:
  tickets:
    - pattern: "[A-Z]+-[0-9]+"
      url: https://jira.example.com/browse/{ticket}
  rules:
    - name: fix-ticket-in-footer
      require_ticket: true
      target: footer
      types: [fix]
      message: "a fix names its ticket in a footer, e.g. Refs: APP-12"
```

A ticket in the subject doesn't count, only one in a trailer:

```console
$ heraut commit verify "fix: crash on empty query (APP-12)"
Commit message violates rule(s): a fix names its ticket in a footer, e.g. Refs: APP-12.

$ heraut commit verify "fix: crash on empty query

Refs: APP-12"
✓ commit message is valid
  type:        fix
  breaking:    false
  description: crash on empty query
  tickets:     APP-12 (https://jira.example.com/browse/APP-12)
```

### A feature explains why

```yaml
    - name: feat-explains-why
      require: "\\S"
      target: body
      types: [feat]
      message: a feat explains why in the body
```

```console
$ heraut commit verify "feat: search by name"
Commit message violates rule(s): a feat explains why in the body.
```

### No trailer left behind by another tool

```yaml
    - name: no-change-id
      deny: "(?m)^Change-Id: "
      target: footer
      message: remove the Change-Id footer
```

`(?m)` makes `^` match at the start of every trailer, not only the first.

### No breaking change

```yaml
    - name: no-breaking
      deny_breaking: true
      message: "breaking changes are not allowed: a major release is cut on purpose"
```

Under `semver`, a breaking change makes the next release a major one. With this rule, no commit can
do that, so you choose when a major version comes, with `heraut release --set-version`.

Add `scopes` or `types` to protect one part of a project and leave the rest free:

```yaml
    - name: stable-api
      deny_breaking: true
      scopes: [api]
```

```console
$ heraut commit verify "feat(api)!: drop v1 routes

Clients move to v2."
Commit message violates rule(s): breaking changes are not allowed (rule "stable-api").

$ heraut commit verify "feat(ui)!: new layout

The old one is gone."
✓ commit message is valid
  type:        feat
  scope:       ui
  breaking:    true
  description: new layout
```
