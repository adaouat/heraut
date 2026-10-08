package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

type step struct{ commit, tag string }

func commit(msg string) step { return step{commit: msg} }
func tag(name string) step   { return step{tag: name} }

const (
	exitOK         = 0
	exitConfig     = 2
	exitRuntime    = 3
	exitPromotion  = 4
	exitAnyFailure = -1 // any non-zero code: the spec does not pin this failure's class
)

type scenario struct {
	name     string
	config   string
	history  []step
	args     []string
	env      []string // extra environment for this row only
	branch   string   // checked out before the history is replayed (empty: main)
	wantExit int
	wantOut  string   // exact trimmed stdout; checked only when wantExit == exitOK
	wantText []string // lower-cased, whitespace-collapsed substrings of stdout+stderr
	notText  []string // substrings that must be absent
}

// normalize lower-cases and collapses whitespace: the CLI error panel re-cases and wraps text.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func runScenarios(t *testing.T, bin string, env []string, tests []scenario) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := harness.NewRepo(t)
			repo.WriteConfig(tc.config)
			if tc.branch != "" {
				repo.Checkout(tc.branch)
			}
			for _, s := range tc.history {
				if s.tag != "" {
					repo.Tag(s.tag)
				} else {
					repo.Commit(s.commit)
				}
			}

			res := repo.Run(bin, append(append([]string{}, env...), tc.env...), tc.args...)

			if tc.wantExit == exitAnyFailure {
				require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			} else {
				require.Equal(t, tc.wantExit, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			}
			if tc.wantExit == exitOK {
				assert.Equal(t, tc.wantOut, strings.TrimSpace(res.Stdout))
			}
			all := normalize(res.Stdout + " " + res.Stderr)
			for _, want := range tc.wantText {
				assert.Contains(t, all, want)
			}
			for _, not := range tc.notText {
				assert.NotContains(t, all, not)
			}
		})
	}
}
