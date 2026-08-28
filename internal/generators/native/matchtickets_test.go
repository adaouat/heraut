package native_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/stretchr/testify/assert"
)

// TestMatchTickets covers T241: MatchTickets is the exported entry point internal/app uses
// (heraut commit tickets) to verify commits.tickets patterns against real commit text, using
// the exact same matching native applies when rendering changelog/release-notes ticket links.
func TestMatchTickets(t *testing.T) {
	tickets := []config.Ticket{
		{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
	}

	got := native.MatchTickets("fix: resolve PROJ-42", tickets)

	assert.Equal(t, []native.TicketMatch{
		{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"},
	}, got)
}

func TestMatchTickets_NoMatch(t *testing.T) {
	tickets := []config.Ticket{
		{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
	}

	assert.Empty(t, native.MatchTickets("fix: unrelated change", tickets))
}
