package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ticketCfg() *config.Config {
	return &config.Config{
		Commits: &config.Commits{
			Tickets: []config.Ticket{
				{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
			},
		},
	}
}

func TestCheckTicketsInRange_NoRange_OmitsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01fix: resolve PROJ-42\x01\x00", "", nil)

	_, err := app.CheckTicketsInRange(mr, ticketCfg(), "")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"log", "--format=%h%x01%s%x01%b%x00"}, mr.Calls[0].Args)
}

func TestCheckTicketsInRange_WithRange_AppendsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01fix: resolve PROJ-42\x01\x00", "", nil)

	_, err := app.CheckTicketsInRange(mr, ticketCfg(), "main..HEAD")
	require.NoError(t, err)

	assert.Equal(t, []string{"log", "main..HEAD", "--format=%h%x01%s%x01%b%x00"}, mr.Calls[0].Args)
}

func TestCheckTicketsInRange_MatchesSubjectAndBody(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(
		"aaa1111\x01fix: resolve PROJ-42\x01\x00"+
			"bbb2222\x01chore: cleanup\x01Refs PROJ-99\x00"+
			"ccc3333\x01docs: typo\x01\x00",
		"", nil)

	results, err := app.CheckTicketsInRange(mr, ticketCfg(), "")
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "aaa1111", results[0].SHA)
	assert.Equal(t, []native.TicketMatch{{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"}}, results[0].Matches)

	assert.Equal(t, "bbb2222", results[1].SHA)
	assert.Equal(t, []native.TicketMatch{{Text: "PROJ-99", Href: "https://jira.example.com/browse/PROJ-99"}}, results[1].Matches)

	assert.Equal(t, "ccc3333", results[2].SHA)
	assert.Empty(t, results[2].Matches)
}

func TestCheckTicketsInRange_NoTicketsConfigured_ReturnsNoMatchesNotError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("aaa1111\x01fix: resolve PROJ-42\x01\x00", "", nil)

	results, err := app.CheckTicketsInRange(mr, &config.Config{}, "")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].Matches)
}
