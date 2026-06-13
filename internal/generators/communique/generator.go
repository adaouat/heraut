package communique

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Generator implements port.Generator for communique.
type Generator struct {
	runner port.Runner
	cfg    *config.ContentDriver
}

var _ port.Generator = (*Generator)(nil)

// New constructs a Generator for communique.
func New(runner port.Runner, cfg *config.ContentDriver) *Generator {
	return &Generator{runner: runner, cfg: cfg}
}

// Check verifies that communique is available on PATH.
func (g *Generator) Check() error {
	_, _, err := g.runner.Run("communique", "--version")
	if err != nil {
		return fmt.Errorf("communique not found: %w", err)
	}
	return nil
}

// Validate ensures cfg.Config is set and the file exists.
func (g *Generator) Validate() error {
	if g.cfg.Config == "" {
		return fmt.Errorf("communique: config file is required")
	}
	if _, err := os.Stat(g.cfg.Config); err != nil {
		return fmt.Errorf("communique config %q: %w", g.cfg.Config, err)
	}
	return nil
}

// Generate invokes communique generate --config <file> <tag> and returns stdout.
//
// communique is opaque to heraut — link resolution lives entirely in the user's
// communique config — so the per-platform LinkContext is accepted and ignored
// (ADR-0021). Multi-platform users get identical notes across platforms.
func (g *Generator) Generate(tag string, _ *port.LinkContext) (string, error) {
	stdout, _, err := g.runner.Run("communique", "generate", "--config", g.cfg.Config, tag)
	if err != nil {
		return "", fmt.Errorf("communique: %w", err)
	}
	return stdout, nil
}
