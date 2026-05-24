package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
)

func TestNewRootCmd(t *testing.T) {
	root := cmd.NewRootCmd()
	if root == nil {
		t.Fatal("NewRootCmd() returned nil")
	}
	if root.Use != "heraut" {
		t.Errorf("Use = %q, want %q", root.Use, "heraut")
	}
	if root.Short == "" {
		t.Error("Short description is empty")
	}

	flags := root.PersistentFlags()
	for _, name := range []string{"config", "dry-run", "verbose", "env", "force"} {
		if flags.Lookup(name) == nil {
			t.Errorf("persistent flag %q not registered", name)
		}
	}
}
