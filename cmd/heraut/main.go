package main

import (
	"context"
	"os"

	"charm.land/fang/v2"

	"github.com/adaouat/heraut/internal/cmd"
)

// Version is injected at build time via -ldflags "-X main.Version={{.Tag}}".
var Version = "dev"

func main() {
	root := cmd.NewRootCmd(Version)
	err := fang.Execute(context.Background(), root,
		fang.WithVersion(Version), fang.WithColorSchemeFunc(colorScheme))
	os.Exit(cmd.ExitCode(err))
}
