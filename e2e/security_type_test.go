package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// ADR-0075: security is a built-in commit type, rendered right after Bug Fixes, and the body of a
// commit is never read to decide its section.
func TestSecurityCommitType(t *testing.T) {
	bin := harness.Binary(t)
	verify := func(msg string) []string { return []string{"commit", "verify", msg} }
	notes := []string{"notes", "v1.1.0", "--offline"}
	released := func(commits ...step) []step {
		return append(append([]step{commit("feat: first"), tag("v1.0.0")}, commits...), tag("v1.1.0"))
	}
	const removeSecurity = "commits:\n  types:\n    - name: security\n      remove: true\n"

	runScenarios(t, bin, nil, []scenario{
		{name: "verify accepts security", config: semverCfg(""),
			args:    verify("security: patch the token leak"),
			wantOut: "✓ commit message is valid\n  type:        security\n  breaking:    false\n  description: patch the token leak"},
		{name: "verify accepts a scoped security commit", config: semverCfg(""),
			args:    verify("security(auth): rotate the signing key"),
			wantOut: "✓ commit message is valid\n  type:        security\n  scope:       auth\n  breaking:    false\n  description: rotate the signing key"},
		{name: "a repository can take the type out of the allow-list", config: semverCfg("") + removeSecurity,
			args:     verify("security: patch the token leak"),
			wantExit: exitUsage, wantText: []string{`commit type "security" is not allowed`}},

		{name: "a security commit with a scope", config: notesCfg,
			history: released(commit("security(auth): rotate the signing key")),
			args:    notes,
			wantOut: "### 🛡️ Security\n\n- *(auth)* Rotate the signing key"},
		{name: "a body mentioning security no longer files an unconfigured type under Security", config: notesCfg,
			history: released(commit("wip: experiment\n\nthis addresses a security vulnerability")),
			args:    notes,
			wantOut: "### 💼 Other\n\n- Experiment"},
		{name: "with the type removed, a security commit falls to Other", config: notesCfg + removeSecurity,
			history: released(commit("security: patch the token leak")),
			args:    notes,
			wantOut: "### 💼 Other\n\n- Patch the token leak"},
	})
}

// The Security section sits right after Bug Fixes, ahead of Refactor, whatever order the commits came in.
func TestSecurityCommitType_SectionOrder(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(notesCfg)
	applySteps(repo, []step{commit("feat: first"), tag("v1.0.0"),
		commit("refactor: tidy up"), commit("security: patch the token leak"), commit("fix: stop the crash"), commit("feat: add login"),
		tag("v1.1.0")})

	res := repo.Run(bin, nil, "notes", "v1.1.0", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	var headings []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		if h, ok := strings.CutPrefix(line, "### "); ok {
			headings = append(headings, h)
		}
	}
	assert.Equal(t, []string{"🚀 Features", "🐛 Bug Fixes", "🛡️ Security", "🚜 Refactor"}, headings)
	assert.Contains(t, res.Stdout, "- Patch the token leak")
}
