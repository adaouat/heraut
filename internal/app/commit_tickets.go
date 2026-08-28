package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/port"
)

// TicketCheckResult is one commit's ticket-pattern matches for `heraut commit tickets`.
type TicketCheckResult struct {
	SHA     string
	Subject string
	Matches []native.TicketMatch
}

// CheckTicketsInRange walks every commit in revRange (or the full history reachable from
// HEAD when revRange is "") and matches each against cfg.Tickets() (commits.tickets) using
// native.MatchTickets — the exact same matching applied when rendering changelog/
// release-notes ticket links, against the same text (subject, then body on its own line
// when non-empty). A nil cfg or unconfigured commits.tickets yields zero matches per
// commit, not an error — callers decide whether "nothing configured" is worth failing on.
func CheckTicketsInRange(runner port.Runner, cfg *config.Config, revRange string) ([]TicketCheckResult, error) {
	var tickets []config.Ticket
	if cfg != nil {
		tickets = cfg.Tickets()
	}

	args := []string{"log"}
	if revRange != "" {
		args = append(args, revRange)
	}
	args = append(args, "--format=%h%x01%s%x01%b%x00")

	stdout, _, err := runner.Run("git", args...)
	if err != nil {
		return nil, fmt.Errorf("listing commits in range %q: %w", revRange, err)
	}

	var results []TicketCheckResult
	for _, record := range strings.Split(stdout, "\x00") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x01", 3)
		if len(fields) != 3 {
			continue
		}
		sha, subject, body := fields[0], fields[1], fields[2]

		text := subject
		if body != "" {
			text += "\n" + body
		}
		results = append(results, TicketCheckResult{
			SHA:     sha,
			Subject: subject,
			Matches: native.MatchTickets(text, tickets),
		})
	}
	return results, nil
}
