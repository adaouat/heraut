# Commit-rule filters and named tickets (T557 rework + T558) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unreleased `deny_breaking` rule kind with an `is_breaking` filter plus a `refuse` kind, and let `require_ticket` rules name which `commits.tickets` entries they accept.

**Architecture:** Config structs and two-phase validation in `internal/config/`; rule evaluation in `internal/app/commit.go` (`ruleAppliesTo` for filters, `evaluateRule` for kinds); ticket matching through `native.MatchTickets` on a narrowed `[]config.Ticket`. No new packages, no layer changes.

**Tech Stack:** Go, yaml.v3, testify, JSON Schema (`schema.json`), hk (`typos`, `golangci_lint`, `docs_links`, `heraut-commit-lint`).

**Spec:** `docs/superpowers/specs/2026-11-12-commit-rule-filters-and-ticket-refs-design.md` — the source of every name, message and rule below.

## Global Constraints

- Branch `feat/deny-breaking-rule`; never commit to `main`; do not push.
- Conventional commits, subject ≤ 72 chars, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`, never a `Claude-Session` line. Never `--no-verify`.
- Every commit compiles and passes `go test ./...` (hk stashes unstaged edits but keeps untracked files).
- TDD: the failing test runs red before the implementation.
- Wire names: `commits.tickets[].name`, `commits.rules[].is_breaking`, `commits.rules[].refuse`, `commits.rules[].tickets`. Go fields: `Ticket.Name`, `CommitRule.IsBreaking`, `CommitRule.Refuse`, `CommitRule.Tickets []string`, all `omitempty`. `deny_breaking` / `DenyBreaking` disappears everywhere.
- Exact messages (path → message, hint):
  - `commits.tickets[i].name` → `duplicate ticket name "<n>" (already used by tickets[<first>])`, hint `ticket names must be unique within commits.tickets`
  - `commits.rules[i]` (none set) → `must set exactly one of deny, require, require_ticket, or refuse`; (several) → `set only one of deny, require, require_ticket, or refuse, not more than one`
  - `commits.rules[i].refuse` → `needs at least one of types, scopes or is_breaking`, hint `without a filter this rule refuses every commit`
  - `commits.rules[i].target` with refuse → `not allowed with refuse, which matches no text`, hint `remove target from this rule`
  - `commits.rules[i].tickets` without require_ticket → `needs require_ticket: true`
  - `commits.rules[i].tickets[j]` → `unknown ticket "<n>"`, hint `name an entry of commits.tickets (set its name)`
  - default refuse message → `commit refused (rule "<name>")`
  - default require_ticket message with `tickets` → `commit must reference a ticket of kind <a> or <b> (rule "<name>")` (names joined with ` or `, in rule order); without `tickets`, unchanged.
- `is_breaking: false`, `refuse: false` and `tickets: []` behave as unset. A name listed twice in `tickets` is accepted.
- Ticket and rule-ticket names match exactly (case-sensitive).
- User pages: no links to specs/ADRs, no task ids, synthetic names only, every example's output copied from the built binary.

## Review Focus

1. A `require_ticket` + `tickets: [jira]` rule with `target: footer` — a Jira key only in the subject is still refused (narrowing must not bypass `target`). Test in Task 3.
2. A mix of named and unnamed ticket entries — `require_ticket: true` without `tickets` still accepts a match from an unnamed entry. Test in Task 3.
3. A named entry whose pattern has a capture group (`#([0-9]+)`) — narrowing to it still matches `#34`. Test in Task 3.
4. `refuse` + `is_breaking` on a merge or fixup commit — still accepted unconditionally. Test in Task 2.
5. `tickets: [Jira]` when the entry is `jira` — config error, not a silent never-match. Test in Task 3.

---

### Task 1: Spec and ADR first

**Files:**
- Modify: `docs/specs/02-configuration.md` (§ `commits.tickets` near line 800, § `commits.rules` lines ~818–850)
- Modify: `docs/adr/0056-configurable-commit-message-rules.md` (rewrite the `## Addendum (2026-11-11)` section, do not append another)

