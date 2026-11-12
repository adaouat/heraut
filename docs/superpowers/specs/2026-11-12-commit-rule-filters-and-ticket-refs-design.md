# Commit rules: a breaking-commit filter, `refuse`, and named tickets — design

Roadmap: T557 (reworked) + T558, one pull request on `feat/deny-breaking-rule`.
Status: approved in conversation 2026-11-12, pending this document's review.

## Intent

`commits.rules` (ADR-0056) lets a project add its own checks to `heraut commit verify`/`check`. Two
things can't be written today:

1. **Rules about breaking changes other than "refuse them".** T557 added `deny_breaking: true`, a rule
   *kind* that refuses any breaking commit. The user wants "a breaking commit must reference a ticket"
   too, which needs "is breaking" as a *condition* on a rule, not a verdict. `deny_breaking` can't be
   combined with `require_ticket`: on a breaking commit with a ticket, one says refuse and the other
   accept.
2. **Rules about which ticket.** `commits.tickets` can list several patterns (Jira keys, GitHub issues),
   but `require_ticket: true` accepts any of them. The user's example: `docs` commits must reference a
   Jira ticket, every other type a GitHub issue.

The model the user settled on separates the two roles of a rule cleanly:

- **Filters** pick which commits a rule checks: `types`, `scopes`, and new `is_breaking`.
- **Kinds** say what is checked, exactly one per rule: `deny`, `require`, `require_ticket`, and new
  `refuse`.

## The configuration

```yaml
commits:
  tickets:
    - name: jira                                   # new, optional
      pattern: "[A-Z]+-[0-9]+"
      url: https://jira.example.com/browse/{ticket}
    - name: github
      pattern: "#([0-9]+)"
      url: https://github.com/acme/app/issues/{ticket}
  rules:
    - name: no-breaking                            # refuse every breaking commit
      is_breaking: true
      refuse: true
    - name: breaking-needs-ticket                  # a breaking commit references any ticket
      is_breaking: true
      require_ticket: true
    - name: breaking-needs-jira                    # ... a Jira ticket specifically
      is_breaking: true
      require_ticket: true
      tickets: [jira]
    - name: docs-need-jira
      types: [docs]
      require_ticket: true
      tickets: [jira]
```

### `commits.tickets[].name` (new)

- Optional string. A rule can only point at a named entry.
- Unique among `commits.tickets` when set. Error at `commits.tickets[i].name`:
  `duplicate ticket name "jira" (already used by tickets[0])`, mirroring `forges[].name`.
- No format restriction beyond non-empty, as for `forges[].name`.
- Not shown anywhere else (changelog, `heraut commit tickets`, `--json`): out of scope.

### `commits.rules[].is_breaking` (new filter)

- Boolean. `true`: the rule only checks commits the parser marks breaking (`Commit.Breaking`: a `!`
  before the header's colon, or a `BREAKING CHANGE` / `BREAKING-CHANGE` footer).
- `false` is the same as leaving it out. There is no "only non-breaking commits" filter (YAGNI).
- Combined with `types` and `scopes`: a commit must match every filter the rule sets.

### `commits.rules[].refuse` (new kind, replaces `deny_breaking`)

- Boolean. `true`: refuse every commit the rule's filters select.
- Counts in "exactly one kind" with `deny`, `require`, `require_ticket`. `false` counts as unset.
- Must have at least one filter (`types`, `scopes` or `is_breaking`), or it would refuse every commit.
  Error at `commits.rules[i].refuse`: `needs at least one of types, scopes or is_breaking`, hint
  `without a filter this rule refuses every commit`.
- `target` is refused with it (nothing is matched against text). Error at `commits.rules[i].target`:
  `not allowed with refuse, which matches no text`, hint `remove target from this rule`.
- `message` optional. Default: `commit refused (rule "<name>")`.

Why not `deny: true`: `deny` already takes a regex string; a key with two types was rejected for the
same reason as `require_ticket: <name>` below.

### `commits.rules[].tickets` (new, refines `require_ticket`)

- List of `commits.tickets[].name` values. The commit must match at least one of those entries'
  patterns, in the rule's `target`.
- Only valid with `require_ticket: true`. Error at `commits.rules[i].tickets`:
  `needs require_ticket: true`. Not implicit: the rule's kind stays visible, and `tickets` stays free
  to refine another kind later without a breaking change.
- Each name must exist in `commits.tickets`. Error at `commits.rules[i].tickets[j]`:
  `unknown ticket "jira"`, hint `name an entry of commits.tickets (set its name)`.
- An empty list (`tickets: []`) is the same as leaving it out: any ticket.
- Default message when `message` is unset and `tickets` is set:
  `commit must reference a ticket of kind jira or github (rule "<name>")`. Without `tickets`, the
  existing default is unchanged.

Why not `require_ticket: jira` (one key taking `true`, a name or a list): rejected by the user, it
gives one key three shapes, complicating the schema and editor autocomplete.

### `deny_breaking` is removed

Added by T557 on this branch, never merged or released, so it goes without deprecation. Everything
T557 wrote for it is reworked: `CommitRule.DenyBreaking`, its validation and evaluation, schema,
sample, spec 02, the ADR-0056 addendum, the "Commit rules" guide, the `commit-conventions.md` table,
the generated reference, the fixture, and heraut's own `.config/heraut.yml`, which becomes:

```yaml
    - name: no-breaking
      is_breaking: true
      refuse: true
      message: "breaking changes are not allowed: a major release is cut with the Release workflow's version input"
```

The behaviour pinned by T557's tests carries over (same commits refused, same messages where a
message is set); the tests are rewritten against the new keys, not deleted.

