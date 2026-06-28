package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

func TestWithEnvDerivations_CarriesTickets(t *testing.T) {
	driver := &config.ContentDriver{Generator: "git-cliff"}
	cfg := &config.Config{
		Changelog: driver,
		Commits:   &config.Commits{Tickets: []config.Ticket{{Pattern: "[A-Z]+-[0-9]+", URL: "https://x.test/{ticket}"}}},
	}

	got := withEnvDerivations(driver, cfg, "")
	assert.Len(t, got.Tickets, 1)
	assert.Empty(t, driver.Tickets) // the original driver is never mutated
}
