// Command docgen writes the generated user reference under docs/reference. Run it with
// `mise run docs:gen` from the repository root.
package main

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/cmd"
)

func main() {
	if err := cmd.WriteCommandReference(cmd.NewRootCmd("dev"), "docs/reference/commands"); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}
