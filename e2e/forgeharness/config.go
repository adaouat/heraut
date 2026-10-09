//go:build e2e_forge

// Package forgeharness drives the built heraut binary against real sandbox repositories (Lane B,
// ADR-0066). Every file is behind the e2e_forge build tag.
package forgeharness

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// Config holds the sandbox coordinates; it never contains a token.
type Config struct {
	GitHubRepo    string
	GitLabProject string
	Pattern       string

	GitHubEnrichRepo    string
	GitLabEnrichProject string
}

// LoadConfig reads the sandbox coordinates from the environment.
func LoadConfig() Config {
	c := Config{
		GitHubRepo:    os.Getenv("HERAUT_E2E_GITHUB_REPO"),
		GitLabProject: os.Getenv("HERAUT_E2E_GITLAB_PROJECT"),
		Pattern:       os.Getenv("HERAUT_E2E_REPO_PATTERN"),

		GitHubEnrichRepo:    os.Getenv("HERAUT_E2E_GITHUB_ENRICH_REPO"),
		GitLabEnrichProject: os.Getenv("HERAUT_E2E_GITLAB_ENRICH_PROJECT"),
	}
	if c.Pattern == "" {
		c.Pattern = "*testing*"
	}
	return c
}

// Guard refuses a repository whose base name does not match the sandbox pattern.
func (c Config) Guard(coordinates string) error {
	base := coordinates[strings.LastIndex(coordinates, "/")+1:]
	ok, err := path.Match(c.Pattern, base)
	if err != nil {
		return fmt.Errorf("invalid HERAUT_E2E_REPO_PATTERN %q: %w", c.Pattern, err)
	}
	if !ok {
		return fmt.Errorf("refusing %q: its name does not match the sandbox pattern %q", coordinates, c.Pattern)
	}
	return nil
}

// ForEnrich returns a copy of c whose repository for the named forge is the enrichment pair.
func (c Config) ForEnrich(name string) Config {
	switch name {
	case "github":
		c.GitHubRepo = c.GitHubEnrichRepo
	case "gitlab":
		c.GitLabProject = c.GitLabEnrichProject
	}
	return c
}