## Evaluation

In `app.verifyRules`, unchanged in shape:

1. `ruleAppliesTo` gains `is_breaking`: a rule with `IsBreaking` skips a commit whose `Breaking` is
   false.
2. `evaluateRule` gains `refuse` (always violated once the rule applies) and narrows the ticket list
   for `require_ticket` to the named entries before calling `native.MatchTickets`.

Violations are still collected, all rules on every commit.

## Compatibility

Additive under ADR-0080: three new optional keys. Existing configs behave the same (`require_ticket:
true` without `tickets` still means any ticket). `deny_breaking` never shipped.

## Docs

- Spec 02 `commits.tickets` and `commits.rules` sections; rule table split into filters and kinds.
- ADR-0056: the 2026-11-11 addendum is rewritten (not appended to) to describe the final model and why
  `deny_breaking` was dropped before release.
- `schema.json` (`Ticket.name`, `CommitRule.is_breaking`, `refuse`, `tickets`; drop `deny_breaking`),
  `docs/heraut.sample.yml`, `testdata/config/valid/commit-rules.yml` (and an invalid fixture if the
  schema can express a case, e.g. `refuse` not boolean), `mise run docs:gen`.
- User pages: the "Commit rules" guide (filters vs kinds, `refuse`, the breaking-commit ticket
  example, docs→Jira / others→GitHub by listing the other types), `tickets.md` (`name`, and a pointer
  to choosing a kind in a rule), `commit-conventions.md` table. Every example run against the built
  binary.

## Tests (first)

- Validator: ticket name duplicate; `refuse` valid with each filter, invalid with none, with `target`,
  with another kind; `tickets` without `require_ticket`, unknown name, valid names; `is_breaking`
  valid with any kind.
- `VerifyCommit`: `is_breaking` filter with `require_ticket` (breaking without ticket refused, with
  ticket accepted, non-breaking without ticket accepted); `tickets` narrowing (Jira accepted, GitHub
  refused for a `[jira]` rule); `refuse` with `types`, `scopes`, `is_breaking`; default messages.
- Own-config test keeps its table against the new keys.
- Schema: valid fixture covers every new key.

## Non-goals

- "Every type except X" for `types`. Raised by the user, to be discussed after this task.
- Forbidding tickets on a commit (`require_ticket: false` stays "not this kind").
- An "only non-breaking" filter.
- Ticket names in changelog, release notes, `heraut commit tickets` or `--json` output.
- `heraut commit create` (the wizard) honouring rules.

## Calls made without the user (check these)

- `is_breaking: false` means unset; no "only non-breaking" filter.
- `refuse` with no filter is a config error, and `refuse` refuses `target`.
- `refuse`'s default message: `commit refused (rule "<name>")`.
- `tickets: []` means any ticket; a name listed twice (`[jira, jira]`) is accepted, as harmless.
- `commits.tickets[].name` has no format rule beyond non-empty, like `forges[].name`.
- The ADR-0056 addendum is rewritten rather than appended to, since `deny_breaking` never shipped.
- Roadmap: T557's entry and note are amended to say `deny_breaking` became `is_breaking` + `refuse`;
  T558's text is updated to the names in this document (it still says `breaking`); both are ticked
  in the same pull request.

## Left to the builder

Commit split, test names, exact Go field names (`IsBreaking`, `Refuse`, `Tickets`), wording of the
guide around the examples.
