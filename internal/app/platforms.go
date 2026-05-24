package app

import (
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/port"
)

func buildGitHubPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	return github.New(runner, cfg), nil
}
