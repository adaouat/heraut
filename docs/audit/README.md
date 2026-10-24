# Modernization audit trail

Snapshot (2026-10-24) of the written results of the code-modernization effort on heraut (an in-place
refactor, roadmap Phases 73 to 79, tasks T391 to T422). Local paths are scrubbed. The generated working
files (topology, reports, dated earlier versions) are not kept; the decisions that matter live in
[`docs/adr/`](../adr/) and [`docs/tasks/`](../tasks/).

| File | What it is |
|---|---|
| [`MODERNIZATION_BRIEF.md`](MODERNIZATION_BRIEF.md) | The approved plan, its phases and the final state of the effort |
| [`BUSINESS_RULES.md`](BUSINESS_RULES.md) | Behaviour rules mined from the code (Given/When/Then, with priorities) |
| [`RULE_REVIEWS.md`](RULE_REVIEWS.md), [`RULE_REVIEWS.json`](RULE_REVIEWS.json) | A person's verdicts on the flagged rules |
| [`SECURITY_FINDINGS.md`](SECURITY_FINDINGS.md) | Security findings and their resolution |

References inside these files to `analysis/heraut/...` name working files that were not committed.
