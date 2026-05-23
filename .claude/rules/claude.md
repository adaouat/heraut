# Claude behaviour rules

## Architectural decisions

- Challenge design choices when something seems wrong, suboptimal, or inconsistent with
  the stated goals — even if the user proposed it. State the concern clearly and explain
  why, then let the user decide.
- Never silently accept a decision that contradicts an ADR or a rule in this project.
  Surface the conflict and ask for clarification.

## Task discipline

- Never implement more than one roadmap task per session without explicit user approval.
- Never implement anything not on the current roadmap without asking first.
- If a task feels too large to implement safely in one step, break it down and propose
  the breakdown before starting.
- Always follow the three-step roadmap flow (see `workflow.md`). No shortcuts.

## TDD discipline

- Always write the failing test before writing implementation code.
- Never write implementation first and tests after.
- If the user asks to skip tests, push back and explain why the tests are needed.

## Code discipline

- Never implement more than what the current task requires.
- Never refactor, clean up, or improve surrounding code as part of a task unless the task
  explicitly asks for it.
- Never add features "while we're here".
- If linting or tests fail, fix the root cause — never silence the error or skip the
  check.

## Roadmap discipline

- Before starting a task: read `docs/tasks/todo.md`, confirm the task is `[ ]`, flip to
  `[~]`, commit.
- After completing a task: flip `[~]` → `[x]`, add a note in `docs/tasks/roadmap.md` under
  the task (actual decisions, deferred items, deviations), commit alongside the
  implementation.
- If a new task surfaces mid-implementation, add it to `docs/tasks/todo.md` — do not
  silently implement it.

## Source-of-truth hierarchy

When information conflicts, the order of trust is:

1. The current user message
2. `docs/adr/` (architecture decisions for this repo)
3. `docs/specs/` (behavioral specification)
4. `docs/tasks/roadmap.md` (planned approach)
5. Memory and prior conversation context

If two committed docs disagree, raise it — do not silently pick one.

## Irreversible operations

- Always ask before: force-pushing, deleting files/branches, resetting git history,
  modifying CI/CD pipelines, pushing to remote.
- Confirm scope explicitly — approval for one action does not imply approval for similar
  actions in other contexts.

## Memory

- Save important architectural decisions, user preferences, and non-obvious constraints
  to memory so they survive context resets.
- Verify memory before acting on it — stale memory is worse than no memory.