- [ ] **Step 1:** In spec 02 add `name` to the `commits.tickets` fields; split the `commits.rules` table into **Filters** (`types`, `scopes`, `is_breaking`) and **Kinds** (`deny`, `require`, `require_ticket` + `tickets`, `refuse`), with every validation rule and default message from Global Constraints; replace the `deny_breaking` example with the spec's YAML block (no-breaking, breaking-needs-ticket, breaking-needs-jira, docs-need-jira).
- [ ] **Step 2:** Rewrite the ADR addendum: title `## Addendum (2026-11-11, revised 2026-11-12): breaking-commit filter, \`refuse\`, named tickets`; why `deny_breaking` was dropped before release (verdict vs condition); why `refuse` not `deny: true`; why `tickets` beside `require_ticket` not `require_ticket: <name>`; why `tickets` needs explicit `require_ticket`.
- [ ] **Step 3:** `go test -run TestDocumentationLinks .` → PASS.
- [ ] **Step 4: Commit** `docs(adr): revise 0056 addendum for is_breaking, refuse and named tickets`

### Task 2: `is_breaking` filter and `refuse` kind replace `deny_breaking`

**Files:**
- Modify: `internal/config/commits.go` (`CommitRule`), `internal/config/validate_commits.go` (rules loop), `internal/app/commit.go` (`ruleAppliesTo`, `evaluateRule`)
- Modify: `schema.json` (`CommitRule`), `testdata/config/valid/commit-rules.yml`, `docs/heraut.sample.yml`, `.config/heraut.yml`, `docs/reference/configuration.md` (generated)
- Test: `internal/config/validator_test.go`, `internal/app/commit_test.go`, `internal/app/commit_own_config_test.go`

**Interfaces:**
- Produces: `config.CommitRule{IsBreaking bool; Refuse bool}` (`yaml:"is_breaking,omitempty"`, `yaml:"refuse,omitempty"`); `DenyBreaking` removed.

- [ ] **Step 1: Rewrite the failing tests.**
  - `TestValidate_CommitsRule_ValidConfigs`: replace the two `deny_breaking` rows with `refuse with is_breaking` (`is_breaking: true`, `refuse: true`, no message), `refuse with types` (`types: [chore]`, `refuse: true`), `refuse with scopes`, and `is_breaking with require` (`is_breaking: true`, `require: 'x'`, `message: m`).
  - `TestValidate_CommitsRuleExactlyOneOf…`: rows `deny and refuse both set` → `only one`; `refuse false counts as unset` → `exactly one`.
  - Replace `TestValidate_CommitsRuleDenyBreakingRefusesTarget` with `TestValidate_CommitsRuleRefuseRefusesTarget` (path `commits.rules[0].target`, message contains `not allowed with refuse`) and add `TestValidate_CommitsRuleRefuseNeedsAFilter` (`refuse: true` alone; `is_breaking: false` + `refuse: true` — both give path `commits.rules[0].refuse`, message contains `needs at least one of types, scopes or is_breaking`).
  - Replace `TestVerifyCommit_Rules_DenyBreaking` with `TestVerifyCommit_Rules_RefuseBreaking`: same 8 rows, rule `{Name: "no-breaking", IsBreaking: true, Refuse: true, Types: tc.types, Message: "cut a major on purpose"}`; add rows `merge commit` (`Merge branch 'x'`, accepted) and `fixup commit` (`fixup! feat!: drop x`, accepted).
  - Replace `…DenyBreaking_DefaultMessage` with `TestVerifyCommit_Rules_Refuse_DefaultMessage` → error contains `commit refused (rule "no-breaking")`.
  - Add `TestVerifyCommit_Rules_IsBreakingFilter`: rule `{IsBreaking: true, Require: "x-ok", Message: "needs x-ok"}`; `feat: plain` accepted, `feat!: drop` refused, `feat!: drop x-ok` accepted.
  - Add `TestVerifyCommit_Rules_RefuseByType`: `{Types: []string{"chore"}, Refuse: true}` → `chore: x` refused, `fix: x` accepted.
  - `commit_own_config_test.go`: unchanged table (it already asserts the message text, which stays).
