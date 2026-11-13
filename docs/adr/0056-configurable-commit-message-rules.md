# ADR-0056: `commits.rules` — configurable commit-message pattern rules

- **Status**: Accepted
- **Date**: 2026-09-13
- **Deciders**: bchatard

---

## Context

`heraut commit verify`/`check` already enforce two policy dimensions on commit messages,
both in `app.VerifyCommit` (`internal/app/commit.go:51`): an allow-list of types
(`commits.types`) and, when `scopes_restricted: true`, an allow-list of scopes
(`commits.scopes`). Projects want additional, project-specific constraints heraut has no
way to express today — e.g. rejecting commits containing a banned string (`WIP`,
secret-looking patterns) or requiring that a commit reference a ticket.

`commits.tickets` (`internal/config/config.go:47`) already defines one or more
`{pattern, url}` pairs, but they are purely additive: `native.MatchTickets` uses them to
*link* ticket references found in a message when rendering the changelog. Nothing today
*requires* a commit to match one of those patterns.

Two shapes were considered for expressing new pattern constraints:

1. A small number of purpose-built fields (e.g. `deny_patterns: [{regex, message}]` +
   `tickets_required: bool`).
2. One generic `commits.rules` list of matcher objects, mirroring the "matcher → outcome"
   shape `versioning.bump[]` already uses (`BumpRule{Type, Regex, Breaking} → Bump`,
   `internal/config/config.go:100`).

## Decision

**Add `commits.rules`, a list of `CommitRule` objects.** Each rule expresses exactly one
constraint against a chosen part of the message, optionally scoped to specific commit
types/scopes, and is evaluated by `app.VerifyCommit` for every commit `heraut commit
verify`/`check` sees.

```yaml
commits:
  tickets:
    - pattern: '[A-Z]{2,}-\d+'
      url: "https://jira.example.com/browse/{ticket}"

  rules:
    - name: no-wip-markers
      deny: '(?i)\bwip\b'
      message: "commit message must not contain WIP"

    - name: require-ticket-on-fix
      require_ticket: true
      target: footer
      types: [fix]
      message: "fix commits must reference a ticket in a footer trailer (e.g. Refs: JIRA-123)"
```

```go
// CommitRule enforces one pattern constraint on a commit message, checked by
// `heraut commit verify`/`check` alongside the type/scope checks. Exactly one of Deny,
// Require, or RequireTicket must be set.
type CommitRule struct {
	Name          string   `yaml:"name"`
	Deny          string   `yaml:"deny,omitempty"`           // regex that must NOT match Target
	Require       string   `yaml:"require,omitempty"`        // regex that MUST match Target
	RequireTicket bool     `yaml:"require_ticket,omitempty"` // shorthand: Target must match one commits.tickets[].pattern
	Target        string   `yaml:"target,omitempty"`         // header | body | footer | message (default: message)
	Message       string   `yaml:"message,omitempty"`        // shown on failure; required for deny/require
	Types         []string `yaml:"types,omitempty"`          // optional scoping: only these commit types
	Scopes        []string `yaml:"scopes,omitempty"`         // optional scoping: only these commit scopes
}
```

**`require_ticket` reuses `commits.tickets` instead of a duplicated regex.** A commit
matches when the target text matches *any* configured `commits.tickets[].pattern` — the
same OR-across-patterns behavior `native.MatchTickets` already gives changelog linking.
This keeps `commits.tickets` the single source of truth for "what counts as a ticket
reference"; a `require: '<jira-regex>'` alternative would force rule authors to hand-copy
a pattern that already exists elsewhere and let the two drift.

**`target` maps onto fields `conventionalcommit.Parse` already produces**
(`internal/conventionalcommit/conventionalcommit.go:38`): `header` is the raw first line,
`body` is `Commit.Body`, `footer` is the footer lines joined, `message` (default) is the
full raw text — today's implicit behavior, unchanged when `target` is omitted. This lets a
rule scope a ticket requirement to a footer trailer (a common commitlint-style convention)
or keep a deny pattern from tripping on unrelated footer trailers like `Co-Authored-By`.

**Validation** (`internal/config/validator.go`), mirroring the existing
`scopes_restricted` + non-empty-`Scopes` check:

- Exactly one of `deny` / `require` / `require_ticket` must be set per rule.
- `deny` / `require` must compile as valid regex — checked at config-validate time, not
  deferred to the first commit that hits the rule.
- `require_ticket: true` requires a non-empty `commits.tickets` — a project enabling it
  before defining ticket patterns gets a config error, not a rule that can never pass.
- `message` is required when `deny` or `require` is set (not derivable); optional for
  `require_ticket`, which gets a synthesized default.
- `target`, if set, must be one of `header` / `body` / `footer` / `message`.

**Evaluation semantics:**

