package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

const ticketsCfg = `commits:
  tickets:
    - name: jira
      pattern: "[A-Z]+-[0-9]+"
      url: https://jira.example.com/browse/{ticket}
    - name: github
      pattern: "#([0-9]+)"
      url: https://github.com/acme/app/issues/{ticket}
  rules:
`

// commits.rules filters (types, scopes, is_breaking) pick the commits a rule checks; its kind
// (refuse, require_ticket with named tickets) decides. verify and check exit 1 on a refused
// commit; check config exits 2 on a rule that cannot work.
func TestCommitRules(t *testing.T) {
	bin := harness.Binary(t)
	verify := func(msg string) []string { return []string{"commit", "verify", msg} }
	noBreaking := semverCfg("") + ticketsCfg + "    - name: no-breaking\n      is_breaking: true\n      refuse: true\n"
	fixNeedsJira := semverCfg("") + ticketsCfg +
		"    - name: fix-needs-jira\n      types: [fix]\n      require_ticket: true\n      tickets: [jira]\n"

	runScenarios(t, bin, nil, []scenario{
		{name: "verify refuses a breaking commit", config: noBreaking,
			args:     verify("feat(api)!: drop v1 routes"),
			wantExit: exitUsage, wantText: []string{`commit refused (rule "no-breaking")`}},
		{name: "verify refuses a BREAKING CHANGE footer", config: noBreaking,
			args:     verify("feat: new routes\n\nBREAKING CHANGE: v1 is gone"),
			wantExit: exitUsage, wantText: []string{`commit refused (rule "no-breaking")`}},
		{name: "verify accepts a non-breaking commit", config: noBreaking,
			args:    verify("feat: add login"),
			wantOut: "✓ commit message is valid\n  type:        feat\n  breaking:    false\n  description: add login"},

		{name: "a jira-only rule refuses a GitHub issue", config: fixNeedsJira,
			args:     verify("fix: crash on empty query #34"),
			wantExit: exitUsage, wantText: []string{`commit must reference a ticket of kind jira (rule "fix-needs-jira")`}},
		{name: "a jira-only rule accepts a Jira key", config: fixNeedsJira,
			args: verify("fix: crash on empty query APP-12"),
			wantOut: "✓ commit message is valid\n  type:        fix\n  breaking:    false\n  description: crash on empty query APP-12\n" +
				"  tickets:     APP-12 (https://jira.example.com/browse/APP-12)"},
		{name: "a jira-only rule leaves other types alone", config: fixNeedsJira,
			args: verify("feat: add login #34"),
			wantOut: "✓ commit message is valid\n  type:        feat\n  breaking:    false\n  description: add login #34\n" +
				"  tickets:     #34 (https://github.com/acme/app/issues/34)"},

		{name: "check refuses a breaking commit in the range", config: noBreaking,
			history:  []step{commit("feat: first"), tag("v1.0.0"), commit("feat: add login"), commit("feat!: drop v1 routes")},
			args:     []string{"commit", "check", "--from-latest-tag"},
			wantExit: exitUsage, wantText: []string{"drop v1 routes", `commit refused (rule "no-breaking")`},
			notText: []string{"add login"}},

		{name: "check config: refuse without a filter", config: semverCfg("") + ticketsCfg + "    - name: refuse-all\n      refuse: true\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{"commits.rules[0].refuse: needs at least one of types, scopes or is_breaking"}},
		{name: "check config: tickets naming an unknown entry", config: semverCfg("") + ticketsCfg +
			"    - name: needs-linear\n      require_ticket: true\n      tickets: [linear]\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{`commits.rules[0].tickets[0]: unknown ticket "linear"`}},
	})
}