- [ ] **Step 2:** `go test ./internal/config ./internal/app` → FAIL (unknown field `IsBreaking`/`Refuse`).
- [ ] **Step 3: Implement.** Swap the field; count `Refuse` (not `DenyBreaking`) in exactly-one; the two new refuse errors (target check replaces the `deny_breaking` one, still `else if` before the target-enum check); `ruleAppliesTo` returns false when `r.IsBreaking && !c.Breaking`; `evaluateRule` `case r.Refuse:` always violated with the default message. Update the `CommitRule` doc comments.
- [ ] **Step 4:** Config side: `.config/heraut.yml` rule becomes `is_breaking: true` + `refuse: true` (same `message`); `schema.json` drops `deny_breaking`, adds `is_breaking` and `refuse` (booleans, default false, descriptions ≤ 300 chars) and updates the `CommitRule` and `target`/`message` descriptions; fixture's `no-breaking` rule uses the new keys; sample comment lists `deny/require/require_ticket/refuse` and the `no-breaking` example uses `is_breaking` + `refuse`. Run `mise run docs:gen`.
- [ ] **Step 5:** `go test ./...` → PASS; `rtk proxy grep -rn deny_breaking --exclude-dir=.claude . ` → only `docs/tasks/roadmap.md`, the design doc and the ADR prose explaining the drop.
- [ ] **Step 6: Commit** `feat(config): replace deny_breaking with is_breaking and refuse`

### Task 3: Named tickets and a rule's `tickets` list

**Files:**
- Modify: `internal/config/config.go` (`Ticket`), `internal/config/commits.go` (`CommitRule`), `internal/config/validate_commits.go` (`validateTickets`, rules loop), `internal/app/commit.go` (`evaluateRule`)
- Modify: `schema.json` (ticket item, `CommitRule`), `testdata/config/valid/commit-rules.yml`, `docs/heraut.sample.yml`, `docs/reference/configuration.md` (generated)
- Test: `internal/config/validator_test.go`, `internal/app/commit_test.go`

**Interfaces:**
- Consumes: `CommitRule.IsBreaking` (Task 2).
- Produces: `config.Ticket{Name string \`yaml:"name,omitempty"\`}`; `config.CommitRule{Tickets []string \`yaml:"tickets,omitempty"\`}`.

- [ ] **Step 1: Failing tests.**
  - `TestValidate_TicketNameDuplicate`: two entries named `jira` → path `commits.tickets[1].name`, message contains `duplicate ticket name "jira"`. Also a row where the first entry has an empty `url` (the name check must run before the url `continue`).
  - `TestValidate_CommitsRuleTickets` (table, path + substring): `tickets` without `require_ticket` (with `deny`) → `commits.rules[0].tickets`, `needs require_ticket: true`; unknown name → `commits.rules[0].tickets[0]`, `unknown ticket "linear"`; case mismatch `Jira` vs `jira` → `commits.rules[0].tickets[0]`, `unknown ticket "Jira"`.
  - `TestValidate_CommitsRule_ValidConfigs` rows: `require_ticket with tickets` (`[jira]`), `tickets listed twice` (`[jira, jira]`), `empty tickets list` (`[]`), `is_breaking with require_ticket and tickets`. Give the test's shared `tickets:` block names `jira` and `github` (`'#([0-9]+)'`).
  - `TestVerifyCommit_Rules_RequireTicket_Named` (table over cfg with tickets `jira` `[A-Z]+-[0-9]+` and `github` `#([0-9]+)`):
    - rule `tickets: [jira]`: `fix: x APP-12` accepted; `fix: x #34` refused.
    - rule `tickets: [github]`: `fix: x #34` accepted (capture-group pattern, Review Focus 3).
    - rule `tickets: [jira]`, `Target: "footer"`: `fix: x APP-12` refused; `fix: x\n\nRefs: APP-12` accepted (Review Focus 1).
    - rule without `tickets`, cfg with an extra unnamed entry `LIN-[0-9]+`: `fix: LIN-7` accepted (Review Focus 2).
    - rule `{IsBreaking: true, RequireTicket: true, Tickets: [jira]}`: `feat!: drop` refused; `feat!: drop APP-1` accepted; `feat: plain` accepted.
  - `TestVerifyCommit_Rules_RequireTicket_NamedDefaultMessage`: `Tickets: ["jira", "github"]`, no message → error contains `commit must reference a ticket of kind jira or github (rule "needs-ticket")`.
