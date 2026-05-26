package pipeline

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

const defaultCommitMessage = "chore(release): ${version}"

type gitHelper struct {
	runner port.Runner
}

func (g *gitHelper) run(name string, args ...string) error {
	_, _, err := g.runner.Run(name, args...)
	return err
}

func (g *gitHelper) commitChangelog(file, msg string) error {
	if err := g.run("git", "add", file); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := g.run("git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := g.run("git", "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func (g *gitHelper) tag(tag, msg string, annotated bool) error {
	if annotated {
		return g.run("git", "tag", "-a", tag, "-m", msg)
	}
	return g.run("git", "tag", tag)
}

func commitMessage(template, version string) string {
	if template == "" {
		template = defaultCommitMessage
	}
	return strings.ReplaceAll(template, "${version}", version)
}
