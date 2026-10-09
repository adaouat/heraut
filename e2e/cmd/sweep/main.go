//go:build e2e_forge

// Command sweep deletes stale e2e-* branches, tags and releases left in the forge sandboxes by
// crashed runs. Usage: go run -tags e2e_forge ./e2e/cmd/sweep [-older-than 24h] [-dry-run]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/adaouat/heraut/e2e/forgeharness"
)

func main() {
	olderThan := flag.Duration("older-than", 24*time.Hour, "delete run resources older than this")
	dryRun := flag.Bool("dry-run", false, "only report what would be deleted")
	flag.Parse()

	c := forgeharness.LoadConfig()
	var forges []forgeharness.Forge
	if c.GitHubRepo != "" {
		tok, err := forgeharness.GitHubToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitHub(c, tok))
	}
	if c.GitLabProject != "" {
		tok, err := forgeharness.GitLabToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitLab(c, tok))
	}
	if c.GitHubEnrichRepo != "" {
		tok, err := forgeharness.GitHubToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitHub(c.ForEnrich("github"), tok))
	}
	if c.GitLabEnrichProject != "" {
		tok, err := forgeharness.GitLabToken()
		exitOn(err)
		forges = append(forges, forgeharness.NewGitLab(c.ForEnrich("gitlab"), tok))
	}
	if len(forges) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to sweep: set HERAUT_E2E_GITHUB_REPO / HERAUT_E2E_GITLAB_PROJECT (and the *_ENRICH_* pair)")
		return
	}
	for _, f := range forges {
		exitOn(f.Check())
		deleted, err := forgeharness.Sweep(f, *olderThan, time.Now(), *dryRun)
		for _, d := range deleted {
			fmt.Printf("%s: %s\n", f.Name(), d)
		}
		exitOn(err)
	}
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "sweep:", err)
		os.Exit(1)
	}
}