- [ ] **Step 2:** `go test ./internal/config ./internal/app` → FAIL (unknown fields).
- [ ] **Step 3: Implement.** Name uniqueness in `validateTickets` (first-seen map, as `validateForges`); in the rules loop, the two `tickets` errors (`tickets` non-empty without `RequireTicket`; each name not among named entries); in `evaluateRule`'s `RequireTicket` case, narrow `tickets` to entries whose `Name` is in `r.Tickets` when `r.Tickets` is non-empty, and build the default message from `r.Tickets` with `strings.Join(r.Tickets, " or ")`.
- [ ] **Step 4:** `schema.json`: ticket item gains `name` (string, minLength 1); `CommitRule` gains `tickets` (array of strings). Fixture: name both entries, add a `docs-need-jira` rule. Sample: name the two ticket entries, add a `tickets: [jira]` example. `mise run docs:gen`.
- [ ] **Step 5:** `go test ./...` → PASS.
- [ ] **Step 6: Commit** `feat(config): let require_ticket rules name their tickets`

### Task 4: User docs

**Files:**
- Modify: `docs/guides/commit-rules.md`, `docs/guides/tickets.md`, `docs/guides/commit-conventions.md` (the `rules` table row), `docs/guides/README.md` and `docs/README.md` (only if the one-line descriptions need new words)

- [ ] **Step 1:** Build: `go build -o <scratchpad>/heraut ./cmd/heraut`; in a scratch repo with a `.heraut.yml` holding every example, run each `heraut commit verify` shown in the guides and copy the real output.
- [ ] **Step 2:** `commit-rules.md`: replace the single checks table with **Which commits** (`types`, `scopes`, `is_breaking`) and **What is checked** (`deny`, `require`, `require_ticket` + `tickets`, `refuse`) plus `target`/`message`; "No breaking change" uses `is_breaking` + `refuse` (and the `scopes: [api]` variant); new examples "A breaking change references a ticket", "Docs reference Jira, everything else GitHub" (the second rule lists the other types and says so plainly), "Freeze a part of the project" (`scopes: [legacy]` + `refuse: true`, `message: the legacy module is frozen`).
- [ ] **Step 3:** `tickets.md`: entries get `name:`; under "Require a ticket", one paragraph + example on `tickets: [jira]` and a link to `commit-rules.md`. `commit-conventions.md` row mentions `refuse` instead of `deny_breaking`.
- [ ] **Step 4:** `go test -run TestDocumentationLinks .` → PASS; `hk check` clean.
- [ ] **Step 5: Commit** `docs(guides): cover is_breaking, refuse and named tickets`

### Task 5: Roadmap

**Files:**
- Modify: `docs/tasks/roadmap.md` (Phase 102 row and section)

- [ ] **Step 1:** T557: append to its note a **Reworked before merge** bullet: `deny_breaking` became `is_breaking` + `refuse` (T558's design), never released. T558: update its text to the final names, flip to `[x]`, add a `Done 2026-11-12.` note (where it lives, tests, docs, deferred: "every type except", ticket names in outputs). Phase 102 row → `Done — T557–T558`.
- [ ] **Step 2: Commit** `docs(roadmap): complete T558`