- `types` / `scopes` are AND-scoping filters on the rule — a rule with `types: [fix]` only
  evaluates against commits parsed as `fix`; omitting both applies the rule to every
  commit.
- All matching rules for a commit are evaluated and every failure collected into one
  error, rather than returning on the first failure like the existing type/scope checks —
  a single commit can trip multiple rules, and `commit check` scans many commits per run.
- Rule regexes compile once (config-validate time), not per commit — `commit check` calls
  `VerifyCommit` once per commit in a range, so per-call compilation would scale with
  range size for no reason.

## Consequences

- **Real config-schema growth**, tracked as the roadmap tasks below rather than in this
  ADR: `internal/config/commits.go` (`CommitRule` struct, `Commits.Rules`),
  `internal/config/validator.go`, `schema.json`, `docs/heraut.sample.yml`,
  `docs/specs/02-configuration.md` (new `commits.rules` section).
- **`heraut commit create` (the interactive wizard) is unaffected.** `commits.rules` is a
  `verify`/`check`-time gate, not a wizard-time prompt. Whether the wizard should surface
  rule violations live as the user types is a separate decision, out of scope here.
- **`require_ticket` is a soft coupling to `commits.tickets`**, enforced at config-validate
  time rather than silently: turning it on with no ticket patterns configured is a config
  error, not a rule nobody can ever satisfy.

## Alternatives considered

- **Two narrow fields — `deny_patterns` + `tickets_required: bool`.** Rejected: simpler
  for exactly the two cases named today, but neither can express a type/scope-conditional
  rule (e.g. "only `fix` commits need a ticket") without inventing a second scoping
  mechanism per field. The generic `rules` list gets `types`/`scopes` scoping once, shared
  by every rule.
- **No `require_ticket` shorthand — express "must reference a ticket" via `require` with a
  hand-written regex.** Rejected: `commits.tickets` can hold multiple patterns; forcing
  the rule author to hand-write an OR'd regex duplicates config that already exists for
  changelog linking and will drift out of sync as ticket patterns are added or changed.
- **No `target` field — always match the full raw message.** Rejected: the
  ticket-in-footer convention is common enough, and `conventionalcommit.Parse` already
  produces the header/body/footer split for free, that not exposing it would force users
  into looser regexes to approximate scoping heraut can already do precisely.

## Addendum (2026-11-11, revised 2026-11-12): breaking-commit filter, `refuse`, named tickets

heraut's own config needed to refuse breaking-change commits, so that a major release is only cut on
purpose; users also wanted "a breaking change must reference a ticket" and "`docs` commits reference
Jira, the others GitHub". None of these could be written: two `deny` regexes can approximate the first
(`^[A-Za-z]+(\([^)]*\))?!:` on the header, `^BREAKING[ -]CHANGE: ` on the footer) but copy what
`conventionalcommit.Parse` already decides in `Commit.Breaking`, and `require_ticket` accepts any
`commits.tickets` entry.

A rule now has **filters**, which pick the commits it checks, and **exactly one kind**, which says
what is checked:

- **`is_breaking: true`**, a filter beside `types` and `scopes`: the rule only checks commits the
  parser marks breaking. `false` is the same as leaving it out.
- **`refuse: true`**, a fourth kind beside `deny`, `require` and `require_ticket`: it refuses every
  commit the filters select. It needs at least one filter, or it would refuse every commit, and takes
  no `target`, since it matches no text.
- **`commits.tickets[].name`**, optional and unique, and a rule's **`tickets: [names]`**, which
  narrows `require_ticket` to those entries. Names reference entries the way `forges[].name` does.

Decisions along the way:

- **`deny_breaking: true` was dropped before release.** The first version of this addendum added it
  as a kind. It is a verdict ("breaking means refused"), and "a breaking change must reference a
  ticket" needs a condition: on a breaking commit with a ticket, `deny_breaking` and `require_ticket`
  would disagree. `is_breaking` + `refuse` expresses both. It never shipped, so it has no deprecation.
- **`refuse: true`, not `deny: true`.** `deny` already takes a regex; one key with two types was
  rejected for the same reason as the next point.
- **`tickets` beside `require_ticket`, not `require_ticket: <name>`.** One key taking `true`, a name
  or a list complicates the schema and editor completion, and a list sits next to `types`/`scopes`.
- **`tickets` needs an explicit `require_ticket: true`.** If it implied the kind it would be a kind of
  its own, and could not later refine another kind without a breaking change.

**`except_types` / `except_scopes`** (added the same day, before release) mirror `types`/`scopes` and
leave commits out, so "every type but `docs`" needs no hand-kept list. A rule sets one of each pair,
not both; a commit without a scope is not excluded by `except_scopes`. `types: ["!docs"]` was rejected
because an unquoted `!docs` is a YAML tag, and `types: {except: [...]}` because one key would take two
shapes.

Left out: an "only non-breaking commits" filter, ticket names in any output, and the `commit create`
wizard.
