package app

import (
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/port"
)

func buildGitHubPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	return github.New(runner, cfg), nil
}

func buildGitLabPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	return gitlab.New(runner, cfg), nil
}

// platformConfigFromTarget builds the config.Platform an existing platform driver accepts from a
// resolved forge identity plus a target's publish options. The drivers are unchanged (ADR-0043 P3):
// the identity supplies host and project/repository, so publishing inherits CI/git auto-detection,
// while the token still resolves via TokenEnv because the drivers own their auth environment.
func platformConfigFromTarget(t config.Target, f config.Forge, id port.ForgeIdentity) config.Platform {
	cfg := config.Platform{
		Name:       f.Name,
		Type:       id.Type,
		BaseURL:    id.Host,
		TokenEnv:   f.TokenEnv,
		Draft:      t.Draft,
		Prerelease: t.Prerelease,
		Assets:     t.Assets,
	}
	if id.Type == "github" {
		cfg.Repository = id.Project
	} else {
		cfg.Project = id.Project
		cfg.Repository = id.Repository
	}
	return cfg
}
