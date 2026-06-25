package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/stretchr/testify/assert"
)

func TestPrintCommitCheckResults_AllValid_NoVerbose_OnlySummary(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "feat: a", Err: nil},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, false, &buf)

	assert.Equal(t, 0, failed)
	out := buf.String()
	assert.NotContains(t, out, "aaa1111")
	assert.NotContains(t, out, "bbb2222")
	assert.Contains(t, out, "0 of 2 commits invalid")
}

func TestPrintCommitCheckResults_SomeInvalid_NoVerbose_OnlyFailuresShown(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "not conventional", Err: errors.New("validating commit message: boom")},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, false, &buf)

	assert.Equal(t, 1, failed)
	out := buf.String()
	assert.Contains(t, out, "aaa1111")
	assert.Contains(t, out, "not conventional")
	assert.NotContains(t, out, "bbb2222")
	assert.Contains(t, out, "1 of 2 commits invalid")
}

func TestPrintCommitCheckResults_Verbose_ShowsEveryCommit(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "not conventional", Err: errors.New("boom")},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, true, &buf)

	assert.Equal(t, 1, failed)
	out := buf.String()
	assert.Contains(t, out, "aaa1111")
	assert.Contains(t, out, "bbb2222")
	assert.Contains(t, out, "1 of 2 commits invalid")
}
