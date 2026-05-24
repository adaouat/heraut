package main

import (
	"context"
	"os"

	"charm.land/fang/v2"

	"github.com/adaouat/heraut/internal/cmd"
)

// Build-time variables injected via -ldflags.
var (
	Version    = "dev"
	ProjectURL = "https://github.com/adaouat/heraut"
	LatestURL  = "https://api.github.com/repos/adaouat/heraut/releases/latest"
)

func main() {
	root := cmd.NewRootCmd()
	if err := fang.Execute(context.Background(), root, fang.WithVersion(Version)); err != nil {
		os.Exit(1)
	}
}
