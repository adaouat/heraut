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
