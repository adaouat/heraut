package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
)

func TestNewRootCmd(t *testing.T) {
	root := cmd.NewRootCmd("dev")
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

func TestNewRootCmd_RegistersWhatsNew(t *testing.T) {
	root := cmd.NewRootCmd("v1.0.0")
	for _, c := range root.Commands() {
		if c.Name() == "whatsnew" {
			return
		}
	}
	t.Error("whatsnew subcommand not registered")
}
