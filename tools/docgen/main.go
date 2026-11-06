// Command docgen writes the generated user reference under docs/reference. Run it with
// `mise run docs:gen` from the repository root.
package main

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/cmd"
)

const configPage = "docs/reference/configuration.md"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := cmd.WriteCommandReference(cmd.NewRootCmd("dev"), "docs/reference/commands"); err != nil {
		return err
	}
	schema, err := os.ReadFile("schema.json")
	if err != nil {
		return fmt.Errorf("reading schema.json: %w", err)
	}
	doc, err := os.ReadFile(configPage)
	if err != nil {
		return fmt.Errorf("reading %s: %w", configPage, err)
	}
	table, err := configReference(schema)
	if err != nil {
		return err
	}
	out, err := replaceGenerated(string(doc), table)
	if err != nil {
		return fmt.Errorf("%s: %w", configPage, err)
	}
	if err := os.WriteFile(configPage, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", configPage, err)
	}
	return nil
}
