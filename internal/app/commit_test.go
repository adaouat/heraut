package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestVerifyCommit_DefaultTypes_Valid(t *testing.T) {
	err := app.VerifyCommit(nil, "feat: add x")
	assert.NoError(t, err)
}

func TestVerifyCommit_DefaultTypes_RejectsUnknownType(t *testing.T) {
	err := app.VerifyCommit(nil, "wip: not a real type")
	assert.Error(t, err)
}

func TestVerifyCommit_DefaultTypes_AllTenAccepted(t *testing.T) {
	for _, typ := range app.DefaultCommitTypes {
		err := app.VerifyCommit(nil, typ+": something")
		assert.NoError(t, err, "type %q should be accepted by default", typ)
	}
}

func TestVerifyCommit_ConfiguredTypes_OverridesDefault(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat", "fix"}}}

	assert.NoError(t, app.VerifyCommit(cfg, "feat: add x"))
	assert.Error(t, app.VerifyCommit(cfg, "docs: update readme")) // not in the configured list, even though it's a default type
}

func TestVerifyCommit_InvalidGrammar_Errors(t *testing.T) {
	err := app.VerifyCommit(nil, "not a conventional commit at all")
	assert.Error(t, err)
}

func TestVerifyCommit_MergeCommit_Skipped(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat"}}} // strict — would reject almost anything
	err := app.VerifyCommit(cfg, "Merge branch 'main' into feature/x")
	assert.NoError(t, err)
}

func TestVerifyCommit_FixupCommit_Skipped(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat"}}}
	err := app.VerifyCommit(cfg, "fixup! docs: typo")
	assert.NoError(t, err)
}

func TestVerifyCommit_SquashCommit_Skipped(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat"}}}
	err := app.VerifyCommit(cfg, "squash! docs: typo")
	assert.NoError(t, err)
}

func TestVerifyCommit_EmptyConfiguredTypes_FallsBackToDefault(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{}}}
	err := app.VerifyCommit(cfg, "feat: add x")
	assert.NoError(t, err)
}
