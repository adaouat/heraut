# Commit rules

`commits.rules` adds checks of your own to `heraut commit verify` and `heraut commit check`, on top of
the allowed types and scopes. A commit that breaks a rule is refused with the rule's message, and the
exit code is 1. To run the checks in a commit-msg hook and on every pull request, see
[Check commit messages](commit-conventions.md).

## What a rule is made of

Each rule has a `name`, says **which commits** it checks, and **what** it checks in them.

**Which commits.** Leave these out and the rule checks every commit; set several and a commit must
match all of them.

| Setting | Checks only |
|---|---|
| `types: [feat, fix]` | commits of these types |
| `scopes: [api]` | commits of these scopes |
| `except_types: [docs]` | commits of every other type. Not with `types` on the same rule. |
| `except_scopes: [deps]` | commits of every other scope, and commits without a scope. Not with `scopes` on the same rule. |
| `is_breaking: true` | breaking changes: `feat!:`, `feat(api)!:`, or a `BREAKING CHANGE:` or `BREAKING-CHANGE:` footer |

**What is checked.** Exactly one of these per rule:

| Check | Refuses a commit when |
|---|---|
| `deny: <regex>` | the regular expression matches |
| `require: <regex>` | the regular expression doesn't match |
| `require_ticket: true` | no ticket is mentioned, by the patterns in `commits.tickets` (see [Link tickets](tickets.md)). Add `tickets: [jira]` to accept only the entries with those names. |
| `refuse: true` | always: every commit the rule checks is refused. It needs at least one setting from the table above. |

Then, optionally:

| Setting | Effect |
|---|---|
| `target` | The part of the message `deny`, `require` and `require_ticket` look at: `header` (the first line), `body`, `footer` (the trailers, each as `Token: value` on its own line) or `message` (everything, the default). `refuse` takes no `target`. |
| `message` | What the author sees when the rule refuses a commit. Required for `deny` and `require`; `require_ticket` and `refuse` have a default that names the rule. |

Every rule runs on every commit, and a commit that breaks several gets all their messages at once:

```console
$ heraut commit verify "fix: wip"
Commit message violates rule(s): commit message must not contain WIP; a fix names its ticket in a
footer, e.g. Refs: APP-12.
```

Merge and fixup commits are accepted as they are.

## Examples

Each example below is one entry under `commits.rules`.

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
      is_breaking: true
      refuse: true
```

```console
$ heraut commit verify "feat!: drop v1 routes"
Commit message violates rule(s): commit refused (rule "no-breaking").
```

Under `semver`, a breaking change makes the next release a major one. With this rule, no commit can
do that, so you choose when a major version comes, with `heraut release --set-version`.

Add `scopes` or `types` to protect one part of a project and leave the rest free:

```yaml
    - name: stable-api
      is_breaking: true
      refuse: true
      scopes: [api]
```

```console
$ heraut commit verify "feat(api)!: drop v1 routes

Clients move to v2."
Commit message violates rule(s): commit refused (rule "stable-api").

$ heraut commit verify "feat(ui)!: new layout

The old one is gone."
✓ commit message is valid
  type:        feat
  scope:       ui
  breaking:    true
  description: new layout
```

### A breaking change references a Jira ticket

Name the ticket patterns, then point a rule at one of them:

```yaml
commits:
  tickets:
    - name: jira
      pattern: "[A-Z]+-[0-9]+"
      url: https://jira.example.com/browse/{ticket}
    - name: github
      pattern: "#([0-9]+)"
      url: https://github.com/acme/app/issues/{ticket}
  rules:
    - name: breaking-needs-jira
      is_breaking: true
      require_ticket: true
      tickets: [jira]
```

```console
$ heraut commit verify "feat!: drop v1 routes"
Commit message violates rule(s): commit must reference a ticket of kind jira (rule
"breaking-needs-jira").
```

`feat!: drop v1 routes, see APP-7` passes. Leave `tickets` out to accept any of the patterns.

### Documentation references Jira, everything else GitHub

```yaml
    - name: docs-need-jira
      types: [docs]
      require_ticket: true
      tickets: [jira]
    - name: others-need-github
      except_types: [docs]
      require_ticket: true
      tickets: [github]
```

`except_types` keeps the second rule right when you add a type to `commits.types` later.

```console
$ heraut commit verify "docs: explain setup #12"
Commit message violates rule(s): commit must reference a ticket of kind jira (rule "docs-need-jira").

$ heraut commit verify "fix: crash on empty query APP-3"
Commit message violates rule(s): commit must reference a ticket of kind github (rule
"others-need-github").
```

### A ticket on every commit, except dependency bumps

Dependency bots commit with the `deps` scope (`chore(deps): bump yaml.v3 to 3.0.2`), and those
commits have no ticket to name:

```yaml
    - name: ticket-except-deps
      except_scopes: [deps]
      require_ticket: true
```

```console
$ heraut commit verify "chore: tidy up"
Commit message violates rule(s): commit must reference a ticket matching one of the configured
commits.tickets patterns (rule "ticket-except-deps").
```

`chore(deps): bump yaml.v3 to 3.0.2` passes. A commit without a scope is still checked.

### Freeze a part of the project

```yaml
    - name: frozen-legacy
      scopes: [legacy]
      refuse: true
      message: the legacy module is frozen
```

```console
$ heraut commit verify "fix(legacy): patch #34"
Commit message violates rule(s): the legacy module is frozen.
```

### Mistakes in a rule

`heraut check config` reports a rule that can't work:

```console
$ heraut check config
  .heraut.yml  (from .heraut.yml)
✗ commits.rules[0].refuse: needs at least one of types, scopes, except_types, except_scopes or is_breaking
  hint: without a filter this rule refuses every commit
✗ commits.rules[1].tickets: needs require_ticket: true
✗ commits.rules[2].except_types: not allowed with types
  hint: use either types or except_types
3 error(s)
```

The exit code is then 2.
