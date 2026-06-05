package main

import (
	"context"
	"os"

	"github.com/adaouat/forge/cli"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/ui"
)

// Version is injected at build time via -ldflags "-X main.Version={{.Tag}}".
var Version = "dev"

func main() {
	root := cmd.NewRootCmd(Version)
	err := cli.Run(context.Background(), root, Version, ui.Accent())
	os.Exit(cmd.ExitCode(err))
}
